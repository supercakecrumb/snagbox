// Package config loads snagbox configuration from environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds everything snagbox needs at startup.
type Config struct {
	TelegramBotToken string
	AdminTelegramIDs []int64
	DatabaseURL      string
	S3Endpoint       string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3UseSSL         bool
	PublicBaseURL    string
	Port             string
	LogLevel         slog.Level
	SessionEncKey    []byte
	// DigestHour is the local-time hour (0–23) at which the daily inbox
	// digest is sent to admins. Nil disables the digest.
	DigestHour *int
}

// Load reads the configuration from environment variables and returns a
// single joined error describing every missing or invalid variable.
func Load() (Config, error) {
	var errs []error

	required := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
		return v
	}

	cfg := Config{
		TelegramBotToken: required("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:      required("DATABASE_URL"),
		S3Endpoint:       required("S3_ENDPOINT"),
		S3AccessKey:      required("S3_ACCESS_KEY"),
		S3SecretKey:      required("S3_SECRET_KEY"),
		S3Bucket:         "snagbox",
		Port:             "8080",
		LogLevel:         slog.LevelInfo,
	}

	cfg.PublicBaseURL = strings.TrimRight(required("PUBLIC_BASE_URL"), "/")

	if v := os.Getenv("ADMIN_TG_IDS"); v == "" {
		errs = append(errs, errors.New("ADMIN_TG_IDS is required"))
	} else {
		for _, part := range strings.Split(v, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("ADMIN_TG_IDS: invalid id %q", part))
				continue
			}
			cfg.AdminTelegramIDs = append(cfg.AdminTelegramIDs, id)
		}
	}

	if v := os.Getenv("S3_BUCKET"); v != "" {
		cfg.S3Bucket = v
	}

	if v := os.Getenv("S3_USE_SSL"); v != "" {
		ssl, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("S3_USE_SSL: invalid bool %q", v))
		} else {
			cfg.S3UseSSL = ssl
		}
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		switch strings.ToLower(v) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			errs = append(errs, fmt.Errorf("LOG_LEVEL: must be one of debug/info/warn/error, got %q", v))
		}
	}

	if v := os.Getenv("SESSION_ENC_KEY"); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("SESSION_ENC_KEY: invalid base64: %w", err))
		case len(key) != 32:
			errs = append(errs, fmt.Errorf("SESSION_ENC_KEY: must decode to 32 bytes, got %d", len(key)))
		default:
			cfg.SessionEncKey = key
		}
	}

	if v := os.Getenv("DIGEST_HOUR"); v != "" {
		hour, err := strconv.Atoi(v)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("DIGEST_HOUR: invalid int %q", v))
		case hour < 0 || hour > 23:
			errs = append(errs, fmt.Errorf("DIGEST_HOUR: must be 0-23, got %d", hour))
		default:
			cfg.DigestHour = &hour
		}
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}
