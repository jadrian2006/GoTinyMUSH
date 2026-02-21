package functions

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Register functions: setq, setr, r

func fnSetq(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		return
	}
	// setq(register, value[, register, value, ...])
	for i := 0; i+1 < len(args); i += 2 {
		regName := strings.TrimSpace(args[i])
		value := args[i+1]
		if len(regName) == 1 {
			idx := qidxChar(regName[0])
			if idx >= 0 && idx < eval.MaxGlobalRegs && ctx.RData != nil {
				ctx.RData.QRegs[idx] = value
			}
		} else if ctx.RData != nil {
			// Named register
			ctx.RData.XRegs[strings.ToLower(regName)] = value
		}
	}
}

func fnSetr(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		return
	}
	regName := strings.TrimSpace(args[0])
	value := args[1]
	if len(regName) == 1 {
		idx := qidxChar(regName[0])
		if idx >= 0 && idx < eval.MaxGlobalRegs && ctx.RData != nil {
			ctx.RData.QRegs[idx] = value
		}
	} else if ctx.RData != nil {
		ctx.RData.XRegs[strings.ToLower(regName)] = value
	}
	buf.WriteString(value)
}

func fnR(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	regName := strings.TrimSpace(args[0])
	if len(regName) == 1 {
		idx := qidxChar(regName[0])
		if idx >= 0 && idx < eval.MaxGlobalRegs && ctx.RData != nil {
			buf.WriteString(ctx.RData.QRegs[idx])
		}
	} else if ctx.RData != nil {
		if val, ok := ctx.RData.XRegs[strings.ToLower(regName)]; ok {
			buf.WriteString(val)
		}
	}
}

// qidxChar converts a register character (0-9, a-z) to an index (0-35).
func qidxChar(ch byte) int {
	if ch >= '0' && ch <= '9' {
		return int(ch - '0')
	}
	if ch >= 'a' && ch <= 'z' {
		return int(ch-'a') + 10
	}
	if ch >= 'A' && ch <= 'Z' {
		return int(ch-'A') + 10
	}
	return -1
}

// Side-effect functions (stubs - record notifications but don't execute)

func fnPemit(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	ctx.Notifications = append(ctx.Notifications, eval.Notification{
		Target:  ref,
		Message: args[1],
	})
}

func fnRemit(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	ctx.Notifications = append(ctx.Notifications, eval.Notification{
		Target:  ref,
		Message: args[1],
		Type:    eval.NotifyRemit,
	})
}

func fnThink(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	ctx.Notifications = append(ctx.Notifications, eval.Notification{
		Target:  ctx.Player,
		Message: args[0],
	})
}

func fnSet(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.GameState == nil {
		return
	}
	first := strings.TrimSpace(args[0])
	second := args[1]

	// Check for obj/attr form
	if slashIdx := strings.IndexByte(first, '/'); slashIdx >= 0 {
		objStr := first[:slashIdx]
		attrName := first[slashIdx+1:]
		ref := resolveDBRef(ctx, objStr)
		if ref == gamedb.Nothing {
			return
		}
		if !ctx.GameState.Controls(ctx.Player, ref) {
			return
		}
		ctx.GameState.SetAttrByName(ref, attrName, second)
		return
	}

	// Flag set/clear: set(obj, FLAG) or set(obj, !FLAG)
	ref := resolveDBRef(ctx, first)
	if ref == gamedb.Nothing {
		return
	}
	if !ctx.GameState.Controls(ctx.Player, ref) {
		return
	}
	ctx.GameState.SetFlag(ref, strings.TrimSpace(second))
}

func fnCreate(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.GameState == nil {
		buf.WriteString("#-1")
		return
	}
	name := strings.TrimSpace(args[0])
	if name == "" {
		buf.WriteString("#-1")
		return
	}

	owner := ctx.Player

	switch {
	case len(args) >= 3 && strings.EqualFold(strings.TrimSpace(args[1]), "e"):
		// create(name, e, dest) — create an exit
		loc := ctx.GameState.PlayerLocation(owner)
		destRef := gamedb.Nothing
		destStr := strings.TrimSpace(args[2])
		if destStr != "" {
			destRef = resolveDBRef(ctx, destStr)
		}
		ref := ctx.GameState.CreateExit(name, loc, destRef, owner)
		buf.WriteString(fmt.Sprintf("#%d", ref))

	case len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[1]), "r"):
		// create(name, r) — create a room
		ref := ctx.GameState.CreateObject(name, gamedb.TypeRoom, owner)
		buf.WriteString(fmt.Sprintf("#%d", ref))

	default:
		// create(name) — create a thing, place in player inventory
		ref := ctx.GameState.CreateObject(name, gamedb.TypeThing, owner)
		obj, ok := ctx.DB.Objects[ref]
		if !ok {
			buf.WriteString("#-1")
			return
		}
		// Place in player's inventory
		obj.Location = owner
		if pObj, ok := ctx.DB.Objects[owner]; ok {
			obj.Next = pObj.Contents
			pObj.Contents = ref
		}
		buf.WriteString(fmt.Sprintf("#%d", ref))
	}
}

func fnTel(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.GameState == nil {
		return
	}
	victim := resolveDBRef(ctx, args[0])
	dest := resolveDBRef(ctx, args[1])
	if victim == gamedb.Nothing || dest == gamedb.Nothing {
		return
	}
	if !ctx.GameState.Controls(ctx.Player, victim) {
		return
	}
	ctx.GameState.Teleport(victim, dest)
}

func fnLink(_ *eval.EvalContext, _ []string, _ *strings.Builder, _, _ gamedb.DBRef) {
	// Stub - would link object
}

