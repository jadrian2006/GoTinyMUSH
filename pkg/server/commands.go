package server

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// CommandHandler is the signature for game command implementations.
type CommandHandler func(g *Game, d *Descriptor, args string, switches []string)

// Command represents a registered game command.
type Command struct {
	Name    string
	Handler CommandHandler
	NoGuest bool // if true, guests cannot use this command
}

// InitCommands registers all available game commands.
// Aliases are loaded separately from goTinyAlias.conf via LoadAliasConfig.
func InitCommands() map[string]*Command {
	cmds := make(map[string]*Command)

	register := func(name string, handler CommandHandler) {
		cmds[strings.ToLower(name)] = &Command{Name: name, Handler: handler}
	}
	registerNG := func(name string, handler CommandHandler) {
		cmds[strings.ToLower(name)] = &Command{Name: name, Handler: handler, NoGuest: true}
	}

	// Communication
	register("say", cmdSay)
	register("\"", cmdSay)
	register("pose", cmdPose)
	register(":", cmdPose)
	register(";", cmdPoseNoSpc)
	register("page", cmdPage)
	register("@emit", cmdEmit)
	register("think", cmdThink)
	register("@pemit", cmdPemit)

	// Movement
	register("go", cmdGo)
	register("home", cmdHome)

	// Information
	register("look", cmdLook)
	register("examine", cmdExamine)
	register("inventory", cmdInventory)
	register("WHO", cmdWho)
	register("DOING", cmdDoing)
	register("score", cmdScore)

	// Building (no guest)
	registerNG("@dig", cmdDig)
	registerNG("@open", cmdOpen)
	registerNG("@describe", cmdDescribe)
	registerNG("@name", cmdRename)
	registerNG("@set", cmdSet)
	registerNG("@create", cmdCreate)
	registerNG("@destroy", cmdDestroy)
	registerNG("@link", cmdLink)
	registerNG("@unlink", cmdUnlink)
	registerNG("@parent", cmdParent)
	registerNG("@chown", cmdChown)
	registerNG("@clone", cmdClone)
	registerNG("@wipe", cmdWipe)
	registerNG("@lock", cmdLock)
	registerNG("@unlock", cmdUnlock)

	// Admin/wizard (no guest)
	registerNG("@teleport", cmdTeleport)
	registerNG("@force", cmdForce)
	registerNG("@trigger", cmdTriggerCmd)
	registerNG("@wait", cmdWaitCmd)
	registerNG("@notify", cmdNotify)
	registerNG("@halt", cmdHalt)
	registerNG("@boot", cmdBoot)
	registerNG("@wall", cmdWall)
	registerNG("@newpassword", cmdNewPassword)
	registerNG("@find", cmdFind)
	registerNG("@stats", cmdStats)
	registerNG("@ps", cmdPs)

	// Eval / softcode
	register("@eval", cmdEval)
	registerNG("@switch", cmdSwitch)
	registerNG("@dolist", cmdDolist)
	registerNG("@program", cmdProgram)
	register("@quitprogram", cmdQuitProgram)

	// Database (no guest)
	registerNG("@dump", cmdDump)
	registerNG("@backup", cmdBackup)
	registerNG("@readcache", cmdReadCache)
	registerNG("@archive", cmdArchive)

	// Softcode / Queue management (no guest)
	registerNG("@function", cmdFunction)
	registerNG("@drain", cmdDrain)
	registerNG("@edit", cmdEdit)
	registerNG("@admin", cmdAdmin)

	// Attribute management (no guest)
	registerNG("@attribute", cmdAttribute)
	register("@attlist", cmdAttlist)

	// SQL (no guest)
	registerNG("@sql", cmdSQL)
	registerNG("@sqlinit", cmdSQLInit)
	registerNG("@sqldisconnect", cmdSQLDisconnect)

	// Session
	register("QUIT", cmdQuit)
	register("@doing", cmdSetDoing)

	// Help system
	register("help", cmdHelp)
	register("@help", cmdHelp)
	register("qhelp", cmdQhelp)
	register("wizhelp", cmdWizhelp)
	register("news", cmdNews)
	register("man", cmdMan)
	register("wiznews", cmdWizNews)
	register("+jhelp", cmdJhelp)
	// NOTE: +help is NOT registered here. CrystalMUSH uses softcode $+help
	// on Global Commands(#123) in the master room. The original crystal.conf
	// has "helpfile +help text/plushelp" commented out (line 47).

	// Player object commands
	registerNG("get", cmdGet)
	registerNG("take", cmdGet)
	registerNG("drop", cmdDrop)
	registerNG("give", cmdGive)
	register("enter", cmdEnter)
	register("leave", cmdLeave)
	register("whisper", cmdWhisper)
	register("use", cmdUse)
	registerNG("kill", cmdKill)

	// Communication
	register("@oemit", cmdOemit)
	register("@remit", cmdRemit)

	// Admin/Builder utilities
	registerNG("@password", cmdPassword)
	register("@version", cmdVersion)
	register("version", cmdVersion)
	register("@motd", cmdMotd)
	registerNG("@chzone", cmdChzone)
	registerNG("@search", cmdSearch)
	registerNG("@decompile", cmdDecompile)
	registerNG("@power", cmdPower)

	// Attribute-setting @commands (all no guest)
	// Success/Failure messages
	registerNG("@success", makeAttrSetter(4))     // A_SUCC
	registerNG("@osuccess", makeAttrSetter(1))     // A_OSUCC
	registerNG("@asuccess", makeAttrSetter(12))    // A_ASUCC
	registerNG("@fail", makeAttrSetter(3))         // A_FAIL
	registerNG("@ofail", makeAttrSetter(2))        // A_OFAIL
	registerNG("@afail", makeAttrSetter(13))       // A_AFAIL
	registerNG("@drop", makeAttrSetter(9))         // A_DROP (attribute setter)
	registerNG("@odrop", makeAttrSetter(8))        // A_ODROP
	registerNG("@adrop", makeAttrSetter(14))       // A_ADROP
	registerNG("@kill", makeAttrSetter(11))        // A_KILL
	registerNG("@okill", makeAttrSetter(10))       // A_OKILL
	registerNG("@akill", makeAttrSetter(15))       // A_AKILL
	// Enter/Leave attributes — numbers from constants.h
	registerNG("@enter", makeAttrSetter(33))       // A_ENTER = 33
	registerNG("@oenter", makeAttrSetter(53))      // A_OENTER = 53
	registerNG("@oxenter", makeAttrSetter(34))     // A_OXENTER = 34
	registerNG("@aenter", makeAttrSetter(35))      // A_AENTER = 35
	registerNG("@leave", makeAttrSetter(50))       // A_LEAVE = 50
	registerNG("@oleave", makeAttrSetter(51))      // A_OLEAVE = 51
	registerNG("@aleave", makeAttrSetter(52))      // A_ALEAVE = 52
	registerNG("@oxleave", makeAttrSetter(54))     // A_OXLEAVE = 54
	// Use attributes
	registerNG("@use", makeAttrSetter(45))         // A_USE = 45
	registerNG("@ouse", makeAttrSetter(46))        // A_OUSE = 46
	registerNG("@ause", makeAttrSetter(16))        // A_AUSE = 16
	// Player info
	registerNG("@sex", makeAttrSetter(7))          // A_SEX = 7
	registerNG("@alias", makeAttrSetter(58))       // A_ALIAS = 58
	registerNG("@away", makeAttrSetter(73))        // A_AWAY = 73
	registerNG("@idle", makeAttrSetter(74))        // A_IDLE = 74
	registerNG("@listen", makeAttrSetter(26))      // A_LISTEN = 26
	registerNG("@ahear", makeAttrSetter(29))       // A_AHEAR = 29
	// Move attributes
	registerNG("@move", makeAttrSetter(55))        // A_MOVE = 55
	registerNG("@omove", makeAttrSetter(56))       // A_OMOVE = 56
	registerNG("@amove", makeAttrSetter(57))       // A_AMOVE = 57
	// Description variants
	registerNG("@odescribe", makeAttrSetter(37))   // A_ODESC = 37
	registerNG("@adescribe", makeAttrSetter(36))   // A_ADESC = 36
	registerNG("@idesc", makeAttrSetter(32))       // A_IDESC = 32
	// Payment
	registerNG("@pay", makeAttrSetter(23))         // A_PAY = 23
	registerNG("@opay", makeAttrSetter(22))        // A_OPAY = 22
	registerNG("@apay", makeAttrSetter(21))        // A_APAY = 21
	registerNG("@cost", makeAttrSetter(24))        // A_COST = 24
	// Startup/daily
	registerNG("@startup", makeAttrSetter(19))     // A_STARTUP = 19
	registerNG("@daily", makeAttrSetter(204))      // A_DAILY = 204
	// Format overrides
	registerNG("@conformat", makeAttrSetter(214))  // A_LCON_FMT = 214
	registerNG("@exitformat", makeAttrSetter(215)) // A_LEXITS_FMT = 215
	registerNG("@nameformat", makeAttrSetter(222)) // A_NAME_FMT = 222
	// Enter/Leave aliases
	registerNG("@ealias", makeAttrSetter(64))      // A_EALIAS = 64
	registerNG("@lalias", makeAttrSetter(65))      // A_LALIAS = 65
	// Filtering
	registerNG("@filter", makeAttrSetter(92))      // A_FILTER = 92
	registerNG("@infilter", makeAttrSetter(91))    // A_INFILTER = 91
	registerNG("@forwardlist", makeAttrSetter(95)) // A_FORWARDLIST = 95
	registerNG("@prefix", makeAttrSetter(90))      // A_PREFIX = 90
	registerNG("@inprefix", makeAttrSetter(89))    // A_INPREFIX = 89
	// Enter/Leave/Use failure variants
	registerNG("@efail", makeAttrSetter(66))       // A_EFAIL = 66
	registerNG("@oefail", makeAttrSetter(67))      // A_OEFAIL = 67
	registerNG("@aefail", makeAttrSetter(68))      // A_AEFAIL = 68
	registerNG("@lfail", makeAttrSetter(69))       // A_LFAIL = 69
	registerNG("@olfail", makeAttrSetter(70))      // A_OLFAIL = 70
	registerNG("@alfail", makeAttrSetter(71))      // A_ALFAIL = 71
	registerNG("@ufail", makeAttrSetter(75))       // A_UFAIL = 75
	registerNG("@oufail", makeAttrSetter(76))      // A_OUFAIL = 76
	registerNG("@aufail", makeAttrSetter(77))      // A_AUFAIL = 77
	// Teleport messages
	registerNG("@tport", makeAttrSetter(79))       // A_TPORT = 79
	registerNG("@otport", makeAttrSetter(80))      // A_OTPORT = 80
	registerNG("@oxtport", makeAttrSetter(81))     // A_OXTPORT = 81
	registerNG("@atport", makeAttrSetter(82))      // A_ATPORT = 82
	// Charges
	registerNG("@charges", makeAttrSetter(17))     // A_CHARGES = 17
	registerNG("@runout", makeAttrSetter(18))      // A_RUNOUT = 18
	// Reject
	registerNG("@reject", makeAttrSetter(72))      // A_REJECT = 72

	// Spellcheck
	registerNG("@dictionary", cmdDictionary)

	// Comsys (channel system)
	register("addcom", cmdAddcom)
	register("delcom", cmdDelcom)
	register("clearcom", cmdClearcom)
	register("comlist", cmdComlist)
	register("comtitle", cmdComtitle)
	register("allcom", cmdAllcom)
	registerNG("@ccreate", cmdCcreate)
	registerNG("@cdestroy", cmdCdestroy)
	register("@clist", cmdClist)
	register("@cwho", cmdCwho)
	registerNG("@cboot", cmdCboot)
	registerNG("@cemit", cmdCemit)
	registerNG("@cset", cmdCset)
	registerNG("@cinfo", cmdCinfo)

	// Mail system (no guest)
	registerNG("@mail", cmdMail)
	registerNG("-", cmdMailDash)

	return cmds
}

