package events

import (
	"sync"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Subscriber receives events from the bus.
type Subscriber interface {
	Receive(ev Event)
	Closed() bool
}

// Bus is a per-player pub/sub event bus with support for global subscribers.
// Game code emits structured events; each subscriber (Descriptor, scrollback
// writer, logger, etc.) encodes them per-transport.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[gamedb.DBRef][]Subscriber
	global      []Subscriber
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[gamedb.DBRef][]Subscriber),
	}
}

// Subscribe registers a subscriber for a specific player's events.
func (b *Bus) Subscribe(player gamedb.DBRef, sub Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[player] = append(b.subscribers[player], sub)
}

// Unsubscribe removes a subscriber for a specific player.
func (b *Bus) Unsubscribe(player gamedb.DBRef, sub Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[player]
	for i, s := range subs {
		if s == sub {
			b.subscribers[player] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(b.subscribers[player]) == 0 {
		delete(b.subscribers, player)
	}
}

// SubscribeGlobal registers a subscriber that receives all events.
func (b *Bus) SubscribeGlobal(sub Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.global = append(b.global, sub)
}

// Emit sends an event to the player specified in ev.Player and all global subscribers.
func (b *Bus) Emit(ev Event) {
	b.mu.RLock()
	subs := b.subscribers[ev.Player]
	globals := b.global
	b.mu.RUnlock()

	for _, s := range subs {
		if !s.Closed() {
			s.Receive(ev)
		}
	}
	for _, s := range globals {
		if !s.Closed() {
			s.Receive(ev)
		}
	}
}

// EmitToPlayer sends an event to a specific player (overriding ev.Player).
func (b *Bus) EmitToPlayer(player gamedb.DBRef, ev Event) {
	ev.Player = player
	b.Emit(ev)
}

// EmitToRoom sends an event to all connected players in a room.
// It walks the room's contents chain using the database.
func (b *Bus) EmitToRoom(db *gamedb.Database, room gamedb.DBRef, ev Event) {
	roomObj, ok := db.Objects[room]
	if !ok {
		return
	}

	b.mu.RLock()
	globals := b.global
	b.mu.RUnlock()

	seen := make(map[gamedb.DBRef]bool)
	next := roomObj.Contents
	for next != gamedb.Nothing && !seen[next] {
		seen[next] = true
		obj, ok := db.Objects[next]
		if !ok {
			break
		}
		playerEv := ev
		playerEv.Player = next
		playerEv.Room = room

		b.mu.RLock()
		subs := b.subscribers[next]
		b.mu.RUnlock()

		for _, s := range subs {
			if !s.Closed() {
				s.Receive(playerEv)
			}
		}
		next = obj.Next
	}

	// Global subscribers get the original event with Room set
	ev.Room = room
	for _, s := range globals {
		if !s.Closed() {
			s.Receive(ev)
		}
	}
}

// EmitToRoomExcept sends an event to all connected players in a room except one.
func (b *Bus) EmitToRoomExcept(db *gamedb.Database, room gamedb.DBRef, except gamedb.DBRef, ev Event) {
	roomObj, ok := db.Objects[room]
	if !ok {
		return
	}

	b.mu.RLock()
	globals := b.global
	b.mu.RUnlock()

	seen := make(map[gamedb.DBRef]bool)
	next := roomObj.Contents
	for next != gamedb.Nothing && !seen[next] {
		seen[next] = true
		obj, ok := db.Objects[next]
		if !ok {
			break
		}
		if next != except {
			playerEv := ev
			playerEv.Player = next
			playerEv.Room = room

			b.mu.RLock()
			subs := b.subscribers[next]
			b.mu.RUnlock()

			for _, s := range subs {
				if !s.Closed() {
					s.Receive(playerEv)
				}
			}
		}
		next = obj.Next
	}

	ev.Room = room
	for _, s := range globals {
		if !s.Closed() {
			s.Receive(ev)
		}
	}
}

// PlayerSubscribers returns the number of subscribers for a player.
func (b *Bus) PlayerSubscribers(player gamedb.DBRef) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[player])
}

// Cleanup removes closed subscribers from all lists.
func (b *Bus) Cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for player, subs := range b.subscribers {
		var active []Subscriber
		for _, s := range subs {
			if !s.Closed() {
				active = append(active, s)
			}
		}
		if len(active) == 0 {
			delete(b.subscribers, player)
		} else {
			b.subscribers[player] = active
		}
	}

	var activeGlobal []Subscriber
	for _, s := range b.global {
		if !s.Closed() {
			activeGlobal = append(activeGlobal, s)
		}
	}
	b.global = activeGlobal
}
