package server

import (
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Attribute flag aliases — canonical values are in gamedb package.
const (
	AFNoProg  = gamedb.AFNoProg
	AFPrivate = gamedb.AFPrivate
	AFNoParse = gamedb.AFNoParse
	AFRegexp  = gamedb.AFRegexp
	AFNow     = gamedb.AFNow
)

// MatchDollarCommands searches objects for $-pattern attributes that match the input.
// Returns true if a match was found and queued/executed.
func (g *Game) MatchDollarCommands(player, cause gamedb.DBRef, input string) bool {
	// Objects to search, matching C TinyMUSH order (command_core.c atr_match):
	// room → room contents → player → player inventory → master room → zones
	var searchObjs []gamedb.DBRef

	// Room and room contents first (C checks location before player/inventory)
	loc := g.PlayerLocation(player)
	if loc != gamedb.Nothing {
		searchObjs = append(searchObjs, loc)
		for _, next := range g.DB.SafeContents(loc) {
			if next != player { // Skip player (searched separately below)
				searchObjs = append(searchObjs, next)
			}
		}
	}

	// Player's own attributes
	searchObjs = append(searchObjs, player)

	// Player's inventory
	searchObjs = append(searchObjs, g.DB.SafeContents(player)...)

	// Master room contents — global commands live here in heavy softcode games
	masterRoom := g.MasterRoomRef()
	if loc != masterRoom {
		// Search master room itself
		searchObjs = append(searchObjs, masterRoom)
		// Search its contents
		searchObjs = append(searchObjs, g.DB.SafeContents(masterRoom)...)
	}

	// Zone-based commands: check player's zone and room's zone
	if pObj, ok := g.DB.Objects[player]; ok && pObj.Zone != gamedb.Nothing {
		searchObjs = g.addZoneObjects(searchObjs, pObj.Zone)
	}
	if loc != gamedb.Nothing {
		if locObj, ok := g.DB.Objects[loc]; ok && locObj.Zone != gamedb.Nothing {
			searchObjs = g.addZoneObjects(searchObjs, locObj.Zone)
		}
	}

	if IsDebug() {
		names := make([]string, len(searchObjs))
		for i, ref := range searchObjs {
			if o, ok := g.DB.Objects[ref]; ok {
				names[i] = fmt.Sprintf("#%d(%s)", ref, o.Name)
			} else {
				names[i] = fmt.Sprintf("#%d(?)", ref)
			}
		}
		DebugLog("DOLLAR search list (%d objs): %v", len(searchObjs), names)
	}

	// Search each object's attributes
	for _, objRef := range searchObjs {
		if g.matchDollarOnObject(objRef, player, cause, input) {
			DebugLog("DOLLAR MATCHED on #%d", objRef)
			return true
		}
	}
	DebugLog("DOLLAR NO MATCH for %q", input)
	return false
}

// addZoneObjects appends a zone object and its contents to the search list.
func (g *Game) addZoneObjects(searchObjs []gamedb.DBRef, zone gamedb.DBRef) []gamedb.DBRef {
	searchObjs = append(searchObjs, zone)
	searchObjs = append(searchObjs, g.DB.SafeContents(zone)...)
	return searchObjs
}

// matchDollarOnObject checks a single object for $-command matches.
func (g *Game) matchDollarOnObject(objRef, player, cause gamedb.DBRef, input string) bool {
	obj, ok := g.DB.Objects[objRef]
	if !ok {
		return false
	}

	// Skip halted objects
	if obj.HasFlag(gamedb.FlagHalt) {
		DebugLog("DOLLAR #%d(%s) HALTED, skipping", objRef, obj.Name)
		return false
	}

	dollarCount := 0
	for _, attr := range obj.Attrs {
		text := eval.StripAttrPrefix(attr.Value)
		if !strings.HasPrefix(text, "$") {
			continue
		}
		dollarCount++

		// Parse attribute flags from the raw value
		// Format: "owner:flags:$pattern:command"
		attrFlags := parseAttrFlags(attr.Value)
		if attrFlags&AFNoProg != 0 {
			continue
		}

		// Split "$pattern:command"
		rest := text[1:] // skip $
		colonIdx := findUnescapedColon(rest)
		if colonIdx < 0 {
			continue
		}
		pattern := rest[:colonIdx]
		command := rest[colonIdx+1:]

		// Match the pattern against input
		matched, args := matchWild(pattern, input)
		if IsDebug() && dollarCount <= 10 {
			DebugLog("DOLLAR #%d(%s) attr %d: pattern=%q input=%q matched=%v", objRef, obj.Name, attr.Number, pattern, input, matched)
		}
		if !matched {
			continue
		}

		// Create queue entry
		entry := &QueueEntry{
			Player:  objRef,
			Cause:   cause,
			Caller:  player,
			Command: command,
			Args:    args,
		}

		if attrFlags&AFNow != 0 {
			// Execute immediately
			g.ExecuteQueueEntry(entry)
		} else {
			g.Queue.Add(entry)
		}
		return true
	}

	// Check parent chain
	parentRef := obj.Parent
	if IsDebug() && parentRef != gamedb.Nothing {
		if pObj, ok := g.DB.Objects[parentRef]; ok {
			DebugLog("DOLLAR #%d(%s) checking parent #%d(%s) attrs=%d", objRef, obj.Name, parentRef, pObj.Name, len(pObj.Attrs))
		} else {
			DebugLog("DOLLAR #%d(%s) parent #%d NOT FOUND in DB", objRef, obj.Name, parentRef)
		}
	}
	visited := make(map[gamedb.DBRef]bool)
	visited[objRef] = true
	for parentRef != gamedb.Nothing && !visited[parentRef] {
		visited[parentRef] = true
		if g.matchDollarOnParent(parentRef, objRef, player, cause, input) {
			return true
		}
		if pObj, ok := g.DB.Objects[parentRef]; ok {
			parentRef = pObj.Parent
		} else {
			break
		}
	}

	return false
}

// matchDollarOnParent checks a parent object's attributes, skipping AF_PRIVATE.
func (g *Game) matchDollarOnParent(parentRef, childRef, player, cause gamedb.DBRef, input string) bool {
	parent, ok := g.DB.Objects[parentRef]
	if !ok {
		return false
	}

	dollarCount := 0
	for _, attr := range parent.Attrs {
		text := eval.StripAttrPrefix(attr.Value)
		if !strings.HasPrefix(text, "$") {
			continue
		}
		dollarCount++
		attrFlags := parseAttrFlags(attr.Value)
		if attrFlags&AFNoProg != 0 || attrFlags&AFPrivate != 0 {
			DebugLog("DOLLAR parent #%d attr %d SKIPPED flags=0x%x (noprog=%v private=%v)", parentRef, attr.Number, attrFlags, attrFlags&AFNoProg != 0, attrFlags&AFPrivate != 0)
			continue
		}

		rest := text[1:]
		colonIdx := findUnescapedColon(rest)
		if colonIdx < 0 {
			continue
		}
		pattern := rest[:colonIdx]
		command := rest[colonIdx+1:]

		matched, args := matchWild(pattern, input)
		if IsDebug() && dollarCount <= 10 {
			DebugLog("DOLLAR parent #%d attr %d: pattern=%q input=%q matched=%v", parentRef, attr.Number, pattern, input, matched)
		}
		if !matched {
			continue
		}

		DebugLog("DOLLAR parent #%d MATCHED for child #%d", parentRef, childRef)
		entry := &QueueEntry{
			Player:  childRef, // Execute as child, not parent
			Cause:   cause,
			Caller:  player,
			Command: command,
			Args:    args,
		}
		g.Queue.Add(entry)
		return true
	}
	return false
}

// ExecuteQueueEntry executes a queued command.
// Like TinyMUSH's process_cmdline, it splits on semicolons to handle
// multi-command strings (e.g. "@drain me;@notify me").
func (g *Game) ExecuteQueueEntry(entry *QueueEntry) {
	// Check HALT flag — halted objects should not execute queue entries
	if obj, ok := g.DB.Objects[entry.Player]; ok {
		if obj.HasFlag(gamedb.FlagHalt) {
			return
		}
	}

	// When fix_escape_eval is enabled, strip double-escaped specials (\\[ → \[, etc.)
	// so that data written for C TinyMUSH's extra eval pass displays correctly.
	// Applied here (not just QueueAttrAction) so ALL queued paths are covered:
	// $-commands, @trigger, @force, STARTUP, ACONNECT, etc.
	if g.Conf != nil && g.Conf.FixEscapeEval {
		entry.Command = stripDoubleEscapeSpecials(entry.Command)
	}

	ctx := MakeEvalContextWithGame(g, entry.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	ctx.Cause = entry.Cause
	ctx.Caller = entry.Caller

	// Restore register data if present
	if entry.RData != nil {
		ctx.RData = entry.RData
	}

	// Split on semicolons FIRST (respecting braces), then evaluate each command.
	// This mirrors TinyMUSH's process_cmdline which uses parse_to(&cmdline, ';', 0)
	// to split BEFORE evaluation, preserving brace-protected content for @wait etc.
	cmds := splitSemicolonRespectingBraces(entry.Command)

	// Snapshot q-registers onto descriptors so @program can capture them.
	descs := g.Conns.GetByPlayer(entry.Player)

	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}

		// C TinyMUSH splits command args BEFORE evaluation (process_cmdent).
		// For commands whose RHS body is stored for deferred execution
		// (@wait, @dolist, @switch, @trigger), we split on '=' first,
		// evaluate only the LHS, and preserve the body raw. The body is
		// evaluated later in the appropriate context (e.g. per-iteration
		// for @dolist, when wait fires for @wait).
		if handled := g.handleDeferredBodyCmd(cmd, ctx, entry, descs); handled {
			continue
		}

		// Evaluate each individual command with args as %0-%9.
		// We do NOT use EvStrip here so that brace grouping is preserved for
		// commands like @switch that need to split on commas respecting braces.
		// In C TinyMUSH, parse_arglist splits args by parse_to (brace-aware)
		// BEFORE evaluation; we approximate this by preserving braces through eval.
		evaluated := ctx.Exec(cmd, eval.EvFCheck|eval.EvEval, entry.Args)
		evaluated = strings.TrimSpace(evaluated)
		DebugLog("EVAL player=#%d cmd=%q evaluated=%q args=%v", entry.Player, truncDebug(cmd, 200), truncDebug(evaluated, 200), entry.Args)
		if evaluated == "" {
			continue
		}

		// If the original command was a brace-wrapped group (from @dolist, @wait,
		// etc.), strip the outer braces and dispatch each semicolon-separated
		// piece individually. Each piece gets its own eval+dispatch cycle,
		// matching C TinyMUSH's process_cmdline which splits on ';' then
		// evaluates each command separately.
		// NOTE: We check the RAW cmd for leading '{', not 'evaluated', because
		// the evaluator strips outer braces during eval.
		if len(cmd) >= 2 && cmd[0] == '{' && cmd[len(cmd)-1] == '}' {
			inner := evaluated[1 : len(evaluated)-1]
			innerCmds := splitSemicolonRespectingBraces(inner)
			for _, ic := range innerCmds {
				ic = strings.TrimSpace(ic)
				if ic == "" {
					continue
				}
				// Re-evaluate each piece with EvFCheck so bare function
				// calls (like add(), parse()) that were protected by the
				// outer braces now get evaluated.
				ic = ctx.Exec(ic, eval.EvFCheck|eval.EvEval, entry.Args)
				ic = strings.TrimSpace(ic)
				if ic == "" {
					continue
				}
				if len(descs) > 0 {
					DispatchCommand(g, descs[0], ic)
				} else {
					g.ExecuteAsObject(entry.Player, entry.Cause, ic)
				}
			}
			continue
		}

		// Find a descriptor for this player to dispatch through
		if len(descs) > 0 {
			DispatchCommand(g, descs[0], evaluated)
		} else {
			// Object executing without a connected player - execute internally
			g.ExecuteAsObject(entry.Player, entry.Cause, evaluated)
		}
	}

	// Snapshot final q-registers onto descriptors so @program can capture them.
	rDataSnapshot := ctx.RData.Clone()
	for _, dd := range descs {
		dd.LastRData = rDataSnapshot
	}

	// Clear q-register snapshot from descriptors
	for _, dd := range descs {
		dd.LastRData = nil
	}

	// Handle any notifications from the eval context
	for _, n := range ctx.Notifications {
		switch n.Type {
		case eval.NotifyRemit:
			g.Conns.SendToRoom(g.DB, n.Target, n.Message)
		case eval.NotifyOEmit:
			// Send to all in target's location except target
			obj, ok := g.DB.Objects[n.Target]
			if ok {
				g.Conns.SendToRoomExcept(g.DB, obj.Location, n.Target, n.Message)
			}
		default:
			g.Conns.SendToPlayer(n.Target, n.Message)
		}
	}
}

