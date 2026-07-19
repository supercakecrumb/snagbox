package bot

import (
	"sync"
	"testing"
	"time"

	"github.com/supercakecrumb/snagbox/internal/store"
)

func TestAlbumBufferFlushesOnce(t *testing.T) {
	ab := &albumBuffer{
		groups: make(map[string]*albumGroup),
		delay:  20 * time.Millisecond,
	}

	var (
		mu       sync.Mutex
		calls    int
		gotFiles []string
		gotCap   string
	)
	done := make(chan struct{})
	ab.flush = func(g *albumGroup) {
		mu.Lock()
		calls++
		gotFiles = append([]string(nil), g.fileIDs...)
		gotCap = g.caption
		mu.Unlock()
		close(done)
	}

	user := store.User{ID: 7}
	ab.add("mg1", 100, user, "a caption", "file1")
	ab.add("mg1", 100, user, "", "file2")
	ab.add("mg1", 100, user, "", "file3")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("flush was not called")
	}
	// Allow any spurious extra timer to fire (it should not).
	time.Sleep(40 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected exactly 1 flush, got %d", calls)
	}
	if gotCap != "a caption" {
		t.Fatalf("expected caption %q, got %q", "a caption", gotCap)
	}
	want := []string{"file1", "file2", "file3"}
	if len(gotFiles) != len(want) {
		t.Fatalf("expected %d file ids, got %d: %v", len(want), len(gotFiles), gotFiles)
	}
	for i := range want {
		if gotFiles[i] != want[i] {
			t.Fatalf("file id %d: expected %q, got %q", i, want[i], gotFiles[i])
		}
	}
}