// DispatchCommand parses and dispatches a player command.
func DispatchCommand(g *Game, d *Descriptor, input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle single-character prefixes: " for say, : for pose, ; for pose-nospc, & for setvattr
	switch input[0] {
	case '"':
		cmdSay(g, d, input[1:], nil)
		return
	case ':':
		cmdPose(g, d, input[1:], nil)
		return
	case ';':
		cmdPoseNoSpc(g, d, input[1:], nil)
		return
	case '&':
		if g.IsGuest(d.Player) {
			d.Send("Permission denied.")
			return
		}
		cmdSetVAttr(g, d, input[1:], nil)
		return
	}

	// Split command and args
	var cmdName, args string
	spaceIdx := strings.IndexByte(input, ' ')
	if spaceIdx >= 0 {
		cmdName = input[:spaceIdx]
		args = strings.TrimSpace(input[spaceIdx+1:])
	} else {
		cmdName = input
	}

	// Parse /switches from command name (e.g. "@dolist/now" -> "@dolist", ["now"])
	var switches []string
	if slashIdx := strings.IndexByte(cmdName, '/'); slashIdx >= 0 {
		parts := strings.Split(cmdName, "/")
		cmdName = parts[0]
		switches = parts[1:]
	}

	// Look up command (exact match first)
	lower := strings.ToLower(cmdName)
	if cmd, ok := g.Commands[lower]; ok {
		if cmd.NoGuest && g.IsGuest(d.Player) {
			d.Send("Permission denied.")
			return
		}
		cmd.Handler(g, d, args, switches)
		return
	}

	// Prefix matching for @-commands: C TinyMUSH allows abbreviations
	// (e.g., @fo = @force, @sw = @switch, @tr = @trigger)
	if len(lower) > 1 && lower[0] == '@' {
		var matchedCmd *Command
		matchCount := 0
		for name, cmd := range g.Commands {
			if strings.HasPrefix(name, lower) {
				matchedCmd = cmd
				matchCount++
			}
		}
		if matchCount == 1 && matchedCmd != nil {
			if matchedCmd.NoGuest && g.IsGuest(d.Player) {
				d.Send("Permission denied.")
				return
			}
			matchedCmd.Handler(g, d, args, switches)
			return
		}
	}

	// Unrecognized @<attr> commands: treat as &<attr> (set variable attribute).
	// Many MUSHes use @va-@vz and similar as shorthand for setting attributes.
	if len(lower) > 1 && lower[0] == '@' && args != "" {
		attrName := lower[1:]
		// Only do this if it looks like an attribute set (has obj=value)
		if strings.Contains(args, "=") {
			if g.IsGuest(d.Player) {
				d.Send("Permission denied.")
				return
			}
			cmdSetVAttr(g, d, attrName+" "+args, nil)
			return
		}
	}

	// Try channel alias matching
	if g.Comsys != nil {
		if ca := g.Comsys.LookupAlias(d.Player, strings.ToLower(cmdName)); ca != nil {
			g.ComsysProcessAlias(d, ca, args)
			return
		}
	}

	// Try to match as an exit name
	if tryMoveByExit(g, d, input) {
		return
	}

	// Check enter/leave aliases (C TinyMUSH: A_LALIAS/A_EALIAS on objects)
	// When inside an object, its LALIAS lists aliases that trigger "leave".
	// EALIAS on objects in the room lists aliases that trigger "enter <obj>".
	if tryEnterLeaveAlias(g, d, input) {
		return
	}

	// Try $-command matching on objects in room/inventory
	if g.MatchDollarCommands(d.Player, d.Player, input) {
		return
	}

	d.Send("Huh?  (Type \"help\" for help.)")
}

// HasSwitch checks if a switch list contains a specific switch (case-insensitive).
func HasSwitch(switches []string, name string) bool {
	for _, s := range switches {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

// --- Communication Commands ---

// evalExpr evaluates softcode in a string (function calls in [], %substitutions).
func evalExpr(g *Game, player gamedb.DBRef, text string) string {
	ctx := MakeEvalContextWithGame(g, player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	return ctx.Exec(text, eval.EvFCheck|eval.EvEval, nil)
}

func cmdSay(g *Game, d *Descriptor, args string, _ []string) {
	args = strings.TrimSpace(args)
	if args == "" {
		d.Send("Say what?")
		return
	}
	args = evalExpr(g, d.Player, args)
	playerName := g.PlayerName(d.Player)
	loc := g.PlayerLocation(d.Player)

	// Emit structured event to self
	g.EmitEvent(d.Player, "SAY", events.Event{
		Type:   events.EvSay,
		Source: d.Player,
		Room:   loc,
		Text:   fmt.Sprintf("You say \"%s\"", args),
		Data:   map[string]any{"message": args, "speaker": playerName},
	})
	// Emit structured event to room (except speaker)
	msg := fmt.Sprintf("%s says \"%s\"", playerName, args)
	g.EmitEventToRoomExcept(loc, d.Player, "SAY", events.Event{
		Type:   events.EvSay,
		Source: d.Player,
		Room:   loc,
		Text:   msg,
		Data:   map[string]any{"message": args, "speaker": playerName},
	})
	g.MatchListenPatterns(loc, d.Player, msg)
}

func cmdPose(g *Game, d *Descriptor, args string, _ []string) {
	args = evalExpr(g, d.Player, strings.TrimSpace(args))
	playerName := g.PlayerName(d.Player)
	loc := g.PlayerLocation(d.Player)
	msg := fmt.Sprintf("%s %s", playerName, args)
	g.EmitEventToRoom(loc, "POSE", events.Event{
		Type:   events.EvPose,
		Source: d.Player,
		Room:   loc,
		Text:   msg,
		Data:   map[string]any{"pose": args, "player": playerName},
	})
	g.MatchListenPatterns(loc, d.Player, msg)
}

func cmdPoseNoSpc(g *Game, d *Descriptor, args string, _ []string) {
	args = evalExpr(g, d.Player, args)
	playerName := g.PlayerName(d.Player)
	loc := g.PlayerLocation(d.Player)
	msg := fmt.Sprintf("%s%s", playerName, args)
	g.EmitEventToRoom(loc, "POSE", events.Event{
		Type:   events.EvPose,
		Source: d.Player,
		Room:   loc,
		Text:   msg,
		Data:   map[string]any{"pose": args, "player": playerName, "nospace": true},
	})
	g.MatchListenPatterns(loc, d.Player, msg)
}

func cmdPage(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Page whom?")
		return
	}
	// Format: page name=message or page name message
	var targetName, message string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetName = strings.TrimSpace(args[:eqIdx])
		message = strings.TrimSpace(args[eqIdx+1:])
	} else {
		parts := strings.SplitN(args, " ", 2)
		targetName = parts[0]
		if len(parts) > 1 {
			message = parts[1]
		}
	}

	target := LookupPlayer(g.DB, targetName)
	if target == gamedb.Nothing {
		d.Send("I don't recognize that player.")
		return
	}

	if !g.Conns.IsConnected(target) {
		targetObj := g.DB.Objects[target]
		d.Send(fmt.Sprintf("%s is not connected.", DisplayName(targetObj.Name)))
		return
	}

	senderName := g.PlayerName(d.Player)
	targetObj := g.DB.Objects[target]

	pageData := map[string]any{
		"sender":  senderName,
		"target":  DisplayName(targetObj.Name),
		"message": message,
	}

	if message == "" {
		g.EmitEvent(d.Player, "PAGE", events.Event{
			Type: events.EvPage, Source: d.Player,
			Text: fmt.Sprintf("You page %s.", DisplayName(targetObj.Name)),
			Data: pageData,
		})
		g.EmitEvent(target, "PAGE", events.Event{
			Type: events.EvPage, Source: d.Player,
			Text: fmt.Sprintf("%s pages you.", senderName),
			Data: pageData,
		})
	} else {
		message = evalExpr(g, d.Player, message)
		pageData["message"] = message
		if strings.HasPrefix(message, ":") {
			pose := strings.TrimPrefix(message, ":")
			g.EmitEvent(d.Player, "PAGE", events.Event{
				Type: events.EvPage, Source: d.Player,
				Text: fmt.Sprintf("Long distance to %s: %s %s", DisplayName(targetObj.Name), senderName, pose),
				Data: pageData,
			})
			g.EmitEvent(target, "PAGE", events.Event{
				Type: events.EvPage, Source: d.Player,
				Text: fmt.Sprintf("From afar, %s %s", senderName, pose),
				Data: pageData,
			})
		} else if strings.HasPrefix(message, ";") {
			pose := strings.TrimPrefix(message, ";")
			g.EmitEvent(d.Player, "PAGE", events.Event{
				Type: events.EvPage, Source: d.Player,
				Text: fmt.Sprintf("Long distance to %s: %s%s", DisplayName(targetObj.Name), senderName, pose),
				Data: pageData,
			})
			g.EmitEvent(target, "PAGE", events.Event{
				Type: events.EvPage, Source: d.Player,
				Text: fmt.Sprintf("From afar, %s%s", senderName, pose),
				Data: pageData,
			})
		} else {
			g.EmitEvent(d.Player, "PAGE", events.Event{
				Type: events.EvPage, Source: d.Player,
				Text: fmt.Sprintf("You page %s with \"%s\"", DisplayName(targetObj.Name), message),
				Data: pageData,
			})
			g.EmitEvent(target, "PAGE", events.Event{
				Type: events.EvPage, Source: d.Player,
				Text: fmt.Sprintf("%s pages: %s", senderName, message),
				Data: pageData,
			})
		}
	}
}

func cmdEmit(g *Game, d *Descriptor, args string, switches []string) {
	if args == "" {
		return
	}

	if HasSwitch(switches, "room") {
		// @emit/room target=message — emit to the room containing target
		eqIdx := strings.IndexByte(args, '=')
		if eqIdx < 0 {
			d.Send("Usage: @emit/room target = message")
			return
		}
		targetStr := strings.TrimSpace(args[:eqIdx])
		message := strings.TrimSpace(args[eqIdx+1:])
		targetStr = evalExpr(g, d.Player, targetStr)
		message = evalExpr(g, d.Player, message)
		target := g.ResolveRef(d.Player, targetStr)
		if target == gamedb.Nothing {
			target = g.MatchObject(d.Player, targetStr)
		}
		if target == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
		// Emit to the room of the target
		loc := g.PlayerLocation(target)
		if loc == gamedb.Nothing {
			if obj, ok := g.DB.Objects[target]; ok {
				loc = obj.Location
			}
		}
		if loc != gamedb.Nothing {
			g.EmitEventToRoom(loc, "EMIT", events.Event{
				Type:   events.EvEmit,
				Source: d.Player,
				Room:   loc,
				Text:   message,
			})
			g.MatchListenPatterns(loc, d.Player, message)
		}
		return
	}

	args = evalExpr(g, d.Player, args)
	loc := g.PlayerLocation(d.Player)
	g.EmitEventToRoom(loc, "EMIT", events.Event{
		Type:   events.EvEmit,
		Source: d.Player,
		Room:   loc,
		Text:   args,
	})
	g.MatchListenPatterns(loc, d.Player, args)
}

func cmdThink(g *Game, d *Descriptor, args string, _ []string) {
	// Evaluate the expression and show result only to the player
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	result := ctx.Exec(args, eval.EvFCheck|eval.EvEval, nil)
	d.Send(result)
}

func cmdPemit(g *Game, d *Descriptor, args string, switches []string) {
	// @pemit target=message
	// @pemit/contents target=message  (send to all contents of target)
	// @pemit/list targets=message     (targets is space-separated dbrefs)
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("@pemit: I need a target and message separated by =.")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	message := strings.TrimSpace(args[eqIdx+1:])

	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	targetStr = ctx.Exec(targetStr, eval.EvFCheck|eval.EvEval, nil)
	message = ctx.Exec(message, eval.EvFCheck|eval.EvEval, nil)

	if HasSwitch(switches, "contents") {
		// @pemit/contents: send to all contents of the target location
		target := g.ResolveRef(d.Player, targetStr)
		if target == gamedb.Nothing {
			target = g.MatchObject(d.Player, targetStr)
		}
		if target == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
		if _, ok := g.DB.Objects[target]; !ok {
			d.Send("I don't see that here.")
			return
		}
		for _, cur := range g.DB.SafeContents(target) {
			g.SendMarkedToPlayer(cur, "EMIT", message)
		}
		return
	}

	if HasSwitch(switches, "list") {
		// @pemit/list: send to each dbref in space-separated list
		targets := strings.Fields(targetStr)
		for _, ts := range targets {
			ref := g.ResolveRef(d.Player, strings.TrimSpace(ts))
			if ref != gamedb.Nothing {
				g.SendMarkedToPlayer(ref, "EMIT", message)
			}
		}
		return
	}

	// Default: single target
	target := g.ResolveRef(d.Player, targetStr)
	if target == gamedb.Nothing {
		target = LookupPlayer(g.DB, targetStr)
	}
	if target == gamedb.Nothing {
		target = g.MatchObject(d.Player, targetStr)
	}
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	g.SendMarkedToPlayer(target, "EMIT", message)
}

// --- Movement Commands ---

func cmdGo(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Go where?")
		return
	}
	if !tryMoveByExit(g, d, args) {
		d.Send("You can't go that way.")
	}
}

