package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var packageName = "git.sr.ht/~barveyhirdman/chainkills/discord"

var RegisterChannelCommand = &discordgo.ApplicationCommand{
	Name:        "register-channel",
	Description: "Register this channel for notifications",
}

var IgnoreSystemIDCommand = &discordgo.ApplicationCommand{
	Name:        "ignore-system-id",
	Description: "Ignore a system by ID",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "system_id",
			Description: "The ID of the system to ignore",
			Required:    true,
		},
	},
}

var IgnoreSystemNameCommand = &discordgo.ApplicationCommand{
	Name:        "ignore-system-name",
	Description: "Ignore a system by name",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "system_name",
			Description: "The name of the system to ignore",
			Required:    true,
		},
	},
}

var IgnoreRegionIDCommand = &discordgo.ApplicationCommand{
	Name:        "ignore-region-id",
	Description: "Ignore a region by ID",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "region_id",
			Description: "The ID of the region to ignore",
			Required:    true,
		},
	},
}

func HandleRegisterChannel(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	_, span := otel.Tracer(packageName).Start(context.Background(), "HandleIgnoreSystemID")
	defer span.End()

	guild, err := getGuild(span, s, i.GuildID)
	if err != nil {
		return
	}

	channel, err := getChannel(span, s, i.ChannelID)
	if err != nil {
		return
	}

	backend, err := getBackend(span)
	if err != nil {
		return
	}

	if err := backend.RegisterChannel(ctx, guild.ID, channel.ID); err != nil {
		slog.Error("failed to register channel", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return
	}

	slog.Info("registered channel", "guild", guild.Name, "channel", channel.Name)

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf(
				"Channel %s registered for notifications",
				channel.Name,
			),
		},
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		slog.Error("failed to respond to interaction", "error", err)
	}

	span.SetStatus(codes.Ok, "ok")
}

func HandleIgnoreSystemID(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	sctx, span := otel.Tracer(packageName).Start(context.Background(), "HandleIgnoreSystemID")
	defer span.End()

	backend, err := getBackend(span)
	if err != nil {
		return
	}

	if err := backend.IgnoreSystemID(sctx, i.ApplicationCommandData().Options[0].IntValue()); err != nil {
		slog.Error("failed to add ignored system id", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return
	}

	systemID := i.ApplicationCommandData().Options[0].IntValue()

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf(
				"System ID %d has been ignored",
				systemID,
			),
		},
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		slog.Error("failed to respond to interaction", "error", err)
	}

	span.SetStatus(codes.Ok, "ok")
}

func HandleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, span := otel.Tracer(packageName).Start(context.Background(), "HandleSlashCommand")
	defer span.End()

	// Check if the command is a slash command
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch i.ApplicationCommandData().Name {
	case "ignore-system-id":
		HandleIgnoreSystemID(ctx, s, i)
	case "ignore-system-name":
		HandleIgnoreSystemName(ctx, s, i)
	case "ignore-region-id":
		HandleIgnoreRegionID(ctx, s, i)
	case "register-channel":
		HandleRegisterChannel(ctx, s, i)
	}
}

func HandleIgnoreSystemName(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	sctx, span := otel.Tracer(packageName).Start(context.Background(), "HandleIgnoreSystemName")
	defer span.End()

	backend, err := getBackend(span)
	if err != nil {
		return
	}

	systemName := i.ApplicationCommandData().Options[0].StringValue()

	if err := backend.IgnoreSystemName(sctx, systemName); err != nil {
		slog.Error("failed to add ignored system name", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf(
				"System name %s has been ignored",
				systemName,
			),
		},
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		slog.Error("failed to respond to interaction", "error", err)
	}

	span.SetStatus(codes.Ok, "ok")
}

func HandleIgnoreRegionID(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	sctx, span := otel.Tracer(packageName).Start(context.Background(), "HandleIgnoreRegionID")
	defer span.End()

	backend, err := getBackend(span)
	if err != nil {
		return
	}

	if err := backend.IgnoreSystemID(sctx, i.ApplicationCommandData().Options[0].IntValue()); err != nil {
		slog.Error("failed to add ignored system id", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return
	}

	systemID := i.ApplicationCommandData().Options[0].IntValue()

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf(
				"System ID %d has been ignored",
				systemID,
			),
		},
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		slog.Error("failed to respond to interaction", "error", err)
	}

	span.SetStatus(codes.Ok, "ok")
}
