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
	"github.com/julianshen/og"
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
	if err := register.Fetch(out); err != nil {
		slog.Error("failed to fetch killmails")
	}

	tick := time.NewTicker(time.Duration(config.Get().RefreshInterval) * time.Second)

	go func() {
		for range tick.C {
			change, err := register.Update()
			if err != nil {
				slog.Error("failed to update systems", "error", err)
			}

			if change {
				if err := register.Fetch(out); err != nil {
					slog.Error("failed to fetch killmails")
				}
			}
		}
	}()

	stopOutbox := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopOutbox:
				return
			case msg := <-out:
				embed, err := prepareEmbed(msg.Zkill.URL, msg.Color())
				if err != nil {
					slog.Error("failed to prepare embed", "error", err)
					return
				}
				if _, err := session.ChannelMessageSendEmbed(config.Get().Discord.Channel, embed); err != nil {
					slog.Error("failed to send message", "error", err)
					return
				}
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	tick.Stop()
	session.Close()
	close(stopOutbox)
	slog.Info("exiting")
}

func prepareEmbed(url string, color int) (*discordgo.MessageEmbed, error) {
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
		Color:       color,
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
