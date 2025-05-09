package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Port              string
	Environment       string
	PostgresHost      string
	PostgresPort      string
	PostgresUser      string
	PostgresPassword  string
	PostgresDB        string
	PostgresDSN       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	WorkerConcurrency int
	RetryLimit        int
	LogRetentionHours int
	RetryDelays       []time.Duration
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		Environment:       getEnv("ENV", "development"),
		PostgresHost:      getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:      getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:      getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword:  getEnv("POSTGRES_PASSWORD", "postgres"),
		PostgresDB:        getEnv("POSTGRES_DB", "webhook_service"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvAsInt("REDIS_DB", 0),
		WorkerConcurrency: getEnvAsInt("WORKER_CONCURRENCY", 10),
		RetryLimit:        getEnvAsInt("RETRY_LIMIT", 5),
		LogRetentionHours: getEnvAsInt("LOG_RETENTION_HOURS", 72),
		// Default retry delays with exponential backoff: 10s, 30s, 1m, 5m, 15m
		RetryDelays: []time.Duration{
			10 * time.Second,
			30 * time.Second,
			1 * time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		},
	}

	// Build PostgreSQL DSN
	cfg.PostgresDSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDB)

	return cfg, nil
}

// Helper function to get an environment variable or a default value
func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

// Helper function to get an environment variable as int
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, fmt.Sprintf("%d", defaultValue))
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
