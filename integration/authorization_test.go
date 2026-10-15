//go:build integration

package integration

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/supercakecrumb/snagbox/internal/api"
	"github.com/supercakecrumb/snagbox/internal/store"
	"github.com/supercakecrumb/snagbox/internal/token"
)

func TestAgentPermissionBoundaries(t *testing.T) {
	dsn := os.Getenv("SNAGBOX_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || !strings.Contains(u.Path, "test") {
		t.Fatal("database name must contain test")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	suffix := uuid.NewString()
	own, err := st.CreateProject(ctx, "own-"+suffix, "own", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateProject(ctx, "other-"+suffix, "other", "")
	if err != nil {
		t.Fatal(err)
	}
	plain, hash := token.New()
	agent, err := st.CreateAgent(ctx, "agent-"+suffix, hash, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAgentPerms(ctx, agent.ID, []int64{own.ID}, false); err != nil {
		t.Fatal(err)
	}
	ownID, otherID, inboxID := uuid.New(), uuid.New(), uuid.New()
	for _, issue := range []store.Issue{
		{ID: ownID, ProjectID: &own.ID, Source: "api", Body: "allowed", CreatedAt: time.Now()},
		{ID: otherID, ProjectID: &other.ID, Source: "api", Body: "forbidden", CreatedAt: time.Now()},
		{ID: inboxID, Source: "api", Body: "inbox", CreatedAt: time.Now()},
	} {
		if err := st.CreateIssue(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uuid.UUID{ownID, otherID, inboxID} {
		if err := st.AddAttachment(ctx, store.Attachment{ID: id, IssueID: id, S3Key: "test/" + id.String(), Mime: "image/png", Filename: "photo.png"}); err != nil {
			t.Fatal(err)
		}
	}
	params := store.ListIssuesParams{AgentID: &agent.ID}
	issues, err := st.ListIssues(ctx, params)
	if err != nil || len(issues) != 1 || issues[0].ID != ownID {
		t.Fatal("agent browse crossed project boundary")
	}
	params.ProjectID = &other.ID
	issues, err = st.ListIssues(ctx, params)
	if err != nil || len(issues) != 0 {
		t.Fatal("explicit project filter bypassed permission")
	}
	if _, err := st.GetAttachmentForAgent(ctx, ownID, agent.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{otherID, inboxID} {
		if _, err := st.GetAttachmentForAgent(ctx, id, agent.ID); !errors.Is(err, store.ErrNotFound) {
			t.Fatal("attachment scope bypass")
		}
		if err := st.AckIssue(ctx, agent.ID, id); !errors.Is(err, store.ErrNotFound) {
			t.Fatal("ack scope bypass")
		}
	}
	if err := st.AckIssue(ctx, agent.ID, ownID); err != nil {
		t.Fatal(err)
	}
	if err := st.AckIssue(ctx, agent.ID, ownID); err != nil {
		t.Fatal("ack lost idempotency")
	}
	if err := st.AckIssues(ctx, agent.ID, []uuid.UUID{otherID, inboxID}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := st.Pool.QueryRow(ctx, "SELECT count(*) FROM acks WHERE agent_id=$1", agent.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("bulk ack crossed permission boundary")
	}
	// A missing permission is rejected before any blob access (blob is deliberately nil).
	h := api.New(st, nil, "https://example.test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, id := range []uuid.UUID{otherID, inboxID} {
		req := httptest.NewRequest("GET", "/api/v1/attachments/"+id.String(), nil)
		req.Header.Set("Authorization", "Bearer "+plain)
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("unauthorized attachment got %d", rec.Code)
		}
	}
	if err := st.SetAgentPerms(ctx, agent.ID, nil, true); err != nil {
		t.Fatal(err)
	}
	issues, err = st.ListIssues(ctx, store.ListIssuesParams{AgentID: &agent.ID})
	if err != nil || len(issues) != 1 || issues[0].ID != inboxID {
		t.Fatal("inbox permission not isolated")
	}
	if _, err := st.GetAttachmentForAgent(ctx, ownID, agent.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("revoked permission remained active")
	}
}
