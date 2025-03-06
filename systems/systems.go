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
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"github.com/julianshen/og"
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
		URL  string `json:"url"`
		Hash string `json:"hash"`
		NPC  bool   `json:"npc"`
	} `json:"zkb"`
}

func (k *Killmail) Color() int {
	if config.Get().IsFriend(k.Victim.AllianceID, k.Victim.CorporationID, k.Victim.CharacterID) {
		return ColorOurLoss
	}

	for _, attacker := range k.Attackers {
		if config.Get().IsFriend(attacker.AllianceID, attacker.CorporationID, attacker.CharacterID) {
			return ColorOurKill
		}
	}

	return ColorWhatever
}

func (k *Killmail) Embed() (*discordgo.MessageEmbed, error) {
	url := k.Zkill.URL
	slog.Debug("preparing embed", "url", url)

	siteData, err := og.GetPageInfoFromUrl(url)
	if err != nil {
		return nil, err
	}

	embed := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeLink,
		URL:         url,
		Description: siteData.Description,
		Title:       siteData.Title,
		Color:       k.Color(),
		Provider: &discordgo.MessageEmbedProvider{
			URL:  "https://zkillboard.com",
			Name: siteData.SiteName,
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:    siteData.Images[0].Url,
			Width:  int(siteData.Images[0].Width),
			Height: int(siteData.Images[0].Height),
		},
	}

	slog.Info("prepared embed", "embed", embed)
	return embed, nil
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

func (s *SystemRegister) Update() (bool, error) {
	client := http.Client{}

	origHash := listHash(s.systems)

	url := fmt.Sprintf("%s/api/map/systems?slug=%s", config.Get().Wanderer.Host, config.Get().Wanderer.Slug)
	slog.Debug("fetching systems on map", "url", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.Get().Wanderer.Token))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("%s/%s:%s", config.Get().AdminName, config.Get().AppName, config.Get().Version))

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

	return changed, nil
}

func (s *SystemRegister) Fetch(out chan Killmail) error {
	s.mx.Lock()
	systems := s.systems
	s.mx.Unlock()

	kms, err := FetchKillmails(systems)
	if err != nil {
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
