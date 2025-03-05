package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
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

	cfg, err := config.Read(configPath)
	if err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		slog.Error("failed to create discord session", "error", err)
		os.Exit(1)
	}

	if err := session.Open(); err != nil {
		slog.Error("failed to open discord session", "error", err)
		os.Exit(1)
	}

	register := systems.Register(systems.WithConfig(cfg))
	if _, err := register.Update(); err != nil {
		slog.Error("failed to update systems", "error", err)
		os.Exit(1)
	}

	out := make(chan systems.Killmail)
	go register.Start(out)

	tick := time.NewTicker(60 * time.Second)

	// go func() {
	// 	for range tick.C {
	// 		_, err := register.Update()
	// 		if err != nil {
	// 			slog.Error("failed to update systems", "error", err)
	// 		}
	// 	}
	// }()

	stopOutbox := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopOutbox:
				return
			case msg := <-out:
				if _, err := session.ChannelMessageSend(cfg.Discord.Channel, msg.Zkill.URL); err != nil {
					slog.Error("failed to send message", "error", err)
					return
				}
			case e := <-register.Errors():
				slog.Error("error from register", "error", e)
			case <-tick.C:
				_, err := register.Update()
				if err != nil {
					slog.Error("failed to update systems", "error", err)
				}
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	tick.Stop()
	register.Stop()
	session.Close()
	close(stopOutbox)
	slog.Info("exiting")
}
