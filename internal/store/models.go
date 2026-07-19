package store

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned by single-row lookups when no row matches.
var ErrNotFound = errors.New("store: not found")

// User is a Telegram-authenticated person with an admin or member role.
type User struct {
	ID         int64
	TelegramID int64
	Name       string
	Role       string
	CreatedAt  time.Time
}

// Project groups issues under a stable slug.
type Project struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	CreatedAt   time.Time
}

// Agent is a machine consumer that drains issues via a bearer token.
type Agent struct {
	ID         int64
	Name       string
	TokenHash  string
	CreatedBy  *int64
	CreatedAt  time.Time
	LastSeenAt *time.Time
}

// IngestToken authorizes API submissions, optionally scoped to a project.
type IngestToken struct {
	ID        int64
	TokenHash string
	Label     string
	ProjectID *int64
	CreatedAt time.Time
}

// Issue is a captured report. A nil ProjectID means the inbox.
type Issue struct {
	ID           uuid.UUID
	ProjectID    *int64
	Source       string
	AuthorUserID *int64
	Body         string
	Meta         map[string]any
	CreatedAt    time.Time
}

// Attachment is a file stored in blob storage and linked to an issue.
type Attachment struct {
	ID        uuid.UUID
	IssueID   uuid.UUID
	S3Key     string
	Mime      string
	SizeBytes int64
	Filename  string
}

// QueueItem pairs an issue with its attachments for agent delivery.
type QueueItem struct {
	Issue       Issue
	Attachments []Attachment
}
