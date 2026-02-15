package server

import (
	"fmt"
	"log"
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
	// Objects to search: player itself, inventory, room, room contents, master room contents
	var searchObjs []gamedb.DBRef
	debugDollar := strings.HasPrefix(input, "+") // debug + commands
	if debugDollar {
		log.Printf("DOLLARDEBUG MatchDollarCommands player=#%d input=%q", player, input)
	}

	// Player's own attributes
	searchObjs = append(searchObjs, player)

	// Player's inventory
	if pObj, ok := g.DB.Objects[player]; ok {
		next := pObj.Contents
		for next != gamedb.Nothing {
			searchObjs = append(searchObjs, next)
			if obj, ok := g.DB.Objects[next]; ok {
				next = obj.Next
			} else {
				break
			}
		}
	}

	// Room and room contents
	loc := g.PlayerLocation(player)
	if loc != gamedb.Nothing {
		searchObjs = append(searchObjs, loc)
		if locObj, ok := g.DB.Objects[loc]; ok {
			next := locObj.Contents
			for next != gamedb.Nothing {
				if next != player { // Skip player (already searched)
					searchObjs = append(searchObjs, next)
				}
				if obj, ok := g.DB.Objects[next]; ok {
					next = obj.Next
				} else {
					break
				}
			}
		}
	}

	// Master room contents — global commands live here in heavy softcode games
	masterRoom := g.MasterRoomRef()
	if loc != masterRoom {
		if mrObj, ok := g.DB.Objects[masterRoom]; ok {
			// Search master room itself
			searchObjs = append(searchObjs, masterRoom)
			// Search its contents
			next := mrObj.Contents
			for next != gamedb.Nothing {
				searchObjs = append(searchObjs, next)
				if obj, ok := g.DB.Objects[next]; ok {
					next = obj.Next
				} else {
					break
				}
			}
		}
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

	if debugDollar {
		names := make([]string, len(searchObjs))
		for i, ref := range searchObjs {
			if o, ok := g.DB.Objects[ref]; ok {
				names[i] = fmt.Sprintf("#%d(%s)", ref, o.Name)
			} else {
				names[i] = fmt.Sprintf("#%d(?)", ref)
			}
		}
		log.Printf("DOLLARDEBUG search list (%d objs): %v", len(searchObjs), names)
	}

	// Search each object's attributes
	for _, objRef := range searchObjs {
		if g.matchDollarOnObject(objRef, player, cause, input) {
			if debugDollar {
				log.Printf("DOLLARDEBUG MATCHED on #%d", objRef)
			}
			return true
		}
	}
	if debugDollar {
		log.Printf("DOLLARDEBUG NO MATCH for %q", input)
	}
	return false
}

// addZoneObjects appends a zone object and its contents to the search list.
func (g *Game) addZoneObjects(searchObjs []gamedb.DBRef, zone gamedb.DBRef) []gamedb.DBRef {
	searchObjs = append(searchObjs, zone)
	if zObj, ok := g.DB.Objects[zone]; ok {
		next := zObj.Contents
		for next != gamedb.Nothing {
			searchObjs = append(searchObjs, next)
			if obj, ok := g.DB.Objects[next]; ok {
				next = obj.Next
			} else {
				break
			}
		}
	}
	return searchObjs
}

// matchDollarOnObject checks a single object for $-command matches.
func (g *Game) matchDollarOnObject(objRef, player, cause gamedb.DBRef, input string) bool {
	obj, ok := g.DB.Objects[objRef]
	if !ok {
		return false
	}

	debugDollar := strings.HasPrefix(input, "+") && objRef == 123

	// Skip halted objects
	if obj.HasFlag(gamedb.FlagHalt) {
		if debugDollar {
			log.Printf("DOLLARDEBUG #%d HALTED, skipping", objRef)
		}
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
			if debugDollar {
				log.Printf("DOLLARDEBUG #%d attr %d: NOPROG, skipping", objRef, attr.Number)
			}
			continue
		}

		// Split "$pattern:command"
		rest := text[1:] // skip $
		colonIdx := findUnescapedColon(rest)
		if colonIdx < 0 {
			if debugDollar {
				log.Printf("DOLLARDEBUG #%d attr %d: no colon found in %q", objRef, attr.Number, text[:min(60, len(text))])
			}
			continue
		}
		pattern := rest[:colonIdx]
		command := rest[colonIdx+1:]

		// Match the pattern against input
		matched, args := matchWild(pattern, input)
		if debugDollar && dollarCount <= 5 {
			log.Printf("DOLLARDEBUG #%d attr %d: pattern=%q input=%q matched=%v", objRef, attr.Number, pattern, input, matched)
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

	for _, attr := range parent.Attrs {
		text := eval.StripAttrPrefix(attr.Value)
		if !strings.HasPrefix(text, "$") {
			continue
		}
		attrFlags := parseAttrFlags(attr.Value)
		if attrFlags&AFNoProg != 0 || attrFlags&AFPrivate != 0 {
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
		if !matched {
			continue
		}

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

		// Evaluate each individual command with args as %0-%9.
		// We do NOT use EvStrip here so that brace grouping is preserved for
		// commands like @switch that need to split on commas respecting braces.
		// In C TinyMUSH, parse_arglist splits args by parse_to (brace-aware)
		// BEFORE evaluation; we approximate this by preserving braces through eval.
		evaluated := ctx.Exec(cmd, eval.EvFCheck|eval.EvEval, entry.Args)
		evaluated = strings.TrimSpace(evaluated)
		if evaluated == "" {
			continue
		}
		if strings.Contains(strings.ToLower(evaluated), "@switch") || strings.Contains(strings.ToLower(evaluated), "@pemit") {
			log.Printf("QUEUEDEBUG ExecuteQueueEntry player=#%d evaluated=%q", entry.Player, truncDebug(evaluated, 300))
		}

		// If the result is a brace-wrapped command group (from @dolist, @wait,
		// etc.), strip the outer braces and re-split on semicolons.
		// This matches C's process_cmdline behavior where brace-grouped
		// action lists are unwrapped before command dispatch.
		if len(evaluated) >= 2 && evaluated[0] == '{' && evaluated[len(evaluated)-1] == '}' {
			inner := evaluated[1 : len(evaluated)-1]
			innerCmds := splitSemicolonRespectingBraces(inner)
			for _, ic := range innerCmds {
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
	log.Printf("OBJEXEC ExecuteAsObject player=#%d cause=#%d input=%q", player, cause, truncDebug(input, 200))

	// Handle say/pose prefixes
	switch input[0] {
	case '"':
		g.ObjSay(player, input[1:])
		return
	case ':':
		g.ObjPose(player, input[1:])
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
			g.Conns.SendToPlayer(obj.Owner, args)
		}
	case "@pemit":
		if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
			targetStr := strings.TrimSpace(args[:eqIdx])
			message := args[eqIdx+1:]
			log.Printf("OBJEXEC @pemit target=%q message=%q switches=%q", targetStr, truncDebug(message, 120), switches)
			target := g.ResolveRef(player, targetStr)
			if target == gamedb.Nothing {
				target = g.MatchObject(player, targetStr)
			}
			if target == gamedb.Nothing {
				break
			}
			if strings.HasPrefix(switches, "content") {
				// @pemit/contents: send to all contents of target
				if tObj, ok := g.DB.Objects[target]; ok {
					cur := tObj.Contents
					for cur != gamedb.Nothing {
						g.SendMarkedToPlayer(cur, "EMIT", message)
						if cObj, ok := g.DB.Objects[cur]; ok {
							cur = cObj.Next
						} else {
							break
						}
					}
				}
			} else if strings.HasPrefix(switches, "list") {
				// @pemit/list: send to multiple targets
				for _, t := range strings.Fields(targetStr) {
					ref := g.ResolveRef(player, t)
					if ref != gamedb.Nothing {
						g.SendMarkedToPlayer(ref, "EMIT", message)
					}
				}
			} else {
				g.SendMarkedToPlayer(target, "EMIT", message)
			}
		}
	case "@emit":
		loc := g.PlayerLocation(player)
		if loc != gamedb.Nothing {
			g.SendMarkedToRoom(loc, "EMIT", args)
		}
	case "@oemit":
		if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
			targetStr := strings.TrimSpace(args[:eqIdx])
			message := args[eqIdx+1:]
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
			message := args[eqIdx+1:]
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

	log.Printf("SWITCHDEBUG doSwitchObj player=#%d expr=%q exprStr=%q parts=%d", player, expr, exprStr, len(parts))
	for pi, p := range parts {
		log.Printf("SWITCHDEBUG   part[%d]=%q", pi, truncDebug(p, 120))
	}

	for i := 0; i+1 < len(parts); i += 2 {
		pattern := ctx.Exec(strings.TrimSpace(parts[i]), eval.EvFCheck|eval.EvEval, nil)
		log.Printf("SWITCHDEBUG   checking pattern=%q against expr=%q match=%v", pattern, expr, wildMatchSimple(strings.ToLower(pattern), strings.ToLower(expr)))
		if wildMatchSimple(strings.ToLower(pattern), strings.ToLower(expr)) {
			// In C TinyMUSH, do_switch dispatches the matched action body
			// to process_cmdline() for execution — it does NOT evaluate the
			// action as an expression. The action body was already partially
			// evaluated by parse_arglist (% subs and [] content), with braces
			// preserved. We strip outer braces and dispatch as command(s).
			raw := stripOuterBraces(strings.TrimSpace(parts[i+1]))
			raw = strings.ReplaceAll(raw, "#$", expr)
			log.Printf("SWITCHDEBUG   MATCHED i=%d, dispatching action=%q", i, truncDebug(raw, 200))
			g.dispatchSwitchAction(player, cause, raw)
			return
		}
	}
	if len(parts)%2 == 1 {
		raw := stripOuterBraces(strings.TrimSpace(parts[len(parts)-1]))
		raw = strings.ReplaceAll(raw, "#$", expr)
		log.Printf("SWITCHDEBUG   DEFAULT, dispatching action=%q", truncDebug(raw, 200))
	}
}

// dispatchSwitchAction executes a @switch action body as one or more commands.
// The action may contain semicolons for multiple commands and/or nested braces.
func (g *Game) dispatchSwitchAction(player, cause gamedb.DBRef, action string) {
	cmds := splitSemicolonRespectingBraces(action)
	log.Printf("SWITCHDEBUG dispatchSwitchAction player=#%d cmds=%d action=%q", player, len(cmds), truncDebug(action, 200))
	for ci, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		// Strip outer braces from sub-commands (brace groups from @dolist etc.)
		cmd = stripOuterBraces(cmd)
		log.Printf("SWITCHDEBUG   dispatch cmd[%d]=%q", ci, truncDebug(cmd, 200))
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

// ObjSay handles say for non-connected objects.
func (g *Game) ObjSay(player gamedb.DBRef, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	name := g.PlayerName(player)
	loc := g.PlayerLocation(player)
	g.SendMarkedToRoom(loc, "SAY", name+" says \""+msg+"\"")
}

// ObjPose handles pose for non-connected objects.
func (g *Game) ObjPose(player gamedb.DBRef, msg string) {
	msg = strings.TrimSpace(msg)
	name := g.PlayerName(player)
	loc := g.PlayerLocation(player)
	g.SendMarkedToRoom(loc, "POSE", name+" "+msg)
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
		return
	}

	attrName := strings.ToUpper(strings.TrimSpace(parts[1]))
	text := g.GetAttrTextByName(target, attrName)
	if text == "" {
		return
	}

	// Parse comma-separated args
	var trigArgs []string
	if argStr != "" {
		trigArgs = strings.Split(argStr, ",")
		for i := range trigArgs {
			trigArgs[i] = strings.TrimSpace(trigArgs[i])
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
	text := g.GetAttrTextByName(target, attrName)
	if text == "" {
		return
	}

	// Parse comma-separated args
	var trigArgs []string
	if argStr != "" {
		trigArgs = strings.Split(argStr, ",")
		for i := range trigArgs {
			trigArgs[i] = strings.TrimSpace(trigArgs[i])
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
func (g *Game) ProcessQueue() {
	// Move ready entries from wait queue
	g.Queue.PromoteReady()

	// Process up to N entries per tick (10ms tick × 500/tick = 50,000 entries/sec max)
	maxPerTick := 500
	for i := 0; i < maxPerTick; i++ {
		entry := g.Queue.PopImmediate()
		if entry == nil {
			break
		}
		g.safeExecuteQueueEntry(entry)
	}
}

// safeExecuteQueueEntry wraps ExecuteQueueEntry with panic recovery and a
// watchdog that logs slow entries (but still blocks until completion).
func (g *Game) safeExecuteQueueEntry(entry *QueueEntry) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in queue entry (player=#%d cmd=%q): %v",
				entry.Player, entry.Command, r)
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
func (g *Game) StartQueueProcessor() {
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		heartbeat := time.NewTicker(60 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("PANIC in queue processor: %v", r)
						}
					}()
					g.ProcessQueue()
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
func (g *Game) MatchListenPatterns(loc gamedb.DBRef, speaker gamedb.DBRef, message string) {
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return
	}

	// Walk contents of the room
	next := locObj.Contents
	for next != gamedb.Nothing {
		obj, ok := g.DB.Objects[next]
		if !ok {
			break
		}
		// Check for MONITOR flag (or HAS_LISTEN)
		if next != speaker && (obj.HasFlag(gamedb.FlagMonitor) || obj.HasFlag2(gamedb.Flag2HasListen)) {
			g.checkListenAttrs(next, speaker, message)
		}
		next = obj.Next
	}

	// Also check the room itself
	if loc != speaker && (locObj.HasFlag(gamedb.FlagMonitor) || locObj.HasFlag2(gamedb.Flag2HasListen)) {
		g.checkListenAttrs(loc, speaker, message)
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
	matched := matchWildHelper(strings.ToLower(pattern), strings.ToLower(str), &args)
	return matched, args
}

func matchWildHelper(pattern, str string, args *[]string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			pattern = pattern[1:]
			if len(pattern) == 0 {
				*args = append(*args, str)
				return true
			}
			// Try matching the rest of the pattern at every position
			for i := len(str); i >= 0; i-- {
				testArgs := make([]string, len(*args))
				copy(testArgs, *args)
				testArgs = append(testArgs, str[:i])
				if matchWildHelper(pattern, str[i:], &testArgs) {
					*args = testArgs
					return true
				}
			}
			return false
		case '?':
			if len(str) == 0 {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
		default:
			if len(str) == 0 || pattern[0] != str[0] {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
		}
	}
	return len(str) == 0
}

// parseAttrFlags extracts the flags portion from "owner:flags:value".
func parseAttrFlags(raw string) int {
	colonCount := 0
	start := 0
	for i, ch := range raw {
		if ch == ':' {
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