// splitDeferredBody detects commands like "@wait <spec>={body}" where the body
// should be stored raw (not evaluated). It matches the given prefix (case-insensitive),
// finds the first '=' at brace-depth 0, and returns:
//   - cmdPrefix: the command name with any /switches (e.g. "@switch/first")
//   - lhs: the argument before '=' (e.g. expression or time spec)
//   - body: everything after '=' (preserved raw)
//
// This implements C TinyMUSH's split-before-eval behavior for deferred commands.
func splitDeferredBody(cmd, prefix string) (lhs, body string, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	pfx := strings.ToLower(prefix)
	if !strings.HasPrefix(lower, pfx) {
		return "", "", false
	}
	// Must be followed by space or /
	rest := cmd[len(pfx):]
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '/') {
		return "", "", false
	}
	// Skip /switches and find the first space (start of args)
	argsStart := 0
	for argsStart < len(rest) && rest[argsStart] != ' ' {
		argsStart++
	}
	// Skip spaces to get to actual args
	for argsStart < len(rest) && rest[argsStart] == ' ' {
		argsStart++
	}
	if argsStart >= len(rest) {
		return "", "", false
	}
	argsStr := rest[argsStart:]

	// Find the '=' at brace depth 0 within the args portion
	depth := 0
	for i := 0; i < len(argsStr); i++ {
		switch argsStr[i] {
		case '\\':
			i++ // skip escaped char
		case '{':
			depth++
		case '}':
			depth--
		case '[':
			depth++
		case ']':
			depth--
		case '=':
			if depth == 0 {
				lhs = strings.TrimSpace(argsStr[:i])
				body = strings.TrimSpace(argsStr[i+1:])
				return lhs, body, true
			}
		}
	}
	return "", "", false
}

