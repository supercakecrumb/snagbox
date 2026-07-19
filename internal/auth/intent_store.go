// Package auth provides Postgres-backed adapters for the msgr-authkit
// bot-first login-link flow, an admin Telegram-ID allowlist, session-cookie
// middleware, and a CSRF token helper for the web admin UI.
package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	authkit "github.com/supercakecrumb/msgr-authkit"
)

// PGIntentStore is a Postgres-backed implementation of authkit.IntentStore.
type PGIntentStore struct {
	db *sql.DB
}

// NewPGIntentStore builds a PGIntentStore over the given database handle.
func NewPGIntentStore(db *sql.DB) *PGIntentStore {
	return &PGIntentStore{db: db}
}

// identityJSON is the on-disk representation of authkit.Identity stored in the
// auth_intents.identity_json column.
type identityJSON struct {
	MessengerID     string            `json:"messenger_id"`
	MessengerUserID string            `json:"messenger_user_id"`
	Username        string            `json:"username,omitempty"`
	Name            string            `json:"name,omitempty"`
	Surname         string            `json:"surname,omitempty"`
	BirthDate       *time.Time        `json:"birth_date,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
}

// encodeIdentity serializes an optional identity to a nullable JSON string.
func encodeIdentity(id *authkit.Identity) (sql.NullString, error) {
	if id == nil {
		return sql.NullString{}, nil
	}
	payload := identityJSON{
		MessengerID:     id.Messenger.ID,
		MessengerUserID: id.MessengerUserID,
		Username:        id.Username,
		Name:            id.Name,
		Surname:         id.Surname,
		BirthDate:       id.BirthDate,
		Attributes:      id.Attributes,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("auth: encode identity: %w", err)
	}
	return sql.NullString{String: string(raw), Valid: true}, nil
}

// decodeIdentity deserializes a nullable JSON string back into an identity.
func decodeIdentity(s sql.NullString) (*authkit.Identity, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	var payload identityJSON
	if err := json.Unmarshal([]byte(s.String), &payload); err != nil {
		return nil, fmt.Errorf("auth: decode identity: %w", err)
	}
	id := &authkit.Identity{
		Messenger:       authkit.NewMessenger(payload.MessengerID),
		MessengerUserID: payload.MessengerUserID,
		Username:        payload.Username,
		Name:            payload.Name,
		Surname:         payload.Surname,
		BirthDate:       payload.BirthDate,
		Attributes:      payload.Attributes,
	}
	return id, nil
}

// encodeMetadata serializes an optional key-value map to a nullable JSON string.
func encodeMetadata(m map[string]string) (sql.NullString, error) {
	if len(m) == 0 {
		return sql.NullString{}, nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("auth: encode metadata: %w", err)
	}
	return sql.NullString{String: string(raw), Valid: true}, nil
}

// decodeMetadata deserializes a nullable JSON string back into a key-value map.
func decodeMetadata(s sql.NullString) (map[string]string, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s.String), &m); err != nil {
		return nil, fmt.Errorf("auth: decode metadata: %w", err)
	}
	return m, nil
}

// intentRowScanner abstracts *sql.Row and *sql.Rows so scanIntent can be reused
// for single-row and multi-row queries.
type intentRowScanner interface {
	Scan(dest ...any) error
}

// scanIntent reads a full auth_intents row into an authkit.AuthIntent.
func scanIntent(row intentRowScanner) (authkit.AuthIntent, error) {
	var (
		intent     authkit.AuthIntent
		messenger  string
		state      string
		mode       string
		identity   sql.NullString
		metadata   sql.NullString
		expiresAt  sql.NullTime
		consumedAt sql.NullTime
	)
	if err := row.Scan(
		&intent.ID,
		&intent.Code,
		&messenger,
		&intent.Audience,
		&intent.SubjectID,
		&state,
		&identity,
		&metadata,
		&mode,
		&intent.MaxRedemptions,
		&intent.RedemptionCount,
		&expiresAt,
		&intent.CreatedAt,
		&consumedAt,
	); err != nil {
		return authkit.AuthIntent{}, err
	}

	intent.Messenger = authkit.NewMessenger(messenger)
	intent.State = authkit.IntentState(state)
	intent.RedemptionMode = authkit.IntentRedemptionMode(mode)

	id, err := decodeIdentity(identity)
	if err != nil {
		return authkit.AuthIntent{}, err
	}
	intent.Identity = id

	meta, err := decodeMetadata(metadata)
	if err != nil {
		return authkit.AuthIntent{}, err
	}
	intent.Metadata = meta

	if expiresAt.Valid {
		t := expiresAt.Time
		intent.ExpiresAt = &t
	}
	if consumedAt.Valid {
		t := consumedAt.Time
		intent.ConsumedAt = &t
	}
	return intent, nil
}

// Create inserts a new auth intent.
func (s *PGIntentStore) Create(ctx context.Context, intent authkit.AuthIntent) error {
	identity, err := encodeIdentity(intent.Identity)
	if err != nil {
		return err
	}
	metadata, err := encodeMetadata(intent.Metadata)
	if err != nil {
		return err
	}
	var expiresAt sql.NullTime
	if intent.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: *intent.ExpiresAt, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO auth_intents (
			id, code, messenger, audience, subject_id, state,
			identity_json, metadata_json, redemption_mode,
			max_redemptions, redemption_count, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		intent.ID,
		intent.Code,
		intent.Messenger.ID,
		intent.Audience,
		intent.SubjectID,
		string(intent.State),
		identity,
		metadata,
		string(intent.RedemptionMode),
		intent.MaxRedemptions,
		intent.RedemptionCount,
		expiresAt,
		intent.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("auth: create intent: %w", err)
	}
	return nil
}

// FindByCode returns the intent identified by the messenger+code pair.
func (s *PGIntentStore) FindByCode(ctx context.Context, messenger authkit.Messenger, code string) (authkit.AuthIntent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, messenger, audience, subject_id, state,
		       identity_json, metadata_json, redemption_mode,
		       max_redemptions, redemption_count, expires_at, created_at, consumed_at
		FROM auth_intents
		WHERE messenger = $1 AND code = $2`,
		messenger.ID, code)

	intent, err := scanIntent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return authkit.AuthIntent{}, authkit.ErrIntentNotFound
	}
	if err != nil {
		return authkit.AuthIntent{}, fmt.Errorf("auth: find intent by code: %w", err)
	}
	return intent, nil
}

