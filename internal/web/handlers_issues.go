package web

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/store"
)

// issueView is a single issue plus its resolved project label and attachments
// for the browser page.
type issueView struct {
	Issue       store.Issue
	Project     string // project name, or "inbox"
	Attachments []store.Attachment
}

// handleIssues renders the issue browser, filtered by ?project= (slug, "inbox",
// or empty for all) and ?limit= (capped at 100).
func (s *Server) handleIssues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		s.serverError(w, "list projects", err)
		return
	}
	names := make(map[int64]string, len(projects))
	for _, p := range projects {
		names[p.ID] = p.Name
	}

	filter := r.URL.Query().Get("project")
	params := store.ListIssuesParams{}
	switch {
	case filter == "inbox":
		params.InboxOnly = true
	case filter != "":
		p, err := s.store.GetProjectBySlug(ctx, filter)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "unknown project", http.StatusNotFound)
			return
		}
		if err != nil {
			s.serverError(w, "get project by slug", err)
			return
		}
		pid := p.ID
		params.ProjectID = &pid
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	params.Limit = limit

	issues, err := s.store.ListIssues(ctx, params)
	if err != nil {
		s.serverError(w, "list issues", err)
		return
	}

	views := make([]issueView, 0, len(issues))
	for _, iss := range issues {
		// GetIssue is an N+1 lookup, acceptable for this capped admin page.
		_, atts, err := s.store.GetIssue(ctx, iss.ID)
		if err != nil {
			s.serverError(w, "get issue", err)
			return
		}
		project := "inbox"
		if iss.ProjectID != nil {
			if name, ok := names[*iss.ProjectID]; ok {
				project = name
			}
		}
		views = append(views, issueView{Issue: iss, Project: project, Attachments: atts})
	}

	s.render(w, "issues", map[string]any{
		"Nav":      "issues",
		"CSRF":     s.csrfToken(r),
		"Issues":   views,
		"Projects": projects,
		"Filter":   filter,
		"Limit":    limit,
	})
}

// handleAttachment streams an attachment's bytes from blob storage. It is
// session-gated by the protected mux, letting admins view photos directly.
func (s *Server) handleAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid attachment id", http.StatusBadRequest)
		return
	}

	att, err := s.store.GetAttachment(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "get attachment", err)
		return
	}

	rc, err := s.blob.Get(r.Context(), att.S3Key)
	if errors.Is(err, blob.ErrNotFound) {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "get blob", err)
		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", att.Mime)
	if _, err := io.Copy(w, rc); err != nil {
		s.logger.Error("stream attachment", "attachment_id", id, "error", err)
	}
}
