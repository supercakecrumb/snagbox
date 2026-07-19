// Command snagbox is a self-hosted issue-intake service: a Telegram bot and
// HTTP API collect issues (text + photos) into per-project queues that agents
// drain independently.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	authkit "github.com/supercakecrumb/msgr-authkit"

	"github.com/supercakecrumb/snagbox/internal/api"
	"github.com/supercakecrumb/snagbox/internal/auth"
	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/bot"
	"github.com/supercakecrumb/snagbox/internal/config"
	"github.com/supercakecrumb/snagbox/internal/store"
	"github.com/supercakecrumb/snagbox/internal/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}
	if err := run(); err != nil {
		slog.Error("snagbox: fatal", "error", err)
		os.Exit(1)
	}
}

// runHealthcheck probes the local /healthz endpoint. It reads PORT directly
// from the environment, not via config.Load, so the Docker HEALTHCHECK stays
// cheap.
func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	bl, err := blob.New(ctx, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	if err != nil {
		return fmt.Errorf("open blob: %w", err)
	}

	// A database/sql handle over the same pool backs the authkit adapters.
	sqlDB := stdlib.OpenDBFromPool(st.Pool)
	defer func() { _ = sqlDB.Close() }()

	intentStore := auth.NewPGIntentStore(sqlDB)
	linkStore := auth.NewPGLinkStore(sqlDB)
	sessionIssuer := auth.NewPGSessionIssuer(sqlDB, 7*24*time.Hour)

	// Login is only available when a 32-byte session key is configured.
	var (
		botLoginSvc bot.LoginLinkService
		webRedeemer web.LoginRedeemer
	)
	if len(cfg.SessionEncKey) == 32 {
		authService, err := authkit.NewAuthService(intentStore, linkStore, sessionIssuer,
			authkit.WithSignedQueryLoginLinks(cfg.PublicBaseURL+"/login", cfg.SessionEncKey, "auth_token"))
		if err != nil {
			return fmt.Errorf("init auth service: %w", err)
		}
		botLoginSvc = authService
		webRedeemer = authService
	} else {
		logger.Warn("SESSION_ENC_KEY not set — admin dashboard login disabled")
	}

	apiHandler := api.New(st, bl, cfg.PublicBaseURL, logger)

	tg, err := bot.New(cfg, st, bl, botLoginSvc, logger)
	if err != nil {
		return fmt.Errorf("init bot: %w", err)
	}

	cookieSecure := strings.HasPrefix(cfg.PublicBaseURL, "https://")
	webServer := web.NewServer(web.Deps{
		Store:        st,
		Blob:         bl,
		AuthService:  webRedeemer,
		Sessions:     sessionIssuer,
		ServerSecret: cfg.SessionEncKey,
		CookieSecure: cookieSecure,
		Logger:       logger,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	})
	mux.Handle("/api/v1/", apiHandler.Routes())
	mux.Handle("/", webServer.Handler())
	slog.Info("admin web UI mounted")

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go tg.Start(ctx)
	slog.Info("telegram bot started")

	slog.Info("snagbox started", "port", cfg.Port)

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