func tryMoveByExit(g *Game, d *Descriptor, name string) bool {
	loc := g.PlayerLocation(d.Player)
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return false
	}

	// Walk exits chain
	seenExits := make(map[gamedb.DBRef]bool)
	exitRef := locObj.Exits
	for exitRef != gamedb.Nothing && !seenExits[exitRef] {
		seenExits[exitRef] = true
		exitObj, ok := g.DB.Objects[exitRef]
		if !ok {
			break
		}
		// Exit names can have aliases separated by ;
		// TinyMUSH uses prefix matching: "o" matches "Out", "ou" matches "Out", etc.
		exitNames := strings.Split(exitObj.Name, ";")
		for _, ename := range exitNames {
			ename = strings.TrimSpace(ename)
			if len(name) > 0 && len(ename) >= len(name) && strings.EqualFold(ename[:len(name)], name) {
				// Found matching exit - move player
				// TinyMUSH stores exit destination in Location field
				dest := exitObj.Location
				if dest == gamedb.Nothing || dest == gamedb.Home {
					// Home exit
					playerObj := g.DB.Objects[d.Player]
					dest = playerObj.Link
				}
				if dest == gamedb.Nothing {
					d.Send("That exit doesn't lead anywhere.")
					return true
				}
				// Check exit lock
				if !CouldDoIt(g, d.Player, exitRef, aLock) {
					HandleLockFailure(g, d, exitRef, aFail, aOFail, aAFail, "You can't go that way.")
					return true
				}
				// Exit SUCC (4) to player, OSUCC (1) to room, ASUCC (12) action
				if succ := g.GetAttrText(exitRef, 4); succ != "" {
					ctx := MakeEvalContextForObj(g, exitRef, d.Player, func(c *eval.EvalContext) {
						functions.RegisterAll(c)
					})
					msg := ctx.Exec(succ, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
					if msg != "" {
						d.Send(msg)
					}
				}
				if osucc := g.GetAttrText(exitRef, 1); osucc != "" {
					ctx := MakeEvalContextForObj(g, exitRef, d.Player, func(c *eval.EvalContext) {
						functions.RegisterAll(c)
					})
					msg := ctx.Exec(osucc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
					if msg != "" {
						g.Conns.SendToRoomExcept(g.DB, loc, d.Player, msg)
					}
				}
				g.QueueAttrAction(exitRef, d.Player, 12, nil) // exit ASUCC
				g.MovePlayer(d, dest)
				return true
			}
		}
		exitRef = exitObj.Next
	}
	return false
}

// matchesExitFromList checks if cmd matches any alias in a semicolon-separated
// alias list (like EALIAS/LALIAS values). Uses case-insensitive prefix matching,
// matching C TinyMUSH's matches_exit_from_list behavior.
func matchesExitFromList(cmd, aliasList string) bool {
	if aliasList == "" {
		return false
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	for _, alias := range strings.Split(aliasList, ";") {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if len(cmd) <= len(alias) && strings.EqualFold(alias[:len(cmd)], cmd) {
			return true
		}
	}
	return false
}

// tryEnterLeaveAlias checks enter/leave aliases on objects.
// C TinyMUSH checks A_LALIAS on the player's location (for "leave" triggers)
// and A_EALIAS on objects in the room (for "enter" triggers).
func tryEnterLeaveAlias(g *Game, d *Descriptor, cmd string) bool {
	playerObj, ok := g.DB.Objects[d.Player]
	if !ok {
		return false
	}
	loc := playerObj.Location
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return false
	}

	// Check LALIAS on current location (leave alias)
	if lalias := g.GetAttrText(loc, 65); lalias != "" { // A_LALIAS = 65
		if matchesExitFromList(cmd, lalias) {
			cmdLeave(g, d, "", nil)
			return true
		}
	}

	// Check EALIAS on objects in the room (enter alias)
	seen := make(map[gamedb.DBRef]bool)
	next := locObj.Contents
	for next != gamedb.Nothing && !seen[next] {
		seen[next] = true
		obj, ok := g.DB.Objects[next]
		if !ok {
			break
		}
		if next != d.Player {
			if ealias := g.GetAttrText(next, 64); ealias != "" { // A_EALIAS = 64
				if matchesExitFromList(cmd, ealias) {
					cmdEnter(g, d, fmt.Sprintf("#%d", next), nil)
					return true
				}
			}
		}
		next = obj.Next
	}

	return false
}

func cmdHome(g *Game, d *Descriptor, _ string, _ []string) {
	playerObj, ok := g.DB.Objects[d.Player]
	if !ok {
		return
	}
	home := playerObj.Link
	if home == gamedb.Nothing {
		d.Send("You have no home!")
		return
	}
	d.Send("There's no place like home...")
	g.MovePlayer(d, home)
}

// --- Information Commands ---

func cmdLook(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" || strings.EqualFold(args, "here") {
		// Look at current room
		loc := g.PlayerLocation(d.Player)
		g.ShowRoom(d, loc)
		return
	}
	// Look at something specific
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	g.ShowObject(d, target)
}

func cmdExamine(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Examine what?")
		return
	}

	// Handle examine obj/attr syntax
	objName := args
	attrName := ""
	if idx := strings.IndexByte(args, '/'); idx >= 0 {
		objName = args[:idx]
		attrName = args[idx+1:]
	}

	target := g.MatchObject(d.Player, objName)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}

	// Check if player can examine this object
	if !Examinable(g, d.Player, target) {
		// Non-examinable: just show the description like look
		g.ShowObject(d, target)
		return
	}

	if attrName != "" {
		// C TinyMUSH always uses parse_attrib_wild + exam_wildattrs for
		// obj/attr — both exact names and wildcard patterns go through
		// the same quick_wild matching path.
		pattern := strings.ToLower(strings.TrimSpace(attrName))
		obj, ok := g.DB.Objects[target]
		if !ok {
			d.Send("I don't see that here.")
			return
		}
		found := false
		for _, attr := range obj.Attrs {
			name := g.DB.GetAttrName(attr.Number)
			if name == "" {
				name = fmt.Sprintf("ATTR_%d", attr.Number)
			}
			if !wildMatchSimple(pattern, strings.ToLower(name)) {
				continue
			}
			info := ParseAttrInfo(attr.Value)
			def := g.LookupAttrDef(attr.Number)
			if !CanReadAttr(g, d.Player, target, def, info.Flags, info.Owner) {
				continue
			}
			text := eval.StripAttrPrefix(attr.Value)
			// C TinyMUSH: only show annotation if player controls object or owns attr
			showAnnotation := Controls(g, d.Player, target) || info.Owner == d.Player
			annotation := ""
			if showAnnotation {
				annotation = attrAnnotation(g, d.Player, info, def)
			}
			if annotation != "" {
				d.Send(fmt.Sprintf("  %s %s: %s", name, annotation, text))
			} else {
				d.Send(fmt.Sprintf("  %s: %s", name, text))
			}
			found = true
		}
		if !found {
			d.Send("No matching attributes found.")
		}
		return
	}

	g.ShowExamine(d, target)
}

func cmdInventory(g *Game, d *Descriptor, _ string, _ []string) {
	if _, ok := g.DB.Objects[d.Player]; !ok {
		return
	}
	contents := g.DB.SafeContents(d.Player)
	if len(contents) == 0 {
		d.Send("You aren't carrying anything.")
		return
	}
	d.Send("You are carrying:")
	for _, next := range contents {
		if obj, ok := g.DB.Objects[next]; ok {
			d.Send(fmt.Sprintf("  %s", DisplayName(obj.Name)))
		}
	}
}

func cmdWho(g *Game, d *Descriptor, _ string, _ []string) {
	g.ShowWho(d)
}

func cmdDoing(g *Game, d *Descriptor, _ string, _ []string) {
	g.ShowWho(d)
}

func cmdScore(g *Game, d *Descriptor, _ string, _ []string) {
	playerObj, ok := g.DB.Objects[d.Player]
	if !ok {
		return
	}
	d.Send(fmt.Sprintf("You have %d %s.", playerObj.Pennies, g.MoneyName(playerObj.Pennies)))
}

// --- Building Commands ---

func cmdDig(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Dig what?")
		return
	}
	// @dig name[=exit_to[;alias],exit_from[;alias]]
	parts := strings.SplitN(args, "=", 2)
	roomName := strings.TrimSpace(parts[0])

	newRef := g.CreateObject(roomName, gamedb.TypeRoom, d.Player)
	d.Send(fmt.Sprintf("Room %s created as #%d.", roomName, newRef))

	// Handle exit creation if specified
	if len(parts) > 1 {
		exitParts := strings.SplitN(parts[1], ",", 2)
		if exitParts[0] != "" {
			exitTo := strings.TrimSpace(exitParts[0])
			exitRef := g.CreateExit(exitTo, g.PlayerLocation(d.Player), newRef, d.Player)
			d.Send(fmt.Sprintf("Exit %s created as #%d.", exitTo, exitRef))
		}
		if len(exitParts) > 1 && exitParts[1] != "" {
			exitFrom := strings.TrimSpace(exitParts[1])
			exitRef := g.CreateExit(exitFrom, newRef, g.PlayerLocation(d.Player), d.Player)
			d.Send(fmt.Sprintf("Exit %s created as #%d.", exitFrom, exitRef))
		}
	}
}

func cmdOpen(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Open what?")
		return
	}
	// @open exit_name=destination
	parts := strings.SplitN(args, "=", 2)
	exitName := strings.TrimSpace(parts[0])
	dest := gamedb.Nothing
	if len(parts) > 1 {
		dest = g.ResolveRef(d.Player, strings.TrimSpace(parts[1]))
	}
	loc := g.PlayerLocation(d.Player)
	exitRef := g.CreateExit(exitName, loc, dest, d.Player)
	d.Send(fmt.Sprintf("Exit %s created as #%d.", exitName, exitRef))
}

