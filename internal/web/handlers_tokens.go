package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/supercakecrumb/snagbox/internal/store"
	"github.com/supercakecrumb/snagbox/internal/token"
)

// tokenRow pairs an ingest token with its resolved project label for display.
type tokenRow struct {
	Token   store.IngestToken
	Project string // project name, or "inbox" when unscoped
}

// tokenListData builds the shared data used to render the ingest-token list.
func (s *Server) tokenListData(r *http.Request) (map[string]any, error) {
	tokens, err := s.store.ListIngestTokens(r.Context())
	if err != nil {
		return nil, err
	}
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(projects))
	for _, p := range projects {
		names[p.ID] = p.Name
	}

	rows := make([]tokenRow, 0, len(tokens))
	for _, t := range tokens {
		label := "inbox"
		if t.ProjectID != nil {
			if name, ok := names[*t.ProjectID]; ok {
				label = name
			}
		}
		rows = append(rows, tokenRow{Token: t, Project: label})
	}
	return map[string]any{
		"Nav":      "tokens",
		"CSRF":     s.csrfToken(r),
		"Tokens":   rows,
		"Projects": projects,
	}, nil
}

// handleTokens lists all ingest tokens with a create form.
func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	data, err := s.tokenListData(r)
	if err != nil {
		s.serverError(w, "list ingest tokens", err)
		return
	}
	s.render(w, "tokens", data)
}

// handleTokenCreate creates an ingest token, showing the plaintext exactly once
// by re-rendering the list (never stored in plaintext).
func (s *Server) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))

	var projectID *int64
	if raw := strings.TrimSpace(r.FormValue("project_id")); raw != "" {
		pid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			http.Error(w, "invalid project id", http.StatusBadRequest)
			return
		}
		projectID = &pid
	}

	plain, hash := token.New()
	if _, err := s.store.CreateIngestToken(r.Context(), hash, label, projectID); err != nil {
		s.serverError(w, "create ingest token", err)
		return
	}

	data, err := s.tokenListData(r)
	if err != nil {
		s.serverError(w, "list ingest tokens", err)
		return
	}
	data["NewToken"] = plain
	data["NewTokenName"] = label
	s.render(w, "tokens", data)
}

// handleTokenDelete removes an ingest token by id (PRG).
func (s *Server) handleTokenDelete(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteIngestToken(r.Context(), id); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, "delete ingest token", err)
		return
	}
	http.Redirect(w, r, "/tokens", http.StatusFound)
}
