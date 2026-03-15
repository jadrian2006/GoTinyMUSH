package server

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// --- Player Commands ---

// cmdAddwatch subscribes the player to a watched room.
// Syntax: addwatch <room>
func cmdAddwatch(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		g.Notify(d.Player, "Usage: addwatch <room>")
		return
	}
	room := g.MatchObject(d.Player, args)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}
	if !g.Watchsys.IsWatched(room) {
		g.Notify(d.Player, "That room is not in the watch system.")
		return
	}

	sub := &gamedb.WatchSub{
		Player:   d.Player,
		Room:     room,
		IsActive: true,
	}
	if err := g.Watchsys.Subscribe(sub); err != nil {
		g.Notify(d.Player, err.Error())
		return
	}

	// Persist to bolt
	if g.Store != nil {
		g.Store.PutWatchSub(sub)
	}

	roomObj, _ := g.DB.Objects[room]
	g.Notify(d.Player, fmt.Sprintf("Now watching %s.", DisplayName(roomObj.Name)))
}

// cmdDelwatch unsubscribes the player from a watched room.
// Syntax: delwatch <room>
func cmdDelwatch(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		g.Notify(d.Player, "Usage: delwatch <room>")
		return
	}
	room := g.MatchObject(d.Player, args)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}

	if err := g.Watchsys.Unsubscribe(d.Player, room); err != nil {
		g.Notify(d.Player, err.Error())
		return
	}

	// Persist to bolt
	if g.Store != nil {
		g.Store.DeleteWatchSub(d.Player, room)
	}

	roomObj, _ := g.DB.Objects[room]
	g.Notify(d.Player, fmt.Sprintf("No longer watching %s.", DisplayName(roomObj.Name)))
}

// cmdClearwatch removes all watch subscriptions for the player.
// Syntax: clearwatch
func cmdClearwatch(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}

	removed := g.Watchsys.ClearPlayer(d.Player)
	if len(removed) == 0 {
		g.Notify(d.Player, "You are not watching any rooms.")
		return
	}

	// Persist to bolt
	if g.Store != nil {
		g.Store.DeleteWatchSubsForPlayer(d.Player)
	}

	g.Notify(d.Player, fmt.Sprintf("Removed %d watch subscription(s).", len(removed)))
}

// cmdWatchlist lists the player's watch subscriptions.
// Syntax: watchlist
func cmdWatchlist(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}

	subs := g.Watchsys.PlayerSubs(d.Player)
	if len(subs) == 0 {
		g.Notify(d.Player, "You are not watching any rooms. Use 'addwatch <room>' to subscribe.")
		return
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("%-6s %-30s %-10s %s\n", "Room", "Name", "Active", "Quiet"))
	buf.WriteString(strings.Repeat("-", 60) + "\n")
	for _, sub := range subs {
		roomObj, ok := g.DB.Objects[sub.Room]
		name := "(destroyed)"
		if ok {
			name = DisplayName(roomObj.Name)
		}
		active := "on"
		if !sub.IsActive {
			active = "off"
		}
		quiet := ""
		if sub.QuietMode {
			quiet = "quiet"
		}
		buf.WriteString(fmt.Sprintf("#%-5d %-30s %-10s %s\n", sub.Room, name, active, quiet))
	}
	g.Notify(d.Player, buf.String())
}

// cmdAllwatch toggles all watch notifications on or off.
// Syntax: allwatch on|off
func cmdAllwatch(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	args = strings.TrimSpace(strings.ToLower(args))
	switch args {
	case "on":
		g.Watchsys.SetAllQuiet(d.Player, false)
		// Persist all subs
		if g.Store != nil {
			for _, sub := range g.Watchsys.PlayerSubs(d.Player) {
				g.Store.PutWatchSub(sub)
			}
		}
		g.Notify(d.Player, "All watch notifications enabled.")
	case "off":
		g.Watchsys.SetAllQuiet(d.Player, true)
		if g.Store != nil {
			for _, sub := range g.Watchsys.PlayerSubs(d.Player) {
				g.Store.PutWatchSub(sub)
			}
		}
		g.Notify(d.Player, "All watch notifications silenced.")
	default:
		g.Notify(d.Player, "Usage: allwatch on|off")
	}
}

