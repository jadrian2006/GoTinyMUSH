package functions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// --- Pronoun functions ---
// These read the SEX attribute (attr #7) and return appropriate pronouns.

func getSex(ctx *eval.EvalContext, args []string) string {
	if len(args) < 1 { return "" }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { return "" }
	return strings.ToLower(strings.TrimSpace(getAttrByName(ctx, ref, "SEX")))
}

// fnSubj — subjective pronoun: he/she/they/it
func fnSubj(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	sex := getSex(ctx, args)
	switch {
	case strings.HasPrefix(sex, "m"): buf.WriteString("he")
	case strings.HasPrefix(sex, "f"): buf.WriteString("she")
	case strings.HasPrefix(sex, "p"): buf.WriteString("they")
	default: buf.WriteString("it")
	}
}

// fnObj — objective pronoun: him/her/them/it
func fnObj(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	sex := getSex(ctx, args)
	switch {
	case strings.HasPrefix(sex, "m"): buf.WriteString("him")
	case strings.HasPrefix(sex, "f"): buf.WriteString("her")
	case strings.HasPrefix(sex, "p"): buf.WriteString("them")
	default: buf.WriteString("it")
	}
}

// fnPoss — possessive pronoun: his/her/their/its
func fnPoss(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	sex := getSex(ctx, args)
	switch {
	case strings.HasPrefix(sex, "m"): buf.WriteString("his")
	case strings.HasPrefix(sex, "f"): buf.WriteString("her")
	case strings.HasPrefix(sex, "p"): buf.WriteString("their")
	default: buf.WriteString("its")
	}
}

// fnAposs — absolute possessive pronoun: his/hers/theirs/its
func fnAposs(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	sex := getSex(ctx, args)
	switch {
	case strings.HasPrefix(sex, "m"): buf.WriteString("his")
	case strings.HasPrefix(sex, "f"): buf.WriteString("hers")
	case strings.HasPrefix(sex, "p"): buf.WriteString("theirs")
	default: buf.WriteString("its")
	}
}

// --- Timestamp functions ---

// fnLastaccess — returns last access time as secs since epoch.
func fnLastaccess(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("-1"); return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("-1"); return }
	if obj.LastAccess.IsZero() { buf.WriteString("-1"); return }
	buf.WriteString(strconv.FormatInt(obj.LastAccess.Unix(), 10))
}

// fnLastmod — returns last modification time as secs since epoch.
func fnLastmod(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("-1"); return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("-1"); return }
	if obj.LastMod.IsZero() { buf.WriteString("-1"); return }
	buf.WriteString(strconv.FormatInt(obj.LastMod.Unix(), 10))
}

// fnLastcreate — returns CREATED_TIME attr (attr 203) as secs since epoch.
func fnLastcreate(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("-1"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("-1"); return }
	text := getAttrByName(ctx, ref, "CREATED_TIME")
	if text == "" { buf.WriteString("-1"); return }
	buf.WriteString(text)
}

// fnObjmem — returns rough memory size of an object.
func fnObjmem(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	size := 128 + len(obj.Name) // base struct + name
	for _, attr := range obj.Attrs {
		size += 16 + len(attr.Value) // overhead + value
	}
	writeInt(buf, size)
}

// matchNameAlias checks if searchName matches an object's name or any alias.
// Returns 2 for exact match, 1 for prefix match, 0 for no match.
// Object names use semicolons for aliases: "Radiant Bath;bath;rb"
// Matching follows C TinyMUSH's string_match: search term can match the
// beginning of any word in the name (e.g., "bath" matches "Radiant Bath").
func matchNameAlias(objName, searchName string) int {
	searchLower := strings.ToLower(searchName)
	for _, alias := range strings.Split(objName, ";") {
		alias = strings.TrimSpace(alias)
		aliasLower := strings.ToLower(alias)
		if aliasLower == searchLower {
			return 2 // exact match
		}
		if stringMatch(aliasLower, searchLower) {
			return 1 // prefix/word match
		}
	}
	return 0
}

// stringMatch implements C TinyMUSH's string_match: checks if sub is a prefix
// of any word in src (words separated by non-alphanumeric characters).
// Both src and sub should already be lowercased.
func stringMatch(src, sub string) bool {
	if sub == "" || src == "" {
		return false
	}
	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], sub) {
			return true
		}
		// Skip to end of current word (alphanumeric chars)
		for i < len(src) && isAlnum(src[i]) {
			i++
		}
		// Skip non-alphanumeric chars to start of next word
		for i < len(src) && !isAlnum(src[i]) {
			i++
		}
	}
	return false
}

func isAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// searchContentChain searches a linked list of objects for name/alias/prefix matches.
// Returns the best match (exact wins over prefix, first prefix wins).
func searchContentChain(db *gamedb.Database, first gamedb.DBRef, name string) gamedb.DBRef {
	var prefixMatch gamedb.DBRef = gamedb.Nothing
	next := first
	for next != gamedb.Nothing {
		obj, ok := db.Objects[next]
		if !ok {
			break
		}
		switch matchNameAlias(obj.Name, name) {
		case 2:
			return next // exact match wins immediately
		case 1:
			if prefixMatch == gamedb.Nothing {
				prefixMatch = next
			}
		}
		next = obj.Next
	}
	return prefixMatch
}

