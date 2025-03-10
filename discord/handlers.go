package discord

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

func HandleGuildCreate(s *discordgo.Session, m *discordgo.GuildCreate) {
	channels := map[string]Channel{}
	for _, channel := range m.Channels {
		c := Channel{
			ID:      channel.ID,
			Name:    channel.Name,
			GuildID: channel.GuildID,
			Kind:    channel.Type,
		}

		channels[c.ID] = c
	}

	guild := Guild{
		ID:       m.ID,
		Name:     m.Name,
		Channels: channels,
	}

	addGuild(guild)

	slog.Info("joined guild", "name", m.Name, "channels", channels)
}

func HandleGuildDelete(s *discordgo.Session, m *discordgo.GuildDelete) {
	removeGuild(m.ID)

	slog.Info("left guild", "id", m.ID)
}
