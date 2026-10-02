package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Max time without a heartbeat ack before the gateway connection is dead.
const heartbeatStaleAfter = 90 * time.Second

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

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if dg.LastHeartbeatAck.IsZero() || time.Since(dg.LastHeartbeatAck) > heartbeatStaleAfter {
			http.Error(w, "gateway heartbeat stale", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	go http.ListenAndServe(":8080", nil)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
	dg.Close()
}