func cmdDescribe(g *Game, d *Descriptor, args string, _ []string) {
	// @desc obj=text
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("@describe: Usage: @desc thing = description")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	desc := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	g.SetAttr(target, 6, desc) // A_DESC = 6
	d.Send("Set.")
}

func cmdRename(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("@name: Usage: @name thing = new name")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	newName := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		oldName := obj.Name
		obj.Name = newName
		g.PersistObject(obj)
		if obj.ObjType() == gamedb.TypePlayer && g.Store != nil {
			g.Store.UpdatePlayerIndex(obj, oldName)
		}
		d.Send("Name set.")
	}
}

// --- Eval ---

func cmdEval(g *Game, d *Descriptor, args string, _ []string) {
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	result := ctx.Exec(args, eval.EvFCheck|eval.EvEval, nil)
	d.Send(result)
}

// --- Session ---

func cmdQuit(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Texts != nil {
		if txt := g.Texts.GetQuit(); txt != "" {
			d.SendNoNewline(txt)
		} else {
			d.Send("Going home.")
		}
	} else {
		d.Send("Going home.")
	}
	g.DisconnectPlayer(d)
}

func cmdReadCache(g *Game, d *Descriptor, _ string, _ []string) {
	// Wizard-only command
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if g.TextDir == "" {
		d.Send("No text directory configured (-textdir flag).")
		return
	}
	count := g.ReloadTextFiles()
	d.Send(fmt.Sprintf("Text file cache reloaded. %d file(s) loaded from %s.", count, g.TextDir))
}

func cmdSetDoing(g *Game, d *Descriptor, args string, _ []string) {
	d.DoingStr = args
	d.Send("Set.")
}

// --- Game Helper Methods ---

// Game holds the complete game state.
type Game struct {
	DB          *gamedb.Database
	Conns       *ConnManager
	Commands    map[string]*Command
	Queue       *CommandQueue
	NextRef     gamedb.DBRef
	DBPath      string           // Path for saving the database
	Store       *boltstore.Store // nil = no bbolt persistence
	Texts       *TextFiles       // Cached text files (connect.txt, motd.txt, etc.)
	TextDir     string           // Path to text files directory (for @readcache)
	Comsys      *Comsys          // Channel/communication system (nil if disabled)
	Mail        *Mail            // Built-in mail system (nil if disabled)
	Conf        *GameConf        // Game configuration from conf file
	FuncAliases map[string]string // Function aliases (alias -> target, uppercase)
	BadNames    []string          // Forbidden player names from alias config
	HelpMain    *HelpFile         // help.txt
	HelpQuick   *HelpFile         // qhelp.txt
	HelpWiz     *HelpFile         // wizhelp.txt
	HelpNews    *HelpFile         // news.txt
	HelpPlus    *HelpFile         // plushelp.txt
	HelpMan     *HelpFile         // mushman.txt
	HelpWizNews *HelpFile         // wiznews.txt
	HelpJobs    *HelpFile         // jhelp.txt
	MOTD        string            // Message of the day (settable by wizards)
	WizMOTD     string            // Wizard MOTD (@motd/wizard)
	DownMOTD    string            // Down MOTD (@motd/down)
	FullMOTD    string            // Full MOTD (@motd/full)
	Spell       *SpellChecker     // Spellcheck engine (nil if disabled)
	SQLDB       *SQLStore         // SQLite3 database (nil if disabled)
	GameFuncs   map[string]*eval.UFunction // @function-defined functions (uppercase name -> def)
	ConfPath    string   // Path to game config file (for archive)
	DictDir     string   // Path to dictionary directory (for archive)
	AliasConfs  []string // Paths to alias config files (for archive)
	ArchiveDir  string   // Path to archive output directory
	EventBus    *events.Bus // Structured event bus for multi-transport output
	Guests      *GuestManager // Guest player tracking and cleanup
	objExecDepth int // Recursion depth counter for ExecuteAsObject
	objExecCount map[gamedb.DBRef]int // Per-object execution counter for rate limiting
	objExecCountReset time.Time // When the counter was last reset
}

// Emit sends an event to the player specified in ev.Player via the event bus.
func (g *Game) Emit(ev events.Event) {
	g.EventBus.Emit(ev)
}

// EmitRoom sends an event to all players in a room via the event bus.
func (g *Game) EmitRoom(room gamedb.DBRef, ev events.Event) {
	g.EventBus.EmitToRoom(g.DB, room, ev)
}

// EmitRoomExcept sends an event to all players in a room except one.
func (g *Game) EmitRoomExcept(room gamedb.DBRef, except gamedb.DBRef, ev events.Event) {
	g.EventBus.EmitToRoomExcept(g.DB, room, except, ev)
}

// PersistObject writes a single object to the bolt store (no-op if Store is nil).
func (g *Game) PersistObject(obj *gamedb.Object) {
	if g.Store == nil || obj == nil {
		return
	}
	if err := g.Store.PutObject(obj); err != nil {
		log.Printf("ERROR: persist object #%d: %v", obj.DBRef, err)
	}
}

// PersistObjects writes multiple objects to the bolt store in one transaction.
func (g *Game) PersistObjects(objs ...*gamedb.Object) {
	if g.Store == nil {
		return
	}
	if err := g.Store.PutObjects(objs...); err != nil {
		log.Printf("ERROR: persist objects: %v", err)
	}
}

// NewGame creates a new Game instance.
func NewGame(db *gamedb.Database) *Game {
	// Find the next available dbref and clear stale CONNECTED flags
	maxRef := gamedb.DBRef(0)
	for ref, obj := range db.Objects {
		if ref > maxRef {
			maxRef = ref
		}
		// Clear CONNECTED flag — nobody is connected at startup.
		// The flatfile may have this baked in from when the dump was taken.
		if obj.Flags[1]&gamedb.Flag2Connected != 0 {
			obj.Flags[1] &^= gamedb.Flag2Connected
		}
	}
	bus := events.NewBus()
	cm := NewConnManager()
	cm.EventBus = bus
	return &Game{
		DB:        db,
		Conns:     cm,
		Commands:  InitCommands(),
		Queue:     NewCommandQueue(),
		NextRef:   maxRef + 1,
		GameFuncs: make(map[string]*eval.UFunction),
		EventBus:  bus,
		Guests:    NewGuestManager(),
	}
}

// stringMatchWord implements C TinyMUSH's string_match: checks if sub is a prefix
// of any word in src (words separated by non-alphanumeric characters).
// Both src and sub should already be lowercased.
func stringMatchWord(src, sub string) bool {
	if sub == "" || src == "" {
		return false
	}
	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], sub) {
			return true
		}
		for i < len(src) && isAlnumByte(src[i]) {
			i++
		}
		for i < len(src) && !isAlnumByte(src[i]) {
			i++
		}
	}
	return false
}

func isAlnumByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// DisplayName returns the display name of an object (before the first semicolon).
// In TinyMUSH, object names can contain semicolon-separated aliases
// (e.g., "Crystal Tuner;tuner;ct") — only the first part is the display name.
func DisplayName(name string) string {
	if idx := strings.IndexByte(name, ';'); idx >= 0 {
		return name[:idx]
	}
	return name
}

// PlayerName returns the name of a player.
func (g *Game) PlayerName(player gamedb.DBRef) string {
	if obj, ok := g.DB.Objects[player]; ok {
		return DisplayName(obj.Name)
	}
	return "Unknown"
}

// PlayerLocation returns the location of a player.
func (g *Game) PlayerLocation(player gamedb.DBRef) gamedb.DBRef {
	if obj, ok := g.DB.Objects[player]; ok {
		return obj.Location
	}
	return gamedb.Nothing
}

