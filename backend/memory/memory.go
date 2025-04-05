package memory

import (
	"context"
	"sync"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/backend/model"
)

var ttl = 24 * time.Hour

type Backend struct {
	mx *sync.Mutex

	count uint64
	items map[string]time.Time
}

func New() (*Backend, error) {
	return &Backend{
		mx: &sync.Mutex{},

		count: 0,
		items: make(map[string]time.Time),
	}, nil
}

func (c *Backend) AddKillmail(_ context.Context, id string) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if _, ok := c.items[id]; ok {
		return nil
	}

	c.items[id] = time.Now()
	c.count += 1

	c.evict()
	return nil
}

func (c *Backend) KillmailExists(_ context.Context, id string) (bool, error) {
	if _, ok := c.items[id]; ok {
		return true, nil
	}

	return false, nil
}

func (c *Backend) evict() {
	for k, added := range c.items {
		if added.Before(time.Now().Add(-1 * ttl)) {
			delete(c.items, k)
		}
	}
}

func (c *Backend) GetIgnoredSystemIDs(ctx context.Context) ([]string, error) {
	return make([]string, 0), nil
}
func (c *Backend) GetIgnoredSystemNames(ctx context.Context) ([]string, error) {
	return make([]string, 0), nil
}
func (c *Backend) GetIgnoredRegionIDs(ctx context.Context) ([]string, error) {
	return make([]string, 0), nil
}
func (c *Backend) GetRegisteredChannels(ctx context.Context) ([]model.Channel, error) {
	return make([]model.Channel, 0), nil
}
func (c *Backend) IgnoreSystemID(ctx context.Context, id int64) error {
	return nil
}
func (c *Backend) IgnoreSystemName(ctx context.Context, name string) error {
	return nil
}
func (c *Backend) IgnoreRegionID(ctx context.Context, id int64) error {
	return nil
}
func (c *Backend) RegisterChannel(ctx context.Context, guildID string, channelID string) error {
	return nil
}
