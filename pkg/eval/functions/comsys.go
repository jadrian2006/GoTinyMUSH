package functions

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnCinfo returns a field value for a channel.
// Usage: cinfo(<channel>, <field>)
// Fields: owner, description, header, flags, numsent, subscribers, joinlock, translock, recvlock, charge
// Requires caller to be channel owner or Wizard.
func fnCinfo(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil || len(args) < 2 {
		return
	}
	name := strings.TrimSpace(args[0])
	field := strings.TrimSpace(args[1])
	if name == "" || field == "" {
		return
	}
	buf.WriteString(ctx.GameState.ChannelInfo(executor, name, field))
}

// fnComlist returns a space-separated list of visible channels.
// Usage: comlist() or comlist(<separator>)
// C: fun_comlist — shows public channels, owned channels, and comm_all channels.
func fnComlist(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	sep := " "
	if len(args) > 0 && args[0] != "" {
		sep = args[0]
	}
	names := ctx.GameState.ChannelList(executor)
	buf.WriteString(strings.Join(names, sep))
}

// fnCwho returns a space-separated list of connected, listening players on a channel.
// Usage: cwho(<channel>)
// C: fun_cwho — respects Hidden flag.
func fnCwho(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	name := strings.TrimSpace(args[0])
	players := ctx.GameState.ChannelWho(executor, name)
	if players == nil {
		buf.WriteString("#-1 CHANNEL NOT FOUND")
		return
	}
	parts := make([]string, len(players))
	for i, p := range players {
		parts[i] = fmt.Sprintf("#%d", p)
	}
	buf.WriteString(strings.Join(parts, " "))
}

// fnCwhoall returns a space-separated list of all subscribers on a channel.
// Usage: cwhoall(<channel>)
// C: fun_cwhoall — includes disconnected players.
func fnCwhoall(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	name := strings.TrimSpace(args[0])
	players := ctx.GameState.ChannelWhoAll(executor, name)
	if players == nil {
		buf.WriteString("#-1 CHANNEL NOT FOUND")
		return
	}
	parts := make([]string, len(players))
	for i, p := range players {
		parts[i] = fmt.Sprintf("#%d", p)
	}
	buf.WriteString(strings.Join(parts, " "))
}

// fnComowner returns the owner dbref of a channel.
// Usage: comowner(<channel>)
// C: fun_comowner
func fnComowner(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	name := strings.TrimSpace(args[0])
	owner := ctx.GameState.ChannelOwner(name)
	if owner == gamedb.Nothing {
		buf.WriteString("#-1 CHANNEL NOT FOUND")
		return
	}
	buf.WriteString(fmt.Sprintf("#%d", owner))
}

// fnComdesc returns the description of a channel.
// Usage: comdesc(<channel>)
// C: fun_comdesc
func fnComdesc(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	name := strings.TrimSpace(args[0])
	owner := ctx.GameState.ChannelOwner(name)
	if owner == gamedb.Nothing {
		buf.WriteString("#-1 CHANNEL NOT FOUND")
		return
	}
	buf.WriteString(ctx.GameState.ChannelDesc(name))
}

// fnComheader returns the header of a channel.
// Usage: comheader(<channel>)
// C: fun_comheader
func fnComheader(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	name := strings.TrimSpace(args[0])
	owner := ctx.GameState.ChannelOwner(name)
	if owner == gamedb.Nothing {
		buf.WriteString("#-1 CHANNEL NOT FOUND")
		return
	}
	buf.WriteString(ctx.GameState.ChannelHeader(name))
}

// fnComalias returns space-separated alias names for a player.
// Usage: comalias(<player>)
// C: fun_comalias — requires Controls or Comm_All.
func fnComalias(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	target := ctx.GameState.LookupPlayer(args[0])
	if target == gamedb.Nothing {
		buf.WriteString("#-1 NO SUCH PLAYER")
		return
	}
	buf.WriteString(ctx.GameState.PlayerComAliases(executor, target))
}

// fnCominfo returns the channel name for a player's alias.
// Usage: cominfo(<player>, <alias>)
// C: fun_cominfo — requires Controls or Comm_All.
func fnCominfo(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	target := ctx.GameState.LookupPlayer(args[0])
	if target == gamedb.Nothing {
		buf.WriteString("#-1 NO SUCH PLAYER")
		return
	}
	alias := strings.TrimSpace(args[1])
	buf.WriteString(ctx.GameState.PlayerComInfo(executor, target, alias))
}

// fnComtitle returns the title for a player's alias.
// Usage: comtitle(<player>, <alias>)
// C: fun_comtitle — requires Controls or Comm_All.
func fnComtitle(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	target := ctx.GameState.LookupPlayer(args[0])
	if target == gamedb.Nothing {
		buf.WriteString("#-1 NO SUCH PLAYER")
		return
	}
	alias := strings.TrimSpace(args[1])
	buf.WriteString(ctx.GameState.PlayerComTitle(executor, target, alias))
}

// fnCemit sends a message to a channel (function form).
// Usage: cemit(<channel>, <message>)
// C: fun_cemit
func fnCemit(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	name := strings.TrimSpace(args[0])
	message := args[1]
	result := ctx.GameState.ChannelEmit(executor, name, message)
	if result != "" {
		buf.WriteString(result)
	}
}