// handleDeferredBodyCmd checks if cmd is a deferred-body command (@wait,
// @dolist, @switch, @swi) and handles it with split-before-eval semantics.
// The LHS (before '=') is evaluated; the RHS body is preserved raw.
// Returns true if the command was handled.
func (g *Game) handleDeferredBodyCmd(cmd string, ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor) bool {
	for _, prefix := range []string{"@wait", "@dolist", "@switch", "@swi", "@trigger", "@tr"} {
		if lhs, body, ok := splitDeferredBody(cmd, prefix); ok {
			// Extract /switches from the command prefix
			switches := extractDeferredSwitches(cmd, prefix)
			switch prefix {
			case "@wait":
				g.handleWaitDeferred(ctx, entry, descs, lhs, body)
			case "@dolist":
				g.handleDolistDeferred(ctx, entry, descs, switches, lhs, body)
			case "@switch", "@swi":
				g.handleSwitchDeferred(ctx, entry, descs, switches, lhs, body)
			case "@trigger", "@tr":
				g.handleTriggerDeferred(ctx, entry, descs, switches, lhs, body)
			}
			return true
		}
	}
	return false
}

// extractDeferredSwitches pulls /switch names from a command like "@dolist/now".
func extractDeferredSwitches(cmd, prefix string) []string {
	rest := cmd[len(prefix):]
	if len(rest) == 0 || rest[0] != '/' {
		return nil
	}
	// rest starts with "/something ..." — extract up to space
	spIdx := strings.IndexByte(rest, ' ')
	if spIdx < 0 {
		spIdx = len(rest)
	}
	switchStr := rest[1:spIdx] // e.g. "first" or "now"
	return strings.Split(strings.ToLower(switchStr), "/")
}

// handleWaitDeferred handles @wait with split-before-eval.
// Evaluates LHS (time/semaphore spec), preserves body raw for deferred execution.
func (g *Game) handleWaitDeferred(ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor, lhs, body string) {
	evalLHS := ctx.Exec(lhs, eval.EvFCheck|eval.EvEval, entry.Args)
	evalLHS = strings.TrimSpace(evalLHS)

	body = stripOuterBraces(body)
	if body == "" {
		return
	}

	qe := &QueueEntry{
		Player:  entry.Player,
		Cause:   entry.Cause,
		Caller:  entry.Caller,
		Command: body,
		Args:    entry.Args,
	}
	if ctx.RData != nil {
		qe.RData = ctx.RData.Clone()
	}

	if isNumeric(evalLHS) {
		secs := toIntSimple(evalLHS)
		if secs < 0 {
			secs = 0
		}
		qe.WaitUntil = time.Now().Add(time.Duration(secs) * time.Second)
		g.Queue.AddWait(qe)
	} else if slashIdx := strings.IndexByte(evalLHS, '/'); slashIdx >= 0 {
		objStr := evalLHS[:slashIdx]
		attrStr := evalLHS[slashIdx+1:]
		target := g.ResolveRef(entry.Player, objStr)
		if target == gamedb.Nothing {
			return
		}
		qe.SemObj = target
		qe.SemAttr = g.ResolveAttrNum(attrStr)
		g.Queue.AddSemaphore(qe)
	} else {
		g.Queue.Add(qe)
	}
}

// handleDolistDeferred handles @dolist with split-before-eval.
// Evaluates LHS (list), preserves body raw. Substitutes ## and #@ per element.
func (g *Game) handleDolistDeferred(ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor, switches []string, lhs, body string) {
	delim := ""
	if HasSwitch(switches, "delimit") {
		// First space-delimited token in lhs is the delimiter
		trimmed := strings.TrimSpace(lhs)
		spIdx := strings.IndexByte(trimmed, ' ')
		if spIdx > 0 {
			delim = trimmed[:spIdx]
			lhs = strings.TrimSpace(trimmed[spIdx+1:])
		}
	}

	evalLHS := ctx.Exec(lhs, eval.EvFCheck|eval.EvEval, entry.Args)
	evalLHS = strings.TrimSpace(evalLHS)

	body = stripOuterBraces(body)
	if body == "" || evalLHS == "" {
		return
	}

	var elements []string
	if delim != "" {
		elements = strings.Split(evalLHS, delim)
	} else {
		elements = strings.Fields(evalLHS)
	}

	immediate := HasSwitch(switches, "now")

	for i, elem := range elements {
		cmd := strings.ReplaceAll(body, "##", elem)
		cmd = strings.ReplaceAll(cmd, "#@", fmt.Sprintf("%d", i+1))
		if immediate {
			g.evalAndDispatch(ctx, entry, descs, cmd)
		} else {
			// Process inline so that subsequent `;`-separated commands
			// in the same queue entry run AFTER all dolist iterations.
			// This matches practical C TinyMUSH behavior where @dolist
			// iterations complete before the next `;` command's visible
			// effects (e.g. @pemit footer after @dolist rows).
			qe := &QueueEntry{
				Player:  entry.Player,
				Cause:   entry.Cause,
				Caller:  entry.Caller,
				Command: cmd,
				Args:    entry.Args,
			}
			if ctx.RData != nil {
				qe.RData = ctx.RData.Clone()
			}
			g.ExecuteQueueEntry(qe)
		}
	}
}

// handleSwitchDeferred handles @switch/@swi with split-before-eval.
// Evaluates LHS (expression), splits raw RHS on commas, matches patterns.
func (g *Game) handleSwitchDeferred(ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor, switches []string, lhs, body string) {
	DebugLog("DEFERRED @switch player=#%d lhs=%q", entry.Player, truncDebug(lhs, 200))
	expr := ctx.Exec(lhs, eval.EvFCheck|eval.EvEval, entry.Args)
	expr = strings.TrimSpace(expr)
	DebugLog("DEFERRED @switch player=#%d expr=%q parts_body=%q", entry.Player, truncDebug(expr, 100), truncDebug(body, 200))

	parts := splitCommaRespectingBraces(body)

	firstOnly := HasSwitch(switches, "first")
	matched := false


	for i := 0; i+1 < len(parts); i += 2 {
		// Evaluate the pattern
		pattern := ctx.Exec(strings.TrimSpace(parts[i]), eval.EvFCheck|eval.EvEval, entry.Args)
		if wildMatchSimple(strings.ToLower(pattern), strings.ToLower(expr)) {
			action := strings.TrimSpace(parts[i+1])
			action = stripOuterBraces(action)
			action = strings.ReplaceAll(action, "#$", expr)
			g.dispatchActionBody(ctx, entry, descs, action)
			matched = true
			if firstOnly {
				return
			}
		}
	}

	// Default case: odd trailing element (only if no match yet, or @switch/all behavior)
	if len(parts)%2 == 1 && !matched {
		action := strings.TrimSpace(parts[len(parts)-1])
		action = stripOuterBraces(action)
		action = strings.ReplaceAll(action, "#$", expr)
		g.dispatchActionBody(ctx, entry, descs, action)
	}
}

