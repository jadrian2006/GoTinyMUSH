package server

import (
	"log"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/events"
)

// ScrollbackWriter is a global event bus subscriber that writes channel
// messages to SQLite for scrollback retrieval.
type ScrollbackWriter struct {
	sqldb  *SQLStore
	game   *Game
	mu     sync.Mutex
	closed bool
}

// NewScrollbackWriter creates a scrollback writer and registers it as a
// global subscriber on the event bus.
func NewScrollbackWriter(game *Game) *ScrollbackWriter {
	if game.SQLDB == nil {
		return nil
	}

	// Initialize scrollback tables
	if err := game.SQLDB.InitScrollbackTables(); err != nil {
		log.Printf("scrollback: failed to init tables: %v", err)
		return nil
	}

	sw := &ScrollbackWriter{
		sqldb: game.SQLDB,
		game:  game,
	}

	game.EventBus.SubscribeGlobal(sw)
	log.Printf("scrollback: writer registered on event bus")
	return sw
}

// Receive implements events.Subscriber. Only EvChannel events are stored.
func (sw *ScrollbackWriter) Receive(ev events.Event) {
	if ev.Type != events.EvChannel {
		return
	}
	if ev.Channel == "" {
		return
	}

	senderName := ""
	if ev.Source >= 0 {
		senderName = sw.game.PlayerName(ev.Source)
	}

	if err := sw.sqldb.InsertScrollback(ev.Channel, ev.Source, senderName, ev.Text); err != nil {
		log.Printf("scrollback: insert error: %v", err)
	}
}

// Closed implements events.Subscriber.
func (sw *ScrollbackWriter) Closed() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.closed
}

// Close marks the writer as closed so the bus stops delivering events.
func (sw *ScrollbackWriter) Close() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.closed = true
}

// StartRetentionCleanup starts an hourly goroutine that purges old scrollback.
func StartRetentionCleanup(sqldb *SQLStore, retention time.Duration) {
	if sqldb == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			purged, err := sqldb.PurgeOldScrollback(retention)
			if err != nil {
				log.Printf("scrollback cleanup error: %v", err)
				continue
			}
			if purged > 0 {
				log.Printf("scrollback: purged %d old channel entries", purged)
			}
			personalPurged, err := sqldb.PurgeOldPersonalScrollback(retention)
			if err != nil {
				log.Printf("personal scrollback cleanup error: %v", err)
				continue
			}
			if personalPurged > 0 {
				log.Printf("scrollback: purged %d old personal entries", personalPurged)
			}
		}
	}()
}
