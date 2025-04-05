package backend

import (
	"context"

	"git.sr.ht/~barveyhirdman/chainkills/backend/memory"
	"git.sr.ht/~barveyhirdman/chainkills/backend/model"
	"git.sr.ht/~barveyhirdman/chainkills/backend/redict"
	"git.sr.ht/~barveyhirdman/chainkills/config"
)

var backend Engine

const engine = "redict"

type Engine interface {
	AddKillmail(ctx context.Context, id string) error
	KillmailExists(ctx context.Context, id string) (bool, error)
	GetIgnoredSystemIDs(ctx context.Context) ([]string, error)
	GetIgnoredSystemNames(ctx context.Context) ([]string, error)
	GetIgnoredRegionIDs(ctx context.Context) ([]string, error)
	IgnoreSystemID(ctx context.Context, id int64) error
	IgnoreSystemName(ctx context.Context, name string) error
	IgnoreRegionID(ctx context.Context, id int64) error
	RegisterChannel(ctx context.Context, guildID string, channelID string) error
	GetRegisteredChannels(ctx context.Context) ([]model.Channel, error)
}

func Backend() (Engine, error) {
	var err error
	if backend == nil {
		switch engine {
		case "memory":
			backend, err = memory.New()
		case "redict":
			fallthrough
		default:
			backend, err = redict.New(config.Get().Redict.Address)
		}
	}

	return backend, err
}