// cmdWatchwho lists who is watching a room.
// Syntax: watchwho <room>
func cmdWatchwho(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		g.Notify(d.Player, "Usage: watchwho <room>")
		return
	}
	room := g.MatchObject(d.Player, args)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}
	if !g.Watchsys.IsWatched(room) {
		g.Notify(d.Player, "That room is not in the watch system.")
		return
	}

	subs := g.Watchsys.RoomSubscribers(room)
	roomObj, _ := g.DB.Objects[room]
	roomName := DisplayName(roomObj.Name)

	if len(subs) == 0 {
		g.Notify(d.Player, fmt.Sprintf("No one is watching %s.", roomName))
		return
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Watchers of %s:\n", roomName))
	for _, sub := range subs {
		pObj, ok := g.DB.Objects[sub.Player]
		if !ok {
			continue
		}
		status := ""
		if !sub.IsActive {
			status = " (off)"
		} else if sub.QuietMode {
			status = " (quiet)"
		}
		online := ""
		if g.Conns.IsConnected(sub.Player) {
			online = " [online]"
		}
		buf.WriteString(fmt.Sprintf("  %s%s%s\n", DisplayName(pObj.Name), status, online))
	}
	g.Notify(d.Player, buf.String())
}

// --- Admin Commands (wizard only) ---

// cmdWregister registers a room in the watch system.
// Syntax: @wregister <room>
func cmdWregister(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		g.Notify(d.Player, "Usage: @wregister <room>")
		return
	}
	room := g.MatchObject(d.Player, args)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}
	// Verify it's a room
	roomObj, ok := g.DB.Objects[room]
	if !ok || roomObj.ObjType() != gamedb.TypeRoom {
		g.Notify(d.Player, "That is not a room.")
		return
	}

	wr := &gamedb.WatchRoom{
		Dbref:  room,
		Format: gamedb.DefaultWatchFormat,
		Owner:  d.Player,
		Flags:  gamedb.WatchPublic,
	}
	if err := g.Watchsys.RegisterRoom(wr); err != nil {
		g.Notify(d.Player, err.Error())
		return
	}

	if g.Store != nil {
		g.Store.PutWatchRoom(wr)
	}

	g.Notify(d.Player, fmt.Sprintf("Room %s (#%d) registered in the watch system.", DisplayName(roomObj.Name), room))
}

// cmdWunregister removes a room from the watch system.
// Syntax: @wunregister <room>
func cmdWunregister(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		g.Notify(d.Player, "Usage: @wunregister <room>")
		return
	}
	room := g.MatchObject(d.Player, args)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}

	removed, err := g.Watchsys.UnregisterRoom(room)
	if err != nil {
		g.Notify(d.Player, err.Error())
		return
	}

	// Persist removal
	if g.Store != nil {
		g.Store.DeleteWatchRoom(room)
		for _, sub := range removed {
			g.Store.DeleteWatchSub(sub.Player, sub.Room)
		}
	}

	roomObj, _ := g.DB.Objects[room]
	g.Notify(d.Player, fmt.Sprintf("Room %s (#%d) unregistered. %d subscription(s) removed.",
		DisplayName(roomObj.Name), room, len(removed)))
}

// cmdWtag sets the display format for a watched room.
// Syntax: @wtag <room>=<format>
// The | character is replaced with the room name at display time.
func cmdWtag(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	parts := strings.SplitN(args, "=", 2)
	if len(parts) != 2 {
		g.Notify(d.Player, "Usage: @wtag <room>=<format>  (use | as room name placeholder)")
		return
	}
	room := g.MatchObject(d.Player, strings.TrimSpace(parts[0]))
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}

	wr := g.Watchsys.GetRoom(room)
	if wr == nil {
		g.Notify(d.Player, "That room is not in the watch system.")
		return
	}

	format := strings.TrimSpace(parts[1])
	if format == "" {
		format = gamedb.DefaultWatchFormat
	}
	wr.Format = format

	if g.Store != nil {
		g.Store.PutWatchRoom(wr)
	}

	g.Notify(d.Player, fmt.Sprintf("Watch format for #%d set to: %s", room, format))
}

// cmdWcolor sets the ANSI color for a watched room's name.
// Syntax: @wcolor <room>=<color>
// Color chars map to %x codes: r=red, g=green, c=cyan, h=hilite, etc.
func cmdWcolor(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	parts := strings.SplitN(args, "=", 2)
	if len(parts) != 2 {
		g.Notify(d.Player, "Usage: @wcolor <room>=<color>  (e.g. c, hr, g)")
		return
	}
	room := g.MatchObject(d.Player, strings.TrimSpace(parts[0]))
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}

	wr := g.Watchsys.GetRoom(room)
	if wr == nil {
		g.Notify(d.Player, "That room is not in the watch system.")
		return
	}

	wr.Color = strings.TrimSpace(parts[1])

	if g.Store != nil {
		g.Store.PutWatchRoom(wr)
	}

	if wr.Color == "" {
		g.Notify(d.Player, fmt.Sprintf("Watch color for #%d cleared.", room))
	} else {
		g.Notify(d.Player, fmt.Sprintf("Watch color for #%d set to: %s", room, wr.Color))
	}
}

