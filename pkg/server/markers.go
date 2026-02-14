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

// SendMarkedToRoom sends a message to all connected players in a room,
// wrapping per-player with their configured marker.
func (g *Game) SendMarkedToRoom(room gamedb.DBRef, markerType string, msg string) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	next := roomObj.Contents
	for next != gamedb.Nothing {
		if g.Conns.IsConnected(next) {
			g.SendMarkedToPlayer(next, markerType, msg)
		}
		obj, ok := g.DB.Objects[next]
		if !ok {
			break
		}
		next = obj.Next
	}
}

// SendMarkedToRoomExcept sends a message to all connected players in a room
// except the specified player, wrapping per-player with their configured marker.
func (g *Game) SendMarkedToRoomExcept(room gamedb.DBRef, except gamedb.DBRef, markerType string, msg string) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	next := roomObj.Contents
	for next != gamedb.Nothing {
		if next != except && g.Conns.IsConnected(next) {
			g.SendMarkedToPlayer(next, markerType, msg)
		}
		obj, ok := g.DB.Objects[next]
		if !ok {
			break
		}
		next = obj.Next
	}
}

// EmitEvent sends a structured event to a player via the event bus.
// The event's Text is marker-wrapped for the recipient.
func (g *Game) EmitEvent(player gamedb.DBRef, markerType string, ev events.Event) {
	ev.Player = player
	ev.Text = g.WrapMarker(player, markerType, ev.Text)
	g.EventBus.Emit(ev)
}

// EmitEventToRoom sends a structured event to all connected players in a room.
// Each player's copy has marker-wrapped text.
func (g *Game) EmitEventToRoom(room gamedb.DBRef, markerType string, ev events.Event) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	next := roomObj.Contents
	for next != gamedb.Nothing {
		if g.Conns.IsConnected(next) {
			g.EmitEvent(next, markerType, ev)
		}
		obj, ok := g.DB.Objects[next]
		if !ok {
			break
		}
		next = obj.Next
	}
}

// EmitEventToRoomExcept sends a structured event to all connected players in a
// room except one. Each player's copy has marker-wrapped text.
func (g *Game) EmitEventToRoomExcept(room gamedb.DBRef, except gamedb.DBRef, markerType string, ev events.Event) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	next := roomObj.Contents
	for next != gamedb.Nothing {
		if next != except && g.Conns.IsConnected(next) {
			g.EmitEvent(next, markerType, ev)
		}
		obj, ok := g.DB.Objects[next]
		if !ok {
			break
		}
		next = obj.Next
	}
}
