package web

import (
	"net/http"

	authkit "github.com/supercakecrumb/msgr-authkit"

	"github.com/supercakecrumb/snagbox/internal/auth"
)

// handleLoginGet renders the login page and, when an auth_token query parameter
// is present, redeems the bot-issued login link into a session cookie.
func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		s.render(w, "login", map[string]any{
			"Nav":      "login",
			"Disabled": true,
		})
		return
	}

	token := r.URL.Query().Get("auth_token")
	if token == "" {
		s.render(w, "login", map[string]any{"Nav": "login"})
		return
	}

	session, err := s.authSvc.RedeemLoginLink(r.Context(), authkit.RedeemLoginLinkInput{LinkToken: token})
	if err != nil {
		s.logger.Warn("redeem login link", "error", err)
		s.render(w, "login", map[string]any{
			"Nav":   "login",
			"Error": "This login link is invalid or expired. Send /login to the bot again.",
		})
		return
	}

	auth.SetSessionCookie(w, session, s.cookieSecure)
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout clears the session cookie and returns to the login page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w, s.cookieSecure)
	http.Redirect(w, r, "/login", http.StatusFound)
}
