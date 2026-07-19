package config

import (
	"log/slog"
	"strings"
	"testing"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:abc")
	t.Setenv("ADMIN_TG_IDS", "42, 1337")
	t.Setenv("DATABASE_URL", "postgres://snagbox:snagbox@localhost:5432/snagbox")
	t.Setenv("S3_ENDPOINT", "localhost:9000")
	t.Setenv("S3_ACCESS_KEY", "minio")
	t.Setenv("S3_SECRET_KEY", "minio123")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_USE_SSL", "")
	t.Setenv("PUBLIC_BASE_URL", "https://snagbox.example.com/")
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("SESSION_ENC_KEY", "")
}

func TestLoadHappyPath(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.TelegramBotToken != "123:abc" {
		t.Errorf("TelegramBotToken = %q, want %q", cfg.TelegramBotToken, "123:abc")
	}
	if len(cfg.AdminTelegramIDs) != 2 || cfg.AdminTelegramIDs[0] != 42 || cfg.AdminTelegramIDs[1] != 1337 {
		t.Errorf("AdminTelegramIDs = %v, want [42 1337]", cfg.AdminTelegramIDs)
	}
	if cfg.S3Bucket != "snagbox" {
		t.Errorf("S3Bucket = %q, want default %q", cfg.S3Bucket, "snagbox")
	}
	if cfg.S3UseSSL {
		t.Error("S3UseSSL = true, want default false")
	}
	if cfg.PublicBaseURL != "https://snagbox.example.com" {
		t.Errorf("PublicBaseURL = %q, want trailing slash trimmed", cfg.PublicBaseURL)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want default %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.SessionEncKey != nil {
		t.Errorf("SessionEncKey = %v, want nil", cfg.SessionEncKey)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want error for missing DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error %q does not mention DATABASE_URL", err)
	}
}

func TestLoadBadAdminTGIDs(t *testing.T) {
	setValidEnv(t)
	t.Setenv("ADMIN_TG_IDS", "42,not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want error for bad ADMIN_TG_IDS")
	}
	if !strings.Contains(err.Error(), "ADMIN_TG_IDS") {
		t.Errorf("error %q does not mention ADMIN_TG_IDS", err)
	}
}

func TestLoadLogLevel(t *testing.T) {
	for name, want := range map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"error": slog.LevelError,
	} {
		t.Run(name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv("LOG_LEVEL", name)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.LogLevel != want {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, want)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		setValidEnv(t)
		t.Setenv("LOG_LEVEL", "verbose")

		_, err := Load()
		if err == nil {
			t.Fatal("Load() succeeded, want error for invalid LOG_LEVEL")
		}
		if !strings.Contains(err.Error(), "LOG_LEVEL") {
			t.Errorf("error %q does not mention LOG_LEVEL", err)
		}
	})
}
