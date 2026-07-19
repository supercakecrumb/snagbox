package web

import (
	"net/http"

	"github.com/supercakecrumb/snagbox/internal/store"
)

// agentStat pairs an agent with its current unacked queue depth.
type agentStat struct {
	Agent store.Agent
	Depth int
}

// handleDashboard renders headline counts and a per-agent queue-depth overview.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	inbox, err := s.store.InboxCount(ctx)
	if err != nil {
		s.serverError(w, "count inbox", err)
		return
	}
	issues, err := s.store.IssueCount(ctx)
	if err != nil {
		s.serverError(w, "count issues", err)
		return
	}
	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		s.serverError(w, "list projects", err)
		return
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		s.serverError(w, "list users", err)
		return
	}
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		s.serverError(w, "list agents", err)
		return
	}

	stats := make([]agentStat, 0, len(agents))
	for _, a := range agents {
		depth, err := s.store.AgentQueueDepth(ctx, a.ID)
		if err != nil {
			s.serverError(w, "agent queue depth", err)
			return
		}
		stats = append(stats, agentStat{Agent: a, Depth: depth})
	}

	s.render(w, "dashboard", map[string]any{
		"Nav":          "dashboard",
		"CSRF":         s.csrfToken(r),
		"InboxCount":   inbox,
		"IssueCount":   issues,
		"ProjectCount": len(projects),
		"UserCount":    len(users),
		"AgentCount":   len(agents),
		"Agents":       stats,
	})
}

// serverError logs the underlying error and returns a generic 500 without
// leaking internal detail to the client.
func (s *Server) serverError(w http.ResponseWriter, msg string, err error) {
	s.logger.Error(msg, "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
