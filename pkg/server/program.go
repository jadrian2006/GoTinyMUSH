package server

import (
	"fmt"
	"log"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// ProgramData holds the state for an active @program session on a descriptor.
type ProgramData struct {
	WaitCause gamedb.DBRef      // Object that initiated @program (enactor)
	WaitData  *eval.RegisterData // Saved q-registers from initiating context
}

// cmdProgram implements @program <player>=<obj>/<attr>
// Captures the target player's next line of input and executes the specified
// attribute with the input available as %0.
func cmdProgram(g *Game, d *Descriptor, args string, switches []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("@program: Usage: @program <player>=<obj>/<attr>")
		return
	}

	targetStr := strings.TrimSpace(args[:eqIdx])
	objAttr := strings.TrimSpace(args[eqIdx+1:])

	// Resolve target player
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok || targetObj.ObjType() != gamedb.TypePlayer {
		d.Send("@program: Target must be a player.")
		return
	}

	// Permission check: caller must control the target player
	if !Controls(g, d.Player, target) {
		d.Send("Permission denied.")
		return
	}

	// Parse obj/attr
	slashIdx := strings.IndexByte(objAttr, '/')
	if slashIdx < 0 {
		d.Send("@program: Usage: @program <player>=<obj>/<attr>")
		return
	}
	objStr := strings.TrimSpace(objAttr[:slashIdx])
	attrName := strings.ToUpper(strings.TrimSpace(objAttr[slashIdx+1:]))

	// Resolve the object relative to the caller
	obj := g.MatchObject(d.Player, objStr)
	if obj == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}

	// Look up the attribute text
	cmdText := g.GetAttrTextByName(obj, attrName)
	if cmdText == "" {
		d.Send("No such attribute.")
		return
	}

	// Store the command text as A_PROGCMD on the target player
	g.SetAttrRaw(target, gamedb.A_PROGCMD, cmdText, d.Player, gamedb.AFInternal|gamedb.AFDark)

	// Find a descriptor for the target player and set program state
	targetDescs := g.Conns.GetByPlayer(target)
	if len(targetDescs) == 0 {
		d.Send("@program: That player is not connected.")
		// Clean up the attr we just set
		g.removeAttr(target, gamedb.A_PROGCMD)
		return
	}

	targetDesc := targetDescs[0]
	targetDesc.ProgData = &ProgramData{
		WaitCause: d.Player,
	}

	log.Printf("@program: player #%d programmed by #%d, attr %s on #%d",
		target, d.Player, attrName, obj)
}

// cmdQuitProgram implements @quitprogram [<player>]
// Cancels an active @program on yourself or the specified player.
func cmdQuitProgram(g *Game, d *Descriptor, args string, switches []string) {
	args = strings.TrimSpace(args)

	var targetDesc *Descriptor
	var target gamedb.DBRef

	if args == "" {
		// Cancel own program
		targetDesc = d
		target = d.Player
	} else {
		// Cancel another player's program
		target = g.MatchObject(d.Player, args)
		if target == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
		if !Controls(g, d.Player, target) {
			d.Send("Permission denied.")
			return
		}
		descs := g.Conns.GetByPlayer(target)
		if len(descs) == 0 {
			d.Send("That player is not connected.")
			return
		}
		targetDesc = descs[0]
	}

	if targetDesc.ProgData == nil {
		d.Send("That player is not in a program.")
		return
	}

	targetDesc.ProgData = nil
	g.removeAttr(target, gamedb.A_PROGCMD)
	g.Conns.SendToPlayer(target, "Program terminated.")
}

// HandleProgInput handles input from a player who is in @program mode.
// The input is substituted as %0 in the stored command text and executed.
func (g *Game) HandleProgInput(d *Descriptor, input string) {
	// Retrieve A_PROGCMD text from the player object
	cmdText := g.GetAttrTextDirect(d.Player, gamedb.A_PROGCMD)
	if cmdText == "" {
		// No command stored — clear program state
		d.ProgData = nil
		return
	}

	// Save and clear program state
	progData := d.ProgData
	d.ProgData = nil
	g.removeAttr(d.Player, gamedb.A_PROGCMD)

	// Create a queue entry with input as %0
	entry := &QueueEntry{
		Player:  progData.WaitCause,
		Cause:   d.Player,
		Caller:  progData.WaitCause,
		Command: cmdText,
		Args:    []string{input},
		RData:   progData.WaitData,
	}

	// Execute immediately
	g.ExecuteQueueEntry(entry)
}

// removeAttr removes an attribute from an object's attribute list.
func (g *Game) removeAttr(obj gamedb.DBRef, attrNum int) {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return
	}
	for i, attr := range o.Attrs {
		if attr.Number == attrNum {
			o.Attrs = append(o.Attrs[:i], o.Attrs[i+1:]...)
			g.PersistObject(o)
			return
		}
	}
}

// formatProgPrompt formats a prompt for a programmed player.
func formatProgPrompt(playerName string) string {
	return fmt.Sprintf("[%s]>", playerName)
}
