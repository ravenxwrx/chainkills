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
	Verbose           bool     `yaml:"verbose"`
	OnlyWHKills       bool     `yaml:"only_wh_kills"`
	RefreshInterval   int      `yaml:"refresh_interval"`
	AdminName         string   `yaml:"admin_name"`
	AdminEmail        string   `yaml:"admin_email"`
	AppName           string   `yaml:"app_name"`
	Version           string   `yaml:"version"`
	FetchTimeFrame    int      `yaml:"fetch_timeframe"`
	IgnoreSystemNames []string `yaml:"ignore_system_names"`
	IgnoreSystemIDs   []int    `yaml:"ignore_system_ids"`
	IgnoreRegionIDs   []int    `yaml:"ignore_region_ids"`
	Redict            Redict   `yaml:"redict"`
	Wanderer          Wanderer `yaml:"wanderer"`
	Discord           Discord  `yaml:"discord"`
	Friends           Friends  `yaml:"friends"`
}

type Redict struct {
	Cache    bool
	Database int    `yaml:"database"`
	TTL      int    `yaml:"ttl"` // Time to live for keys in minutes
	Address  string `yaml:"address"`
}

type Wanderer struct {
	Token string `yaml:"token"`
	Slug  string `yaml:"slug"`
	Host  string `yaml:"host"`
}

type Discord struct {
	DryRun   bool `yaml:"dry_run"`
	Verbose  bool
	Token    string
	Channels []string
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

	// Create config instance with some default values
	cfg := Cfg{
		RefreshInterval: 60,
		FetchTimeFrame:  1,
		Redict: Redict{
			TTL: 1440, // 24 hours
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
