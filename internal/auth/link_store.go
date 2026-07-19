package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	authkit "github.com/supercakecrumb/msgr-authkit"
)

// PGLinkStore is a Postgres-backed implementation of authkit.LinkStore.
type PGLinkStore struct {
	db *sql.DB
}

// NewPGLinkStore builds a PGLinkStore over the given database handle.
func NewPGLinkStore(db *sql.DB) *PGLinkStore {
	return &PGLinkStore{db: db}
}

// FindByIdentity resolves the internal account link for an external identity.
func (s *PGLinkStore) FindByIdentity(ctx context.Context, identity authkit.Identity) (authkit.AccountLink, error) {
	var (
		appUserID  string
		username   sql.NullString
		name       sql.NullString
		surname    sql.NullString
		birthDate  sql.NullTime
		attributes sql.NullString
		linkedAt   time.Time
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT app_user_id, username, name, surname, birth_date, attributes_json, linked_at
		FROM account_links
		WHERE messenger = $1 AND messenger_user_id = $2`,
		identity.Messenger.ID, identity.MessengerUserID).
		Scan(&appUserID, &username, &name, &surname, &birthDate, &attributes, &linkedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return authkit.AccountLink{}, authkit.ErrIdentityLinkNotFound
	}
	if err != nil {
		return authkit.AccountLink{}, fmt.Errorf("auth: find account link: %w", err)
	}

	attrs, err := decodeMetadata(attributes)
	if err != nil {
		return authkit.AccountLink{}, err
	}

	link := authkit.AccountLink{
		AppUserID: appUserID,
		Identity: authkit.Identity{
			Messenger:       authkit.NewMessenger(identity.Messenger.ID),
			MessengerUserID: identity.MessengerUserID,
			Username:        username.String,
			Name:            name.String,
			Surname:         surname.String,
			Attributes:      attrs,
		},
		LinkedAt: linkedAt,
	}
	if birthDate.Valid {
		t := birthDate.Time
		link.Identity.BirthDate = &t
	}
	return link, nil
}

// Upsert creates or refreshes an identity->app-user mapping.
func (s *PGLinkStore) Upsert(ctx context.Context, link authkit.AccountLink) error {
	attributes, err := encodeMetadata(link.Identity.Attributes)
	if err != nil {
		return err
	}
	var birthDate sql.NullTime
	if link.Identity.BirthDate != nil {
		birthDate = sql.NullTime{Time: *link.Identity.BirthDate, Valid: true}
	}
	// Default a zero linked_at to now so the NOT NULL column is always set.
	linkedAt := link.LinkedAt
	if linkedAt.IsZero() {
		linkedAt = time.Now().UTC()
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO account_links (
			app_user_id, messenger, messenger_user_id, username, name,
			surname, birth_date, attributes_json, linked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (messenger, messenger_user_id) DO UPDATE SET
			app_user_id = excluded.app_user_id,
			username = excluded.username,
			name = excluded.name,
			surname = excluded.surname,
			birth_date = excluded.birth_date,
			attributes_json = excluded.attributes_json,
			linked_at = excluded.linked_at`,
		link.AppUserID,
		link.Identity.Messenger.ID,
		link.Identity.MessengerUserID,
		link.Identity.Username,
		link.Identity.Name,
		link.Identity.Surname,
		birthDate,
		attributes,
		linkedAt,
	)
	if err != nil {
		return fmt.Errorf("auth: upsert account link: %w", err)
	}
	return nil
}
