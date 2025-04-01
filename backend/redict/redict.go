package redict

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/config"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var packageName string = "git.sr.ht/~barveyhirdman/chainkills/backend/redict"

const (
	spanAddKillmail           = "AddKillmail"
	spanKillmailExists        = "KillmailExists"
	spanGetIgnoredSystemIDs   = "GetIgnoredSystemIDs"
	spanGetIgnoredSystemNames = "GetIgnoredSystemNames"
	spanGetIgnoredRegionIDs   = "GetIgnoredRegionIDs"
	spanIgnoreSystemID        = "IgnoreSystemID"
	spanIgnoreSystemName      = "IgnoreSystemName"
	spanIgnoreRegionID        = "IgnoreRegionID"

	keyIgnoredSystemIDs   = "ignored_system_ids"
	keyIgnoredSystemNames = "ignored_system_names"
	keyIgnoredRegionIDs   = "ignored_region_ids"
)

type Backend struct {
	redict *redis.Client
}

func New(url string) (*Backend, error) {
	redict := redis.NewClient(&redis.Options{
		Addr: url,
		DB:   1,
	})

	return &Backend{
		redict: redict,
	}, nil
}

func (r *Backend) AddKillmail(ctx context.Context, id string) error {
	_, span := otel.Tracer(packageName).Start(ctx, spanAddKillmail)
	defer span.End()

	span.SetAttributes(attribute.String("id", id))

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, id)
	if err := r.redict.Set(context.Background(), key, "", time.Duration(config.Get().Redict.TTL)*time.Minute).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "ok")

	return nil
}

func (r *Backend) KillmailExists(ctx context.Context, id string) (bool, error) {
	_, span := otel.Tracer(packageName).Start(ctx, spanKillmailExists)
	defer span.End()

	span.SetAttributes(attribute.String("id", id))

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, id)
	_, err := r.redict.Get(context.Background(), key).Result()
	if err == nil {
		span.SetAttributes(attribute.String("cache", "hit"))
		slog.Debug("cache hit", "id", id)
		return true, nil
	} else if err == redis.Nil {
		span.SetAttributes(attribute.String("cache", "miss"))
		slog.Debug("cache miss", "id", id)
		return false, nil
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())

	return false, err
}

func (r *Backend) GetIgnoredSystemIDs(ctx context.Context) ([]string, error) {
	_, span := otel.Tracer(packageName).Start(ctx, spanGetIgnoredSystemIDs)
	defer span.End()

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, keyIgnoredSystemIDs)
	ids, err := r.redict.SMembers(context.Background(), key).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "ok")
	return ids, nil
}
func (r *Backend) GetIgnoredSystemNames(ctx context.Context) ([]string, error) {
	_, span := otel.Tracer(packageName).Start(ctx, spanGetIgnoredSystemNames)
	defer span.End()

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, keyIgnoredSystemNames)
	ids, err := r.redict.SMembers(context.Background(), key).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "ok")
	return ids, nil
}
func (r *Backend) GetIgnoredRegionIDs(ctx context.Context) ([]string, error) {
	_, span := otel.Tracer(packageName).Start(ctx, spanGetIgnoredRegionIDs)
	defer span.End()

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, keyIgnoredRegionIDs)
	ids, err := r.redict.SMembers(context.Background(), key).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "ok")
	return ids, nil
}

func (r *Backend) IgnoreSystemID(ctx context.Context, id int64) error {
	sctx, span := otel.Tracer(packageName).Start(ctx, spanIgnoreSystemID)
	defer span.End()

	span.SetAttributes(attribute.Int64("id", id))

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, keyIgnoredSystemIDs)
	if _, err := r.redict.SAdd(sctx, key, id).Result(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "ok")
	return nil
}

func (r *Backend) IgnoreSystemName(ctx context.Context, name string) error {
	sctx, span := otel.Tracer(packageName).Start(ctx, spanIgnoreSystemName)
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, keyIgnoredSystemNames)
	if _, err := r.redict.SAdd(sctx, key, name).Result(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "ok")
	return nil
}

func (r *Backend) IgnoreRegionID(ctx context.Context, id int64) error {
	sctx, span := otel.Tracer(packageName).Start(ctx, spanIgnoreRegionID)
	defer span.End()

	span.SetAttributes(attribute.Int64("id", id))

	key := fmt.Sprintf("%s:%s", config.Get().Redict.Prefix, keyIgnoredRegionIDs)
	if _, err := r.redict.SAdd(sctx, key, id).Result(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "ok")
	return nil
}
