package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type panelConfig struct {
	ChannelID  string
	RoleID     string
	Prefix     string
	CategoryID string // empty means "use the panel channel's own category"
	NextTicket int    // counts up: ticket-1, ticket-2, ...
}

// configs is saved to disk, one file per guild
var (
	stateMu sync.Mutex

	configs      = map[string]panelConfig{} // guildID -> current config
	awaitingText = map[string]string{}      // userID -> guildID they're sending panel text for
)

const configsDir = "/data/configs"

func configPath(guildID string) string {
	return filepath.Join(configsDir, guildID+".json")
}

func loadConfigs() {
	entries, err := os.ReadDir(configsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		guildID, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(configsDir, entry.Name()))
		if err != nil {
			log.Println("Failed to read config for guild", guildID, ":", err)
			continue
		}
		var cfg panelConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Println("Failed to load config for guild", guildID, ":", err)
			continue
		}
		configs[guildID] = cfg
	}
}

func getConfig(guildID string) panelConfig {
	stateMu.Lock()
	defer stateMu.Unlock()
	return configs[guildID]
}

// Applies one change to a guild's config, saves and returns the result.
func setConfig(guildID string, mutate func(*panelConfig)) panelConfig {
	stateMu.Lock()
	cfg := configs[guildID]
	mutate(&cfg)
	configs[guildID] = cfg
	data, _ := json.Marshal(cfg)
	stateMu.Unlock()

	if err := os.MkdirAll(configsDir, 0700); err != nil {
		log.Println("Failed to create configs dir:", err)
		return cfg
	}
	if err := os.WriteFile(configPath(guildID), data, 0600); err != nil {
		log.Println("Failed to save config for guild", guildID, ":", err)
	}
	return cfg
}
