package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/supercakecrumb/snagbox/internal/store"
	"github.com/supercakecrumb/snagbox/internal/token"
)

// agentRow pairs an agent with its current queue depth for the list view.
type agentRow struct {
	Agent store.Agent
	Depth int
}

// agentListData builds the shared data used to render the agents list, minus
// any one-time-token fields the caller adds.
func (s *Server) agentListData(r *http.Request) (map[string]any, error) {
	agents, err := s.store.ListAgents(r.Context())
	if err != nil {
		return nil, err
	}
	rows := make([]agentRow, 0, len(agents))
	for _, a := range agents {
		depth, err := s.store.AgentQueueDepth(r.Context(), a.ID)
		if err != nil {
			return nil, err
		}
		rows = append(rows, agentRow{Agent: a, Depth: depth})
	}
	return map[string]any{
		"Nav":    "agents",
		"CSRF":   s.csrfToken(r),
		"Agents": rows,
	}, nil
}

// handleAgents lists all agents with their queue depths.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	data, err := s.agentListData(r)
	if err != nil {
		s.serverError(w, "list agents", err)
		return
	}
	s.render(w, "agents", data)
}

// handleAgentCreate creates an agent, generating a bearer token that is shown
// exactly once by re-rendering the list (never stored in plaintext).
func (s *Server) handleAgentCreate(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	plain, hash := token.New()
	if _, err := s.store.CreateAgent(r.Context(), name, hash, nil); err != nil {
		s.serverError(w, "create agent", err)
		return
	}

	data, err := s.agentListData(r)
	if err != nil {
		s.serverError(w, "list agents", err)
		return
	}
	data["NewToken"] = plain
	data["NewTokenName"] = name
	s.render(w, "agents", data)
}

// projectPerm describes a project checkbox on the agent detail page.
type projectPerm struct {
	Project store.Project
	Checked bool
}

// handleAgentDetail renders per-project permission checkboxes for an agent,
// pre-checked from its current permissions.
func (s *Server) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	agents, err := s.store.ListAgents(r.Context())
	if err != nil {
		s.serverError(w, "list agents", err)
		return
	}
	var agent store.Agent
	found := false
	for _, a := range agents {
		if a.ID == id {
			agent = a
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		s.serverError(w, "list projects", err)
		return
	}
	projectIDs, inbox, err := s.store.ListAgentPerms(r.Context(), id)
	if err != nil {
		s.serverError(w, "list agent perms", err)
		return
	}
	checked := make(map[int64]bool, len(projectIDs))
	for _, pid := range projectIDs {
		checked[pid] = true
	}

	perms := make([]projectPerm, 0, len(projects))
	for _, p := range projects {
		perms = append(perms, projectPerm{Project: p, Checked: checked[p.ID]})
	}

	s.render(w, "agent_detail", map[string]any{
		"Nav":   "agents",
		"CSRF":  s.csrfToken(r),
		"Agent": agent,
		"Perms": perms,
		"Inbox": inbox,
	})
}

// handleAgentPerms replaces an agent's project/inbox permissions (PRG).
func (s *Server) handleAgentPerms(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	var projectIDs []int64
	for _, raw := range r.Form["project_ids"] {
		pid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			http.Error(w, "invalid project id", http.StatusBadRequest)
			return
		}
		projectIDs = append(projectIDs, pid)
	}
	inbox := r.FormValue("inbox") != ""

	if err := s.store.SetAgentPerms(r.Context(), id, projectIDs, inbox); err != nil {
		s.serverError(w, "set agent perms", err)
		return
	}
	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10), http.StatusFound)
}

// handleAgentDelete removes an agent by id (PRG).
func (s *Server) handleAgentDelete(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteAgent(r.Context(), id); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, "delete agent", err)
		return
	}
	http.Redirect(w, r, "/agents", http.StatusFound)
}
