package config

import (
	"os"
)

type Config struct {
	Port         string
	DatabasePath string
	BotStorePath string
}

func LoadConfig() *Config {
	return &Config{
		Port:         getEnv("PORT", "8080"),
		DatabasePath: getEnv("DATABASE_PATH", "./gowa_data.db"),
		BotStorePath: getEnv("BOT_STORE_PATH", "examplestore.db"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
