package systems

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"git.sr.ht/~barveyhirdman/chainkills/config"
)

func FetchKillmails(systems []System) (map[string]Killmail, error) {
	killmails := make(map[string]Killmail)

	var outerError error

	wg := &sync.WaitGroup{}
	for _, system := range systems {
		wg.Add(1)

		go func() {
			defer wg.Done()
			kms, err := FetchSystemKillmails(fmt.Sprintf("%d", system.SolarSystemID))
			if err != nil {
				slog.Error("failed to fetch system killmails", "system", system.SolarSystemID, "error", err)
				outerError = err
			}

			for k, v := range kms {
				killmails[k] = v
			}
		}()
	}

	wg.Wait()
	return killmails, outerError
}

func FetchSystemKillmails(systemID string) (map[string]Killmail, error) {
	url := fmt.Sprintf("https://zkillboard.com/api/w-space/systemID/%s/pastSeconds/10800/", systemID)
	slog.Debug("fetching killmails", "system", systemID, "url", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s:%s %s", config.Get().AdminName, config.Get().AppName, config.Get().Version, config.Get().AdminEmail))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)

	var killmails []Killmail
	if err := decoder.Decode(&killmails); err != nil {
		return nil, err
	}

	cache, err := Cache()
	if err != nil {
		return nil, err
	}

	kms := make(map[string]Killmail)

	for i := range killmails {
		if killmails[i].Zkill.NPC {
			continue
		}

		km := killmails[i]
		id := fmt.Sprintf("%d", km.KillmailID)

		if exists, _ := cache.Exists(id); exists {
			continue
		}

		km.Zkill.URL = fmt.Sprintf("https://zkillboard.com/kill/%d/", km.KillmailID)

		esiKM, err := GetEsiKillmail(km.KillmailID, km.Zkill.Hash)
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
	}

	return kms, nil
}

func GetEsiKillmail(id uint64, hash string) (Killmail, error) {
	url := fmt.Sprintf("https://esi.evetech.net/latest/killmails/%d/%s/?datasource=tranquility", id, hash)
	slog.Debug("fetching killmail", "id", id, "hash", hash, "url", url)

	resp, err := http.Get(url)
	if err != nil {
		return Killmail{}, err
	}

	var km Killmail
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&km); err != nil {
		return Killmail{}, err
	}

	return km, nil
}
