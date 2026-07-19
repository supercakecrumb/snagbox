// Package digest sends a once-a-day summary of the snagbox inbox to admins,
// so unsorted issues don't sit unnoticed between agent runs.
package digest

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Notifier delivers a plain-text message to a Telegram chat. Satisfied by
// *bot.Bot.
type Notifier interface {
	SendText(ctx context.Context, chatID int64, text string) error
}

// Counter reports how many issues are sitting unsorted in the inbox.
// Satisfied by *store.Store.
type Counter interface {
	InboxCount(ctx context.Context) (int, error)
}

// Digest periodically messages admins the current inbox backlog.
type Digest struct {
	notifier Notifier
	counter  Counter
	admins   []int64
	hour     int
	logger   *slog.Logger
	now      func() time.Time // injectable for tests
}

// New builds a Digest that fires daily at hour (local time, 0–23).
func New(notifier Notifier, counter Counter, admins []int64, hour int, logger *slog.Logger) *Digest {
	if logger == nil {
		logger = slog.Default()
	}
	return &Digest{
		notifier: notifier,
		counter:  counter,
		admins:   admins,
		hour:     hour,
		logger:   logger,
		now:      time.Now,
	}
}

// Run blocks until ctx is canceled, sending one digest at each occurrence of
// the configured hour.
func (d *Digest) Run(ctx context.Context) {
	for {
		wait := time.Until(d.nextRun(d.now()))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			d.sendOnce(ctx)
		}
	}
}

// nextRun returns the next time the digest hour occurs strictly after from.
func (d *Digest) nextRun(from time.Time) time.Time {
	next := time.Date(from.Year(), from.Month(), from.Day(), d.hour, 0, 0, 0, from.Location())
	if !next.After(from) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func (d *Digest) sendOnce(ctx context.Context) {
	n, err := d.counter.InboxCount(ctx)
	if err != nil {
		d.logger.Error("digest: count inbox failed", "error", err)
		return
	}
	if n == 0 {
		return
	}
	text := fmt.Sprintf("📥 %d issue(s) waiting in the snagbox inbox.", n)
	for _, id := range d.admins {
		if err := d.notifier.SendText(ctx, id, text); err != nil {
			d.logger.Error("digest: notify admin failed", "error", err, "admin_id", id)
		}
	}
}