func fnTrigger(_ *eval.EvalContext, _ []string, _ *strings.Builder, _, _ gamedb.DBRef) {
	// Stub - would trigger an attribute
}

func fnWipe(_ *eval.EvalContext, _ []string, _ *strings.Builder, _, _ gamedb.DBRef) {
	// Stub - would wipe attributes
}

func fnForce(_ *eval.EvalContext, _ []string, _ *strings.Builder, _, _ gamedb.DBRef) {
	// Stub - would force object to execute command
}

func fnWait(_ *eval.EvalContext, _ []string, _ *strings.Builder, _, _ gamedb.DBRef) {
	// Stub - would queue a delayed command
}

// Utility functions

func fnNull(_ *eval.EvalContext, _ []string, _ *strings.Builder, _, _ gamedb.DBRef) {
	// Evaluates args (already done) but returns nothing
}

func fnLit(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	// Return argument literally (no-eval flag means it wasn't evaluated)
	buf.WriteString(args[0])
}

func fnSubeval(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	result := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	buf.WriteString(result)
}

// Random functions

func fnRand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("0")
		return
	}
	n := toInt(args[0])
	if n <= 0 {
		buf.WriteString("0")
		return
	}
	writeInt(buf, rand.IntN(n))
}

func fnDie(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("0")
		return
	}
	n := toInt(args[0])     // number of dice
	sides := toInt(args[1]) // sides per die
	if n <= 0 || sides <= 0 {
		buf.WriteString("0")
		return
	}
	total := 0
	for i := 0; i < n; i++ {
		total += rand.IntN(sides) + 1
	}
	writeInt(buf, total)
}

func fnLrand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		return
	}
	bot := toInt(args[0])
	top := toInt(args[1])
	count := toInt(args[2])
	if count < 1 || top < bot {
		return
	}
	if count > 10000 {
		count = 10000
	}
	sep := " "
	if len(args) > 3 && args[3] != "" {
		sep = args[3]
	}
	span := top - bot + 1
	for i := 0; i < count; i++ {
		if i > 0 {
			buf.WriteString(sep)
		}
		writeInt(buf, bot+rand.IntN(span))
	}
}

// Time functions

func fnTime(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(time.Now().Format("Mon Jan 02 15:04:05 2006"))
}

func fnSecs(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
}

func fnConvsecs(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	secs, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
	if err != nil {
		buf.WriteString("#-1 INVALID ARGUMENT")
		return
	}
	t := time.Unix(secs, 0)
	buf.WriteString(t.Format("Mon Jan 02 15:04:05 2006"))
}

func fnConvtime(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("-1")
		return
	}
	// Try common MUSH time format
	layouts := []string{
		"Mon Jan 02 15:04:05 2006",
		"Mon Jan 2 15:04:05 2006",
		time.RFC1123,
		time.RFC1123Z,
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, strings.TrimSpace(args[0]))
		if err == nil {
			buf.WriteString(strconv.FormatInt(t.Unix(), 10))
			return
		}
	}
	buf.WriteString("-1")
}

func fnTimefmt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	format := args[0]
	t := time.Now()
	if len(args) > 1 {
		secs, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
		if err == nil {
			t = time.Unix(secs, 0)
		}
	}
	// Convert strftime-style format to Go format
	buf.WriteString(strftimeToGo(format, t))
}

// strftimeToGo converts a C-style strftime format string using time.Time
func strftimeToGo(format string, t time.Time) string {
	var out strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			i++
			switch format[i] {
			case 'Y':
				out.WriteString(t.Format("2006"))
			case 'y':
				out.WriteString(t.Format("06"))
			case 'm':
				out.WriteString(t.Format("01"))
			case 'd':
				out.WriteString(t.Format("02"))
			case 'H':
				out.WriteString(t.Format("15"))
			case 'M':
				out.WriteString(t.Format("04"))
			case 'S':
				out.WriteString(t.Format("05"))
			case 'A':
				out.WriteString(t.Format("Monday"))
			case 'a':
				out.WriteString(t.Format("Mon"))
			case 'B':
				out.WriteString(t.Format("January"))
			case 'b', 'h':
				out.WriteString(t.Format("Jan"))
			case 'p':
				out.WriteString(t.Format("PM"))
			case 'I':
				out.WriteString(t.Format("03"))
			case 'c':
				out.WriteString(t.Format("Mon Jan 02 15:04:05 2006"))
			case 'x':
				out.WriteString(t.Format("01/02/06"))
			case 'X':
				out.WriteString(t.Format("15:04:05"))
			case 'Z':
				out.WriteString(t.Format("MST"))
			case 'j':
				out.WriteString(fmt.Sprintf("%03d", t.YearDay()))
			case 'w':
				out.WriteString(strconv.Itoa(int(t.Weekday())))
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case '%':
				out.WriteByte('%')
			default:
				out.WriteByte('%')
				out.WriteByte(format[i])
			}
		} else {
			out.WriteByte(format[i])
		}
	}
	return out.String()
}

// Info functions

func fnVersion(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.VersionStr != "" {
		buf.WriteString(ctx.VersionStr)
	} else {
		buf.WriteString("GoTinyMUSH")
	}
}

func fnMudname(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.MudName != "" {
		buf.WriteString(ctx.MudName)
	} else {
		buf.WriteString("GoTinyMUSH")
	}
}

// --- Register functions ---

// fnX — read a named variable: x(name)
func fnX(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.RData == nil { return }
	name := strings.ToLower(strings.TrimSpace(args[0]))
	if val, ok := ctx.RData.XRegs[name]; ok {
		buf.WriteString(val)
	}
}

// fnSetx — set a named register: setx(name, value)
func fnSetx(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.RData == nil { return }
	name := strings.ToLower(strings.TrimSpace(args[0]))
	ctx.RData.XRegs[name] = args[1]
}

