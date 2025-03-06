package systems

import (
	"sync"
	"time"
)

var (
	duplicateCache *KillmailCache
	ttl            time.Duration = 2 * time.Hour
)

type KillmailCache struct {
	mx *sync.Mutex

	count uint64
	items map[string]time.Time
}

func newCache() *KillmailCache {
	return &KillmailCache{
		mx: &sync.Mutex{},

		count: 0,
		items: make(map[string]time.Time),
	}
}

func Cache() *KillmailCache {
	if duplicateCache == nil {
		duplicateCache = newCache()
	}

	return duplicateCache
}

func (c *KillmailCache) AddItem(id string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if _, ok := c.items[id]; ok {
		return
	}

	c.items[id] = time.Now()
	c.count += 1

	c.evict()
}

func (c *KillmailCache) Exists(id string) bool {
	if _, ok := c.items[id]; ok {
		return true
	}

	return false
}

func (c *KillmailCache) evict() {
	for k, added := range c.items {
		if added.Before(time.Now().Add(-1 * ttl)) {
			delete(c.items, k)
		}
	}
}
