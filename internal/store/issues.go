package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateIssue inserts iss using its caller-provided ID. A nil Meta is
// stored as an empty JSON object.
func (s *Store) CreateIssue(ctx context.Context, iss Issue) error {
	meta := iss.Meta
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO issues (id, project_id, source, author_user_id, body, meta, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		iss.ID, iss.ProjectID, iss.Source, iss.AuthorUserID, iss.Body, meta, iss.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert issue: %w", err)
	}
	return nil
}

// AddAttachment inserts a.
func (s *Store) AddAttachment(ctx context.Context, a Attachment) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO attachments (id, issue_id, s3_key, mime, size_bytes, filename)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ID, a.IssueID, a.S3Key, a.Mime, a.SizeBytes, a.Filename)
	if err != nil {
		return fmt.Errorf("insert attachment: %w", err)
	}
	return nil
}

// GetIssue loads an issue and its attachments ordered by filename.
func (s *Store) GetIssue(ctx context.Context, id uuid.UUID) (Issue, []Attachment, error) {
	var iss Issue
	err := s.Pool.QueryRow(ctx, `
		SELECT id, project_id, source, author_user_id, body, meta, created_at
		FROM issues WHERE id = $1`, id).
		Scan(&iss.ID, &iss.ProjectID, &iss.Source, &iss.AuthorUserID, &iss.Body, &iss.Meta, &iss.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Issue{}, nil, ErrNotFound
	}
	if err != nil {
		return Issue{}, nil, fmt.Errorf("select issue: %w", err)
	}

	atts, err := s.attachmentsByIssue(ctx, id)
	if err != nil {
		return Issue{}, nil, err
	}
	return iss, atts, nil
}

func (s *Store) attachmentsByIssue(ctx context.Context, issueID uuid.UUID) ([]Attachment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, issue_id, s3_key, mime, size_bytes, filename
		FROM attachments WHERE issue_id = $1 ORDER BY filename`, issueID)
	if err != nil {
		return nil, fmt.Errorf("select attachments: %w", err)
	}
	defer rows.Close()

	var out []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.IssueID, &a.S3Key, &a.Mime, &a.SizeBytes, &a.Filename); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return out, nil
}

// GetAttachment loads a single attachment by id.
func (s *Store) GetAttachment(ctx context.Context, id uuid.UUID) (Attachment, error) {
	var a Attachment
	err := s.Pool.QueryRow(ctx, `
		SELECT id, issue_id, s3_key, mime, size_bytes, filename
		FROM attachments WHERE id = $1`, id).
		Scan(&a.ID, &a.IssueID, &a.S3Key, &a.Mime, &a.SizeBytes, &a.Filename)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("select attachment: %w", err)
	}
	return a, nil
}

// ListIssuesParams filters a browse query over issues.
type ListIssuesParams struct {
	// AgentID limits machine-client browsing to current project permissions.
	AgentID   *int64
	ProjectID *int64
	InboxOnly bool
	Since     *time.Time
	Limit     int
}

// ListIssues returns issues newest first, applying the project/inbox and
// since filters. Attachments are not loaded.
func (s *Store) ListIssues(ctx context.Context, p ListIssuesParams) ([]Issue, error) {
	var (
		conds []string
		args  []any
	)
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, strings.Replace(cond, "$?", "$"+strconv.Itoa(len(args)), 1))
	}

	if p.AgentID != nil {
		add("EXISTS (SELECT 1 FROM agent_project_perms perm WHERE perm.agent_id = $? AND perm.project_id IS NOT DISTINCT FROM issues.project_id)", *p.AgentID)
	}
	switch {
	case p.InboxOnly:
		conds = append(conds, "project_id IS NULL")
	case p.ProjectID != nil:
		add("project_id = $?", *p.ProjectID)
	}
	if p.Since != nil {
		add("created_at >= $?", *p.Since)
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	args = append(args, limit)

	q := `SELECT id, project_id, source, author_user_id, body, meta, created_at FROM issues`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args))

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("select issues: %w", err)
	}
	defer rows.Close()

	var out []Issue
	for rows.Next() {
		var iss Issue
		if err := rows.Scan(&iss.ID, &iss.ProjectID, &iss.Source, &iss.AuthorUserID, &iss.Body, &iss.Meta, &iss.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		out = append(out, iss)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issues: %w", err)
	}
	return out, nil
}

// GetAttachmentForAgent authorizes against the attachment's issue before returning storage metadata.
func (s *Store) GetAttachmentForAgent(ctx context.Context, id uuid.UUID, agentID int64) (Attachment, error) {
	var a Attachment
	err := s.Pool.QueryRow(ctx, `
  SELECT a.id, a.issue_id, a.s3_key, a.mime, a.size_bytes, a.filename
  FROM attachments a JOIN issues i ON i.id = a.issue_id
  WHERE a.id = $1 AND EXISTS (
   SELECT 1 FROM agent_project_perms p WHERE p.agent_id = $2
    AND p.project_id IS NOT DISTINCT FROM i.project_id
  )`, id, agentID).Scan(&a.ID, &a.IssueID, &a.S3Key, &a.Mime, &a.SizeBytes, &a.Filename)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("select authorized attachment: %w", err)
	}
	return a, nil
}
