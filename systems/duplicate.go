package systems

import (
	"context"
	"sync"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/config"
	"github.com/redis/go-redis/v9"
)

const (
	engine = "redict"
)

var (
	duplicateCache CacheEngine
	ttl            time.Duration = 2 * time.Hour
)

type CacheEngine interface {
	AddItem(id string) error
	Exists(id string) (bool, error)
}

type MemoryCache struct {
	mx *sync.Mutex

	count uint64
	items map[string]time.Time
}

func newMemoryCache() (*MemoryCache, error) {
	return &MemoryCache{
		mx: &sync.Mutex{},

		count: 0,
		items: make(map[string]time.Time),
	}, nil
}

func Cache() (CacheEngine, error) {
	var err error
	if duplicateCache == nil {
		switch engine {
		case "memory":
			duplicateCache, err = newMemoryCache()
		case "redict":
			fallthrough
		default:
			duplicateCache, err = newRedictCache("redis://127.0.0.1:6379")
		}
	}

	return duplicateCache, err
}

func (c *MemoryCache) AddItem(id string) error {
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

func (c *MemoryCache) Exists(id string) (bool, error) {
	if _, ok := c.items[id]; ok {
		return true, nil
	}

	return false, nil
}

func (c *MemoryCache) evict() {
	for k, added := range c.items {
		if added.Before(time.Now().Add(-1 * ttl)) {
			delete(c.items, k)
		}
	}
}

type RedictCache struct {
	redict *redis.Client
}

func newRedictCache(url string) (*RedictCache, error) {
	redict := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	return &RedictCache{
		redict: redict,
	}, nil
}

func (r *RedictCache) AddItem(id string) error {
	if err := r.redict.Set(context.Background(), id, "", time.Duration(config.Get().Redict.TTL)*time.Minute).Err(); err != nil {
		return err
	}

	return nil
}

func (r *RedictCache) Exists(id string) (bool, error) {
	_, err := r.redict.Get(context.Background(), id).Result()
	if err == nil {
		return true, nil
	} else if err == redis.Nil {
		return false, nil
	}

	return false, err
}
