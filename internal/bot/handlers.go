package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	authkit "github.com/supercakecrumb/msgr-authkit"

	"github.com/supercakecrumb/snagbox/internal/store"
)

// handleDefault handles ordinary private-chat messages: text and photos are
// filed as inbox issues. Non-private chats and empty messages are ignored.
func (b *Bot) handleDefault(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Chat.Type != models.ChatTypePrivate {
		return
	}

	user, ok := b.resolveUser(ctx, update.Message.From)
	if !ok {
		b.reply(ctx, update.Message.Chat.ID, "You're not authorized to file issues here.")
		return
	}

	switch {
	case len(update.Message.Photo) > 0:
		largest := update.Message.Photo[len(update.Message.Photo)-1]
		caption := update.Message.Caption
		if mgID := update.Message.MediaGroupID; mgID != "" {
			b.albums.add(mgID, update.Message.Chat.ID, user, caption, largest.FileID)
			return
		}
		b.createIssueWithPhotos(ctx, update.Message.Chat.ID, user, caption, []string{largest.FileID})
	case update.Message.Text != "":
		b.createIssueWithPhotos(ctx, update.Message.Chat.ID, user, update.Message.Text, nil)
	}
}

// createIssueWithPhotos files an inbox issue with body and any photos, then
// sends a confirmation with the project-tagging keyboard.
func (b *Bot) createIssueWithPhotos(ctx context.Context, chatID int64, user store.User, body string, photoFileIDs []string) {
	if body == "" && len(photoFileIDs) == 0 {
		return
	}

	issueID := uuid.Must(uuid.NewV7())
	authorID := user.ID
	if err := b.store.CreateIssue(ctx, store.Issue{
		ID:           issueID,
		ProjectID:    nil,
		Source:       "telegram",
		AuthorUserID: &authorID,
		Body:         body,
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		b.logger.Error("create issue", "issue_id", issueID, "error", err)
		b.reply(ctx, chatID, "Sorry, I couldn't file that issue.")
		return
	}

	stored := 0
	for _, fileID := range photoFileIDs {
		if err := b.storePhoto(ctx, issueID, fileID); err != nil {
			b.logger.Error("store photo", "issue_id", issueID, "error", err)
			continue
		}
		stored++
	}

	text := fmt.Sprintf("✅ Filed issue <code>%s</code> (%d photo(s)). Tag it to a project:",
		shortID(issueID), stored)
	b.replyMarkup(ctx, chatID, text, b.projectKeyboard(ctx, issueID.String()))
}

// storePhoto downloads a Telegram photo and links it to the issue as an
// attachment.
func (b *Bot) storePhoto(ctx context.Context, issueID uuid.UUID, fileID string) error {
	file, err := b.client.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, file.FilePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("download photo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download photo: unexpected status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read photo body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	attID := uuid.Must(uuid.NewV7())
	key := fmt.Sprintf("issues/%s/%s.jpg", issueID, attID)
	if err := b.blob.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return fmt.Errorf("put blob: %w", err)
	}

	if err := b.store.AddAttachment(ctx, store.Attachment{
		ID:        attID,
		IssueID:   issueID,
		S3Key:     key,
		Mime:      contentType,
		SizeBytes: int64(len(data)),
		Filename:  attID.String() + ".jpg",
	}); err != nil {
		return fmt.Errorf("add attachment: %w", err)
	}
	return nil
}

// projectKeyboard builds an inline keyboard with one button per project (two
// per row) plus a final "leave in inbox" button.
func (b *Bot) projectKeyboard(ctx context.Context, issueID string) *models.InlineKeyboardMarkup {
	inbox := models.InlineKeyboardButton{Text: "🗂 Leave in inbox", CallbackData: "tag:" + issueID + ":inbox"}

	projects, err := b.store.ListProjects(ctx)
	if err != nil {
		b.logger.Error("list projects for keyboard", "error", err)
		return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{inbox}}}
	}

	var rows [][]models.InlineKeyboardButton
	var row []models.InlineKeyboardButton
	for _, p := range projects {
		row = append(row, models.InlineKeyboardButton{
			Text:         p.Name,
			CallbackData: "tag:" + issueID + ":" + strconv.FormatInt(p.ID, 10),
		})
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, []models.InlineKeyboardButton{inbox})

	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// handleTagCallback processes a "tag:<issueID>:<pid|inbox>" callback, moving the
