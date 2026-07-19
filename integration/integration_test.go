//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/supercakecrumb/snagbox/client"
	"github.com/supercakecrumb/snagbox/internal/api"
	"github.com/supercakecrumb/snagbox/internal/blob"
	"github.com/supercakecrumb/snagbox/internal/store"
	"github.com/supercakecrumb/snagbox/internal/token"
)

// TestIngestQueueAckFanout exercises the full ingest -> per-agent queue
// fan-out -> attachment download -> ack cursor flow against a real Postgres
// and MinIO. It is skipped unless SNAGBOX_TEST_DATABASE_URL is set.
func TestIngestQueueAckFanout(t *testing.T) {
	dbURL := os.Getenv("SNAGBOX_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("SNAGBOX_TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()

	st, err := store.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	// Isolate the run: clear all app tables so seeding starts from empty.
	if _, err := st.Pool.Exec(ctx,
		"TRUNCATE issues, attachments, acks, agents, agent_project_perms, ingest_tokens, projects, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	endpoint := os.Getenv("SNAGBOX_TEST_S3_ENDPOINT")
	ak := os.Getenv("SNAGBOX_TEST_S3_ACCESS_KEY")
	sk := os.Getenv("SNAGBOX_TEST_S3_SECRET_KEY")
	bl, err := blob.New(ctx, endpoint, ak, sk, "snagbox-itest", false)
	if err != nil {
		t.Fatalf("blob.New: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.New(st, bl, "http://example.test", logger)
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	// Seed a project, an ingest token scoped to it, and two agents both
	// permitted on the project.
	proj, err := st.CreateProject(ctx, "geodrill", "Geodrill", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	ingPlain, ingHash := token.New()
	if _, err := st.CreateIngestToken(ctx, ingHash, "itest", &proj.ID); err != nil {
		t.Fatalf("CreateIngestToken: %v", err)
	}

	agentTokens := make([]string, 0, 2)
	for _, name := range []string{"agent-one", "agent-two"} {
		plain, hash := token.New()
		a, err := st.CreateAgent(ctx, name, hash, nil)
		if err != nil {
			t.Fatalf("CreateAgent %s: %v", name, err)
		}
		if err := st.SetAgentPerms(ctx, a.ID, []int64{proj.ID}, false); err != nil {
			t.Fatalf("SetAgentPerms %s: %v", name, err)
		}
		agentTokens = append(agentTokens, plain)
	}
	agentOne, agentTwo := agentTokens[0], agentTokens[1]

	// Report an issue with one photo via the client SDK.
	photoBytes := []byte("\x89PNGfake")
	c := client.New(srv.URL, ingPlain)
	iss, err := c.ReportIssue(ctx, client.ReportRequest{
		Text: "bug with a screenshot",
		Photos: []client.Photo{{
			Filename:    "shot.png",
			ContentType: "image/png",
			Data:        photoBytes,
		}},
	})
	if err != nil {
		t.Fatalf("ReportIssue: %v", err)
	}
	if iss.ID == "" {
		t.Fatal("reported issue has empty ID")
	}
	if iss.ProjectID == nil || *iss.ProjectID != proj.ID {
		t.Fatalf("issue ProjectID = %v, want %d", iss.ProjectID, proj.ID)
	}
	if len(iss.Attachments) != 1 {
		t.Fatalf("issue attachments = %d, want 1", len(iss.Attachments))
	}
	attID := iss.Attachments[0].ID

	// Fan-out: both agents' queues should contain exactly the one issue.
	for i, tok := range []string{agentOne, agentTwo} {
		q := getQueue(t, srv.URL, tok)
		if len(q) != 1 {
			t.Fatalf("agent %d queue length = %d, want 1", i+1, len(q))
		}
		if q[0].ID != iss.ID {
			t.Fatalf("agent %d queue issue ID = %q, want %q", i+1, q[0].ID, iss.ID)
		}
		if len(q[0].Attachments) != 1 || q[0].Attachments[0].URL == "" {
			t.Fatalf("agent %d queue attachment URL missing: %+v", i+1, q[0].Attachments)
		}
	}

	// Download the attachment through the API (the URL host won't resolve,
	// so we hit the test server directly with the attachment id).
	got := downloadAttachment(t, srv.URL, attID, agentOne)
	if string(got) != string(photoBytes) {
		t.Fatalf("downloaded attachment = %q, want %q", got, photoBytes)
	}

	// Ack with agent one only; cursors are independent.
	ackIssue(t, srv.URL, iss.ID, agentOne)
	if q := getQueue(t, srv.URL, agentOne); len(q) != 0 {
		t.Fatalf("agent-one queue after ack = %d, want 0", len(q))
	}
	if q := getQueue(t, srv.URL, agentTwo); len(q) != 1 {
		t.Fatalf("agent-two queue after agent-one ack = %d, want 1", len(q))
	}

	// Browse issues by project.
	browsed := browseIssues(t, srv.URL, "geodrill", agentOne)
	if len(browsed) != 1 {
		t.Fatalf("browse project=geodrill returned %d issues, want 1", len(browsed))
	}
	if browsed[0].ID != iss.ID {
		t.Fatalf("browsed issue ID = %q, want %q", browsed[0].ID, iss.ID)
	}
}

// getQueue fetches an agent's pending queue and decodes it.
func getQueue(t *testing.T, srvURL, agentToken string) []client.Issue {
	t.Helper()
	return getIssues(t, srvURL+"/api/v1/queue", agentToken)
}

// browseIssues fetches the issue browse endpoint filtered by project.
func browseIssues(t *testing.T, srvURL, project, agentToken string) []client.Issue {
	t.Helper()
	return getIssues(t, srvURL+"/api/v1/issues?project="+project, agentToken)
}

// getIssues performs an authenticated GET returning a JSON array of issues.
func getIssues(t *testing.T, url, agentToken string) []client.Issue {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build GET %s: %v", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+agentToken)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", url, resp.StatusCode)
	}
	var out []client.Issue
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode GET %s: %v", url, err)
	}
	return out
}

// downloadAttachment fetches an attachment's bytes through the API.
func downloadAttachment(t *testing.T, srvURL, attID, agentToken string) []byte {
	t.Helper()
	url := srvURL + "/api/v1/attachments/" + attID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build GET %s: %v", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+agentToken)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read attachment body: %v", err)
	}
	return data
}

// ackIssue acknowledges one issue for the given agent, expecting 204.
func ackIssue(t *testing.T, srvURL, issueID, agentToken string) {
	t.Helper()
	url := srvURL + "/api/v1/issues/" + issueID + "/ack"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("build POST %s: %v", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+agentToken)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST %s: status %d, want 204", url, resp.StatusCode)
	}
}
