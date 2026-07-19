package api

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/supercakecrumb/snagbox/internal/store"
)

func TestIssueToDTO_NilMetaAndAttachments(t *testing.T) {
	iss := store.Issue{
		ID:        uuid.Must(uuid.NewV7()),
		Source:    "api",
		Body:      "hello",
		Meta:      nil,
		CreatedAt: time.Now().UTC(),
	}

	dto := issueToDTO(iss, nil, "")
	if dto.Meta == nil {
		t.Fatal("Meta should be non-nil empty map, got nil")
	}
	if len(dto.Meta) != 0 {
		t.Errorf("Meta should be empty, got %v", dto.Meta)
	}
	if dto.Attachments == nil {
		t.Fatal("Attachments should be non-nil empty slice, got nil")
	}
	if len(dto.Attachments) != 0 {
		t.Errorf("Attachments should be empty, got %v", dto.Attachments)
	}
	if dto.Body != "hello" {
		t.Errorf("Body = %q, want %q", dto.Body, "hello")
	}
	if dto.ID != iss.ID.String() {
		t.Errorf("ID = %q, want %q", dto.ID, iss.ID.String())
	}
}

func TestIssueToDTO_AttachmentURLs(t *testing.T) {
	issID := uuid.Must(uuid.NewV7())
	attID := uuid.Must(uuid.NewV7())
	iss := store.Issue{ID: issID, Source: "api", Body: "x"}
	atts := []store.Attachment{{
		ID:        attID,
		IssueID:   issID,
		S3Key:     "issues/x/y",
		Mime:      "image/png",
		SizeBytes: 42,
		Filename:  "a.png",
	}}

	// With base URL: URL is filled.
	dto := issueToDTO(iss, atts, "https://snag.example")
	if len(dto.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(dto.Attachments))
	}
	want := "https://snag.example/api/v1/attachments/" + attID.String()
	if dto.Attachments[0].URL != want {
		t.Errorf("URL = %q, want %q", dto.Attachments[0].URL, want)
	}
	if dto.Attachments[0].Size != 42 {
		t.Errorf("Size = %d, want 42", dto.Attachments[0].Size)
	}

	// Without base URL: URL is empty.
	dtoNoURL := issueToDTO(iss, atts, "")
	if dtoNoURL.Attachments[0].URL != "" {
		t.Errorf("URL should be empty without base URL, got %q", dtoNoURL.Attachments[0].URL)
	}
}
