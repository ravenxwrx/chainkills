package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"git.sr.ht/~barveyhirdman/chainkills/common"
	"gopkg.in/yaml.v3"
)

var c *Cfg

type Cfg struct {
	AdminName       string   `yaml:"admin_name"`
	AdminEmail      string   `yaml:"admin_email"`
	AppName         string   `yaml:"app_name"`
	Version         string   `yaml:"version"`
	RefreshInterval int      `yaml:"refresh_interval"`
	OnlyWHKills     bool     `yaml:"only_wh_kills"`
	IgnoreSystems   []string `yaml:"ignore_systems"`
	Redict          Redict   `yaml:"redict"`
	Wanderer        Wanderer `yaml:"wanderer"`
	Discord         Discord  `yaml:"discord"`
	Friends         Friends  `yaml:"friends"`
}

type Redict struct {
	Address  string `yaml:"address"`
	Database int    `yaml:"database"`
	TTL      int    `yaml:"ttl"` // Time to live for keys in minutes
}

type Wanderer struct {
	Token string `yaml:"token"`
	Slug  string `yaml:"slug"`
	Host  string `yaml:"host"`
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

func Read(path string) error {
	if p, err := filepath.Abs(path); err != nil {
		slog.Warn("find to get absolute filepath", "error", err)
	} else {
		slog.Debug("opening config", "path", p)
	}

	fp, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}

	cfg := Cfg{
		RefreshInterval: 60, // Default value
		Redict: Redict{
			TTL: 60,
		},
	}

	if err := yaml.NewDecoder(fp).Decode(&cfg); err != nil {
		return err
	}

	c = &cfg

	return nil
}

func Get() *Cfg {
	return c
}
