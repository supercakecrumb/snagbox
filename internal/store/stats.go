package store

import (
	"context"
	"fmt"
)

// InboxCount returns the number of issues that have not been tagged to a
// project (project_id IS NULL).
func (s *Store) InboxCount(ctx context.Context) (int, error) {
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM issues WHERE project_id IS NULL`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count inbox issues: %w", err)
	}
	return n, nil
}

// IssueCount returns the total number of issues.
func (s *Store) IssueCount(ctx context.Context) (int, error) {
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM issues`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count issues: %w", err)
	}
	return n, nil
}

// AgentQueueDepth returns how many issues the agent is permitted to see and has
// not yet acked. It reuses the exact predicate from QueueForAgent.
func (s *Store) AgentQueueDepth(ctx context.Context, agentID int64) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*)
		FROM issues i
		WHERE NOT EXISTS (SELECT 1 FROM acks a WHERE a.agent_id = $1 AND a.issue_id = i.id)
		  AND EXISTS (
		    SELECT 1 FROM agent_project_perms p
		    WHERE p.agent_id = $1
		      AND (p.project_id = i.project_id OR (p.project_id IS NULL AND i.project_id IS NULL))
		  )`, agentID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count agent queue depth: %w", err)
	}
	return n, nil
}
