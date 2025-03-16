package systems

import (
	"log/slog"

	"git.sr.ht/~barveyhirdman/chainkills/common"
	"git.sr.ht/~barveyhirdman/chainkills/config"
	"github.com/bwmarrin/discordgo"
	"github.com/julianshen/og"
)

type Killmail struct {
	KillmailID        uint64          `json:"killmail_id"`
	Attackers         []CharacterInfo `json:"attackers"`
	Victim            CharacterInfo   `json:"victim"`
	OriginalTimestamp string          `json:"killmail_time"`
	Zkill             struct {
		URL  string `json:"url"`
		Hash string `json:"hash"`
		NPC  bool   `json:"npc"`
	} `json:"zkb"`
}

type CharacterInfo struct {
	CharacterID   uint64 `json:"character_id"`
	CorporationID uint64 `json:"corporation_id"`
	AllianceID    uint64 `json:"alliance_id"`
}

func (c CharacterInfo) IsFriend() bool {
	friends := config.Get().Friends

	if c.AllianceID > 0 && common.Contains(friends.Alliances, c.AllianceID) {
		return true
	}

	if c.CorporationID > 0 && common.Contains(friends.Corporations, c.CorporationID) {
		return true
	}

	if c.CharacterID > 0 && common.Contains(friends.Characters, c.CharacterID) {
		return true
	}

	return false
}

func (k *Killmail) Color() int {
	if config.Get().IsFriend(k.Victim.AllianceID, k.Victim.CorporationID, k.Victim.CharacterID) {
		return ColorOurLoss
	}

	for _, attacker := range k.Attackers {
		if config.Get().IsFriend(attacker.AllianceID, attacker.CorporationID, attacker.CharacterID) {
			return ColorOurKill
		}
	}

	return ColorWhatever
}

func (k *Killmail) Embed() (*discordgo.MessageEmbed, error) {
	url := k.Zkill.URL
	slog.Debug("preparing embed", "url", url)

	siteData, err := og.GetPageInfoFromUrl(url)
	if err != nil {
		return nil, err
	}

	embed := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeLink,
		URL:         url,
		Description: siteData.Description,
		Title:       siteData.Title,
		Color:       k.Color(),
		Provider: &discordgo.MessageEmbedProvider{
			URL:  "https://zkillboard.com",
			Name: siteData.SiteName,
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:    siteData.Images[0].Url,
			Width:  int(siteData.Images[0].Width),
			Height: int(siteData.Images[0].Height),
		},
	}

	slog.Info("prepared embed", "embed", embed)
	return embed, nil
}
