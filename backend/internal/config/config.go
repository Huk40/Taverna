package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TrustedProxyCIDRs  []string
	Port               string
	DatabaseURL        string
	MigrationsDir      string
	CORSAllowedOrigins []string
	ModelAssetDir      string
	ExportArtifactDir  string
	MaxModelAssetBytes int64
	AuthCookieSecure   bool
	SMTPAllowInsecure  bool
	PublicAppURL       string
	SMTPHost           string
	SMTPPort           string
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
}

func Load() Config {
	return Config{
		TrustedProxyCIDRs:  splitCSV(getEnv("TRUSTED_PROXY_CIDRS", "")),
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://taverna:taverna@localhost:5432/taverna?sslmode=disable"),
		MigrationsDir:      getEnv("MIGRATIONS_DIR", "migrations"),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		ModelAssetDir:      getEnv("MODEL_ASSET_DIR", "storage/model-assets"),
		ExportArtifactDir:  getEnv("EXPORT_ARTIFACT_DIR", "storage/export-artifacts"),
		MaxModelAssetBytes: getEnvInt64("MAX_MODEL_ASSET_BYTES", 50<<20),
		AuthCookieSecure:   getEnvBool("AUTH_COOKIE_SECURE", false),
		SMTPAllowInsecure:  getEnvBool("SMTP_ALLOW_INSECURE", false),
		PublicAppURL:       getEnv("PUBLIC_APP_URL", "http://localhost:5173/"),
		SMTPHost:           strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:           strings.TrimSpace(os.Getenv("SMTP_PORT")),
		SMTPUsername:       strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:       os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:           strings.TrimSpace(os.Getenv("SMTP_FROM")),
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

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
