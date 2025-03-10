package discord

import "github.com/bwmarrin/discordgo"

var status *State

func Init() {
	status = &State{
		Guilds: make(map[string]Guild, 0),
	}
}

type State struct {
	Guilds map[string]Guild
}

type Guild struct {
	ID       string
	Name     string
	Channels map[string]Channel
}

type Channel struct {
	ID      string                `yaml:"id"`
	GuildID string                `yaml:"guild_id"`
	Name    string                `yaml:"name"`
	Kind    discordgo.ChannelType `yaml:"kind"`
}

func addGuild(g Guild) {
	status.Guilds[g.ID] = g
}

func removeGuild(id string) {
	delete(status.Guilds, id)
}
