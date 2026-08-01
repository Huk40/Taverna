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
	ModelAssetDir      string
	ExportArtifactDir  string
	MaxModelAssetBytes int64
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://taverna:taverna@localhost:5432/taverna?sslmode=disable"),
		MigrationsDir:      getEnv("MIGRATIONS_DIR", "migrations"),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		ModelAssetDir:      getEnv("MODEL_ASSET_DIR", "storage/model-assets"),
		ExportArtifactDir:  getEnv("EXPORT_ARTIFACT_DIR", "storage/export-artifacts"),
		MaxModelAssetBytes: getEnvInt64("MAX_MODEL_ASSET_BYTES", 50<<20),
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

func getEnvInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	var parsed int64
	for _, char := range value {
		if char < '0' || char > '9' {
			return fallback
		}
		parsed = parsed*10 + int64(char-'0')
	}
	if parsed <= 0 {
		return fallback
	}
	return parsed
}