// MovePlayer moves a player to a new location.
func (g *Game) MovePlayer(d *Descriptor, dest gamedb.DBRef) {
	player := d.Player
	playerObj, ok := g.DB.Objects[player]
	if !ok {
		return
	}

	oldLoc := playerObj.Location

	// Source room: ALEAVE action (52), OLEAVE to room (51)
	if oldLoc != gamedb.Nothing {
		g.QueueAttrAction(oldLoc, player, 52, nil) // ALEAVE
		if oleave := g.GetAttrText(oldLoc, 51); oleave != "" {
			ctx := MakeEvalContextForObj(g, oldLoc, player, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			msg := ctx.Exec(oleave, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			if msg != "" {
				g.Conns.SendToRoomExcept(g.DB, oldLoc, player, msg)
			}
		} else {
			g.Conns.SendToRoomExcept(g.DB, oldLoc, player,
				fmt.Sprintf("%s has left.", DisplayName(playerObj.Name)))
		}
		g.RemoveFromContents(oldLoc, player)
	}

	// Set new location
	playerObj.Location = dest

	// Add to new location's contents chain
	g.AddToContents(dest, player)

	// Announce arrival (default, before ShowRoom evaluates OSUCC)
	g.Conns.SendToRoomExcept(g.DB, dest, player,
		fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))

	// Persist moved player and affected rooms
	persistList := []*gamedb.Object{playerObj}
	if oldLoc != gamedb.Nothing {
		if oldLocObj, ok := g.DB.Objects[oldLoc]; ok {
			persistList = append(persistList, oldLocObj)
		}
	}
	if destObj, ok := g.DB.Objects[dest]; ok {
		persistList = append(persistList, destObj)
	}
	g.PersistObjects(persistList...)

	// Show the room to the player (DESC + SUCC + CONFORMAT/EXITFORMAT)
	// ShowRoom handles SUCC/OSUCC/ASUCC display via the lock-check path.
	g.ShowRoom(d, dest)

	// Dest room: AENTER action (35), OENTER to room (53)
	g.QueueAttrAction(dest, player, 35, nil) // AENTER
	if oenter := g.GetAttrText(dest, 53); oenter != "" {
		ctx := MakeEvalContextForObj(g, dest, player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		msg := ctx.Exec(oenter, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if msg != "" {
			g.Conns.SendToRoomExcept(g.DB, dest, player, msg)
		}
	}

	// Notify listeners on arrival
	g.MatchListenPatterns(dest, player,
		fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))
}

// RemoveFromContents removes an object from a location's contents chain.
func (g *Game) RemoveFromContents(loc gamedb.DBRef, obj gamedb.DBRef) {
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return
	}
	if locObj.Contents == obj {
		if o, ok := g.DB.Objects[obj]; ok {
			locObj.Contents = o.Next
			o.Next = gamedb.Nothing
		}
		return
	}
	prev := locObj.Contents
	seen := make(map[gamedb.DBRef]bool)
	for prev != gamedb.Nothing && !seen[prev] {
		seen[prev] = true
		prevObj, ok := g.DB.Objects[prev]
		if !ok {
			break
		}
		if prevObj.Next == obj {
			if o, ok := g.DB.Objects[obj]; ok {
				prevObj.Next = o.Next
				o.Next = gamedb.Nothing
			}
			return
		}
		prev = prevObj.Next
	}
}

// AddToContents adds obj to dest's contents chain safely.
// Like C TinyMUSH's move_object, it ensures no cycles by checking
// if the object is already in the chain before inserting.
func (g *Game) AddToContents(dest, obj gamedb.DBRef) {
	destObj, ok := g.DB.Objects[dest]
	if !ok {
		return
	}
	o, ok := g.DB.Objects[obj]
	if !ok {
		return
	}
	// Check if obj is already in this contents chain — prevent cycles
	next := destObj.Contents
	for next != gamedb.Nothing {
		if next == obj {
			return // already in chain
		}
		if nObj, ok := g.DB.Objects[next]; ok {
			next = nObj.Next
		} else {
			break
		}
	}
	o.Next = destObj.Contents
	destObj.Contents = obj
}

// ShowRoom displays a room to a player.
func (g *Game) ShowRoom(d *Descriptor, room gamedb.DBRef) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		d.Send("You see nothing special.")
		return
	}

	makeCtx := func() *eval.EvalContext {
		return MakeEvalContextForObj(g, room, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
	}

	// Room name — use NAMEFORMAT (222) if set, otherwise plain name
	nameFmt := g.GetAttrText(room, 222) // A_NAME_FMT
	if nameFmt != "" {
		ctx := makeCtx()
		d.Send(ctx.Exec(nameFmt, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	} else {
		d.Send(DisplayName(roomObj.Name))
	}

	// Description — executor is the room (so v() resolves room attrs), enactor is the player
	desc := g.GetAttrText(room, 6) // A_DESC = 6
	if desc != "" {
		ctx := makeCtx()
		evaluated := ctx.Exec(desc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		d.Send(evaluated)
	}

	// C TinyMUSH's look_in shows SUCC/FAIL after DESC, conditional on A_LOCK.
	// For rooms, if the player passes the lock → show SUCC (4), OSUCC (1), ASUCC (12).
	// If the player fails the lock → show FAIL (3), OFAIL (2), AFAIL (13).
	// Many rooms use SUCC for content/exit display (modal rooms, custom formatting).
	// When SUCC provides non-empty output, it typically includes Players/Contents/Exits,
	// so we skip the default CONFORMAT/EXITFORMAT fallback to avoid duplication.
	succShown := false
	if roomObj.ObjType() == gamedb.TypeRoom {
		if CouldDoIt(g, d.Player, room, aLock) {
			if succ := g.GetAttrText(room, 4); succ != "" { // A_SUCC
				ctx := makeCtx()
				msg := ctx.Exec(succ, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
				if msg != "" {
					d.Send(msg)
					succShown = true
				}
			}
			if osucc := g.GetAttrText(room, 1); osucc != "" { // A_OSUCC
				ctx := makeCtx()
				msg := ctx.Exec(osucc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
				if msg != "" {
					g.Conns.SendToRoomExcept(g.DB, room, d.Player, msg)
				}
			}
			g.QueueAttrAction(room, d.Player, 12, nil) // A_ASUCC
		} else {
			HandleLockFailure(g, d, room, aFail, aOFail, aAFail, "")
		}
	}

	// Build list of visible content dbrefs (excluding the looking player)
	var contentRefs []gamedb.DBRef
	for _, next := range g.DB.SafeContents(room) {
		obj, ok := g.DB.Objects[next]
		if !ok {
			continue
		}
		if next != d.Player && !obj.IsGoing() {
			visible := false
			if obj.ObjType() == gamedb.TypePlayer {
				if g.Conns.IsConnected(next) {
					if obj.HasFlag(gamedb.FlagDark) && !SeeAll(g, d.Player) && !Controls(g, d.Player, next) {
						// DARK player hidden
					} else {
						visible = true
					}
				}
			} else if obj.ObjType() == gamedb.TypeThing {
				if !obj.HasFlag(gamedb.FlagDark) || SeeAll(g, d.Player) || Controls(g, d.Player, next) {
					visible = true
				}
			}
			if visible {
				contentRefs = append(contentRefs, next)
			}
		}
		next = obj.Next
	}

	// Contents — use CONFORMAT (214) if set, otherwise default "Contents:" list
	conFmt := g.GetAttrText(room, 214) // A_LCON_FMT
	conFmtHandled := false
	if conFmt != "" {
		// Build space-separated dbref list for %0
		var refStrs []string
		for _, ref := range contentRefs {
			refStrs = append(refStrs, fmt.Sprintf("#%d", ref))
		}
		ctx := makeCtx()
		result := ctx.Exec(conFmt, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{strings.Join(refStrs, " ")})
		if result != "" {
			d.Send(result)
			conFmtHandled = true
		}
	}
	if !succShown && !conFmtHandled && len(contentRefs) > 0 {
		d.Send("Contents:")
		for _, ref := range contentRefs {
			if obj, ok := g.DB.Objects[ref]; ok {
				d.Send("  " + DisplayName(obj.Name))
			}
		}
	}

	// Build list of visible exit dbrefs
	// TinyMUSH Can_See_Exit(p,x,l): !Darkened(p,x) && (!(l) || Light(x))
	// DARK exits are ALWAYS hidden (even from wizards) — no SeeAll bypass.
	// In a DARK room, only LIGHT exits are visible.
	roomIsDark := roomObj.HasFlag(gamedb.FlagDark)
	var exitRefs []gamedb.DBRef
	exitRef := roomObj.Exits
	for exitRef != gamedb.Nothing {
		exitObj, ok := g.DB.Objects[exitRef]
		if !ok {
			break
		}
		canSee := true
		if exitObj.HasFlag(gamedb.FlagDark) {
			// DARK exits are always hidden (Can_See_Exit: !Darkened)
			canSee = false
		} else if roomIsDark && !exitObj.HasFlag2(gamedb.Flag2Light) {
			// In a DARK room, only LIGHT exits are visible
			canSee = false
		}
		if canSee {
			exitRefs = append(exitRefs, exitRef)
		}
		exitRef = exitObj.Next
	}

	// Exits — use EXITFORMAT (215) if set, otherwise default "Obvious exits:" list
	exitFmt := g.GetAttrText(room, 215) // A_LEXITS_FMT
	exitFmtHandled := false
	if exitFmt != "" {
		var refStrs []string
		for _, ref := range exitRefs {
			refStrs = append(refStrs, fmt.Sprintf("#%d", ref))
		}
		ctx := makeCtx()
		result := ctx.Exec(exitFmt, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{strings.Join(refStrs, " ")})
		if result != "" {
			d.Send(result)
			exitFmtHandled = true
		}
	}
	if !succShown && !exitFmtHandled && len(exitRefs) > 0 {
		d.Send("Obvious exits:")
		var exitNames []string
		for _, ref := range exitRefs {
			if exitObj, ok := g.DB.Objects[ref]; ok {
				name := exitObj.Name
				if idx := strings.IndexByte(name, ';'); idx >= 0 {
					name = name[:idx]
				}
				exitNames = append(exitNames, name)
			}
		}
		d.Send("  " + strings.Join(exitNames, "  "))
	}

	// ADESC (36) — action list executed on the room when looked at
	g.QueueAttrAction(room, d.Player, 36, nil) // A_ADESC
}

// ShowObject displays an object to a player.
// Implements the C TinyMUSH did_it pattern: DESC to player, ODESC to room, ADESC action.
func (g *Game) ShowObject(d *Descriptor, target gamedb.DBRef) {
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("I don't see that here.")
		return
	}
	d.Send(DisplayName(obj.Name))

	// DESC (6) — description shown to the looker
	desc := g.GetAttrText(target, 6) // A_DESC
	if desc != "" {
		ctx := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		d.Send(ctx.Exec(desc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	} else {
		d.Send("You see nothing special.")
	}

	// ODESC (37) — message shown to others in the room
	odesc := g.GetAttrText(target, 37) // A_ODESC
	if odesc != "" {
		ctx := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		msg := ctx.Exec(odesc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		loc := g.PlayerLocation(d.Player)
		g.Conns.SendToRoomExcept(g.DB, loc, d.Player, msg)
	}

	// ADESC (36) — action list executed on the target object
	g.QueueAttrAction(target, d.Player, 36, nil) // A_ADESC
}

// ShowExamine shows detailed object info (wizard/owner command).
func (g *Game) ShowExamine(d *Descriptor, target gamedb.DBRef) {
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("I don't see that here.")
		return
	}
	d.Send(fmt.Sprintf("%s(#%d%s)", obj.Name, target, flagString(obj)))
	d.Send(fmt.Sprintf("Type: %s  Flags: %s  Owner: %s(#%d)",
		obj.ObjType().String(), flagString(obj), g.PlayerName(obj.Owner), obj.Owner))
	if obj.ObjType() == gamedb.TypeExit {
		// For exits: Location = destination, Exits = source room
		if obj.Location != gamedb.Nothing {
			d.Send(fmt.Sprintf("Destination: %s(#%d)", g.ObjName(obj.Location), obj.Location))
		}
		if obj.Exits != gamedb.Nothing {
			d.Send(fmt.Sprintf("Source: %s(#%d)", g.ObjName(obj.Exits), obj.Exits))
		}
	} else {
		if obj.Location != gamedb.Nothing {
			d.Send(fmt.Sprintf("Location: %s(#%d)", g.ObjName(obj.Location), obj.Location))
		}
		if obj.Link != gamedb.Nothing && obj.Link != gamedb.DBRef(-3) {
			d.Send(fmt.Sprintf("Home: %s(#%d)", g.ObjName(obj.Link), obj.Link))
		}
	}
	if obj.Zone != gamedb.Nothing {
		d.Send(fmt.Sprintf("Zone: %s(#%d)", g.ObjName(obj.Zone), obj.Zone))
	}
	if obj.Parent != gamedb.Nothing {
		d.Send(fmt.Sprintf("Parent: %s(#%d)", g.ObjName(obj.Parent), obj.Parent))
	}

	// Show default lock ("Key:") if set
	lockText := g.GetAttrText(target, aLock)
	if lockText != "" {
		d.Send(fmt.Sprintf("Key: %s", lockText))
	} else if obj.Lock != nil {
		d.Send(fmt.Sprintf("Key: %s", UnparseBoolExp(g, obj.Lock)))
	}

	// Check per-player TRUNC_LENGTH for attribute display truncation
	truncLen := 0
	if ts := g.GetAttrTextByName(d.Player, "TRUNC_LENGTH"); ts != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(ts)); err == nil && n > 0 {
			truncLen = n
		}
	}

	// Show attributes with permission checks
	for _, attr := range obj.Attrs {
		info := ParseAttrInfo(attr.Value)
		def := g.LookupAttrDef(attr.Number)
		// Use CanReadAttr for proper permission enforcement (replaces isInternalAttr)
		if !CanReadAttr(g, d.Player, target, def, info.Flags, info.Owner) {
			continue
		}
		name := g.DB.GetAttrName(attr.Number)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", attr.Number)
		}
		text := eval.StripAttrPrefix(attr.Value)
		if truncLen > 0 && len(text) > truncLen {
			text = text[:truncLen] + "..."
		}
		// C TinyMUSH: only show annotation if player controls object or owns attr
		showAnnotation := Controls(g, d.Player, target) || info.Owner == d.Player
		annotation := ""
		if showAnnotation {
			annotation = attrAnnotation(g, d.Player, info, def)
		}
		if annotation != "" {
			d.Send(fmt.Sprintf("  %s %s: %s", name, annotation, text))
		} else {
			d.Send(fmt.Sprintf("  %s: %s", name, text))
		}
	}

	// Contents section
	examContents := g.DB.SafeContents(target)
	if len(examContents) > 0 {
		d.Send("Contents:")
		for _, cRef := range examContents {
			if cObj, ok := g.DB.Objects[cRef]; ok {
				d.Send(fmt.Sprintf("%s(#%d%s)", cObj.Name, cRef, flagString(cObj)))
			}
		}
	}

	// Exits section (only for rooms — for exits, Exits field is the source room, already shown above)
	if obj.ObjType() != gamedb.TypeExit && obj.Exits != gamedb.Nothing {
		d.Send("Exits:")
		seenEx := make(map[gamedb.DBRef]bool)
		exitRef := obj.Exits
		for exitRef != gamedb.Nothing && !seenEx[exitRef] {
			seenEx[exitRef] = true
			if eObj, ok := g.DB.Objects[exitRef]; ok {
				d.Send(fmt.Sprintf("%s(#%d%s)", eObj.Name, exitRef, flagString(eObj)))
				exitRef = eObj.Next
			} else {
				break
			}
		}
	}
}

// attrAnnotation builds a TinyMUSH-style annotation string for an attribute.
// C TinyMUSH's view_atr shows: [#owner instance_flags(def_flags)]
// Per-instance flags (aflags) and definition flags (ap->flags) are shown
// separately: instance flags directly, definition flags in parentheses.
func attrAnnotation(g *Game, player gamedb.DBRef, info AttrInfo, def *gamedb.AttrDef) string {
	var parts []string
	// Show owner if different from object owner (non-default)
	if info.Owner != gamedb.Nothing && info.Owner != gamedb.DBRef(0) {
		parts = append(parts, fmt.Sprintf("#%d", info.Owner))
	}

	// Per-instance flags (from the attribute value's \x01 header)
	instStr := attrFlagString(info.Flags)
	// Definition flags (from the AttrDef loaded from flatfile)
	defStr := ""
	if def != nil {
		defStr = attrFlagString(def.Flags)
	}

	// Format: "inst(def)", "(def)", or "inst"
	var flagPart string
	if instStr != "" && defStr != "" {
		flagPart = instStr + "(" + defStr + ")"
	} else if defStr != "" {
		flagPart = "(" + defStr + ")"
	} else if instStr != "" {
		flagPart = instStr
	}
	if flagPart != "" {
		parts = append(parts, flagPart)
	}

	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// attrFlagString converts attribute flags to a compact display string.
// Letter mappings match C TinyMUSH's view_atr exactly.
func attrFlagString(flags int) string {
	var buf strings.Builder
	if flags&gamedb.AFLock != 0 {
		buf.WriteByte('+')
	}
	if flags&gamedb.AFNoProg != 0 {
		buf.WriteByte('$')
	}
	if flags&gamedb.AFCase != 0 {
		buf.WriteByte('C')
	}
	if flags&gamedb.AFDefault != 0 {
		buf.WriteByte('D')
	}
	if flags&gamedb.AFHTML != 0 {
		buf.WriteByte('H')
	}
	if flags&gamedb.AFPrivate != 0 {
		buf.WriteByte('I')
	}
	if flags&gamedb.AFRMatch != 0 {
		buf.WriteByte('M')
	}
	if flags&gamedb.AFNoName != 0 {
		buf.WriteByte('N')
	}
	if flags&gamedb.AFNoParse != 0 {
		buf.WriteByte('P')
	}
	if flags&gamedb.AFNow != 0 {
		buf.WriteByte('Q')
	}
	if flags&gamedb.AFRegexp != 0 {
		buf.WriteByte('R')
	}
	if flags&gamedb.AFStructure != 0 {
		buf.WriteByte('S')
	}
	if flags&gamedb.AFTrace != 0 {
		buf.WriteByte('T')
	}
	if flags&gamedb.AFVisual != 0 {
		buf.WriteByte('V')
	}
	if flags&gamedb.AFNoClone != 0 {
		buf.WriteByte('c')
	}
	if flags&gamedb.AFDark != 0 {
		buf.WriteByte('d')
	}
	if flags&gamedb.AFGod != 0 {
		buf.WriteByte('g')
	}
	if flags&gamedb.AFConst != 0 {
		buf.WriteByte('k')
	}
	if flags&gamedb.AFMDark != 0 {
		buf.WriteByte('m')
	}
	if flags&gamedb.AFWizard != 0 {
		buf.WriteByte('w')
	}
	if flags&gamedb.AFPropagate != 0 {
		buf.WriteByte('p')
	}
	return buf.String()
}

func typeChar(t gamedb.ObjectType) string {
	switch t {
	case gamedb.TypeRoom:
		return "R"
	case gamedb.TypeExit:
		return "E"
	case gamedb.TypePlayer:
		return "P"
	case gamedb.TypeThing:
		return ""
	default:
		return ""
	}
}

// flagLetters maps flag word/bit pairs to their TinyMUSH display character.
// Ordered to produce consistent output matching TinyMUSH examine.
var flagLetters = []struct {
	Word   int
	Bit    int
	Letter byte
}{
	// Flag word 0 — uppercase letters
	{0, gamedb.FlagDark, 'D'},
	{0, gamedb.FlagHaven, 'H'},
	{0, gamedb.FlagInherit, 'I'},
	{0, gamedb.FlagJumpOK, 'J'},
	{0, gamedb.FlagLinkOK, 'L'},
	{0, gamedb.FlagMonitor, 'M'},
	{0, gamedb.FlagNoSpoof, 'N'},
	{0, gamedb.FlagOpaque, 'O'},
	{0, gamedb.FlagQuiet, 'Q'},
	{0, gamedb.FlagSticky, 'S'},
	{0, gamedb.FlagTrace, 'T'},
	{0, gamedb.FlagVisual, 'V'},
	{0, gamedb.FlagWizard, 'W'},
	{0, gamedb.FlagRoyalty, 'Z'},
	{0, gamedb.FlagVerbose, 'v'},
	{0, gamedb.FlagGoing, 'G'},
	{0, gamedb.FlagChownOK, 'C'},
	{0, gamedb.FlagEnterOK, 'e'},
	{0, gamedb.FlagImmortal, 'i'},
	{0, gamedb.FlagMyopic, 'm'},
	{0, gamedb.FlagPuppet, 'p'},
	{0, gamedb.FlagRobot, 'r'},
	{0, gamedb.FlagSafe, 's'},
	{0, gamedb.FlagHalt, 'h'},
	{0, gamedb.FlagDestroyOK, 'd'},
	{0, gamedb.FlagSeeThru, 't'},
	{0, gamedb.FlagHearThru, 'a'},
	{0, gamedb.FlagHasStartup, '='},
	// Flag word 1 — lowercase letters and symbols
	{1, gamedb.Flag2Abode, 'A'},
	{1, gamedb.Flag2Unfindable, 'U'},
	{1, gamedb.Flag2ParentOK, 'Y'},
	{1, gamedb.Flag2Light, 'l'},
	{1, gamedb.Flag2Connected, 'c'},
	{1, gamedb.Flag2Slave, 'x'},
	{1, gamedb.Flag2Ansi, 'X'},
	{1, gamedb.Flag2Bounce, 'b'},
	{1, gamedb.Flag2ControlOK, 'z'},
	{1, gamedb.Flag2StopMatch, '!'},
	{1, gamedb.Flag2NoBLeed, '-'},
	{1, gamedb.Flag2Gagged, 'j'},
	{1, gamedb.Flag2Fixed, 'f'},
	{1, gamedb.Flag2Staff, 'w'},
	{1, gamedb.Flag2Watcher, '+'},
	{1, gamedb.Flag2HasCommands, '$'},
	{1, gamedb.Flag2HasDaily, '*'},
	{1, gamedb.Flag2HasListen, '@'},
	{1, gamedb.Flag2HTML, '~'},
	{1, gamedb.Flag2ZoneParent, 'o'},
	{1, gamedb.Flag2Blind, 'B'},
	{1, gamedb.Flag2Floating, 'F'},
}

func flagString(obj *gamedb.Object) string {
	var buf strings.Builder
	switch obj.ObjType() {
	case gamedb.TypeRoom:
		buf.WriteByte('R')
	case gamedb.TypeExit:
		buf.WriteByte('E')
	case gamedb.TypePlayer:
		buf.WriteByte('P')
	}
	for _, fl := range flagLetters {
		if fl.Word == 0 && obj.HasFlag(fl.Bit) {
			buf.WriteByte(fl.Letter)
		} else if fl.Word == 1 && obj.HasFlag2(fl.Bit) {
			buf.WriteByte(fl.Letter)
		}
	}
	return buf.String()
}

// isInternalAttr returns true for attributes that should never be shown
// (equivalent to TinyMUSH's AF_INTERNAL flag).
func isInternalAttr(attrNum int) bool {
	switch attrNum {
	case 5: // A_PASS — password hash (AF_DARK|AF_INTERNAL)
		return true
	case 200: // A_LASTPAGE — last page recipient (AF_INTERNAL)
		return true
	case 205, 206, 207: // A_MAILTO, A_MAILMSG, A_MAILSUB (AF_INTERNAL)
		return true
	case 210: // A_PROGCMD — @program command (AF_INTERNAL)
		return true
	case 230: // A_PAGEGROUP — page group (AF_INTERNAL)
		return true
	case 253: // A_LIST — internal attr list (AF_INTERNAL)
		return true
	case 255: // A_TEMP — internal temp (AF_INTERNAL)
		return true
	}
	return false
}

// AttrInfo holds parsed owner and flags from an attribute's raw value prefix.
type AttrInfo struct {
	Owner gamedb.DBRef
	Flags int
}

// ParseAttrInfo extracts owner and flags from "\x01owner:flags:text" format.
// Returns zero values if no prefix or malformed.
func ParseAttrInfo(raw string) AttrInfo {
	if len(raw) == 0 || raw[0] != '\x01' {
		return AttrInfo{Owner: gamedb.Nothing, Flags: 0}
	}
	colonCount := 0
	start := 1
	var ownerStr, flagsStr string
	for i := 1; i < len(raw); i++ {
		if raw[i] == ':' {
			colonCount++
			if colonCount == 1 {
				ownerStr = raw[start:i]
				start = i + 1
			}
			if colonCount == 2 {
				flagsStr = raw[start:i]
				break
			}
		}
	}
	owner := toIntSimple(ownerStr)
	flags := toIntSimple(flagsStr)
	return AttrInfo{Owner: gamedb.DBRef(owner), Flags: flags}
}

// LookupAttrNum resolves an attribute name to its number. Returns -1 if not found.
func (g *Game) LookupAttrNum(name string) int {
	name = strings.ToUpper(name)
	// Check user-defined attrs
	if def, ok := g.DB.AttrByName[name]; ok {
		return def.Number
	}
	// Check well-known attrs
	for num, n := range gamedb.WellKnownAttrs {
		if strings.EqualFold(n, name) {
			return num
		}
	}
	return -1
}

// LookupAttrDef returns the AttrDef for an attribute number, or nil if none.
// For well-known attrs without explicit AttrDef entries, synthesizes one from
// WellKnownAttrFlags so that built-in flag checks (AF_INTERNAL etc.) work.
func (g *Game) LookupAttrDef(attrNum int) *gamedb.AttrDef {
	if def, ok := g.DB.AttrNames[attrNum]; ok {
		return def
	}
	// Fall back to well-known attr flags
	if flags, ok := gamedb.WellKnownAttrFlags[attrNum]; ok {
		name := gamedb.WellKnownAttrs[attrNum]
		return &gamedb.AttrDef{Number: attrNum, Name: name, Flags: flags}
	}
	return nil
}

// ShowWho displays the WHO list.
func (g *Game) ShowWho(d *Descriptor) {
	isWiz := Wizard(g, d.Player)

	now := time.Now()

	// Header — matches C TinyMUSH dump_users() format
	if isWiz {
		d.Send("Player Name        On For Idle   Room    Cmds   Host")
	} else {
		d.Send(fmt.Sprintf("%-16s%9s %4s  %s", "Player Name", "On For", "Idle", "Doing"))
	}

	type whoEntry struct {
		name  string
		onFor string
		idle  string
		doing string
		loc   gamedb.DBRef
		cmds  int
		host  string
	}
	var entries []whoEntry

	descs := g.Conns.AllDescriptors()
	for _, dd := range descs {
		if dd.State != ConnConnected {
			continue
		}
		name := g.PlayerName(dd.Player)
		onFor := FormatConnTime(now.Sub(dd.ConnTime))
		idle := FormatIdleTime(now.Sub(dd.LastCmd))
		// Extract host/IP (strip port)
		host := dd.Addr
		if idx := strings.LastIndex(host, ":"); idx >= 0 {
			host = host[:idx]
		}
		loc := g.PlayerLocation(dd.Player)
		entries = append(entries, whoEntry{name, onFor, idle, dd.DoingStr, loc, dd.CmdCount, host})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	for _, e := range entries {
		if isWiz {
			// C format: "%-16s%9s %4s%-3s#%-6d%5d%3s%-25s"
			// We skip player flags (%-3s) and site flags (%3s) for now
			d.Send(fmt.Sprintf("%-16s%9s %4s   #%-6d%5d   %-25s",
				e.name, e.onFor, e.idle, e.loc, e.cmds, e.host))
		} else {
			// C format: "%-16s%9s %4s  %s"
			d.Send(fmt.Sprintf("%-16s%9s %4s  %s", e.name, e.onFor, e.idle, e.doing))
		}
	}

	record := len(entries)
	d.Send(fmt.Sprintf("%d Players logged in, %d record, no maximum.", record, record))
}

// MatchObject resolves a name to a dbref, searching contents and location.
func (g *Game) MatchObject(player gamedb.DBRef, name string) gamedb.DBRef {
	name = strings.TrimSpace(name)
	if name == "" {
		return gamedb.Nothing
	}
	if strings.EqualFold(name, "me") {
		return player
	}
	if strings.EqualFold(name, "here") {
		return g.PlayerLocation(player)
	}
	// Handle #dbref
	if name[0] == '#' {
		n := 0
		for _, ch := range name[1:] {
			if ch >= '0' && ch <= '9' {
				n = n*10 + int(ch-'0')
			} else {
				return gamedb.Nothing
			}
		}
		return gamedb.DBRef(n)
	}
	// Handle *player — global player name lookup
	if name[0] == '*' {
		pName := strings.ToLower(strings.TrimSpace(name[1:]))
		if pName == "" {
			return gamedb.Nothing
		}
		for ref, obj := range g.DB.Objects {
			if obj.ObjType() != gamedb.TypePlayer {
				continue
			}
			// Check name and aliases
			for _, alias := range strings.Split(obj.Name, ";") {
				if strings.EqualFold(strings.TrimSpace(alias), pName) {
					return ref
				}
			}
		}
		return gamedb.Nothing
	}

	playerObj, ok := g.DB.Objects[player]
	if !ok {
		return gamedb.Nothing
	}

	nameLower := strings.ToLower(name)

	// matchAliases checks name and semicolon-separated aliases for exact or prefix match.
	// Returns 2 for exact match, 1 for prefix match, 0 for no match.
	// Uses word-boundary matching: "bath" matches "Radiant Bath" (C TinyMUSH string_match).
	matchAliases := func(objName string) int {
		for _, alias := range strings.Split(objName, ";") {
			alias = strings.TrimSpace(alias)
			aliasLower := strings.ToLower(alias)
			if aliasLower == nameLower {
				return 2 // exact
			}
			if stringMatchWord(aliasLower, nameLower) {
				return 1 // prefix/word match
			}
		}
		return 0
	}

	// searchContents searches a contents list for exact then prefix matches.
	searchContents := func(contents []gamedb.DBRef) gamedb.DBRef {
		var prefixMatch gamedb.DBRef = gamedb.Nothing
		for _, next := range contents {
			obj, ok := g.DB.Objects[next]
			if !ok {
				continue
			}
			switch matchAliases(obj.Name) {
			case 2:
				return next // exact match wins immediately
			case 1:
				if prefixMatch == gamedb.Nothing {
					prefixMatch = next // remember first prefix match
				}
			}
		}
		return prefixMatch
	}

	// Search room contents
	loc := playerObj.Location
	if found := searchContents(g.DB.SafeContents(loc)); found != gamedb.Nothing {
		return found
	}

	// Search player inventory
	if found := searchContents(g.DB.SafeContents(player)); found != gamedb.Nothing {
		return found
	}

	return gamedb.Nothing
}

// ResolveRef resolves a string (name or #dbref) to a DBRef.
func (g *Game) ResolveRef(player gamedb.DBRef, s string) gamedb.DBRef {
	s = strings.TrimSpace(s)
	if s == "" {
		return gamedb.Nothing
	}
	if s[0] == '#' {
		n := 0
		for _, ch := range s[1:] {
			if ch >= '0' && ch <= '9' {
				n = n*10 + int(ch-'0')
			}
		}
		return gamedb.DBRef(n)
	}
	return g.MatchObject(player, s)
}

// ObjName returns the name of an object by dbref.
func (g *Game) ObjName(ref gamedb.DBRef) string {
	if obj, ok := g.DB.Objects[ref]; ok {
		return DisplayName(obj.Name)
	}
	return fmt.Sprintf("#%d", ref)
}

// GetAttrText returns the text of an attribute on an object.
// It checks the object first, then walks the parent chain (like TinyMUSH's atr_pget).
func (g *Game) GetAttrText(obj gamedb.DBRef, attrNum int) string {
	return g.getAttrTextWithParents(obj, attrNum, 10)
}

// getAttrTextWithParents walks the parent chain up to maxDepth levels.
func (g *Game) getAttrTextWithParents(obj gamedb.DBRef, attrNum int, maxDepth int) string {
	current := obj
	for depth := 0; depth <= maxDepth; depth++ {
		o, ok := g.DB.Objects[current]
		if !ok {
			return ""
		}
		for _, attr := range o.Attrs {
			if attr.Number == attrNum {
				return eval.StripAttrPrefix(attr.Value)
			}
		}
		// Walk to parent
		if o.Parent == gamedb.Nothing || o.Parent == current {
			return ""
		}
		current = o.Parent
	}
	return ""
}

// GetAttrTextDirect returns the text of an attribute on an object only (no parent chain).
func (g *Game) GetAttrTextDirect(obj gamedb.DBRef, attrNum int) string {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return ""
	}
	for _, attr := range o.Attrs {
		if attr.Number == attrNum {
			return eval.StripAttrPrefix(attr.Value)
		}
	}
	return ""
}

// SetAttr sets an attribute on an object, preserving existing per-instance flags.
// If the attribute doesn't exist on the object and the attribute definition has
// AF_PROPAGATE, the attribute metadata (owner, per-instance flags) is copied
// from the parent chain before applying the new value (lazy propagation).
func (g *Game) SetAttr(obj gamedb.DBRef, attrNum int, value string) {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return
	}
	owner := fmt.Sprintf("%d", o.Owner)

	for i, attr := range o.Attrs {
		if attr.Number == attrNum {
			existing := ParseAttrInfo(attr.Value)
			fullValue := fmt.Sprintf("\x01%s:%d:%s", owner, existing.Flags, value)
			o.Attrs[i].Value = fullValue
			g.PersistObject(o)
			return
		}
	}

	// Attribute doesn't exist on this object yet.
	// Check for AF_PROPAGATE: if the attr definition has it, copy metadata
	// from the parent chain so per-instance flags and owner are preserved.
	instFlags := 0
	if def := g.LookupAttrDef(attrNum); def != nil && def.Flags&gamedb.AFPropagate != 0 {
		if parentInfo := g.findParentAttr(obj, attrNum); parentInfo != nil {
			instFlags = parentInfo.Flags
			// Use parent attr's owner if set, otherwise use object's owner
			if parentInfo.Owner != gamedb.Nothing && parentInfo.Owner != gamedb.DBRef(0) {
				owner = fmt.Sprintf("%d", parentInfo.Owner)
			}
		}
	}

	fullValue := fmt.Sprintf("\x01%s:%d:%s", owner, instFlags, value)
	o.Attrs = append(o.Attrs, gamedb.Attribute{Number: attrNum, Value: fullValue})
	g.PersistObject(o)
}

// findParentAttr walks the parent chain looking for an attribute.
// Returns the AttrInfo from the first parent that has it, or nil.
func (g *Game) findParentAttr(obj gamedb.DBRef, attrNum int) *AttrInfo {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return nil
	}
	// Walk parent chain (with depth limit to prevent cycles)
	cur := o.Parent
	for depth := 0; depth < 20 && cur != gamedb.Nothing; depth++ {
		pObj, ok := g.DB.Objects[cur]
		if !ok {
			break
		}
		for _, attr := range pObj.Attrs {
			if attr.Number == attrNum {
				info := ParseAttrInfo(attr.Value)
				return &info
			}
		}
		cur = pObj.Parent
	}
	return nil
}

// SetAttrRaw sets an attribute with explicit owner and flags.
func (g *Game) SetAttrRaw(obj gamedb.DBRef, attrNum int, value string, owner gamedb.DBRef, flags int) {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return
	}
	fullValue := fmt.Sprintf("\x01%d:%d:%s", owner, flags, value)
	for i, attr := range o.Attrs {
		if attr.Number == attrNum {
			o.Attrs[i].Value = fullValue
			g.PersistObject(o)
			return
		}
	}
	o.Attrs = append(o.Attrs, gamedb.Attribute{Number: attrNum, Value: fullValue})
	g.PersistObject(o)
}

// SetAttrChecked sets an attribute with permission enforcement.
// Returns true if set, false with error message if denied.
func (g *Game) SetAttrChecked(player, obj gamedb.DBRef, attrNum int, value string) (bool, string) {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false, "No such object."
	}
	// Look up attrdef for master flags
	def := g.LookupAttrDef(attrNum)
	// Find existing instance flags
	instFlags := 0
	for _, attr := range o.Attrs {
		if attr.Number == attrNum {
			info := ParseAttrInfo(attr.Value)
			instFlags = info.Flags
			break
		}
	}
	if !CanSetAttr(g, player, obj, def, instFlags) {
		return false, "Permission denied."
	}
	g.SetAttr(obj, attrNum, value)
	return true, ""
}

