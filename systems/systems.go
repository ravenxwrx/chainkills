package systems

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
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

	logger := slog.Default().With(
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
	)

	client := http.Client{}

	origHash := listHash(s.systems)

	url := fmt.Sprintf("%s/api/map/systems?slug=%s", config.Get().Wanderer.Host, config.Get().Wanderer.Slug)
	logger.Info("fetching systems on map", "url", url)
	span.AddEvent("fetch systems", trace.WithAttributes(
		attribute.String("url", url),
	))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error("failed to create request", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.Get().Wanderer.Token))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("%s/%s:%s", config.Get().AdminName, config.Get().AppName, config.Get().Version))

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to fetch systems", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	list := struct{ Data []System }{}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&list); err != nil {
		logger.Error("failed to decode systems", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		resp.Body.Close()
		return false, err
	}
	resp.Body.Close()

	tmpRegistry := make([]System, 0)

	logger.Info("filtering systems",
		"wormholes_only", config.Get().OnlyWHKills,
		"ignored_system_names", config.Get().IgnoreSystemNames,
		"ignored_system_ids", config.Get().IgnoreSystemIDs,
		"ignored_region_ids", config.Get().IgnoreRegionIDs,
	)

	for _, sys := range list.Data {
		if config.Get().OnlyWHKills && !isWH(sys) {
			logger.Debug("discarding system",
				"reason", "wormhole kills only is turned on",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		if common.Contains(config.Get().IgnoreSystemNames, sys.Name) {
			logger.Debug("discarding system",
				"reason", "system is on ignore list",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		if common.Contains(config.Get().IgnoreSystemIDs, sys.SolarSystemID) {
			logger.Debug("discarding system",
				"reason", "system is on ignore list",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		systemData, ok := GetSystem(sys.SolarSystemID)
		if ok {
			if common.Contains(config.Get().IgnoreRegionIDs, systemData.RegionID) {
				logger.Debug("discarding system",
					"reason", "system is on ignore list",
					"system_name", sys.Name,
					"system_id", sys.SolarSystemID,
				)
				continue
			}
		} else {
			slog.Warn("failed to get system data", "system_id", sys.SolarSystemID)
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

	logger.Debug("fetch complete", "change", changed, "system_count", len(tmpRegistry))
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

	slog.Debug(
		"fetching killmails",
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
		"systems", s.systems,
	)

	kms, err := FetchKillmails(sctx, systems)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	for _, km := range kms {
		common.GetBackpressureMonitor().Increase("killmail")
		out <- km
	}

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