// handleTriggerDeferred handles @trigger with split-before-eval semantics.
// C TinyMUSH's @trigger has CS_ARGV: the RHS (trigger args) is split on
// commas, and each piece is evaluated separately with fresh EvFCheck.
// This prevents EvFCheck clearing in one arg from affecting later args
// (e.g. "num(me), name(me)" — both get evaluated).
func (g *Game) handleTriggerDeferred(ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor, switches []string, lhs, body string) {
	// Evaluate LHS (obj/attr) with fresh EvFCheck
	evalLHS := ctx.Exec(lhs, eval.EvFCheck|eval.EvEval, entry.Args)
	evalLHS = strings.TrimSpace(evalLHS)

	// Parse obj/attr
	parts := strings.SplitN(evalLHS, "/", 2)
	if len(parts) != 2 {
		return
	}

	target := g.ResolveRef(entry.Player, parts[0])
	if target == gamedb.Nothing {
		return
	}

	attrName := strings.ToUpper(strings.TrimSpace(parts[1]))
	attrNum := g.ResolveAttrNum(attrName)
	if attrNum < 0 {
		return
	}
	text := g.GetAttrText(target, attrNum)
	if text == "" {
		return
	}

	// RHS: split on commas (CS_ARGV), evaluate each piece with fresh EvFCheck.
	// Each arg gets its own evaluation pass so bare function calls work.
	var trigArgs []string
	if body != "" {
		rawArgs := splitCommaRespectingBraces(body)
		for _, arg := range rawArgs {
			evaluated := ctx.Exec(strings.TrimSpace(arg), eval.EvFCheck|eval.EvEval, entry.Args)
			trigArgs = append(trigArgs, evaluated)
		}
	}

	qe := &QueueEntry{
		Player:  target,
		Cause:   entry.Cause,
		Caller:  entry.Player,
		Command: text,
		Args:    trigArgs,
	}
	if HasSwitch(switches, "now") {
		g.ExecuteQueueEntry(qe)
	} else {
		g.Queue.Add(qe)
	}
}

// dispatchActionBody splits an action body on semicolons and evaluates+dispatches each.
func (g *Game) dispatchActionBody(ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor, action string) {
	cmds := splitSemicolonRespectingBraces(action)
	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		cmd = stripOuterBraces(cmd)
		g.evalAndDispatch(ctx, entry, descs, cmd)
	}
}

// evalAndDispatch evaluates a raw command body (handling %#, %0-%9, [brackets],
// functions) then dispatches the result. This matches C's process_cmdline
// which evaluates before dispatching.
func (g *Game) evalAndDispatch(ctx *eval.EvalContext, entry *QueueEntry, descs []*Descriptor, rawCmd string) {
	// Check for deferred-body commands (@switch, @dolist, @wait, @trigger)
	// BEFORE evaluating the full command. In C TinyMUSH, these commands are
	// handled by splitting LHS/body BEFORE evaluation. The body is preserved
	// raw and each piece is evaluated separately during dispatch.
	// This is critical because action bodies like:
	//   {&current_scan_by me=%0;@pemit %#=...[filter(...)...]...}
	// contain [bracket] expressions that Go's eval would evaluate prematurely
	// if the entire command were eval'd first. C treats [] inside {} as literal,
	// but Go evaluates them — so we must catch @switch before the eval pass.
	if g.handleDeferredBodyCmd(rawCmd, ctx, entry, descs) {
		return
	}

	evaluated := ctx.Exec(rawCmd, eval.EvFCheck|eval.EvEval, entry.Args)
	evaluated = strings.TrimSpace(evaluated)
	DebugLog("DISPATCH player=#%d raw=%q eval=%q", entry.Player, truncDebug(rawCmd, 200), truncDebug(evaluated, 200))
	if evaluated == "" {
		return
	}

	// Always dispatch through ExecuteAsObject with the queue entry's Player as
	// executor. This ensures "me" resolves to the executing object (e.g. the sled),
	// not the connected player. In C TinyMUSH, commands dispatched from within
	// softcode always execute with the queue entry's player context.
	g.ExecuteAsObject(entry.Player, entry.Cause, evaluated)
}

// splitSemicolonRespectingBraces splits a string on semicolons, respecting
// brace and bracket nesting. This mirrors TinyMUSH's parse_to(&cmdline, ';', 0).
func splitSemicolonRespectingBraces(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\x1b':
			// Skip ANSI escape sequences (ESC[...m) to avoid unmatched '['
			if i+1 < len(s) && s[i+1] == '[' {
				i += 2
				for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
					i++
				}
			}
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		case '\\':
			i++ // skip escaped char
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// ExecuteAsObject executes a command as a non-connected object.
func (g *Game) ExecuteAsObject(player, cause gamedb.DBRef, input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Recursion depth protection — prevent infinite $-command loops
	const maxObjExecDepth = 50
	g.objExecDepth++
	defer func() { g.objExecDepth-- }()
	if g.objExecDepth > maxObjExecDepth {
		DebugLog("OBJEXEC depth limit (%d) exceeded for player=#%d input=%q — breaking cycle", maxObjExecDepth, player, truncDebug(input, 200))
		return
	}

	DebugLog("OBJEXEC ExecuteAsObject player=#%d cause=#%d input=%q", player, cause, truncDebug(input, 200))

	// Handle say/pose/setvattr prefixes
	switch input[0] {
	case '"':
		g.ObjSay(player, input[1:])
		return
	case ':':
		g.ObjPose(player, input[1:])
		return
	case '&':
		// &ATTR obj=value — set variable attribute
		g.objSetVAttr(player, input[1:])
		return
	}

	// Split command and args
	var cmdName, args string
	if spaceIdx := strings.IndexByte(input, ' '); spaceIdx >= 0 {
		cmdName = input[:spaceIdx]
		args = strings.TrimSpace(input[spaceIdx+1:])
	} else {
		cmdName = input
	}

	cmdLower := strings.ToLower(cmdName)

	// Handle /switches on @commands
	var switches string
	if slashIdx := strings.IndexByte(cmdLower, '/'); slashIdx >= 0 {
		switches = cmdLower[slashIdx+1:]
		cmdLower = cmdLower[:slashIdx]
	}

	// Handle key commands that objects can execute
	switch cmdLower {
	case "think":
		// Args arrive already evaluated from queue — send directly to owner
		if obj, ok := g.DB.Objects[player]; ok {
			g.Conns.SendToPlayer(obj.Owner, stripAllBraces(args))
		}
	case "@pemit":
		if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
			targetStr := strings.TrimSpace(args[:eqIdx])
			message := strings.TrimSpace(stripAllBraces(args[eqIdx+1:]))
			DebugLog("OBJEXEC @pemit target=%q message=%q switches=%q", targetStr, truncDebug(message, 120), switches)
			target := g.ResolveRef(player, targetStr)
			if target == gamedb.Nothing {
				target = g.MatchObject(player, targetStr)
			}
			if target == gamedb.Nothing {
				break
			}
			if strings.HasPrefix(switches, "content") {
				// @pemit/contents: send to all contents of target
				for _, cur := range g.DB.SafeContents(target) {
					g.SendMarkedToPlayer(cur, "EMIT", message)
					g.CheckPemitListen(cur, player, message)
				}
				// C TinyMUSH also delivers to the room itself (notify_all_from_inside
				// uses MSG_ME_ALL), triggering LISTEN/^-patterns on the room.
				g.CheckPemitListen(target, player, message)
				// C's notify_all_from_inside also has MSG_F_UP which triggers
				// AUDIBLE outward relay when the target is an AUDIBLE container.
				g.AudibleRelay(target, player, message)
			} else if strings.HasPrefix(switches, "list") {
				// @pemit/list: send to multiple targets
				for _, t := range strings.Fields(targetStr) {
					ref := g.ResolveRef(player, t)
					if ref != gamedb.Nothing {
						g.SendMarkedToPlayer(ref, "EMIT", message)
						g.CheckPemitListen(ref, player, message)
					}
				}
			} else {
				g.SendMarkedToPlayer(target, "EMIT", message)
				// C TinyMUSH: @pemit to an object triggers its LISTEN/^ patterns
				g.CheckPemitListen(target, player, message)
			}
		}
	case "@emit":
		loc := g.PlayerLocation(player)
		if loc != gamedb.Nothing {
			g.SendMarkedToRoom(loc, "EMIT", stripAllBraces(args))
		}
	case "@oemit":
		if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
			targetStr := strings.TrimSpace(args[:eqIdx])
			message := strings.TrimSpace(stripAllBraces(args[eqIdx+1:]))
			target := g.ResolveRef(player, targetStr)
			if target != gamedb.Nothing {
				if tObj, ok := g.DB.Objects[target]; ok {
					g.SendMarkedToRoomExcept(tObj.Location, target, "EMIT", message)
				}
			}
		}
	case "@remit":
		if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
			roomStr := strings.TrimSpace(args[:eqIdx])
			message := strings.TrimSpace(stripAllBraces(args[eqIdx+1:]))
			room := g.ResolveRef(player, roomStr)
			if room != gamedb.Nothing {
				g.SendMarkedToRoom(room, "EMIT", message)
			}
		}
	case "@trigger":
		g.DoTrigger(player, cause, args)
	case "@set":
		g.DoSet(player, args)
	case "@wait":
		g.DoWait(player, cause, args)
	case "@switch":
		g.doSwitchObj(player, cause, args)
	default:
		// Fall through to the full command dispatch system using a synthetic descriptor.
		// This allows STARTUP and other non-connected object commands (@function, @drain,
		// @notify, @dolist, etc.) to work without being individually hardcoded here.
		synth := g.MakeObjDescriptor(player)
		DispatchCommand(g, synth, input)
	}
}

