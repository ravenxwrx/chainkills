package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"git.sr.ht/~barveyhirdman/chainkills/config"
	"git.sr.ht/~barveyhirdman/chainkills/systems"
	"github.com/bwmarrin/discordgo"
)

var (
	configPath string
)

func main() {
	flag.StringVar(&configPath, "config", "config.yaml", "Path to config")
	flag.Parse()

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

	session, err := discordgo.New("Bot " + config.Get().Discord.Token)
	if err != nil {
		slog.Error("failed to create discord session", "error", err)
		os.Exit(1)
	}

	if err := session.Open(); err != nil {
		slog.Error("failed to open discord session", "error", err)
		os.Exit(1)
	}

	register := systems.Register()
	if _, err := register.Update(); err != nil {
		slog.Error("failed to update systems", "error", err)
		os.Exit(1)
	}

	out := make(chan systems.Killmail)

	tickerDuration := time.Duration(config.Get().RefreshInterval) * time.Second
	slog.Debug("starting ticker", "interval", tickerDuration.String())
	tick := time.NewTicker(tickerDuration)

	wg := &sync.WaitGroup{}
	go func() {
		for {
			select {
			case <-tick.C:
				change, err := register.Update()
				if err != nil {
					slog.Error("failed to update systems", "error", err)
				}

				if change {
					if err := register.Fetch(out); err != nil {
						slog.Error("failed to fetch killmails")
					}
				}
			case msg := <-out:
				wg.Add(1)
				func() {
					defer wg.Done()
					embed, err := msg.Embed()
					if err != nil {
						slog.Error("failed to prepare embed", "error", err)
						return
					}
					if _, err := session.ChannelMessageSendEmbed(config.Get().Discord.Channel, embed); err != nil {
						slog.Error("failed to send message", "error", err)
						return
					}
				}()
			}
		}
	}()

	if err := register.Fetch(out); err != nil {
		slog.Error("failed to fetch killmails")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	tick.Stop()
	session.Close()
	close(out)
	wg.Wait()
	slog.Info("exiting")
}