// fnLregs — list all set register names.
func fnLregs(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.RData == nil { return }
	var names []string
	for i := 0; i < eval.MaxGlobalRegs; i++ {
		if ctx.RData.QRegs[i] != "" {
			if i < 10 {
				names = append(names, string(rune('0'+i)))
			} else {
				names = append(names, string(rune('a'+i-10)))
			}
		}
	}
	for k := range ctx.RData.XRegs {
		names = append(names, k)
	}
	buf.WriteString(strings.Join(names, " "))
}

// fnQvars — set multiple q-registers from a list.
// qvars(list[, delim])
func fnQvars(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.RData == nil { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	values := splitList(args[0], delim)
	for i, val := range values {
		if i >= eval.MaxGlobalRegs { break }
		ctx.RData.QRegs[i] = val
	}
}

// fnXvars — set multiple named variables from a list.
// xvars(name-list, value-list[, delim])
func fnXvars(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.RData == nil { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	names := splitList(args[0], delim)
	values := splitList(args[1], delim)
	for i, name := range names {
		val := ""
		if i < len(values) { val = values[i] }
		ctx.RData.XRegs[strings.ToLower(strings.TrimSpace(name))] = val
	}
}

// fnClearvars — clear all registers.
func fnClearvars(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.RData == nil { return }
	for i := range ctx.RData.QRegs {
		ctx.RData.QRegs[i] = ""
	}
	ctx.RData.XRegs = make(map[string]string)
}

// fnLet — set registers then evaluate expression, restoring after.
// let(reg1, val1, ..., regN, valN, expr)
func fnLet(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	saved := ctx.RData.Clone()
	// Set register pairs
	for i := 0; i+2 <= len(args)-1; i += 2 {
		regName := strings.TrimSpace(ctx.Exec(args[i], eval.EvFCheck|eval.EvEval, nil))
		value := ctx.Exec(args[i+1], eval.EvFCheck|eval.EvEval, nil)
		if len(regName) == 1 {
			idx := qidxChar(regName[0])
			if idx >= 0 && idx < eval.MaxGlobalRegs && ctx.RData != nil {
				ctx.RData.QRegs[idx] = value
			}
		} else if ctx.RData != nil {
			ctx.RData.XRegs[strings.ToLower(regName)] = value
		}
	}
	// Last arg is the expression
	result := ctx.Exec(args[len(args)-1], eval.EvFCheck|eval.EvEval, nil)
	buf.WriteString(result)
	ctx.RData = saved
}

// fnLocalize — evaluate expression, then restore registers.
func fnLocalize(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	saved := ctx.RData.Clone()
	result := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	ctx.RData = saved
	buf.WriteString(result)
}

// fnPrivate — same as localize() in our implementation.
func fnPrivate(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnLocalize(ctx, args, buf, caller, cause)
}

// fnUprivate — like ulocal() — u() with register preservation.
func fnUprivate(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	saved := ctx.RData.Clone()
	result := ctx.CallUFun(args[0], args[1:])
	ctx.RData = saved
	buf.WriteString(result)
}

// --- Regex functions ---

// fnRegmatch — match a string against a regex, optionally capturing groups.
// regmatch(string, pattern[, register-list])
func fnRegmatch(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regmatchHelper(ctx, args, buf, false)
}

// fnRegmatchi — case-insensitive regmatch.
func fnRegmatchi(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regmatchHelper(ctx, args, buf, true)
}

func regmatchHelper(ctx *eval.EvalContext, args []string, buf *strings.Builder, caseInsensitive bool) {
	if len(args) < 2 { buf.WriteString("0"); return }
	pattern := args[1]
	if caseInsensitive { pattern = "(?i)" + pattern }
	re, err := regexp.Compile(pattern)
	if err != nil { buf.WriteString("0"); return }
	matches := re.FindStringSubmatch(args[0])
	if matches == nil { buf.WriteString("0"); return }
	// Store captures in registers if register list provided
	if len(args) > 2 && ctx.RData != nil {
		regs := strings.Fields(args[2])
		for i, regName := range regs {
			if i >= len(matches) { break }
			regName = strings.TrimSpace(regName)
			if len(regName) == 1 {
				idx := qidxChar(regName[0])
				if idx >= 0 && idx < eval.MaxGlobalRegs {
					ctx.RData.QRegs[idx] = matches[i]
				}
			} else {
				ctx.RData.XRegs[strings.ToLower(regName)] = matches[i]
			}
		}
	}
	buf.WriteString("1")
}

// fnRegedit — regex-based search and replace.
// regedit(string, pattern, replacement[, pattern, replacement, ...])
func fnRegedit(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regeditHelper(args, buf, false)
}

// fnRegediti — case-insensitive regedit.
func fnRegediti(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regeditHelper(args, buf, true)
}

func regeditHelper(args []string, buf *strings.Builder, caseInsensitive bool) {
	if len(args) < 3 { return }
	result := args[0]
	for i := 1; i+1 < len(args); i += 2 {
		pattern := args[i]
		if caseInsensitive { pattern = "(?i)" + pattern }
		re, err := regexp.Compile(pattern)
		if err != nil { continue }
		result = re.ReplaceAllString(result, args[i+1])
	}
	buf.WriteString(result)
}

// fnRegeditall — regex-based search and replace (all occurrences, same as regedit in Go).
func fnRegeditall(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regeditHelper(args, buf, false)
}

// fnRegeditalli — case-insensitive regeditall.
func fnRegeditalli(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regeditHelper(args, buf, true)
}

// fnRegrab — grab first list element matching regex.
// regrab(list, pattern[, delim])
func fnRegrab(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regrabHelper(args, buf, false, false)
}

// fnRegrabi — case-insensitive regrab.
func fnRegrabi(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regrabHelper(args, buf, true, false)
}

// fnRegraball — grab all list elements matching regex.
func fnRegraball(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regrabHelper(args, buf, false, true)
}

// fnRegraballi — case-insensitive regraball.
func fnRegraballi(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regrabHelper(args, buf, true, true)
}

func regrabHelper(args []string, buf *strings.Builder, caseInsensitive, all bool) {
	if len(args) < 2 { return }
	pattern := args[1]
	if caseInsensitive { pattern = "(?i)" + pattern }
	re, err := regexp.Compile(pattern)
	if err != nil { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	var results []string
	for _, w := range words {
		if re.MatchString(w) {
			if !all {
				buf.WriteString(w)
				return
			}
			results = append(results, w)
		}
	}
	buf.WriteString(strings.Join(results, delim))
}

// fnRegrep — search attributes on an object for a regex pattern.
// regrep(object, attr-pattern, search-regex)
func fnRegrep(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regrepHelper(ctx, args, buf, false)
}

// fnRegrepi — case-insensitive regrep.
func fnRegrepi(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regrepHelper(ctx, args, buf, true)
}

func regrepHelper(ctx *eval.EvalContext, args []string, buf *strings.Builder, caseInsensitive bool) {
	if len(args) < 3 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { return }
	pattern := args[2]
	if caseInsensitive { pattern = "(?i)" + pattern }
	re, err := regexp.Compile(pattern)
	if err != nil { return }
	attrPattern := args[1]
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
		text := eval.StripAttrPrefix(attr.Value)
		if re.MatchString(text) {
			results = append(results, attrName)
		}
	}
	buf.WriteString(strings.Join(results, " "))
}

// --- Stack functions ---
// TinyMUSH per-object stacks are stored in the eval context for simplicity.
// We use a per-context stack stored as a string list in a special named register.

func getStack(ctx *eval.EvalContext) []string {
	if ctx.RData == nil { return nil }
	s, ok := ctx.RData.XRegs["__stack"]
	if !ok || s == "" { return nil }
	return strings.Split(s, "\x00")
}

func setStack(ctx *eval.EvalContext, stack []string) {
	if ctx.RData == nil { return }
	if len(stack) == 0 {
		delete(ctx.RData.XRegs, "__stack")
	} else {
		ctx.RData.XRegs["__stack"] = strings.Join(stack, "\x00")
	}
}

// fnPush — push a value onto the stack.
func fnPush(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	stack := getStack(ctx)
	stack = append([]string{args[0]}, stack...)
	setStack(ctx, stack)
}

// fnPop — pop a value from the stack.
func fnPop(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	if len(stack) == 0 { return }
	buf.WriteString(stack[0])
	setStack(ctx, stack[1:])
}

// fnPeek — peek at the top of the stack without removing.
func fnPeek(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	if len(stack) == 0 { return }
	idx := 0
	if len(args) > 0 { idx = toInt(args[0]) }
	if idx < 0 || idx >= len(stack) { return }
	buf.WriteString(stack[idx])
}

// fnEmpty — returns 1 if stack is empty.
func fnEmpty(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	buf.WriteString(boolToStr(len(stack) == 0))
}

// fnLstack — list all stack items.
func fnLstack(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	delim := " "
	if len(args) > 0 && args[0] != "" { delim = args[0] }
	buf.WriteString(strings.Join(stack, delim))
}

// fnDup — duplicate top stack element.
func fnDup(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	if len(stack) == 0 { return }
	stack = append([]string{stack[0]}, stack...)
	setStack(ctx, stack)
}

// fnSwap — swap top two stack elements.
func fnSwap(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	if len(stack) < 2 { return }
	stack[0], stack[1] = stack[1], stack[0]
	setStack(ctx, stack)
}

// fnPopn — pop N elements from stack, return as list.
func fnPopn(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	n := 1
	if len(args) > 0 { n = toInt(args[0]) }
	stack := getStack(ctx)
	if n > len(stack) { n = len(stack) }
	if n <= 0 { return }
	var results []string
	for i := 0; i < n; i++ {
		results = append(results, stack[i])
	}
	setStack(ctx, stack[n:])
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	buf.WriteString(strings.Join(results, delim))
}

// fnToss — pop and discard top element (like pop but no return).
func fnToss(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	stack := getStack(ctx)
	if len(stack) == 0 { return }
	setStack(ctx, stack[1:])
}

// --- Misc functions ---

var startTime = time.Now()

// fnStarttime — returns the server start time as epoch secs.
func fnStarttime(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(strconv.FormatInt(startTime.Unix(), 10))
}

// fnRestarttime — returns the server restart time (same as start for us).
func fnRestarttime(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(strconv.FormatInt(startTime.Unix(), 10))
}

// fnPorts — returns ports a player is connected from.
func fnPorts(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	// Stub: we don't track per-port info, return empty
}

// fnConnrecord — returns the peak connections count.
func fnConnrecord(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	// Stub
	buf.WriteString("0")
}

// fnFcount — returns function invocation counter.
func fnFcount(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	writeInt(buf, ctx.FuncInvkCtr)
}

// fnFdepth — returns current function nesting depth.
func fnFdepth(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	writeInt(buf, ctx.FuncNestLev)
}

// fnConfig — returns a configuration parameter value. Stub for now.
func fnConfig(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	// Stub: return empty for unknown config params
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "mud_name":
		buf.WriteString("GoTinyMUSH")
	case "port":
		buf.WriteString("6250")
	default:
		buf.WriteString("")
	}
}

// fnEvalFn — evaluate with extended argument passing.
// eval(obj, attr) — get attr from obj and evaluate it
func fnEvalFn(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { return }
	attrName := strings.ToUpper(strings.TrimSpace(args[1]))
	text := getAttrByName(ctx, ref, attrName)
	if text == "" { return }
	result := ctx.Exec(text, eval.EvFCheck|eval.EvEval, nil)
	buf.WriteString(result)
}

// fnBeep — send beep character (wizard-only, stub).
func fnBeep(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteByte('\a')
}

// fnSearch — search the database by criteria.
// search([player] [class]=<restriction>[,<low>[,<high>]])
// lsearch([player] [class]=<restriction>[,<low>[,<high>]])
// Implements the C TinyMUSH search_setup/search_perform logic.
func fnSearch(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	// Handle both search() and lsearch() arg formats:
	// search("all type=player")        — single arg with space-separated player/class
	// search("all type=player,0,100")  — single arg with range
	// lsearch(all, type, player)       — comma-separated: player, class, restriction
	// lsearch(all, type, player, 0, 100) — with range
	var raw string
	if len(args) == 0 {
		raw = "" // No args: search own objects (wizard defaults to all)
	} else if len(args) >= 3 && !strings.Contains(args[0], "=") && !strings.Contains(args[1], "=") {
		// Comma-separated format: args[0]=player, args[1]=class, args[2]=restriction[, low[, high]]
		raw = strings.TrimSpace(args[0]) + " " + strings.TrimSpace(args[1]) + "=" + strings.TrimSpace(args[2])
		if len(args) >= 4 {
			raw += "," + strings.TrimSpace(args[3])
		}
		if len(args) >= 5 {
			raw += "," + strings.TrimSpace(args[4])
		}
	} else {
		// Standard single-arg format, rejoin any extra args as comma-separated range
		raw = strings.TrimSpace(args[0])
		for i := 1; i < len(args); i++ {
			raw += "," + args[i]
		}
	}

	// Determine if caller is a wizard
	callerObj, callerOK := ctx.DB.Objects[ctx.Player]
	isWiz := callerOK && callerObj.Flags[0]&gamedb.FlagWizard != 0

	// Parse the search specification
	// Format: [player] [class]=<restriction>[,<low>[,<high>]]
	var ownerRef gamedb.DBRef = ctx.Player // default: search own objects
	searchAll := false
	searchClass := ""
	restriction := ""
	lowBound := gamedb.DBRef(0)
	highBound := gamedb.DBRef(len(ctx.DB.Objects) - 1)
	filterType := gamedb.ObjectType(-1) // -1 means no type filter

	// Split on first '=' to get left side and right side
	eqIdx := strings.Index(raw, "=")
	var leftSide, rightSide string
	if eqIdx >= 0 {
		leftSide = strings.TrimSpace(raw[:eqIdx])
		rightSide = raw[eqIdx+1:]
	} else {
		// No '=' — treat entire thing as left side (e.g. "all" or player name)
		leftSide = raw
		rightSide = ""
	}

	// Parse low,high from right side (comma-separated after restriction)
	if rightSide != "" {
		parts := strings.SplitN(rightSide, ",", 3)
		restriction = strings.TrimSpace(parts[0])
		if len(parts) >= 2 {
			if v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(parts[1], "#"))); err == nil {
				lowBound = gamedb.DBRef(v)
			}
		}
		if len(parts) >= 3 {
			if v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(parts[2], "#"))); err == nil {
				highBound = gamedb.DBRef(v)
			}
		}
	}

	// Parse left side: could be "[player] [class]" or just "[class]" or just "[player]"
	if leftSide != "" {
		// Split by spaces
		words := strings.Fields(leftSide)
		if eqIdx >= 0 {
			// There was an '=', so the last word is the class
			if len(words) == 1 {
				searchClass = strings.ToLower(words[0])
			} else {
				// First word(s) = player, last word = class
				searchClass = strings.ToLower(words[len(words)-1])
				playerName := strings.Join(words[:len(words)-1], " ")
				if strings.EqualFold(playerName, "all") {
					if isWiz {
						searchAll = true
					}
				} else {
					ref := resolveDBRef(ctx, playerName)
					if ref != gamedb.Nothing {
						ownerRef = ref
					}
				}
			}
		} else {
			// No '=' — treat as player name or "all"
			playerName := strings.Join(words, " ")
			if strings.EqualFold(playerName, "all") {
				if isWiz {
					searchAll = true
				}
			} else {
				ref := resolveDBRef(ctx, playerName)
				if ref != gamedb.Nothing {
					ownerRef = ref
				}
			}
		}
	}

	// Non-wizard can only search own objects.
	// Wizard with no explicit player specification defaults to searching all
	// objects (matching C TinyMUSH behavior where wizard default is ANY_OWNER).
	if !isWiz {
		ownerRef = ctx.Player
		searchAll = false
	} else if leftSide == "" || (eqIdx >= 0 && len(strings.Fields(leftSide)) == 1) {
		// Wizard didn't specify a player name — default to search all
		// leftSide=="" means no args at all; single word with '=' means just a class, no player
		searchAll = true
	}

	// Determine type filter from class
	restrictionUpper := strings.ToUpper(restriction)
	switch searchClass {
	case "type":
		switch restrictionUpper {
		case "ROOM":
			filterType = gamedb.TypeRoom
		case "EXIT":
			filterType = gamedb.TypeExit
		case "THING", "OBJECT":
			filterType = gamedb.TypeThing
		case "PLAYER":
			filterType = gamedb.TypePlayer
		case "GARBAGE":
			filterType = gamedb.TypeGarbage
		}
	case "rooms":
		filterType = gamedb.TypeRoom
	case "exits":
		filterType = gamedb.TypeExit
	case "objects", "things":
		filterType = gamedb.TypeThing
	case "players":
		filterType = gamedb.TypePlayer
	case "eroom":
		filterType = gamedb.TypeRoom
	case "eexit":
		filterType = gamedb.TypeExit
	case "eobject", "ething":
		filterType = gamedb.TypeThing
	case "eplayer":
		filterType = gamedb.TypePlayer
	}

	// Parse flags= restriction into a list of flag names
	var flagNames []string
	var flagNegate []bool
	if searchClass == "flags" {
		for i := 0; i < len(restriction); i++ {
			negate := false
			if restriction[i] == '!' {
				negate = true
				i++
				if i >= len(restriction) {
					break
				}
			}
			fname := flagCharToName(restriction[i])
			if fname != "" {
				flagNames = append(flagNames, fname)
				flagNegate = append(flagNegate, negate)
			}
		}
	}

	// Parse parent= and zone= restrictions
	var parentRef gamedb.DBRef = gamedb.Nothing
	var zoneRef gamedb.DBRef = gamedb.Nothing
	if searchClass == "parent" && restriction != "" {
		parentRef = resolveDBRef(ctx, restriction)
	}
	if searchClass == "zone" && restriction != "" {
		zoneRef = resolveDBRef(ctx, restriction)
	}

	// Prepare eval expression for eval=/evaluate=/eplayer=/eroom= etc.
	isEvalClass := false
	switch searchClass {
	case "eval", "evaluate", "eplayer", "eroom", "eobject", "ething", "eexit":
		isEvalClass = true
	}

	// Determine if name-based classes need name matching
	isNameClass := false
	switch searchClass {
	case "name", "rooms", "exits", "objects", "things", "players":
		isNameClass = true
	}

	// Perform the search
	var results []string
	for ref := lowBound; ref <= highBound; ref++ {
		obj, ok := ctx.DB.Objects[ref]
		if !ok {
			continue
		}

		// Skip GOING objects
		if obj.Flags[0]&gamedb.FlagGoing != 0 {
			continue
		}

		// Owner filter
		if !searchAll && obj.Owner != ownerRef {
			continue
		}

		// Type filter
		if filterType >= 0 && obj.ObjType() != filterType {
			continue
		}

		// Class-specific filters
		switch searchClass {
		case "type":
			// Already handled by filterType above
		case "name":
			if restriction != "" && !wildMatch(restriction+"*", obj.Name) {
				continue
			}
		case "rooms", "exits", "objects", "things", "players":
			if isNameClass && restriction != "" && !wildMatch(restriction+"*", obj.Name) {
				continue
			}
		case "flags":
			match := true
			for i, fname := range flagNames {
				has := objHasFlag(obj, fname)
				if flagNegate[i] {
					if has {
						match = false
						break
					}
				} else {
					if !has {
						match = false
						break
					}
				}
			}
			if !match {
				continue
			}
		case "parent":
			if parentRef != gamedb.Nothing && obj.Parent != parentRef {
				continue
			}
		case "zone":
			if zoneRef != gamedb.Nothing && obj.Zone != zoneRef {
				continue
			}
		case "eval", "evaluate", "eplayer", "eroom", "eobject", "ething", "eexit":
			if isEvalClass && restriction != "" {
				// Replace ## with current dbref
				expr := strings.ReplaceAll(restriction, "##", fmt.Sprintf("#%d", ref))
				result := ctx.Exec(expr, eval.EvFCheck|eval.EvEval, nil)
				result = strings.TrimSpace(result)
				if result == "" || result == "0" || result == "#-1" {
					continue
				}
			}
		case "":
			// No class — list all matching owner (already filtered)
		default:
			// Unknown class — skip silently
			continue
		}

		results = append(results, fmt.Sprintf("#%d", ref))
	}
	buf.WriteString(strings.Join(results, " "))
}

