package bot

import (
	"sync"
	"time"

	"github.com/supercakecrumb/snagbox/internal/store"
)

// albumGroup accumulates the photos of a single Telegram media group (album)
// until it is flushed into one issue.
type albumGroup struct {
	chatID  int64
	user    store.User
	caption string
	fileIDs []string
	timer   *time.Timer
}

// albumBuffer groups album photos by MediaGroupID and flushes each group after
// a quiet period, since Telegram delivers album items as separate messages.
type albumBuffer struct {
	mu     sync.Mutex
	groups map[string]*albumGroup
	delay  time.Duration
	flush  func(g *albumGroup) // set by Bot to call createIssueWithPhotos
}

// newAlbumBuffer returns an albumBuffer that flushes groups after delay.
func newAlbumBuffer(delay time.Duration) *albumBuffer {
	return &albumBuffer{
		groups: make(map[string]*albumGroup),
		delay:  delay,
	}
}

// add appends a photo to its media group, (re)starting the quiet-period timer.
// When the timer fires the group is removed and handed to the flush callback.
func (ab *albumBuffer) add(mgID string, chatID int64, user store.User, caption, fileID string) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	g, ok := ab.groups[mgID]
	if !ok {
		g = &albumGroup{chatID: chatID, user: user}
		ab.groups[mgID] = g
	}
	g.fileIDs = append(g.fileIDs, fileID)
	if caption != "" {
		g.caption = caption
	}

	if g.timer != nil {
		g.timer.Stop()
	}
	g.timer = time.AfterFunc(ab.delay, func() {
		ab.mu.Lock()
		delete(ab.groups, mgID)
		ab.mu.Unlock()
		if ab.flush != nil {
			ab.flush(g)
		}
	})
}
