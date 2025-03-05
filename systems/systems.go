package systems

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"git.sr.ht/~barveyhirdman/chainkills/config"
)

var (
	whPattern = regexp.MustCompile("J[0-9]{6}")
	register  *SystemRegister
)

type System struct {
	Name          string `json:"name"`
	SolarSystemID int    `json:"solar_system_id"`
}

type SystemRegister struct {
	mx     *sync.Mutex
	stop   chan struct{}
	errors chan error

	cfg      *config.Cfg
	systems  []System
	listener *Listener
}

type Option func(*SystemRegister)

func WithConfig(cfg *config.Cfg) Option {
	return func(s *SystemRegister) {
		s.cfg = cfg
	}
}

func Register(opts ...Option) *SystemRegister {
	if register == nil {
		register = &SystemRegister{
			mx:   &sync.Mutex{},
			stop: make(chan struct{}),

			systems: []System{
				// {Name: "Ahbazon", SolarSystemID: 30005196},
			},
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

func (s *SystemRegister) Update() (bool, error) {
	client := http.Client{}

	origHash := listHash(s.systems)

	url := fmt.Sprintf("%s/api/map/systems?slug=%s", s.cfg.Wanderer.Host, s.cfg.Wanderer.Slug)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.cfg.Wanderer.Token))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("%s/%s:%s", s.cfg.AdminName, s.cfg.AppName, s.cfg.Version))

	slog.Debug("sending request", "request", req)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}

	slog.Debug("received response", "response", resp)

	list := struct{ Data []System }{}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&list); err != nil {
		return false, err
	}

	tmpRegistry := make([]System, 0)
	for _, sys := range list.Data {

		if s.cfg.OnlyWHKills && !isWH(sys) {
			continue
		}

		if contains(s.cfg.IgnoreSystems, sys.Name) {
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

	return changed, nil
}

func (s *SystemRegister) Start(outbox chan Killmail) {
	s.mx.Lock()
	systems := s.systems
	s.mx.Unlock()

	listener, err := NewListener(systems)
	if err != nil {
		s.errors <- err
		close(s.errors)
		return
	}

	s.listener = listener

	done := make(chan struct{})
	go listener.Start(outbox, done, s.errors)
	<-done
}

func (s *SystemRegister) Stop() {
	close(s.listener.Stop)
	close(s.stop)
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

func contains[T string | int](haystack []T, needle T) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}

	return false
}