// fnStats — return database statistics.
func fnStats(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	rooms, things, exits, players := 0, 0, 0, 0
	for _, obj := range ctx.DB.Objects {
		switch obj.ObjType() {
		case gamedb.TypeRoom: rooms++
		case gamedb.TypeThing: things++
		case gamedb.TypeExit: exits++
		case gamedb.TypePlayer: players++
		}
	}
	total := len(ctx.DB.Objects)
	buf.WriteString(fmt.Sprintf("%d objects = %d rooms, %d exits, %d things, %d players",
		total, rooms, exits, things, players))
}

// fnHasmodule — check if a module is loaded. Stub: always returns 0.
func fnHasmodule(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString("0")
}

// fnRestarts — number of server restarts. Stub: always 0.
func fnRestarts(_ *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString("0")
}

// fnHears — check if object can hear another. Simplified.
func fnHears(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	listener := resolveDBRef(ctx, args[0])
	speaker := resolveDBRef(ctx, args[1])
	if listener == gamedb.Nothing || speaker == gamedb.Nothing { buf.WriteString("0"); return }
	lObj, ok1 := ctx.DB.Objects[listener]
	sObj, ok2 := ctx.DB.Objects[speaker]
	if !ok1 || !ok2 { buf.WriteString("0"); return }
	// Same location or listener is in speaker
	buf.WriteString(boolToStr(lObj.Location == sObj.Location || lObj.Location == speaker || listener == sObj.Location))
}

