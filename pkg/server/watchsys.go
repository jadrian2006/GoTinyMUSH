package server

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Watchsys manages the room-watch notification system.
type Watchsys struct {
	mu       sync.RWMutex
	Rooms    map[gamedb.DBRef]*gamedb.WatchRoom   // room dbref -> watch config
	Subs     map[gamedb.DBRef][]*gamedb.WatchSub  // room dbref -> subscriber list
	ByPlayer map[gamedb.DBRef][]*gamedb.WatchSub  // player dbref -> their subscriptions
}

// NewWatchsys creates an empty watch system manager.
func NewWatchsys() *Watchsys {
	return &Watchsys{
		Rooms:    make(map[gamedb.DBRef]*gamedb.WatchRoom),
		Subs:     make(map[gamedb.DBRef][]*gamedb.WatchSub),
		ByPlayer: make(map[gamedb.DBRef][]*gamedb.WatchSub),
	}
}

// LoadWatchData populates the watchsys from parsed data.
func (ws *Watchsys) LoadWatchData(rooms []gamedb.WatchRoom, subs []gamedb.WatchSub) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	for i := range rooms {
		ws.Rooms[rooms[i].Dbref] = &rooms[i]
	}
	for i := range subs {
		s := subs[i]
		ws.Subs[s.Room] = append(ws.Subs[s.Room], &s)
		ws.ByPlayer[s.Player] = append(ws.ByPlayer[s.Player], &s)
	}
	log.Printf("watchsys: loaded %d rooms, %d subscriptions (%d players)",
		len(ws.Rooms), len(subs), len(ws.ByPlayer))
}

// GetRoom returns a watch room by dbref.
func (ws *Watchsys) GetRoom(room gamedb.DBRef) *gamedb.WatchRoom {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.Rooms[room]
}

// AllRooms returns a snapshot of all watch rooms.
func (ws *Watchsys) AllRooms() []*gamedb.WatchRoom {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	result := make([]*gamedb.WatchRoom, 0, len(ws.Rooms))
	for _, wr := range ws.Rooms {
		result = append(result, wr)
	}
	return result
}

// IsWatched returns true if a room is registered in the watch system.
func (ws *Watchsys) IsWatched(room gamedb.DBRef) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	_, ok := ws.Rooms[room]
	return ok
}

// PlayerSubs returns all subscriptions for a player.
func (ws *Watchsys) PlayerSubs(player gamedb.DBRef) []*gamedb.WatchSub {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.ByPlayer[player]
}

// RoomSubscribers returns all subscriptions for a room.
func (ws *Watchsys) RoomSubscribers(room gamedb.DBRef) []*gamedb.WatchSub {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.Subs[room]
}

// FindSub finds a player's subscription to a specific room.
func (ws *Watchsys) FindSub(player, room gamedb.DBRef) *gamedb.WatchSub {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	for _, sub := range ws.ByPlayer[player] {
		if sub.Room == room {
			return sub
		}
	}
	return nil
}

// RegisterRoom adds a room to the watch system.
func (ws *Watchsys) RegisterRoom(wr *gamedb.WatchRoom) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if _, exists := ws.Rooms[wr.Dbref]; exists {
		return fmt.Errorf("room #%d is already registered", wr.Dbref)
	}
	ws.Rooms[wr.Dbref] = wr
	return nil
}

// UnregisterRoom removes a room and all its subscriptions.
// Returns the removed subscriptions for bolt cleanup.
func (ws *Watchsys) UnregisterRoom(room gamedb.DBRef) ([]*gamedb.WatchSub, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if _, exists := ws.Rooms[room]; !exists {
		return nil, fmt.Errorf("room #%d is not registered", room)
	}
	delete(ws.Rooms, room)

	// Remove all subscriptions for this room
	removed := ws.Subs[room]
	delete(ws.Subs, room)

	// Remove from ByPlayer index
	for _, sub := range removed {
		ws.removeFromPlayer(sub.Player, room)
	}
	return removed, nil
}