// issue to the chosen project (or leaving it in the inbox).
func (b *Bot) handleTagCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	q := update.CallbackQuery
	if q == nil {
		return
	}

	parts := strings.SplitN(q.Data, ":", 3)
	if len(parts) != 3 {
		b.answerCallback(ctx, q.ID, "bad request")
		return
	}
	issueID, err := uuid.Parse(parts[1])
	if err != nil {
		b.answerCallback(ctx, q.ID, "bad request")
		return
	}

	var (
		projectID   *int64
		projectName = "inbox"
	)
	if parts[2] != "inbox" {
		pid, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			b.answerCallback(ctx, q.ID, "bad request")
			return
		}
		projectID = &pid
		projectName = b.projectName(ctx, pid)
	}

	err = b.store.SetIssueProject(ctx, issueID, projectID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		b.answerCallback(ctx, q.ID, "issue not found")
		return
	case err != nil:
		b.logger.Error("set issue project", "issue_id", issueID, "error", err)
		b.answerCallback(ctx, q.ID, "something went wrong")
		return
	}

	b.answerCallback(ctx, q.ID, "Filed ✓")

	if msg := q.Message.Message; msg != nil {
		text := fmt.Sprintf("✅ Issue <code>%s</code> filed under <b>%s</b>",
			shortID(issueID), html.EscapeString(projectName))
		if _, err := b.client.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:    msg.Chat.ID,
			MessageID: msg.ID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		}); err != nil {
			b.logger.Error("edit tag message", "issue_id", issueID, "error", err)
		}
	}
}

// projectName returns a project's display name for pid, falling back to "inbox".
func (b *Bot) projectName(ctx context.Context, pid int64) string {
	projects, err := b.store.ListProjects(ctx)
	if err != nil {
		b.logger.Error("list projects for name", "error", err)
		return "inbox"
	}
	for _, p := range projects {
		if p.ID == pid {
			return p.Name
		}
	}
	return "inbox"
}

// handleStart replies with a short description of what the bot does. The
// "/start login" deep link (used by the admin page's "Login with Telegram"
// button) is treated as a login request and issues a dashboard login link.
func (b *Bot) handleStart(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	if fields := strings.Fields(update.Message.Text); len(fields) > 1 && fields[1] == "login" {
		b.handleLogin(ctx, nil, update)
		return
	}
	text := "Send me text or photos and I'll file them as an issue. " +
		"They land in the inbox, and you can tag each one to a project with the buttons I add.\n\n" +
		"/last — show the most recent issues\n" +
		"/projects — list configured projects\n" +
		"/login — get an admin dashboard login link"
	b.reply(ctx, update.Message.Chat.ID, text)
}

