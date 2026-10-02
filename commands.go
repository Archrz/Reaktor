package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var adminPerms = int64(discordgo.PermissionAdministrator)

var configCmd = &discordgo.ApplicationCommand{
	Name:                     "config",
	Description:              "Configure the ticket panel",
	DefaultMemberPermissions: &adminPerms,
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "channel",
			Description: "Set where the panel message gets posted",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "channel",
					Description: "The channel to post the panel in",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "role",
			Description: "Set which role handles tickets opened from the panel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "role",
					Description: "The role that gets added to every ticket channel",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "prefix",
			Description: "Set the name prefix for ticket channels",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "value",
					Description: "e.g. \"ticket\" creates channels like ticket-1, ticket-2, ...",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "category",
			Description: "Set which category ticket channels go in",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionChannel,
					Name:         "category",
					Description:  "Category for ticket channels",
					ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildCategory},
					Required:     true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "message",
			Description: "Set the panel's message and post it for confirmation",
		},
	},
}

func reply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

// Handles each /config subcommand.
func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "config" || len(data.Options) == 0 {
		return
	}
	sub := data.Options[0]
	var opt *discordgo.ApplicationCommandInteractionDataOption
	if len(sub.Options) > 0 {
		opt = sub.Options[0]
	}

	switch sub.Name {
	case "channel":
		id := opt.ChannelValue(nil).ID
		setConfig(i.GuildID, func(c *panelConfig) { c.ChannelID = id })
		reply(s, i, fmt.Sprintf("Support channel set to <#%s>.", id))

	case "role":
		id := opt.RoleValue(nil, "").ID
		setConfig(i.GuildID, func(c *panelConfig) { c.RoleID = id })
		reply(s, i, fmt.Sprintf("Reader role set to <@&%s>.", id))

	case "prefix":
		v := opt.StringValue()
		setConfig(i.GuildID, func(c *panelConfig) { c.Prefix = v })
		reply(s, i, fmt.Sprintf("Channel prefix set to %q.", v))

	case "category":
		id := opt.ChannelValue(nil).ID
		setConfig(i.GuildID, func(c *panelConfig) { c.CategoryID = id })
		reply(s, i, fmt.Sprintf("Ticket category set to <#%s>.", id))

	case "message":
		cfg := getConfig(i.GuildID)
		if cfg.ChannelID == "" || cfg.RoleID == "" || cfg.Prefix == "" {
			reply(s, i, "Set the channel, role, and prefix first.")
			return
		}

		stateMu.Lock()
		awaitingText[i.Member.User.ID] = i.GuildID
		stateMu.Unlock()
		reply(s, i, "Send the panel message as your next message.")
	}
}
