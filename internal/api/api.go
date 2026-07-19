// Package api implements snagbox's HTTP API: issue ingestion (ingest-token
// auth) and the per-agent fan-out queue with acks (agent-token auth).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/store"
)

// Handler serves the snagbox HTTP API.
type Handler struct {
	store         *store.Store
	blob          *blob.Store
	publicBaseURL string // no trailing slash; used to build attachment URLs
	logger        *slog.Logger
}

// New builds a Handler over the given store and blob store. publicBaseURL is
// used to construct attachment download URLs.
func New(st *store.Store, bl *blob.Store, publicBaseURL string, logger *slog.Logger) *Handler {
	return &Handler{
		store:         st,
		blob:          bl,
		publicBaseURL: publicBaseURL,
		logger:        logger,
	}
}

// Routes returns the API mux with all endpoints registered.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/issues", h.ingestAuth(h.createIssue))
	mux.HandleFunc("GET /api/v1/queue", h.agentAuth(h.getQueue))
	mux.HandleFunc("POST /api/v1/issues/{id}/ack", h.agentAuth(h.ackIssue))
	mux.HandleFunc("POST /api/v1/queue/ack", h.agentAuth(h.bulkAck))
	mux.HandleFunc("GET /api/v1/issues", h.agentAuth(h.listIssues))
	mux.HandleFunc("GET /api/v1/attachments/{id}", h.agentAuth(h.getAttachment))
	return mux
}

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes a generic JSON error body. Internal detail must be logged
// separately, never included in msg for 500-class responses.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