// handleLast lists the most recent issues, defaulting to 5 and capping at 20.
func (b *Bot) handleLast(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	n := 5
	if fields := strings.Fields(update.Message.Text); len(fields) > 1 {
		if parsed, err := strconv.Atoi(fields[1]); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > 20 {
		n = 20
	}

	issues, err := b.store.ListIssues(ctx, store.ListIssuesParams{Limit: n})
	if err != nil {
		b.logger.Error("list issues", "error", err)
		b.reply(ctx, update.Message.Chat.ID, "Sorry, I couldn't load recent issues.")
		return
	}
	if len(issues) == 0 {
		b.reply(ctx, update.Message.Chat.ID, "No issues yet.")
		return
	}

	names := b.projectNames(ctx)
	var sb strings.Builder
	for i, iss := range issues {
		where := "inbox"
		if iss.ProjectID != nil {
			if name, ok := names[*iss.ProjectID]; ok {
				where = name
			}
		}
		fmt.Fprintf(&sb, "%d. <code>%s</code> • %s • %s\n",
			i+1, shortID(iss.ID), html.EscapeString(where), html.EscapeString(firstLine(iss.Body, 60)))
	}
	b.replyHTML(ctx, update.Message.Chat.ID, sb.String())
}

// handleProjects lists configured projects as "slug — name" bullets.
func (b *Bot) handleProjects(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	projects, err := b.store.ListProjects(ctx)
	if err != nil {
		b.logger.Error("list projects", "error", err)
		b.reply(ctx, update.Message.Chat.ID, "Sorry, I couldn't load projects.")
		return
	}
	if len(projects) == 0 {
		b.reply(ctx, update.Message.Chat.ID, "No projects configured yet.")
		return
	}

	var sb strings.Builder
	for _, p := range projects {
		fmt.Fprintf(&sb, "• <b>%s</b> — %s\n", html.EscapeString(p.Slug), html.EscapeString(p.Name))
	}
	b.replyHTML(ctx, update.Message.Chat.ID, sb.String())
}

// handleLogin issues a one-time admin dashboard login link. It only works in
// private chats for allowlisted admins.
func (b *Bot) handleLogin(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Chat.Type != models.ChatTypePrivate {
		return
	}
	from := update.Message.From
	if _, ok := b.resolveUser(ctx, from); !ok || from == nil || !b.admins[from.ID] {
		b.reply(ctx, update.Message.Chat.ID, "Only admins can access the dashboard.")
		return
	}
	if b.loginSvc == nil {
		b.reply(ctx, update.Message.Chat.ID, "Admin dashboard isn't configured on this server.")
		return
	}

	tgID := from.ID
	out, err := b.loginSvc.CreateLoginLink(ctx, authkit.CreateLoginLinkInput{
		Messenger: authkit.NewMessenger("telegram"),
		Audience:  "web",
		SubjectID: strconv.FormatInt(tgID, 10),
		Identity: &authkit.Identity{
			Messenger:       authkit.NewMessenger("telegram"),
			MessengerUserID: strconv.FormatInt(tgID, 10),
			Username:        from.Username,
			Name:            from.FirstName,
			Surname:         from.LastName,
		},
	})
	if err != nil {
		b.logger.Error("create login link", "tg_id", tgID, "error", err)
		b.reply(ctx, update.Message.Chat.ID, "Sorry, I couldn't create a login link.")
		return
	}

	// Send with the link preview disabled: Telegram's preview crawler fetches
	// URLs in outgoing messages, and that GET would redeem the one-time login
	// link before the admin ever clicks it, leaving them with an "invalid or
	// expired" page. IsDisabled stops the crawler from consuming the token.
	disabled := true
	if _, err := b.client.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:             update.Message.Chat.ID,
		Text:               fmt.Sprintf("Login link (valid briefly): %s", out.LoginURL),
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: &disabled},
	}); err != nil {
		b.logger.Error("send login link", "chat_id", update.Message.Chat.ID, "error", err)
	}
}

// projectNames maps project ids to display names, best-effort.
func (b *Bot) projectNames(ctx context.Context) map[int64]string {
	projects, err := b.store.ListProjects(ctx)
	if err != nil {
		b.logger.Error("list projects for names", "error", err)
		return nil
	}
	names := make(map[int64]string, len(projects))
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	return names
}

// --- small helpers ---

// shortID returns the first 8 characters of a uuid for compact display.
func shortID(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

// firstLine returns the first line of s, truncated to max runes.
func firstLine(s string, max int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if _, err := b.client.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		b.logger.Error("send message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) replyHTML(ctx context.Context, chatID int64, text string) {
	if _, err := b.client.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}); err != nil {
		b.logger.Error("send message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) replyMarkup(ctx context.Context, chatID int64, text string, kb *models.InlineKeyboardMarkup) {
	if _, err := b.client.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: kb,
	}); err != nil {
		b.logger.Error("send message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) answerCallback(ctx context.Context, callbackQueryID, text string) {
	if _, err := b.client.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	}); err != nil {
		b.logger.Error("answer callback", "error", err)
	}
}
