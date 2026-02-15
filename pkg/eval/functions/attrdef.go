package functions

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnLattrdef returns a space-separated list of user-defined attribute names.
// Usage: lattrdef([<pattern>[, <type>]])
// Type can be: player, thing/object, room, exit — filters to attrs present on that type.
// Non-wizards only see VISUAL attribute definitions.
func fnLattrdef(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	pattern := ""
	objType := ""
	if len(args) > 0 {
		pattern = strings.TrimSpace(args[0])
	}
	if len(args) > 1 {
		objType = strings.TrimSpace(args[1])
	}
	buf.WriteString(ctx.GameState.ListAttrDefs(executor, pattern, objType))
}

// fnAttrdefflags returns the definition flags string for a user-defined attribute.
// Usage: attrdefflags(<attrname>)
// Returns flag letters like "IVwp". Non-wizards can only query VISUAL attrs.
func fnAttrdefflags(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil || len(args) < 1 {
		return
	}
	buf.WriteString(ctx.GameState.AttrDefFlags(executor, args[0]))
}

// fnHasattrdef returns 1 if a user-defined or built-in attribute definition exists, 0 otherwise.
// Usage: hasattrdef(<attrname>)
func fnHasattrdef(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil || len(args) < 1 {
		buf.WriteString("0")
		return
	}
	buf.WriteString(ctx.GameState.HasAttrDef(args[0]))
}

// fnSetattrdef modifies flags on a user-defined attribute definition.
// Usage: setattrdef(<attrname>, <flags>)
// Wizard-only. Flags: WIZARD VISUAL NO_INHERIT HIDDEN PROPAGATE etc.
// Prefix with ! to clear: setattrdef(MYATTR, !WIZARD VISUAL)
// Returns empty on success, error string on failure.
func fnSetattrdef(ctx *eval.EvalContext, args []string, buf *strings.Builder, executor, _ gamedb.DBRef) {
	if ctx.GameState == nil || len(args) < 2 {
		buf.WriteString("#-1 FUNCTION REQUIRES 2 ARGUMENTS")
		return
	}
	result := ctx.GameState.SetAttrDefFlags(executor, args[0], args[1])
	buf.WriteString(result)
}
