package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	authkit "github.com/supercakecrumb/msgr-authkit"
)

// SessionCookieName is the name of the cookie that carries the web session
// token for the snagbox admin UI.
const SessionCookieName = "snagbox_session"

// SessionValidator validates a session token and returns the resolved
// web session. PGSessionIssuer satisfies this interface.
type SessionValidator interface {
	Validate(ctx context.Context, token string) (authkit.WebSession, error)
}

type contextKey string

const sessionContextKey contextKey = "snagbox_web_session"

// RequireSession returns middleware that enforces a valid web session on the
// wrapped handler, redirecting to loginPath when the session is missing or
// invalid and storing the resolved session in the request context otherwise.
func RequireSession(validator SessionValidator, loginPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := sessionFromRequest(r, validator)
			if !ok {
				http.Redirect(w, r, loginPath, http.StatusFound)
				return
			}
			ctx := context.WithValue(r.Context(), sessionContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sessionFromRequest(r *http.Request, validator SessionValidator) (authkit.WebSession, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return authkit.WebSession{}, false
	}
	session, err := validator.Validate(r.Context(), cookie.Value)
	if err != nil {
		return authkit.WebSession{}, false
	}
	return session, true
}

// SessionFromContext returns the web session stored by RequireSession, if any.
func SessionFromContext(ctx context.Context) (authkit.WebSession, bool) {
	session, ok := ctx.Value(sessionContextKey).(authkit.WebSession)
	return session, ok
}

// SetSessionCookie writes the session token to the response as an HttpOnly
// cookie that expires with the session.
func SetSessionCookie(w http.ResponseWriter, session authkit.WebSession, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie on the client.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ErrCSRFInvalid indicates a submitted CSRF token did not match the expected
// value for the session.
var ErrCSRFInvalid = errors.New("auth: invalid csrf token")

// CSRFTokenFor derives a deterministic CSRF token for a session token using an
// HMAC keyed by serverSecret.
func CSRFTokenFor(serverSecret []byte, sessionToken string) string {
	mac := hmac.New(sha256.New, serverSecret)
	_, _ = mac.Write([]byte(sessionToken))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCSRF reports whether submitted matches the CSRF token expected for the
// given session token, using a constant-time comparison.
func VerifyCSRF(serverSecret []byte, sessionToken, submitted string) bool {
	expected := CSRFTokenFor(serverSecret, sessionToken)
	return hmac.Equal([]byte(expected), []byte(submitted))
}
