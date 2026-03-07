package eventbus

import (
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// mockGame implements GameInterface for testing.
type mockGame struct {
	objects map[gamedb.DBRef]*gamedb.Object
	triggers []triggerCall
}

type triggerCall struct {
	player, enactor, obj gamedb.DBRef
	attr                 string
	args                 []string
}

func newMockGame() *mockGame {
	return &mockGame{
		objects: make(map[gamedb.DBRef]*gamedb.Object),
	}
}

func (m *mockGame) addObject(ref gamedb.DBRef, parent gamedb.DBRef, owner gamedb.DBRef) {
	m.objects[ref] = &gamedb.Object{
		DBRef:  ref,
		Parent: parent,
		Owner:  owner,
	}
}

func (m *mockGame) GetObject(ref gamedb.DBRef) *gamedb.Object {
	return m.objects[ref]
}

func (m *mockGame) GetParent(ref gamedb.DBRef) gamedb.DBRef {
	if obj, ok := m.objects[ref]; ok {
		return obj.Parent
	}
	return gamedb.Nothing
}

func (m *mockGame) EvalLockString(player, thing gamedb.DBRef, lock string) bool {
	return true // Default: all locks pass
}

func (m *mockGame) TriggerAttr(player, enactor, obj gamedb.DBRef, attr string, args []string) {
	m.triggers = append(m.triggers, triggerCall{player, enactor, obj, attr, args})
}

func TestNewEventQueue(t *testing.T) {
	q := NewEventQueue("test.tick")
	if q.Name != "test.tick" {
		t.Errorf("expected name test.tick, got %s", q.Name)
	}
	if q.Rate != 1 {
		t.Errorf("expected default rate 1, got %d", q.Rate)
	}
	if q.Scope != ScopeGlobal {
		t.Errorf("expected default scope global, got %s", q.Scope)
	}
	if !q.Enabled {
		t.Error("expected queue to be enabled by default")
	}
	if q.MaxSubs != 50 {
		t.Errorf("expected default max_subs 50, got %d", q.MaxSubs)
	}
}

func TestCreateAndGetQueue(t *testing.T) {
	m := NewQueueManager()
	q := NewEventQueue("sled.tick")
	if err := m.CreateQueue(q); err != nil {
		t.Fatal(err)
	}
	got := m.GetQueue("sled.tick")
	if got != q {
		t.Error("GetQueue did not return the created queue")
	}
	// Case insensitive
	got = m.GetQueue("SLED.TICK")
	if got != q {
		t.Error("GetQueue should be case insensitive")
	}
}

func TestDuplicateQueue(t *testing.T) {
	m := NewQueueManager()
	m.CreateQueue(NewEventQueue("test"))
	err := m.CreateQueue(NewEventQueue("test"))
	if err == nil {
		t.Error("expected error creating duplicate queue")
	}
}

func TestAlias(t *testing.T) {
	m := NewQueueManager()
	q := NewEventQueue("weather.change")
	m.CreateQueue(q)
	if err := m.AddAlias("wx", "weather.change"); err != nil {
		t.Fatal(err)
	}
	got := m.GetQueue("wx")
	if got != q {
		t.Error("alias did not resolve to queue")
	}
}

func TestDestroyQueue(t *testing.T) {
	m := NewQueueManager()
	q := NewEventQueue("temp")
	m.CreateQueue(q)
	m.AddAlias("t", "temp")
	if err := m.DestroyQueue("temp"); err != nil {
		t.Fatal(err)
	}
	if m.GetQueue("temp") != nil {
		t.Error("queue should be gone after destroy")
	}
	if m.GetQueue("t") != nil {
		t.Error("alias should be gone after destroy")
	}
}

func TestPublishAndSubscribe(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10) // publisher
	g.addObject(2, gamedb.Nothing, 20) // subscriber

	q := NewEventQueue("test.event")
	q.Rate = 0 // event-driven
	q.Scope = ScopeGlobal
	m.CreateQueue(q)

	// Subscribe
	result := m.Subscribe(g, 2, "ON_TEST", "test.event", gamedb.Nothing)
	if result != "" {
		t.Fatalf("subscribe failed: %s", result)
	}

	// Publish
	result = m.Publish(g, 1, "test.event", `{"hello":"world"}`)
	if result != "" {
		t.Fatalf("publish failed: %s", result)
	}

	if len(q.Pending) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(q.Pending))
	}

	// Process
	hadWork := m.ProcessEventBus(g)
	if !hadWork {
		t.Error("expected ProcessEventBus to report work done")
	}

	if len(g.triggers) != 1 {
		t.Fatalf("expected 1 trigger call, got %d", len(g.triggers))
	}
	if g.triggers[0].attr != "ON_TEST" {
		t.Errorf("expected attr ON_TEST, got %s", g.triggers[0].attr)
	}
	if g.triggers[0].args[0] != `{"hello":"world"}` {
		t.Errorf("unexpected trigger data: %s", g.triggers[0].args[0])
	}
}

