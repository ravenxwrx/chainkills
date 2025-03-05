package config

import (
	"os"

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
