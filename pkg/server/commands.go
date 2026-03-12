package server

import (
	"fmt"
	"log"
	mathrand "math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/eventbus"
	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// CommandHandler is the signature for game command implementations.
type CommandHandler func(g *Game, d *Descriptor, args string, switches []string)

// stripEqSep strips at most one leading space from the RHS of an '='
// split.  This matches C TinyMUSH's EV_STRIP_LS behaviour when the
// strip happens BEFORE evaluation: the conventional "= msg" separator
// space is removed, but leading spaces produced by function evaluation
// (e.g. switch() returning %b) are preserved.  Using strings.TrimSpace
// here would destroy content whitespace when the command string has
// already been evaluated by the queue executor.
func stripEqSep(s string) string {
	if len(s) > 0 && s[0] == ' ' {
		return s[1:]
	}
	return s
}

// Command represents a registered game command.
type Command struct {
	Name    string
	Handler CommandHandler
	NoGuest bool // if true, guests cannot use this command
	IsAlias bool // true for abbreviation aliases loaded from goTinyAlias.conf
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
	register("r", cmdReply)
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
	registerNG("@pcreate", cmdPcreate)
	registerNG("@botcreate", cmdBotcreate)
	registerNG("@find", cmdFind)
	registerNG("@stats", cmdStats)
	registerNG("@ps", cmdPs)

	// Eval / softcode
	register("@eval", cmdEval)
	registerNG("@switch", cmdSwitch)
	registerNG("@swi", cmdSwitch)
	registerNG("@dolist", cmdDolist)
	registerNG("@program", cmdProgram)
	register("@quitprogram", cmdQuitProgram)

	// Database (no guest)
	registerNG("@dump", cmdDump)
	registerNG("@fixdb", cmdFixDB)
	registerNG("@fixall", cmdFixAll)
	registerNG("@backup", cmdBackup)
	registerNG("@readcache", cmdReadCache)
	registerNG("@archive", cmdArchive)

	// Softcode / Queue management (no guest)
	registerNG("@function", cmdFunction)
	registerNG("@drain", cmdDrain)
	registerNG("@edit", cmdEdit)
	registerNG("@admin", cmdAdmin)
	registerNG("@verb", cmdVerb)

	// Attribute management (no guest)
	registerNG("@attribute", cmdAttribute)
	register("@attlist", cmdAttlist)
	registerNG("@cpattr", cmdCpattr)
	registerNG("@mvattr", cmdMvattr)

	// SQL (no guest)
	registerNG("@sql", cmdSQL)
	registerNG("@sqlinit", cmdSQLInit)
	registerNG("@sqldisconnect", cmdSQLDisconnect)

	// Session
	register("QUIT", cmdQuit)
	register("LOGOUT", cmdLogout)
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
	registerNG("@poor", cmdPoor) // wizard: set building pennies
	register("enter", cmdEnter)
	register("leave", cmdLeave)
	register("whisper", cmdWhisper)
	register("use", cmdUse)
	registerNG("kill", cmdKill)
	register("slay", cmdSlay) // wizard-only guaranteed kill

	// Communication
	register("@oemit", cmdOemit)
	register("@remit", cmdRemit)

	// Admin/Builder utilities
	registerNG("@password", cmdPassword)
	register("@version", cmdVersion)
	register("version", cmdVersion)
	register("@uptime", cmdUptime)
	register("uptime", cmdUptime)
	register("@motd", cmdMotd)
	registerNG("@chzone", cmdChzone)
	registerNG("@search", cmdSearch)
	registerNG("@entrances", cmdEntrances)
	registerNG("@quota", cmdQuota)
	registerNG("@decompile", cmdDecompile)
	registerNG("@power", cmdPower)
	registerNG("@apikey", cmdApikey)
	registerNG("@hook", cmdHook)
	registerNG("@instance", cmdInstance)

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
	registerNG("@roomformat", makeAttrSetter(232)) // A_ROOMFORMAT = 232

	// Sensory commands
	register("smell", makeSensoryCommand(233, 234, 235, "You don't smell anything special."))
	register("touch", makeSensoryCommand(236, 237, 238, "You don't feel anything special."))
	register("taste", makeSensoryCommand(239, 240, 241, "You don't taste anything special."))
	register("listen", makeSensoryCommand(242, 243, 244, "You don't hear anything special."))
	// Sensory attribute setters
	registerNG("@smell", makeAttrSetter(233))    // A_SMELL
	registerNG("@osmell", makeAttrSetter(234))   // A_OSMELL
	registerNG("@asmell", makeAttrSetter(235))   // A_ASMELL
	registerNG("@touch", makeAttrSetter(236))    // A_TOUCH
	registerNG("@otouch", makeAttrSetter(237))   // A_OTOUCH
	registerNG("@atouch", makeAttrSetter(238))   // A_ATOUCH
	registerNG("@taste", makeAttrSetter(239))    // A_TASTE
	registerNG("@otaste", makeAttrSetter(240))   // A_OTASTE
	registerNG("@ataste", makeAttrSetter(241))   // A_ATASTE
	registerNG("@sound", makeAttrSetter(242))    // A_SOUND
	registerNG("@osound", makeAttrSetter(243))   // A_OSOUND
	registerNG("@asound", makeAttrSetter(244))   // A_ASOUND
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

	// Event Bus (no guest)
	registerNG("@queue", cmdQueue)

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
			g.Notify(d.Player, "Permission denied.")
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

	// C TinyMUSH resolution order:
	// 1. Exact built-in command (non-alias)
	// 2. @-command prefix matching
	// 3. Channel aliases
	// 4. Exit matching (exits beat abbreviation aliases like "sa" for "say")
	// 5. Abbreviation aliases from goTinyAlias.conf
	// 6. Enter/leave aliases
	// 7. $-commands

	lower := strings.ToLower(cmdName)

	// Helper to execute a matched command with hooks
	execCmd := func(cmd *Command) {
		if cmd.NoGuest && g.IsGuest(d.Player) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		if g.Hooks != nil {
			g.executeWithHooks(d, cmdName, args, func() {
				cmd.Handler(g, d, args, switches)
			})
		} else {
			cmd.Handler(g, d, args, switches)
		}
	}

	// C TinyMUSH dispatch order:
	// 1. Single-char prefix commands (", :, ;) — handled before DispatchCommand
	// 2. HOME — handled before DispatchCommand
	// 3. Exit matching — exits before command table, matching C process_command
	// 4. Exact command match (built-ins + aliases from alias.conf)
	// 5. @-command prefix matching
	// 6. Channel aliases
	// 7. Enter/leave aliases
	// 8. $-command matching

	// 3. Exit matching — C TinyMUSH checks exits BEFORE the command hash table.
	// This ensures exit aliases like "sa" take priority over command aliases
	// like "alias sa say".
	if tryMoveByExit(g, d, input) {
		return
	}

	// 4. Exact match on commands (including aliases from alias.conf).
	if cmd, ok := g.Commands[lower]; ok {
		execCmd(cmd)
		return
	}

	// 5. Prefix matching for @-commands (skip aliases — they are exact-match only)
	if len(lower) > 1 && lower[0] == '@' {
		var matchedCmd *Command
		matchedName := ""
		matchCount := 0
		for name, cmd := range g.Commands {
			if !cmd.IsAlias && strings.HasPrefix(name, lower) {
				matchCount++
				// Prefer the shortest matching command name, so that
				// "@fo" → "@force" (6) rather than "@forwardlist" (13).
				// This matches C TinyMUSH's prefix resolution order.
				if matchedCmd == nil || len(name) < len(matchedName) {
					matchedCmd = cmd
					matchedName = name
				}
			}
		}
		if matchCount >= 1 && matchedCmd != nil {
			execCmd(matchedCmd)
			return
		}
	}

	// Unrecognized @<attr> commands: treat as &<attr> (set variable attribute).
	if len(lower) > 1 && lower[0] == '@' && args != "" {
		attrName := lower[1:]
		if strings.Contains(args, "=") {
			if g.IsGuest(d.Player) {
				g.Notify(d.Player, "Permission denied.")
				return
			}
			cmdSetVAttr(g, d, attrName+" "+args, nil)
			return
		}
	}

	// 6. Channel alias matching
	if g.Comsys != nil {
		if ca := g.Comsys.LookupAlias(d.Player, strings.ToLower(cmdName)); ca != nil {
			g.ComsysProcessAlias(d, ca, args)
			return
		}
	}

	// 7. Enter/leave aliases (A_LALIAS/A_EALIAS on objects)
	if tryEnterLeaveAlias(g, d, input) {
		return
	}

	// 8. $-command matching on objects in room/inventory
	if g.MatchDollarCommands(d.Player, d.Player, input) {
		return
	}

	g.Notify(d.Player, "Huh?  (Type \"help\" for help.)")
}

// Notify sends a message to all connections for a player, matching C's notify().
func (g *Game) Notify(player gamedb.DBRef, msg string) {
	g.Conns.SendToPlayer(player, msg)
}

// HasSwitch checks if a switch list contains a specific switch.
// C TinyMUSH uses prefix matching on switches: "delim" matches "delimit".
// The switch (from user input) must be a prefix of name (the canonical switch name).
func HasSwitch(switches []string, name string) bool {
	nameLower := strings.ToLower(name)
	for _, s := range switches {
		sLower := strings.ToLower(s)
		if strings.HasPrefix(nameLower, sLower) {
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
	g.AudibleRelay(loc, d.Player, msg)
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
	g.AudibleRelay(loc, d.Player, msg)
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
		g.Notify(d.Player, "You have not paged anyone.")
		return
	}
	// Format: page name=message or page name message
	var targetName, message string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetName = strings.TrimSpace(args[:eqIdx])
		message = stripEqSep(args[eqIdx+1:])
	} else {
		parts := strings.SplitN(args, " ", 2)
		targetName = parts[0]
		if len(parts) > 1 {
			message = parts[1]
		}
	}

	target := LookupPlayer(g.DB, targetName)
	if target == gamedb.Nothing {
		g.Notify(d.Player, fmt.Sprintf("I don't recognize %s.", targetName))
		g.Notify(d.Player, "No one to page.")
		return
	}

	if !g.Conns.IsConnected(target) {
		targetObj := g.DB.Objects[target]
		g.Notify(d.Player, fmt.Sprintf("%s is not connected.", DisplayName(targetObj.Name)))
		return
	}

	senderName := g.PlayerName(d.Player)
	targetObj := g.DB.Objects[target]

	pageData := map[string]any{
		"sender":  senderName,
		"target":  DisplayName(targetObj.Name),
		"message": message,
	}

	// Store last page target for reply — A_LASTPAGE = 200
	g.SetAttr(d.Player, 200, fmt.Sprintf("%d", target))
	g.SetAttr(target, 200, fmt.Sprintf("%d", d.Player))

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

func cmdReply(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "No one to page.")
		return
	}

	// Read A_LASTPAGE (200) to find who we last paged / who last paged us
	lastStr := g.GetAttrTextDirect(d.Player, 200)
	if lastStr == "" {
		g.Notify(d.Player, "No one to page.")
		return
	}

	targetRef, err := strconv.Atoi(lastStr)
	if err != nil {
		g.Notify(d.Player, "No one to page.")
		return
	}
	target := gamedb.DBRef(targetRef)

	targetObj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No one to page.")
		return
	}

	// Reuse cmdPage logic by constructing "targetname=message"
	cmdPage(g, d, fmt.Sprintf("%s=%s", DisplayName(targetObj.Name), args), nil)
}

