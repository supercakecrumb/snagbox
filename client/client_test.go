package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportIssueJSON(t *testing.T) {
	var (
		gotContentType string
		gotAuth        string
		gotBody        struct {
			Text string         `json:"text"`
			Meta map[string]any `json:"meta"`
		}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		writeIssue(w, http.StatusCreated, Issue{
			ID:     "issue-1",
			Source: "api",
			Text:   gotBody.Text,
			Meta:   gotBody.Meta,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok-123")
	iss, err := c.ReportIssue(context.Background(), ReportRequest{
		Text: "something broke",
		Meta: map[string]any{"severity": "high"},
	})
	if err != nil {
		t.Fatalf("ReportIssue: %v", err)
	}

	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Errorf("content type = %q, want application/json", gotContentType)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("authorization = %q, want Bearer tok-123", gotAuth)
	}
	if gotBody.Text != "something broke" {
		t.Errorf("server text = %q, want %q", gotBody.Text, "something broke")
	}
	if gotBody.Meta["severity"] != "high" {
		t.Errorf("server meta severity = %v, want high", gotBody.Meta["severity"])
	}
	if iss.ID != "issue-1" || iss.Text != "something broke" {
		t.Errorf("parsed issue = %+v, want id issue-1 text 'something broke'", iss)
	}
}

func TestReportIssueMultipart(t *testing.T) {
	var (
		gotText      string
		gotFilename  string
		gotPhotoData []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("content type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		gotText = r.FormValue("text")

		files := r.MultipartForm.File["photo"]
		if len(files) != 1 {
			t.Fatalf("photo parts = %d, want 1", len(files))
		}
		gotFilename = files[0].Filename
		f, err := files[0].Open()
		if err != nil {
			t.Fatalf("open photo: %v", err)
		}
		defer func() { _ = f.Close() }()
		gotPhotoData, _ = io.ReadAll(f)

		writeIssue(w, http.StatusCreated, Issue{
			ID:   "issue-2",
			Text: gotText,
			Attachments: []Attachment{{
				ID:       "att-1",
				Filename: gotFilename,
				Mime:     "image/png",
				Size:     int64(len(gotPhotoData)),
				URL:      "http://example.test/api/v1/attachments/att-1",
			}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok-abc")
	iss, err := c.ReportIssue(context.Background(), ReportRequest{
		Text: "with a photo",
		Photos: []Photo{{
			Filename:    "shot.png",
			ContentType: "image/png",
			Data:        []byte("PNGDATA"),
		}},
	})
	if err != nil {
		t.Fatalf("ReportIssue: %v", err)
	}

	if gotText != "with a photo" {
		t.Errorf("server text = %q, want %q", gotText, "with a photo")
	}
	if gotFilename != "shot.png" {
		t.Errorf("server filename = %q, want shot.png", gotFilename)
	}
	if string(gotPhotoData) != "PNGDATA" {
		t.Errorf("server photo data = %q, want PNGDATA", gotPhotoData)
	}
	if len(iss.Attachments) != 1 || iss.Attachments[0].ID != "att-1" {
		t.Errorf("parsed attachments = %+v, want one with id att-1", iss.Attachments)
	}
}

func TestReportIssueAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
	}))
	defer srv.Close()

	c := New(srv.URL, "bad")
	_, err := c.ReportIssue(context.Background(), ReportRequest{Text: "hi"})
	if err == nil {
		t.Fatal("ReportIssue: expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status code = %d, want 401", apiErr.StatusCode)
	}
	if apiErr.Message != "unauthorized" {
		t.Errorf("message = %q, want unauthorized", apiErr.Message)
	}
}

func TestReportIssueEmpty(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	_, err := c.ReportIssue(context.Background(), ReportRequest{})
	if err == nil {
		t.Fatal("ReportIssue: expected error for empty request, got nil")
	}
	if hit {
		t.Error("server was hit for an empty request; validation should prevent the call")
	}
}

// writeIssue encodes an issue as a JSON response for the fake server.
func writeIssue(w http.ResponseWriter, status int, iss Issue) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(iss)
}

// writeErr encodes an error body matching the snagbox API shape.
func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
