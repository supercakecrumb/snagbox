// Package bot runs snagbox's Telegram intake bot: allowlisted users send text
// and photos, which become inbox issues that can be tagged to a project via
// inline buttons.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	authkit "github.com/supercakecrumb/msgr-authkit"

	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/config"
	"github.com/supercakecrumb/snagbox/internal/store"
)

// LoginLinkService issues bot-first login links for the admin dashboard. It is
// satisfied by *authkit.AuthService and may be nil when login is disabled.
type LoginLinkService interface {
	CreateLoginLink(ctx context.Context, in authkit.CreateLoginLinkInput) (authkit.CreateLoginLinkOutput, error)
}

// Bot is the Telegram intake bot. It turns allowlisted users' messages into
// inbox issues and offers inline buttons to tag them to a project.
type Bot struct {
	client   *tgbot.Bot
	token    string
	store    *store.Store
	blob     *blob.Store
	loginSvc LoginLinkService // nil when the admin dashboard login is disabled
	admins   map[int64]bool   // from cfg.AdminTelegramIDs, for bootstrap seeding
	http     *http.Client
	logger   *slog.Logger
	baseCtx  context.Context // set in Start, used by album flush timers
	albums   *albumBuffer
}

// New builds a Bot, wiring the Telegram client, its handlers and the album
// buffer. loginSvc may be nil to disable the /login dashboard flow. It does not
// start polling; call Start for that.
func New(cfg config.Config, st *store.Store, bl *blob.Store, loginSvc LoginLinkService, logger *slog.Logger) (*Bot, error) {
	admins := make(map[int64]bool, len(cfg.AdminTelegramIDs))
	for _, id := range cfg.AdminTelegramIDs {
		admins[id] = true
	}

	b := &Bot{
		token:    cfg.TelegramBotToken,
		store:    st,
		blob:     bl,
		loginSvc: loginSvc,
		admins:   admins,
		http:     &http.Client{Timeout: 30 * time.Second},
		logger:   logger,
		albums:   newAlbumBuffer(2 * time.Second),
	}

	// An album flushes into a single issue with all its photos once the
	// media group has gone quiet for the buffer delay.
	b.albums.flush = func(g *albumGroup) {
		ctx := b.baseCtx
		if ctx == nil {
			ctx = context.Background()
		}
		b.createIssueWithPhotos(ctx, g.chatID, g.user, g.caption, g.fileIDs)
	}

	client, err := tgbot.New(cfg.TelegramBotToken, tgbot.WithDefaultHandler(b.handleDefault))
	if err != nil {
		return nil, fmt.Errorf("create telegram client: %w", err)
	}
	b.client = client

	// MatchTypeCommand compares the command name without its leading slash, so
	// the patterns are registered slash-less ("start", not "/start").
	// Registering them with a slash never matches, and the message falls
	// through to handleDefault and gets filed as an issue.
	client.RegisterHandler(tgbot.HandlerTypeMessageText, "start", tgbot.MatchTypeCommand, b.handleStart)
	client.RegisterHandler(tgbot.HandlerTypeMessageText, "last", tgbot.MatchTypeCommand, b.handleLast)
	client.RegisterHandler(tgbot.HandlerTypeMessageText, "projects", tgbot.MatchTypeCommand, b.handleProjects)
	client.RegisterHandler(tgbot.HandlerTypeMessageText, "login", tgbot.MatchTypeCommand, b.handleLogin)
	client.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "tag:", tgbot.MatchTypePrefix, b.handleTagCallback)

	return b, nil
}

// Start records the base context for background flush timers and blocks polling
// Telegram until ctx is canceled.
func (b *Bot) Start(ctx context.Context) {
	b.baseCtx = ctx
	b.client.Start(ctx)
}

// Client returns the underlying Telegram client.
func (b *Bot) Client() *tgbot.Bot { return b.client }

// SendText sends a plain-text message to a chat, used by the daily digest.
func (b *Bot) SendText(ctx context.Context, chatID int64, text string) error {
	if _, err := b.client.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		return fmt.Errorf("send text: %w", err)
	}
	return nil
}

// resolveUser maps a Telegram user to a stored user, seeding admins on first
// contact. The bool is false when the user is not allowed to file issues.
func (b *Bot) resolveUser(ctx context.Context, from *models.User) (store.User, bool) {
	if from == nil {
		return store.User{}, false
	}

	u, err := b.store.GetUserByTelegramID(ctx, from.ID)
	if err == nil {
		return u, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		b.logger.Error("resolve user", "tg_id", from.ID, "error", err)
		return store.User{}, false
	}

	if !b.admins[from.ID] {
		return store.User{}, false
	}

	u, err = b.store.CreateUser(ctx, from.ID, userDisplayName(from), "admin")
	if err != nil {
		b.logger.Error("seed admin user", "tg_id", from.ID, "error", err)
		return store.User{}, false
	}
	return u, true
}

// userDisplayName joins the Telegram first and last name, falling back to the
// username or a numeric placeholder.
func userDisplayName(from *models.User) string {
	name := strings.TrimSpace(from.FirstName + " " + from.LastName)
	if name != "" {
		return name
	}
	if from.Username != "" {
		return from.Username
	}
	return fmt.Sprintf("tg-%d", from.ID)
}
