package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// Checks that a message is really one the bot sent, so we can trust it.
func fetchOwnMessage(s *discordgo.Session, channelID, messageID string) (*discordgo.Message, bool) {
	msg, err := s.ChannelMessage(channelID, messageID)

	if err != nil || msg.Author.ID != s.State.User.ID {
		return nil, false
	}
	return msg, true
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
	if !ok || guildID != m.GuildID {
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
