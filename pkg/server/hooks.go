package server

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// HookSet holds the attribute names on the master room for before/after/override/ignore hooks.
type HookSet struct {
	Before   string // Attr name: evaluated pre-command; return "0" cancels
	After    string // Attr name: evaluated post-command
	Override string // Attr name: replaces command entirely
	Ignore   string // Attr name: return "1" to silently block for this player
}

// executeWithHooks wraps a command handler with hook checks.
// Looks up hooks for the command name on the master room.
// Returns true if the command was handled (including by override/ignore blocking).
func (g *Game) executeWithHooks(d *Descriptor, cmdName string, args string, handler func()) bool {
	if g.Hooks == nil || (g.Conf != nil && !g.Conf.HooksEnabled) {
		handler()
		return true
	}

	upper := strings.ToUpper(cmdName)
	hooks, ok := g.Hooks[upper]
	if !ok {
		handler()
		return true
	}

	masterRoom := g.MasterRoomRef()
	if masterRoom == gamedb.Nothing {
		handler()
		return true
	}

	fullInput := cmdName
	if args != "" {
		fullInput = cmdName + " " + args
	}

	makeCtx := func() *eval.EvalContext {
		return MakeEvalContextForObj(g, masterRoom, d.Player, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
	}

	// Check ignore hook
	if hooks.Ignore != "" {
		ignoreText := g.GetAttrTextByName(masterRoom, hooks.Ignore)
		if ignoreText != "" {
			ctx := makeCtx()
			result := ctx.Exec(ignoreText, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{fullInput, args})
			if strings.TrimSpace(result) == "1" {
				return true // silently blocked
			}
		}
	}

	// Check override hook
	if hooks.Override != "" {
		overrideText := g.GetAttrTextByName(masterRoom, hooks.Override)
		if overrideText != "" {
			ctx := makeCtx()
			result := ctx.Exec(overrideText, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{fullInput, args})
			if result != "" {
				d.Send(result)
			}
			return true // command replaced
		}
	}

	// Check before hook
	if hooks.Before != "" {
		beforeText := g.GetAttrTextByName(masterRoom, hooks.Before)
		if beforeText != "" {
			ctx := makeCtx()
			result := ctx.Exec(beforeText, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{fullInput, args})
			if strings.TrimSpace(result) == "0" {
				return true // cancelled
			}
		}
	}

	// Execute the actual command
	handler()

	// Check after hook
	if hooks.After != "" {
		afterText := g.GetAttrTextByName(masterRoom, hooks.After)
		if afterText != "" {
			ctx := makeCtx()
			ctx.Exec(afterText, eval.EvFCheck|eval.EvEval|eval.EvStrip, []string{fullInput, args})
		}
	}

	return true
}

// cmdHook handles @hook command — wizard-only hook management.
// @hook/before <cmd>=<attr>
// @hook/after <cmd>=<attr>
// @hook/override <cmd>=<attr>
// @hook/ignore <cmd>=<attr>
// @hook/list [<cmd>]
// @hook/clear <cmd>
func cmdHook(g *Game, d *Descriptor, args string, switches []string) {
	if g.Conf != nil && !g.Conf.HooksEnabled {
		d.Send("The hook system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	if g.Hooks == nil {
		g.Hooks = make(map[string]*HookSet)
	}

	if HasSwitch(switches, "list") {
		cmdName := strings.ToUpper(strings.TrimSpace(args))
		if cmdName == "" {
			// List all hooks
			if len(g.Hooks) == 0 {
				d.Send("No hooks defined.")
				return
			}
			d.Send("--- Hooks ---")
			for name, hs := range g.Hooks {
				parts := []string{}
				if hs.Before != "" {
					parts = append(parts, "before="+hs.Before)
				}
				if hs.After != "" {
					parts = append(parts, "after="+hs.After)
				}
				if hs.Override != "" {
					parts = append(parts, "override="+hs.Override)
				}
				if hs.Ignore != "" {
					parts = append(parts, "ignore="+hs.Ignore)
				}
				d.Send(fmt.Sprintf("  %-15s %s", name, strings.Join(parts, ", ")))
			}
			return
		}
		// List specific command
		hs, ok := g.Hooks[cmdName]
		if !ok {
			d.Send(fmt.Sprintf("No hooks defined for %s.", cmdName))
			return
		}
		d.Send(fmt.Sprintf("Hooks for %s:", cmdName))
		d.Send(fmt.Sprintf("  Before:   %s", nonEmpty(hs.Before, "(none)")))
		d.Send(fmt.Sprintf("  After:    %s", nonEmpty(hs.After, "(none)")))
		d.Send(fmt.Sprintf("  Override: %s", nonEmpty(hs.Override, "(none)")))
		d.Send(fmt.Sprintf("  Ignore:   %s", nonEmpty(hs.Ignore, "(none)")))
		return
	}

	if HasSwitch(switches, "clear") {
		cmdName := strings.ToUpper(strings.TrimSpace(args))
		if cmdName == "" {
			d.Send("Usage: @hook/clear <command>")
			return
		}
		delete(g.Hooks, cmdName)
		g.persistHooks()
		d.Send(fmt.Sprintf("All hooks cleared for %s.", cmdName))
		return
	}

	// Set a hook: @hook/<type> <cmd>=<attr>
	hookType := ""
	for _, sw := range switches {
		switch sw {
		case "before", "after", "override", "ignore":
			hookType = sw
		}
	}
	if hookType == "" {
		d.Send("Usage: @hook/<before|after|override|ignore> <command>=<attribute>")
		d.Send("       @hook/list [<command>]")
		d.Send("       @hook/clear <command>")
		return
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send(fmt.Sprintf("Usage: @hook/%s <command>=<attribute>", hookType))
		return
	}
	cmdName := strings.ToUpper(strings.TrimSpace(args[:eqIdx]))
	attrName := strings.ToUpper(strings.TrimSpace(args[eqIdx+1:]))

	if cmdName == "" {
		d.Send("You must specify a command name.")
		return
	}

	hs, ok := g.Hooks[cmdName]
	if !ok {
		hs = &HookSet{}
		g.Hooks[cmdName] = hs
	}

	switch hookType {
	case "before":
		hs.Before = attrName
	case "after":
		hs.After = attrName
	case "override":
		hs.Override = attrName
	case "ignore":
		hs.Ignore = attrName
	}

	// If all fields empty, remove the entry
	if hs.Before == "" && hs.After == "" && hs.Override == "" && hs.Ignore == "" {
		delete(g.Hooks, cmdName)
	}

	g.persistHooks()
	if attrName == "" {
		d.Send(fmt.Sprintf("Hook %s cleared for %s.", hookType, cmdName))
	} else {
		d.Send(fmt.Sprintf("Hook %s set for %s to %s.", hookType, cmdName, attrName))
	}
}

// persistHooks saves hooks to the boltstore.
func (g *Game) persistHooks() {
	if g.Store == nil {
		return
	}
	boltHooks := make(map[string]*boltstore.HookSet, len(g.Hooks))
	for name, hs := range g.Hooks {
		boltHooks[name] = &boltstore.HookSet{
			Before:   hs.Before,
			After:    hs.After,
			Override: hs.Override,
			Ignore:   hs.Ignore,
		}
	}
	g.Store.PutHooks(boltHooks)
}

// nonEmpty returns s if non-empty, otherwise the default.
func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
