package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/supercakecrumb/snagbox/internal/store"
)

// handleUsers lists the Telegram allowlist with a create form.
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.serverError(w, "list users", err)
		return
	}
	s.render(w, "users", map[string]any{
		"Nav":   "users",
		"CSRF":  s.csrfToken(r),
		"Users": users,
	})
}

// handleUserCreate adds a user to the allowlist (PRG).
func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	tgID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("telegram_id")), 10, 64)
	if err != nil {
		http.Error(w, "invalid telegram id", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	role := strings.TrimSpace(r.FormValue("role"))
	if role != "admin" && role != "member" {
		http.Error(w, "role must be admin or member", http.StatusBadRequest)
		return
	}

	if _, err := s.store.CreateUser(r.Context(), tgID, name, role); err != nil {
		s.serverError(w, "create user", err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

// handleUserDelete removes a user from the allowlist (PRG).
func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	if !s.verifyCSRF(r) {
		http.Error(w, "bad csrf", http.StatusBadRequest)
		return
	}
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, "delete user", err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}