// fnKnows — check if object knows about another. Same as hears for now.
func fnKnows(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnHears(ctx, args, buf, caller, cause)
}

// fnMoves — check if object can move to location. Simplified.
func fnMoves(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	mover := resolveDBRef(ctx, args[0])
	dest := resolveDBRef(ctx, args[1])
	if mover == gamedb.Nothing || dest == gamedb.Nothing { buf.WriteString("0"); return }
	_, ok1 := ctx.DB.Objects[mover]
	_, ok2 := ctx.DB.Objects[dest]
	buf.WriteString(boolToStr(ok1 && ok2))
}

// fnWritable — check if object is writable by player.
func fnWritable(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	player := resolveDBRef(ctx, args[0])
	target := resolveDBRef(ctx, args[1])
	if player == gamedb.Nothing || target == gamedb.Nothing { buf.WriteString("0"); return }
	pObj, ok1 := ctx.DB.Objects[player]
	tObj, ok2 := ctx.DB.Objects[target]
	if !ok1 || !ok2 { buf.WriteString("0"); return }
	buf.WriteString(boolToStr(objHasFlag(pObj, "WIZARD") || tObj.Owner == player || player == target))
}

// fnPfind — find player by partial name match.
func fnPfind(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("#-1"); return }
	name := strings.TrimSpace(args[0])
	if strings.HasPrefix(name, "*") { name = name[1:] }
	for ref, obj := range ctx.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && strings.EqualFold(obj.Name, name) {
			buf.WriteString(fmt.Sprintf("#%d", ref))
			return
		}
	}
	// Partial match
	for ref, obj := range ctx.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && strings.HasPrefix(strings.ToLower(obj.Name), strings.ToLower(name)) {
			buf.WriteString(fmt.Sprintf("#%d", ref))
			return
		}
	}
	buf.WriteString("#-1 NO MATCH")
}