// doSwitchObj implements @switch for non-connected objects (no Descriptor).
func (g *Game) doSwitchObj(player, cause gamedb.DBRef, args string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		return
	}

	ctx := MakeEvalContextWithGame(g, player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	ctx.Cause = cause
	ctx.Caller = cause

	exprStr := strings.TrimSpace(args[:eqIdx])
	expr := ctx.Exec(exprStr, eval.EvFCheck|eval.EvEval, nil)

	rest := strings.TrimSpace(args[eqIdx+1:])
	parts := splitCommaRespectingBraces(rest)

	for i := 0; i+1 < len(parts); i += 2 {
		pattern := ctx.Exec(strings.TrimSpace(parts[i]), eval.EvFCheck|eval.EvEval, nil)
		if wildMatchSimple(strings.ToLower(pattern), strings.ToLower(expr)) {
			// In C TinyMUSH, do_switch dispatches the matched action body
			// to process_cmdline() for execution — it does NOT evaluate the
			// action as an expression. The action body was already partially
			// evaluated by parse_arglist (% subs and [] content), with braces
			// preserved. We strip outer braces and dispatch as command(s).
			raw := stripOuterBraces(strings.TrimSpace(parts[i+1]))
			raw = strings.ReplaceAll(raw, "#$", expr)
			g.dispatchSwitchAction(player, cause, raw)
			return
		}
	}
	if len(parts)%2 == 1 {
		raw := stripOuterBraces(strings.TrimSpace(parts[len(parts)-1]))
		raw = strings.ReplaceAll(raw, "#$", expr)
		g.dispatchSwitchAction(player, cause, raw)
	}
}

// dispatchSwitchAction executes a @switch action body as one or more commands.
// The action may contain semicolons for multiple commands and/or nested braces.
func (g *Game) dispatchSwitchAction(player, cause gamedb.DBRef, action string) {
	cmds := splitSemicolonRespectingBraces(action)
	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		// Strip outer braces from sub-commands (brace groups from @dolist etc.)
		cmd = stripOuterBraces(cmd)
		g.ExecuteAsObject(player, cause, cmd)
	}
}

// truncDebug truncates a string for debug logging.
func truncDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ansiVisualLen returns the display width of a string, skipping ANSI escape
// sequences (ESC[...letter). Used for column alignment in tabular output.
func ansiVisualLen(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			continue
		}
		n++
	}
	return n
}

// ansiFmtLeft left-pads a string to width, accounting for ANSI escape codes.
// Equivalent to fmt.Sprintf("%-*s", width, s) but ANSI-aware.
func ansiFmtLeft(s string, width int) string {
	pad := width - ansiVisualLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// stripOuterBraces removes one level of outer brace grouping if present.
// This matches C TinyMUSH where braces protect action bodies during
// comma-splitting in @switch, but the content is then evaluated with
// full function checking at dispatch time.
func stripOuterBraces(s string) string {
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		return s[1 : len(s)-1]
	}
	return s
}

// stripAllBraces removes all unescaped { and } characters from a string.
// This matches C TinyMUSH's EV_STRIP_CURLY behavior where braces are
// stripped during argument evaluation in process_cmdent/parse_arglist.
func stripAllBraces(s string) string {
	if !strings.ContainsAny(s, "{}") {
		return s
	}
	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			buf.WriteByte(s[i])
			i++
			buf.WriteByte(s[i])
			continue
		}
		if s[i] == '{' || s[i] == '}' {
			continue
		}
		buf.WriteByte(s[i])
	}
	return buf.String()
}

// ObjSay handles say for non-connected objects.
func (g *Game) ObjSay(player gamedb.DBRef, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	name := g.PlayerName(player)
	loc := g.PlayerLocation(player)
	fullMsg := name + " says \"" + msg + "\""
	g.SendMarkedToRoom(loc, "SAY", fullMsg)
	g.MatchListenPatterns(loc, player, fullMsg)
	g.AudibleRelay(loc, player, fullMsg)
}

// ObjPose handles pose for non-connected objects.
func (g *Game) ObjPose(player gamedb.DBRef, msg string) {
	msg = strings.TrimSpace(msg)
	name := g.PlayerName(player)
	loc := g.PlayerLocation(player)
	fullMsg := name + " " + msg
	g.SendMarkedToRoom(loc, "POSE", fullMsg)
	g.MatchListenPatterns(loc, player, fullMsg)
	g.AudibleRelay(loc, player, fullMsg)
}

// objSetVAttr handles &ATTR obj=value for non-connected objects (queue context).
// This resolves "me" relative to the executor (player), not the connected player.
func (g *Game) objSetVAttr(player gamedb.DBRef, rest string) {
	rest = strings.TrimSpace(rest)
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		return
	}
	attrName := strings.ToUpper(strings.TrimSpace(rest[:spaceIdx]))
	objVal := strings.TrimSpace(rest[spaceIdx+1:])

	eqIdx := strings.IndexByte(objVal, '=')
	if eqIdx < 0 {
		return
	}
	targetStr := strings.TrimSpace(objVal[:eqIdx])
	value := strings.TrimSpace(objVal[eqIdx+1:])

	if attrName == "" {
		return
	}

	target := g.ResolveRef(player, targetStr)
	if target == gamedb.Nothing {
		target = g.MatchObject(player, targetStr)
	}
	if target == gamedb.Nothing {
		return
	}
	if !Controls(g, player, target) {
		return
	}

	g.SetAttrByNameChecked(player, target, attrName, value)
}