func cmdEmit(g *Game, d *Descriptor, args string, switches []string) {
	if args == "" {
		return
	}

	if HasSwitch(switches, "room") {
		// @emit/room target=message — emit to the room containing target
		eqIdx := strings.IndexByte(args, '=')
		if eqIdx < 0 {
			g.Notify(d.Player, "Usage: @emit/room target = message")
			return
		}
		targetStr := strings.TrimSpace(args[:eqIdx])
		message := stripEqSep(args[eqIdx+1:])
		targetStr = evalExpr(g, d.Player, targetStr)
		message = evalExpr(g, d.Player, message)
		target := g.ResolveRef(d.Player, targetStr)
		if target == gamedb.Nothing {
			target = g.MatchObject(d.Player, targetStr)
		}
		if target == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that here.")
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
			g.AudibleRelay(loc, d.Player, message)
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
	g.AudibleRelay(loc, d.Player, args)
}

func cmdThink(g *Game, d *Descriptor, args string, _ []string) {
	// Evaluate the expression and show result to all player sessions (matches C notify())
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	result := ctx.Exec(args, eval.EvFCheck|eval.EvEval, nil)
	// Process side-effect notifications (pemit(), remit(), etc.) before sending result
	g.deliverNotifications(ctx)
	g.Conns.SendToPlayer(d.Player, result)
}

func cmdPemit(g *Game, d *Descriptor, args string, switches []string) {
	// @pemit target=message
	// @pemit/contents target=message  (send to all contents of target)
	// @pemit/list targets=message     (targets is space-separated dbrefs)
	// CS_TWO_ARG: no = means target=args, message=""
	var targetStr, message string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		message = stripEqSep(args[eqIdx+1:])
	} else {
		targetStr = strings.TrimSpace(args)
		message = ""
	}

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
			g.Notify(d.Player, "I don't see that here.")
			return
		}
		if _, ok := g.DB.Objects[target]; !ok {
			g.Notify(d.Player, "I don't see that here.")
			return
		}
		for _, cur := range g.DB.SafeContents(target) {
			g.SendMarkedToPlayer(cur, "EMIT", message)
			g.CheckPemitListen(cur, d.Player, message)
		}
		// C TinyMUSH also delivers to the room itself (notify_all_from_inside
		// uses MSG_ME_ALL), triggering LISTEN/^-patterns on the room.
		g.CheckPemitListen(target, d.Player, message)
		// C's notify_all_from_inside also has MSG_F_UP which triggers
		// AUDIBLE outward relay when the target is an AUDIBLE container.
		g.AudibleRelay(target, d.Player, message)
		return
	}

	if HasSwitch(switches, "list") {
		// @pemit/list: send to each dbref in space-separated list
		targets := strings.Fields(targetStr)
		for _, ts := range targets {
			ref := g.ResolveRef(d.Player, strings.TrimSpace(ts))
			if ref != gamedb.Nothing {
				g.SendMarkedToPlayer(ref, "EMIT", message)
				g.CheckPemitListen(ref, d.Player, message)
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
		g.Notify(d.Player, "Emit to whom?")
		return
	}
	g.SendMarkedToPlayer(target, "EMIT", message)
	// C TinyMUSH: @pemit to an object triggers its LISTEN/^ patterns
	g.CheckPemitListen(target, d.Player, message)
}

// --- Movement Commands ---

func cmdGo(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Go where?")
		return
	}
	if !tryMoveByExit(g, d, args) {
		g.Notify(d.Player, "You can't go that way.")
	}
}