// SetAttrByName sets an attribute by name.
func (g *Game) SetAttrByName(obj gamedb.DBRef, attrName string, value string) {
	// Look up in well-known first
	for num, name := range gamedb.WellKnownAttrs {
		if strings.EqualFold(name, attrName) {
			g.SetAttr(obj, num, value)
			return
		}
	}
	// Look up in user-defined
	if def, ok := g.DB.AttrByName[attrName]; ok {
		g.SetAttr(obj, def.Number, value)
		return
	}
	// Create new attr def
	newNum := g.DB.NextAttr
	g.DB.NextAttr++
	g.DB.AddAttrDef(newNum, attrName, 0)
	if g.Store != nil {
		if def, ok := g.DB.AttrNames[newNum]; ok {
			g.Store.PutAttrDef(def)
		}
		g.Store.PutMeta()
	}
	g.SetAttr(obj, newNum, value)
}

// CreateObject creates a new object in the database.
func (g *Game) CreateObject(name string, objType gamedb.ObjectType, owner gamedb.DBRef) gamedb.DBRef {
	ref := g.NextRef
	g.NextRef++

	obj := &gamedb.Object{
		DBRef:    ref,
		Name:     name,
		Location: gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    owner,
		Parent:   gamedb.Nothing,
		Flags:    [3]int{int(objType), 0, 0},
	}
	g.DB.Objects[ref] = obj
	g.PersistObject(obj)
	return ref
}

