package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// QueueForAgent returns issues the agent is permitted to see and has not
// yet acked, oldest first, with their attachments attached.
func (s *Store) QueueForAgent(ctx context.Context, agentID int64, limit int) ([]QueueItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT i.id, i.project_id, i.source, i.author_user_id, i.body, i.meta, i.created_at
		FROM issues i
		WHERE NOT EXISTS (SELECT 1 FROM acks a WHERE a.agent_id = $1 AND a.issue_id = i.id)
		  AND EXISTS (
		    SELECT 1 FROM agent_project_perms p
		    WHERE p.agent_id = $1
		      AND (p.project_id = i.project_id OR (p.project_id IS NULL AND i.project_id IS NULL))
		  )
		ORDER BY i.created_at ASC
		LIMIT $2`, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("select queue: %w", err)
	}
	defer rows.Close()

	var (
		items []QueueItem
		ids   []uuid.UUID
	)
	for rows.Next() {
		var iss Issue
		if err := rows.Scan(&iss.ID, &iss.ProjectID, &iss.Source, &iss.AuthorUserID, &iss.Body, &iss.Meta, &iss.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan queue issue: %w", err)
		}
		items = append(items, QueueItem{Issue: iss, Attachments: []Attachment{}})
		ids = append(ids, iss.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue: %w", err)
	}
	if len(items) == 0 {
		return items, nil
	}

	byIssue, err := s.attachmentsForIssues(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if atts := byIssue[items[i].Issue.ID]; atts != nil {
			items[i].Attachments = atts
		}
	}
	return items, nil
}

func (s *Store) attachmentsForIssues(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]Attachment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, issue_id, s3_key, mime, size_bytes, filename
		FROM attachments WHERE issue_id = ANY($1) ORDER BY filename`, ids)
	if err != nil {
		return nil, fmt.Errorf("select queue attachments: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID][]Attachment)
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.IssueID, &a.S3Key, &a.Mime, &a.SizeBytes, &a.Filename); err != nil {
			return nil, fmt.Errorf("scan queue attachment: %w", err)
		}
		out[a.IssueID] = append(out[a.IssueID], a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue attachments: %w", err)
	}
	return out, nil
}

// AckIssue marks issueID acked for agentID. It is idempotent and returns
// ErrNotFound if the issue does not exist.
func (s *Store) AckIssue(ctx context.Context, agentID int64, issueID uuid.UUID) error {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM issues WHERE id = $1)`, issueID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check issue exists: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	_, err = s.Pool.Exec(ctx, `
		INSERT INTO acks (agent_id, issue_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, agentID, issueID)
	if err != nil {
		return fmt.Errorf("insert ack: %w", err)
	}
	return nil
}

// AckIssues bulk-acks issueIDs for agentID, silently skipping ids that do
// not exist. It is idempotent.
func (s *Store) AckIssues(ctx context.Context, agentID int64, issueIDs []uuid.UUID) error {
	if len(issueIDs) == 0 {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO acks (agent_id, issue_id)
		SELECT $1, id FROM issues WHERE id = ANY($2::uuid[])
		ON CONFLICT DO NOTHING`, agentID, issueIDs)
	if err != nil {
		return fmt.Errorf("insert acks: %w", err)
	}
	return nil
}
