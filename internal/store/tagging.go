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
