package systems

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"sync"

	"git.sr.ht/~barveyhirdman/chainkills/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func FetchKillmails(ctx context.Context, systems []System) (map[string]Killmail, error) {
	sctx, span := otel.Tracer(packageName).Start(ctx, "FetchKillmails")
	defer span.End()

	mx := &sync.Mutex{}
	killmails := make(map[string]Killmail)

	var outerError error

	for _, system := range systems {
		kms, err := FetchSystemKillmails(sctx, fmt.Sprintf("%d", system.SolarSystemID))
		if err != nil {
			slog.Error("failed to fetch system killmails", "system", system.SolarSystemID, "error", err)
			outerError = errors.Join(outerError, err)
			continue
		}

		mx.Lock()
		maps.Copy(killmails, kms)
		mx.Unlock()
	}

	slog.Debug("finished fetching killmails in the chain", "count", len(killmails))
	span.AddEvent("finished fetching killmails in the chain",
		trace.WithAttributes(
			attribute.Int("count", len(killmails)),
		),
	)
	return killmails, outerError
}

func FetchSystemKillmails(ctx context.Context, systemID string) (map[string]Killmail, error) {
	sctx, span := otel.Tracer(packageName).Start(ctx, "FetchSystemKillmails")
	defer span.End()

	url := fmt.Sprintf("https://zkillboard.com/api/systemID/%s/pastSeconds/10800/", systemID)
	slog.Debug("fetching killmails", "system", systemID, "url", url)
	span.AddEvent("fetching killmails for system", trace.WithAttributes(
		attribute.String("system", systemID),
		attribute.String("url", url),
	))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s:%s %s", config.Get().AdminName, config.Get().AppName, config.Get().Version, config.Get().AdminEmail))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)

	var killmails []Killmail
	if err := decoder.Decode(&killmails); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		resp.Body.Close()
		return nil, err
	}
	resp.Body.Close()

	cache, err := Cache()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	kms := make(map[string]Killmail)

	for i := range killmails {
		if killmails[i].Zkill.NPC {
			continue
		}

		km := killmails[i]
		id := fmt.Sprintf("%d", km.KillmailID)

		exists, err := cache.Exists(id)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.Error("failed to check id in cache", "error", err)
			continue
		} else if exists {
			span.AddEvent("cache hit", trace.WithAttributes(attribute.String("id", id)))
			slog.Debug("key already exists in cache", "id", id)
			continue
		}

		km.Zkill.URL = fmt.Sprintf("https://zkillboard.com/kill/%d/", km.KillmailID)

		esiKM, err := GetEsiKillmail(sctx, km.KillmailID, km.Zkill.Hash)
		if err != nil {
			return nil, err
		}

		for _, attacker := range esiKM.Attackers {
			if attacker.AllianceID+attacker.CharacterID+attacker.CharacterID == 0 {
				continue
			}

			km.Attackers = append(km.Attackers, attacker)
		}

		km.Victim = esiKM.Victim

		kms[id] = km

		if err := cache.AddItem(id); err != nil {
			span.RecordError(err)
			slog.Error("failed to add item to cache", "id", id, "error", err)
		}
	}

	span.AddEvent("finished fetching killmails in system", trace.WithAttributes(
		attribute.String("system", systemID),
		attribute.Int("count", len(kms)),
	))
	slog.Debug("finished fetching killmails in system", "id", systemID, "count", len(kms))

	return kms, nil
}

func GetEsiKillmail(ctx context.Context, id uint64, hash string) (Killmail, error) {
	_, span := otel.Tracer(packageName).Start(ctx, "FetchSystemKillmails")
	defer span.End()

	url := fmt.Sprintf("https://esi.evetech.net/latest/killmails/%d/%s/?datasource=tranquility", id, hash)
	slog.Debug("fetching killmail", "id", id, "hash", hash, "url", url)
	span.AddEvent("fetching killmail", trace.WithAttributes(
		attribute.Int64("killmail_id", int64(id)),
		attribute.String("killmail_hash", hash),
		attribute.String("url", url),
	))

	resp, err := http.Get(url)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return Killmail{}, err
	}

	var km Killmail
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&km); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		resp.Body.Close()
		return Killmail{}, err
	}
	resp.Body.Close()

	return km, nil
}
