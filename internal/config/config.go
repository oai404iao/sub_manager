package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Addr          string
	DBPath        string
	AdminUser     string
	AdminPassword string
	BaseURL       string
	SigningKey    string
	SecureCookie  bool
}

func Load() Config {
	cfg := Config{
		Addr:          env("SUBMAN_ADDR", "0.0.0.0:8080"),
		DBPath:        env("SUBMAN_DB", "data/sub-manager.db"),
		AdminUser:     env("SUBMAN_ADMIN_USER", "admin"),
		AdminPassword: env("SUBMAN_ADMIN_PASSWORD", "admin123"),
		BaseURL:       env("SUBMAN_BASE_URL", "http://127.0.0.1:8080"),
		SigningKey:    env("SUBMAN_SIGNING_KEY", "development-only-signing-key-change-me"),
	}
	cfg.SecureCookie, _ = strconv.ParseBool(env("SUBMAN_SECURE_COOKIE", "false"))
	if cfg.AdminPassword == "admin123" || cfg.SigningKey == "development-only-signing-key-change-me" {
		log.Print("WARNING: using development credentials/signing key; configure SUBMAN_ADMIN_PASSWORD and SUBMAN_SIGNING_KEY before deployment")
	}
	return cfg
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
