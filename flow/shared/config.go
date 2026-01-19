package shared

import (
	"fmt"
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
