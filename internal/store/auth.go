package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AgentByTokenHash looks up an agent by its token hash.
func (s *Store) AgentByTokenHash(ctx context.Context, hash string) (Agent, error) {
	var a Agent
	err := s.Pool.QueryRow(ctx, `
		SELECT id, name, token_hash, created_by, created_at, last_seen_at
		FROM agents WHERE token_hash = $1`, hash).
		Scan(&a.ID, &a.Name, &a.TokenHash, &a.CreatedBy, &a.CreatedAt, &a.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, fmt.Errorf("select agent by token hash: %w", err)
	}
	return a, nil
}

// TouchAgent records that the agent was just seen.
func (s *Store) TouchAgent(ctx context.Context, agentID int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agents SET last_seen_at = now() WHERE id = $1`, agentID)
	if err != nil {
		return fmt.Errorf("touch agent: %w", err)
	}
	return nil
}

// IngestTokenByHash looks up an ingest token by its hash.
func (s *Store) IngestTokenByHash(ctx context.Context, hash string) (IngestToken, error) {
	var t IngestToken
	err := s.Pool.QueryRow(ctx, `
		SELECT id, token_hash, label, project_id, created_at
		FROM ingest_tokens WHERE token_hash = $1`, hash).
		Scan(&t.ID, &t.TokenHash, &t.Label, &t.ProjectID, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return IngestToken{}, ErrNotFound
	}
	if err != nil {
		return IngestToken{}, fmt.Errorf("select ingest token by hash: %w", err)
	}
	return t, nil
}
