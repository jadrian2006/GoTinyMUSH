package server

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// WrapMarker wraps a message with the player's configured marker for the given type.
// markerType is e.g. "SAY", "POSE", "PAGE", "WHISPER", "EMIT", or a channel name.
// The player's MARKER_<TYPE> attribute value has the format "open|close".
// Missing "|" means open prefix only. Empty/missing attribute returns msg unchanged.
func (g *Game) WrapMarker(player gamedb.DBRef, markerType string, msg string) string {
	attrName := "MARKER_" + strings.ToUpper(markerType)
	val := g.GetAttrTextByName(player, attrName)
	if val == "" {
		return msg
	}
	if idx := strings.IndexByte(val, '|'); idx >= 0 {
		return val[:idx] + msg + val[idx+1:]
	}
	return val + msg
}

// SendMarkedToPlayer sends a message to a player, wrapping it with the player's marker.
func (g *Game) SendMarkedToPlayer(player gamedb.DBRef, markerType string, msg string) {
	wrapped := g.WrapMarker(player, markerType, msg)
	g.Conns.SendToPlayer(player, wrapped)
}

// SendMarkedToRoom sends a message to all connected players in a location,
// wrapping per-player with their configured marker.
// Matches C TinyMUSH's notify_all_from_inside(): if the location itself is a
// connected player (e.g. @emit from an object in a player's inventory), the
// player receives the message.
func (g *Game) SendMarkedToRoom(room gamedb.DBRef, markerType string, msg string) {
	// If the location itself is a connected player, notify it
	if g.Conns.IsConnected(room) {
		g.SendMarkedToPlayer(room, markerType, msg)
	}
	for _, next := range g.DB.SafeContents(room) {
		if g.Conns.IsConnected(next) {
			g.SendMarkedToPlayer(next, markerType, msg)
		}
	}
}

// SendMarkedToRoomExcept sends a message to all connected players in a location
// except the specified player, wrapping per-player with their configured marker.
func (g *Game) SendMarkedToRoomExcept(room gamedb.DBRef, except gamedb.DBRef, markerType string, msg string) {
	if room != except && g.Conns.IsConnected(room) {
		g.SendMarkedToPlayer(room, markerType, msg)
	}
	for _, next := range g.DB.SafeContents(room) {
		if next != except && g.Conns.IsConnected(next) {
			g.SendMarkedToPlayer(next, markerType, msg)
		}
	}
}

// EmitEvent sends a structured event to a player via the event bus.
// The event's Text is marker-wrapped for the recipient.
func (g *Game) EmitEvent(player gamedb.DBRef, markerType string, ev events.Event) {
	ev.Player = player
	ev.Text = g.WrapMarker(player, markerType, ev.Text)
	g.EventBus.Emit(ev)
}

// EmitEventToRoom sends a structured event to all connected players in a location.
// Each player's copy has marker-wrapped text.
// If the location itself is a connected player, it receives the event.
func (g *Game) EmitEventToRoom(room gamedb.DBRef, markerType string, ev events.Event) {
	if g.Conns.IsConnected(room) {
		g.EmitEvent(room, markerType, ev)
	}
	for _, next := range g.DB.SafeContents(room) {
		if g.Conns.IsConnected(next) {
			g.EmitEvent(next, markerType, ev)
		}
	}
}

// EmitEventToRoomExcept sends a structured event to all connected players in a
// location except one. Each player's copy has marker-wrapped text.
func (g *Game) EmitEventToRoomExcept(room gamedb.DBRef, except gamedb.DBRef, markerType string, ev events.Event) {
	if room != except && g.Conns.IsConnected(room) {
		g.EmitEvent(room, markerType, ev)
	}
	for _, next := range g.DB.SafeContents(room) {
		if next != except && g.Conns.IsConnected(next) {
			g.EmitEvent(next, markerType, ev)
		}
	}
}