// cmdWlist lists all registered watch rooms.
// Syntax: @wlist
func cmdWlist(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	rooms := g.Watchsys.AllRooms()
	if len(rooms) == 0 {
		g.Notify(d.Player, "No rooms are registered in the watch system.")
		return
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("%-8s %-30s %-15s %-6s %s\n", "DBRef", "Name", "Format", "Color", "Subs"))
	buf.WriteString(strings.Repeat("-", 70) + "\n")
	for _, wr := range rooms {
		roomObj, ok := g.DB.Objects[wr.Dbref]
		name := "(destroyed)"
		if ok {
			name = DisplayName(roomObj.Name)
		}
		subs := g.Watchsys.RoomSubscribers(wr.Dbref)
		color := wr.Color
		if color == "" {
			color = "-"
		}
		buf.WriteString(fmt.Sprintf("#%-7d %-30s %-15s %-6s %d\n",
			wr.Dbref, name, wr.Format, color, len(subs)))
	}
	g.Notify(d.Player, buf.String())
}

// cmdWinfo shows detailed info about a watched room.
// Syntax: @winfo <room>
func cmdWinfo(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		g.Notify(d.Player, "Usage: @winfo <room>")
		return
	}
	room := g.MatchObject(d.Player, args)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}

	wr := g.Watchsys.GetRoom(room)
	if wr == nil {
		g.Notify(d.Player, "That room is not in the watch system.")
		return
	}

	roomObj, _ := g.DB.Objects[room]
	ownerObj, _ := g.DB.Objects[wr.Owner]

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Watch Room: %s (#%d)\n", DisplayName(roomObj.Name), room))
	buf.WriteString(fmt.Sprintf("Owner:      %s (#%d)\n", DisplayName(ownerObj.Name), wr.Owner))
	buf.WriteString(fmt.Sprintf("Format:     %s\n", wr.Format))
	color := wr.Color
	if color == "" {
		color = "(none)"
	}
	buf.WriteString(fmt.Sprintf("Color:      %s\n", color))

	subs := g.Watchsys.RoomSubscribers(room)
	buf.WriteString(fmt.Sprintf("Subscribers: %d\n", len(subs)))
	for _, sub := range subs {
		pObj, ok := g.DB.Objects[sub.Player]
		if !ok {
			continue
		}
		status := "active"
		if !sub.IsActive {
			status = "off"
		} else if sub.QuietMode {
			status = "quiet"
		}
		buf.WriteString(fmt.Sprintf("  %s (#%d) [%s]\n", DisplayName(pObj.Name), sub.Player, status))
	}
	g.Notify(d.Player, buf.String())
}

// cmdWboot removes a player's subscription from a watched room.
// Syntax: @wboot <player>=<room>
func cmdWboot(g *Game, d *Descriptor, args string, _ []string) {
	if g.Watchsys == nil {
		g.Notify(d.Player, "The watch system is not enabled.")
		return
	}
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	parts := strings.SplitN(args, "=", 2)
	if len(parts) != 2 {
		g.Notify(d.Player, "Usage: @wboot <player>=<room>")
		return
	}

	playerName := strings.TrimSpace(parts[0])
	target := LookupPlayer(g.DB, playerName)
	if target == gamedb.Nothing {
		// Try *player format
		if playerName[0] == '*' {
			target = LookupPlayer(g.DB, playerName[1:])
		}
	}
	if target == gamedb.Nothing {
		g.Notify(d.Player, "No such player.")
		return
	}

	room := g.MatchObject(d.Player, strings.TrimSpace(parts[1]))
	if room == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that room.")
		return
	}

	if err := g.Watchsys.Unsubscribe(target, room); err != nil {
		g.Notify(d.Player, err.Error())
		return
	}

	if g.Store != nil {
		g.Store.DeleteWatchSub(target, room)
	}

	targetObj, _ := g.DB.Objects[target]
	roomObj, _ := g.DB.Objects[room]
	g.Notify(d.Player, fmt.Sprintf("Removed %s from watching %s.",
		DisplayName(targetObj.Name), DisplayName(roomObj.Name)))
	// Notify the booted player
	g.Notify(target, fmt.Sprintf("You have been removed from watching %s.", DisplayName(roomObj.Name)))
}
