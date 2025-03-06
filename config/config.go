package config

import (
	"os"

	"git.sr.ht/~barveyhirdman/chainkills/common"
	"gopkg.in/yaml.v3"
)

type Cfg struct {
	AdminName     string   `json:"admin_name"`
	AppName       string   `json:"app_name"`
	Version       string   `json:"version"`
	OnlyWHKills   bool     `json:"only_wh_kills"`
	IgnoreSystems []string `json:"ignore_systems"`
	Wanderer      Wanderer `json:"wanderer"`
	Discord       Discord  `json:"discord"`
	Friends       Friends  `json:"friends"`
}

type Wanderer struct {
	Token string `json:"token"`
	Slug  string `json:"slug"`
	Host  string `json:"host"`
}

type Discord struct {
	Token   string
	Channel string
}

type Friends struct {
	Alliances    []uint64
	Corporations []uint64
	Characters   []uint64
}

func (c *Cfg) IsFriend(allianceID, corpID, CharacterID uint64) bool {
	return common.Contains(c.Friends.Alliances, allianceID) ||
		common.Contains(c.Friends.Corporations, corpID) ||
		common.Contains(c.Friends.Characters, CharacterID)
}

func Read(path string) (*Cfg, error) {
	fp, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	cfg := Cfg{}

	if err := yaml.NewDecoder(fp).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