// DoTrigger triggers an attribute on an object.
// Format: @trigger obj/attr [= arg0, arg1, ...]
func (g *Game) DoTrigger(player, cause gamedb.DBRef, args string) {
	var objAttr, argStr string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		objAttr = strings.TrimSpace(args[:eqIdx])
		argStr = strings.TrimSpace(args[eqIdx+1:])
	} else {
		objAttr = strings.TrimSpace(args)
	}

	parts := strings.SplitN(objAttr, "/", 2)
	if len(parts) != 2 {
		return
	}

	target := g.ResolveRef(player, parts[0])
	if target == gamedb.Nothing {
		DebugLog("TRIGGER player=#%d target=%q RESOLVE FAILED", player, parts[0])
		return
	}

	attrName := strings.ToUpper(strings.TrimSpace(parts[1]))
	attrNum := g.ResolveAttrNum(attrName)
	if attrNum < 0 {
		DebugLog("TRIGGER player=#%d target=#%d attr=%q ATTR NOT FOUND", player, target, attrName)
		return
	}
	// GetAttrText walks the parent chain (like C's atr_pget)
	text := g.GetAttrText(target, attrNum)
	if text == "" {
		DebugLog("TRIGGER player=#%d target=#%d attr=%q (#%d) TEXT EMPTY", player, target, attrName, attrNum)
		return
	}
	DebugLog("TRIGGER player=#%d target=#%d attr=%q (#%d) text=%q", player, target, attrName, attrNum, truncDebug(text, 200))

	// Parse comma-separated args and evaluate each one (CS_ARGV behavior).
	// C TinyMUSH's @trigger evaluates each arg via parse_arglist before
	// storing in the queue entry, so functions like con(me) are resolved.
	var trigArgs []string
	if argStr != "" {
		ctx := MakeEvalContextWithGame(g, player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		ctx.Cause = cause
		rawArgs := splitCommaRespectingBraces(argStr)
		for _, arg := range rawArgs {
			evaluated := ctx.Exec(strings.TrimSpace(arg), eval.EvFCheck|eval.EvEval, nil)
			trigArgs = append(trigArgs, evaluated)
		}
	}

	entry := &QueueEntry{
		Player:  target,
		Cause:   cause,
		Caller:  player,
		Command: text,
		Args:    trigArgs,
	}
	g.Queue.Add(entry)
}

// DoTriggerNow triggers an attribute and executes it immediately (not queued).
// Format: @trigger/now obj/attr [= arg0, arg1, ...]
func (g *Game) DoTriggerNow(player, cause gamedb.DBRef, args string) {
	var objAttr, argStr string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		objAttr = strings.TrimSpace(args[:eqIdx])
		argStr = strings.TrimSpace(args[eqIdx+1:])
	} else {
		objAttr = strings.TrimSpace(args)
	}

	parts := strings.SplitN(objAttr, "/", 2)
	if len(parts) != 2 {
		return
	}

	target := g.ResolveRef(player, parts[0])
	if target == gamedb.Nothing {
		return
	}

	attrName := strings.ToUpper(strings.TrimSpace(parts[1]))
	attrNum := g.ResolveAttrNum(attrName)
	if attrNum < 0 {
		return
	}
	text := g.GetAttrText(target, attrNum)
	if text == "" {
		return
	}

	// Parse comma-separated args and evaluate each one (CS_ARGV behavior)
	var trigArgs []string
	if argStr != "" {
		ctx := MakeEvalContextWithGame(g, player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		ctx.Cause = cause
		rawArgs := splitCommaRespectingBraces(argStr)
		for _, arg := range rawArgs {
			evaluated := ctx.Exec(strings.TrimSpace(arg), eval.EvFCheck|eval.EvEval, nil)
			trigArgs = append(trigArgs, evaluated)
		}
	}

	entry := &QueueEntry{
		Player:  target,
		Cause:   cause,
		Caller:  player,
		Command: text,
		Args:    trigArgs,
	}
	g.ExecuteQueueEntry(entry)
}

// DoWait queues a delayed command.
// Format: @wait seconds = command  OR  @wait obj/attr = command
func (g *Game) DoWait(player, cause gamedb.DBRef, args string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		return
	}
	waitSpec := strings.TrimSpace(args[:eqIdx])
	command := strings.TrimSpace(args[eqIdx+1:])
	if command == "" {
		return
	}

	entry := &QueueEntry{
		Player:  player,
		Cause:   cause,
		Caller:  player,
		Command: command,
	}

	// Check if it's a number (timed wait) or obj/attr (semaphore)
	if isNumeric(waitSpec) {
		secs := toIntSimple(waitSpec)
		if secs < 0 {
			secs = 0
		}
		entry.WaitUntil = time.Now().Add(time.Duration(secs) * time.Second)
		g.Queue.AddWait(entry)
	} else if slashIdx := strings.IndexByte(waitSpec, '/'); slashIdx >= 0 {
		// Semaphore wait: obj/attr
		objStr := waitSpec[:slashIdx]
		attrStr := waitSpec[slashIdx+1:]
		target := g.ResolveRef(player, objStr)
		if target == gamedb.Nothing {
			return
		}
		entry.SemObj = target
		entry.SemAttr = g.ResolveAttrNum(attrStr)
		g.Queue.AddSemaphore(entry)
	} else {
		// Treat as timed wait with default 0
		g.Queue.Add(entry)
	}
}

// DoForce forces an object to execute a command.
func (g *Game) DoForce(forcer, victim gamedb.DBRef, command string) {
	entry := &QueueEntry{
		Player:  victim,
		Cause:   forcer,
		Caller:  forcer,
		Command: command,
	}
	g.Queue.Add(entry)
}

// DoSet handles @set obj = attr:value or @set obj = [!]flag
func (g *Game) DoSet(player gamedb.DBRef, args string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	value := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(player, targetStr)
	if target == gamedb.Nothing {
		return
	}

	// Check for attr:value format
	if colonIdx := strings.IndexByte(value, ':'); colonIdx >= 0 {
		attrName := strings.ToUpper(strings.TrimSpace(value[:colonIdx]))
		attrValue := strings.TrimSpace(value[colonIdx+1:])
		g.SetAttrByName(target, attrName, attrValue)
		return
	}

	// Flag setting
	g.SetFlag(target, value)
}

// ProcessQueue processes queued commands (called periodically).
func (g *Game) ProcessQueue() bool {
	// Move ready entries from wait queue
	promoted := g.Queue.PromoteReady()

	// Reset per-object execution counters every second
	now := time.Now()
	if g.objExecCountReset.IsZero() || now.Sub(g.objExecCountReset) > time.Second {
		g.objExecCount = make(map[gamedb.DBRef]int)
		g.objExecCountReset = now
	}

	// Process up to N entries per tick (10ms tick × 100/tick = 10,000 entries/sec max)
	maxPerTick := 100
	const maxPerObjPerSec = 200 // Per-object rate limit
	processed := 0
	for i := 0; i < maxPerTick; i++ {
		entry := g.Queue.PopImmediate()
		if entry == nil {
			break
		}
		g.objExecCount[entry.Player]++
		if g.objExecCount[entry.Player] > maxPerObjPerSec {
			if g.objExecCount[entry.Player] == maxPerObjPerSec+1 {
				log.Printf("QUEUE: throttling #%d — exceeded %d executions/sec", entry.Player, maxPerObjPerSec)
			}
			continue // Drop entry
		}
		g.safeExecuteQueueEntry(entry)
		processed++
	}
	return processed > 0 || promoted > 0
}

