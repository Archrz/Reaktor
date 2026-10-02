package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
					Description: "e.g. \"ticket\" creates channels like ticket-123456",
					Required:    true,
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

type panelConfig struct {
	ChannelID string
	RoleID    string
	Prefix    string
}

// configs is saved to disk; awaitingText isn't (it only lives a few seconds).
var (
	stateMu sync.Mutex

	configs      = map[string]panelConfig{} // guildID -> current channel/role/prefix
	awaitingText = map[string]string{}      // userID -> guildID they're sending panel text for
)

const configsPath = "/data/configs.json"

func loadConfigs() {
	data, err := os.ReadFile(configsPath)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Println("Failed to load configs:", err)
	}
}

func getConfig(guildID string) panelConfig {
	stateMu.Lock()
	defer stateMu.Unlock()
	return configs[guildID]
}

// Applies one field change to a guild's config and saves it to disk.
func setConfig(guildID string, mutate func(*panelConfig)) {
	stateMu.Lock()
	cfg := configs[guildID]
	mutate(&cfg)
	configs[guildID] = cfg
	data, err := json.Marshal(configs)
	stateMu.Unlock()
	if err != nil {
		log.Println("Failed to marshal configs:", err)
		return
	}
	if err := os.WriteFile(configsPath, data, 0600); err != nil {
		log.Println("Failed to save configs:", err)
	}
}

// Checks that a message is really one the bot sent, so we can trust it.
func fetchOwnEmbed(s *discordgo.Session, channelID, messageID string) (*discordgo.MessageEmbed, bool) {
	msg, err := s.ChannelMessage(channelID, messageID)
	if err != nil || len(msg.Embeds) == 0 || msg.Author.ID != s.State.User.ID {
		return nil, false
	}
	return msg.Embeds[0], true
}

func main() {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN environment variable required")
	}

	loadConfigs()

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session:", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		if _, err := s.ApplicationCommandCreate(s.State.User.ID, "", configCmd); err != nil {
			log.Println("Failed to register command:", err)
		}
	})

	dg.AddHandler(handleInteraction)
	dg.AddHandler(handleMessage)
	dg.AddHandler(handleReaction)

	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection:", err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	dg.Close()
}

func reply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// Handles each /config option (channel, role, prefix, message).
func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "config" || len(data.Options) == 0 {
		return
	}
	sub := data.Options[0]

	switch sub.Name {
	case "channel":
		channelID := sub.Options[0].ChannelValue(nil).ID
		setConfig(i.GuildID, func(c *panelConfig) { c.ChannelID = channelID })
		reply(s, i, fmt.Sprintf("Support channel set to <#%s>.", channelID))

	case "role":
		roleID := sub.Options[0].RoleValue(nil, "").ID
		setConfig(i.GuildID, func(c *panelConfig) { c.RoleID = roleID })
		reply(s, i, fmt.Sprintf("Reader role set to <@&%s>.", roleID))

	case "prefix":
		prefix := sub.Options[0].StringValue()
		setConfig(i.GuildID, func(c *panelConfig) { c.Prefix = prefix })
		reply(s, i, fmt.Sprintf("Channel prefix set to %q.", prefix))

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

// Catches the admin's message and asks them to confirm it before posting.
func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	stateMu.Lock()
	guildID, ok := awaitingText[m.Author.ID]
	if ok {
		delete(awaitingText, m.Author.ID)
	}
	stateMu.Unlock()
	if !ok {
		return
	}
	cfg := getConfig(guildID)

	confirmMsg, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:   fmt.Sprintf("React ✅ to confirm deploying this panel to <#%s>, or ❌ to redo the message.", cfg.ChannelID),
		Reference: m.Reference(),
		Embed: &discordgo.MessageEmbed{
			Description: m.Content,
			Color:       0x2b2d31,
		},
	})
	if err != nil {
		return
	}
	s.MessageReactionAdd(confirmMsg.ChannelID, confirmMsg.ID, "✅")
	s.MessageReactionAdd(confirmMsg.ChannelID, confirmMsg.ID, "❌")
}

// Sends each reaction to the right handler based on the emoji used.
func handleReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}
	switch r.Emoji.Name {
	case "✅":
		handleConfirmReaction(s, r)
	case "❌":
		handleRedoReaction(s, r)
	case "📩":
		handleTicketReaction(s, r)
	}
}

// Posts the panel once someone reacts with ✅ to confirm it.
func handleConfirmReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	srcEmbed, ok := fetchOwnEmbed(s, r.ChannelID, r.MessageID)
	if !ok {
		return
	}

	cfg := getConfig(r.GuildID)
	if cfg.ChannelID == "" || cfg.RoleID == "" || cfg.Prefix == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Description: srcEmbed.Description,
		Color:       0x2b2d31,
	}

	panelMsg, err := s.ChannelMessageSendEmbed(cfg.ChannelID, embed)
	if err != nil {
		return
	}
	s.MessageReactionAdd(cfg.ChannelID, panelMsg.ID, "📩")
}

// Lets the admin type the panel message again if they don't like it.
func handleRedoReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if _, ok := fetchOwnEmbed(s, r.ChannelID, r.MessageID); !ok {
		return
	}

	stateMu.Lock()
	awaitingText[r.UserID] = r.GuildID
	stateMu.Unlock()

	ref := &discordgo.MessageReference{MessageID: r.MessageID, ChannelID: r.ChannelID, GuildID: r.GuildID}
	s.ChannelMessageSendReply(r.ChannelID, "Send the new panel message.", ref)
}

// Makes a private ticket channel when someone reacts to the panel.
func handleTicketReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if _, ok := fetchOwnEmbed(s, r.ChannelID, r.MessageID); !ok {
		return
	}

	cfg := getConfig(r.GuildID)
	if cfg.RoleID == "" || cfg.Prefix == "" {
		return
	}

	s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)

	channelName := fmt.Sprintf("%s-%s", cfg.Prefix, r.UserID)

	channels, err := s.GuildChannels(r.GuildID)
	if err != nil {
		return
	}
	for _, c := range channels {
		if c.Name == channelName {
			return
		}
	}

	_, err = s.GuildChannelCreateComplex(r.GuildID, discordgo.GuildChannelCreateData{
		Name: channelName,
		Type: discordgo.ChannelTypeGuildText,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:   r.GuildID,
				Type: discordgo.PermissionOverwriteTypeRole,
				Deny: discordgo.PermissionViewChannel,
			},
			{
				ID:    r.UserID,
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages,
			},
			{
				ID:    cfg.RoleID,
				Type:  discordgo.PermissionOverwriteTypeRole,
				Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages,
			},
		},
	})
}
