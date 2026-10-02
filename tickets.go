package main

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

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
	panelMsg, ok := fetchOwnMessage(s, r.ChannelID, r.MessageID)
	if !ok {
		return
	}

	cfg := getConfig(r.GuildID)
	if cfg.RoleID == "" || cfg.Prefix == "" {
		return
	}

	s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)

	cfg = setConfig(r.GuildID, func(c *panelConfig) { c.NextTicket++ })
	channelName := fmt.Sprintf("%s-%d", cfg.Prefix, cfg.NextTicket)

	ch, err := s.GuildChannelCreateComplex(r.GuildID, discordgo.GuildChannelCreateData{
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
			{
				ID:    s.State.User.ID,
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages,
			},
		},
	})
	if err != nil {
		return
	}
	if _, err := s.ChannelMessageSend(ch.ID, panelMsg.Content); err != nil {
		log.Println("Failed to post panel message in ticket channel:", err)
	}
}
