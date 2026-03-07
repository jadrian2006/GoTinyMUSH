package server

import (
	"fmt"
	"log"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	"github.com/crystal-mush/gotinymush/pkg/eventbus"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// --- eventbus.GameInterface implementation ---
// These methods let the eventbus package call back into the game
// without importing the server package (breaking circular deps).

// GetObject returns the game object for a dbref, or nil.
func (g *Game) GetObjectEB(ref gamedb.DBRef) *gamedb.Object {
	if obj, ok := g.DB.Objects[ref]; ok {
		return obj
	}
	return nil
}

// GetParentEB returns the parent dbref of an object.
func (g *Game) GetParentEB(ref gamedb.DBRef) gamedb.DBRef {
	if obj, ok := g.DB.Objects[ref]; ok {
		return obj.Parent
	}
	return gamedb.Nothing
}

// EvalLockStringEB evaluates a lock expression string against a player.
func (g *Game) EvalLockStringEB(player, thing gamedb.DBRef, lock string) bool {
	if lock == "" {
		// Empty lock = wizard-only
		return g.IsWizard(player)
	}
	if lock == "*" {
		return true
	}
	parsed := ParseBoolExp(g, player, lock)
	if parsed == nil {
		return false
	}
	return EvalBoolExp(g, player, thing, thing, parsed, 0)
}

// TriggerAttrEB evaluates an attribute on an object, passing args.
// This is the bus phase execution path — runs softcode directly
// using the existing ExecuteQueueEntry mechanism.
func (g *Game) TriggerAttrEB(player, enactor gamedb.DBRef, obj gamedb.DBRef, attr string, args []string) {
	attrNum := g.ResolveAttrNum(attr)
	if attrNum < 0 {
		return
	}
	text := g.GetAttrText(obj, attrNum)
	if text == "" {
		return
	}

	entry := &QueueEntry{
		Player:  obj,
		Cause:   enactor,
		Caller:  obj,
		Command: text,
		Args:    args,
	}
	// Execute immediately using the same path as the master queue.
	// This ensures consistent softcode behavior (braces, semicolons,
	// %substitutions, etc.) while running in the bus phase.
	g.ExecuteQueueEntry(entry)
}

// gameAdapter wraps *Game to satisfy eventbus.GameInterface.
type gameAdapter struct {
	g *Game
}

func (a *gameAdapter) GetObject(ref gamedb.DBRef) *gamedb.Object {
	return a.g.GetObjectEB(ref)
}

func (a *gameAdapter) GetParent(ref gamedb.DBRef) gamedb.DBRef {
	return a.g.GetParentEB(ref)
}

func (a *gameAdapter) EvalLockString(player, thing gamedb.DBRef, lock string) bool {
	return a.g.EvalLockStringEB(player, thing, lock)
}

func (a *gameAdapter) TriggerAttr(player, enactor, obj gamedb.DBRef, attr string, args []string) {
	a.g.TriggerAttrEB(player, enactor, obj, attr, args)
}

// GameAdapter returns a gameAdapter that satisfies eventbus.GameInterface.
func (g *Game) GameAdapter() eventbus.GameInterface {
	return &gameAdapter{g: g}
}

// --- GameState interface implementation for event bus ---
// These let softcode functions (publish, subscribe, etc.) call through.

func (g *Game) EventBusPublish(caller gamedb.DBRef, queueName, data string) string {
	if g.EventQueues == nil {
		return "#-1 EVENT BUS NOT AVAILABLE"
	}
	return g.EventQueues.Publish(g.GameAdapter(), caller, queueName, data)
}

func (g *Game) EventBusSubscribe(obj gamedb.DBRef, attr, queueName string, bind gamedb.DBRef) string {
	if g.EventQueues == nil {
		return "#-1 EVENT BUS NOT AVAILABLE"
	}
	return g.EventQueues.Subscribe(g.GameAdapter(), obj, attr, queueName, bind)
}

func (g *Game) EventBusUnsubscribe(obj gamedb.DBRef, attr, queueName string) string {
	if g.EventQueues == nil {
		return "#-1 EVENT BUS NOT AVAILABLE"
	}
	return g.EventQueues.Unsubscribe(obj, attr, queueName)
}

func (g *Game) EventBusQueueList() string {
	if g.EventQueues == nil {
		return ""
	}
	return strings.Join(g.EventQueues.QueueNames(), " ")
}

func (g *Game) EventBusQueueInfo(queueName, mode string) string {
	if g.EventQueues == nil {
		return "#-1 EVENT BUS NOT AVAILABLE"
	}
	q := g.EventQueues.GetQueue(queueName)
	if q == nil {
		return "#-1 QUEUE NOT FOUND"
	}

	switch mode {
	case "subs":
		var refs []string
		for _, sub := range q.Subscribers {
			refs = append(refs, fmt.Sprintf("#%d", sub.Object))
		}
		return strings.Join(refs, " ")
	case "stats":
		s := &q.Stats
		avgPayload := int64(0)
		if s.PublishCount > 0 {
			avgPayload = s.TotalPayloadBytes / s.PublishCount
		}
		avgEval := int64(0)
		if s.DeliverCount > 0 {
			avgEval = s.TotalEvalNs / s.DeliverCount
		}
		return fmt.Sprintf(`{"publish_count":%d,"fire_count":%d,"deliver_count":%d,"dropped":%d,"rejected":%d,"avg_payload":%d,"max_payload":%d,"avg_eval_ns":%d,"max_eval_ns":%d,"active_subs":%d,"active_pubs":%d,"pending":%d,"budget_exhausted":%d,"gen1_count":%d,"gen2_rejected":%d}`,
			s.PublishCount, s.FireCount, s.DeliverCount, s.DroppedCount, s.RejectCount,
			avgPayload, s.MaxPayloadBytes, avgEval, s.MaxEvalNs,
			q.ActiveSubCount(), q.ActivePubCount(), q.PendingCount(),
			s.BudgetExhausted, s.Gen1Count, s.Gen2Rejected)
	default:
		// Return queue info as JSON
		return fmt.Sprintf(`{"name":"%s","rate":%d,"scope":"%s","max_subs":%d,"enabled":%v,"subs":%d,"pubs":%d,"pending":%d}`,
			q.Name, q.Rate, q.Scope, q.MaxSubs, q.Enabled,
			q.ActiveSubCount(), q.ActivePubCount(), q.PendingCount())
	}
}

// --- Persistence helpers ---

// queueToStored converts an EventQueue to its storable form.
func queueToStored(q *eventbus.EventQueue) *boltstore.StoredEventQueue {
	return &boltstore.StoredEventQueue{
		Name:      q.Name,
		Aliases:   q.Aliases,
		Rate:      q.Rate,
		Scope:     int(q.Scope),
		MaxSubs:   q.MaxSubs,
		Enabled:   q.Enabled,
		PubLock:   q.PubLock,
		SubLock:   q.SubLock,
		AdminLock: q.AdminLock,
		CreatedAt: q.Stats.CreatedAt,
	}
}

// storedToQueue converts a StoredEventQueue back to a live EventQueue.
func storedToQueue(sq *boltstore.StoredEventQueue) *eventbus.EventQueue {
	q := eventbus.NewEventQueue(sq.Name)
	q.Aliases = sq.Aliases
	q.Rate = sq.Rate
	q.Scope = eventbus.ScopeType(sq.Scope)
	q.MaxSubs = sq.MaxSubs
	q.Enabled = sq.Enabled
	q.PubLock = sq.PubLock
	q.SubLock = sq.SubLock
	q.AdminLock = sq.AdminLock
	q.Stats.CreatedAt = sq.CreatedAt
	return q
}

// PersistEventQueue saves an event queue to bolt (no-op if Store is nil).
func (g *Game) PersistEventQueue(q *eventbus.EventQueue) {
	if g.Store == nil {
		return
	}
	if err := g.Store.PutEventQueue(queueToStored(q)); err != nil {
		log.Printf("ERROR: persist event queue %q: %v", q.Name, err)
	}
}

// DeleteEventQueue removes an event queue from bolt.
func (g *Game) DeleteEventQueue(name string) {
	if g.Store == nil {
		return
	}
	if err := g.Store.DeleteEventQueue(name); err != nil {
		log.Printf("ERROR: delete event queue %q: %v", name, err)
	}
}

// LoadEventQueues restores event queues from bolt into the QueueManager.
func (g *Game) LoadEventQueues() {
	if g.Store == nil || g.EventQueues == nil {
		return
	}
	stored, err := g.Store.LoadEventQueues()
	if err != nil {
		log.Printf("WARNING: failed to load event queues: %v", err)
		return
	}
	for i := range stored {
		sq := &stored[i]
		q := storedToQueue(sq)
		if err := g.EventQueues.CreateQueue(q); err != nil {
			log.Printf("WARNING: load event queue %q: %v", sq.Name, err)
			continue
		}
		// Re-register aliases
		for _, alias := range sq.Aliases {
			g.EventQueues.AddAlias(alias, sq.Name)
		}
	}
	if len(stored) > 0 {
		log.Printf("Loaded %d event queue(s) from bolt", len(stored))
	}
}