// CreateExit creates a new exit linking source to dest.
func (g *Game) CreateExit(name string, source, dest, owner gamedb.DBRef) gamedb.DBRef {
	ref := g.CreateObject(name, gamedb.TypeExit, owner)
	exitObj := g.DB.Objects[ref]
	// TinyMUSH exit semantics: Location = destination, Exits = source room
	exitObj.Location = dest
	exitObj.Exits = source

	// Add to source room's exit chain
	if srcObj, ok := g.DB.Objects[source]; ok {
		exitObj.Next = srcObj.Exits
		srcObj.Exits = ref
		g.PersistObjects(exitObj, srcObj)
	}
	return ref
}

// --- Attribute-setting command factory ---

// makeAttrSetter returns a CommandHandler that sets a specific attribute on a target object.
func makeAttrSetter(attrNum int) CommandHandler {
	return func(g *Game, d *Descriptor, args string, _ []string) {
		eqIdx := strings.IndexByte(args, '=')
		if eqIdx < 0 {
			d.Send("I need an object and a value separated by =.")
			return
		}
		targetStr := strings.TrimSpace(args[:eqIdx])
		value := strings.TrimSpace(args[eqIdx+1:])
		target := g.MatchObject(d.Player, targetStr)
		if target == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
		if !g.Controls(d.Player, target) {
			d.Send("Permission denied.")
			return
		}
		ok, errMsg := g.SetAttrChecked(d.Player, target, attrNum, value)
		if !ok {
			d.Send(errMsg)
		} else {
			d.Send("Set.")
		}
	}
}

