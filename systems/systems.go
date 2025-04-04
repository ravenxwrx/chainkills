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
	"strconv"
	"sync"

	"git.sr.ht/~barveyhirdman/chainkills/backend"
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
		if err := resp.Body.Close(); err != nil {
			logger.Error("failed to close response body", "error", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return false, err
	}
	if err := resp.Body.Close(); err != nil {
		logger.Error("failed to close response body", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	tmpRegistry := make([]System, 0)

	logger.Info("filtering systems",
		"wormholes_only", config.Get().OnlyWHKills,
		"ignored_system_names", ignoredSystemNames(),
		"ignored_system_ids", ignoredSystemIDs(),
		"ignored_region_ids", ignoredRegionIDs(),
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

		if common.ContainsKey(ignoredSystemNames(), sys.Name) {
			logger.Debug("discarding system",
				"reason", "system is on ignore list",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		if common.ContainsKey(ignoredSystemIDs(), sys.SolarSystemID) {
			logger.Debug("discarding system",
				"reason", "system is on ignore list",
				"system_name", sys.Name,
				"system_id", sys.SolarSystemID,
			)
			continue
		}

		systemData, ok := GetSystem(sys.SolarSystemID)
		if ok {
			if common.ContainsKey(ignoredRegionIDs(), systemData.RegionID) {
				logger.Debug("discarding system",
					"reason", "region is on ignore list",
					"system_name", sys.Name,
					"system_id", sys.SolarSystemID,
					"region_id", systemData.RegionID,
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

func isWH(sys System) bool {
	return whPattern.MatchString(sys.Name)
}

// ignoredSystemIDs returns a map of system IDs that should be ignored
// from the config and the backend both by name and ID
func ignoredSystemIDs() map[int]struct{} {
	ids := make(map[int]struct{}, 0)

	for _, sys := range config.Get().IgnoreSystemIDs {
		ids[sys] = struct{}{}
	}

	if b, err := backend.Backend(); err == nil {
		if idsFromBackend, err := b.GetIgnoredSystemIDs(context.Background()); err == nil {
			for _, idStr := range idsFromBackend {
				if id, err := strconv.ParseInt(idStr, 10, 0); err == nil {
					ids[int(id)] = struct{}{}
					continue
				}

				slog.Warn("failed to parse ignored system id", "id", idStr)
			}
		} else {
			slog.Warn("failed to get ignored system ids from backend", "error", err)
		}
	} else {
		slog.Warn("failed to get backend", "error", err)
	}

	return ids
}

func ignoredSystemNames() map[string]struct{} {
	names := make(map[string]struct{}, 0)

	for _, sys := range config.Get().IgnoreSystemNames {
		names[sys] = struct{}{}
	}

	if b, err := backend.Backend(); err == nil {
		if namesFromBackend, err := b.GetIgnoredSystemNames(context.Background()); err == nil {
			for _, name := range namesFromBackend {
				names[name] = struct{}{}
			}
		} else {
			slog.Warn("failed to get ignored system names from backend", "error", err)
		}
	} else {
		slog.Warn("failed to get backend", "error", err)
	}

	return names
}

func ignoredRegionIDs() map[int]struct{} {
	ids := make(map[int]struct{}, 0)

	for _, sys := range config.Get().IgnoreRegionIDs {
		ids[sys] = struct{}{}
	}

	if b, err := backend.Backend(); err == nil {
		if idsFromBackend, err := b.GetIgnoredRegionIDs(context.Background()); err == nil {
			for _, idStr := range idsFromBackend {
				if id, err := strconv.ParseInt(idStr, 10, 0); err == nil {
					ids[int(id)] = struct{}{}
					continue
				}

				slog.Warn("failed to parse ignored system id", "id", idStr)
			}
		} else {
			slog.Warn("failed to get ignored region ids from backend", "error", err)
		}
	} else {
		slog.Warn("failed to get backend", "error", err)
	}

	return ids
}
