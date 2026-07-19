// Package client is a tiny, dependency-free Go SDK for reporting issues to a
// snagbox server. It covers only issue creation (the write path); consuming
// the queue is a plain HTTP concern handled by agents.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// Client reports issues to a snagbox server.
type Client struct {
	baseURL string
	token   string
	hc      *http.Client
}

// Option customizes a Client at construction time.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.hc = hc
	}
}

// New returns a client for a snagbox base URL (e.g. https://snagbox.example.com)
// authenticating with an ingest token. Trailing slash on baseURL is trimmed.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		hc:      &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Photo is a single image attachment to upload with an issue.
type Photo struct {
	Filename    string // required
	ContentType string // optional; defaults to application/octet-stream
	Data        []byte
}

// ReportRequest describes an issue to file.
type ReportRequest struct {
	Text   string
	Photos []Photo
	Meta   map[string]any
}

// Attachment is a stored file returned with an issue.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Mime     string `json:"mime"`
	Size     int64  `json:"size_bytes"`
	URL      string `json:"url"`
}

// Issue is a filed report as returned by the server.
type Issue struct {
	ID          string         `json:"id"`
	ProjectID   *int64         `json:"project_id"`
	Source      string         `json:"source"`
	Text        string         `json:"text"`
	Meta        map[string]any `json:"meta"`
	CreatedAt   time.Time      `json:"created_at"`
	Attachments []Attachment   `json:"attachments"`
}

// APIError is returned when the server responds with a non-2xx status. It
// carries the HTTP status code and the server-supplied error message so
// callers can inspect them.
type APIError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("snagbox: report issue: status %d: %s", e.StatusCode, e.Message)
}

// ReportIssue files one issue. With photos it sends multipart/form-data;
// otherwise a JSON body.
func (c *Client) ReportIssue(ctx context.Context, req ReportRequest) (*Issue, error) {
	if req.Text == "" && len(req.Photos) == 0 {
		return nil, fmt.Errorf("snagbox: empty issue: no text or photos")
	}

	var (
		body        io.Reader
		contentType string
	)
	if len(req.Photos) > 0 {
		b, ct, err := buildMultipart(req)
		if err != nil {
			return nil, err
		}
		body, contentType = b, ct
	} else {
		b, err := buildJSON(req)
		if err != nil {
			return nil, err
		}
		body, contentType = b, "application/json"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/issues", body)
	if err != nil {
		return nil, fmt.Errorf("snagbox: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("snagbox: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiErrorFromResponse(resp)
	}

	var iss Issue
	if err := json.NewDecoder(resp.Body).Decode(&iss); err != nil {
		return nil, fmt.Errorf("snagbox: decode response: %w", err)
	}
	return &iss, nil
}

// buildMultipart encodes the request as multipart/form-data and returns the
// body and its content type.
func buildMultipart(req ReportRequest) (io.Reader, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("text", req.Text); err != nil {
		return nil, "", fmt.Errorf("snagbox: write text field: %w", err)
	}
	if req.Meta != nil {
		metaJSON, err := json.Marshal(req.Meta)
		if err != nil {
			return nil, "", fmt.Errorf("snagbox: marshal meta: %w", err)
		}
		if err := mw.WriteField("meta", string(metaJSON)); err != nil {
			return nil, "", fmt.Errorf("snagbox: write meta field: %w", err)
		}
	}
	for _, p := range req.Photos {
		ct := p.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="photo"; filename=%q`, p.Filename))
		h.Set("Content-Type", ct)
		part, err := mw.CreatePart(h)
		if err != nil {
			return nil, "", fmt.Errorf("snagbox: create photo part: %w", err)
		}
		if _, err := part.Write(p.Data); err != nil {
			return nil, "", fmt.Errorf("snagbox: write photo data: %w", err)
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("snagbox: close multipart writer: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}

// buildJSON encodes the request as a JSON body, omitting meta when nil.
func buildJSON(req ReportRequest) (io.Reader, error) {
	body := struct {
		Text string         `json:"text"`
		Meta map[string]any `json:"meta,omitempty"`
	}{
		Text: req.Text,
		Meta: req.Meta,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("snagbox: marshal body: %w", err)
	}
	return bytes.NewReader(b), nil
}

// apiErrorFromResponse reads the error body and builds an *APIError.
func apiErrorFromResponse(resp *http.Response) error {
	msg := resp.Status
	raw, err := io.ReadAll(resp.Body)
	if err == nil && len(raw) > 0 {
		var body struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &body) == nil && body.Error != "" {
			msg = body.Error
		} else {
			msg = strings.TrimSpace(string(raw))
		}
	}
	return &APIError{StatusCode: resp.StatusCode, Message: msg}
}