// --- Player Object Commands ---

func cmdGet(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Get what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("I don't see that here.")
		return
	}
	// Can only pick up THINGs
	if obj.ObjType() != gamedb.TypeThing {
		d.Send("You can't pick that up.")
		return
	}
	// Must be in the same room
	loc := g.PlayerLocation(d.Player)
	if obj.Location != loc {
		d.Send("You can't pick that up.")
		return
	}
	// Check lock
	if !CouldDoIt(g, d.Player, target, aLock) {
		HandleLockFailure(g, d, target, aFail, aOFail, aAFail, "You can't pick that up.")
		return
	}

	// Remove from room contents, add to player inventory
	g.RemoveFromContents(loc, target)
	obj.Location = d.Player
	g.AddToContents(d.Player, target)
	playerObj := g.DB.Objects[d.Player]
	g.PersistObjects(obj, playerObj)

	d.Send(fmt.Sprintf("You pick up %s.", DisplayName(obj.Name)))
	g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
		fmt.Sprintf("%s picks up %s.", g.PlayerName(d.Player), DisplayName(obj.Name)))

	// Fire ASUCC if present
	g.QueueAttrAction(target, d.Player, 12, nil) // A_ASUCC = 12
}

func cmdDrop(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Drop what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("I don't see that here.")
		return
	}
	// Must be in player's inventory
	if obj.Location != d.Player {
		d.Send("You aren't carrying that.")
		return
	}

	// Remove from inventory, add to room contents
	g.RemoveFromContents(d.Player, target)
	loc := g.PlayerLocation(d.Player)
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return
	}
	obj.Location = loc
	g.AddToContents(loc, target)
	g.PersistObjects(obj, locObj)

	d.Send(fmt.Sprintf("You drop %s.", DisplayName(obj.Name)))
	g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
		fmt.Sprintf("%s drops %s.", g.PlayerName(d.Player), DisplayName(obj.Name)))

	// Fire ADROP if present
	g.QueueAttrAction(target, d.Player, 14, nil) // A_ADROP = 14
}

func cmdGive(g *Game, d *Descriptor, args string, _ []string) {
	// give player = amount or give player = object
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Give what to whom?")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	whatStr := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("I don't see that here.")
		return
	}

	// Try as penny amount first
	amount := toIntSimple(whatStr)
	if amount > 0 {
		playerObj := g.DB.Objects[d.Player]
		if playerObj.Pennies < amount {
			d.Send(fmt.Sprintf("You don't have that many %s.", g.MoneyName(2)))
			return
		}
		playerObj.Pennies -= amount
		targetObj.Pennies += amount
		g.PersistObjects(playerObj, targetObj)
		d.Send(fmt.Sprintf("You give %d %s to %s.", amount, g.MoneyName(amount), DisplayName(targetObj.Name)))
		g.Conns.SendToPlayer(target,
			fmt.Sprintf("%s gives you %d %s.", g.PlayerName(d.Player), amount, g.MoneyName(amount)))
		return
	}

	// Try as object
	thing := g.MatchObject(d.Player, whatStr)
	if thing == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	thingObj, ok := g.DB.Objects[thing]
	if !ok || thingObj.Location != d.Player {
		d.Send("You aren't carrying that.")
		return
	}

	// Move from player inventory to target inventory
	g.RemoveFromContents(d.Player, thing)
	thingObj.Location = target
	g.AddToContents(target, thing)
	g.PersistObjects(thingObj, targetObj)

	d.Send(fmt.Sprintf("You give %s to %s.", DisplayName(thingObj.Name), DisplayName(targetObj.Name)))
	g.Conns.SendToPlayer(target,
		fmt.Sprintf("%s gives you %s.", g.PlayerName(d.Player), DisplayName(thingObj.Name)))
}

func cmdEnter(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Enter what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("I don't see that here.")
		return
	}
	if obj.ObjType() != gamedb.TypeThing && obj.ObjType() != gamedb.TypeRoom {
		d.Send("You can't enter that.")
		return
	}
	if !obj.HasFlag(gamedb.FlagEnterOK) && !g.Controls(d.Player, target) {
		d.Send("Permission denied.")
		return
	}
	// Check enter lock
	if !CouldDoIt(g, d.Player, target, aLEnter) {
		HandleLockFailure(g, d, target, aEFail, aOEFail, aAEFail, "Permission denied.")
		return
	}

	loc := g.PlayerLocation(d.Player)
	playerObj := g.DB.Objects[d.Player]

	// Remove from current location
	g.RemoveFromContents(loc, d.Player)

	// Announce departure
	g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
		fmt.Sprintf("%s has left.", DisplayName(playerObj.Name)))

	// Move inside target
	playerObj.Location = target
	g.AddToContents(target, d.Player)
	g.PersistObjects(playerObj, obj)

	d.Send(fmt.Sprintf("You enter %s.", DisplayName(obj.Name)))
	g.Conns.SendToRoomExcept(g.DB, target, d.Player,
		fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))

	g.ShowRoom(d, target)
	g.QueueAttrAction(target, d.Player, 35, nil) // A_AENTER = 35
}

func cmdLeave(g *Game, d *Descriptor, _ string, _ []string) {
	playerObj, ok := g.DB.Objects[d.Player]
	if !ok {
		return
	}
	loc := playerObj.Location
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		d.Send("You can't leave.")
		return
	}
	// The container's location is where we go
	dest := locObj.Location
	if dest == gamedb.Nothing {
		d.Send("You can't leave.")
		return
	}
	// Check leave lock
	if !CouldDoIt(g, d.Player, loc, aLLeave) {
		HandleLockFailure(g, d, loc, aLFail, aOLFail, aALFail, "You can't leave.")
		return
	}

	// Remove from container
	g.RemoveFromContents(loc, d.Player)
	g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
		fmt.Sprintf("%s has left.", DisplayName(playerObj.Name)))

	// Move to container's location
	destObj, ok := g.DB.Objects[dest]
	if !ok {
		d.Send("You can't leave.")
		return
	}
	playerObj.Location = dest
	g.AddToContents(dest, d.Player)
	g.PersistObjects(playerObj, destObj)

	d.Send("You leave.")
	g.Conns.SendToRoomExcept(g.DB, dest, d.Player,
		fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))

	g.ShowRoom(d, dest)
	g.QueueAttrAction(loc, d.Player, 52, nil) // A_ALEAVE = 52
}

func cmdWhisper(g *Game, d *Descriptor, args string, _ []string) {
	// whisper player = message
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Whisper what to whom?")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	message := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok || targetObj.ObjType() != gamedb.TypePlayer {
		d.Send("I don't see that player here.")
		return
	}

	senderName := g.PlayerName(d.Player)
	d.Send(g.WrapMarker(d.Player, "WHISPER", fmt.Sprintf("You whisper \"%s\" to %s.", message, DisplayName(targetObj.Name))))
	g.SendMarkedToPlayer(target, "WHISPER",
		fmt.Sprintf("%s whispers \"%s\"", senderName, message))

	// Others in the room see that a whisper happened
	loc := g.PlayerLocation(d.Player)
	for _, dd := range g.Conns.AllDescriptors() {
		if dd.State != ConnConnected {
			continue
		}
		if dd.Player == d.Player || dd.Player == target {
			continue
		}
		if g.PlayerLocation(dd.Player) == loc {
			dd.Send(g.WrapMarker(dd.Player, "WHISPER", fmt.Sprintf("%s whispers something to %s.", senderName, DisplayName(targetObj.Name))))
		}
	}
}

func cmdUse(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Use what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	// Check use lock
	if !CouldDoIt(g, d.Player, target, aLUse) {
		HandleLockFailure(g, d, target, aUFail, aOUFail, aAUFail, "Permission denied.")
		return
	}
	// Fire A_USE
	useText := g.GetAttrText(target, 45) // A_USE = 45
	if useText != "" {
		d.Send(useText)
	}
	// Fire A_OUSE to room
	ouText := g.GetAttrText(target, 46) // A_OUSE = 46
	if ouText != "" {
		loc := g.PlayerLocation(d.Player)
		g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
			fmt.Sprintf("%s %s", g.PlayerName(d.Player), ouText))
	}
	// Fire A_AUSE action
	g.QueueAttrAction(target, d.Player, 16, nil) // A_AUSE = 16
}

func cmdKill(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Kill whom?")
		return
	}
	// kill player [= cost]
	targetStr := args
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		return
	}

	senderName := g.PlayerName(d.Player)
	d.Send(fmt.Sprintf("You killed %s!", DisplayName(targetObj.Name)))
	g.Conns.SendToPlayer(target,
		fmt.Sprintf("%s killed you!", senderName))

	// Send to home
	home := targetObj.Link
	if home != gamedb.Nothing {
		loc := targetObj.Location
		g.RemoveFromContents(loc, target)
		g.Conns.SendToRoomExcept(g.DB, loc, target,
			fmt.Sprintf("%s has left.", DisplayName(targetObj.Name)))
		targetObj.Location = home
		g.AddToContents(home, target)
		if destObj, ok := g.DB.Objects[home]; ok {
			g.PersistObjects(targetObj, destObj)
		}
		g.Conns.SendToRoomExcept(g.DB, home, target,
			fmt.Sprintf("%s has arrived.", DisplayName(targetObj.Name)))
		// Show room to victim
		for _, dd := range g.Conns.GetByPlayer(target) {
			g.ShowRoom(dd, home)
		}
	}
}

func cmdDictionary(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("@dictionary: Usage: @dictionary <object> = <word1> [<word2> ...]")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	value := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if !g.Controls(d.Player, target) {
		d.Send("Permission denied.")
		return
	}
	g.SetAttrByName(target, "DICTIONARY", value)
	d.Send("Set.")
}

// DisconnectPlayer handles a player disconnecting.
func (g *Game) DisconnectPlayer(d *Descriptor) {
	if d.State == ConnConnected {
		playerName := g.PlayerName(d.Player)
		loc := g.PlayerLocation(d.Player)

		// Fire ADISCONNECT triggers (player + master room + master room contents)
		connCount := len(g.Conns.GetByPlayer(d.Player))
		g.FireConnectAttr(d.Player, connCount, 40) // A_ADISCONNECT = 40

		// Clear CONNECTED flag on last disconnect (C TinyMUSH behavior)
		if connCount <= 1 {
			if obj, ok := g.DB.Objects[d.Player]; ok {
				obj.Flags[1] &^= gamedb.Flag2Connected
			}
		}

		g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
			fmt.Sprintf("%s has disconnected.", playerName))

		// Guest cleanup: if this was the last connection for a guest,
		// schedule destruction after a grace period.
		if g.Guests.IsGuest(d.Player) {
			player := d.Player
			go func() {
				time.Sleep(60 * time.Second)
				// Check if guest reconnected during grace period
				if len(g.Conns.GetByPlayer(player)) == 0 {
					g.DestroyGuest(player)
				}
			}()
		}
	}
	d.Close()
}
