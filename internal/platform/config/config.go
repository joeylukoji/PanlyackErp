package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Address, DatabaseURL, AIURL, AIKey, Secret, StoragePath string
	MaxUpload                                               int64
}

func Load() (Config, error) {
	c := Config{Address: env("HTTP_ADDR", "127.0.0.1:8080"), DatabaseURL: os.Getenv("DATABASE_URL"), AIURL: env("AI_BASE_URL", "http://127.0.0.1:8091"), AIKey: os.Getenv("AI_API_KEY"), Secret: os.Getenv("SESSION_SECRET"), StoragePath: env("DOCUMENT_STORAGE_PATH", "var/documents"), MaxUpload: 10 << 20}
	if os.Getenv("APP_ENV") == "production" {
		return c, errors.New("this local MVP requires an authentication deployment before production use")
	}
	if c.DatabaseURL == "" || c.AIKey == "" || len(c.Secret) < 32 {
		return c, errors.New("DATABASE_URL, AI_API_KEY and SESSION_SECRET (at least 32 characters) are required")
	}
	if v := os.Getenv("MAX_UPLOAD_MB"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > 10 {
			return c, errors.New("MAX_UPLOAD_MB must be 1 to 10")
		}
		c.MaxUpload = n << 20
	}
	return c, nil
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
