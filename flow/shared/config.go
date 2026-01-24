package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

// Config holds the BunnyDB configuration
type Config struct {
	// Catalog database
	CatalogHost     string
	CatalogPort     int
	CatalogUser     string
	CatalogPassword string
	CatalogDatabase string

	// Temporal
	TemporalHostPort string
	TemporalNamespace string

	// Worker settings
	WorkerTaskQueue string

	// Auth
	JWTSecret     string
	AdminUser     string
	AdminPassword string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		CatalogHost:       getEnvOrDefault("BUNNY_CATALOG_HOST", "localhost"),
		CatalogPort:       getEnvIntOrDefault("BUNNY_CATALOG_PORT", 5432),
		CatalogUser:       getEnvOrDefault("BUNNY_CATALOG_USER", "postgres"),
		CatalogPassword:   getEnvOrDefault("BUNNY_CATALOG_PASSWORD", "bunnydb"),
		CatalogDatabase:   getEnvOrDefault("BUNNY_CATALOG_DATABASE", "bunnydb"),
		TemporalHostPort:  getEnvOrDefault("TEMPORAL_HOST_PORT", "localhost:7233"),
		TemporalNamespace: getEnvOrDefault("TEMPORAL_NAMESPACE", "default"),
		WorkerTaskQueue:   getEnvOrDefault("BUNNY_WORKER_TASK_QUEUE", "bunny-worker"),
		JWTSecret:         getEnvOrDefault("BUNNY_JWT_SECRET", ""),
		AdminUser:         getEnvOrDefault("BUNNY_ADMIN_USER", "admin"),
		AdminPassword:     getEnvOrDefault("BUNNY_ADMIN_PASSWORD", ""),
	}

	// Generate a random JWT secret if not provided
	if config.JWTSecret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		config.JWTSecret = hex.EncodeToString(b)
		slog.Warn("BUNNY_JWT_SECRET not set, generated random secret (tokens will not survive restart)")
	}

	return config, nil
}

// CatalogConnectionString returns the connection string for the catalog database
func (c *Config) CatalogConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.CatalogHost, c.CatalogPort, c.CatalogUser, c.CatalogPassword, c.CatalogDatabase,
	)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