func resolveDBRef(ctx *eval.EvalContext, s string) gamedb.DBRef {
	s = strings.TrimSpace(s)
	// Handle "me" and "here"
	if strings.EqualFold(s, "me") {
		return ctx.Player
	}
	if strings.EqualFold(s, "here") {
		if obj, ok := ctx.DB.Objects[ctx.Player]; ok {
			return obj.Location
		}
		return gamedb.Nothing
	}
	if strings.HasPrefix(s, "#") {
		n, err := strconv.Atoi(s[1:])
		if err == nil { return gamedb.DBRef(n) }
	}
	// Try matching by player name (exact only for *name syntax)
	if strings.HasPrefix(s, "*") { s = s[1:] }
	for _, obj := range ctx.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && strings.EqualFold(obj.Name, s) {
			return obj.DBRef
		}
	}
	// Try matching in current location (room contents, inventory, exits)
	if ctx.Player != gamedb.Nothing {
		if pObj, ok := ctx.DB.Objects[ctx.Player]; ok {
			// Search room contents
			loc := pObj.Location
			if locObj, ok := ctx.DB.Objects[loc]; ok {
				if found := searchContentChain(ctx.DB, locObj.Contents, s); found != gamedb.Nothing {
					return found
				}
				// Search exits
				if found := searchContentChain(ctx.DB, locObj.Exits, s); found != gamedb.Nothing {
					return found
				}
			}
			// Search inventory
			if found := searchContentChain(ctx.DB, pObj.Contents, s); found != gamedb.Nothing {
				return found
			}
		}
	}
	return gamedb.Nothing
}

func fnName(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		name := obj.Name
		// Return just the display name (before first ;) — aliases are separated by semicolons
		if idx := strings.IndexByte(name, ';'); idx >= 0 { name = name[:idx] }
		buf.WriteString(name)
	} else {
		buf.WriteString("#-1 NOT FOUND")
	}
}

func fnNum(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	buf.WriteString(fmt.Sprintf("#%d", ref))
}

func fnLoc(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Location))
	} else {
		buf.WriteString("#-1")
	}
}

func fnOwner(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Owner))
	} else {
		buf.WriteString("#-1")
	}
}

func fnType(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(obj.ObjType().String())
	} else {
		buf.WriteString("#-1 NOT FOUND")
	}
}

func fnFlags(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1 NOT FOUND"); return }

	// Build flag string like TinyMUSH does
	switch obj.ObjType() {
	case gamedb.TypeRoom: buf.WriteByte('R')
	case gamedb.TypeExit: buf.WriteByte('E')
	case gamedb.TypePlayer: buf.WriteByte('P')
	default: // THING has no letter
	}
	f1 := obj.Flags[0]
	if f1&gamedb.FlagWizard != 0 { buf.WriteByte('W') }
	if f1&gamedb.FlagDark != 0 { buf.WriteByte('D') }
	if f1&gamedb.FlagHaven != 0 { buf.WriteByte('H') }
	if f1&gamedb.FlagHalt != 0 { buf.WriteByte('h') }
	if f1&gamedb.FlagSafe != 0 { buf.WriteByte('s') }
	if f1&gamedb.FlagInherit != 0 { buf.WriteByte('I') }
	if f1&gamedb.FlagNoSpoof != 0 { buf.WriteByte('N') }
	if f1&gamedb.FlagVisual != 0 { buf.WriteByte('V') }
	if f1&gamedb.FlagOpaque != 0 { buf.WriteByte('O') }
	if f1&gamedb.FlagQuiet != 0 { buf.WriteByte('Q') }
	if f1&gamedb.FlagPuppet != 0 { buf.WriteByte('p') }
	if f1&gamedb.FlagSticky != 0 { buf.WriteByte('S') }
	if f1&gamedb.FlagMonitor != 0 { buf.WriteByte('M') }
	if f1&gamedb.FlagRobot != 0 { buf.WriteByte('r') }
	if f1&gamedb.FlagRoyalty != 0 { buf.WriteByte('Z') }
	if f1&gamedb.FlagEnterOK != 0 { buf.WriteByte('e') }
	if f1&gamedb.FlagLinkOK != 0 { buf.WriteByte('L') }
	if f1&gamedb.FlagJumpOK != 0 { buf.WriteByte('J') }
	if f1&gamedb.FlagVerbose != 0 { buf.WriteByte('v') }
	if f1&gamedb.FlagTerse != 0 { buf.WriteByte('t') }
	if f1&gamedb.FlagTrace != 0 { buf.WriteByte('T') }
	if f1&gamedb.FlagHasStartup != 0 { buf.WriteByte('c') }
	// Flag2
	f2 := obj.Flags[1]
	if f2&gamedb.Flag2Ansi != 0 { buf.WriteByte('X') }
	if f2&gamedb.Flag2Connected != 0 { buf.WriteByte('C') }
	if f2&gamedb.Flag2Unfindable != 0 { buf.WriteByte('U') }
}

