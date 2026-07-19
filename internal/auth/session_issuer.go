package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	authkit "github.com/supercakecrumb/msgr-authkit"
)

// PGSessionIssuer is a Postgres-backed implementation of authkit.SessionIssuer.
// Session tokens are stored only as SHA-256 hashes, never in plaintext.
type PGSessionIssuer struct {
	db  *sql.DB
	ttl time.Duration
}

// NewPGSessionIssuer builds a PGSessionIssuer over the given database handle
// with the provided session time-to-live.
func NewPGSessionIssuer(db *sql.DB, ttl time.Duration) *PGSessionIssuer {
	return &PGSessionIssuer{db: db, ttl: ttl}
}

// Issue creates a new web session for appUserID, persisting only the token
// hash, and returns the session with its plaintext token.
func (s *PGSessionIssuer) Issue(ctx context.Context, appUserID string) (authkit.WebSession, error) {
	token, err := randomToken(32)
	if err != nil {
		return authkit.WebSession{}, fmt.Errorf("auth: generate session token: %w", err)
	}

	now := time.Now().UTC()
	session := authkit.WebSession{
		SessionID: uuid.NewString(),
		SubjectID: appUserID,
		Token:     token,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.ttl),
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO web_sessions (session_id, subject_id, token_hash, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		session.SessionID, session.SubjectID, hashToken(token), session.IssuedAt, session.ExpiresAt)
	if err != nil {
		return authkit.WebSession{}, fmt.Errorf("auth: insert web session: %w", err)
	}
	return session, nil
}

// Validate resolves a session by its token, returning authkit.ErrSessionNotFound
// when unknown and authkit.ErrSessionExpired when past its expiration.
func (s *PGSessionIssuer) Validate(ctx context.Context, token string) (authkit.WebSession, error) {
	var session authkit.WebSession
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, subject_id, issued_at, expires_at
		FROM web_sessions WHERE token_hash = $1`,
		hashToken(token)).
		Scan(&session.SessionID, &session.SubjectID, &session.IssuedAt, &session.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return authkit.WebSession{}, authkit.ErrSessionNotFound
	}
	if err != nil {
		return authkit.WebSession{}, fmt.Errorf("auth: select web session: %w", err)
	}

	session.Token = token
	if time.Now().UTC().After(session.ExpiresAt) {
		return authkit.WebSession{}, authkit.ErrSessionExpired
	}
	return session, nil
}

// Revoke deletes the session identified by token.
func (s *PGSessionIssuer) Revoke(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM web_sessions WHERE token_hash = $1`, hashToken(token))
	if err != nil {
		return fmt.Errorf("auth: delete web session: %w", err)
	}
	return nil
}

// hashToken returns the hex-encoded SHA-256 of a session token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// randomToken returns the hex encoding of n cryptographically random bytes.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
