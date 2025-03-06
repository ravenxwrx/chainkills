package systems

import (
	"bytes"
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
)

const (
	ColorOurKill  int = 0x93c47d
	ColorOurLoss  int = 0x990000
	ColorWhatever int = 0xbcbcbc
)

var (
	whPattern = regexp.MustCompile("J[0-9]{6}")
	register  *SystemRegister
)

type System struct {
	Name          string `json:"name"`
	SolarSystemID int    `json:"solar_system_id"`
}

type Killmail struct {
	KillmailID uint64 `json:"killmail_id"`
	Attackers  []struct {
		CharacterID   uint64 `json:"character_id"`
		CorporationID uint64 `json:"corporation_id"`
		AllianceID    uint64 `json:"alliance_id"`
	} `json:"attackers"`
	Victim struct {
		CharacterID   uint64 `json:"character_id"`
		CorporationID uint64 `json:"corporation_id"`
		AllianceID    uint64 `json:"alliance_id"`
	} `json:"victim"`
	Zkill struct {
		URL string `json:"url"`
	} `json:"zkb"`
}

func (k *Killmail) Color(cfg *config.Cfg) int {
	if cfg.IsFriend(k.Victim.AllianceID, k.Victim.CorporationID, k.Victim.CharacterID) {
		return ColorOurLoss
	}

	for _, attacker := range k.Attackers {
		if cfg.IsFriend(attacker.AllianceID, attacker.CorporationID, attacker.CharacterID) {
			return ColorOurKill
		}
	}

	return ColorWhatever
}

type SystemRegister struct {
	mx     *sync.Mutex
	stop   chan struct{}
	errors chan error

	ws      *websocket.Conn
	cfg     *config.Cfg
	systems []System
}

type Option func(*SystemRegister)

func WithConfig(cfg *config.Cfg) Option {
	return func(s *SystemRegister) {
		s.cfg = cfg
	}
}

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

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}

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

		if common.Contains(s.cfg.IgnoreSystems, sys.Name) {
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

			if Cache().Exists(fmt.Sprintf("%d", msg.KillmailID)) {
				continue
			} else {
				Cache().AddItem(fmt.Sprintf("%d", msg.KillmailID))
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
