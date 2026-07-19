package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/store"
)

// maxMultipartMemory bounds in-memory buffering of multipart uploads; larger
// parts spill to temp files.
const maxMultipartMemory = 32 << 20

// createIssue ingests a new issue from an ingest-token-authenticated client.
// It accepts either multipart/form-data (text + optional photos) or JSON.
func (h *Handler) createIssue(w http.ResponseWriter, r *http.Request) {
	tok := ingestFromCtx(r.Context())

	var (
		text  string
		meta  map[string]any
		files []*multipart.FileHeader
	)

	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid multipart form")
			return
		}
		text = r.FormValue("text")
		if raw := r.FormValue("meta"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &meta); err != nil {
				writeErr(w, http.StatusBadRequest, "invalid meta json")
				return
			}
		}
		if r.MultipartForm != nil {
			files = r.MultipartForm.File["photo"]
		}
	default:
		var body struct {
			Text string         `json:"text"`
			Meta map[string]any `json:"meta"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "invalid json body")
			return
		}
		text = body.Text
		meta = body.Meta
	}

	if strings.TrimSpace(text) == "" && len(files) == 0 {
		writeErr(w, http.StatusBadRequest, "empty issue")
		return
	}

	issueID := uuid.Must(uuid.NewV7())
	iss := store.Issue{
		ID:        issueID,
		ProjectID: tok.ProjectID,
		Source:    "api",
		Body:      text,
		Meta:      meta,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.CreateIssue(r.Context(), iss); err != nil {
		h.logger.Error("create issue", "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	atts := make([]store.Attachment, 0, len(files))
	for _, fh := range files {
		att, err := h.storeAttachment(r, issueID, fh)
		if err != nil {
			h.logger.Error("store attachment", "issue_id", issueID, "error", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		atts = append(atts, att)
	}

	writeJSON(w, http.StatusCreated, issueToDTO(iss, atts, h.publicBaseURL))
}

// storeAttachment uploads one multipart file to blob storage and records the
// attachment row.
func (h *Handler) storeAttachment(r *http.Request, issueID uuid.UUID, fh *multipart.FileHeader) (store.Attachment, error) {
	file, err := fh.Open()
	if err != nil {
		return store.Attachment{}, fmt.Errorf("open upload: %w", err)
	}
	defer func() { _ = file.Close() }()

	attID := uuid.Must(uuid.NewV7())
	base := sanitize(fh.Filename)
	key := fmt.Sprintf("issues/%s/%s-%s", issueID, attID, base)

	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.blob.Put(r.Context(), key, file, fh.Size, contentType); err != nil {
		return store.Attachment{}, fmt.Errorf("put blob: %w", err)
	}

	att := store.Attachment{
		ID:        attID,
		IssueID:   issueID,
		S3Key:     key,
		Mime:      contentType,
		SizeBytes: fh.Size,
		Filename:  base,
	}
	if err := h.store.AddAttachment(r.Context(), att); err != nil {
		return store.Attachment{}, fmt.Errorf("add attachment: %w", err)
	}
	return att, nil
}

// sanitize reduces a client-supplied filename to a safe base name, replacing
// path separators and whitespace with underscores.
func sanitize(name string) string {
	// Normalize Windows separators so filepath.Base strips them even on
	// non-Windows hosts, then take the base name only.
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	base = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			return '_'
		}
		return r
	}, base)
	if base == "" || base == "." || base == ".." {
		return "file"
	}
	return base
}

// getQueue returns the agent's pending queue, oldest first, with attachment
// download URLs.
func (h *Handler) getQueue(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	items, err := h.store.QueueForAgent(r.Context(), agent.ID, limit)
	if err != nil {
		h.logger.Error("queue for agent", "agent_id", agent.ID, "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]issueDTO, 0, len(items))
	for _, it := range items {
		out = append(out, issueToDTO(it.Issue, it.Attachments, h.publicBaseURL))
	}
	writeJSON(w, http.StatusOK, out)
}

// ackIssue acknowledges a single issue for the agent.
func (h *Handler) ackIssue(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	err = h.store.AckIssue(r.Context(), agent.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "issue not found")
		return
	}
	if err != nil {
		h.logger.Error("ack issue", "agent_id", agent.ID, "issue_id", id, "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bulkAck acknowledges a batch of issues for the agent.
func (h *Handler) bulkAck(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())

	var body struct {
		IssueIDs []string `json:"issue_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	ids := make([]uuid.UUID, 0, len(body.IssueIDs))
	for _, s := range body.IssueIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid issue id")
			return
		}
		ids = append(ids, id)
	}

	if err := h.store.AckIssues(r.Context(), agent.ID, ids); err != nil {
		h.logger.Error("bulk ack", "agent_id", agent.ID, "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listIssues browses issues with optional project, since and limit filters.
func (h *Handler) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var params store.ListIssuesParams

	switch project := q.Get("project"); project {
	case "":
		// no project filter
	case "inbox":
		params.InboxOnly = true
	default:
		proj, err := h.store.GetProjectBySlug(r.Context(), project)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusBadRequest, "unknown project")
			return
		}
		if err != nil {
			h.logger.Error("resolve project", "slug", project, "error", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		params.ProjectID = &proj.ID
	}

	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid since timestamp")
			return
		}
		params.Since = &t
	}

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.Limit = n
		}
	}

	issues, err := h.store.ListIssues(r.Context(), params)
	if err != nil {
		h.logger.Error("list issues", "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]issueDTO, 0, len(issues))
	for _, iss := range issues {
		out = append(out, issueToDTO(iss, nil, ""))
	}
	writeJSON(w, http.StatusOK, out)
}

// getAttachment streams an attachment's bytes from blob storage.
func (h *Handler) getAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid attachment id")
		return
	}

	att, err := h.store.GetAttachment(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		h.logger.Error("get attachment", "attachment_id", id, "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	rc, err := h.blob.Get(r.Context(), att.S3Key)
	if errors.Is(err, blob.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		h.logger.Error("get blob", "attachment_id", id, "key", att.S3Key, "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer func() { _ = rc.Close() }()

	w.Header().Set("Content-Type", att.Mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", att.Filename))
	if _, err := io.Copy(w, rc); err != nil {
		h.logger.Error("stream attachment", "attachment_id", id, "error", err)
	}
}
