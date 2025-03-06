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
	"github.com/gorilla/websocket"
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

	ws, _, err := websocket.DefaultDialer.Dial("wss://zkillboard.com/websocket/", nil)
	if err != nil {
		slog.Error("failed to establish websocket connection", "error", err)
	}

	register := systems.Register(systems.WithConfig(cfg), systems.WithWebsocket(ws))
	if _, err := register.Update(); err != nil {
		slog.Error("failed to update systems", "error", err)
		os.Exit(1)
	}

	out := make(chan systems.Killmail)
	refresh := make(chan struct{})
	register.Start(out, refresh)

	tick := time.NewTicker(10 * time.Second)

	go func() {
		for range tick.C {
			slog.Debug("updating system list")
			change, err := register.Update()
			if err != nil {
				slog.Error("failed to update systems", "error", err)
			}

			if change {
				slog.Debug("change in systems detected")
				refresh <- struct{}{}
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
				embed, err := prepareEmbed(msg.Zkill.URL, msg.Color(cfg))
				if err != nil {
					slog.Error("failed to prepare embed", "error", err)
					return
				}
				if _, err := session.ChannelMessageSendEmbed(cfg.Discord.Channel, embed); err != nil {
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
	if err := register.Stop(); err != nil {
		slog.Error("failed to stop system register", "error", err)
	}
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