func TestScopeObject(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10) // sled A
	g.addObject(2, gamedb.Nothing, 10) // sled B
	g.addObject(3, gamedb.Nothing, 20) // recorder bound to sled A

	q := NewEventQueue("sled.tick")
	q.Rate = 0
	q.Scope = ScopeObject
	m.CreateQueue(q)

	// Recorder subscribes bound to sled A (#1)
	m.Subscribe(g, 3, "ON_TICK", "sled.tick", 1)

	// Sled A publishes — should trigger
	m.Publish(g, 1, "sled.tick", `{"from":"A"}`)
	m.ProcessEventBus(g)
	if len(g.triggers) != 1 {
		t.Fatalf("expected 1 trigger from sled A, got %d", len(g.triggers))
	}

	// Sled B publishes — should NOT trigger
	g.triggers = nil
	m.Publish(g, 2, "sled.tick", `{"from":"B"}`)
	m.ProcessEventBus(g)
	if len(g.triggers) != 0 {
		t.Fatalf("expected 0 triggers from sled B, got %d", len(g.triggers))
	}
}

func TestScopeParent(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(100, gamedb.Nothing, 10) // Pern Weather Parent
	g.addObject(200, gamedb.Nothing, 10) // Mars Weather Parent
	g.addObject(1, 100, 10)              // Pern weather system (parent #100)
	g.addObject(2, 200, 10)              // Mars weather system (parent #200)
	g.addObject(3, 100, 20)              // Pern subscriber (parent #100)

	q := NewEventQueue("weather.change")
	q.Rate = 0
	q.Scope = ScopeParent
	m.CreateQueue(q)

	m.Subscribe(g, 3, "ON_WX", "weather.change", gamedb.Nothing)

	// Pern publishes — subscriber shares parent #100, should fire
	m.Publish(g, 1, "weather.change", `{"planet":"Pern"}`)
	m.ProcessEventBus(g)
	if len(g.triggers) != 1 {
		t.Fatalf("expected 1 trigger from Pern, got %d", len(g.triggers))
	}

	// Mars publishes — subscriber has different parent, should NOT fire
	g.triggers = nil
	m.Publish(g, 2, "weather.change", `{"planet":"Mars"}`)
	m.ProcessEventBus(g)
	if len(g.triggers) != 0 {
		t.Fatalf("expected 0 triggers from Mars, got %d", len(g.triggers))
	}
}

func TestGenerationLimit(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10)

	q := NewEventQueue("test")
	q.Rate = 0
	q.Scope = ScopeGlobal
	m.CreateQueue(q)

	// Simulate being inside bus phase at generation 1
	m.BusPhaseActive = true
	m.CurrentGeneration = 1

	// This should be rejected (would be gen 2)
	result := m.Publish(g, 1, "test", `{"data":"looped"}`)
	if result != "#-1 EVENT GENERATION LIMIT" {
		t.Errorf("expected generation limit error, got: %q", result)
	}

	if q.Stats.Gen2Rejected != 1 {
		t.Errorf("expected Gen2Rejected=1, got %d", q.Stats.Gen2Rejected)
	}

	m.BusPhaseActive = false
}

