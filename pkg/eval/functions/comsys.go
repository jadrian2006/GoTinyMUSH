package functions

import (
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
