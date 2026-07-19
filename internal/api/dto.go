package api

import (
	"time"

	"github.com/supercakecrumb/snagbox/internal/store"
)

// issueDTO is the JSON shape returned for an issue across all endpoints.
type issueDTO struct {
	ID          string          `json:"id"`
	ProjectID   *int64          `json:"project_id"`
	Source      string          `json:"source"`
	Body        string          `json:"text"` // maps from Issue.Body
	Meta        map[string]any  `json:"meta"`
	CreatedAt   time.Time       `json:"created_at"`
	Attachments []attachmentDTO `json:"attachments"`
}

// attachmentDTO is the JSON shape returned for an attachment.
type attachmentDTO struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Mime     string `json:"mime"`
	Size     int64  `json:"size_bytes"`
	URL      string `json:"url,omitempty"` // set only where a download URL is meaningful
}

// issueToDTO maps an issue and its attachments to the wire DTO. When
// publicBaseURL is non-empty each attachment gets a download URL.
func issueToDTO(iss store.Issue, atts []store.Attachment, publicBaseURL string) issueDTO {
	meta := iss.Meta
	if meta == nil {
		meta = map[string]any{}
	}
	dto := issueDTO{
		ID:          iss.ID.String(),
		ProjectID:   iss.ProjectID,
		Source:      iss.Source,
		Body:        iss.Body,
		Meta:        meta,
		CreatedAt:   iss.CreatedAt,
		Attachments: []attachmentDTO{},
	}
	for _, a := range atts {
		dto.Attachments = append(dto.Attachments, attachmentToDTO(a, publicBaseURL))
	}
	return dto
}

// attachmentToDTO maps a single attachment, filling URL when publicBaseURL
// is non-empty.
func attachmentToDTO(a store.Attachment, publicBaseURL string) attachmentDTO {
	dto := attachmentDTO{
		ID:       a.ID.String(),
		Filename: a.Filename,
		Mime:     a.Mime,
		Size:     a.SizeBytes,
	}
	if publicBaseURL != "" {
		dto.URL = publicBaseURL + "/api/v1/attachments/" + a.ID.String()
	}
	return dto
}
