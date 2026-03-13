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
//
// Supports #lambda/expression syntax: if the object part is "#lambda", the expression
// after the "/" is used directly as the code to evaluate, without any attribute lookup.
func (ctx *EvalContext) CallIterFun(objAttr string, callArgs []string) string {
	parts := strings.SplitN(objAttr, "/", 2)

	// Handle #lambda/expression — inline anonymous function
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "#lambda") {
		text := parts[1]
		if text == "" {
			return ""
		}
		return ctx.Exec(text, EvFCheck|EvEval, callArgs)
	}

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

	// Internal fetch without permission checks — matches C TinyMUSH behavior
	// for iteration functions (filter, map, fold, etc.)
	text := ctx.GetAttrRaw(ref, attrName)
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
//
// Supports #lambda/expression syntax: if the object part is "#lambda", the expression
// after the "/" is used directly as the code to evaluate, without any attribute lookup.
func (ctx *EvalContext) CallUFun(objAttr string, callArgs []string) string {
	parts := strings.SplitN(objAttr, "/", 2)

	// Handle #lambda/expression — inline anonymous function
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "#lambda") {
		text := parts[1]
		if text == "" {
			return ""
		}
		return ctx.Exec(text, EvFCheck|EvEval, callArgs)
	}

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

	// Look up the attribute without permission checks — matches C TinyMUSH's
	// u()/ulocal() which use atr_pget (raw fetch), not See_attr.
	text := ctx.GetAttrRaw(ref, attrName)
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

	// Handle *player — global player lookup
	if s[0] == '*' {
		s = s[1:]
		for _, obj := range ctx.DB.Objects {
			if obj.ObjType() != gamedb.TypePlayer {
				continue
			}
			if strings.EqualFold(obj.Name, s) {
				return obj.DBRef
			}
			for _, attr := range obj.Attrs {
				if attr.Number == 58 {
					aliasStr := StripAttrPrefix(attr.Value)
					if aliasStr != "" {
						for _, a := range strings.Split(aliasStr, ";") {
							if strings.EqualFold(strings.TrimSpace(a), s) {
								return obj.DBRef
							}
						}
					}
					break
				}
			}
		}
		return gamedb.Nothing
	}

	// Try matching in current location (room contents, inventory, exits).
	// Matches C TinyMUSH's match_thing() which searches all local scopes.
	if ctx.Player != gamedb.Nothing {
		if pObj, ok := ctx.DB.Objects[ctx.Player]; ok {
			loc := pObj.Location
			// Search room contents
			if locObj, ok := ctx.DB.Objects[loc]; ok {
				if ref := searchContentChain(ctx.DB, locObj.Contents, s); ref != gamedb.Nothing {
					return ref
				}
				// Search room exits
				if ref := searchContentChain(ctx.DB, locObj.Exits, s); ref != gamedb.Nothing {
					return ref
				}
			}
			// Search inventory
			if ref := searchContentChain(ctx.DB, pObj.Contents, s); ref != gamedb.Nothing {
				return ref
			}
		}
	}

	return gamedb.Nothing
}

// searchContentChain walks a contents/exits chain looking for a name match.
func searchContentChain(db *gamedb.Database, first gamedb.DBRef, name string) gamedb.DBRef {
	current := first
	for current != gamedb.Nothing {
		obj, ok := db.Objects[current]
		if !ok {
			break
		}
		// Check each alias in the semicolon-separated name
		for _, alias := range strings.Split(obj.Name, ";") {
			if strings.EqualFold(strings.TrimSpace(alias), name) {
				return current
			}
		}
		current = obj.Next
	}
	return gamedb.Nothing
}

// GetAttrByNameHelper fetches an attribute's text value by name from an object.
// Walks the parent chain like TinyMUSH's atr_pget. Checks read permissions
// using ctx.Player. Use GetAttrRaw for internal reads (u(), v()) that should
// not be subject to permission checks.
func (ctx *EvalContext) GetAttrByNameHelper(ref gamedb.DBRef, attrName string) string {
	return ctx.getAttrByNameInternal(ref, attrName, true)
}

// GetAttrRaw fetches an attribute's text value by name without permission checks.
// Matches C TinyMUSH's atr_pget behavior used by u()/v()/ulocal()/udefault().
func (ctx *EvalContext) GetAttrRaw(ref gamedb.DBRef, attrName string) string {
	return ctx.getAttrByNameInternal(ref, attrName, false)
}

func (ctx *EvalContext) getAttrByNameInternal(ref gamedb.DBRef, attrName string, checkPerms bool) string {
	// Resolve the attribute number first
	attrName = strings.ToUpper(strings.TrimSpace(attrName))
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

	// Look up master attribute flags for AF_PRIVATE check.
	// Matches C TinyMUSH's atr_pget_str: if master def has AF_PRIVATE, don't
	// walk to parents. If instance on parent has AF_PRIVATE, skip that copy.
	masterFlags := 0
	if def, ok := ctx.DB.AttrByName[attrName]; ok {
		masterFlags = def.Flags
	} else if flags, ok := gamedb.WellKnownAttrFlags[attrNum]; ok {
		masterFlags = flags
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
				// At depth > 0 (on a parent), check instance AF_PRIVATE
				if depth > 0 {
					instFlags := parseInstanceFlags(attr.Value)
					if instFlags&gamedb.AFPrivate != 0 {
						break // skip this parent's copy, try grandparent
					}
				}
				// Check read permission if GameState is available
				if checkPerms && ctx.GameState != nil {
					if !ctx.GameState.CanReadAttrGS(ctx.Player, ref, attrNum, attr.Value) {
						return ""
					}
				}
				return StripAttrPrefix(attr.Value)
			}
		}
		// Before walking to parent, check master definition for AF_PRIVATE
		if depth == 0 && obj.Parent != gamedb.Nothing && obj.Parent != current {
			if masterFlags&gamedb.AFPrivate != 0 {
				return "" // master says don't inherit
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

// AnsiCodeNum maps a single character code to its SGR number string.
// Used by fnAnsi to combine multiple codes into one escape sequence.
func AnsiCodeNum(ch byte) string {
	switch ch {
	case 'n', 'N':
		return "0"
	case 'h', 'H':
		return "1"
	case 'i', 'I':
		return "7"
	case 'f', 'F':
		return "5"
	case 'u', 'U':
		return "4"
	case 'x', 'X':
		return "30"
	case 'r', 'R':
		return "31"
	case 'g', 'G':
		return "32"
	case 'y', 'Y':
		return "33"
	case 'b', 'B':
		return "34"
	case 'm', 'M':
		return "35"
	case 'c', 'C':
		return "36"
	case 'w', 'W':
		return "37"
	}
	return ""
}