// safeExecuteQueueEntry wraps ExecuteQueueEntry with panic recovery and a
// watchdog that logs slow entries (but still blocks until completion).
func (g *Game) safeExecuteQueueEntry(entry *QueueEntry) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in queue entry (player=#%d cmd=%q): %v\n%s",
				entry.Player, entry.Command, r, debug.Stack())
		}
	}()

	// Watchdog: log if entry takes longer than 5 seconds
	timer := time.AfterFunc(5*time.Second, func() {
		cmdSnippet := entry.Command
		if len(cmdSnippet) > 80 {
			cmdSnippet = cmdSnippet[:80]
		}
		log.Printf("SLOW queue entry >5s (player=#%d cmd=%q)", entry.Player, cmdSnippet)
	})
	g.ExecuteQueueEntry(entry)
	timer.Stop()
}

// StartQueueProcessor starts the background queue processing loop.
// Uses adaptive tick rate: fast (10ms) when processing, slow (100ms) when idle.
func (g *Game) StartQueueProcessor() {
	go func() {
		const fastTick = 10 * time.Millisecond
		const idleTick = 100 * time.Millisecond
		ticker := time.NewTicker(idleTick)
		defer ticker.Stop()
		heartbeat := time.NewTicker(60 * time.Second)
		defer heartbeat.Stop()
		idle := true
		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("PANIC in queue processor: %v", r)
						}
					}()
					hadWork := g.ProcessQueue()
					if hadWork && idle {
						idle = false
						ticker.Reset(fastTick)
					} else if !hadWork && !idle {
						idle = true
						ticker.Reset(idleTick)
					}
				}()
			case <-heartbeat.C:
				imm, wait, sem := g.Queue.Stats()
				if imm > 0 || wait > 0 || sem > 0 {
					log.Printf("Queue heartbeat: %d immediate, %d waiting, %d semaphore", imm, wait, sem)
				}
			}
		}
	}()
}

// QueueAttrAction queues the action in an attribute on an object, if it exists.
// Used for ACONNECT, ADISCONNECT, STARTUP, etc.
func (g *Game) QueueAttrAction(obj, cause gamedb.DBRef, attrNum int, args []string) {
	text := g.GetAttrText(obj, attrNum)
	if text == "" {
		return
	}
	entry := &QueueEntry{
		Player:  obj,
		Cause:   cause,
		Caller:  cause,
		Command: text,
		Args:    args,
	}
	g.Queue.Add(entry)
}

