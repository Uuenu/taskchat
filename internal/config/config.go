package config

import (
	"os"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server       ServerConfig
	MongoDB      MongoDBConfig
	Tinode       TinodeConfig
	JWT          JWTConfig
	ChatProvider string
}

type ServerConfig struct {
	Port string
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type TinodeConfig struct {
	Host            string
	Port            string
	ServiceLogin    string
	ServicePassword string
}

type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// Load reads configuration from environment variables with defaults
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://mongodb:27017"),
			Database: getEnv("MONGODB_DATABASE", "chat_db"),
		},
		Tinode: TinodeConfig{
			Host:            getEnv("TINODE_HOST", "tinode"),
			Port:            getEnv("TINODE_PORT", "6060"),
			ServiceLogin:    getEnv("TINODE_SERVICE_LOGIN", ""),
			ServicePassword: getEnv("TINODE_SERVICE_PASSWORD", ""),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-in-prod"),
			Expiration: 24 * time.Hour,
		},
		ChatProvider: getEnv("CHAT_PROVIDER", "tinode"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
