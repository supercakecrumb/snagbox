package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// --- Users ---

// CreateUser inserts a user and returns the stored row.
func (s *Store) CreateUser(ctx context.Context, telegramID int64, name, role string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name, role)
		VALUES ($1, $2, $3)
		RETURNING id, telegram_id, name, role, created_at`,
		telegramID, name, role).
		Scan(&u.ID, &u.TelegramID, &u.Name, &u.Role, &u.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

// GetUserByTelegramID looks up a user by Telegram id.
func (s *Store) GetUserByTelegramID(ctx context.Context, tgID int64) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, telegram_id, name, role, created_at
		FROM users WHERE telegram_id = $1`, tgID).
		Scan(&u.ID, &u.TelegramID, &u.Name, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by telegram id: %w", err)
	}
	return u, nil
}

// ListUsers returns all users ordered by id.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, telegram_id, name, role, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("select users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return out, nil
}

// DeleteUser removes a user by id.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// --- Projects ---

// CreateProject inserts a project and returns the stored row.
func (s *Store) CreateProject(ctx context.Context, slug, name, description string) (Project, error) {
	var p Project
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO projects (slug, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, slug, name, description, created_at`,
		slug, name, description).
		Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}
	return p, nil
}

// GetProjectBySlug looks up a project by slug.
func (s *Store) GetProjectBySlug(ctx context.Context, slug string) (Project, error) {
	var p Project
	err := s.Pool.QueryRow(ctx, `
		SELECT id, slug, name, description, created_at
		FROM projects WHERE slug = $1`, slug).
		Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("select project by slug: %w", err)
	}
	return p, nil
}

// ListProjects returns all projects ordered by id.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, slug, name, description, created_at FROM projects ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("select projects: %w", err)
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return out, nil
}

// DeleteProject removes a project by id.
func (s *Store) DeleteProject(ctx context.Context, id int64) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

// --- Agents ---

// CreateAgent inserts an agent and returns the stored row.
func (s *Store) CreateAgent(ctx context.Context, name, tokenHash string, createdBy *int64) (Agent, error) {
	var a Agent
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO agents (name, token_hash, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, name, token_hash, created_by, created_at, last_seen_at`,
		name, tokenHash, createdBy).
		Scan(&a.ID, &a.Name, &a.TokenHash, &a.CreatedBy, &a.CreatedAt, &a.LastSeenAt)
	if err != nil {
		return Agent{}, fmt.Errorf("insert agent: %w", err)
	}
	return a, nil
}

// ListAgents returns all agents ordered by id.
func (s *Store) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, token_hash, created_by, created_at, last_seen_at
		FROM agents ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("select agents: %w", err)
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.TokenHash, &a.CreatedBy, &a.CreatedAt, &a.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return out, nil
}

// DeleteAgent removes an agent by id.
func (s *Store) DeleteAgent(ctx context.Context, id int64) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

// SetAgentPerms replaces all permission rows for the agent. Each projectID
// becomes a scoped row; inbox adds a NULL project_id row.
func (s *Store) SetAgentPerms(ctx context.Context, agentID int64, projectIDs []int64, inbox bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set agent perms: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if _, err := tx.Exec(ctx, `DELETE FROM agent_project_perms WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("clear agent perms: %w", err)
	}
	for _, pid := range projectIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_project_perms (agent_id, project_id) VALUES ($1, $2)`,
			agentID, pid); err != nil {
			return fmt.Errorf("insert agent perm: %w", err)
		}
	}
	if inbox {
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_project_perms (agent_id, project_id) VALUES ($1, NULL)`,
			agentID); err != nil {
			return fmt.Errorf("insert agent inbox perm: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set agent perms: %w", err)
	}
	return nil
}

// ListAgentPerms returns the agent's scoped project ids and whether it has
// the inbox permission.
func (s *Store) ListAgentPerms(ctx context.Context, agentID int64) (projectIDs []int64, inbox bool, err error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT project_id FROM agent_project_perms WHERE agent_id = $1 ORDER BY project_id`, agentID)
	if err != nil {
		return nil, false, fmt.Errorf("select agent perms: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pid *int64
		if err := rows.Scan(&pid); err != nil {
			return nil, false, fmt.Errorf("scan agent perm: %w", err)
		}
		if pid == nil {
			inbox = true
			continue
		}
		projectIDs = append(projectIDs, *pid)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate agent perms: %w", err)
	}
	return projectIDs, inbox, nil
}

// --- Ingest tokens ---

// CreateIngestToken inserts an ingest token and returns the stored row.
func (s *Store) CreateIngestToken(ctx context.Context, tokenHash, label string, projectID *int64) (IngestToken, error) {
	var t IngestToken
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO ingest_tokens (token_hash, label, project_id)
		VALUES ($1, $2, $3)
		RETURNING id, token_hash, label, project_id, created_at`,
		tokenHash, label, projectID).
		Scan(&t.ID, &t.TokenHash, &t.Label, &t.ProjectID, &t.CreatedAt)
	if err != nil {
		return IngestToken{}, fmt.Errorf("insert ingest token: %w", err)
	}
	return t, nil
}

// ListIngestTokens returns all ingest tokens ordered by id.
func (s *Store) ListIngestTokens(ctx context.Context) ([]IngestToken, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, token_hash, label, project_id, created_at
		FROM ingest_tokens ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("select ingest tokens: %w", err)
	}
	defer rows.Close()

	var out []IngestToken
	for rows.Next() {
		var t IngestToken
		if err := rows.Scan(&t.ID, &t.TokenHash, &t.Label, &t.ProjectID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ingest token: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ingest tokens: %w", err)
	}
	return out, nil
}

// DeleteIngestToken removes an ingest token by id.
func (s *Store) DeleteIngestToken(ctx context.Context, id int64) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM ingest_tokens WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete ingest token: %w", err)
	}
	return nil
}