// Subscribe adds a player's subscription to a room.
func (ws *Watchsys) Subscribe(sub *gamedb.WatchSub) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if _, exists := ws.Rooms[sub.Room]; !exists {
		return fmt.Errorf("room #%d is not in the watch system", sub.Room)
	}
	// Check for duplicate
	for _, existing := range ws.ByPlayer[sub.Player] {
		if existing.Room == sub.Room {
			return fmt.Errorf("already watching that room")
		}
	}
	ws.Subs[sub.Room] = append(ws.Subs[sub.Room], sub)
	ws.ByPlayer[sub.Player] = append(ws.ByPlayer[sub.Player], sub)
	return nil
}

// Unsubscribe removes a player's subscription from a room.
func (ws *Watchsys) Unsubscribe(player, room gamedb.DBRef) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Remove from room's subscriber list
	subs := ws.Subs[room]
	found := false
	for i, sub := range subs {
		if sub.Player == player {
			ws.Subs[room] = append(subs[:i], subs[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("not watching that room")
	}
	ws.removeFromPlayer(player, room)
	return nil
}

// ClearPlayer removes all subscriptions for a player.
// Returns removed subs for bolt cleanup.
func (ws *Watchsys) ClearPlayer(player gamedb.DBRef) []*gamedb.WatchSub {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	removed := ws.ByPlayer[player]
	delete(ws.ByPlayer, player)

	// Remove from each room's subscriber list
	for _, sub := range removed {
		subs := ws.Subs[sub.Room]
		for i, s := range subs {
			if s.Player == player {
				ws.Subs[sub.Room] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
	return removed
}

// SetQuiet toggles quiet mode on a single subscription.
func (ws *Watchsys) SetQuiet(player, room gamedb.DBRef, quiet bool) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for _, sub := range ws.ByPlayer[player] {
		if sub.Room == room {
			sub.QuietMode = quiet
			return true
		}
	}
	return false
}

// SetAllQuiet toggles quiet mode on all subscriptions for a player.
func (ws *Watchsys) SetAllQuiet(player gamedb.DBRef, quiet bool) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for _, sub := range ws.ByPlayer[player] {
		sub.QuietMode = quiet
	}
}

// removeFromPlayer removes a room subscription from the ByPlayer index.
// Must be called with mu held.
func (ws *Watchsys) removeFromPlayer(player, room gamedb.DBRef) {
	subs := ws.ByPlayer[player]
	for i, sub := range subs {
		if sub.Room == room {
			ws.ByPlayer[player] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// --- Notification Logic ---

// sanitizeRoomName converts a room name to a valid marker attribute suffix.
// "Hangar Floor" -> "HANGAR_FLOOR"
func sanitizeRoomName(name string) string {
	s := strings.ToUpper(name)
	s = strings.ReplaceAll(s, " ", "_")
	// Strip any non-alphanumeric/underscore chars
	var b strings.Builder
	for _, ch := range s {
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// FormatWatchPrefix builds the notification prefix from a WatchRoom's
// Format and Color settings, pulling the room name live from the db.
func (g *Game) FormatWatchPrefix(wr *gamedb.WatchRoom) string {
	obj, ok := g.DB.Objects[wr.Dbref]
	if !ok {
		return ""
	}
	name := DisplayName(obj.Name)

	if wr.Color != "" {
		// Apply ANSI color codes: each char in Color maps to a %x code
		var codes string
		for i := 0; i < len(wr.Color); i++ {
			codes += eval.AnsiCode(wr.Color[i])
		}
		if codes != "" {
			name = codes + name + "\033[0m"
		}
	}

	return strings.Replace(wr.Format, "|", name, 1)
}

// WrapWatchMarker wraps a watch notification using the player's markers.
// Lookup: MARKER_<ROOM_NAME> → MARKER_WATCH → no extra wrapping.
func (g *Game) WrapWatchMarker(player gamedb.DBRef, roomName string, msg string) string {
	// 1. Try per-room marker: MARKER_HANGAR_FLOOR
	sanitized := sanitizeRoomName(roomName)
	result := g.WrapMarker(player, sanitized, msg)
	if result != msg {
		return result
	}
	// 2. Try global watch marker: MARKER_WATCH
	result = g.WrapMarker(player, "WATCH", msg)
	if result != msg {
		return result
	}
	// 3. No marker — return as-is (already has format prefix)
	return msg
}

// NotifyWatch sends watch notifications for a player entering or leaving a room.
// action should be "arrived" or "left".
// Dark players (DARK flag set) do NOT trigger watch notifications, matching
// the expectation that dark movement is invisible to watchers.
func (g *Game) NotifyWatch(movingPlayer gamedb.DBRef, room gamedb.DBRef, action string) {
	if g.Watchsys == nil {
		return
	}

	wr := g.Watchsys.GetRoom(room)
	if wr == nil {
		return
	}

	subs := g.Watchsys.RoomSubscribers(room)
	if len(subs) == 0 {
		return
	}

	playerObj, ok := g.DB.Objects[movingPlayer]
	if !ok {
		return
	}

	// Dark players don't trigger watch arrival/departure notifications
	if playerObj.HasFlag(gamedb.FlagDark) {
		return
	}

	playerName := DisplayName(playerObj.Name)

	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	roomName := DisplayName(roomObj.Name)

	// Build message: prefix + "PlayerName has arrived/left."
	prefix := g.FormatWatchPrefix(wr)
	innerMsg := fmt.Sprintf("%s%s has %s.", prefix, playerName, action)

	for _, sub := range subs {
		if !sub.IsActive || sub.QuietMode {
			continue
		}
		if sub.Player == movingPlayer {
			continue // don't notify yourself
		}
		if !g.Conns.IsConnected(sub.Player) {
			continue
		}
		// Skip if subscriber is IN the watched room (they see @aenter/@aleave already)
		if subObj, ok := g.DB.Objects[sub.Player]; ok && subObj.Location == room {
			continue
		}
		// Pagelock check: respect the watcher's page lock against the moving player
		if !CouldDoIt(g, movingPlayer, sub.Player, aLPage) {
			continue
		}
		// Wrap with player's per-room or global marker
		wrapped := g.WrapWatchMarker(sub.Player, roomName, innerMsg)
		g.Notify(sub.Player, wrapped)
	}
}

// NotifyWatchActivity forwards room activity (say, pose, emit, etc.) to
// remote watchers who are subscribed to the room but not physically present.
// This matches the C softcode behavior where a ^* listen pattern on watched
// rooms forwards all room messages with a [RoomName]> prefix.
func (g *Game) NotifyWatchActivity(room gamedb.DBRef, source gamedb.DBRef, msg string) {
	if g.Watchsys == nil {
		return
	}

	wr := g.Watchsys.GetRoom(room)
	if wr == nil {
		return
	}

	subs := g.Watchsys.RoomSubscribers(room)
	if len(subs) == 0 {
		return
	}

	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	roomName := DisplayName(roomObj.Name)

	// Build message: prefix + room activity text
	prefix := g.FormatWatchPrefix(wr)
	innerMsg := prefix + msg

	for _, sub := range subs {
		if !sub.IsActive || sub.QuietMode {
			continue
		}
		// Don't forward to the person who generated the activity
		if sub.Player == source {
			continue
		}
		if !g.Conns.IsConnected(sub.Player) {
			continue
		}
		// Skip if subscriber is IN the watched room (they already see it directly)
		if subObj, ok := g.DB.Objects[sub.Player]; ok && subObj.Location == room {
			continue
		}
		// Pagelock check
		if !CouldDoIt(g, source, sub.Player, aLPage) {
			continue
		}
		// Wrap with player's per-room or global marker
		wrapped := g.WrapWatchMarker(sub.Player, roomName, innerMsg)
		g.Notify(sub.Player, wrapped)
	}
}
