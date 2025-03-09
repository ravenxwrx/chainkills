package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
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

	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	l := slog.New(h)
	slog.SetDefault(l)

	err := config.Read(configPath)
	if err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

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

	session.LogLevel = discordgo.LogInformational
	session.ShouldReconnectOnError = true
	session.ShouldRetryOnRateLimit = true

	if err := session.Open(); err != nil {
		slog.Error("failed to open discord session", "error", err)
		os.Exit(1)
	}

	printMemStats()

	register := systems.Register()
	if _, err := register.Update(rootCtx); err != nil {
		slog.Error("failed to update systems", "error", err)
		os.Exit(1)
	}

	out := make(chan systems.Killmail)

	tickerDuration := time.Duration(config.Get().RefreshInterval) * time.Second
	slog.Debug("starting ticker", "interval", tickerDuration.String())
	tick := time.NewTicker(tickerDuration)
	memTick := time.NewTicker(1 * time.Minute)

	go func() {
		for range tick.C {
			_, err := register.Update(rootCtx)
			if err != nil {
				slog.Error("failed to update systems", "error", err)
			}

			if err := register.Fetch(rootCtx, out); err != nil {
				slog.Error("failed to fetch killmails")
			}

			printMemStats()
		}
	}()

	go func() {
		for range memTick.C {
			printMemStats()
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
			if _, err := session.ChannelMessageSendEmbed(config.Get().Discord.Channel, embed); err != nil {
				slog.Error("failed to send message", "error", err)
				return
			}

			common.GetBackpressureMonitor().Decrease("killmail")

			printMemStats()
		}
	}()

	if err := register.Fetch(rootCtx, out); err != nil {
		slog.Error("failed to fetch killmails")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	tick.Stop()
	memTick.Stop()
	session.Close()
	close(out)
	slog.Info("exiting")
}

func printMemStats() {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)

	slog.Debug("memory stats",
		"sys", fmt.Sprintf("%f.3", float64(stats.Sys)/1024/1024),
		"heap", fmt.Sprintf("%f.3", float64(stats.HeapSys)/1024/1024),
		"stack", fmt.Sprintf("%f.3", float64(stats.StackSys)/1024/1024),
		"goroutines", runtime.NumGoroutine(),
	)
}
