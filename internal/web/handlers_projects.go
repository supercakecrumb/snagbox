package web

import (
	"net/http"
	"strings"
)

// handleProjects lists all projects with a create form.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		s.serverError(w, "list projects", err)
		return
	}
	s.render(w, "projects", map[string]any{
		"Nav":      "projects",
		"CSRF":     s.csrfToken(r),
		"Projects": projects,
	})
}

// handleProjectCreate inserts a project from the submitted form (PRG).
func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if _, err := s.store.CreateProject(r.Context(), slug, name, description); err != nil {
		s.serverError(w, "create project", err)
		return
	}
	http.Redirect(w, r, "/projects", http.StatusFound)
}

// handleProjectDelete removes a project by id (PRG).
func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		s.serverError(w, "delete project", err)
		return
	}
	http.Redirect(w, r, "/projects", http.StatusFound)
}
