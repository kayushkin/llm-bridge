package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr    string
	DBPath        string
	ModelStoreURL string
	AgentStoreURL string
	NatsURL       string
}

func Load() *Config {
	return &Config{
		ListenAddr:    envOr("LLMBRIDGE_LISTEN_ADDR", ":8160"),
		DBPath:        envOr("LLMBRIDGE_DB_PATH", filepath.Join(os.Getenv("HOME"), ".llm-bridge", "bridge.db")),
		ModelStoreURL: os.Getenv("LLMBRIDGE_MODEL_STORE_URL"),
		AgentStoreURL: os.Getenv("LLMBRIDGE_AGENT_STORE_URL"),
		NatsURL:       os.Getenv("NATS_URL"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
