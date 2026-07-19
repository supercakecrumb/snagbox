// Package web implements snagbox's server-rendered admin dashboard: managing
// projects, agents (queue consumers) and their per-project permissions,
// ingest tokens, and the Telegram user allowlist, plus an issue browser and
// the bot-first login-link redemption endpoint.
package web

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	authkit "github.com/supercakecrumb/msgr-authkit"

	"github.com/supercakecrumb/snagbox/internal/auth"
	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/store"
)

// LoginRedeemer redeems a signed login-link token into a web session. It is
// satisfied by *authkit.AuthService and may be nil when login is disabled.
type LoginRedeemer interface {
	RedeemLoginLink(ctx context.Context, in authkit.RedeemLoginLinkInput) (authkit.WebSession, error)
}

// Server renders and serves the snagbox admin dashboard.
type Server struct {
	store        *store.Store
	blob         *blob.Store
	authSvc      LoginRedeemer
	sessions     auth.SessionValidator
	serverSecret []byte
	cookieSecure bool
	tmpl         map[string]*template.Template
	logger       *slog.Logger
}

// Deps carries the dependencies required to build a Server. AuthService may be
// nil, in which case login is disabled and protected pages redirect to /login.
type Deps struct {
	Store        *store.Store
	Blob         *blob.Store
	AuthService  LoginRedeemer
	Sessions     auth.SessionValidator
	ServerSecret []byte
	CookieSecure bool
	Logger       *slog.Logger
}

// NewServer builds a Server, parsing its embedded templates up front.
func NewServer(d Deps) *Server {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		store:        d.Store,
		blob:         d.Blob,
		authSvc:      d.AuthService,
		sessions:     d.Sessions,
		serverSecret: d.ServerSecret,
		cookieSecure: d.CookieSecure,
		tmpl:         parseTemplates(),
		logger:       logger,
	}
}

// Handler returns the admin UI mux: the login/logout endpoints are open, every
// other page is wrapped in the session-enforcing middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", s.handleLoginGet)
	mux.HandleFunc("POST /logout", s.handleLogout)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /{$}", s.handleDashboard)
	protected.HandleFunc("GET /issues", s.handleIssues)
	protected.HandleFunc("GET /attachments/{id}", s.handleAttachment)
	protected.HandleFunc("GET /projects", s.handleProjects)
	protected.HandleFunc("POST /projects", s.handleProjectCreate)
	protected.HandleFunc("POST /projects/{id}/delete", s.handleProjectDelete)
	protected.HandleFunc("GET /agents", s.handleAgents)
	protected.HandleFunc("POST /agents", s.handleAgentCreate)
	protected.HandleFunc("GET /agents/{id}", s.handleAgentDetail)
	protected.HandleFunc("POST /agents/{id}/perms", s.handleAgentPerms)
	protected.HandleFunc("POST /agents/{id}/delete", s.handleAgentDelete)
	protected.HandleFunc("GET /tokens", s.handleTokens)
	protected.HandleFunc("POST /tokens", s.handleTokenCreate)
	protected.HandleFunc("POST /tokens/{id}/delete", s.handleTokenDelete)
	protected.HandleFunc("GET /users", s.handleUsers)
	protected.HandleFunc("POST /users", s.handleUserCreate)
	protected.HandleFunc("POST /users/{id}/delete", s.handleUserDelete)

	mux.Handle("/", auth.RequireSession(s.sessions, "/login")(protected))
	return mux
}

// render executes the named page template's "layout" with data.
func (s *Server) render(w http.ResponseWriter, name string, data map[string]any) {
	tmpl, ok := s.tmpl[name]
	if !ok {
		s.logger.Error("unknown template", "name", name)
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		s.logger.Error("render template", "name", name, "error", err)
	}
}

// csrfToken returns the CSRF token bound to the request's session, or an empty
// string when no session is present.
func (s *Server) csrfToken(r *http.Request) string {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		return ""
	}
	return auth.CSRFTokenFor(s.serverSecret, session.Token)
}

// verifyCSRF reports whether the request carries a valid csrf_token form field
// for its session.
func (s *Server) verifyCSRF(r *http.Request) bool {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		return false
	}
	return auth.VerifyCSRF(s.serverSecret, session.Token, r.FormValue("csrf_token"))
}

// pathID parses the {id} int64 path value.
func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
