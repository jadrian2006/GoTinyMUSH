package functions

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnCwatchrooms returns space-separated dbrefs of rooms a player is watching.
// Usage: cwatchrooms() or cwatchrooms(<player>)
// Non-wizards can only query themselves.
func fnCwatchrooms(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	target := executor
	if len(args) > 0 && args[0] != "" {
		target = resolveDBRef(ctx, strings.TrimSpace(args[0]))
		if target == gamedb.Nothing {
			buf.WriteString("#-1 NO SUCH PLAYER")
			return
		}
	}
	rooms := ctx.GameState.WatchPlayerRooms(executor, target)
	if rooms == nil {
		buf.WriteString("#-1 PERMISSION DENIED")
		return
	}
	parts := make([]string, len(rooms))
	for i, r := range rooms {
		parts[i] = fmt.Sprintf("#%d", r)
	}
	buf.WriteString(strings.Join(parts, " "))
}

// fnIswatching returns 1 if player is watching room, 0 otherwise.
// Usage: iswatching(<player>,<room>)
func fnIswatching(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	player := resolveDBRef(ctx, strings.TrimSpace(args[0]))
	room := resolveDBRef(ctx, strings.TrimSpace(args[1]))
	if player == gamedb.Nothing || room == gamedb.Nothing {
		buf.WriteString("0")
		return
	}
	if ctx.GameState.IsWatching(player, room) {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnWatchsubs returns space-separated dbrefs of subscribers for a watch room.
// Usage: watchsubs(<room>)
// Wizard-only.
func fnWatchsubs(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	room := resolveDBRef(ctx, strings.TrimSpace(args[0]))
	if room == gamedb.Nothing {
		buf.WriteString("#-1 NOT A WATCH ROOM")
		return
	}
	if !ctx.GameState.IsWatchRoom(room) {
		buf.WriteString("#-1 NOT A WATCH ROOM")
		return
	}
	players := ctx.GameState.WatchRoomSubs(executor, room)
	if players == nil {
		buf.WriteString("#-1 PERMISSION DENIED")
		return
	}
	parts := make([]string, len(players))
	for i, p := range players {
		parts[i] = fmt.Sprintf("#%d", p)
	}
	buf.WriteString(strings.Join(parts, " "))
}

// fnIswatchroom returns 1 if room is registered in the watch system, 0 otherwise.
// Usage: iswatchroom(<room>)
func fnIswatchroom(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	room := resolveDBRef(ctx, strings.TrimSpace(args[0]))
	if room == gamedb.Nothing {
		buf.WriteString("0")
		return
	}
	if ctx.GameState.IsWatchRoom(room) {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnWatchcolor returns the ANSI color code for a watch room's prefix.
// Usage: watchcolor(<room>)
func fnWatchcolor(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	room := resolveDBRef(ctx, strings.TrimSpace(args[0]))
	if room == gamedb.Nothing {
		buf.WriteString("#-1 NOT A WATCH ROOM")
		return
	}
	color := ctx.GameState.WatchRoomColor(room)
	if color == "" && !ctx.GameState.IsWatchRoom(room) {
		buf.WriteString("#-1 NOT A WATCH ROOM")
		return
	}
	buf.WriteString(color)
}

// fnWatchformat returns the format string for a watch room's prefix.
// Usage: watchformat(<room>)
// The | character in the format is replaced with the room name at display time.
func fnWatchformat(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	room := resolveDBRef(ctx, strings.TrimSpace(args[0]))
	if room == gamedb.Nothing {
		buf.WriteString("#-1 NOT A WATCH ROOM")
		return
	}
	format := ctx.GameState.WatchRoomFormat(room)
	if format == "" && !ctx.GameState.IsWatchRoom(room) {
		buf.WriteString("#-1 NOT A WATCH ROOM")
		return
	}
	buf.WriteString(format)
}