// knownFlags maps flag names to [word, bitmask]. Word -1 means type check.
var knownFlags = map[string][2]int{
	"WIZARD": {0, gamedb.FlagWizard}, "DARK": {0, gamedb.FlagDark},
	"HAVEN": {0, gamedb.FlagHaven}, "HALT": {0, gamedb.FlagHalt},
	"SAFE": {0, gamedb.FlagSafe}, "INHERIT": {0, gamedb.FlagInherit},
	"NOSPOOF": {0, gamedb.FlagNoSpoof}, "VISUAL": {0, gamedb.FlagVisual},
	"OPAQUE": {0, gamedb.FlagOpaque}, "QUIET": {0, gamedb.FlagQuiet},
	"PUPPET": {0, gamedb.FlagPuppet}, "STICKY": {0, gamedb.FlagSticky},
	"MONITOR": {0, gamedb.FlagMonitor}, "ROBOT": {0, gamedb.FlagRobot},
	"ROYALTY": {0, gamedb.FlagRoyalty}, "ENTER_OK": {0, gamedb.FlagEnterOK},
	"LINK_OK": {0, gamedb.FlagLinkOK}, "JUMP_OK": {0, gamedb.FlagJumpOK},
	"VERBOSE": {0, gamedb.FlagVerbose}, "TERSE": {0, gamedb.FlagTerse},
	"TRACE": {0, gamedb.FlagTrace}, "MYOPIC": {0, gamedb.FlagMyopic},
	"CHOWN_OK": {0, gamedb.FlagChownOK}, "DESTROY_OK": {0, gamedb.FlagDestroyOK},
	"GOING": {0, gamedb.FlagGoing}, "IMMORTAL": {0, gamedb.FlagImmortal},
	"CONNECTED": {1, gamedb.Flag2Connected}, "ANSI": {1, gamedb.Flag2Ansi},
	"UNFINDABLE": {1, gamedb.Flag2Unfindable}, "ABODE": {1, gamedb.Flag2Abode},
	"PARENT_OK": {1, gamedb.Flag2ParentOK}, "LIGHT": {1, gamedb.Flag2Light},
	"CONTROL_OK": {1, gamedb.Flag2ControlOK}, "SLAVE": {1, gamedb.Flag2Slave},
	"BOUNCE": {1, gamedb.Flag2Bounce}, "STOP": {1, gamedb.Flag2StopMatch},
	"NO_BLEED": {1, gamedb.Flag2NoBLeed}, "GAGGED": {1, gamedb.Flag2Gagged},
	"FIXED": {1, gamedb.Flag2Fixed},
	"PLAYER": {-1, int(gamedb.TypePlayer)}, "ROOM": {-1, int(gamedb.TypeRoom)},
	"EXIT": {-1, int(gamedb.TypeExit)}, "THING": {-1, int(gamedb.TypeThing)},
}

// objHasFlag checks if an object has a named flag.
func objHasFlag(obj *gamedb.Object, flagName string) bool {
	info, ok := knownFlags[flagName]
	if !ok { return false }
	if info[0] == -1 {
		return obj.ObjType() == gamedb.ObjectType(info[1])
	}
	return obj.Flags[info[0]]&info[1] != 0
}

func fnHasflag(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	flagName := strings.ToUpper(strings.TrimSpace(args[1]))
	buf.WriteString(boolToStr(objHasFlag(obj, flagName)))
}

func fnHasattr(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	attrName := strings.ToUpper(strings.TrimSpace(args[1]))
	// Look up attr number
	if def, ok := ctx.DB.AttrByName[attrName]; ok {
		for _, attr := range obj.Attrs {
			if attr.Number == def.Number {
				text := eval.StripAttrPrefix(attr.Value)
				buf.WriteString(boolToStr(text != ""))
				return
			}
		}
	}
	// Also check well-known
	for num, name := range gamedb.WellKnownAttrs {
		if strings.EqualFold(name, attrName) {
			for _, attr := range obj.Attrs {
				if attr.Number == num {
					text := eval.StripAttrPrefix(attr.Value)
					buf.WriteString(boolToStr(text != ""))
					return
				}
			}
		}
	}
	buf.WriteString("0")
}

func fnGet(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	// Format: obj/attr
	parts := strings.SplitN(args[0], "/", 2)
	if len(parts) != 2 { return }
	ref := resolveDBRef(ctx, parts[0])
	attrName := strings.ToUpper(strings.TrimSpace(parts[1]))
	text := getAttrByName(ctx, ref, attrName)
	buf.WriteString(text)
}

func fnXget(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	ref := resolveDBRef(ctx, args[0])
	attrName := strings.ToUpper(strings.TrimSpace(args[1]))
	text := getAttrByName(ctx, ref, attrName)
	buf.WriteString(text)
}

func fnV(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := strings.TrimSpace(args[0])
	if len(s) == 1 {
		ch := strings.ToUpper(s)[0]
		if ch >= 'A' && ch <= 'Z' {
			attrNum := 100 + int(ch-'A') // A_VA = 100 (matches C TinyMUSH constants.h)
			text := ctx.GetAttrText(ctx.Player, attrNum)
			buf.WriteString(text)
			return
		}
	}
	// Generic attribute get
	attrName := strings.ToUpper(s)
	text := getAttrByName(ctx, ctx.Player, attrName)
	buf.WriteString(text)
}

// fnU implements u(obj/attr, arg0, arg1, ...)
func fnU(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	result := ctx.CallUFun(args[0], args[1:])
	buf.WriteString(result)
}

// fnUlocal is like u() but preserves registers
func fnUlocal(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	saved := ctx.RData.Clone()
	result := ctx.CallUFun(args[0], args[1:])
	ctx.RData = saved
	buf.WriteString(result)
}

