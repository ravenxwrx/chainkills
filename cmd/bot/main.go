package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/common"
	"git.sr.ht/~barveyhirdman/chainkills/config"
	"git.sr.ht/~barveyhirdman/chainkills/discord"
	"git.sr.ht/~barveyhirdman/chainkills/instrumentation"
	"git.sr.ht/~barveyhirdman/chainkills/systems"
	"github.com/bwmarrin/discordgo"
)

var (
	configPath string
)

func main() {
	flag.StringVar(&configPath, "config", "config.yaml", "Path to config")
	flag.Parse()

	rootCtx := context.Background()

	err := config.Read(configPath)
	if err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if config.Get().Verbose {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	l := slog.New(h).With("version", config.Get().Version)
	slog.SetDefault(l)

	shutdownFns, err := instrumentation.Init(rootCtx)
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownFns.Shutdown(rootCtx); err != nil {
			slog.Error("failed to shut down tracer cleanly", "error", err)
		}
	}()

	discord.Init()
	session, err := discordgo.New("Bot " + config.Get().Discord.Token)
	if err != nil {
		slog.Error("failed to create discord session", "error", err)
		os.Exit(1)
	}

	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	registeredCommands := make(map[string]*discordgo.ApplicationCommand, 0)
	registeredHandlers := make(map[string]func(), 0)

	registeredHandlers["HandleGuildCreate"] = session.AddHandler(discord.HandleGuildCreate)
	registeredHandlers["HandleGuildDelete"] = session.AddHandler(discord.HandleGuildDelete)
	registeredHandlers["HandleSlasCommand"] = session.AddHandler(discord.HandleSlashCommand)

	commands := []*discordgo.ApplicationCommand{
		discord.IgnoreSystemIDCommand,
		discord.IgnoreSystemNameCommand,
		discord.IgnoreRegionIDCommand,
	}
	cmdWg := &sync.WaitGroup{}
	session.AddHandler(func(s *discordgo.Session, m *discordgo.Ready) {
		for _, cmd := range commands {
			cmdWg.Add(1)

			go func(c *discordgo.ApplicationCommand) {
				defer cmdWg.Done()

				if _, err := s.ApplicationCommandCreate(s.State.User.ID, "", c); err != nil {
					slog.Error("failed to register command", "command", cmd.Name, "error", err)
					return
				}

				opts := make([]string, 0, len(c.Options))
				for _, opt := range c.Options {
					opts = append(opts, fmt.Sprintf("%s - %s", opt.Name, opt.Type.String()))
				}
				slog.Info("registered command", "command", c.Name, "options", strings.Join(opts, ", "))
				registeredCommands[c.Name] = c
			}(cmd)
		}
	})

	defer func() {
		if session.State != nil && session.State.User != nil {
			for _, v := range registeredHandlers {
				v()
			}

			for _, v := range registeredCommands {
				err := session.ApplicationCommandDelete(session.State.User.ID, "", v.ID)
				if err != nil {
					slog.Error("failed to delete command", "comand", v.Name, "error", err)
				}
			}
		}

		session.Close()
	}()

	if config.Get().Discord.Verbose {
		session.LogLevel = discordgo.LogDebug
	}

	if err := session.Open(); err != nil {
		slog.Error("failed to open discord session", "error", err)
		os.Exit(1)
	}

	register := systems.Register()
	if _, err := register.Update(rootCtx); err != nil {
		slog.Error("failed to update systems", "error", err)
		os.Exit(1)
	}

	// wait for commands to be registered
	slog.Info("waiting for commands to be registered")
	cmdWg.Wait()

	out := make(chan systems.Killmail)

	tickerDuration := time.Duration(config.Get().RefreshInterval) * time.Second
	slog.Debug("starting ticker", "interval", tickerDuration.String())
	tick := time.NewTicker(tickerDuration)

	errors := make(chan error)
	go func() {
		for {
			select {
			case <-tick.C:
				_, err := register.Update(rootCtx)
				if err != nil {
					slog.Error("failed to update systems", "error", err)
				}
			case e := <-errors:
				slog.Error("error received", "error", e)
			}
		}
	}()

	stop := make(chan struct{})
	fetchLoop := true
	go func() {
		retries := 0
		for {
			if err := systems.StartListener(out, stop, errors); err != nil {
				slog.Error("failed to start listener", "error", err)
			}
			if !fetchLoop {
				return
			}
			dur := 5 * time.Second
			slog.Warn("listener failed", "retries", retries, "sleep", dur.String())
			time.Sleep(dur)
		}
	}()

	go func() {
		for msg := range out {
			if msg.KillmailID == 0 {
				continue
			}

			channels := config.Get().Discord.Channels

			validChannels := make([]string, 0)
			for _, c := range channels {
				if _, err := session.State.Channel(c); err == nil {
					validChannels = append(validChannels, c)
				} else {
					slog.Warn("channel not found", "channel", c)
				}
			}
			if config.Get().Discord.DryRun {
				slog.Warn("dry run enabled, not sending message",
					"message", msg,
					"channels", validChannels,
				)
				continue
			}

			embed, err := msg.Embed()
			if err != nil {
				slog.Error("failed to prepare embed", "error", err)
				return
			}
			cwg := &sync.WaitGroup{}
			common.GetBackpressureMonitor().Increase("channel_send")
			for _, channel := range validChannels {
				cwg.Add(1)
				go func(ccc string) {
					defer func() {
						common.GetBackpressureMonitor().Decrease("channel_send")
						cwg.Done()
					}()
					if _, err := session.ChannelMessageSendEmbed(ccc, embed); err != nil {
						slog.Error("failed to send message", "error", err)
						return
					}
				}(channel)
			}
			cwg.Wait()

			common.GetBackpressureMonitor().Decrease("killmail")
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	fetchLoop = false
	close(stop)
	tick.Stop()
	close(out)
	slog.Info("exiting")
}
