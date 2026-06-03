package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	OpenAIKey   string
	OpenAIModel string
	BraveAPIKey string
	LogLevel    string
}

func Load() (*Config, error) {
	c := &Config{
		DatabaseURL: env("DATABASE_URL", "postgres://papertrail:papertrail@localhost:807/papertrail"),
		OpenAIKey:   os.Getenv("OPENAI_API_KEY"),
		OpenAIModel: env("OPENAI_MODEL", "gpt-4o-mini"),
		BraveAPIKey: os.Getenv("BRAVE_API_KEY"),
		LogLevel:    env("LOG_LEVEL", "info"),
	}
	if c.OpenAIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