// Validation

func fnValid(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("0")
		return
	}
	category := strings.ToLower(strings.TrimSpace(args[0]))
	name := args[1]
	switch category {
	case "attrname":
		// Valid attribute name: alphanumeric, underscore, dash, period
		valid := len(name) > 0
		for _, ch := range name {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.' || ch == '`') {
				valid = false
				break
			}
		}
		buf.WriteString(boolToStr(valid))
	case "objectname":
		buf.WriteString(boolToStr(len(name) > 0 && name[0] != '#'))
	case "playername":
		valid := len(name) > 0
		for _, ch := range name {
			if ch == ' ' || ch == '"' || ch == ';' {
				valid = false
				break
			}
		}
		buf.WriteString(boolToStr(valid))
	default:
		buf.WriteString("0")
	}
}

// --- Additional functions from audit ---

// fnIbreak — break out of an iter/parse loop.
// ibreak([levels]) — breaks out of <levels> nesting levels (default 1).
func fnIbreak(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	levels := 1
	if len(args) > 0 {
		n := toInt(args[0])
		if n > 0 { levels = n }
	}
	if ctx.Loop.InLoop > 0 {
		ctx.Loop.BreakLevel = levels
	}
}

// fnOemit — emit to all in room except target.
// oemit(target, message) — sends message to all in target's location except target.
func fnOemit(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { return }
	ctx.Notifications = append(ctx.Notifications, eval.Notification{
		Target:  ref,
		Message: args[1],
		Type:    eval.NotifyOEmit,
	})
}

// fnRandextract — extract a random element from a list.
// randextract(list[, delim[, count]])
func fnRandextract(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { return }
	count := 1
	if len(args) > 2 {
		n := toInt(args[2])
		if n > 0 { count = n }
	}
	if count > len(words) { count = len(words) }
	// Fisher-Yates partial shuffle
	for i := 0; i < count; i++ {
		j := i + rand.IntN(len(words)-i)
		words[i], words[j] = words[j], words[i]
	}
	buf.WriteString(strings.Join(words[:count], delim))
}

// fnElementpos — return position of an element in a list (1-indexed).
// elementpos(list, element[, delim])
func fnElementpos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	target := args[1]
	for i, w := range words {
		if strings.EqualFold(w, target) {
			writeInt(buf, i+1)
			return
		}
	}
	buf.WriteString("0")
}

// fnPlaymem — return rough memory usage of a player's objects.
func fnPlaymem(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("0"); return }
	total := 0
	for _, obj := range ctx.DB.Objects {
		if obj.Owner == ref {
			total += 128 + len(obj.Name)
			for _, attr := range obj.Attrs {
				total += 16 + len(attr.Value)
			}
		}
	}
	writeInt(buf, total)
}

// fnObjid — return object ID as dbref:createtime.
func fnObjid(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok { buf.WriteString("#-1"); return }
	text := getAttrByName(ctx, ref, "CREATED_TIME")
	if text == "" {
		fmt.Fprintf(buf, "#%d", obj.DBRef)
	} else {
		fmt.Fprintf(buf, "#%d:%s", obj.DBRef, text)
	}
}

// fnCreatetime — return creation time as secs since epoch.
func fnCreatetime(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("-1"); return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { buf.WriteString("-1"); return }
	text := getAttrByName(ctx, ref, "CREATED_TIME")
	if text == "" { buf.WriteString("-1"); return }
	buf.WriteString(text)
}

// fnRegparse — regex match with capture groups stored to named registers.
// regparse(string, pattern, register-list)
func fnRegparse(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regparseHelper(ctx, args, buf, false)
}

func fnRegparsei(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	regparseHelper(ctx, args, buf, true)
}

func regparseHelper(ctx *eval.EvalContext, args []string, buf *strings.Builder, caseInsensitive bool) {
	if len(args) < 3 { buf.WriteString("0"); return }
	if ctx.RData == nil { buf.WriteString("0"); return }

	pattern := args[1]
	if caseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil { buf.WriteString("0"); return }

	matches := re.FindStringSubmatch(args[0])
	if matches == nil { buf.WriteString("0"); return }

	// Store captures in named registers from the register list
	regs := strings.Fields(args[2])
	for i, reg := range regs {
		if i < len(matches) {
			reg = strings.ToLower(strings.TrimSpace(reg))
			ctx.RData.XRegs[reg] = matches[i]
		}
	}
	buf.WriteString("1")
}

// --- TinyMUSH C gap functions ---

// fnCcount — returns the function invocation counter for the current evaluation.
// ccount() → integer
func fnCcount(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(strconv.Itoa(ctx.FuncInvkCtr))
}

// fnCdepth — returns the current function nesting depth.
// cdepth() → integer
func fnCdepth(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(strconv.Itoa(ctx.FuncNestLev))
}

// fnCommand — returns the raw command text currently being evaluated.
// command() → string
func fnCommand(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(ctx.CurrCmd)
}

// fnLvars — lists all named X-register variables.
// lvars() → space-separated list of variable names
func fnLvars(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.RData == nil { return }
	first := true
	for name := range ctx.RData.XRegs {
		if !first { buf.WriteByte(' ') }
		buf.WriteString(name)
		first = false
	}
}

// fnProgrammer — returns the dbref of the player that @programmed the target, or #-1.
// programmer(player) → dbref
// Since we don't have access to Descriptor data from eval context, we check if the
// player has A_PROGCMD set (which indicates active @program state).
func fnProgrammer(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("#-1"); return }
	target := resolveDBRef(ctx, args[0])
	if target == gamedb.Nothing { buf.WriteString("#-1"); return }
	obj, ok := ctx.DB.Objects[target]
	if !ok || obj.ObjType() != gamedb.TypePlayer {
		buf.WriteString("#-1"); return
	}
	// Check if player has A_PROGCMD attribute (set during @program)
	for _, attr := range obj.Attrs {
		if attr.Number == gamedb.A_PROGCMD {
			buf.WriteString("#-1 IN PROGRAM")
			return
		}
	}
	buf.WriteString("#-1")
}

// fnWildparse — wildcard matching with capture into named variables.
// wildparse(string, pattern, var_names) → void (captures stored in named registers)
func fnWildparse(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 || ctx.RData == nil { return }
	str := args[0]
	pattern := args[1]
	varNames := strings.Fields(args[2])

	captures := wildMatchCapture(pattern, str)
	if captures == nil { return }

	for i, name := range varNames {
		if i < len(captures) {
			name = strings.ToLower(strings.TrimSpace(name))
			ctx.RData.XRegs[name] = captures[i]
		}
	}
}

// wildMatchCapture performs glob-style pattern matching and captures * groups.
// Returns nil if no match, otherwise a slice of captured strings (one per * in pattern).
func wildMatchCapture(pattern, str string) []string {
	pattern = strings.ToLower(pattern)
	str = strings.ToLower(str)
	var captures []string
	ok := captureHelper(pattern, str, &captures)
	if !ok { return nil }
	return captures
}

func captureHelper(pattern, str string, captures *[]string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Try every possible capture length
			rest := pattern[1:]
			for i := len(str); i >= 0; i-- {
				capturesBak := len(*captures)
				*captures = append(*captures, str[:i])
				if captureHelper(rest, str[i:], captures) {
					return true
				}
				// Backtrack
				*captures = (*captures)[:capturesBak]
			}
			return false
		case '?':
			if len(str) == 0 { return false }
			pattern = pattern[1:]
			str = str[1:]
		default:
			if len(str) == 0 || pattern[0] != str[0] { return false }
			pattern = pattern[1:]
			str = str[1:]
		}
	}
	return len(str) == 0
}

// fnElockstr — evaluate a lock expression string.
// elockstr(object, actor, lock_string) → 1 if actor passes, 0 otherwise
func fnElockstr(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }
	if ctx.GameState == nil { buf.WriteString("0"); return }
	thing := resolveDBRef(ctx, args[0])
	actor := resolveDBRef(ctx, args[1])
	if thing == gamedb.Nothing || actor == gamedb.Nothing {
		buf.WriteString("#-1 NOT FOUND")
		return
	}
	lockStr := args[2]
	if ctx.GameState.EvalLockStr(ctx.Player, thing, actor, lockStr) {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// fnSession — return session statistics for a connected player.
// session(player) → "commands bytes_sent bytes_recv"
func fnSession(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("-1 -1 -1"); return }
	if ctx.GameState == nil { buf.WriteString("-1 -1 -1"); return }
	target := resolveDBRef(ctx, args[0])
	if target == gamedb.Nothing { buf.WriteString("-1 -1 -1"); return }
	cmds, sent, recv := ctx.GameState.SessionInfo(target)
	buf.WriteString(strconv.Itoa(cmds))
	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(sent))
	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(recv))
}

// fnHelptext — retrieve help text for a topic from a help file.
// helptext(file_id, topic) → text or empty
func fnHelptext(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	if ctx.GameState == nil { return }
	text := ctx.GameState.HelpLookup(ctx.Player, args[0], args[1])
	buf.WriteString(text)
}

// fnObjcall — call a u-function from another object's perspective.
// objcall(executor, obj/attr[, arg1, arg2, ...]) → result
// Like u() but the function executes with executor as %!.
func fnObjcall(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	executor := resolveDBRef(ctx, args[0])
	if executor == gamedb.Nothing {
		buf.WriteString("#-1 NOT FOUND")
		return
	}
	// Permission check: caller must control the executor
	if ctx.GameState != nil && !ctx.GameState.Controls(ctx.Player, executor) {
		buf.WriteString("#-1 PERMISSION DENIED")
		return
	}

	// Save current player, execute as the specified object
	savedPlayer := ctx.Player
	ctx.Player = executor
	defer func() { ctx.Player = savedPlayer }()

	// Call the u-function with remaining args
	var uargs []string
	if len(args) > 2 { uargs = args[2:] }
	result := ctx.CallUFun(args[1], uargs)
	buf.WriteString(result)
}
