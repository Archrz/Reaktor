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

type panelConfig struct {
	ChannelID  string
	RoleID     string
	Prefix     string
	CategoryID string // empty means "use the panel channel's own category"
	NextTicket int    // counts up: ticket-1, ticket-2, ...
}

// configs is saved to disk; awaitingText isn't.
var (
	stateMu sync.Mutex

	configs      = map[string]panelConfig{} // guildID -> current config
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

// Applies one change to a guild's config, saves it, and returns the result.
func setConfig(guildID string, mutate func(*panelConfig)) panelConfig {
	stateMu.Lock()
	cfg := configs[guildID]
	mutate(&cfg)
	configs[guildID] = cfg
	data, err := json.Marshal(configs)
	stateMu.Unlock()
	if err != nil {
		log.Println("Failed to marshal configs:", err)
		return cfg
	}
	if err := os.WriteFile(configsPath, data, 0600); err != nil {
		log.Println("Failed to save configs:", err)
	}
	return cfg
}

// Checks that a message is really one the bot sent, so we can trust it.
func fetchOwnMessage(s *discordgo.Session, channelID, messageID string) (*discordgo.Message, bool) {
	msg, err := s.ChannelMessage(channelID, messageID)
	if err != nil || msg.Author.ID != s.State.User.ID {
		return nil, false
	}
	return msg, true
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

	case "category":
		categoryID := sub.Options[0].ChannelValue(nil).ID
		setConfig(i.GuildID, func(c *panelConfig) { c.CategoryID = categoryID })
		reply(s, i, fmt.Sprintf("Ticket category set to <#%s>.", categoryID))

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

	confirmMsg, err := s.ChannelMessageSendReply(m.ChannelID,
		fmt.Sprintf("React ✅ to confirm deploying this panel to <#%s>, or ❌ to redo the message.", cfg.ChannelID),
		m.Reference())
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
	msg, ok := fetchOwnMessage(s, r.ChannelID, r.MessageID)
	if !ok || msg.MessageReference == nil {
		return
	}
	orig, err := s.ChannelMessage(msg.MessageReference.ChannelID, msg.MessageReference.MessageID)
	if err != nil {
		return
	}

	cfg := getConfig(r.GuildID)
	if cfg.ChannelID == "" || cfg.RoleID == "" || cfg.Prefix == "" {
		return
	}

	panelMsg, err := s.ChannelMessageSend(cfg.ChannelID, orig.Content)
	if err != nil {
		return
	}
	s.MessageReactionAdd(cfg.ChannelID, panelMsg.ID, "📩")
}

// Lets the admin type the panel message again if they don't like it.
func handleRedoReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if _, ok := fetchOwnMessage(s, r.ChannelID, r.MessageID); !ok {
		return
	}

	stateMu.Lock()
	awaitingText[r.UserID] = r.GuildID
	stateMu.Unlock()

	ref := &discordgo.MessageReference{MessageID: r.MessageID, ChannelID: r.ChannelID, GuildID: r.GuildID}
	s.ChannelMessageSendReply(r.ChannelID, "Send the new panel message.", ref)
}

// Falls back to the panel channel's own category if none is set.
func ticketCategory(s *discordgo.Session, cfg panelConfig) string {
	if cfg.CategoryID != "" {
		return cfg.CategoryID
	}
	ch, err := s.Channel(cfg.ChannelID)
	if err != nil {
		return ""
	}
	return ch.ParentID
}

// Makes a private ticket channel when someone reacts to the panel.
func handleTicketReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if _, ok := fetchOwnMessage(s, r.ChannelID, r.MessageID); !ok {
		return
	}

	cfg := getConfig(r.GuildID)
	if cfg.RoleID == "" || cfg.Prefix == "" {
		return
	}

	s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)

	cfg = setConfig(r.GuildID, func(c *panelConfig) { c.NextTicket++ })
	channelName := fmt.Sprintf("%s-%d", cfg.Prefix, cfg.NextTicket)

	s.GuildChannelCreateComplex(r.GuildID, discordgo.GuildChannelCreateData{
		Name:     channelName,
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: ticketCategory(s, cfg),
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
