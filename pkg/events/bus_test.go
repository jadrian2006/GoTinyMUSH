package events

import (
	"sync"
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// mockSubscriber implements Subscriber for testing.
type mockSubscriber struct {
	mu       sync.Mutex
	events   []Event
	isClosed bool
}

func (m *mockSubscriber) Receive(ev Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, ev)
}

func (m *mockSubscriber) Closed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isClosed
}

func (m *mockSubscriber) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Event, len(m.events))
	copy(cp, m.events)
	return cp
}

func TestBusEmitToPlayer(t *testing.T) {
	bus := NewBus()
	sub := &mockSubscriber{}

	player := gamedb.DBRef(1)
	bus.Subscribe(player, sub)

	ev := Event{
		Type:   EvSay,
		Player: player,
		Source: player,
		Text:   "Hello world",
	}
	bus.Emit(ev)

	events := sub.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Text != "Hello world" {
		t.Errorf("expected text %q, got %q", "Hello world", events[0].Text)
	}
	if events[0].Type != EvSay {
		t.Errorf("expected type EvSay, got %v", events[0].Type)
	}
}

func TestBusGlobalSubscriber(t *testing.T) {
	bus := NewBus()
	global := &mockSubscriber{}
	bus.SubscribeGlobal(global)

	player := gamedb.DBRef(5)
	ev := Event{Type: EvChannel, Player: player, Channel: "Public", Text: "test msg"}
	bus.Emit(ev)

	events := global.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 global event, got %d", len(events))
	}
	if events[0].Channel != "Public" {
		t.Errorf("expected channel %q, got %q", "Public", events[0].Channel)
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := NewBus()
	sub := &mockSubscriber{}
	player := gamedb.DBRef(1)

	bus.Subscribe(player, sub)
	bus.Unsubscribe(player, sub)

	bus.Emit(Event{Type: EvText, Player: player, Text: "should not arrive"})

	if len(sub.Events()) != 0 {
		t.Error("expected no events after unsubscribe")
	}
}

func TestBusClosedSubscriberSkipped(t *testing.T) {
	bus := NewBus()
	sub := &mockSubscriber{isClosed: true}
	player := gamedb.DBRef(1)

	bus.Subscribe(player, sub)
	bus.Emit(Event{Type: EvText, Player: player, Text: "no delivery"})

	if len(sub.Events()) != 0 {
		t.Error("closed subscriber should not receive events")
	}
}

func TestBusEmitToRoom(t *testing.T) {
	db := gamedb.NewDatabase()

	// Create room #0 with two players #1 and #2 in contents chain
	room := gamedb.DBRef(0)
	p1 := gamedb.DBRef(1)
	p2 := gamedb.DBRef(2)

	db.Objects[room] = &gamedb.Object{DBRef: room, Contents: p1, Exits: gamedb.Nothing, Location: gamedb.Nothing, Next: gamedb.Nothing, Link: gamedb.Nothing, Zone: gamedb.Nothing, Parent: gamedb.Nothing, Owner: gamedb.DBRef(1)}
	db.Objects[p1] = &gamedb.Object{DBRef: p1, Location: room, Next: p2, Contents: gamedb.Nothing, Exits: gamedb.Nothing, Link: gamedb.Nothing, Zone: gamedb.Nothing, Parent: gamedb.Nothing, Owner: p1, Flags: [3]int{int(gamedb.TypePlayer), 0, 0}}
	db.Objects[p2] = &gamedb.Object{DBRef: p2, Location: room, Next: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing, Link: gamedb.Nothing, Zone: gamedb.Nothing, Parent: gamedb.Nothing, Owner: p2, Flags: [3]int{int(gamedb.TypePlayer), 0, 0}}

	bus := NewBus()
	sub1 := &mockSubscriber{}
	sub2 := &mockSubscriber{}
	bus.Subscribe(p1, sub1)
	bus.Subscribe(p2, sub2)

	ev := Event{Type: EvSay, Source: p1, Text: "Hello room"}
	bus.EmitToRoom(db, room, ev)

	if len(sub1.Events()) != 1 {
		t.Errorf("player 1: expected 1 event, got %d", len(sub1.Events()))
	}
	if len(sub2.Events()) != 1 {
		t.Errorf("player 2: expected 1 event, got %d", len(sub2.Events()))
	}
}

func TestBusEmitToRoomExcept(t *testing.T) {
	db := gamedb.NewDatabase()

	room := gamedb.DBRef(0)
	p1 := gamedb.DBRef(1)
	p2 := gamedb.DBRef(2)

	db.Objects[room] = &gamedb.Object{DBRef: room, Contents: p1, Exits: gamedb.Nothing, Location: gamedb.Nothing, Next: gamedb.Nothing, Link: gamedb.Nothing, Zone: gamedb.Nothing, Parent: gamedb.Nothing, Owner: gamedb.DBRef(1)}
	db.Objects[p1] = &gamedb.Object{DBRef: p1, Location: room, Next: p2, Contents: gamedb.Nothing, Exits: gamedb.Nothing, Link: gamedb.Nothing, Zone: gamedb.Nothing, Parent: gamedb.Nothing, Owner: p1, Flags: [3]int{int(gamedb.TypePlayer), 0, 0}}
	db.Objects[p2] = &gamedb.Object{DBRef: p2, Location: room, Next: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing, Link: gamedb.Nothing, Zone: gamedb.Nothing, Parent: gamedb.Nothing, Owner: p2, Flags: [3]int{int(gamedb.TypePlayer), 0, 0}}

	bus := NewBus()
	sub1 := &mockSubscriber{}
	sub2 := &mockSubscriber{}
	bus.Subscribe(p1, sub1)
	bus.Subscribe(p2, sub2)

	ev := Event{Type: EvSay, Source: p1, Text: "Hello others"}
	bus.EmitToRoomExcept(db, room, p1, ev)

	if len(sub1.Events()) != 0 {
		t.Errorf("player 1 (excluded): expected 0 events, got %d", len(sub1.Events()))
	}
	if len(sub2.Events()) != 1 {
		t.Errorf("player 2: expected 1 event, got %d", len(sub2.Events()))
	}
}

func TestBusCleanup(t *testing.T) {
	bus := NewBus()
	active := &mockSubscriber{}
	closed := &mockSubscriber{isClosed: true}
	player := gamedb.DBRef(1)

	bus.Subscribe(player, active)
	bus.Subscribe(player, closed)
	bus.SubscribeGlobal(&mockSubscriber{isClosed: true})

	bus.Cleanup()

	if bus.PlayerSubscribers(player) != 1 {
		t.Errorf("expected 1 active subscriber, got %d", bus.PlayerSubscribers(player))
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		t    EventType
		want string
	}{
		{EvText, "text"},
		{EvSay, "say"},
		{EvChannel, "channel"},
		{EvMove, "move"},
		{EventType(999), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}
