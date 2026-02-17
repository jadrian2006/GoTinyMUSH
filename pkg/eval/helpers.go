package eval

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// CallIterFun calls a user-defined function specified as "obj/attr" for iteration functions
// (filter, map, fold, step, mix, foreach, while, until, munge, filterbool).
// Unlike CallUFun (used by u()/ulocal()), iteration functions in C TinyMUSH evaluate the
// fetched attr text with the CALLING object as executor (%!), not the target object.
// This means v() inside the callback resolves on the caller, not on obj.
func (ctx *EvalContext) CallIterFun(objAttr string, callArgs []string) string {
	parts := strings.SplitN(objAttr, "/", 2)

	var ref gamedb.DBRef
	var attrName string

	if len(parts) == 2 {
		objStr := strings.TrimSpace(parts[0])
		attrName = strings.ToUpper(strings.TrimSpace(parts[1]))
		ref = ctx.resolveDBRefSimple(objStr)
		if ref == gamedb.Nothing {
			return "#-1 NOT FOUND"
		}
	} else {
		attrName = strings.ToUpper(strings.TrimSpace(objAttr))
		ref = ctx.Player
	}

	if attrName == "" {
		return "#-1 NO SUCH ATTRIBUTE"
	}

	text := ctx.GetAttrByNameHelper(ref, attrName)
	if text == "" {
		return ""
	}

	// Do NOT change ctx.Player — evaluate in the caller's context.
	// C TinyMUSH's filter/map/etc pass 'player' (the executor) to eval_expression_string,
	// not 'thing' (the object where the attr lives).
	return ctx.Exec(text, EvFCheck|EvEval, callArgs)
}

// CallUFun calls a user-defined function specified as "obj/attr" with the given arguments.
// It fetches the attribute text, sets up %0-%9 from callArgs, evaluates it, and returns the result.
func (ctx *EvalContext) CallUFun(objAttr string, callArgs []string) string {
	parts := strings.SplitN(objAttr, "/", 2)

	var ref gamedb.DBRef
	var attrName string

	if len(parts) == 2 {
		// obj/attr format
		objStr := strings.TrimSpace(parts[0])
		attrName = strings.ToUpper(strings.TrimSpace(parts[1]))
		ref = ctx.resolveDBRefSimple(objStr)
		if ref == gamedb.Nothing {
			return "#-1 NOT FOUND"
		}
	} else {
		// Just an attr name — use the executor (%!) as the object
		attrName = strings.ToUpper(strings.TrimSpace(objAttr))
		ref = ctx.Player
	}

	if attrName == "" {
		return "#-1 NO SUCH ATTRIBUTE"
	}

	// Look up the attribute
	text := ctx.GetAttrByNameHelper(ref, attrName)
	if text == "" {
		return ""
	}

	// Evaluate the attribute text with the target object as executor (%!).
	// In TinyMUSH, u(obj/attr) runs the code "as" obj, so unqualified
	// attribute references (v(), u(attr), get(me/attr)) resolve on obj.
	oldPlayer := ctx.Player
	ctx.Player = ref
	result := ctx.Exec(text, EvFCheck|EvEval, callArgs)
	ctx.Player = oldPlayer
	return result
}

// resolveDBRefSimple converts a string to a DBRef (handles #N and *player).
func (ctx *EvalContext) resolveDBRefSimple(s string) gamedb.DBRef {
	s = strings.TrimSpace(s)
	if s == "" {
		return gamedb.Nothing
	}

	// Handle "me"
	if strings.EqualFold(s, "me") {
		return ctx.Player
	}

	// Handle "here"
	if strings.EqualFold(s, "here") {
		if obj, ok := ctx.DB.Objects[ctx.Player]; ok {
			return obj.Location
		}
		return gamedb.Nothing
	}

	// Handle #N
	if s[0] == '#' {
		n := 0
		neg := false
		i := 1
		if i < len(s) && s[i] == '-' {
			neg = true
			i++
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
			i++
		}
		if neg {
			n = -n
		}
		return gamedb.DBRef(n)
	}

	// Handle *player
	if s[0] == '*' {
		s = s[1:]
	}

	// Search by name - players first
	for _, obj := range ctx.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && strings.EqualFold(obj.Name, s) {
			return obj.DBRef
		}
	}

	return gamedb.Nothing
}

// GetAttrByNameHelper fetches an attribute's text value by name from an object.
// Walks the parent chain like TinyMUSH's atr_pget.
func (ctx *EvalContext) GetAttrByNameHelper(ref gamedb.DBRef, attrName string) string {
	// Resolve the attribute number first
	attrNum := -1
	if def, ok := ctx.DB.AttrByName[attrName]; ok {
		attrNum = def.Number
	} else {
		for num, name := range gamedb.WellKnownAttrs {
			if strings.EqualFold(name, attrName) {
				attrNum = num
				break
			}
		}
	}
	if attrNum < 0 {
		return ""
	}

	// Walk the parent chain (up to 10 levels, like TinyMUSH's ITER_PARENTS)
	current := ref
	for depth := 0; depth <= 10; depth++ {
		obj, ok := ctx.DB.Objects[current]
		if !ok {
			return ""
		}
		for _, attr := range obj.Attrs {
			if attr.Number == attrNum {
				// Check read permission if GameState is available
				if ctx.GameState != nil {
					if !ctx.GameState.CanReadAttrGS(ctx.Player, ref, attrNum, attr.Value) {
						return ""
					}
				}
				return StripAttrPrefix(attr.Value)
			}
		}
		if obj.Parent == gamedb.Nothing || obj.Parent == current {
			return ""
		}
		current = obj.Parent
	}
	return ""
}

// AnsiCode maps a single character code to an ANSI escape sequence.
// This is the exported version used by the functions package.
func AnsiCode(ch byte) string {
	switch ch {
	case 'n', 'N':
		return "\033[0m"
	case 'h', 'H':
		return "\033[1m"
	case 'i', 'I':
		return "\033[7m"
	case 'f', 'F':
		return "\033[5m"
	case 'u', 'U':
		return "\033[4m"
	case 'x', 'X':
		return "\033[30m"
	case 'r', 'R':
		return "\033[31m"
	case 'g', 'G':
		return "\033[32m"
	case 'y', 'Y':
		return "\033[33m"
	case 'b', 'B':
		return "\033[34m"
	case 'm', 'M':
		return "\033[35m"
	case 'c', 'C':
		return "\033[36m"
	case 'w', 'W':
		return "\033[37m"
	}
	return ""
}