func fnS(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	// s() just evaluates its argument - it's already evaluated by the time we get here
	buf.WriteString(args[0])
}

func fnObjeval(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	ref := resolveDBRef(ctx, ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil))
	oldPlayer := ctx.Player
	ctx.Player = ref
	result := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval, nil)
	ctx.Player = oldPlayer
	buf.WriteString(result)
}

func fnDefault(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	objAttr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	parts := strings.SplitN(objAttr, "/", 2)
	if len(parts) == 2 {
		ref := resolveDBRef(ctx, parts[0])
		attrName := strings.ToUpper(strings.TrimSpace(parts[1]))
		text := getAttrByName(ctx, ref, attrName)
		if text != "" {
			buf.WriteString(text)
			return
		}
	}
	result := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval, nil)
	buf.WriteString(result)
}

func fnCon(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Contents))
	} else { buf.WriteString("#-1") }
}

func fnExit(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Exits))
	} else { buf.WriteString("#-1") }
}

func fnNext(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Next))
	} else { buf.WriteString("#-1") }
}

func fnLcon(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { return }
	var refs []string
	next := obj.Contents
	for next != gamedb.Nothing {
		refs = append(refs, fmt.Sprintf("#%d", next))
		if nObj, ok := ctx.DB.Objects[next]; ok { next = nObj.Next } else { break }
		if len(refs) > 50000 { break }
	}
	buf.WriteString(strings.Join(refs, " "))
}

func fnLexits(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { return }
	var refs []string
	next := obj.Exits
	for next != gamedb.Nothing {
		refs = append(refs, fmt.Sprintf("#%d", next))
		if nObj, ok := ctx.DB.Objects[next]; ok { next = nObj.Next } else { break }
		if len(refs) > 50000 { break }
	}
	buf.WriteString(strings.Join(refs, " "))
}

func fnLattr(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	// lattr(obj[/pattern])
	s := args[0]
	ref := ctx.Player
	pattern := "*"
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		ref = resolveDBRef(ctx, s[:idx])
		pattern = strings.ToUpper(s[idx+1:])
	} else {
		ref = resolveDBRef(ctx, s)
	}
	obj, ok := ctx.DB.Objects[ref]
	if !ok { return }
	var names []string
	for _, attr := range obj.Attrs {
		name := ctx.DB.GetAttrName(attr.Number)
		if name == "" { name = fmt.Sprintf("ATTR_%d", attr.Number) }
		if pattern == "*" || wildMatch(pattern, name) {
			names = append(names, name)
		}
	}
	buf.WriteString(strings.Join(names, " "))
}

func fnNattr(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		writeInt(buf, len(obj.Attrs))
	} else { buf.WriteString("0") }
}

func fnHome(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1"); return }
	switch obj.ObjType() {
	case gamedb.TypeExit:
		// For exits, home() returns the source room (Exits field)
		buf.WriteString(fmt.Sprintf("#%d", obj.Exits))
	case gamedb.TypeRoom:
		// For rooms, home() returns the dropto (Location field)
		buf.WriteString(fmt.Sprintf("#%d", obj.Location))
	default:
		// For players/things, home() returns Link
		buf.WriteString(fmt.Sprintf("#%d", obj.Link))
	}
}

func fnParent(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Parent))
	} else { buf.WriteString("#-1") }
}

func fnZone(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if obj, ok := ctx.DB.Objects[ref]; ok {
		buf.WriteString(fmt.Sprintf("#%d", obj.Zone))
	} else { buf.WriteString("#-1") }
}

func fnControls(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	controller := resolveDBRef(ctx, args[0])
	target := resolveDBRef(ctx, args[1])
	cObj, ok1 := ctx.DB.Objects[controller]
	tObj, ok2 := ctx.DB.Objects[target]
	if !ok1 || !ok2 { buf.WriteString("0"); return }
	// Simplified: you control it if you own it or you're a wizard
	if cObj.HasFlag(gamedb.FlagWizard) || tObj.Owner == controller || controller == target {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

func fnRoom(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	// Walk up locations until we find a room
	for i := 0; i < 100; i++ {
		obj, ok := ctx.DB.Objects[ref]
		if !ok { buf.WriteString("#-1"); return }
		if obj.ObjType() == gamedb.TypeRoom {
			buf.WriteString(fmt.Sprintf("#%d", ref))
			return
		}
		if obj.Location == gamedb.Nothing { break }
		ref = obj.Location
	}
	buf.WriteString("#-1")
}

// fnElock tests the default lock on an object against a player.
// elock(obj, player) — returns 1 if player passes default lock on obj, 0 otherwise.
// C TinyMUSH's elock() uses get_obj_and_lock() which defaults to A_LOCK (the default lock),
// NOT A_LENTER. elock(obj/enterlock, player) would test the enter lock specifically.
func fnElock(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("0")
		return
	}
	obj := resolveDBRef(ctx, args[0])
	player := resolveDBRef(ctx, args[1])
	if obj == gamedb.Nothing || player == gamedb.Nothing {
		buf.WriteString("0")
		return
	}
	if ctx.GameState != nil {
		if ctx.GameState.EvalObjLock(player, obj, 42) { // A_LOCK = 42 (default lock)
			buf.WriteString("1")
		} else {
			buf.WriteString("0")
		}
	} else {
		buf.WriteString("1") // no game state = pass
	}
}

// fnLockFn returns the text of an object's default lock.
// lock(obj) — returns the lock string.
func fnLockFn(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing {
		buf.WriteString("#-1 NOT FOUND")
		return
	}
	if ctx.GameState != nil {
		text := ctx.GameState.GetAttrTextGS(ref, 38) // A_LOCK = 38
		buf.WriteString(text)
	}
}

// Helper: get attribute text by name (walks parent chain via EvalContext)
func getAttrByName(ctx *eval.EvalContext, ref gamedb.DBRef, attrName string) string {
	return ctx.GetAttrByNameHelper(ref, attrName)
}

// --- Additional object query functions ---

// fnHasattrp — like hasattr but walks the parent chain.
func fnHasattrp(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("0"); return }
	text := getAttrByName(ctx, ref, args[1])
	buf.WriteString(boolToStr(text != ""))
}

