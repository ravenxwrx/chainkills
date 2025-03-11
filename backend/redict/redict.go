package redict

import (
	"context"
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

	if err := r.redict.Set(context.Background(), id, "", time.Duration(config.Get().Redict.TTL)*time.Minute).Err(); err != nil {
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

	_, err := r.redict.Get(context.Background(), id).Result()
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

	ids, err := r.redict.SMembers(context.Background(), keyIgnoredSystemIDs).Result()
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

	ids, err := r.redict.SMembers(context.Background(), keyIgnoredSystemNames).Result()
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

	ids, err := r.redict.SMembers(context.Background(), keyIgnoredRegionIDs).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "ok")
	return ids, nil
}