// stripDoubleEscapeSpecials reduces \\X to \X for special characters X in
// [ ] { } %. This fixes data written for C TinyMUSH where an extra level of
// backslash escaping was used to produce literal brackets, braces, and percent
// signs through C's additional eval/strip pass in the queue path.
func stripDoubleEscapeSpecials(text string) string {
	// Quick scan: bail early if no \\ present
	if !strings.Contains(text, `\\`) {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	i := 0
	for i < len(text) {
		if i+2 < len(text) && text[i] == '\\' && text[i+1] == '\\' {
			switch text[i+2] {
			case '[', ']', '{', '}', '%':
				b.WriteByte('\\')      // keep one backslash
				b.WriteByte(text[i+2]) // then the special char
				i += 3
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// FireConnectAttr fires ACONNECT (or ADISCONNECT) on a player, matching C's
// announce_connattr: fires on the player, the master room, and all objects
// in the master room's contents chain.
func (g *Game) FireConnectAttr(player gamedb.DBRef, connCount int, attrNum int) {
	args := []string{"connect", fmt.Sprintf("%d", connCount)}

	// 1. Fire on the player itself
	g.QueueAttrAction(player, player, attrNum, args)

	// 2. Fire on the master room
	masterRoom := g.MasterRoomRef()
	if masterRoom == gamedb.Nothing {
		return
	}
	g.QueueAttrAction(masterRoom, player, attrNum, args)

	// 3. Fire on every object in the master room's contents
	mrObj, ok := g.DB.Objects[masterRoom]
	if !ok {
		return
	}
	seen := make(map[gamedb.DBRef]bool)
	for obj := mrObj.Contents; obj != gamedb.Nothing && !seen[obj]; {
		seen[obj] = true
		o, exists := g.DB.Objects[obj]
		if !exists {
			break
		}
		g.QueueAttrAction(obj, player, attrNum, args)
		obj = o.Next
	}
}

// RunStartup walks all objects and queues STARTUP (attr #19).
// Checks both the HAS_STARTUP flag and the actual attribute, since imported
// databases may not have the flag set consistently.
func (g *Game) RunStartup() {
	count := 0
	for ref, obj := range g.DB.Objects {
		if obj.IsGoing() {
			continue
		}
		// Check flag first (fast path), then fall back to attribute scan
		text := ""
		if obj.HasFlag(gamedb.FlagHasStartup) {
			text = g.GetAttrText(ref, 19) // A_STARTUP = 19
		}
		if text == "" {
			// Flag may not be set on imported objects — check attr directly
			text = g.GetAttrText(ref, 19)
		}
		if text != "" {
			entry := &QueueEntry{
				Player:  ref,
				Cause:   ref,
				Caller:  ref,
				Command: text,
			}
			g.Queue.Add(entry)
			count++
			// Ensure flag is set for future checks
			if !obj.HasFlag(gamedb.FlagHasStartup) {
				obj.Flags[0] |= gamedb.FlagHasStartup
			}
		}
	}
	if count > 0 {
		log.Printf("Queued %d @startup actions", count)
	}
}

// MatchListenPatterns checks for ^pattern:action on MONITOR objects in a room.
// Called when messages are sent to a room (say, pose, emit).
// Optional exclude refs are skipped (used by AudibleRelay to avoid double-firing
// ^-patterns on the originating container).
func (g *Game) MatchListenPatterns(loc gamedb.DBRef, speaker gamedb.DBRef, message string, exclude ...gamedb.DBRef) {
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return
	}

	// Build exclude set
	excludeSet := make(map[gamedb.DBRef]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}

	// Walk contents of the room
	for _, next := range g.DB.SafeContents(loc) {
		if next == speaker || excludeSet[next] {
			continue
		}
		obj, ok := g.DB.Objects[next]
		if !ok {
			continue
		}
		// Check for MONITOR flag (or HAS_LISTEN)
		if obj.HasFlag(gamedb.FlagMonitor) || obj.HasFlag2(gamedb.Flag2HasListen) {
			g.checkListenAttrs(next, speaker, message)
		}
	}

	// Also check the room itself
	if loc != speaker && !excludeSet[loc] && (locObj.HasFlag(gamedb.FlagMonitor) || locObj.HasFlag2(gamedb.Flag2HasListen)) {
		g.checkListenAttrs(loc, speaker, message)
	}
}

// CheckPemitListen checks LISTEN triggers on a single @pemit target.
// In C TinyMUSH, @pemit to an object triggers its LISTEN/^ patterns just like
// say/pose in the room does. This is how listen-based relay objects work.
func (g *Game) CheckPemitListen(target, cause gamedb.DBRef, message string) {
	obj, ok := g.DB.Objects[target]
	if !ok {
		return
	}
	// Fire ^-pattern listen triggers (includes game's WATCH system)
	if obj.HasFlag(gamedb.FlagMonitor) || obj.HasFlag2(gamedb.Flag2HasListen) {
		g.checkListenAttrs(target, cause, message)
	}
}

// AudibleRelay implements the AUDIBLE (HEARTHRU) relay system from C TinyMUSH.
// When speech occurs in a room, this handles two relay directions:
//
// 1. OUTWARD (inside→outside): If the room IS an object with AUDIBLE flag,
//    relay speech to the object's location with @prefix prepended.
//    The relayed message also triggers INWARD relay on objects in the outer room
//    (including the originating container itself), so occupants see what was relayed.
//
// 2. INWARD (outside→inside): For each AUDIBLE object in the room with LISTEN
//    matching the message, relay to the object's contents with @inprefix.
//
// The speaker param is who spoke, loc is where they spoke.
func (g *Game) AudibleRelay(loc, speaker gamedb.DBRef, message string) {
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return
	}

	// --- OUTWARD relay: inside container → container's location ---
	// If we're inside an AUDIBLE object (e.g., a sled), relay to its location.
	if locObj.HasFlag(gamedb.FlagHearThru) {
		outerLoc := locObj.Location
		if outerLoc != gamedb.Nothing {
			// Get PREFIX attribute (attr 90) and prepend to message
			prefix := g.GetAttrText(loc, 90) // A_PREFIX
			if prefix != "" {
				prefix = evalExpr(g, loc, prefix)
			}
			var relayed string
			if prefix != "" {
				relayed = prefix + " " + message
			} else {
				relayed = message
			}
			// Send to all in the outer room (except the originating container)
			for _, next := range g.DB.SafeContents(outerLoc) {
				if next != loc {
					g.SendMarkedToPlayer(next, "EMIT", relayed)
				}
			}
			// Fire listen ^-patterns in the outer room, but exclude
			// the originating container. In C TinyMUSH, the container is
			// excluded from the neighbor notification (obj != target check
			// in _notify_check_va). The container's ^-patterns already
			// fired on the raw message via MatchListenPatterns(loc, ...).
			g.MatchListenPatterns(outerLoc, speaker, relayed, loc)
		}
	}

	// --- INWARD relay: outside → inside AUDIBLE objects ---
	g.audibleInwardRelay(loc, speaker, message)
}

// audibleInwardRelay checks each AUDIBLE object in a room for LISTEN match
// and relays the message to the object's contents with @inprefix prepended.
func (g *Game) audibleInwardRelay(room, speaker gamedb.DBRef, message string) {
	for _, next := range g.DB.SafeContents(room) {
		if next == speaker {
			continue
		}
		obj, ok := g.DB.Objects[next]
		if !ok || !obj.HasFlag(gamedb.FlagHearThru) {
			continue
		}
		// Check if LISTEN pattern matches
		listenText := g.GetAttrText(next, 26) // A_LISTEN
		if listenText == "" {
			continue
		}
		listenText = evalExpr(g, next, listenText)
		matched, _ := matchWild(listenText, message)
		if !matched {
			continue
		}

		// Get INPREFIX attribute (attr 89)
		inprefix := g.GetAttrText(next, 89) // A_INPREFIX
		if inprefix != "" {
			inprefix = evalExpr(g, next, inprefix)
		}
		var relayed string
		if inprefix != "" {
			relayed = inprefix + " " + message
		} else {
			relayed = message
		}
		// Send to all contents of this AUDIBLE object
		for _, inner := range g.DB.SafeContents(next) {
			g.SendMarkedToPlayer(inner, "EMIT", relayed)
		}
	}
}

// checkListenAttrs scans an object's attributes for ^pattern:action matches.
func (g *Game) checkListenAttrs(obj, cause gamedb.DBRef, message string) {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return
	}
	if o.HasFlag(gamedb.FlagHalt) {
		return
	}

	for _, attr := range o.Attrs {
		text := eval.StripAttrPrefix(attr.Value)
		if !strings.HasPrefix(text, "^") {
			continue
		}

		// Parse "^pattern:action"
		rest := text[1:] // skip ^
		colonIdx := findUnescapedColon(rest)
		if colonIdx < 0 {
			continue
		}
		pattern := rest[:colonIdx]
		action := rest[colonIdx+1:]

		// Match the message against the pattern
		matched, args := matchWild(pattern, message)
		if !matched {
			continue
		}

		DebugLog("LISTEN MATCH obj=#%d pattern=%q action=%q args=%v", obj, pattern, action, args)
		entry := &QueueEntry{
			Player:  obj,
			Cause:   cause,
			Caller:  cause,
			Command: action,
			Args:    args,
		}
		g.Queue.Add(entry)
	}
}

// --- Helper functions ---

// findUnescapedColon finds the first unescaped ':' in a string.
func findUnescapedColon(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// matchWild performs wildcard matching and captures * groups.
// Returns (matched, captured_args).
func matchWild(pattern, str string) (bool, []string) {
	var args []string
	// Match case-insensitively but capture original-case text.
	// We pass both lowered versions (for comparison) and the original str
	// (for capturing matched segments).
	matched := matchWildHelper(strings.ToLower(pattern), strings.ToLower(str), str, 0, &args)
	return matched, args
}

// matchWildHelper matches lowered pattern against lowered str, but captures
// from origStr at the corresponding offset to preserve original case.
func matchWildHelper(pattern, str, origStr string, origOff int, args *[]string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			pattern = pattern[1:]
			if len(pattern) == 0 {
				*args = append(*args, origStr[origOff:origOff+len(str)])
				return true
			}
			// Try matching the rest of the pattern at every position
			for i := len(str); i >= 0; i-- {
				testArgs := make([]string, len(*args))
				copy(testArgs, *args)
				testArgs = append(testArgs, origStr[origOff:origOff+i])
				if matchWildHelper(pattern, str[i:], origStr, origOff+i, &testArgs) {
					*args = testArgs
					return true
				}
			}
			return false
		case '?':
			if len(str) == 0 {
				return false
			}
			// C TinyMUSH captures ? as a single-char arg (like * but one char)
			*args = append(*args, string(origStr[origOff]))
			pattern = pattern[1:]
			str = str[1:]
			origOff++
		default:
			if len(str) == 0 || pattern[0] != str[0] {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
			origOff++
		}
	}
	return len(str) == 0
}

// parseAttrFlags extracts the flags portion from "\x01owner:flags:value".
// Only parses when the ATR_INFO_CHAR (\x01) prefix is present; otherwise
// returns 0 to avoid misinterpreting command text (e.g. "$+wear *:..." colons)
// as flag separators.
func parseAttrFlags(raw string) int {
	if len(raw) == 0 || raw[0] != '\x01' {
		return 0
	}
	colonCount := 0
	start := 0
	for i := 1; i < len(raw); i++ {
		if raw[i] == ':' {
			colonCount++
			if colonCount == 1 {
				start = i + 1
			}
			if colonCount == 2 {
				flagStr := raw[start:i]
				return toIntSimple(flagStr)
			}
		}
	}
	return 0
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return start < len(s)
}

func toIntSimple(s string) int {
	n := 0
	neg := false
	i := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		i = 1
	}
	for ; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
		}
	}
	if neg {
		return -n
	}
	return n
}

// toFloat converts a string to float64, matching C's atof behavior.
// Returns 0.0 for non-numeric strings.
func toFloat(s string) float64 {
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return v
}