// RecordRedemption applies the redemption state transition atomically, using a
// compare-and-swap UPDATE to remain safe under concurrent redemptions.
func (s *PGIntentStore) RecordRedemption(ctx context.Context, intentID string, redeemedAt time.Time) error {
	now := redeemedAt.UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: begin redemption tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		state      string
		mode       string
		maxRedempt int
		count      int
		expiresAt  sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `
		SELECT state, redemption_mode, max_redemptions, redemption_count, expires_at
		FROM auth_intents WHERE id = $1`, intentID).
		Scan(&state, &mode, &maxRedempt, &count, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return authkit.ErrIntentNotFound
	}
	if err != nil {
		return fmt.Errorf("auth: load intent for redemption: %w", err)
	}

	// Precedence: a non-active intent is reported as ErrIntentNotActive before
	// we consider expiration, and expiration is reported (after lazily marking
	// the row 'expired') before the mode-specific "already redeemed" / "limit
	// reached" checks. This mirrors the in-memory reference ordering so callers
	// see a stable error regardless of the backing store.
	if state != string(authkit.IntentActive) {
		return authkit.ErrIntentNotActive
	}

	if expiresAt.Valid && now.After(expiresAt.Time.UTC()) {
		if _, err := tx.ExecContext(ctx,
			`UPDATE auth_intents SET state = 'expired' WHERE id = $1`, intentID); err != nil {
			return fmt.Errorf("auth: mark intent expired: %w", err)
		}
		// Commit the lazy expiration so the state change is durable even though
		// this redemption failed.
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("auth: commit expired intent: %w", err)
		}
		return authkit.ErrIntentExpired
	}

	switch authkit.IntentRedemptionMode(mode) {
	case authkit.IntentOneTime:
		if count >= 1 {
			return authkit.ErrIntentAlreadyRedeemed
		}
	case authkit.IntentReusable:
		if maxRedempt > 0 && count >= maxRedempt {
			return authkit.ErrIntentRedemptionLimitReached
		}
	default:
		return fmt.Errorf("auth: unsupported redemption mode %q", mode)
	}

	newCount := count + 1
	newState := state
	if authkit.IntentRedemptionMode(mode) == authkit.IntentOneTime {
		newState = string(authkit.IntentRevoked)
	}
	if authkit.IntentRedemptionMode(mode) == authkit.IntentReusable && maxRedempt > 0 && newCount >= maxRedempt {
		newState = string(authkit.IntentRevoked)
	}

	// CAS: only apply if the row still has the redemption_count we read and is
	// still active. If another concurrent redemption won the race the guard
	// fails and no rows are affected.
	res, err := tx.ExecContext(ctx, `
		UPDATE auth_intents
		SET redemption_count = $1, consumed_at = $2, state = $3
		WHERE id = $4 AND redemption_count = $5 AND state = 'active'`,
		newCount, now, newState, intentID, count)
	if err != nil {
		return fmt.Errorf("auth: record redemption: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: redemption rows affected: %w", err)
	}
	if n == 0 {
		// Lost the CAS race: a concurrent redemption already advanced the
		// counter, so this attempt is an already-redeemed intent.
		return authkit.ErrIntentAlreadyRedeemed
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth: commit redemption: %w", err)
	}
	return nil
}

// DeleteExpired removes intents whose expiration passed before now.
func (s *PGIntentStore) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM auth_intents WHERE expires_at IS NOT NULL AND expires_at < $1`,
		now.UTC())
	if err != nil {
		return fmt.Errorf("auth: delete expired intents: %w", err)
	}
	return nil
}
