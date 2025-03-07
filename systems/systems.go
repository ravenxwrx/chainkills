package systems

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"git.sr.ht/~barveyhirdman/chainkills/common"
	"git.sr.ht/~barveyhirdman/chainkills/config"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	ColorOurKill  int = 0x93c47d
	ColorOurLoss  int = 0x990000
	ColorWhatever int = 0xbcbcbc

	packageName string = "git.sr.ht/~barveyhirdman/chainkills/systems"
)

var (
	whPattern = regexp.MustCompile("J[0-9]{6}")
	register  *SystemRegister
)

type System struct {
	Name          string `json:"name"`
	SolarSystemID int    `json:"solar_system_id"`
}

func (s System) String() string {
	return fmt.Sprintf("%d - %s", s.SolarSystemID, s.Name)
}

type SystemRegister struct {
	mx     *sync.Mutex
	stop   chan struct{}
	errors chan error

	ws      *websocket.Conn
	systems []System
}

type Option func(*SystemRegister)

func WithWebsocket(ws *websocket.Conn) Option {
	return func(s *SystemRegister) {
		s.ws = ws
	}
}

func Register(opts ...Option) *SystemRegister {
	if register == nil {
		register = &SystemRegister{
			mx:   &sync.Mutex{},
			stop: make(chan struct{}),

			systems: []System{},
		}

		for _, opt := range opts {
			opt(register)
		}
	}

	return register
}

func listHash(list []System) [32]byte {
	listJSON, _ := json.Marshal(list)
	return sha256.Sum256(listJSON)
}

func (s *SystemRegister) Errors() <-chan error {
	return s.errors
}

func (s *SystemRegister) Update(ctx context.Context) (bool, error) {
	_, span := otel.Tracer(packageName).Start(ctx, "Update")
	defer span.End()

	client := http.Client{}

	origHash := listHash(s.systems)

	url := fmt.Sprintf("%s/api/map/systems?slug=%s", config.Get().Wanderer.Host, config.Get().Wanderer.Slug)
	slog.Debug("fetching systems on map", "url", url)
	span.AddEvent("fetch systems", trace.WithAttributes(
		attribute.String("url", url),
	))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.Get().Wanderer.Token))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("%s/%s:%s", config.Get().AdminName, config.Get().AppName, config.Get().Version))

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	list := struct{ Data []System }{}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&list); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	tmpRegistry := make([]System, 0)

	slog.Debug("filtering systems",
		"wormholes_only", config.Get().OnlyWHKills,
		"ignored_systems", config.Get().IgnoreSystems,
	)

	for _, sys := range list.Data {

		if config.Get().OnlyWHKills && !isWH(sys) {
			slog.Debug("discarding system",
				"reason", "wormhole kills only is turned on",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		if common.Contains(config.Get().IgnoreSystems, sys.Name) {
			slog.Debug("discarding system",
				"reason", "system is on ignore list",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		tmpRegistry = append(tmpRegistry, sys)

	}

	newHash := listHash(tmpRegistry)
	changed := !bytes.Equal(origHash[:], newHash[:])

	if len(tmpRegistry) > 0 && changed {
		s.mx.Lock()
		s.systems = tmpRegistry
		s.mx.Unlock()
	}

	slog.Debug("fetch complete", "change", changed, "system_count", len(tmpRegistry))
	span.AddEvent("fetch complete", trace.WithAttributes(
		attribute.Bool("change", changed),
		attribute.Int("system_count", len(tmpRegistry)),
	))
	return changed, nil
}

func (s *SystemRegister) Fetch(ctx context.Context, out chan Killmail) error {
	sctx, span := otel.Tracer(packageName).Start(ctx, "Fetch")
	defer span.End()

	s.mx.Lock()
	systems := s.systems
	s.mx.Unlock()

	systemList := make([]string, len(systems))
	for i := range systems {
		systemList[i] = systems[i].String()
	}

	span.SetAttributes(
		attribute.StringSlice("systems", systemList),
	)
	kms, err := FetchKillmails(sctx, systems)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	for _, km := range kms {
		out <- km
	}

	return nil
}

func (s *SystemRegister) Start(outbox chan Killmail, refresh chan struct{}) {
	s.mx.Lock()
	systems := s.systems
	s.mx.Unlock()

	filters := buildFilters(systems)

	slog.Debug("subscribing to channel", "channel", filters)

	if err := s.ws.WriteJSON(filters); err != nil {
		s.errors <- err
	}

	go func() {
		for {
			var msg Killmail
			if err := s.ws.ReadJSON(&msg); err != nil {
				if !errors.Is(err, &websocket.CloseError{}) {
					slog.Error("failed to receive message", "error", err)
				}
				return
			}

			cache, err := Cache()
			if err != nil {
				slog.Error("failed to get Cache instance", "error", err)
				return
			}

			if exists, _ := cache.Exists(fmt.Sprintf("%d", msg.KillmailID)); exists {
				continue
			} else {
				if err := cache.AddItem(fmt.Sprintf("%d", msg.KillmailID)); err != nil {
					slog.Error("failed to add item to cache", "id", msg.KillmailID, "error", err)
				}
			}

			outbox <- msg
		}
	}()

	go func() {
		for range refresh {
			filters := buildFilters(systems)

			slog.Debug("subscribing to channel", "channel", filters)

			if err := s.ws.WriteJSON(filters); err != nil {
				s.errors <- err
			}
		}
	}()
}

func (s *SystemRegister) Stop() error {
	if err := s.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		return err
	}

	slog.Debug("websocket listener stopped")
	close(s.stop)

	return nil
}

func buildFilters(systems []System) map[string]string {
	channel := "killstream"

	if len(systems) > 0 {
		systemNames := make([]string, 0)
		for _, sys := range systems {
			systemNames = append(systemNames, fmt.Sprintf("%d", sys.SolarSystemID))
		}

		channel = fmt.Sprintf("system:%s", strings.Join(systemNames, ","))
	}

	return map[string]string{
		"action":  "sub",
		"channel": channel,
	}
}

func isWH(sys System) bool {
	return whPattern.MatchString(sys.Name)
}