func TestBudgetCap(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()

	q := NewEventQueue("flood")
	q.Rate = 0
	q.Scope = ScopeGlobal
	m.CreateQueue(q)

	// Create more subscribers than the budget
	for i := range MaxBusHandlersPerTick + 10 {
		ref := gamedb.DBRef(i + 100)
		g.addObject(ref, gamedb.Nothing, 10)
		m.Subscribe(g, ref, "ON_FLOOD", "flood", gamedb.Nothing)
	}

	g.addObject(1, gamedb.Nothing, 10)
	m.Publish(g, 1, "flood", `{}`)
	m.ProcessEventBus(g)

	// Should have fired exactly MaxBusHandlersPerTick handlers
	if len(g.triggers) != MaxBusHandlersPerTick {
		t.Errorf("expected %d triggers (budget cap), got %d", MaxBusHandlersPerTick, len(g.triggers))
	}

	if q.Stats.BudgetExhausted != 1 {
		t.Errorf("expected BudgetExhausted=1, got %d", q.Stats.BudgetExhausted)
	}
}

func TestUnsubscribe(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10)
	g.addObject(2, gamedb.Nothing, 20)

	q := NewEventQueue("test")
	q.Rate = 0
	q.Scope = ScopeGlobal
	m.CreateQueue(q)

	m.Subscribe(g, 2, "ON_TEST", "test", gamedb.Nothing)
	if len(q.Subscribers) != 1 {
		t.Fatal("expected 1 subscriber")
	}

	result := m.Unsubscribe(2, "ON_TEST", "test")
	if result != "" {
		t.Fatalf("unsubscribe failed: %s", result)
	}
	if len(q.Subscribers) != 0 {
		t.Error("expected 0 subscribers after unsubscribe")
	}

	// Publish should produce no triggers
	m.Publish(g, 1, "test", `{}`)
	m.ProcessEventBus(g)
	if len(g.triggers) != 0 {
		t.Error("expected no triggers after unsubscribe")
	}
}

func TestQueueNames(t *testing.T) {
	m := NewQueueManager()
	m.CreateQueue(NewEventQueue("alpha"))
	m.CreateQueue(NewEventQueue("beta"))
	m.CreateQueue(NewEventQueue("gamma"))

	names := m.QueueNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
}

func TestCleanDeadSubscribers(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10) // alive
	// object 2 does NOT exist

	q := NewEventQueue("test")
	q.Rate = 0
	q.Scope = ScopeGlobal
	m.CreateQueue(q)

	q.Subscribers = []Subscription{
		{Object: 1, Attr: "ON_TEST", Bind: gamedb.Nothing},
		{Object: 2, Attr: "ON_TEST", Bind: gamedb.Nothing}, // dead
	}

	removed := m.CleanDeadSubscribers(g)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if len(q.Subscribers) != 1 {
		t.Errorf("expected 1 subscriber remaining, got %d", len(q.Subscribers))
	}
}

func TestDisabledQueue(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10)
	g.addObject(2, gamedb.Nothing, 20)

	q := NewEventQueue("test")
	q.Rate = 0
	q.Scope = ScopeGlobal
	q.Enabled = false
	m.CreateQueue(q)

	m.Subscribe(g, 2, "ON_TEST", "test", gamedb.Nothing)

	result := m.Publish(g, 1, "test", `{}`)
	if result != "#-1 QUEUE DISABLED" {
		t.Errorf("expected disabled error, got %q", result)
	}
}

func TestDuplicateSubscribe(t *testing.T) {
	m := NewQueueManager()
	g := newMockGame()
	g.addObject(1, gamedb.Nothing, 10)

	q := NewEventQueue("test")
	q.Rate = 0
	m.CreateQueue(q)

	m.Subscribe(g, 1, "ON_TEST", "test", gamedb.Nothing)
	result := m.Subscribe(g, 1, "ON_TEST", "test", gamedb.Nothing)
	if result != "#-1 ALREADY SUBSCRIBED" {
		t.Errorf("expected already subscribed error, got %q", result)
	}
}
