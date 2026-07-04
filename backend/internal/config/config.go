package config

import (
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	MigrationsDir      string
	CORSAllowedOrigins []string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://taverna:taverna@localhost:5432/taverna?sslmode=disable"),
		MigrationsDir:      getEnv("MIGRATIONS_DIR", "migrations"),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
	}
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
