package eventbus

import (
	"fmt"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

const (
	MaxBusHandlersPerTick = 50
	MaxBusGeneration      = 2
)

// GameInterface abstracts the game server to avoid circular imports.
// *server.Game satisfies this interface.
type GameInterface interface {
	GetObject(gamedb.DBRef) *gamedb.Object
	GetParent(gamedb.DBRef) gamedb.DBRef
	EvalLockString(player, thing gamedb.DBRef, lock string) bool
	TriggerAttr(player, enactor gamedb.DBRef, obj gamedb.DBRef, attr string, args []string)
}

// BusStats holds global bus-level statistics.
type BusStats struct {
	TotalTicks         int64     // Total ProcessEventBus() invocations
	TotalHandlersFired int64     // Total handler evaluations across all queues
	BudgetExhausted    int64     // Times the global budget was fully consumed
	LastTickTime       time.Time // When the bus last processed
}

// QueueManager is the central registry for all event queues.
type QueueManager struct {
	queues   map[string]*EventQueue // name -> queue
	aliases  map[string]string      // alias -> canonical name
	roundIdx int                    // Round-robin start index for fairness
	order    []string               // Queue iteration order (queue names)

	BusPhaseActive   bool   // True while ProcessEventBus is executing
	CurrentGeneration int   // Generation of the event currently being handled

	Stats BusStats
}

// NewQueueManager creates an empty queue manager.
func NewQueueManager() *QueueManager {
	return &QueueManager{
		queues:  make(map[string]*EventQueue),
		aliases: make(map[string]string),
		order:   make([]string, 0),
	}
}

// resolveAlias returns the canonical queue name for an alias (or the name itself).
func (m *QueueManager) resolveAlias(name string) string {
	lower := strings.ToLower(name)
	if canonical, ok := m.aliases[lower]; ok {
		return canonical
	}
	return lower
}

// GetQueue returns a queue by name or alias. Returns nil if not found.
func (m *QueueManager) GetQueue(name string) *EventQueue {
	canonical := m.resolveAlias(name)
	return m.queues[canonical]
}

// CreateQueue registers a new queue. Returns error if name already exists.
func (m *QueueManager) CreateQueue(q *EventQueue) error {
	lower := strings.ToLower(q.Name)
	if _, exists := m.queues[lower]; exists {
		return fmt.Errorf("queue %q already exists", q.Name)
	}
	if _, isAlias := m.aliases[lower]; isAlias {
		return fmt.Errorf("%q is already an alias", q.Name)
	}
	m.queues[lower] = q
	m.order = append(m.order, lower)
	return nil
}

// DestroyQueue removes a queue and all its aliases.
func (m *QueueManager) DestroyQueue(name string) error {
	canonical := m.resolveAlias(name)
	q := m.queues[canonical]
	if q == nil {
		return fmt.Errorf("queue %q not found", name)
	}

	// Remove all aliases pointing to this queue
	for alias, target := range m.aliases {
		if target == canonical {
			delete(m.aliases, alias)
		}
	}

	delete(m.queues, canonical)

	// Remove from iteration order
	for i, n := range m.order {
		if n == canonical {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}

	return nil
}

// AddAlias creates an alias for an existing queue.
func (m *QueueManager) AddAlias(alias, queueName string) error {
	canonical := m.resolveAlias(queueName)
	if m.queues[canonical] == nil {
		return fmt.Errorf("queue %q not found", queueName)
	}
	lowerAlias := strings.ToLower(alias)
	if _, exists := m.queues[lowerAlias]; exists {
		return fmt.Errorf("%q is a queue name, cannot be used as alias", alias)
	}
	m.aliases[lowerAlias] = canonical
	q := m.queues[canonical]
	q.Aliases = append(q.Aliases, alias)
	return nil
}

// QueueNames returns all canonical queue names.
func (m *QueueManager) QueueNames() []string {
	names := make([]string, 0, len(m.queues))
	for _, name := range m.order {
		names = append(names, m.queues[name].Name)
	}
	return names
}

// Publish adds an event to a queue. Returns an error string or empty on success.
func (m *QueueManager) Publish(g GameInterface, caller gamedb.DBRef, queueName string, data string) string {
	q := m.GetQueue(queueName)
	if q == nil {
		return "#-1 QUEUE NOT FOUND"
	}
	if !q.Enabled {
		return "#-1 QUEUE DISABLED"
	}

	// Check generation limit for bus-to-bus publishes
	generation := 0
	if m.BusPhaseActive {
		generation = m.CurrentGeneration + 1
		if generation >= MaxBusGeneration {
			q.Stats.Gen2Rejected++
			q.Stats.DroppedCount++
			return "#-1 EVENT GENERATION LIMIT"
		}
	}

	// Evaluate pub lock
	if q.PubLock != "" && !g.EvalLockString(caller, gamedb.Nothing, q.PubLock) {
		q.Stats.RejectCount++
		return "#-1 PERMISSION DENIED"
	}

	// Track publisher
	q.Publishers[caller] = true

	// Record payload stats
	payloadSize := len(data)
	q.Stats.PublishCount++
	q.Stats.TotalPayloadBytes += int64(payloadSize)
	if q.Stats.MinPayloadBytes < 0 || payloadSize < q.Stats.MinPayloadBytes {
		q.Stats.MinPayloadBytes = payloadSize
	}
	if payloadSize > q.Stats.MaxPayloadBytes {
		q.Stats.MaxPayloadBytes = payloadSize
	}
	q.Stats.LastPublishTime = time.Now()

	// Track generation
	if generation == 1 {
		q.Stats.Gen1Count++
	}

	// Enqueue the event
	q.Pending = append(q.Pending, PendingEvent{
		Publisher:  caller,
		Data:      data,
		Generation: generation,
		Timestamp: time.Now(),
	})

	return ""
}

// Subscribe adds a subscription to a queue. Returns an error string or empty on success.
func (m *QueueManager) Subscribe(g GameInterface, obj gamedb.DBRef, attr string, queueName string, bind gamedb.DBRef) string {
	q := m.GetQueue(queueName)
	if q == nil {
		return "#-1 QUEUE NOT FOUND"
	}

	// Check sub lock
	if q.SubLock != "" && !g.EvalLockString(obj, gamedb.Nothing, q.SubLock) {
		return "#-1 PERMISSION DENIED"
	}

	// Check max subscribers
	if q.MaxSubs > 0 && len(q.Subscribers) >= q.MaxSubs {
		return "#-1 QUEUE FULL"
	}

	// Check for duplicate subscription
	for _, sub := range q.Subscribers {
		if sub.Object == obj && sub.Attr == attr {
			return "#-1 ALREADY SUBSCRIBED"
		}
	}

	q.Subscribers = append(q.Subscribers, Subscription{
		Object: obj,
		Attr:   attr,
		Bind:   bind,
	})

	return ""
}

// Unsubscribe removes a subscription. Returns an error string or empty on success.
func (m *QueueManager) Unsubscribe(obj gamedb.DBRef, attr string, queueName string) string {
	q := m.GetQueue(queueName)
	if q == nil {
		return "#-1 QUEUE NOT FOUND"
	}

	for i, sub := range q.Subscribers {
		if sub.Object == obj && sub.Attr == attr {
			q.Subscribers = append(q.Subscribers[:i], q.Subscribers[i+1:]...)
			return ""
		}
	}

	return "#-1 NOT SUBSCRIBED"
}