// fnFullname returns name(alias) for players, just name for others.
func fnFullname(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("#-1 NOT FOUND"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1 NOT FOUND"); return }
	buf.WriteString(obj.Name)
	if obj.ObjType() == gamedb.TypePlayer {
		alias := getAttrByName(ctx, ref, "ALIAS")
		if alias != "" {
			buf.WriteByte('(')
			buf.WriteString(alias)
			buf.WriteByte(')')
		}
	}
}

// fnGetEval — get(obj/attr) then evaluate the result.
func fnGetEval(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	text := ctx.GetAttrByNameHelper(ctx.Player,args[0])
	if text != "" {
		result := ctx.Exec(text, eval.EvFCheck|eval.EvEval, nil)
		buf.WriteString(result)
	}
}

// fnEdefault — like default() but evaluates the attribute before testing for empty.
func fnEdefault(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	attrSpec := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	text := ctx.GetAttrByNameHelper(ctx.Player,attrSpec)
	if text != "" {
		result := ctx.Exec(text, eval.EvFCheck|eval.EvEval, nil)
		if result != "" {
			buf.WriteString(result)
			return
		}
	}
	result := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval, nil)
	buf.WriteString(result)
}

// fnMoney — returns object's money/pennies.
func fnMoney(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("#-1 NOT FOUND"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1 NOT FOUND"); return }
	writeInt(buf, obj.Pennies)
}

// fnGrep — search attrs on an object for a pattern.
// grep(<object>, <attr-pattern>, <search-pattern>)
func fnGrep(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	grepHelper(ctx, args, buf, false)
}

// fnGrepi — case-insensitive grep.
func fnGrepi(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	grepHelper(ctx, args, buf, true)
}

func grepHelper(ctx *eval.EvalContext, args []string, buf *strings.Builder, caseInsensitive bool) {
	if len(args) < 3 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("#-1 NOT FOUND"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1 NOT FOUND"); return }

	attrPattern := args[1]
	searchPattern := args[2]
	if caseInsensitive { searchPattern = strings.ToLower(searchPattern) }

	var results []string
	for _, attr := range obj.Attrs {
		attrName := ""
		if def, ok := ctx.DB.AttrNames[attr.Number]; ok {
			attrName = def.Name
		} else if wk, ok := gamedb.WellKnownAttrs[attr.Number]; ok {
			attrName = wk
		}
		if attrName == "" { continue }
		if !wildMatch(attrPattern, attrName) { continue }

		text := attr.Value
		// Strip header if present
		if len(text) > 0 && text[0] == '\x01' {
			if colonIdx := strings.Index(text[1:], ":"); colonIdx >= 0 {
				afterOwner := text[1+colonIdx+1:]
				if colonIdx2 := strings.Index(afterOwner, ":"); colonIdx2 >= 0 {
					text = afterOwner[colonIdx2+1:]
				}
			}
		}

		searchIn := text
		if caseInsensitive { searchIn = strings.ToLower(text) }
		if strings.Contains(searchIn, searchPattern) {
			results = append(results, attrName)
		}
	}
	buf.WriteString(strings.Join(results, " "))
}

// fnAndflags — returns 1 if object has ALL specified flags.
// andflags(<object>, <flag-list>)
func fnAndflags(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("0"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	flagStr := args[1]
	for _, ch := range flagStr {
		negate := false
		if ch == '!' {
			negate = true
			continue
		}
		flagName := flagCharToName(byte(ch))
		if flagName == "" { buf.WriteString("0"); return }
		has := objHasFlag(obj, flagName)
		if negate {
			if has { buf.WriteString("0"); return }
		} else {
			if !has { buf.WriteString("0"); return }
		}
	}
	buf.WriteString("1")
}

// fnOrflags — returns 1 if object has ANY of the specified flags.
func fnOrflags(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("0"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	flagStr := args[1]
	for _, ch := range flagStr {
		if ch == '!' { continue }
		flagName := flagCharToName(byte(ch))
		if flagName == "" { continue }
		if objHasFlag(obj, flagName) {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

// flagCharToName maps single-character flag abbreviations to flag names.
func flagCharToName(ch byte) string {
	switch ch {
	case 'W': return "WIZARD"
	case 'r': return "ROYALTY"
	case 'D': return "DARK"
	case 'v': return "VERBOSE"
	case 'V': return "VISUAL"
	case 'H': return "HAVEN"
	case 'h': return "HALT"
	case 'q': return "QUIET"
	case 'S': return "STICKY"
	case 'T': return "TRACE"
	case 'o': return "OPAQUE"
	case 'p': return "PUPPET"
	case 'N': return "NOSPOOF"
	case 'R': return "ROBOT"
	case 'M': return "MONITOR"
	case 'e': return "ENTER_OK"
	case 'l': return "LINK_OK"
	case 'J': return "JUMP_OK"
	case 'c': return "CHOWN_OK"
	case 'd': return "DESTROY_OK"
	case 'A': return "ABODE"
	case 'i': return "INHERIT"
	case 's': return "SAFE"
	case 'G': return "GOING"
	case 'X': return "IMMORTAL"
	case 'C': return "CONNECTED"
	case 'g': return "GAGGED"
	case 'F': return "FIXED"
	case 'b': return "BOUNCE"
	case 'L': return "LIGHT"
	case 'a': return "ANSI"
	case 'U': return "UNFINDABLE"
	case 'P': return "PARENT_OK"
	case 'K': return "CONTROL_OK"
	case 'n': return "NO_BLEED"
	case 'O': return "STOP"
	case 'Z': return "SLAVE"
	case 'B': return "PLAYER"
	default: return ""
	}
}

// fnHasflags — test multiple flags at once using full flag names.
// hasflags(<object>, <flag-list>)
// Flag list is space-separated full flag names: "PLAYER CONNECTED"
// Returns 1 if object has ALL specified flags, 0 otherwise.
func fnHasflags(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("0"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }

	flagStr := strings.TrimSpace(args[1])
	if flagStr == "" { buf.WriteString("1"); return }

	// Try full flag name parsing (space-separated): "PLAYER CONNECTED"
	// If any token is longer than 1 char, use full name mode
	names := strings.Fields(strings.ToUpper(flagStr))
	useFullNames := false
	for _, n := range names {
		if len(n) > 1 {
			useFullNames = true
			break
		}
	}

	if useFullNames {
		for _, name := range names {
			negate := false
			if name[0] == '!' {
				negate = true
				name = name[1:]
			}
			has := objHasFlag(obj, name)
			if negate {
				if has { buf.WriteString("0"); return }
			} else {
				if !has { buf.WriteString("0"); return }
			}
		}
		buf.WriteString("1")
		return
	}

	// Fall back to single-char abbreviation style (andflags behavior)
	fnAndflags(ctx, args, buf, 0, 0)
}

// knownPowers maps power names to [word, bitmask].
var knownPowers = map[string][2]int{
	"ANNOUNCE": {0, gamedb.PowAnnounce}, "BOOT": {0, gamedb.PowBoot},
	"BUILDER": {1, gamedb.Pow2Builder}, "CHOWN_ANYTHING": {0, gamedb.PowChownAny},
	"COMM_ALL": {0, gamedb.PowCommAll}, "CONTROL_ALL": {0, gamedb.PowControlAll},
	"EXPANDED_WHO": {0, gamedb.PowWizardWho}, "FIND_UNFINDABLE": {0, gamedb.PowFindUnfind},
	"FREE_MONEY": {0, gamedb.PowFreeMoney}, "FREE_QUOTA": {0, gamedb.PowFreeQuota},
	"GUEST": {0, gamedb.PowGuest}, "HALT": {0, gamedb.PowHalt},
	"HIDE": {0, gamedb.PowHide}, "IDLE": {0, gamedb.PowIdle},
	"LONG_FINGERS": {0, gamedb.PowLongfingers}, "NO_DESTROY": {0, gamedb.PowNoDestroy},
	"PASS_LOCKS": {0, gamedb.PowPassLocks}, "PROG": {0, gamedb.PowProg},
	"QUOTA": {0, gamedb.PowChgQuotas}, "SEARCH": {0, gamedb.PowSearch},
	"SEE_ALL": {0, gamedb.PowExamAll}, "SEE_HIDDEN": {0, gamedb.PowSeeHidden},
	"SEE_QUEUE": {0, gamedb.PowSeeQueue}, "STAT_ANY": {0, gamedb.PowStatAny},
	"STEAL_MONEY": {0, gamedb.PowSteal}, "TEL_ANYTHING": {0, gamedb.PowTelUnrst},
	"TEL_ANYWHERE": {0, gamedb.PowTelAnywhr}, "UNKILLABLE": {0, gamedb.PowUnkillable},
	"USE_SQL": {1, gamedb.Pow2UseSQL}, "WATCH_LOGINS": {0, gamedb.PowWatch},
	"LINK_TO_ANYTHING": {1, gamedb.Pow2LinkToAny}, "OPEN_ANYWHERE": {1, gamedb.Pow2OpenAnyLoc},
}

// fnHaspower — test if object has a power.
func fnHaspower(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("0"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	powerName := strings.ToUpper(strings.TrimSpace(args[1]))
	info, ok := knownPowers[powerName]
	if !ok { buf.WriteString("0"); return }
	buf.WriteString(boolToStr(obj.HasPower(info[0], info[1])))
}

// fnFindable — returns 1 if <looker> can find <target>.
func fnFindable(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	looker := resolveDBRef(ctx, args[0])
	target := resolveDBRef(ctx, args[1])
	if looker == gamedb.Nothing || target == gamedb.Nothing { buf.WriteString("0"); return }
	// Unfindable flag check
	if tObj, ok := ctx.DB.Objects[target]; ok {
		if objHasFlag(tObj, "UNFINDABLE") {
			// Wizards can still find
			if lObj, ok := ctx.DB.Objects[looker]; ok {
				if !objHasFlag(lObj, "WIZARD") {
					buf.WriteString("0")
					return
				}
			}
		}
	}
	buf.WriteString("1")
}

// fnSees — returns 1 if <looker> can see <target>.
func fnSees(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	looker := resolveDBRef(ctx, args[0])
	target := resolveDBRef(ctx, args[1])
	if looker == gamedb.Nothing || target == gamedb.Nothing { buf.WriteString("0"); return }
	tObj, ok := ctx.DB.Objects[target]
	if !ok { buf.WriteString("0"); return }
	// Dark objects not visible unless wizard/controller
	if objHasFlag(tObj, "DARK") {
		if lObj, ok := ctx.DB.Objects[looker]; ok {
			if objHasFlag(lObj, "WIZARD") || tObj.Owner == looker {
				buf.WriteString("1")
				return
			}
		}
		buf.WriteString("0")
		return
	}
	// Must be in same location or looker controls target
	lObj, ok := ctx.DB.Objects[looker]
	if !ok { buf.WriteString("0"); return }
	if lObj.Location == tObj.Location || tObj.Owner == looker || objHasFlag(lObj, "WIZARD") {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnVisible — returns 1 if <looker> can examine <target>.
func fnVisible(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	looker := resolveDBRef(ctx, args[0])
	target := resolveDBRef(ctx, args[1])
	if looker == gamedb.Nothing || target == gamedb.Nothing { buf.WriteString("0"); return }
	tObj, ok := ctx.DB.Objects[target]
	if !ok { buf.WriteString("0"); return }
	// Owner, wizard, or VISUAL flag
	if lObj, ok := ctx.DB.Objects[looker]; ok {
		if tObj.Owner == looker || objHasFlag(lObj, "WIZARD") || objHasFlag(tObj, "VISUAL") {
			buf.WriteString("1")
			return
		}
	}
	buf.WriteString("0")
}

// fnWhere — returns "true" location: Location for players/things, source room for exits, self for rooms.
func fnWhere(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("#-1"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("#-1 NOT FOUND"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1 NOT FOUND"); return }
	switch obj.ObjType() {
	case gamedb.TypeRoom:
		buf.WriteString(fmt.Sprintf("#%d", ref))
	case gamedb.TypeExit:
		// For exits, where() returns the source room (Exits field)
		buf.WriteString(fmt.Sprintf("#%d", obj.Exits))
	default:
		buf.WriteString(fmt.Sprintf("#%d", obj.Location))
	}
}

// fnXcon — list contents recursively.
func fnXcon(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("#-1 NOT FOUND"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { return }
	var results []string
	var walk func(gamedb.DBRef)
	walk = func(r gamedb.DBRef) {
		o, ok := ctx.DB.Objects[r]
		if !ok { return }
		next := o.Contents
		for next != gamedb.Nothing {
			results = append(results, fmt.Sprintf("#%d", next))
			walk(next)
			nObj, ok := ctx.DB.Objects[next]
			if !ok { break }
			next = nObj.Next
		}
	}
	next := obj.Contents
	for next != gamedb.Nothing {
		results = append(results, fmt.Sprintf("#%d", next))
		walk(next)
		nObj, ok := ctx.DB.Objects[next]
		if !ok { break }
		next = nObj.Next
	}
	buf.WriteString(strings.Join(results, " "))
}

// fnInzone — returns 1 if object is in the specified zone.
func fnInzone(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	zone := resolveDBRef(ctx, args[1])
	if ref == gamedb.Nothing || zone == gamedb.Nothing { buf.WriteString("0"); return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("0"); return }
	buf.WriteString(boolToStr(obj.Zone == zone))
}

// fnZwho — list players in a zone.
func fnZwho(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	zone := resolveDBRef(ctx, args[0])
	if zone == gamedb.Nothing { buf.WriteString("#-1 NOT FOUND"); return }
	var results []string
	for ref, obj := range ctx.DB.Objects {
		if obj.Zone == zone && obj.ObjType() == gamedb.TypePlayer {
			results = append(results, fmt.Sprintf("#%d", ref))
		}
	}
	buf.WriteString(strings.Join(results, " "))
}

// fnZfun — call a function on the zone object.
// zfun(<attr>, <args...>)
func fnZfun(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	// Get the player's zone
	pObj, ok := ctx.DB.Objects[ctx.Player]
	if !ok { return }
	zone := pObj.Zone
	if zone == gamedb.Nothing { return }
	// Get attr from zone
	attrName := strings.ToUpper(args[0])
	text := getAttrByName(ctx, zone, attrName)
	if text == "" { return }
	// Evaluate with args
	cargs := args[1:]
	result := ctx.Exec(text, eval.EvFCheck|eval.EvEval, cargs)
	buf.WriteString(result)
}

// --- RhostMUSH extension object functions ---

// fnLcmds — list $-commands or ^-listen patterns on an object.
// lcmds(object[, delim[, type]]) → list of command patterns
// type: "$" (default) for $-commands, "^" for ^-listen patterns
func fnLcmds(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	target := resolveDBRef(ctx, args[0])
	if target == gamedb.Nothing {
		buf.WriteString("#-1 NOT FOUND"); return
	}
	obj, ok := ctx.DB.Objects[target]
	if !ok {
		buf.WriteString("#-1 NOT FOUND"); return
	}
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	cmdChar := "$"
	if len(args) > 2 && args[2] != "" { cmdChar = args[2] }

	var cmds []string
	for _, attr := range obj.Attrs {
		text := eval.StripAttrPrefix(attr.Value)
		if strings.HasPrefix(text, cmdChar) {
			// Extract command pattern (before the colon)
			colonIdx := strings.IndexByte(text, ':')
			if colonIdx > 0 {
				cmds = append(cmds, text[1:colonIdx])
			}
		}
	}
	buf.WriteString(strings.Join(cmds, delim))
}

// fnAttrcnt — count attributes on an object.
// attrcnt(object[/pattern]) → count
func fnAttrcnt(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	s := args[0]
	var pattern string
	if slashIdx := strings.IndexByte(s, '/'); slashIdx >= 0 {
		pattern = strings.ToUpper(strings.TrimSpace(s[slashIdx+1:]))
		s = s[:slashIdx]
	}
	target := resolveDBRef(ctx, s)
	if target == gamedb.Nothing {
		buf.WriteString("#-1 NOT FOUND"); return
	}
	obj, ok := ctx.DB.Objects[target]
	if !ok {
		buf.WriteString("#-1 NOT FOUND"); return
	}
	count := 0
	for _, attr := range obj.Attrs {
		name := ""
		if n, ok := gamedb.WellKnownAttrs[attr.Number]; ok {
			name = n
		} else {
			if ad, ok := ctx.DB.AttrNames[attr.Number]; ok {
				name = ad.Name
			}
		}
		if pattern == "" || wildMatch(pattern, name) {
			count++
		}
	}
	buf.WriteString(strconv.Itoa(count))
}

// fnIsobjid — check if a string is a valid object ID format (#dbref:timestamp).
// isobjid(string) → 1 or 0
func fnIsobjid(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	s := strings.TrimSpace(args[0])
	if !strings.HasPrefix(s, "#") { buf.WriteString("0"); return }
	colonIdx := strings.IndexByte(s, ':')
	if colonIdx < 0 { buf.WriteString("0"); return }
	dbrefStr := s[1:colonIdx]
	n, err := strconv.Atoi(dbrefStr)
	if err != nil || n < 0 { buf.WriteString("0"); return }
	ref := gamedb.DBRef(n)
	if _, ok := ctx.DB.Objects[ref]; !ok { buf.WriteString("0"); return }
	buf.WriteString("1")
}

// fnSingletime — convert seconds to human-readable time string.
// singletime(seconds) → "X days X hours X minutes X seconds"
func fnSingletime(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0 seconds"); return }
	total := toInt(args[0])
	if total < 0 { total = -total }

	days := total / 86400
	total %= 86400
	hours := total / 3600
	total %= 3600
	mins := total / 60
	secs := total % 60

	parts := []string{}
	if days > 0 {
		if days == 1 { parts = append(parts, "1 day") } else { parts = append(parts, strconv.Itoa(days)+" days") }
	}
	if hours > 0 {
		if hours == 1 { parts = append(parts, "1 hour") } else { parts = append(parts, strconv.Itoa(hours)+" hours") }
	}
	if mins > 0 {
		if mins == 1 { parts = append(parts, "1 minute") } else { parts = append(parts, strconv.Itoa(mins)+" minutes") }
	}
	if secs > 0 || len(parts) == 0 {
		if secs == 1 { parts = append(parts, "1 second") } else { parts = append(parts, strconv.Itoa(secs)+" seconds") }
	}
	buf.WriteString(strings.Join(parts, " "))
}

// fnLrooms — list rooms reachable within N exits from a starting room.
// lrooms(room[, depth]) → space-separated list of room dbrefs
func fnLrooms(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	start := resolveDBRef(ctx, args[0])
	if start == gamedb.Nothing {
		buf.WriteString("#-1 NOT FOUND"); return
	}
	maxDepth := 1
	if len(args) > 1 { maxDepth = toInt(args[1]) }
	if maxDepth < 1 { maxDepth = 1 }
	if maxDepth > 20 { maxDepth = 20 }

	// BFS through exits
	visited := map[gamedb.DBRef]bool{start: true}
	queue := []gamedb.DBRef{start}
	var results []string

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []gamedb.DBRef
		for _, room := range queue {
			roomObj, ok := ctx.DB.Objects[room]
			if !ok { continue }
			// Walk exits chain
			exitRef := roomObj.Exits
			for exitRef != gamedb.Nothing {
				exitObj, ok := ctx.DB.Objects[exitRef]
				if !ok { break }
				dest := exitObj.Location // Exit's location is its destination
				if dest != gamedb.Nothing {
					destObj, ok := ctx.DB.Objects[dest]
					if ok && destObj.ObjType() == gamedb.TypeRoom && !visited[dest] {
						visited[dest] = true
						nextQueue = append(nextQueue, dest)
						results = append(results, fmt.Sprintf("#%d", dest))
					}
				}
				exitRef = exitObj.Next
			}
		}
		queue = nextQueue
	}
	buf.WriteString(strings.Join(results, " "))
}
