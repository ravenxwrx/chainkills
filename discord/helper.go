package discord

import (
	"log/slog"

	"git.sr.ht/~barveyhirdman/chainkills/backend"
	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func getBackend(span trace.Span) (backend.Engine, error) {
	backend, err := backend.Backend()
	if err != nil {
		slog.Error("failed to get backend", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return backend, nil
}

func getGuild(span trace.Span, s *discordgo.Session, guildID string) (*discordgo.Guild, error) {
	guild, err := s.Guild(guildID)
	if err != nil {
		slog.Error("failed to get guild data", "id", guildID, "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}
	return guild, nil
}

func getChannel(span trace.Span, s *discordgo.Session, channelID string) (*discordgo.Channel, error) {
	channel, err := s.Channel(channelID)
	if err != nil {
		slog.Error("failed to get channel data", "id", channelID, "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}
	return channel, nil
}