func tryMoveByExit(g *Game, d *Descriptor, name string) bool {
	loc := g.PlayerLocation(d.Player)
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		return false
	}

	// C TinyMUSH matches_exit_from_list: exact match only — user input must
	// completely match one semicolon-separated alias segment. No prefix matching.
	matchExit := func(exitObj *gamedb.Object) bool {
		exitNames := strings.Split(exitObj.Name, ";")
		for _, ename := range exitNames {
			if strings.EqualFold(strings.TrimSpace(ename), name) {
				return true
			}
		}
		return false
	}

	// Helper to execute exit traversal (SUCC/OSUCC/ASUCC messages + move)
	doExit := func(exitRef gamedb.DBRef, exitObj *gamedb.Object) bool {
		dest := exitObj.Location
		if dest == gamedb.Nothing || dest == gamedb.Home {
			playerObj := g.DB.Objects[d.Player]
			dest = playerObj.Link
		}
		if dest == gamedb.Nothing {
			g.Notify(d.Player, "That exit doesn't lead anywhere.")
			return true
		}
		if !CouldDoIt(g, d.Player, exitRef, aLock) {
			HandleLockFailure(g, d, exitRef, aFail, aOFail, aAFail, "You can't go that way.")
			return true
		}
		if succ := g.GetAttrText(exitRef, 4); succ != "" {
			ctx := MakeEvalContextForObj(g, exitRef, d.Player, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			msg := ctx.Exec(succ, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			if msg != "" {
				g.Notify(d.Player, msg)
			}
		}
		if osucc := g.GetAttrText(exitRef, 1); osucc != "" {
			pObj := g.DB.Objects[d.Player]
			if pObj != nil && !pObj.HasFlag(gamedb.FlagDark) {
				ctx := MakeEvalContextForObj(g, exitRef, d.Player, func(c *eval.EvalContext) {
					functions.RegisterAll(c)
				})
				msg := ctx.Exec(osucc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
				if msg != "" {
					g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
						DisplayName(pObj.Name)+" "+msg)
				}
			}
		}
		g.QueueAttrAction(exitRef, d.Player, 12, nil) // exit ASUCC
		g.MovePlayer(d, dest)
		return true
	}

	seenExits := make(map[gamedb.DBRef]bool)
	exitRef := locObj.Exits
	for exitRef != gamedb.Nothing && !seenExits[exitRef] {
		seenExits[exitRef] = true
		exitObj, ok := g.DB.Objects[exitRef]
		if !ok {
			break
		}
		if matchExit(exitObj) {
			return doExit(exitRef, exitObj)
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
		g.Notify(d.Player, "You have no home!")
		return
	}
	g.Notify(d.Player, "There's no place like home...")
	g.Notify(d.Player, "There's no place like home...")
	g.Notify(d.Player, "There's no place like home...")
	g.MovePlayer(d, home)
}

// --- Information Commands ---

func cmdLook(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: look has CS_INTERP — evaluate the argument.
	args = evalExpr(g, d.Player, args)

	if args == "" || strings.EqualFold(args, "here") {
		// Look at current room
		loc := g.PlayerLocation(d.Player)
		g.ShowRoom(d, loc)
		return
	}
	// Look at something specific
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Ambiguous {
		g.Notify(d.Player, "I don't know which one you mean!")
		return
	}
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	g.ShowObject(d, target)
}

func cmdExamine(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: examine has CS_INTERP — evaluate the argument so that
	// function calls like loc(*player) resolve before object matching.
	args = evalExpr(g, d.Player, args)

	if args == "" {
		// C TinyMUSH: bare "examine" examines the player's location
		args = "here"
	}

	// Handle examine obj/attr syntax
	objName := args
	attrName := ""
	if idx := strings.IndexByte(args, '/'); idx >= 0 {
		objName = args[:idx]
		attrName = args[idx+1:]
	}

	target := g.MatchObject(d.Player, objName)
	if target == gamedb.Ambiguous {
		g.Notify(d.Player, "I don't know which one you mean!")
		return
	}
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
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
			g.Notify(d.Player, "I don't see that here.")
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
			// C TinyMUSH: if attr has AF_IS_LOCK, parse through boolexp for human-readable names
			if def != nil && def.Flags&gamedb.AFIsLock != 0 && text != "" {
				parsed := ParseBoolExp(g, d.Player, text)
				if parsed != nil {
					text = UnparseBoolExp(g, parsed)
				}
			}
			// C TinyMUSH: only show annotation if player controls object or owns attr
			showAnnotation := Controls(g, d.Player, target) || info.Owner == d.Player
			annotation := ""
			if showAnnotation {
				examObjOwner := ResolveOwner(g, target)
				annotation = attrAnnotation(g, d.Player, target, examObjOwner, info, def)
			}
			if annotation != "" {
				g.Notify(d.Player, fmt.Sprintf("  %s %s: %s", name, annotation, text))
			} else {
				g.Notify(d.Player, fmt.Sprintf("  %s: %s", name, text))
			}
			found = true
		}
		if !found {
			g.Notify(d.Player, "No matching attributes found.")
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
		g.Notify(d.Player, "You aren't carrying anything.")
		return
	}
	g.Notify(d.Player, "You are carrying:")
	for _, next := range contents {
		if _, ok := g.DB.Objects[next]; ok {
			g.Notify(d.Player, g.unparseObject(d.Player, next))
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
	g.Notify(d.Player, fmt.Sprintf("You have %d %s.", playerObj.Pennies, g.MoneyName(playerObj.Pennies)))
}

// --- Building Commands ---

func cmdDig(g *Game, d *Descriptor, args string, switches []string) {
	if args == "" {
		g.Notify(d.Player, "Dig what?")
		return
	}
	// @dig name[=exit_to[;alias],exit_from[;alias]]
	parts := strings.SplitN(args, "=", 2)
	roomName := strings.TrimSpace(parts[0])

	// Charge dig cost
	cost := g.Conf.DigCost
	playerObj := g.DB.Objects[d.Player]
	if playerObj.Pennies < cost {
		g.Notify(d.Player, fmt.Sprintf("Sorry, you don't have enough %s.", g.MoneyName(2)))
		return
	}
	playerObj.Pennies -= cost
	g.PersistObject(playerObj)

	newRef := g.CreateObject(roomName, gamedb.TypeRoom, d.Player)
	g.Notify(d.Player, fmt.Sprintf("%s created with room number %d.", roomName, newRef))

	// Handle exit creation if specified
	// C TinyMUSH: CS_ARGV splits remaining args by = so
	// @dig room=exit1=exit2 gives args[0]=exit1, args[1]=exit2
	if len(parts) > 1 {
		exitParts := strings.SplitN(parts[1], "=", 2)
		if exitParts[0] != "" {
			exitTo := strings.TrimSpace(exitParts[0])
			exitRef := g.CreateExit(exitTo, g.PlayerLocation(d.Player), newRef, d.Player)
			if exitRef != gamedb.Nothing {
				g.Notify(d.Player, "Opened.")
				g.Notify(d.Player, "Linked.")
			}
		}
		if len(exitParts) > 1 && exitParts[1] != "" {
			exitFrom := strings.TrimSpace(exitParts[1])
			exitRef := g.CreateExit(exitFrom, newRef, g.PlayerLocation(d.Player), d.Player)
			if exitRef != gamedb.Nothing {
				g.Notify(d.Player, "Opened.")
				g.Notify(d.Player, "Linked.")
			}
		}
	}

	// @dig/teleport — teleport the player to the new room
	if HasSwitch(switches, "teleport") {
		cmdTeleport(g, d, fmt.Sprintf("#%d", newRef), nil)
	}
}

func cmdOpen(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Open where?")
		return
	}
	// @open exit_name=destination
	parts := strings.SplitN(args, "=", 2)
	exitName := strings.TrimSpace(parts[0])

	// Charge open cost
	cost := g.Conf.OpenCost
	playerObj := g.DB.Objects[d.Player]
	if playerObj.Pennies < cost {
		g.Notify(d.Player, fmt.Sprintf("Sorry, you don't have enough %s.", g.MoneyName(2)))
		return
	}
	playerObj.Pennies -= cost
	g.PersistObject(playerObj)

	dest := gamedb.Nothing
	if len(parts) > 1 {
		dest = g.ResolveRef(d.Player, strings.TrimSpace(parts[1]))
	}
	loc := g.PlayerLocation(d.Player)
	exitRef := g.CreateExit(exitName, loc, dest, d.Player)
	if exitRef == gamedb.Nothing {
		return
	}
	g.Notify(d.Player, "Opened.")
	if dest != gamedb.Nothing {
		g.Notify(d.Player, "Linked.")
	}
}

func cmdDescribe(g *Game, d *Descriptor, args string, _ []string) {
	// @desc obj=text — sets desc. @desc obj — clears desc (C TinyMUSH behavior).
	eqIdx := strings.IndexByte(args, '=')
	var targetStr, desc string
	if eqIdx < 0 {
		// No '=' — clear the attribute (C TinyMUSH: @desc me clears DESC)
		targetStr = strings.TrimSpace(args)
		desc = ""
	} else {
		targetStr = strings.TrimSpace(args[:eqIdx])
		desc = strings.TrimSpace(args[eqIdx+1:])
	}
	// Empty target goes through MatchObject → "I don't see that here."

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	g.SetAttr(target, 6, desc) // A_DESC = 6
	g.Notify(d.Player, "Set.")
}

func cmdRename(g *Game, d *Descriptor, args string, _ []string) {
	// CS_TWO_ARG: no = means target=args, newName=""
	var targetStr, newName string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		newName = strings.TrimSpace(args[eqIdx+1:])
	} else {
		targetStr = strings.TrimSpace(args)
		newName = ""
	}
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if newName == "" {
		g.Notify(d.Player, "Give it what new name?")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		oldName := obj.Name
		obj.Name = newName
		g.PersistObject(obj)
		if obj.ObjType() == gamedb.TypePlayer && g.Store != nil {
			g.Store.UpdatePlayerIndex(obj, oldName)
		}
		g.Notify(d.Player, "Name set.")
	}
}

// --- Eval ---

func cmdEval(g *Game, d *Descriptor, args string, _ []string) {
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	result := ctx.Exec(args, eval.EvFCheck|eval.EvEval, nil)
	g.Conns.SendToPlayer(d.Player, result)
}

// --- Session ---

func cmdQuit(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Texts != nil {
		if txt := g.Texts.GetQuit(); txt != "" {
			d.SendNoNewline(txt)
		} else {
			g.Notify(d.Player, "Going home.")
		}
	} else {
		g.Notify(d.Player, "Going home.")
	}
	g.DisconnectPlayer(d)
}

// cmdLogout disconnects the character but keeps the socket open,
// returning the player to the login screen (C TinyMUSH R_LOGOUT behavior).
func cmdLogout(g *Game, d *Descriptor, _ string, _ []string) {
	g.LogoutPlayer(d)
}

func cmdReadCache(g *Game, d *Descriptor, _ string, _ []string) {
	// Wizard-only command
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if g.TextDir == "" {
		g.Notify(d.Player, "No text directory configured (-textdir flag).")
		return
	}
	count := g.ReloadTextFiles()
	g.Notify(d.Player, fmt.Sprintf("Text file cache reloaded. %d file(s) loaded from %s.", count, g.TextDir))
}

func cmdSetDoing(g *Game, d *Descriptor, args string, _ []string) {
	d.DoingStr = args
	g.Notify(d.Player, "Set.")
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
	EventQueues *eventbus.QueueManager // Pub/sub event queue system
	Guests      *GuestManager // Guest player tracking and cleanup
	objExecDepth int // Recursion depth counter for ExecuteAsObject
	objExecCount map[gamedb.DBRef]int // Per-object execution counter for rate limiting
	objExecCountReset time.Time // When the counter was last reset
	queueWake chan struct{} // Signal to wake queue processor immediately (player input)
	PeakPlayers int        // Historical peak connected player count
	StartTime   time.Time  // Server start time
	Hooks       map[string]*HookSet // Command hooks (uppercase cmd name -> hook set)
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
	// Find the next available dbref, clear stale CONNECTED flags,
	// and auto-set HAS_COMMANDS on objects with $-command attributes.
	// C TinyMUSH sets HAS_COMMANDS during db load; we do it here.
	maxRef := gamedb.DBRef(0)
	hasCommandsFixed := 0
	for ref, obj := range db.Objects {
		if ref > maxRef {
			maxRef = ref
		}
		// Clear CONNECTED flag — nobody is connected at startup.
		// The flatfile may have this baked in from when the dump was taken.
		if obj.Flags[1]&gamedb.Flag2Connected != 0 {
			obj.Flags[1] &^= gamedb.Flag2Connected
		}
		// Auto-set HAS_COMMANDS on objects with $-command attributes.
		if !obj.HasFlag2(gamedb.Flag2HasCommands) {
			for _, attr := range obj.Attrs {
				text := eval.StripAttrPrefix(attr.Value)
				if strings.HasPrefix(text, "$") {
					obj.Flags[1] |= gamedb.Flag2HasCommands
					hasCommandsFixed++
					break
				}
			}
		}
	}
	if hasCommandsFixed > 0 {
		log.Printf("Auto-set HAS_COMMANDS on %d objects with $-command attributes", hasCommandsFixed)
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
		EventBus:    bus,
		EventQueues: eventbus.NewQueueManager(),
		Guests:      NewGuestManager(),
		queueWake:   make(chan struct{}, 1),
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

// RoomOf walks the location chain from an object up to the enclosing room.
// This matches C TinyMUSH's where_room() / Location() behavior for @emit
// from carried objects: a cutter in a player's inventory emits to the room
// the player is in, not to the player's contents list.
func (g *Game) RoomOf(ref gamedb.DBRef) gamedb.DBRef {
	seen := make(map[gamedb.DBRef]bool)
	for {
		obj, ok := g.DB.Objects[ref]
		if !ok {
			return gamedb.Nothing
		}
		if obj.ObjType() == gamedb.TypeRoom {
			return ref
		}
		if seen[ref] {
			return gamedb.Nothing // cycle protection
		}
		seen[ref] = true
		ref = obj.Location
		if ref == gamedb.Nothing {
			return gamedb.Nothing
		}
	}
}

// MovePlayer moves a player to a new location.
func (g *Game) MovePlayer(d *Descriptor, dest gamedb.DBRef) {
	player := d.Player
	playerObj, ok := g.DB.Objects[player]
	if !ok {
		return
	}

	oldLoc := playerObj.Location
	isDark := playerObj.HasFlag(gamedb.FlagDark)

	// Source room: ALEAVE action (52), OLEAVE to room (51)
	if oldLoc != gamedb.Nothing {
		if !isDark {
			g.QueueAttrAction(oldLoc, player, 52, nil) // ALEAVE
			if oleave := g.GetAttrText(oldLoc, 51); oleave != "" {
				ctx := MakeEvalContextForObj(g, oldLoc, player, func(c *eval.EvalContext) {
					functions.RegisterAll(c)
				})
				msg := ctx.Exec(oleave, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
				if msg != "" {
					g.Conns.SendToRoomExcept(g.DB, oldLoc, player,
						DisplayName(playerObj.Name)+" "+msg)
				}
			} else {
				g.Conns.SendToRoomExcept(g.DB, oldLoc, player,
					fmt.Sprintf("%s has left.", DisplayName(playerObj.Name)))
			}
		}
		g.RemoveFromContents(oldLoc, player)
	}

	// Set new location
	playerObj.Location = dest

	// Add to new location's contents chain
	g.AddToContents(dest, player)

	// Announce arrival (default, before ShowRoom evaluates OSUCC)
	if !isDark {
		g.Conns.SendToRoomExcept(g.DB, dest, player,
			fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))
	}

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

	// Dest room: AENTER action (35), OENTER to room (53) - skip if DARK
	if !isDark {
		g.QueueAttrAction(dest, player, 35, nil) // AENTER
		if oenter := g.GetAttrText(dest, 53); oenter != "" {
			ctx := MakeEvalContextForObj(g, dest, player, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			msg := ctx.Exec(oenter, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			if msg != "" {
				g.Conns.SendToRoomExcept(g.DB, dest, player,
					DisplayName(playerObj.Name)+" "+msg)
			}
		}

		// Notify listeners on arrival
		g.MatchListenPatterns(dest, player,
			fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))
	}
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

// RemoveFromExits removes an exit from a room's exit chain.
// Mirrors RemoveFromContents but walks the Exits chain instead of Contents.
func (g *Game) RemoveFromExits(room gamedb.DBRef, exitRef gamedb.DBRef) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	// Head of chain matches — relink head
	if roomObj.Exits == exitRef {
		if o, ok := g.DB.Objects[exitRef]; ok {
			roomObj.Exits = o.Next
			o.Next = gamedb.Nothing
		}
		return
	}
	// Walk the chain to find the exit
	prev := roomObj.Exits
	seen := make(map[gamedb.DBRef]bool)
	for prev != gamedb.Nothing && !seen[prev] {
		seen[prev] = true
		prevObj, ok := g.DB.Objects[prev]
		if !ok {
			break
		}
		if prevObj.Next == exitRef {
			if o, ok := g.DB.Objects[exitRef]; ok {
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
	// Check if obj is already in this contents chain — prevent cycles.
	// Use a seen map to guard against infinite loops from corrupted chains.
	next := destObj.Contents
	seen := make(map[gamedb.DBRef]bool)
	for next != gamedb.Nothing && !seen[next] {
		if next == obj {
			return // already in chain
		}
		seen[next] = true
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
// visibleContents returns the dbrefs of objects visible in a room to a looker.
func (g *Game) visibleContents(room, looker gamedb.DBRef) []gamedb.DBRef {
	var refs []gamedb.DBRef
	for _, next := range g.DB.SafeContents(room) {
		obj, ok := g.DB.Objects[next]
		if !ok || next == looker || obj.IsGoing() {
			continue
		}
		visible := false
		if obj.ObjType() == gamedb.TypePlayer {
			if g.Conns.IsConnected(next) {
				if obj.HasFlag(gamedb.FlagDark) && !SeeAll(g, looker) && !Controls(g, looker, next) {
					// DARK player hidden
				} else {
					visible = true
				}
			}
		} else if obj.ObjType() == gamedb.TypeThing {
			if !obj.HasFlag(gamedb.FlagDark) || SeeAll(g, looker) || Controls(g, looker, next) {
				visible = true
			}
		}
		if visible {
			refs = append(refs, next)
		}
	}
	return refs
}

// visibleExits returns the dbrefs of exits visible in a room to a looker.
func (g *Game) visibleExits(room, looker gamedb.DBRef) []gamedb.DBRef {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return nil
	}
	roomIsDark := roomObj.HasFlag(gamedb.FlagDark)
	exitFmt := g.GetAttrText(room, 215) // A_LEXITS_FMT
	var refs []gamedb.DBRef
	exitRef := roomObj.Exits
	for exitRef != gamedb.Nothing {
		exitObj, ok := g.DB.Objects[exitRef]
		if !ok {
			break
		}
		canSee := true
		if exitObj.HasFlag(gamedb.FlagDark) {
			canSee = false
		} else if roomIsDark && exitFmt == "" && !exitObj.HasFlag2(gamedb.Flag2Light) {
			canSee = false
		}
		if canSee {
			refs = append(refs, exitRef)
		}
		exitRef = exitObj.Next
	}
	return refs
}

func (g *Game) ShowRoom(d *Descriptor, room gamedb.DBRef) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		g.Notify(d.Player, "You see nothing special.")
		return
	}

	// ROOMFORMAT (232) check — lookup chain: room → zones → master room.
	// When set, replaces the entire ShowRoom pipeline.
	if g.Conf == nil || g.Conf.RoomformatEnabled {
		roomFmt := g.GetAttrText(room, 232) // A_ROOMFORMAT
		if roomFmt == "" {
			for _, z := range roomObj.AllZones() {
				if roomFmt = g.GetAttrText(z, 232); roomFmt != "" {
					break
				}
			}
		}
		if roomFmt == "" {
			masterRoom := g.MasterRoomRef()
			if masterRoom != gamedb.Nothing {
				roomFmt = g.GetAttrText(masterRoom, 232)
			}
		}
		if roomFmt != "" {
			contentRefs := g.visibleContents(room, d.Player)
			exitRefs := g.visibleExits(room, d.Player)
			var cStrs, eStrs []string
			for _, ref := range contentRefs {
				cStrs = append(cStrs, fmt.Sprintf("#%d", ref))
			}
			for _, ref := range exitRefs {
				eStrs = append(eStrs, fmt.Sprintf("#%d", ref))
			}
			ctx := MakeEvalContextForObj(g, room, d.Player, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			result := ctx.Exec(roomFmt, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{
				fmt.Sprintf("#%d", room),
				strings.Join(cStrs, " "),
				strings.Join(eStrs, " "),
			})
			if result != "" {
				g.Notify(d.Player, result)
			}
			g.QueueAttrAction(room, d.Player, 36, nil) // A_ADESC
			return
		}
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
		g.Notify(d.Player, ctx.Exec(nameFmt, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	} else {
		g.Notify(d.Player, g.unparseObject(d.Player, room))
	}

	// Description — executor is the room (so v() resolves room attrs), enactor is the player
	// C TinyMUSH: if location is not a room (e.g. a THING you've entered) and player
	// is inside it, use IDESC (Interior Description, attr 32) instead of DESC.
	// Falls back to DESC if IDESC is not set.
	descAttr := 6 // A_DESC
	if roomObj.ObjType() != gamedb.TypeRoom {
		if idesc := g.GetAttrText(room, 32); idesc != "" { // A_IDESC = 32
			descAttr = 32
		}
	}
	desc := g.GetAttrText(room, descAttr)
	if desc != "" {
		ctx := makeCtx()
		evaluated := ctx.Exec(desc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		g.Notify(d.Player, evaluated)
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
					g.Notify(d.Player, msg)
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
			g.Notify(d.Player, result)
			conFmtHandled = true
		}
	}
	if !succShown && !conFmtHandled && len(contentRefs) > 0 {
		g.Notify(d.Player, "Contents:")
		for _, ref := range contentRefs {
			if _, ok := g.DB.Objects[ref]; ok {
				g.Notify(d.Player, g.unparseObject(d.Player, ref))
			}
		}
	}

	// Build list of visible exit dbrefs
	// DARK exits are ALWAYS hidden (even from wizards).
	// In a DARK room without EXITFORMAT, only LIGHT exits are visible.
	// When EXITFORMAT is set, all non-DARK exits are passed to it
	// (DARK rooms use EXITFORMAT for display, so room darkness is irrelevant).
	roomIsDark := roomObj.HasFlag(gamedb.FlagDark)
	exitFmt := g.GetAttrText(room, 215) // A_LEXITS_FMT
	var exitRefs []gamedb.DBRef
	exitRef := roomObj.Exits
	for exitRef != gamedb.Nothing {
		exitObj, ok := g.DB.Objects[exitRef]
		if !ok {
			break
		}
		canSee := true
		if exitObj.HasFlag(gamedb.FlagDark) {
			// DARK exits are always hidden
			canSee = false
		} else if roomIsDark && exitFmt == "" && !exitObj.HasFlag2(gamedb.Flag2Light) {
			// In a DARK room without EXITFORMAT, only LIGHT exits are visible
			canSee = false
		}
		if canSee {
			exitRefs = append(exitRefs, exitRef)
		}
		exitRef = exitObj.Next
	}

	// Exits — use EXITFORMAT (215) if set, otherwise default "Obvious exits:" list
	exitFmtHandled := false
	if exitFmt != "" {
		var refStrs []string
		for _, ref := range exitRefs {
			refStrs = append(refStrs, fmt.Sprintf("#%d", ref))
		}
		ctx := makeCtx()
		result := ctx.Exec(exitFmt, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{strings.Join(refStrs, " ")})
		if result != "" {
			g.Notify(d.Player, result)
			exitFmtHandled = true
		}
	}
	if !succShown && !exitFmtHandled && len(exitRefs) > 0 {
		g.Notify(d.Player, "Obvious exits:")
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
		g.Notify(d.Player, "  " + strings.Join(exitNames, "  "))
	}

	// Instance transparency: if this room is inside an instance THING with
	// INSTANCE_TRANSPARENT set, also show exterior contents/exits.
	if roomObj.Location != gamedb.Nothing {
		if vehicleObj, ok := g.DB.Objects[roomObj.Location]; ok {
			if vehicleObj.ObjType() == gamedb.TypeThing && vehicleObj.HasFlag3(gamedb.Flag3Instance) {
				if g.GetAttrTextByName(roomObj.Location, "INSTANCE_TRANSPARENT") == "1" {
					extLoc := vehicleObj.Location
					if extLoc != gamedb.Nothing {
						g.Notify(d.Player, "--- Outside ---")
						extContents := g.visibleContents(extLoc, d.Player)
						if len(extContents) > 0 {
							var names []string
							for _, ref := range extContents {
								if o, ok := g.DB.Objects[ref]; ok {
									names = append(names, DisplayName(o.Name))
								}
							}
							g.Notify(d.Player, "Outside: " + strings.Join(names, ", "))
						}
						extExits := g.visibleExits(extLoc, d.Player)
						if len(extExits) > 0 {
							var names []string
							for _, ref := range extExits {
								if o, ok := g.DB.Objects[ref]; ok {
									name := o.Name
									if idx := strings.IndexByte(name, ';'); idx >= 0 {
										name = name[:idx]
									}
									names = append(names, name)
								}
							}
							g.Notify(d.Player, "Nearby exits: " + strings.Join(names, "  "))
						}
					}
				}
			}
		}
	}

	// ADESC (36) — action list executed on the room when looked at
	g.QueueAttrAction(room, d.Player, 36, nil) // A_ADESC
}

// ShowObject displays an object to a player.
// Implements the C TinyMUSH did_it pattern: DESC to player, ODESC to room, ADESC action.
func (g *Game) ShowObject(d *Descriptor, target gamedb.DBRef) {
	if _, ok := g.DB.Objects[target]; !ok {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	// C TinyMUSH look_simple: show Name(#dbref flags) via unparse_object
	// when the looker can examine the target; otherwise just the name.
	g.Notify(d.Player, g.unparseObject(d.Player, target))

	// DESC (6) — description shown to the looker
	desc := g.GetAttrText(target, 6) // A_DESC
	if desc != "" {
		ctx := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		g.Notify(d.Player, ctx.Exec(desc, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	} else {
		g.Notify(d.Player, "You see nothing special.")
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
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	control := Examinable(g, d.Player, target)

	if control {
		// 1. Header: Name(#dbref flags)
		g.Notify(d.Player, g.unparseObject(d.Player, target))

		// 2. Type/Flags line with full flag names (C: if mushconf.ex_flags)
		g.Notify(d.Player, flagDescription(g, d.Player, obj))

		// 3. Owner/Key/Pennies line
		// C: "Owner: Name  Key: lockexpr  Pennies: N"
		// C capitalizes first letter of many_coins for this line
		coinName := g.MoneyName(obj.Pennies)
		if len(coinName) > 0 {
			coinName = strings.ToUpper(coinName[:1]) + coinName[1:]
		}
		lockDisplay := ""
		lockText := g.GetAttrText(target, aLock)
		if lockText != "" {
			parsed := ParseBoolExp(g, d.Player, lockText)
			if parsed != nil {
				lockDisplay = UnparseBoolExp(g, parsed)
			} else {
				lockDisplay = lockText
			}
		} else if obj.Lock != nil {
			lockDisplay = UnparseBoolExp(g, obj.Lock)
		}
		if lockDisplay == "" {
			lockDisplay = "*UNLOCKED*"
		}
		g.Notify(d.Player, fmt.Sprintf("Owner: %s  Key: %s %s: %d",
			g.PlayerName(obj.Owner), lockDisplay, coinName, obj.Pennies))

		// 4. Timestamps
		if !obj.LastAccess.IsZero() || !obj.LastMod.IsZero() {
			// C shows "Created:" on its own line, but we don't have CreatedTime in our struct.
			// C shows "Accessed: <time>    Modified: <time>" on one line.
			accessStr := ""
			modStr := ""
			if !obj.LastAccess.IsZero() {
				accessStr = obj.LastAccess.Format("Mon Jan 02 15:04:05 2006")
			}
			if !obj.LastMod.IsZero() {
				modStr = obj.LastMod.Format("Mon Jan 02 15:04:05 2006")
			}
			if accessStr != "" && modStr != "" {
				g.Notify(d.Player, fmt.Sprintf("Accessed: %s    Modified: %s", accessStr, modStr))
			} else if accessStr != "" {
				g.Notify(d.Player, fmt.Sprintf("Accessed: %s", accessStr))
			} else if modStr != "" {
				g.Notify(d.Player, fmt.Sprintf("Modified: %s", modStr))
			}
		}

		// 5. Zone (always shown, even *NOTHING*)
		g.Notify(d.Player, fmt.Sprintf("Zone: %s", g.unparseObject(d.Player, obj.Zone)))

		// 6. Parent (only if set)
		if obj.Parent != gamedb.Nothing {
			g.Notify(d.Player, fmt.Sprintf("Parent: %s", g.unparseObject(d.Player, obj.Parent)))
		}

		// 7. Powers (only if any powers are set)
		if pwrStr := powerDescription(obj); pwrStr != "" {
			g.Notify(d.Player, pwrStr)
		}
	}

	// Check per-player TRUNC_LENGTH for attribute display truncation
	truncLen := 0
	if ts := g.GetAttrTextByName(d.Player, "TRUNC_LENGTH"); ts != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(ts)); err == nil && n > 0 {
			truncLen = n
		}
	}

	// Show attributes with permission checks
	// Resolve the object's resolved owner for annotation comparison
	objResolvedOwner := ResolveOwner(g, target)
	for _, attr := range obj.Attrs {
		info := ParseAttrInfo(attr.Value)
		def := g.LookupAttrDef(attr.Number)
		if !CanReadAttr(g, d.Player, target, def, info.Flags, info.Owner) {
			continue
		}
		name := g.DB.GetAttrName(attr.Number)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", attr.Number)
		}
		text := eval.StripAttrPrefix(attr.Value)
		// C TinyMUSH: if attr has AF_IS_LOCK, parse through boolexp for human-readable names
		if def != nil && def.Flags&gamedb.AFIsLock != 0 && text != "" {
			parsed := ParseBoolExp(g, d.Player, text)
			if parsed != nil {
				text = UnparseBoolExp(g, parsed)
			}
		}
		if truncLen > 0 && len(text) > truncLen {
			text = text[:truncLen] + "..."
		}
		// C TinyMUSH: only show annotation if player controls object or owns attr
		showAnnotation := Controls(g, d.Player, target) || info.Owner == d.Player
		annotation := ""
		if showAnnotation {
			annotation = attrAnnotation(g, d.Player, target, objResolvedOwner, info, def)
		}
		if annotation != "" {
			g.Notify(d.Player, fmt.Sprintf("  %s %s: %s", name, annotation, text))
		} else {
			g.Notify(d.Player, fmt.Sprintf("  %s: %s", name, text))
		}
	}

	if control {
		// Contents section
		examContents := g.DB.SafeContents(target)
		if len(examContents) > 0 {
			g.Notify(d.Player, "Contents:")
			for _, cRef := range examContents {
				g.Notify(d.Player, g.unparseObject(d.Player, cRef))
			}
		}

		// Type-specific sections (matching C TinyMUSH order)
		switch obj.ObjType() {
		case gamedb.TypeRoom:
			// Exits
			if obj.Exits != gamedb.Nothing {
				g.Notify(d.Player, "Exits:")
				seenEx := make(map[gamedb.DBRef]bool)
				exitRef := obj.Exits
				for exitRef != gamedb.Nothing && !seenEx[exitRef] {
					seenEx[exitRef] = true
					if eObj, ok := g.DB.Objects[exitRef]; ok {
						g.Notify(d.Player, g.unparseObject(d.Player, exitRef))
						exitRef = eObj.Next
					} else {
						break
					}
				}
			} else {
				g.Notify(d.Player, "No exits.")
			}
			// Dropto
			if obj.Link != gamedb.Nothing {
				g.Notify(d.Player, fmt.Sprintf("Dropped objects go to: %s", g.unparseObject(d.Player, obj.Link)))
			}

		case gamedb.TypeThing, gamedb.TypePlayer:
			// Exits
			if obj.Exits != gamedb.Nothing {
				g.Notify(d.Player, "Exits:")
				seenEx := make(map[gamedb.DBRef]bool)
				exitRef := obj.Exits
				for exitRef != gamedb.Nothing && !seenEx[exitRef] {
					seenEx[exitRef] = true
					if eObj, ok := g.DB.Objects[exitRef]; ok {
						g.Notify(d.Player, g.unparseObject(d.Player, exitRef))
						exitRef = eObj.Next
					} else {
						break
					}
				}
			} else {
				g.Notify(d.Player, "No exits.")
			}
			// Home
			g.Notify(d.Player, fmt.Sprintf("Home: %s", g.unparseObject(d.Player, obj.Link)))
			// Location
			if obj.Location != gamedb.Nothing {
				g.Notify(d.Player, fmt.Sprintf("Location: %s", g.unparseObject(d.Player, obj.Location)))
			}

		case gamedb.TypeExit:
			// Source
			g.Notify(d.Player, fmt.Sprintf("Source: %s", g.unparseObject(d.Player, obj.Exits)))
			// Destination
			if obj.Location == gamedb.Nothing {
				g.Notify(d.Player, "Destination: *UNLINKED*")
			} else {
				g.Notify(d.Player, fmt.Sprintf("Destination: %s", g.unparseObject(d.Player, obj.Location)))
			}
		}
	} else {
		// Non-controlling viewer: show "Owned by Name"
		g.Notify(d.Player, fmt.Sprintf("Owned by %s", g.PlayerName(obj.Owner)))
	}
}

// attrAnnotation builds a TinyMUSH-style annotation string for an attribute.
// C TinyMUSH's view_atr shows: [#owner instance_flags(def_flags)]
// Per-instance flags (aflags) and definition flags (ap->flags) are shown
// separately: instance flags directly, definition flags in parentheses.
// Owner is only shown when it differs from the object's resolved owner.
func attrAnnotation(g *Game, player, target, objResolvedOwner gamedb.DBRef, info AttrInfo, def *gamedb.AttrDef) string {
	var parts []string
	// Show owner only if different from object's resolved owner
	if info.Owner != gamedb.Nothing && info.Owner != gamedb.DBRef(0) && info.Owner != objResolvedOwner {
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

// Flag visibility permissions for flag_description
const (
	flagPermPublic = 0 // Anyone can see
	flagPermWizard = 1 // Only wizards can see
	flagPermGod    = 2 // Only God can see
)

// flagEntry maps flag word/bit pairs to their TinyMUSH display character and full name.
// Ordered to match C TinyMUSH gen_flags[] table for consistent output.
type flagEntry struct {
	Word     int
	Bit      int
	Letter   byte
	Name     string
	ListPerm int // flagPermPublic/Wizard/God
}

var flagLetters = []flagEntry{
	// Matches C TinyMUSH gen_flags[] order exactly
	{1, gamedb.Flag2Abode, 'A', "ABODE", flagPermPublic},
	{1, gamedb.Flag2Blind, 'B', "BLIND", flagPermPublic},
	{0, gamedb.FlagChownOK, 'C', "CHOWN_OK", flagPermPublic},
	{0, gamedb.FlagDark, 'D', "DARK", flagPermPublic},
	{1, gamedb.Flag2Floating, 'F', "FREE", flagPermPublic},
	{0, gamedb.FlagGoing, 'G', "GOING", flagPermPublic},
	{0, gamedb.FlagHaven, 'H', "HAVEN", flagPermPublic},
	{0, gamedb.FlagInherit, 'I', "INHERIT", flagPermPublic},
	{0, gamedb.FlagJumpOK, 'J', "JUMP_OK", flagPermPublic},
	{1, gamedb.Flag2Key, 'K', "KEY", flagPermPublic},
	{0, gamedb.FlagLinkOK, 'L', "LINK_OK", flagPermPublic},
	{0, gamedb.FlagMonitor, 'M', "MONITOR", flagPermPublic},
	{0, gamedb.FlagNoSpoof, 'N', "NOSPOOF", flagPermWizard},
	{0, gamedb.FlagOpaque, 'O', "OPAQUE", flagPermPublic},
	{0, gamedb.FlagQuiet, 'Q', "QUIET", flagPermPublic},
	{0, gamedb.FlagSticky, 'S', "STICKY", flagPermPublic},
	{0, gamedb.FlagTrace, 'T', "TRACE", flagPermPublic},
	{1, gamedb.Flag2Unfindable, 'U', "UNFINDABLE", flagPermPublic},
	{0, gamedb.FlagVisual, 'V', "VISUAL", flagPermPublic},
	{0, gamedb.FlagWizard, 'W', "WIZARD", flagPermPublic},
	{1, gamedb.Flag2Ansi, 'X', "ANSI", flagPermPublic},
	{1, gamedb.Flag2ParentOK, 'Y', "PARENT_OK", flagPermPublic},
	{0, gamedb.FlagRoyalty, 'Z', "ROYALTY", flagPermPublic},
	{0, gamedb.FlagHearThru, 'a', "AUDIBLE", flagPermPublic},
	{1, gamedb.Flag2Bounce, 'b', "BOUNCE", flagPermPublic},
	{1, gamedb.Flag2Connected, 'c', "CONNECTED", flagPermPublic},
	{0, gamedb.FlagDestroyOK, 'd', "DESTROY_OK", flagPermPublic},
	{0, gamedb.FlagEnterOK, 'e', "ENTER_OK", flagPermPublic},
	{1, gamedb.Flag2Fixed, 'f', "FIXED", flagPermPublic},
	{1, gamedb.Flag2Uninspected, 'g', "UNINSPECTED", flagPermWizard},
	{0, gamedb.FlagHalt, 'h', "HALTED", flagPermPublic},
	{0, gamedb.FlagImmortal, 'i', "IMMORTAL", flagPermPublic},
	{1, gamedb.Flag2Gagged, 'j', "GAGGED", flagPermPublic},
	{1, gamedb.Flag2Light, 'l', "LIGHT", flagPermPublic},
	{0, gamedb.FlagMyopic, 'm', "MYOPIC", flagPermPublic},
	{1, gamedb.Flag2ZoneParent, 'o', "ZONE", flagPermPublic},
	{0, gamedb.FlagPuppet, 'p', "PUPPET", flagPermPublic},
	{0, gamedb.FlagTerse, 'q', "TERSE", flagPermPublic},
	{0, gamedb.FlagRobot, 'r', "ROBOT", flagPermPublic},
	{0, gamedb.FlagSafe, 's', "SAFE", flagPermPublic},
	{0, gamedb.FlagSeeThru, 't', "TRANSPARENT", flagPermPublic},
	{1, gamedb.Flag2Suspect, 'u', "SUSPECT", flagPermWizard},
	{0, gamedb.FlagVerbose, 'v', "VERBOSE", flagPermPublic},
	{1, gamedb.Flag2Staff, 'w', "STAFF", flagPermPublic},
	{1, gamedb.Flag2Slave, 'x', "SLAVE", flagPermWizard},
	{1, gamedb.Flag2ControlOK, 'z', "CONTROL_OK", flagPermPublic},
	{1, gamedb.Flag2StopMatch, '!', "STOP", flagPermPublic},
	{1, gamedb.Flag2HasCommands, '$', "COMMANDS", flagPermPublic},
	{1, gamedb.Flag2NoBLeed, '-', "NOBLEED", flagPermPublic},
	{1, gamedb.Flag2Watcher, '+', "WATCHER", flagPermPublic},
	{1, gamedb.Flag2HasDaily, '*', "HAS_DAILY", flagPermGod},
	{0, gamedb.FlagHasStartup, '=', "HAS_STARTUP", flagPermGod},
	{1, gamedb.Flag2HasFwd, '&', "HAS_FORWARDLIST", flagPermGod},
	{1, gamedb.Flag2HasListen, '@', "HAS_LISTEN", flagPermGod},
	{1, gamedb.Flag2HTML, '~', "HTML", flagPermPublic},
	{1, gamedb.Flag2HeadFlag, '?', "HEAD", flagPermPublic},
	{1, gamedb.Flag2Vacation, '|', "VACATION", flagPermPublic},
	// Flag word 2
	{2, gamedb.Flag3Instance, '^', "INSTANCE", flagPermPublic},
}

// powerNameEntry maps power word/bit pairs to their TinyMUSH display name.
// Ordered to match C TinyMUSH gen_powers[] table.
type powerNameEntry struct {
	Word int // 0=Powers[0], 1=Powers[1]
	Bit  int
	Name string
}

var powerNames = []powerNameEntry{
	{0, gamedb.PowAnnounce, "announce"},
	{0, gamedb.PowMdarkAttr, "attr_read"},
	{0, gamedb.PowWizAttr, "attr_write"},
	{0, gamedb.PowBoot, "boot"},
	{1, gamedb.Pow2Builder, "builder"},
	{0, gamedb.PowChownAny, "chown_anything"},
	{1, gamedb.Pow2Cloak, "cloak"},
	{0, gamedb.PowCommAll, "comm_all"},
	{0, gamedb.PowControlAll, "control_all"},
	{0, gamedb.PowWizardWho, "expanded_who"},
	{0, gamedb.PowFindUnfind, "find_unfindable"},
	{0, gamedb.PowFreeMoney, "free_money"},
	{0, gamedb.PowFreeQuota, "free_quota"},
	{0, gamedb.PowGuest, "guest"},
	{0, gamedb.PowHalt, "halt"},
	{0, gamedb.PowHide, "hide"},
	{0, gamedb.PowIdle, "idle"},
	{1, gamedb.Pow2LinkHome, "link_any_home"},
	{1, gamedb.Pow2LinkToAny, "link_to_anything"},
	{1, gamedb.Pow2LinkVar, "link_variable"},
	{0, gamedb.PowLongfingers, "long_fingers"},
	{0, gamedb.PowNoDestroy, "no_destroy"},
	{1, gamedb.Pow2OpenAnyLoc, "open_anywhere"},
	{0, gamedb.PowPassLocks, "pass_locks"},
	{0, gamedb.PowPoll, "poll"},
	{0, gamedb.PowProg, "prog"},
	{0, gamedb.PowChgQuotas, "quota"},
	{0, gamedb.PowSearch, "search"},
	{0, gamedb.PowExamAll, "see_all"},
	{0, gamedb.PowSeeQueue, "see_queue"},
	{0, gamedb.PowSeeHidden, "see_hidden"},
	{0, gamedb.PowStatAny, "stat_any"},
	{0, gamedb.PowSteal, "steal_money"},
	{0, gamedb.PowTelAnywhr, "tel_anywhere"},
	{0, gamedb.PowTelUnrst, "tel_anything"},
	{0, gamedb.PowUnkillable, "unkillable"},
	{0, gamedb.PowWatch, "watch_logins"},
	{1, gamedb.Pow2Bot, "bot"},
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
		// C's decode_flags skips FLAG_INTERNAL flags in flags() output
		if fl.ListPerm == flagPermGod {
			continue
		}
		if fl.Word == 0 && obj.HasFlag(fl.Bit) {
			buf.WriteByte(fl.Letter)
		} else if fl.Word == 1 && obj.HasFlag2(fl.Bit) {
			buf.WriteByte(fl.Letter)
		} else if fl.Word == 2 && obj.HasFlag3(fl.Bit) {
			buf.WriteByte(fl.Letter)
		}
	}
	return buf.String()
}

// flagDescription produces C TinyMUSH's flag_description output:
// "Type: THING Flags: DESTROY_OK ENTER_OK COMMANDS"
// Full flag names, filtered by viewer's permission level.
func flagDescription(g *Game, player gamedb.DBRef, obj *gamedb.Object) string {
	isWiz := Wizard(g, player)
	isGod := IsGod(g, player)
	var buf strings.Builder
	buf.WriteString("Type: ")
	buf.WriteString(obj.ObjType().String())
	buf.WriteString(" Flags:")
	for _, fl := range flagLetters {
		hasFlag := false
		if fl.Word == 0 {
			hasFlag = obj.HasFlag(fl.Bit)
		} else if fl.Word == 1 {
			hasFlag = obj.HasFlag2(fl.Bit)
		} else if fl.Word == 2 {
			hasFlag = obj.HasFlag3(fl.Bit)
		}
		if !hasFlag {
			continue
		}
		// Permission check
		if fl.ListPerm == flagPermWizard && !isWiz {
			continue
		}
		if fl.ListPerm == flagPermGod && !isGod {
			continue
		}
		buf.WriteByte(' ')
		buf.WriteString(fl.Name)
	}
	return buf.String()
}

// powerDescription produces C TinyMUSH's power_description output:
// "Powers: see_all boot halt"
// Returns empty string if no powers are set.
func powerDescription(obj *gamedb.Object) string {
	var buf strings.Builder
	buf.WriteString("Powers:")
	hasPower := false
	for _, pe := range powerNames {
		if obj.HasPower(pe.Word, pe.Bit) {
			buf.WriteByte(' ')
			buf.WriteString(pe.Name)
			hasPower = true
		}
	}
	if !hasPower {
		return ""
	}
	return buf.String()
}

// unparseObject produces C TinyMUSH's unparse_object output:
// "Name(#dbref flags)" if examinable or has visible flags,
// "*NOTHING*" for Nothing, "*HOME*" for Home, "*VARIABLE*" for Ambiguous.
func (g *Game) unparseObject(player, target gamedb.DBRef) string {
	switch target {
	case gamedb.Nothing:
		return "*NOTHING*"
	case gamedb.Home:
		return "*HOME*"
	case gamedb.Ambiguous:
		return "*VARIABLE*"
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		return fmt.Sprintf("*ILLEGAL*(#%d)", target)
	}
	if obj.ObjType() == gamedb.TypeGarbage {
		return fmt.Sprintf("*GARBAGE*(#%d%s)", target, flagString(obj))
	}
	// C TinyMUSH: MyopicExam(p,x) — VISUAL || (!Myopic(p) && (See_All(p) || same_owner || control_lock))
	// When obey_myopic (look/contents), MYOPIC suppresses dbrefs. When !obey_myopic (examine), use Examinable.
	pObj, pOK := g.DB.Objects[player]
	myopicExam := false
	if obj.HasFlag(gamedb.FlagVisual) {
		myopicExam = true
	} else if pOK && !pObj.HasFlag(gamedb.FlagMyopic) {
		myopicExam = Examinable(g, player, target)
	}
	showFlags := myopicExam ||
		obj.HasFlag(gamedb.FlagChownOK) || obj.HasFlag(gamedb.FlagJumpOK) ||
		obj.HasFlag(gamedb.FlagLinkOK) || obj.HasFlag(gamedb.FlagDestroyOK) ||
		obj.HasFlag2(gamedb.Flag2Abode)
	if showFlags {
		return fmt.Sprintf("%s(#%d%s)", obj.Name, target, flagString(obj))
	}
	return obj.Name
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
		g.Notify(d.Player, "Player Name        On For Idle   Room    Cmds   Host")
	} else {
		g.Notify(d.Player, fmt.Sprintf("%-16s%9s %4s  %s", "Player Name", "On For", "Idle", "Doing"))
	}

	type whoEntry struct {
		name  string
		onFor string
		idle  string
		doing string
		flags string
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
		// Hide DARK players from non-wizards
		if !isWiz {
			if pObj, ok := g.DB.Objects[dd.Player]; ok && pObj.HasFlag(gamedb.FlagDark) {
				continue
			}
		}
		name := g.PlayerName(dd.Player)
		onFor := FormatConnTime(now.Sub(dd.ConnTime))
		idle := FormatIdleTime(now.Sub(dd.LastCmd))
		// Build player flags string (wizard WHO only)
		var flags string
		if isWiz {
			if pObj, ok := g.DB.Objects[dd.Player]; ok {
				if pObj.HasFlag(gamedb.FlagDark) {
					flags += "D"
				}
			}
		}
		// Extract host/IP (strip port and IPv6 brackets)
		host := dd.Addr
		if idx := strings.LastIndex(host, ":"); idx >= 0 {
			host = host[:idx]
		}
		host = strings.Trim(host, "[]")
		loc := g.PlayerLocation(dd.Player)
		entries = append(entries, whoEntry{name, onFor, idle, dd.DoingStr, flags, loc, dd.CmdCount, host})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	for _, e := range entries {
		if isWiz {
			// C format: "%-16s%9s %4s%-3s#%-6d%5d%3s%-25s"
			g.Notify(d.Player, fmt.Sprintf("%-16s%9s %4s%-3s#%-6d%5d   %-25s",
				e.name, e.onFor, e.idle, e.flags, e.loc, e.cmds, e.host))
		} else {
			g.Notify(d.Player, fmt.Sprintf("%-16s%9s %4s  %s", e.name, e.onFor, e.idle, e.doing))
		}
	}

	count := len(entries)
	peak := g.Conns.PeakPlayers
	if count > peak {
		peak = count
	}
	g.Notify(d.Player, fmt.Sprintf("%d Players logged in, %d record, no maximum.", count, peak))
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
			// Check name and semicolon-separated aliases
			for _, alias := range strings.Split(obj.Name, ";") {
				if strings.EqualFold(strings.TrimSpace(alias), pName) {
					return ref
				}
			}
			// Check A_ALIAS attribute (58) — semicolon-separated like C TinyMUSH
			for _, attr := range obj.Attrs {
				if attr.Number == 58 {
					aliasStr := eval.StripAttrPrefix(attr.Value)
					if aliasStr != "" {
						for _, a := range strings.Split(aliasStr, ";") {
							if strings.EqualFold(strings.TrimSpace(a), pName) {
								return ref
							}
						}
					}
					break
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
	// Exact matches on any alias take priority over prefix matches on earlier aliases.
	// Uses word-boundary matching: "bath" matches "Radiant Bath" (C TinyMUSH string_match).
	matchAliases := func(objName string) int {
		aliases := strings.Split(objName, ";")
		// First pass: check for exact match on any alias
		for _, alias := range aliases {
			if strings.EqualFold(strings.TrimSpace(alias), nameLower) {
				return 2 // exact
			}
		}
		// Second pass: check for prefix/word match
		for _, alias := range aliases {
			aliasLower := strings.ToLower(strings.TrimSpace(alias))
			if stringMatchWord(aliasLower, nameLower) {
				return 1 // prefix/word match
			}
		}
		return 0
	}

	// searchContents searches a contents list for exact then prefix matches.
	// Returns (match, quality): quality 2=exact, 1=prefix, 0=none.
	// Returns Ambiguous if 2+ objects match at the same confidence level.
	searchContents := func(contents []gamedb.DBRef) (gamedb.DBRef, int) {
		var exactMatch gamedb.DBRef = gamedb.Nothing
		exactCount := 0
		var prefixMatch gamedb.DBRef = gamedb.Nothing
		prefixCount := 0
		for _, next := range contents {
			obj, ok := g.DB.Objects[next]
			if !ok {
				continue
			}
			switch matchAliases(obj.Name) {
			case 2:
				exactCount++
				if exactMatch == gamedb.Nothing {
					exactMatch = next
				}
			case 1:
				prefixCount++
				if prefixMatch == gamedb.Nothing {
					prefixMatch = next
				}
			}
		}
		if exactCount == 1 {
			return exactMatch, 2
		}
		if exactCount > 1 {
			return gamedb.Ambiguous, 2
		}
		if prefixCount == 1 {
			return prefixMatch, 1
		}
		if prefixCount > 1 {
			return gamedb.Ambiguous, 1
		}
		return gamedb.Nothing, 0
	}

	// C TinyMUSH match_result: exact alias matches win over prefix/word
	// matches regardless of which scope they came from. Search all scopes,
	// track the best match, prefer exact over prefix across scopes.
	bestRef := gamedb.Nothing
	bestQuality := 0

	consider := func(contents []gamedb.DBRef) {
		ref, q := searchContents(contents)
		if q > bestQuality {
			bestRef = ref
			bestQuality = q
		}
	}

	// Search player inventory
	consider(g.DB.SafeContents(player))

	// Search room contents and exits
	loc := playerObj.Location
	if loc != gamedb.Nothing {
		consider(g.DB.SafeContents(loc))
		consider(g.DB.SafeExits(loc))
	}

	// If the player IS a room, search its own exits.
	if playerObj.ObjType() == gamedb.TypeRoom {
		consider(g.DB.SafeExits(player))
	}

	return bestRef
}

// MatchInRoom matches an object name only in the room contents (for get).
func (g *Game) MatchInRoom(player gamedb.DBRef, name string) gamedb.DBRef {
	return g.matchInScope(player, name, true, false)
}

// MatchInInventory matches an object name only in the player's inventory (for drop).
func (g *Game) MatchInInventory(player gamedb.DBRef, name string) gamedb.DBRef {
	return g.matchInScope(player, name, false, true)
}

// matchInScope is the core match logic with configurable search scope.
func (g *Game) matchInScope(player gamedb.DBRef, name string, searchRoom, searchInv bool) gamedb.DBRef {
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

	playerObj, ok := g.DB.Objects[player]
	if !ok {
		return gamedb.Nothing
	}

	nameLower := strings.ToLower(name)

	matchAliases := func(objName string) int {
		for _, alias := range strings.Split(objName, ";") {
			alias = strings.TrimSpace(alias)
			aliasLower := strings.ToLower(alias)
			if aliasLower == nameLower {
				return 2
			}
			if stringMatchWord(aliasLower, nameLower) {
				return 1
			}
		}
		return 0
	}

	searchContents := func(contents []gamedb.DBRef) gamedb.DBRef {
		var exactMatch gamedb.DBRef = gamedb.Nothing
		exactCount := 0
		var prefixMatch gamedb.DBRef = gamedb.Nothing
		prefixCount := 0
		for _, next := range contents {
			obj, ok := g.DB.Objects[next]
			if !ok {
				continue
			}
			switch matchAliases(obj.Name) {
			case 2:
				exactCount++
				if exactMatch == gamedb.Nothing {
					exactMatch = next
				}
			case 1:
				prefixCount++
				if prefixMatch == gamedb.Nothing {
					prefixMatch = next
				}
			}
		}
		if exactCount == 1 {
			return exactMatch
		}
		if exactCount > 1 {
			return gamedb.Ambiguous
		}
		if prefixCount == 1 {
			return prefixMatch
		}
		if prefixCount > 1 {
			return gamedb.Ambiguous
		}
		return gamedb.Nothing
	}

	if searchRoom {
		loc := playerObj.Location
		if found := searchContents(g.DB.SafeContents(loc)); found != gamedb.Nothing {
			return found
		}
	}

	if searchInv {
		if found := searchContents(g.DB.SafeContents(player)); found != gamedb.Nothing {
			return found
		}
	}

	return gamedb.Nothing
}

// ResolveRef resolves a string (name or #dbref) to a DBRef.
func (g *Game) ResolveRef(player gamedb.DBRef, s string) gamedb.DBRef {
	s = strings.TrimSpace(s)
	if s == "" {
		return gamedb.Nothing
	}
	// Strip *player prefix (used for player lookup in TinyMUSH)
	if s[0] == '*' {
		s = s[1:]
		if s == "" {
			return gamedb.Nothing
		}
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

// ObjFlags returns the C-style flag letters string for an object (e.g. "PMOUc").
func (g *Game) ObjFlags(ref gamedb.DBRef) string {
	if obj, ok := g.DB.Objects[ref]; ok {
		return flagString(obj)
	}
	return ""
}

// GetAttrText returns the text of an attribute on an object.
// It checks the object first, then walks the parent chain (like TinyMUSH's atr_pget).
func (g *Game) GetAttrText(obj gamedb.DBRef, attrNum int) string {
	return g.getAttrTextWithParents(obj, attrNum, 10)
}

// getAttrTextWithParents walks the parent chain up to maxDepth levels.
// Matches C TinyMUSH's atr_pget_str behavior for AF_PRIVATE:
//   - At depth 0: if master attr definition has AF_PRIVATE, don't walk to parents
//   - At depth > 0: if instance flags have AF_PRIVATE, skip that parent's copy
func (g *Game) getAttrTextWithParents(obj gamedb.DBRef, attrNum int, maxDepth int) string {
	current := obj
	for depth := 0; depth <= maxDepth; depth++ {
		o, ok := g.DB.Objects[current]
		if !ok {
			return ""
		}
		for _, attr := range o.Attrs {
			if attr.Number == attrNum {
				// At depth > 0 (on a parent), check instance AF_PRIVATE
				if depth > 0 {
					instFlags := parseAttrFlags(attr.Value)
					if instFlags&gamedb.AFPrivate != 0 {
						break // skip this parent's copy, try grandparent
					}
				}
				return eval.StripAttrPrefix(attr.Value)
			}
		}
		// Before walking to parent, check master definition for AF_PRIVATE
		if depth == 0 && o.Parent != gamedb.Nothing && o.Parent != current {
			if masterFlags := g.getMasterAttrFlags(attrNum); masterFlags&gamedb.AFPrivate != 0 {
				return "" // master says don't inherit
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

// getMasterAttrFlags returns the master definition flags for an attribute number.
// Checks both well-known attributes and user-defined attributes.
func (g *Game) getMasterAttrFlags(attrNum int) int {
	// Check well-known built-in attrs
	if flags, ok := gamedb.WellKnownAttrFlags[attrNum]; ok {
		return flags
	}
	// Check user-defined attrs
	if def, ok := g.DB.AttrNames[attrNum]; ok {
		return def.Flags
	}
	return 0
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
func (g *Game) SetAttr(obj gamedb.DBRef, attrNum int, value string, executor ...gamedb.DBRef) {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return
	}
	// C TinyMUSH's atr_add stores Owner(player) — the resolved player-owner
	// of the executor — as the attribute owner. When an executor is provided,
	// use its resolved owner; otherwise fall back to the target's owner.
	attrOwner := o.Owner
	if len(executor) > 0 && executor[0] != gamedb.Nothing {
		attrOwner = ResolveOwner(g, executor[0])
	}
	owner := fmt.Sprintf("%d", attrOwner)

	for i, attr := range o.Attrs {
		if attr.Number == attrNum {
			if value == "" {
				// C TinyMUSH: atr_add with empty value calls atr_clr to delete the attr.
				// Remove the attribute so parent chain inheritance works correctly.
				o.Attrs = append(o.Attrs[:i], o.Attrs[i+1:]...)
				g.PersistObject(o)
				return
			}
			existing := ParseAttrInfo(attr.Value)
			fullValue := fmt.Sprintf("\x01%s:%d:%s", owner, existing.Flags, value)
			o.Attrs[i].Value = fullValue
			// C TinyMUSH auto-sets HAS_COMMANDS when a $-command attribute is set.
			if strings.HasPrefix(value, "$") && !o.HasFlag2(gamedb.Flag2HasCommands) {
				o.Flags[1] |= gamedb.Flag2HasCommands
			}
			g.PersistObject(o)
			return
		}
	}

	// If value is empty and attr doesn't exist, nothing to do.
	if value == "" {
		return
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

	// C TinyMUSH auto-sets HAS_COMMANDS when a $-command attribute is created.
	if strings.HasPrefix(value, "$") && !o.HasFlag2(gamedb.Flag2HasCommands) {
		o.Flags[1] |= gamedb.Flag2HasCommands
	}

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
	g.SetAttr(obj, attrNum, value, player)
	return true, ""
}

// SetAttrByName sets an attribute by name.
// Optional executor parameter is passed through to SetAttr to set the attr owner
// to the executor's resolved player-owner (matching C TinyMUSH's atr_add behavior).
func (g *Game) SetAttrByName(obj gamedb.DBRef, attrName string, value string, executor ...gamedb.DBRef) {
	// Look up in well-known first
	for num, name := range gamedb.WellKnownAttrs {
		if strings.EqualFold(name, attrName) {
			g.SetAttr(obj, num, value, executor...)
			return
		}
	}
	// Look up in user-defined
	if def, ok := g.DB.AttrByName[attrName]; ok {
		g.SetAttr(obj, def.Number, value, executor...)
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
	g.SetAttr(obj, newNum, value, executor...)
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
// makeSensoryCommand creates a handler for sensory commands (smell, touch, taste, listen).
// It follows the did_it pattern: text to player, O-text to room, queue A-text action.
func makeSensoryCommand(attr, oAttr, aAttr int, defaultMsg string) CommandHandler {
	return func(g *Game, d *Descriptor, args string, _ []string) {
		if g.Conf != nil && !g.Conf.SensoryEnabled {
			g.Notify(d.Player, "Huh?  (Type \"help\" for help.)")
			return
		}
		target := g.PlayerLocation(d.Player) // default: current room
		if args != "" {
			t := g.MatchObject(d.Player, args)
			if t == gamedb.Nothing {
				g.Notify(d.Player, "I don't see that here.")
				return
			}
			target = t
		}
		text := g.GetAttrText(target, attr)
		if text == "" {
			g.Notify(d.Player, defaultMsg)
			return
		}
		ctx := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		g.Notify(d.Player, ctx.Exec(text, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))

		// O-text to room
		oText := g.GetAttrText(target, oAttr)
		if oText != "" {
			ctx2 := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			msg := ctx2.Exec(oText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			name := g.PlayerName(d.Player)
			loc := g.PlayerLocation(d.Player)
			g.Conns.SendToRoomExcept(g.DB, loc, d.Player, name+" "+msg)
		}

		// Queue A-text action
		g.QueueAttrAction(target, d.Player, aAttr, nil)
	}
}

func makeAttrSetter(attrNum int) CommandHandler {
	return func(g *Game, d *Descriptor, args string, _ []string) {
		// C TinyMUSH: @attr obj clears the attribute; @attr obj=val sets it.
		eqIdx := strings.IndexByte(args, '=')
		var targetStr, value string
		if eqIdx < 0 {
			// No '=' — clear the attribute
			targetStr = strings.TrimSpace(args)
			value = ""
		} else {
			targetStr = strings.TrimSpace(args[:eqIdx])
			value = strings.TrimSpace(args[eqIdx+1:])
		}
		target := g.MatchObject(d.Player, targetStr)
		if target == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that here.")
			return
		}
		if !g.Controls(d.Player, target) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		ok, errMsg := g.SetAttrChecked(d.Player, target, attrNum, value)
		if !ok {
			g.Notify(d.Player, errMsg)
		} else {
			g.Notify(d.Player, "Set.")
		}
	}
}

// --- Player Object Commands ---

func cmdGet(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: empty args falls through to match which returns Nothing
	target := g.MatchInRoom(d.Player, args)
	if target == gamedb.Ambiguous {
		g.Notify(d.Player, "I don't know which one you mean!")
		return
	}
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	objType := obj.ObjType()
	if objType != gamedb.TypeThing && objType != gamedb.TypePlayer {
		g.Notify(d.Player, "You can't take that!")
		return
	}

	// C: match_neighbor only finds TYPE_THING in room — player matching "me" returns
	// Nothing via match, but if matched (e.g. long fingers), give friendly message.
	if target == d.Player {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// Already carrying it?
	if obj.Location == d.Player {
		g.Notify(d.Player, "You already have that!")
		return
	}

	loc := g.PlayerLocation(d.Player)

	// Check lock
	if !CouldDoIt(g, d.Player, target, aLock) {
		failMsg := "You can't pick that up."
		if obj.Location != loc {
			failMsg = "You can't take that from there."
		}
		HandleLockFailure(g, d, target, aFail, aOFail, aAFail, failMsg)
		return
	}

	// Notify previous owner/container if different from current room
	prevLoc := obj.Location
	if prevLoc != loc {
		if prevObj, ok := g.DB.Objects[prevLoc]; ok {
			_ = prevObj
			g.Conns.SendToPlayer(prevLoc,
				fmt.Sprintf("%s was taken from you.", DisplayName(obj.Name)))
		}
	}

	// Remove from previous location, add to player inventory
	g.RemoveFromContents(prevLoc, target)
	obj.Location = d.Player
	g.AddToContents(d.Player, target)
	g.PersistObject(obj)

	// Notify the object itself
	g.Conns.SendToPlayer(target, "Taken.")

	// C TinyMUSH did_it: SUCC/OSUCC/ASUCC — default "Taken." with no OSUCC
	g.DidItDefault(d.Player, target, aSucc, "Taken.", aOSucc, "", aASucc)
}

func cmdDrop(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: empty args / not found → "You don't have that!"
	target := g.MatchInInventory(d.Player, args)
	if target == gamedb.Ambiguous {
		g.Notify(d.Player, "I don't know which you mean!")
		return
	}
	if target == gamedb.Nothing {
		g.Notify(d.Player, "You don't have that!")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "You don't have that!")
		return
	}

	// Check drop lock (A_LDROP) — C TinyMUSH checks this before allowing drop
	if !CouldDoIt(g, d.Player, target, aLDrop) {
		HandleLockFailure(g, d, target, aDFail, aODFail, aADFail, "You can't drop that.")
		return
	}

	// Validate room BEFORE removing from inventory (prevent object corruption)
	loc := g.PlayerLocation(d.Player)
	locObj, ok := g.DB.Objects[loc]
	if !ok {
		g.Notify(d.Player, "You can't drop that here.")
		return
	}

	// Remove from inventory, add to room contents
	g.RemoveFromContents(d.Player, target)
	obj.Location = loc
	g.AddToContents(loc, target)
	g.PersistObjects(obj, locObj)

	// Notify the object itself
	g.Conns.SendToPlayer(target, "Dropped.")

	// C TinyMUSH did_it: DROP/ODROP/ADROP — default "Dropped." with ODROP "dropped X."
	oDropDefault := fmt.Sprintf("dropped %s.", DisplayName(obj.Name))
	g.DidItDefault(d.Player, target, aDrop, "Dropped.", aODrop, oDropDefault, aADrop)
}

func cmdGive(g *Game, d *Descriptor, args string, _ []string) {
	// C: give <recipient>=<thing/amount> — or no = means give <recipient> with empty item
	var targetStr, whatStr string
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		// No = sign: C treats whole arg as recipient, item is ""
		targetStr = strings.TrimSpace(args)
		whatStr = ""
	} else {
		targetStr = strings.TrimSpace(args[:eqIdx])
		whatStr = strings.TrimSpace(args[eqIdx+1:])
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "Give to whom?")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "Give to whom?")
		return
	}

	// Try as penny amount first (only if it's a pure number)
	if isNumeric(whatStr) {
		amount := toIntSimple(whatStr)
		// C: must be positive
		if amount <= 0 {
			g.Notify(d.Player, fmt.Sprintf("You must specify a positive number of %s.", g.MoneyName(2)))
			return
		}
		// C: can't give money to yourself
		if target == d.Player {
			g.Notify(d.Player, fmt.Sprintf("That player doesn't need that many %s!", g.MoneyName(2)))
			return
		}
		playerObj := g.DB.Objects[d.Player]
		if playerObj.Pennies < amount {
			g.Notify(d.Player, fmt.Sprintf("You don't have that many %s.", g.MoneyName(2)))
			return
		}
		playerObj.Pennies -= amount
		targetObj.Pennies += amount
		g.PersistObjects(playerObj, targetObj)
		g.Notify(d.Player, fmt.Sprintf("You give %d %s to %s.", amount, g.MoneyName(amount), DisplayName(targetObj.Name)))
		g.Conns.SendToPlayer(target,
			fmt.Sprintf("%s gives you %d %s.", g.PlayerName(d.Player), amount, g.MoneyName(amount)))
		return
	}

	// Try as object — match in giver's inventory
	thing := g.MatchInInventory(d.Player, whatStr)
	if thing == gamedb.Ambiguous {
		g.Notify(d.Player, "I don't know which one you mean!")
		return
	}
	if thing == gamedb.Nothing {
		g.Notify(d.Player, "You don't have that!")
		return
	}
	thingObj, ok := g.DB.Objects[thing]
	if !ok || thingObj.Location != d.Player {
		g.Notify(d.Player, "You aren't carrying that.")
		return
	}

	// Validate: can only give THINGs and PLAYERs
	thingType := thingObj.ObjType()
	if thingType != gamedb.TypeThing && thingType != gamedb.TypePlayer {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// Recipient must be Enter_OK or controlled by giver
	if !targetObj.HasFlag(gamedb.FlagEnterOK) && !g.Controls(d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// Check give-lock (LGIVE) on the thing being given
	if !CouldDoIt(g, d.Player, thing, aLGive) {
		HandleLockFailure(g, d, thing, aGFail, aOGFail, aAGFail,
			fmt.Sprintf("You can't give %s away.", DisplayName(thingObj.Name)))
		return
	}

	// Check receive-lock (LRECEIVE) on the recipient
	if !CouldDoIt(g, thing, target, aLRecv) {
		HandleLockFailure(g, d, target, aRFail, aORFail, aARFail,
			fmt.Sprintf("%s doesn't want %s.", DisplayName(targetObj.Name), DisplayName(thingObj.Name)))
		return
	}

	// Move from giver's inventory to recipient
	g.RemoveFromContents(d.Player, thing)
	thingObj.Location = target
	g.AddToContents(target, thing)
	g.PersistObjects(thingObj, targetObj)

	// C: notify recipient first, then "Given." to giver, then thing
	g.Conns.SendToPlayer(target,
		fmt.Sprintf("%s gave you %s.", g.PlayerName(d.Player), DisplayName(thingObj.Name)))
	g.Notify(d.Player, "Given.")
	g.Conns.SendToPlayer(thing,
		fmt.Sprintf("%s gave you to %s.", g.PlayerName(d.Player), DisplayName(targetObj.Name)))

	// Fire DROP/ODROP/ADROP on the thing with giver as cause (C: did_it line 349)
	g.DidIt(d.Player, thing, aDrop, aODrop, aADrop)

	// Fire SUCC/OSUCC/ASUCC on the thing with recipient as cause (C: did_it line 350)
	g.DidIt(target, thing, aSucc, aOSucc, aASucc)
}

// DidIt evaluates and sends message attributes on an object, then queues the action attr.
// Matches C TinyMUSH's did_it(): shows msgAttr text to cause, oMsgAttr text to the room
// (excluding cause), and queues aMsgAttr as an action on the object.
func (g *Game) DidIt(cause, thing gamedb.DBRef, msgAttr, oMsgAttr, aMsgAttr int) {
	// Evaluate and show message to cause
	if msgText := g.GetAttrText(thing, msgAttr); msgText != "" {
		ctx := MakeEvalContextForObj(g, thing, cause, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		msg := ctx.Exec(msgText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if msg != "" {
			g.Conns.SendToPlayer(cause, msg)
		}
	}

	// Evaluate and show O-message to room (excluding cause)
	// C TinyMUSH prefixes O-messages with the cause's name: "Name msg"
	if oMsgText := g.GetAttrText(thing, oMsgAttr); oMsgText != "" {
		loc := g.PlayerLocation(cause)
		if loc != gamedb.Nothing {
			ctx := MakeEvalContextForObj(g, thing, cause, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			msg := ctx.Exec(oMsgText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			if msg != "" {
				name := DisplayName(g.ObjName(cause))
				g.Conns.SendToRoomExcept(g.DB, loc, cause,
					fmt.Sprintf("%s %s", name, msg))
			}
		}
	}

	// Queue the action attribute
	if aMsgAttr > 0 {
		g.QueueAttrAction(thing, cause, aMsgAttr, nil)
	}
}

// DidItDefault is like DidIt but uses fallback strings when attrs are empty.
// Matches C's did_it() when called with literal default strings (e.g. "Taken.", "Dropped.").
func (g *Game) DidItDefault(cause, thing gamedb.DBRef, msgAttr int, msgDefault string, oMsgAttr int, oMsgDefault string, aMsgAttr int) {
	// Show message to cause
	msgText := g.GetAttrText(thing, msgAttr)
	if msgText == "" {
		msgText = msgDefault
	}
	if msgText != "" {
		ctx := MakeEvalContextForObj(g, thing, cause, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		msg := ctx.Exec(msgText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if msg != "" {
			g.Conns.SendToPlayer(cause, msg)
		}
	}

	// Show O-message to room (excluding cause), prefixed with cause's name
	oMsgText := g.GetAttrText(thing, oMsgAttr)
	if oMsgText == "" {
		oMsgText = oMsgDefault
	}
	if oMsgText != "" {
		loc := g.PlayerLocation(cause)
		if loc != gamedb.Nothing {
			ctx := MakeEvalContextForObj(g, thing, cause, func(c *eval.EvalContext) {
				functions.RegisterAll(c)
			})
			msg := ctx.Exec(oMsgText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			if msg != "" {
				name := DisplayName(g.ObjName(cause))
				g.Conns.SendToRoomExcept(g.DB, loc, cause,
					fmt.Sprintf("%s %s", name, msg))
			}
		}
	}

	// Queue the action attribute
	if aMsgAttr > 0 {
		g.QueueAttrAction(thing, cause, aMsgAttr, nil)
	}
}

// cmdPoor sets an object's pennies directly (wizard only).
// Usage: @poor <target>=<amount>
func cmdPoor(g *Game, d *Descriptor, args string, _ []string) {
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @poor <target>=<amount>")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	amountStr := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	amount := toIntSimple(amountStr)
	if amount < 0 {
		amount = 0
	}
	obj.Pennies = amount
	g.PersistObject(obj)
	g.Notify(d.Player, fmt.Sprintf("Set. %s now has %d %s.", DisplayName(obj.Name), amount, g.MoneyName(amount)))
}

func cmdEnter(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Enter what?")
		return
	}
	// C TinyMUSH do_enter uses match_neighbor() — room contents only, NOT inventory.
	// This prevents inventory items (e.g. "Nyki's Crystal Cutter") from matching
	// before room objects (e.g. "Nyki's Sled") when using "enter nyki's".
	target := g.MatchInRoom(d.Player, args)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if obj.ObjType() != gamedb.TypeThing && obj.ObjType() != gamedb.TypeRoom {
		g.Notify(d.Player, "You can't enter that.")
		return
	}
	if !obj.HasFlag(gamedb.FlagEnterOK) && !g.Controls(d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// Check enter lock
	if !CouldDoIt(g, d.Player, target, aLEnter) {
		HandleLockFailure(g, d, target, aEFail, aOEFail, aAEFail, "Permission denied.")
		return
	}

	loc := g.PlayerLocation(d.Player)
	playerObj := g.DB.Objects[d.Player]

	// Instance support: if target is an instance THING, enter its first interior room
	enterDest := target
	if obj.HasFlag3(gamedb.Flag3Instance) {
		firstRoom := g.InstanceFirstRoom(target)
		if firstRoom != gamedb.Nothing {
			enterDest = firstRoom
		}
	}

	// Remove from current location
	g.RemoveFromContents(loc, d.Player)

	// Announce departure
	g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
		fmt.Sprintf("%s has left.", DisplayName(playerObj.Name)))

	// Move inside target (or interior room for instances)
	playerObj.Location = enterDest
	g.AddToContents(enterDest, d.Player)
	if destObj, ok := g.DB.Objects[enterDest]; ok {
		g.PersistObjects(playerObj, obj, destObj)
	} else {
		g.PersistObjects(playerObj, obj)
	}

	g.Notify(d.Player, fmt.Sprintf("You enter %s.", DisplayName(obj.Name)))
	g.Conns.SendToRoomExcept(g.DB, enterDest, d.Player,
		fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))

	g.ShowRoom(d, enterDest)
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
		g.Notify(d.Player, "You can't leave.")
		return
	}
	// The container's location is where we go.
	// Instance support: if we're in an interior room of an instance THING,
	// leave goes to the instance's exterior location (skip the THING itself).
	dest := locObj.Location
	if dest == gamedb.Nothing {
		g.Notify(d.Player, "You can't leave.")
		return
	}
	if destObj, ok := g.DB.Objects[dest]; ok {
		if destObj.ObjType() == gamedb.TypeThing && destObj.HasFlag3(gamedb.Flag3Instance) {
			if destObj.Location != gamedb.Nothing {
				dest = destObj.Location
			}
		}
	}
	// Check leave lock — use strict check (no wizard bypass) so leave locks
	// are absolute. Wizards can use @tel to move around if needed.
	if !CouldDoItStrict(g, d.Player, loc, aLLeave) {
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
		g.Notify(d.Player, "You can't leave.")
		return
	}
	playerObj.Location = dest
	g.AddToContents(dest, d.Player)
	g.PersistObjects(playerObj, destObj)

	g.Notify(d.Player, "You leave.")
	g.Conns.SendToRoomExcept(g.DB, dest, d.Player,
		fmt.Sprintf("%s has arrived.", DisplayName(playerObj.Name)))

	g.ShowRoom(d, dest)
	g.QueueAttrAction(loc, d.Player, 52, nil) // A_ALEAVE = 52
}

func cmdWhisper(g *Game, d *Descriptor, args string, _ []string) {
	// whisper player = message  (CS_TWO_ARG: no = means target=args, msg="")
	var targetStr, message string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		message = stripEqSep(args[eqIdx+1:])
	} else {
		targetStr = strings.TrimSpace(args)
		message = ""
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "Whisper to whom?")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok || targetObj.ObjType() != gamedb.TypePlayer {
		g.Notify(d.Player, "I don't see that player here.")
		return
	}

	senderName := g.PlayerName(d.Player)
	loc := g.PlayerLocation(d.Player)
	whisperData := map[string]any{
		"sender":  senderName,
		"target":  DisplayName(targetObj.Name),
		"message": message,
	}

	// Sender sees their own whisper
	g.EmitEvent(d.Player, "WHISPER", events.Event{
		Type:   events.EvWhisper,
		Source: d.Player,
		Room:   loc,
		Text:   fmt.Sprintf("You whisper \"%s\" to %s.", message, DisplayName(targetObj.Name)),
		Data:   whisperData,
	})
	// Target receives the whisper
	g.EmitEvent(target, "WHISPER", events.Event{
		Type:   events.EvWhisper,
		Source: d.Player,
		Room:   loc,
		Text:   fmt.Sprintf("%s whispers \"%s\"", senderName, message),
		Data:   whisperData,
	})

	// Others in the room see that a whisper happened
	bystanderEv := events.Event{
		Type:   events.EvWhisper,
		Source: d.Player,
		Room:   loc,
		Text:   fmt.Sprintf("%s whispers something to %s.", senderName, DisplayName(targetObj.Name)),
		Data:   map[string]any{"sender": senderName, "target": DisplayName(targetObj.Name)},
	}
	for _, next := range g.DB.SafeContents(loc) {
		if next != d.Player && next != target && g.Conns.IsConnected(next) {
			g.EmitEvent(next, "WHISPER", bystanderEv)
		}
	}
}

func cmdUse(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Use what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	// Check use lock
	if !CouldDoIt(g, d.Player, target, aLUse) {
		HandleLockFailure(g, d, target, aUFail, aOUFail, aAUFail, "Permission denied.")
		return
	}
	// Fire A_USE — evaluate before sending (percent subs, functions, etc.)
	useText := g.GetAttrText(target, 45) // A_USE = 45
	if useText != "" {
		ctx := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		g.Notify(d.Player, ctx.Exec(useText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	}
	// Fire A_OUSE to room — evaluate before sending
	ouText := g.GetAttrText(target, 46) // A_OUSE = 46
	if ouText != "" {
		ctx := MakeEvalContextForObj(g, target, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		msg := ctx.Exec(ouText, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if msg != "" {
			loc := g.PlayerLocation(d.Player)
			g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
				fmt.Sprintf("%s %s", g.PlayerName(d.Player), msg))
		}
	}
	// Fire A_AUSE action
	g.QueueAttrAction(target, d.Player, 16, nil) // A_AUSE = 16
}

func cmdKill(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: kill doesn't take switches — empty args falls through match
	// Parse: kill <target> [= cost]
	targetStr := args
	costStr := ""
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		costStr = strings.TrimSpace(args[eqIdx+1:])
	}

	target := g.MatchObject(d.Player, targetStr)
	switch target {
	case gamedb.Nothing:
		g.Notify(d.Player, "I don't see that player here.")
		return
	case gamedb.Ambiguous:
		g.Notify(d.Player, "I don't know who you mean!")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		return
	}
	objType := targetObj.ObjType()
	if objType != gamedb.TypePlayer && objType != gamedb.TypeThing {
		g.Notify(d.Player, "Sorry, you can only kill players and things.")
		return
	}

	// HAVEN check — can't kill in a HAVEN room (unless wizard)
	victimLoc := targetObj.Location
	if locObj, ok := g.DB.Objects[victimLoc]; ok {
		if locObj.HasFlag(gamedb.FlagHaven) && !g.IsWizard(d.Player) {
			g.Notify(d.Player, "Sorry.")
			return
		}
	}
	// Victim controls their location and killer doesn't → can't kill
	if g.Controls(target, victimLoc) && !g.Controls(d.Player, victimLoc) {
		g.Notify(d.Player, "Sorry.")
		return
	}
	// IMMORTAL flag — can't be killed
	if targetObj.HasFlag(gamedb.FlagImmortal) && !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Sorry.")
		return
	}

	// Parse and clamp cost
	cost := g.Conf.KillMin
	if costStr != "" {
		if v, err := strconv.Atoi(costStr); err == nil {
			cost = v
		}
	}
	if cost < g.Conf.KillMin {
		cost = g.Conf.KillMin
	}
	if cost > g.Conf.KillMax {
		cost = g.Conf.KillMax
	}

	// Charge the killer
	playerObj := g.DB.Objects[d.Player]
	if playerObj.Pennies < cost {
		g.Notify(d.Player, fmt.Sprintf("You don't have enough %s.", g.MoneyName(2)))
		return
	}
	playerObj.Pennies -= cost
	g.PersistObject(playerObj)

	// Probability check: random(killGuarantee) < cost
	killGuarantee := g.Conf.KillGuarantee
	if killGuarantee <= 0 {
		killGuarantee = 100
	}
	roll := randInt(killGuarantee)
	if roll >= cost {
		// Failed
		g.Notify(d.Player, "Your murder attempt failed.")
		g.Conns.SendToPlayer(target,
			fmt.Sprintf("%s tried to kill you!", g.PlayerName(d.Player)))
		return
	}

	// Success — fire KILL/OKILL/AKILL trigger chain
	killMsg := fmt.Sprintf("You killed %s!", DisplayName(targetObj.Name))
	oKillMsg := fmt.Sprintf("killed %s!", DisplayName(targetObj.Name))
	g.DidItDefault(d.Player, target, aKill, killMsg, aOKill, oKillMsg, aAKill)

	// Notify victim
	g.Conns.SendToPlayer(target,
		fmt.Sprintf("%s killed you!", g.PlayerName(d.Player)))

	// Insurance payout (victim gets half cost if under pay limit)
	if cost > 0 {
		insurance := cost / 2
		if insurance > 0 {
			if targetOwner, ok := g.DB.Objects[targetObj.Owner]; ok {
				if targetOwner.Pennies < g.Conf.PayLimit {
					targetOwner.Pennies += insurance
					g.PersistObject(targetOwner)
					g.Conns.SendToPlayer(target,
						fmt.Sprintf("Your insurance policy pays %d %s.", insurance, g.MoneyName(insurance)))
				} else {
					g.Conns.SendToPlayer(target, "Your insurance policy has been revoked.")
				}
			}
		}
	}

	// Send victim home
	g.MoveToHome(target)
}

// cmdSlay is the wizard-only guaranteed kill (no cost, no probability check).
// Matches C TinyMUSH's do_kill called with key=KILL_SLAY.
func cmdSlay(g *Game, d *Descriptor, args string, _ []string) {
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	target := g.MatchObject(d.Player, args)
	switch target {
	case gamedb.Nothing:
		g.Notify(d.Player, "I don't see that player here.")
		return
	case gamedb.Ambiguous:
		g.Notify(d.Player, "I don't know who you mean!")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		return
	}
	objType := targetObj.ObjType()
	if objType != gamedb.TypePlayer && objType != gamedb.TypeThing {
		g.Notify(d.Player, "Sorry, you can only kill players and things.")
		return
	}

	killMsg := fmt.Sprintf("You killed %s!", DisplayName(targetObj.Name))
	oKillMsg := fmt.Sprintf("killed %s!", DisplayName(targetObj.Name))
	g.DidItDefault(d.Player, target, aKill, killMsg, aOKill, oKillMsg, aAKill)
	g.Conns.SendToPlayer(target, fmt.Sprintf("%s killed you!", g.PlayerName(d.Player)))
	g.MoveToHome(target)
}

// randInt returns a pseudo-random integer in [0, n). Matches C TinyMUSH Randomize(n).
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return mathrand.IntN(n)
}

// MoveToHome sends an object to its home, firing ODROP/ADROP on the object being moved.
// Matches C TinyMUSH move_via_generic(victim, HOME, ...).
func (g *Game) MoveToHome(ref gamedb.DBRef) {
	obj, ok := g.DB.Objects[ref]
	if !ok {
		return
	}
	home := obj.Link
	if home == gamedb.Nothing {
		home = gamedb.DBRef(g.Conf.PlayerStartingHome)
	}
	if home == gamedb.Nothing {
		return
	}
	from := obj.Location
	g.RemoveFromContents(from, ref)
	obj.Location = home
	g.AddToContents(home, ref)
	g.PersistObject(obj)
	// Notify room they left
	g.Conns.SendToRoomExcept(g.DB, from, ref,
		fmt.Sprintf("%s has left.", DisplayName(obj.Name)))
	// Notify new room they arrived
	g.Conns.SendToRoomExcept(g.DB, home, ref,
		fmt.Sprintf("%s has arrived.", DisplayName(obj.Name)))
	// Show new room to victim's connections
	for _, dd := range g.Conns.GetByPlayer(ref) {
		g.ShowRoom(dd, home)
	}
}

func cmdDictionary(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "@dictionary: Usage: @dictionary <object> = <word1> [<word2> ...]")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	value := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !g.Controls(d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	g.SetAttrByName(target, "DICTIONARY", value, d.Player)
	g.Notify(d.Player, "Set.")
}

// DisconnectPlayer handles a player disconnecting.
// LogoutPlayer disconnects the character but keeps the socket open,
// resetting the descriptor to the login screen so the player can
// connect as a different character (C TinyMUSH R_LOGOUT behavior).
func (g *Game) LogoutPlayer(d *Descriptor) {
	if d.State == ConnConnected {
		playerName := g.PlayerName(d.Player)
		loc := g.PlayerLocation(d.Player)

		// Update connlog with disconnect timestamp
		if g.Store != nil {
			g.Store.UpdateConnLogDisconnect(d.Player, time.Now().Unix())
		}

		// Fire ADISCONNECT triggers
		connCount := len(g.Conns.GetByPlayer(d.Player))
		g.FireConnectAttr(d.Player, connCount, 40) // A_ADISCONNECT = 40

		// Clear CONNECTED flag on last disconnect
		if connCount <= 1 {
			if obj, ok := g.DB.Objects[d.Player]; ok {
				obj.Flags[1] &^= gamedb.Flag2Connected
			}
		}

		g.Conns.SendToRoomExcept(g.DB, loc, d.Player,
			fmt.Sprintf("%s has disconnected.", playerName))

		// Guest cleanup
		if g.Guests.IsGuest(d.Player) {
			player := d.Player
			go func() {
				time.Sleep(60 * time.Second)
				if len(g.Conns.GetByPlayer(player)) == 0 {
					g.DestroyGuest(player)
				}
			}()
		}
	}

	// Reset descriptor to login state (keep socket open)
	g.Conns.Logout(d)
	d.State = ConnLogin
	d.Player = 0
	d.ConnTime = time.Now()
	d.LastCmd = time.Now()
	d.CmdCount = 0
	d.DoingStr = ""
	d.ProgData = nil
	d.LastRData = nil

	// Show the login screen again
	if g.Texts != nil {
		if txt := g.Texts.GetConnect(); txt != "" {
			d.SendNoNewline(txt)
		} else {
			g.Notify(d.Player, "Welcome to GoTinyMUSH. Commands: connect, create, WHO, QUIT")
		}
	} else {
		g.Notify(d.Player, "Welcome to GoTinyMUSH. Commands: connect, create, WHO, QUIT")
	}
}

func (g *Game) DisconnectPlayer(d *Descriptor) {
	if d.State == ConnConnected {
		playerName := g.PlayerName(d.Player)
		loc := g.PlayerLocation(d.Player)

		// Update connlog with disconnect timestamp
		if g.Store != nil {
			g.Store.UpdateConnLogDisconnect(d.Player, time.Now().Unix())
		}

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
