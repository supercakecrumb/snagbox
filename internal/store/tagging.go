package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SetIssueProject moves an issue to projectID (nil = inbox). Returns
// ErrNotFound if the issue does not exist.
func (s *Store) SetIssueProject(ctx context.Context, issueID uuid.UUID, projectID *int64) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE issues SET project_id = $2 WHERE id = $1`, issueID, projectID)
	if err != nil {
		return fmt.Errorf("update issue project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetIssueProjectForUser limits Telegram callbacks to the issue author or a configured admin.
func (s *Store) SetIssueProjectForUser(ctx context.Context, issueID uuid.UUID, projectID *int64, userID int64, admin bool) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE issues SET project_id = $2 WHERE id = $1 AND (author_user_id = $3 OR $4)`, issueID, projectID, userID, admin)
	if err != nil {
		return fmt.Errorf("update authorized issue project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
