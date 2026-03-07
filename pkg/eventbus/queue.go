package eventbus

import (
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// ScopeType determines which subscribers receive a published event.
type ScopeType int

const (
	ScopeGlobal ScopeType = iota // All subscribers receive every event
	ScopeObject                  // Subscribers only receive from a bound publisher
	ScopeParent                  // Subscribers only receive from same-parent publishers
)

func (s ScopeType) String() string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeObject:
		return "object"
	case ScopeParent:
		return "parent"
	default:
		return "unknown"
	}
}

// ParseScopeType converts a string to a ScopeType.
func ParseScopeType(s string) (ScopeType, bool) {
	switch s {
	case "global":
		return ScopeGlobal, true
	case "object":
		return ScopeObject, true
	case "parent":
		return ScopeParent, true
	default:
		return ScopeGlobal, false
	}
}

// Subscription represents a single subscriber to a queue.
type Subscription struct {
	Object gamedb.DBRef // The subscribing object
	Attr   string       // Attribute to trigger on that object
	Bind   gamedb.DBRef // For scope:object — which publisher (Nothing = any)
}

// PendingEvent is an event waiting to be delivered to subscribers.
type PendingEvent struct {
	Publisher  gamedb.DBRef // The object that published the event
	Data      string       // JSON payload
	Generation int         // 0 = original, increments on each bus-to-bus hop
	Timestamp time.Time    // When the event was published
}

// QueueStats tracks lifetime and current statistics for a queue.
type QueueStats struct {
	// Lifetime counters (reset via @queue/stats/reset)
	FireCount       int64 // Total times the queue fired (delivered to subscribers)
	PublishCount    int64 // Total publish() calls received
	DeliverCount    int64 // Total individual subscriber deliveries
	DroppedCount    int64 // Events dropped (budget cap, destroyed subscriber, etc)
	RejectCount     int64 // publish() calls rejected by pub lock
	BudgetExhausted int64 // Times this queue had handlers deferred due to budget cap
	Gen1Count       int64 // Generation 1 events (bus-to-bus hops)
	Gen2Rejected    int64 // Generation 2+ events rejected by depth limit

	// Payload size tracking
	TotalPayloadBytes int64 // Sum of all event data sizes
	MinPayloadBytes   int   // Smallest event seen
	MaxPayloadBytes   int   // Largest event seen

	// Handler performance timing
	TotalEvalNs int64 // Total nanoseconds spent in subscriber handlers
	MinEvalNs   int64 // Fastest handler execution
	MaxEvalNs   int64 // Slowest handler execution

	// Timestamps
	LastFireTime    time.Time // When the queue last fired
	LastPublishTime time.Time // When last publish() was received
	CreatedAt       time.Time // Queue creation time
}

// EventQueue is a named, wizard-defined pub/sub queue.
type EventQueue struct {
	Name    string
	Aliases []string
	Rate    int       // 0 = event-driven only, N = fire every N ticks
	Scope   ScopeType
	MaxSubs int
	Enabled bool
	Counter int // Internal tick counter for rate timing

	// Locks — stored as MUSH lock key strings, parsed via ParseBoolExp at eval time
	PubLock   string
	SubLock   string
	AdminLock string

	// Runtime state (not persisted)
	Subscribers []Subscription
	Publishers  map[gamedb.DBRef]bool // Tracked unique publishers
	Pending     []PendingEvent        // Events waiting to be delivered

	// Stats
	Stats QueueStats
}

// NewEventQueue creates a new queue with sensible defaults.
func NewEventQueue(name string) *EventQueue {
	return &EventQueue{
		Name:        name,
		Rate:        1, // Default: every tick
		Scope:       ScopeGlobal,
		MaxSubs:     50,
		Enabled:     true,
		Subscribers: make([]Subscription, 0),
		Publishers:  make(map[gamedb.DBRef]bool),
		Pending:     make([]PendingEvent, 0),
		Stats: QueueStats{
			CreatedAt:       time.Now(),
			MinPayloadBytes: -1, // Sentinel: no events seen yet
			MinEvalNs:       -1,
		},
	}
}

// ActiveSubCount returns the current number of subscribers.
func (q *EventQueue) ActiveSubCount() int {
	return len(q.Subscribers)
}

// ActivePubCount returns the number of unique publishers seen.
func (q *EventQueue) ActivePubCount() int {
	return len(q.Publishers)
}

// PendingCount returns the number of events waiting for delivery.
func (q *EventQueue) PendingCount() int {
	return len(q.Pending)
}
