package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/supercakecrumb/snagbox/internal/store"
	"github.com/supercakecrumb/snagbox/internal/token"
)

// ctxKey is an unexported type for request-context keys, avoiding collisions.
type ctxKey int

const (
	ctxKeyIngest ctxKey = iota
	ctxKeyAgent
)

// bearerToken extracts the plaintext bearer token from the Authorization
// header. It returns ("", false) when the header is missing or malformed.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}

// ingestAuth authenticates an ingest token and stores it in the request
// context for the wrapped handler.
func (h *Handler) ingestAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plaintext, ok := bearerToken(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		tok, err := h.store.IngestTokenByHash(r.Context(), token.Hash(plaintext))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			h.logger.Error("ingest auth: lookup token", "error", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyIngest, tok)
		next(w, r.WithContext(ctx))
	}
}

// ingestFromCtx returns the ingest token stored by ingestAuth.
func ingestFromCtx(ctx context.Context) store.IngestToken {
	tok, _ := ctx.Value(ctxKeyIngest).(store.IngestToken)
	return tok
}

// agentAuth authenticates an agent token, refreshes its last-seen timestamp
// best-effort, and stores the agent in the request context.
func (h *Handler) agentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plaintext, ok := bearerToken(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		agent, err := h.store.AgentByTokenHash(r.Context(), token.Hash(plaintext))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			h.logger.Error("agent auth: lookup token", "error", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if err := h.store.TouchAgent(r.Context(), agent.ID); err != nil {
			h.logger.Warn("agent auth: touch agent", "agent_id", agent.ID, "error", err)
		}
		ctx := context.WithValue(r.Context(), ctxKeyAgent, agent)
		next(w, r.WithContext(ctx))
	}
}

// agentFromCtx returns the agent stored by agentAuth.
func agentFromCtx(ctx context.Context) store.Agent {
	agent, _ := ctx.Value(ctxKeyAgent).(store.Agent)
	return agent
}
