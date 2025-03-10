package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/common"
	"git.sr.ht/~barveyhirdman/chainkills/config"
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
	l := slog.New(h)
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

	session, err := discordgo.New("Bot " + config.Get().Discord.Token)
	if err != nil {
		slog.Error("failed to create discord session", "error", err)
		os.Exit(1)
	}

	if config.Get().Verbose {
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

	out := make(chan systems.Killmail)

	tickerDuration := time.Duration(config.Get().RefreshInterval) * time.Second
	slog.Debug("starting ticker", "interval", tickerDuration.String())
	tick := time.NewTicker(tickerDuration)

	go func() {
		for range tick.C {
			_, err := register.Update(rootCtx)
			if err != nil {
				slog.Error("failed to update systems", "error", err)
			}

			if err := register.Fetch(rootCtx, out); err != nil {
				slog.Error("failed to fetch killmails")
			}
		}
	}()

	go func() {
		for msg := range out {
			if msg.KillmailID == 0 {
				continue
			}

			embed, err := msg.Embed()
			if err != nil {
				slog.Error("failed to prepare embed", "error", err)
				return
			}
			cwg := &sync.WaitGroup{}
			common.GetBackpressureMonitor().Increase("channel_send")
			for _, channel := range config.Get().Discord.Channels {
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

	if err := register.Fetch(rootCtx, out); err != nil {
		slog.Error("failed to fetch killmails")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	tick.Stop()
	session.Close()
	close(out)
	slog.Info("exiting")
}
