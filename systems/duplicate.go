package systems

import (
	"context"
	"sync"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/config"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	engine = "redict"
)

var (
	duplicateCache CacheEngine
	ttl            time.Duration = 2 * time.Hour
)

type CacheEngine interface {
	AddItem(ctx context.Context, id string) error
	Exists(ctx context.Context, id string) (bool, error)
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
			duplicateCache, err = newRedictCache("127.0.0.1:6379")
		}
	}

	return duplicateCache, err
}

func (c *MemoryCache) AddItem(_ context.Context, id string) error {
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

func (c *MemoryCache) Exists(_ context.Context, id string) (bool, error) {
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
		Addr: url,
		DB:   1,
	})

	return &RedictCache{
		redict: redict,
	}, nil
}

func (r *RedictCache) AddItem(ctx context.Context, id string) error {
	_, span := otel.Tracer("chainkills").Start(ctx, "AddItem")
	defer span.End()

	if err := r.redict.Set(context.Background(), id, "", time.Duration(config.Get().Redict.TTL)*time.Minute).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (r *RedictCache) Exists(ctx context.Context, id string) (bool, error) {
	_, span := otel.Tracer("chainkills").Start(ctx, "Exists")
	defer span.End()

	_, err := r.redict.Get(context.Background(), id).Result()
	switch err {
	case nil:
		span.SetAttributes(attribute.String("cache", "hit"))
		return true, nil
	case redis.Nil:
		span.SetAttributes(attribute.String("cache", "miss"))
		return false, nil
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())

	return false, err
}
