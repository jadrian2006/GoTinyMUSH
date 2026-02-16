package server

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/archive"
	mushcrypt "github.com/crystal-mush/gotinymush/pkg/crypt"
	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/flatfile"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// --- Building Commands ---

func cmdCreate(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Create what?")
		return
	}
	// @create name [= cost]
	parts := strings.SplitN(args, "=", 2)
	name := strings.TrimSpace(parts[0])
	ref := g.CreateObject(name, gamedb.TypeThing, d.Player)
	obj := g.DB.Objects[ref]
	// Place in player's inventory
	playerObj := g.DB.Objects[d.Player]
	obj.Location = d.Player
	g.AddToContents(d.Player, ref)
	obj.Link = g.PlayerLocation(d.Player) // home = current room
	g.PersistObjects(obj, playerObj)
	d.Send(fmt.Sprintf("Created: %s(#%d)", name, ref))
}

func cmdDestroy(g *Game, d *Descriptor, args string, switches []string) {
	if args == "" {
		d.Send("Destroy what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}
	// Check control
	if !g.Controls(d.Player, target) {
		d.Send("Permission denied.")
		return
	}
	if obj.HasFlag(gamedb.FlagSafe) && !HasSwitch(switches, "override") {
		d.Send("That object is SAFE. Use @set to remove the SAFE flag first, or use @destroy/override.")
		return
	}
	// Mark as GOING
	obj.Flags[0] |= gamedb.FlagGoing
	// Remove from location
	if obj.Location != gamedb.Nothing {
		g.RemoveFromContents(obj.Location, target)
	}
	g.PersistObject(obj)
	d.Send(fmt.Sprintf("Destroyed: %s(#%d)", obj.Name, target))
}

func cmdLink(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @link object = destination")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	destStr := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	dest := g.ResolveRef(d.Player, destStr)
	if dest == gamedb.Nothing {
		d.Send("I don't see that destination.")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		if obj.ObjType() == gamedb.TypeExit {
			// For exits, destination is stored in Location
			obj.Location = dest
		} else {
			// For players/things, @link sets Home (Link field)
			obj.Link = dest
		}
		g.PersistObject(obj)
		d.Send(fmt.Sprintf("Linked %s(#%d) to %s(#%d).", obj.Name, target, g.ObjName(dest), dest))
	}
}

func cmdUnlink(g *Game, d *Descriptor, args string, _ []string) {
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		if obj.ObjType() == gamedb.TypeExit {
			obj.Location = gamedb.Nothing
		} else {
			obj.Link = gamedb.Nothing
		}
		g.PersistObject(obj)
		d.Send(fmt.Sprintf("Unlinked %s(#%d).", obj.Name, target))
	}
}

func cmdParent(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @parent object = parent")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	parentStr := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	parent := gamedb.Nothing
	if parentStr != "" {
		parent = g.ResolveRef(d.Player, parentStr)
		if parent == gamedb.Nothing {
			d.Send("I don't see that parent.")
			return
		}
	}
	if obj, ok := g.DB.Objects[target]; ok {
		obj.Parent = parent
		g.PersistObject(obj)
		if parent == gamedb.Nothing {
			d.Send(fmt.Sprintf("Parent of %s(#%d) cleared.", obj.Name, target))
		} else {
			d.Send(fmt.Sprintf("Parent of %s(#%d) set to %s(#%d).", obj.Name, target, g.ObjName(parent), parent))
		}
	}
}

// PropagateParentAttrs copies PROPAGATE-flagged attributes from parent to child.
// Only copies attributes that the child doesn't already have.
// Returns the number of attributes propagated.
func (g *Game) PropagateParentAttrs(parent, child gamedb.DBRef) int {
	parentObj, ok := g.DB.Objects[parent]
	if !ok {
		return 0
	}
	childObj, ok := g.DB.Objects[child]
	if !ok {
		return 0
	}

	// Build set of attr numbers the child already has
	childAttrs := make(map[int]bool)
	for _, attr := range childObj.Attrs {
		childAttrs[attr.Number] = true
	}

	count := 0
	for _, attr := range parentObj.Attrs {
		// Check if this attribute's definition has AF_PROPAGATE
		def := g.LookupAttrDef(attr.Number)
		if def == nil || def.Flags&gamedb.AFPropagate == 0 {
			continue
		}
		// Skip if child already has it
		if childAttrs[attr.Number] {
			continue
		}
		// Copy the attribute value from parent to child
		childObj.Attrs = append(childObj.Attrs, gamedb.Attribute{
			Number: attr.Number,
			Value:  attr.Value,
		})
		count++
	}

	if count > 0 {
		g.PersistObject(childObj)
	}
	return count
}

func cmdChown(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @chown object = player")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	ownerStr := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	owner := g.ResolveRef(d.Player, ownerStr)
	if owner == gamedb.Nothing {
		d.Send("I don't see that player.")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		obj.Owner = owner
		g.PersistObject(obj)
		d.Send(fmt.Sprintf("Owner of %s(#%d) changed to %s(#%d).", obj.Name, target, g.ObjName(owner), owner))
	}
}

func cmdClone(g *Game, d *Descriptor, args string, switches []string) {
	if args == "" {
		d.Send("Clone what?")
		return
	}
	// @clone[/parent][/inventory] obj [= newname]
	parts := strings.SplitN(args, "=", 2)
	target := g.MatchObject(d.Player, strings.TrimSpace(parts[0]))
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	srcObj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}
	newName := srcObj.Name
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		newName = strings.TrimSpace(parts[1])
	}

	ref := g.CreateObject(newName, srcObj.ObjType(), d.Player)
	newObj := g.DB.Objects[ref]

	// /parent switch: set parent to the original instead of copying its parent
	if HasSwitch(switches, "parent") {
		newObj.Parent = target
	} else {
		newObj.Parent = srcObj.Parent
	}
	newObj.Link = srcObj.Link
	if srcObj.ObjType() == gamedb.TypeExit {
		newObj.Location = srcObj.Location // Copy destination for exits
	}

	// Copy attributes (unless /parent, where we inherit from parent chain)
	if !HasSwitch(switches, "parent") {
		for _, attr := range srcObj.Attrs {
			newObj.Attrs = append(newObj.Attrs, gamedb.Attribute{
				Number: attr.Number,
				Value:  attr.Value,
			})
		}
	}

	// Place in player's inventory (default and /inventory behavior)
	playerObj := g.DB.Objects[d.Player]
	newObj.Location = d.Player
	g.AddToContents(d.Player, ref)

	g.PersistObjects(newObj, playerObj)
	d.Send(fmt.Sprintf("Cloned %s(#%d) to %s(#%d).", srcObj.Name, target, newName, ref))
}

func cmdWipe(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Wipe what?")
		return
	}
	// @wipe obj[/pattern]
	objStr := args
	pattern := "*"
	if slashIdx := strings.IndexByte(args, '/'); slashIdx >= 0 {
		objStr = args[:slashIdx]
		pattern = strings.ToUpper(args[slashIdx+1:])
	}
	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		return
	}

	if pattern == "*" {
		count := len(obj.Attrs)
		obj.Attrs = nil
		g.PersistObject(obj)
		d.Send(fmt.Sprintf("Wiped %d attributes from %s(#%d).", count, obj.Name, target))
	} else {
		var remaining []gamedb.Attribute
		count := 0
		for _, attr := range obj.Attrs {
			name := g.DB.GetAttrName(attr.Number)
			if name != "" && wildMatchSimple(pattern, strings.ToUpper(name)) {
				count++
			} else {
				remaining = append(remaining, attr)
			}
		}
		obj.Attrs = remaining
		g.PersistObject(obj)
		d.Send(fmt.Sprintf("Wiped %d attributes matching %s from %s(#%d).", count, pattern, obj.Name, target))
	}
}

func cmdLock(g *Game, d *Descriptor, args string, switches []string) {
	// @lock/attr obj/attrname — lock an attribute (sets AF_LOCK)
	if HasSwitch(switches, "attr") {
		g.lockAttrInstance(d, args, true)
		return
	}

	// @lock obj = lockkey (simplified - just store as text)
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @lock object = key")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	lockStr := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	lockAttrNum := aLock // 38
	if HasSwitch(switches, "enter") || HasSwitch(switches, "enterlock") {
		lockAttrNum = aLEnter // 55
	} else if HasSwitch(switches, "leave") || HasSwitch(switches, "leavelock") {
		lockAttrNum = aLLeave // 56
	} else if HasSwitch(switches, "use") || HasSwitch(switches, "uselock") {
		lockAttrNum = aLUse // 58
	}
	g.SetAttr(target, lockAttrNum, lockStr)
	d.Send("Locked.")
}

func cmdUnlock(g *Game, d *Descriptor, args string, switches []string) {
	// @unlock/attr obj/attrname — unlock an attribute (clears AF_LOCK)
	if HasSwitch(switches, "attr") {
		g.lockAttrInstance(d, args, false)
		return
	}

	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	lockAttrNum := aLock // 38
	if HasSwitch(switches, "enter") || HasSwitch(switches, "enterlock") {
		lockAttrNum = aLEnter // 55
	} else if HasSwitch(switches, "leave") || HasSwitch(switches, "leavelock") {
		lockAttrNum = aLLeave // 56
	} else if HasSwitch(switches, "use") || HasSwitch(switches, "uselock") {
		lockAttrNum = aLUse // 58
	}
	g.SetAttr(target, lockAttrNum, "")
	d.Send("Unlocked.")
}

// lockAttrInstance sets or clears AF_LOCK on an individual attribute instance.
// args should be "obj/attrname".
func (g *Game) lockAttrInstance(d *Descriptor, args string, lock bool) {
	slashIdx := strings.IndexByte(args, '/')
	if slashIdx < 0 {
		d.Send("Usage: @lock/attr obj/attrname")
		return
	}
	objName := strings.TrimSpace(args[:slashIdx])
	attrName := strings.ToUpper(strings.TrimSpace(args[slashIdx+1:]))

	target := g.MatchObject(d.Player, objName)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		d.Send("Permission denied.")
		return
	}

	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}

	// Resolve attr number
	attrNum := -1
	if def, ok := g.DB.AttrByName[attrName]; ok {
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
		d.Send(fmt.Sprintf("No such attribute: %s", attrName))
		return
	}

	for i, attr := range obj.Attrs {
		if attr.Number == attrNum {
			info := ParseAttrInfo(attr.Value)
			text := eval.StripAttrPrefix(attr.Value)
			owner := info.Owner
			if owner == gamedb.Nothing {
				owner = obj.Owner
			}
			if lock {
				info.Flags |= gamedb.AFLock
			} else {
				info.Flags &^= gamedb.AFLock
			}
			obj.Attrs[i].Value = fmt.Sprintf("\x01%d:%d:%s", owner, info.Flags, text)
			g.PersistObject(obj)
			if lock {
				d.Send("Attribute locked.")
			} else {
				d.Send("Attribute unlocked.")
			}
			return
		}
	}
	d.Send(fmt.Sprintf("No such attribute: %s", attrName))
}

// --- Admin/Wizard Commands ---

func cmdTeleport(g *Game, d *Descriptor, args string, _ []string) {
	// @tel dest  OR  @tel victim = dest
	var victim gamedb.DBRef
	var destStr string

	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		victimStr := strings.TrimSpace(args[:eqIdx])
		destStr = strings.TrimSpace(args[eqIdx+1:])
		victim = g.MatchObject(d.Player, victimStr)
		if victim == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
	} else {
		victim = d.Player
		destStr = strings.TrimSpace(args)
	}

	if strings.EqualFold(destStr, "home") {
		if obj, ok := g.DB.Objects[victim]; ok {
			destStr = fmt.Sprintf("#%d", obj.Link)
		}
	}

	dest := g.ResolveRef(d.Player, destStr)
	if dest == gamedb.Nothing {
		d.Send("I don't see that destination.")
		return
	}

	// Find descriptor for victim (if connected)
	descs := g.Conns.GetByPlayer(victim)

	// Remove from old location
	if obj, ok := g.DB.Objects[victim]; ok {
		oldLoc := obj.Location
		if oldLoc != gamedb.Nothing {
			g.RemoveFromContents(oldLoc, victim)
			g.Conns.SendToRoomExcept(g.DB, oldLoc, victim,
				fmt.Sprintf("%s has left.", DisplayName(obj.Name)))
		}
		obj.Location = dest
		g.AddToContents(dest, victim)
		persistList := []*gamedb.Object{obj}
		if destObj, ok := g.DB.Objects[dest]; ok {
			persistList = append(persistList, destObj)
		}
		if oldLoc != gamedb.Nothing {
			if oldLocObj, ok := g.DB.Objects[oldLoc]; ok {
				persistList = append(persistList, oldLocObj)
			}
		}
		g.PersistObjects(persistList...)
		g.Conns.SendToRoomExcept(g.DB, dest, victim,
			fmt.Sprintf("%s has arrived.", DisplayName(obj.Name)))
	}

	if victim == d.Player {
		g.ShowRoom(d, dest)
	} else {
		d.Send(fmt.Sprintf("Teleported %s to %s(#%d).", g.ObjName(victim), g.ObjName(dest), dest))
		if len(descs) > 0 {
			g.ShowRoom(descs[0], dest)
		}
	}
}

func cmdForce(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @force object = command")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	command := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if !g.Controls(d.Player, target) {
		d.Send("Permission denied.")
		return
	}
	g.DoForce(d.Player, target, command)
}

func cmdTriggerCmd(g *Game, d *Descriptor, args string, switches []string) {
	if HasSwitch(switches, "now") {
		g.DoTriggerNow(d.Player, d.Player, args)
	} else {
		g.DoTrigger(d.Player, d.Player, args)
	}
	d.Send("Triggered.")
}

func cmdWaitCmd(g *Game, d *Descriptor, args string, _ []string) {
	g.DoWait(d.Player, d.Player, args)
	d.Send("Queued.")
}

func cmdNotify(g *Game, d *Descriptor, args string, _ []string) {
	// @notify obj[/attr] [= count]
	var objAttr, countStr string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		objAttr = strings.TrimSpace(args[:eqIdx])
		countStr = strings.TrimSpace(args[eqIdx+1:])
	} else {
		objAttr = strings.TrimSpace(args)
	}

	parts := strings.SplitN(objAttr, "/", 2)
	target := g.MatchObject(d.Player, parts[0])
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}

	attr := 0
	if len(parts) > 1 {
		attr = g.ResolveAttrNum(parts[1])
	}

	count := 1
	if countStr != "" {
		count = toIntSimple(countStr)
	}
	if count < 1 {
		count = 1
	}

	woken := g.Queue.NotifySemaphore(target, attr, count)
	d.Send(fmt.Sprintf("Notified %d command(s).", woken))
}

func cmdHalt(g *Game, d *Descriptor, args string, switches []string) {
	if HasSwitch(switches, "all") {
		// @halt/all - halt all objects' queue entries
		removed := g.Queue.HaltAll()
		d.Send(fmt.Sprintf("All halted. %d command(s) removed from queue.", removed))
		return
	}
	target := d.Player
	if args != "" {
		target = g.MatchObject(d.Player, args)
		if target == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
	}
	removed := g.Queue.HaltPlayer(target)
	// Note: C TinyMUSH's @halt only clears queue entries — it does NOT set
	// the HALT flag. The HALT flag is only set via @set obj=HALT. This is
	// important because STARTUP patterns like "@halt me; @wait 60=@tr me/loop"
	// rely on the object still being able to queue new commands after @halt.
	d.Send(fmt.Sprintf("Halted. %d command(s) removed from queue.", removed))
}

func cmdBoot(g *Game, d *Descriptor, args string, _ []string) {
	target := LookupPlayer(g.DB, strings.TrimSpace(args))
	if target == gamedb.Nothing {
		d.Send("No such player.")
		return
	}
	descs := g.Conns.GetByPlayer(target)
	if len(descs) == 0 {
		d.Send("That player is not connected.")
		return
	}
	for _, dd := range descs {
		dd.Send("You have been booted.")
		g.DisconnectPlayer(dd)
	}
	d.Send(fmt.Sprintf("Booted %s.", g.ObjName(target)))
}

func cmdWall(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		return
	}
	name := g.PlayerName(d.Player)
	msg := fmt.Sprintf("## %s shouts: %s", name, args)
	for _, dd := range g.Conns.AllDescriptors() {
		if dd.State == ConnConnected {
			dd.Send(msg)
		}
	}
}

func cmdNewPassword(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @newpassword player = password")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	newPass := strings.TrimSpace(args[eqIdx+1:])
	target := LookupPlayer(g.DB, targetStr)
	if target == gamedb.Nothing {
		d.Send("No such player.")
		return
	}
	// God protection: only God can change God's password
	if IsGod(g, target) && !IsGod(g, d.Player) {
		d.Send("Only God can change God's password. Use the -godpass flag to reset it externally.")
		return
	}
	// Encrypt and store
	hash := mushcrypt.Crypt(newPass, "XX")
	g.SetAttr(target, aPass, hash)
	d.Send(fmt.Sprintf("Password for %s changed.", g.ObjName(target)))
}

func cmdFind(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Find what?")
		return
	}
	pattern := strings.ToLower(strings.TrimSpace(args))
	count := 0
	for _, obj := range g.DB.Objects {
		if obj.IsGoing() {
			continue
		}
		if wildMatchSimple(pattern, strings.ToLower(obj.Name)) {
			d.Send(fmt.Sprintf("  %s(#%d%s) Owner: %s(#%d)",
				obj.Name, obj.DBRef, typeChar(obj.ObjType()),
				g.ObjName(obj.Owner), obj.Owner))
			count++
			if count >= 200 {
				d.Send("*** Too many results, truncated ***")
				break
			}
		}
	}
	d.Send(fmt.Sprintf("%d object(s) found.", count))
}

func cmdStats(g *Game, d *Descriptor, _ string, _ []string) {
	rooms, things, exits, players, garbage := 0, 0, 0, 0, 0
	for _, obj := range g.DB.Objects {
		switch obj.ObjType() {
		case gamedb.TypeRoom:
			rooms++
		case gamedb.TypeThing:
			things++
		case gamedb.TypeExit:
			exits++
		case gamedb.TypePlayer:
			if obj.IsGoing() {
				garbage++
			} else {
				players++
			}
		case gamedb.TypeGarbage:
			garbage++
		default:
			if obj.IsGoing() {
				garbage++
			} else {
				things++
			}
		}
	}
	d.Send(fmt.Sprintf("Database statistics:"))
	d.Send(fmt.Sprintf("  %d rooms, %d things, %d exits, %d players, %d garbage",
		rooms, things, exits, players, garbage))
	d.Send(fmt.Sprintf("  %d total objects", len(g.DB.Objects)))
	d.Send(fmt.Sprintf("  %d attribute definitions", len(g.DB.AttrNames)))
	imm, wait, sem := g.Queue.Stats()
	d.Send(fmt.Sprintf("  Queue: %d immediate, %d waiting, %d semaphore", imm, wait, sem))
	d.Send(fmt.Sprintf("  %d active connections", g.Conns.Count()))
}

func cmdPs(g *Game, d *Descriptor, _ string, switches []string) {
	imm, wait, sem := g.Queue.Stats()
	d.Send(fmt.Sprintf("Queue: %d immediate, %d waiting, %d semaphore", imm, wait, sem))
	d.Send(fmt.Sprintf("Total: %d entries", imm+wait+sem))

	if HasSwitch(switches, "all") {
		entries := g.Queue.Peek(50)
		if len(entries) == 0 {
			d.Send("(no entries)")
			return
		}
		for i, e := range entries {
			name := g.PlayerName(e.Player)
			cmd := e.Command
			if len(cmd) > 60 {
				cmd = cmd[:60] + "..."
			}
			qtype := "imm"
			if !e.WaitUntil.IsZero() {
				qtype = "wait"
			}
			if e.SemObj >= 0 {
				qtype = "sem"
			}
			d.Send(fmt.Sprintf("  [%d] %s player=%s(#%d) cmd=%s", i+1, qtype, name, e.Player, cmd))
		}
	}
}

// --- Softcode Commands ---

func cmdSwitch(g *Game, d *Descriptor, args string, _ []string) {
	// @switch expr = pattern1, action1 [, pattern2, action2, ...] [, default]
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @switch expression = pattern1, action1, ...")
		return
	}

	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})

	exprStr := strings.TrimSpace(args[:eqIdx])
	expr := ctx.Exec(exprStr, eval.EvFCheck|eval.EvEval, nil)

	rest := strings.TrimSpace(args[eqIdx+1:])
	parts := splitCommaRespectingBraces(rest)

	// Walk pattern/action pairs
	for i := 0; i+1 < len(parts); i += 2 {
		pattern := ctx.Exec(strings.TrimSpace(parts[i]), eval.EvFCheck|eval.EvEval, nil)
		if wildMatchSimple(strings.ToLower(pattern), strings.ToLower(expr)) {
			// In C TinyMUSH, do_switch dispatches the matched action body
			// to process_cmdline() — it does NOT evaluate it as an expression.
			// Strip braces, replace #$ with expr, dispatch as command(s).
			raw := stripOuterBraces(strings.TrimSpace(parts[i+1]))
			raw = strings.ReplaceAll(raw, "#$", expr)
			dispatchSwitchActionDesc(g, d, raw)
			return
		}
	}
	// Default (odd trailing entry)
	if len(parts)%2 == 1 {
		raw := stripOuterBraces(strings.TrimSpace(parts[len(parts)-1]))
		raw = strings.ReplaceAll(raw, "#$", expr)
		dispatchSwitchActionDesc(g, d, raw)
	}
}

// dispatchSwitchActionDesc executes a @switch action body for a connected player.
func dispatchSwitchActionDesc(g *Game, d *Descriptor, action string) {
	cmds := splitSemicolonRespectingBraces(action)
	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		cmd = stripOuterBraces(cmd)
		DispatchCommand(g, d, cmd)
	}
}

// --- Updated @set with flag support ---

// attrFlagNames maps attribute flag names to their bit values.
var attrFlagNames = map[string]int{
	"WIZARD":     gamedb.AFWizard,
	"DARK":       gamedb.AFDark,
	"MDARK":      gamedb.AFMDark,
	"VISUAL":     gamedb.AFVisual,
	"NO_COMMAND": gamedb.AFNoCMD,
	"NO_CLONE":   gamedb.AFNoClone,
	"PRIVATE":    gamedb.AFPrivate,
	"REGEXP":     gamedb.AFRegexp,
	"CASE":       gamedb.AFCase,
	"NOPARSE":    gamedb.AFNoParse,
	"GOD":        gamedb.AFGod,
	"NOPROG":     gamedb.AFNoProg,
	"ODARK":      gamedb.AFODark,
	"HTML":       gamedb.AFHTML,
	"NOW":        gamedb.AFNow,
}

// cmdSetVAttr handles the &ATTR obj=value shortcut (equivalent to @set obj=ATTR:value).
func cmdSetVAttr(g *Game, d *Descriptor, args string, _ []string) {
	// Input arrives with the & already stripped: "ATTR obj=value"
	// Split into attr name and "obj=value"
	spaceIdx := strings.IndexByte(args, ' ')
	if spaceIdx < 0 {
		d.Send("Usage: &ATTR object=value")
		return
	}
	attrName := strings.ToUpper(strings.TrimSpace(args[:spaceIdx]))
	rest := strings.TrimSpace(args[spaceIdx+1:])

	eqIdx := strings.IndexByte(rest, '=')
	if eqIdx < 0 {
		d.Send("Usage: &ATTR object=value")
		return
	}
	targetStr := strings.TrimSpace(rest[:eqIdx])
	value := strings.TrimSpace(rest[eqIdx+1:])

	if attrName == "" {
		d.Send("Usage: &ATTR object=value")
		return
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		d.Send("Permission denied.")
		return
	}

	ok, errMsg := g.SetAttrByNameChecked(d.Player, target, attrName, value)
	if !ok {
		d.Send(errMsg)
	} else {
		if value == "" {
			d.Send(fmt.Sprintf("%s - Cleared.", attrName))
		} else {
			d.Send("Set.")
		}
	}
}

// cmdEdit implements @edit obj/attr=search,replace
// Special search patterns: $ = append to end, ^ = prepend to start
// Escaped: \$ or \^ searches for literal $ or ^
func cmdEdit(g *Game, d *Descriptor, args string, _ []string) {
	// Parse obj/attr = search,replace
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @edit obj/attr = search, replace")
		return
	}
	objAttr := strings.TrimSpace(args[:eqIdx])
	rest := args[eqIdx+1:]

	slashIdx := strings.IndexByte(objAttr, '/')
	if slashIdx < 0 {
		d.Send("Usage: @edit obj/attr = search, replace")
		return
	}
	objStr := strings.TrimSpace(objAttr[:slashIdx])
	attrName := strings.TrimSpace(objAttr[slashIdx+1:])

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		d.Send("Permission denied.")
		return
	}

	// Parse search,replace respecting braces
	// The format is: search , replace
	// Braces protect commas: {foo,bar},{baz,qux}
	from, to := parseEditArgs(rest)

	// Handle escaped ^ and $ (search for literal)
	if len(from) == 2 && (from[0] == '\\' || from[0] == '%') && (from[1] == '$' || from[1] == '^') {
		from = from[1:]
	}

	// Resolve attr
	attrNum := g.LookupAttrNum(strings.ToUpper(attrName))
	if attrNum < 0 {
		d.Send(fmt.Sprintf("No such attribute: %s", attrName))
		return
	}

	// Get current value
	current := g.GetAttrTextDirect(target, attrNum)

	// Perform edit
	var result string
	switch from {
	case "$":
		result = current + to
	case "^":
		result = to + current
	default:
		result = strings.ReplaceAll(current, from, to)
	}

	g.SetAttr(target, attrNum, result)

	obj := g.DB.Objects[target]
	d.Send(fmt.Sprintf("Set - %s/%s: %s", obj.Name, strings.ToUpper(attrName), result))
}

// parseEditArgs splits "search,replace" respecting brace quoting.
// Returns (from, to). If only one part, to is empty.
func parseEditArgs(s string) (string, string) {
	// Only trim leading space before the search term, preserve the replacement as-is
	// This matches TinyMUSH behavior: @edit obj/attr=$, text  -> append " text"
	parts := splitEditComma(s)
	from := stripBraces(strings.TrimSpace(parts[0]))
	to := ""
	if len(parts) > 1 {
		to = stripBraces(parts[1])
	}
	return from, to
}

// splitEditComma splits on the first comma not inside braces.
func splitEditComma(s string) []string {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return []string{s[:i], s[i+1:]}
			}
		}
	}
	return []string{s}
}

// stripBraces removes one level of outer braces if present.
func stripBraces(s string) string {
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		return s[1 : len(s)-1]
	}
	return s
}

func cmdSet(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @set thing = attribute:value  or  @set thing = [!]flag")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	value := strings.TrimSpace(args[eqIdx+1:])

	// Check for per-attribute flag setting: @set obj/attr = [!]flagname
	if slashIdx := strings.IndexByte(targetStr, '/'); slashIdx >= 0 {
		objName := strings.TrimSpace(targetStr[:slashIdx])
		attrName := strings.ToUpper(strings.TrimSpace(targetStr[slashIdx+1:]))
		target := g.MatchObject(d.Player, objName)
		if target == gamedb.Nothing {
			d.Send("I don't see that here.")
			return
		}
		if !Controls(g, d.Player, target) {
			d.Send("Permission denied.")
			return
		}
		g.setAttrFlag(d, target, attrName, value)
		return
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}

	// Check for attr:value format
	if colonIdx := strings.IndexByte(value, ':'); colonIdx >= 0 {
		attrName := strings.ToUpper(strings.TrimSpace(value[:colonIdx]))
		attrValue := strings.TrimSpace(value[colonIdx+1:])
		if !Controls(g, d.Player, target) {
			d.Send("Permission denied.")
			return
		}
		ok, errMsg := g.SetAttrByNameChecked(d.Player, target, attrName, attrValue)
		if !ok {
			d.Send(errMsg)
		} else {
			d.Send("Set.")
		}
		return
	}

	// Flag setting
	if !Controls(g, d.Player, target) {
		d.Send("Permission denied.")
		return
	}
	if g.SetFlag(target, value) {
		d.Send("Set.")
	} else {
		d.Send("I don't know that flag.")
	}
}

// setAttrFlag handles @set obj/attr = [!]flagname — sets or clears an attribute flag.
func (g *Game) setAttrFlag(d *Descriptor, target gamedb.DBRef, attrName string, flagStr string) {
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}

	// Resolve attr number
	attrNum := -1
	if def, ok := g.DB.AttrByName[attrName]; ok {
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
		d.Send(fmt.Sprintf("No such attribute: %s", attrName))
		return
	}

	// Parse [!]flagname
	clearing := false
	fname := strings.TrimSpace(flagStr)
	if strings.HasPrefix(fname, "!") {
		clearing = true
		fname = strings.TrimSpace(fname[1:])
	}
	fname = strings.ToUpper(fname)

	bit, ok2 := attrFlagNames[fname]
	if !ok2 {
		d.Send(fmt.Sprintf("Unknown attribute flag: %s", fname))
		return
	}

	// AF_GOD and AF_WIZARD flags require special permissions
	if bit == gamedb.AFGod && !IsGod(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if bit == gamedb.AFWizard && !SetsWizAttrs(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	// Find the attribute and modify its flags
	for i, attr := range obj.Attrs {
		if attr.Number == attrNum {
			info := ParseAttrInfo(attr.Value)
			text := eval.StripAttrPrefix(attr.Value)
			owner := info.Owner
			if owner == gamedb.Nothing {
				owner = obj.Owner
			}
			if clearing {
				info.Flags &^= bit
			} else {
				info.Flags |= bit
			}
			obj.Attrs[i].Value = fmt.Sprintf("\x01%d:%d:%s", owner, info.Flags, text)
			g.PersistObject(obj)
			d.Send("Set.")
			return
		}
	}
	d.Send(fmt.Sprintf("No such attribute: %s", attrName))
}

// SetAttrByNameChecked sets an attribute by name with permission enforcement.
func (g *Game) SetAttrByNameChecked(player, obj gamedb.DBRef, attrName string, value string) (bool, string) {
	// Look up attr number
	attrNum := -1
	for num, name := range gamedb.WellKnownAttrs {
		if strings.EqualFold(name, attrName) {
			attrNum = num
			break
		}
	}
	if attrNum < 0 {
		if def, ok := g.DB.AttrByName[attrName]; ok {
			attrNum = def.Number
		}
	}
	if attrNum < 0 {
		// New attr — create it; permission check is just Controls (already done by caller)
		g.SetAttrByName(obj, attrName, value)
		return true, ""
	}
	return g.SetAttrChecked(player, obj, attrNum, value)
}

// --- Helper methods on Game ---

// Controls returns true if the player controls the target.
// Delegates to the full permission model in perms.go.
func (g *Game) Controls(player, target gamedb.DBRef) bool {
	return Controls(g, player, target)
}

// --- Utility ---

// wildMatchSimple is a simple glob matcher for internal use.
func wildMatchSimple(pattern, str string) bool {
	return matchSimple(pattern, str)
}

func matchSimple(pattern, str string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			for i := len(str); i >= 0; i-- {
				if matchSimple(pattern[1:], str[i:]) {
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

// splitCommaRespectingBraces splits on commas but respects {} nesting.
// splitCommaRespectingBraces splits on commas at brace depth 0.
// Only {/} affect depth — [/]/(/), are NOT tracked, matching C TinyMUSH's
// parse_to behavior. C uses a stack for [/( that tolerates unmatched brackets
// (e.g., *[* in glob patterns). Our simple depth counter can't replicate that,
// so we only track braces which are always properly balanced in MUSH code.
func splitCommaRespectingBraces(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{', '(':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
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

// --- @dump command ---

func cmdDump(g *Game, d *Descriptor, args string, _ []string) {
	// Only wizards can dump
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	d.Send("WARNING: @dump is deprecated. Use @archive for full game backups.")

	path := g.DBPath
	if path == "" {
		path = "game.flatfile"
	}

	d.Send("Saving database...")
	go func() {
		if err := flatfile.Save(path, g.DB); err != nil {
			log.Printf("ERROR: Database save failed: %v", err)
			g.Conns.SendToPlayer(d.Player, fmt.Sprintf("Save failed: %v", err))
		} else {
			log.Printf("Database saved to %s (%d objects)", path, len(g.DB.Objects))
			g.Conns.SendToPlayer(d.Player, fmt.Sprintf("Save complete. %d objects written to %s.", len(g.DB.Objects), path))
		}
	}()
}

// --- @backup command ---

func cmdBackup(g *Game, d *Descriptor, args string, _ []string) {
	// Only wizards can backup
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	if g.Store == nil {
		d.Send("No bolt database configured. Use -bolt flag to enable.")
		return
	}

	path := args
	if path == "" {
		path = fmt.Sprintf("game-backup-%s.bolt", time.Now().Format("20060102-150405"))
	}

	d.Send(fmt.Sprintf("Backing up database to %s...", path))
	go func() {
		if err := g.Store.Backup(path); err != nil {
			log.Printf("ERROR: Backup failed: %v", err)
			g.Conns.SendToPlayer(d.Player, fmt.Sprintf("Backup failed: %v", err))
		} else {
			log.Printf("Backup complete: %s", path)
			g.Conns.SendToPlayer(d.Player, fmt.Sprintf("Backup complete: %s", path))
		}
	}()
}

// --- @dolist command ---

func cmdDolist(g *Game, d *Descriptor, args string, switches []string) {
	// @dolist <list> = <command>
	// @dolist/delimit <sep> <list> = <command>
	// ## in command is replaced with current element
	// #@ is the iteration number (1-based)
	delim := "" // empty = split on whitespace
	if HasSwitch(switches, "delimit") {
		// First space-delimited token in args is the delimiter
		spIdx := strings.IndexByte(strings.TrimSpace(args), ' ')
		if spIdx > 0 {
			trimmed := strings.TrimSpace(args)
			delim = trimmed[:spIdx]
			args = strings.TrimSpace(trimmed[spIdx+1:])
		}
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @dolist <list> = <command>")
		return
	}

	listStr := strings.TrimSpace(args[:eqIdx])
	command := strings.TrimSpace(args[eqIdx+1:])

	if listStr == "" || command == "" {
		d.Send("Usage: @dolist <list> = <command>")
		return
	}

	// Evaluate the list
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	listStr = ctx.Exec(listStr, eval.EvFCheck|eval.EvEval, nil)

	// Split into elements
	var elements []string
	if delim != "" {
		elements = strings.Split(listStr, delim)
	} else {
		elements = strings.Fields(listStr)
	}

	immediate := HasSwitch(switches, "now")

	// Queue or execute each iteration
	for i, elem := range elements {
		// Replace ## with current element and #@ with iteration number
		cmd := strings.ReplaceAll(command, "##", elem)
		cmd = strings.ReplaceAll(cmd, "#@", fmt.Sprintf("%d", i+1))

		if immediate {
			// Execute immediately via DispatchCommand
			DispatchCommand(g, d, cmd)
		} else {
			entry := &QueueEntry{
				Player:  d.Player,
				Cause:   d.Player,
				Caller:  d.Player,
				Command: cmd,
			}
			g.Queue.Add(entry)
		}
	}
}

// --- Communication Commands ---

func cmdOemit(g *Game, d *Descriptor, args string, _ []string) {
	// @oemit target = message — emits to target's room, excluding target
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @oemit target = message")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	message := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}

	loc := g.PlayerLocation(target)
	if loc == gamedb.Nothing {
		loc = g.PlayerLocation(d.Player)
	}
	message = evalExpr(g, d.Player, message)
	g.SendMarkedToRoomExcept(loc, target, "EMIT", message)
}

func cmdRemit(g *Game, d *Descriptor, args string, _ []string) {
	// @remit room = message
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @remit room = message")
		return
	}
	roomStr := strings.TrimSpace(args[:eqIdx])
	message := strings.TrimSpace(args[eqIdx+1:])

	room := g.ResolveRef(d.Player, roomStr)
	if room == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	message = evalExpr(g, d.Player, message)
	g.SendMarkedToRoom(room, "EMIT", message)
}

// --- Builder/Admin Utilities ---

func cmdPassword(g *Game, d *Descriptor, args string, _ []string) {
	// @password old = new
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @password old = new")
		return
	}
	oldPass := strings.TrimSpace(args[:eqIdx])
	newPass := strings.TrimSpace(args[eqIdx+1:])

	if oldPass == "" || newPass == "" {
		d.Send("You must specify both old and new passwords.")
		return
	}

	// Verify old password
	currentHash := g.GetAttrText(d.Player, aPass)
	if currentHash == "" {
		d.Send("You don't have a password set.")
		return
	}
	check := mushcrypt.Crypt(oldPass, currentHash[:2])
	if check != currentHash {
		d.Send("Sorry.")
		return
	}

	// Set new password
	hash := mushcrypt.Crypt(newPass, "XX")
	g.SetAttr(d.Player, aPass, hash)
	d.Send("Password changed.")
}

func cmdVersion(g *Game, d *Descriptor, _ string, _ []string) {
	d.Send(VersionString())
}

func cmdMotd(g *Game, d *Descriptor, args string, switches []string) {
	if HasSwitch(switches, "wizard") {
		if !Wizard(g, d.Player) { d.Send("Permission denied."); return }
		if args == "" {
			if g.WizMOTD != "" { d.Send(g.WizMOTD) } else { d.Send("No wizard MOTD set.") }
		} else {
			g.WizMOTD = args
			d.Send("Wizard MOTD set.")
		}
		return
	}
	if HasSwitch(switches, "down") {
		if !Wizard(g, d.Player) { d.Send("Permission denied."); return }
		if args == "" {
			if g.DownMOTD != "" { d.Send(g.DownMOTD) } else { d.Send("No down MOTD set.") }
		} else {
			g.DownMOTD = args
			d.Send("Down MOTD set.")
		}
		return
	}
	if HasSwitch(switches, "full") {
		if !Wizard(g, d.Player) { d.Send("Permission denied."); return }
		if args == "" {
			if g.FullMOTD != "" { d.Send(g.FullMOTD) } else { d.Send("No full MOTD set.") }
		} else {
			g.FullMOTD = args
			d.Send("Full MOTD set.")
		}
		return
	}

	if args == "" {
		// Show current MOTD
		if g.MOTD != "" {
			d.Send(g.MOTD)
		} else if g.Texts != nil {
			motd := g.Texts.GetMotd()
			if motd != "" {
				d.Send(motd)
			} else {
				d.Send("No message of the day.")
			}
		} else {
			d.Send("No message of the day.")
		}
		return
	}
	// Wizard-only: set MOTD
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	g.MOTD = args
	d.Send("MOTD set.")
}

func cmdChzone(g *Game, d *Descriptor, args string, switches []string) {
	// @chzone obj = zone
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @chzone object = zone")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	zoneStr := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}

	zone := gamedb.Nothing
	if zoneStr != "" && !strings.EqualFold(zoneStr, "none") {
		zone = g.ResolveRef(d.Player, zoneStr)
		if zone == gamedb.Nothing {
			d.Send("I don't see that zone.")
			return
		}

		// Validate zone type: must be THING or ROOM
		zoneObj, zOk := g.DB.Objects[zone]
		if !zOk {
			d.Send("No such zone object.")
			return
		}
		zoneType := zoneObj.ObjType()
		if zoneType != gamedb.TypeThing && zoneType != gamedb.TypeRoom {
			d.Send("Invalid zone object type.")
			return
		}

		// Room-to-room restriction: only rooms may be zoned to rooms
		if zoneType == gamedb.TypeRoom && targetObj.ObjType() != gamedb.TypeRoom {
			d.Send("Only rooms may be zoned to parent rooms.")
			return
		}
	}

	// Permission check on target:
	// Wizard, Controls, CheckZoneForPlayer, or same owner
	if !Wizard(g, d.Player) &&
		!Controls(g, d.Player, target) &&
		!CheckZoneForPlayer(g, d.Player, target, 0) &&
		targetObj.Owner != d.Player {
		d.Send("Permission denied.")
		return
	}

	// Permission check on new zone (if setting, not clearing)
	if zone != gamedb.Nothing {
		zoneObj := g.DB.Objects[zone]
		if !Wizard(g, d.Player) &&
			!Controls(g, d.Player, zone) &&
			zoneObj.Owner != d.Player {
			d.Send("Permission denied.")
			return
		}
	}

	// Set the zone
	targetObj.Zone = zone
	g.PersistObject(targetObj)

	// Flag stripping
	if HasSwitch(switches, "nostrip") && Wizard(g, d.Player) {
		// /nostrip (wizard-only): only strip WIZARD (unless God)
		if !IsGod(g, d.Player) && targetObj.ObjType() != gamedb.TypePlayer {
			targetObj.Flags[0] &^= gamedb.FlagWizard
			g.PersistObject(targetObj)
		}
	} else {
		StripPrivFlags(g, target)
	}

	if zone == gamedb.Nothing {
		d.Send(fmt.Sprintf("Zone of %s(#%d) cleared.", targetObj.Name, target))
	} else {
		d.Send(fmt.Sprintf("Zone of %s(#%d) set to %s(#%d).", targetObj.Name, target, g.ObjName(zone), zone))
	}
}

func cmdSearch(g *Game, d *Descriptor, args string, _ []string) {
	// @search [type=TYPE] [name=PATTERN]
	var typeFilter gamedb.ObjectType = -1
	var namePattern string

	for _, part := range strings.Fields(args) {
		if eqIdx := strings.IndexByte(part, '='); eqIdx >= 0 {
			key := strings.ToLower(part[:eqIdx])
			val := part[eqIdx+1:]
			switch key {
			case "type":
				switch strings.ToLower(val) {
				case "room", "rooms":
					typeFilter = gamedb.TypeRoom
				case "thing", "things":
					typeFilter = gamedb.TypeThing
				case "exit", "exits":
					typeFilter = gamedb.TypeExit
				case "player", "players":
					typeFilter = gamedb.TypePlayer
				}
			case "name":
				namePattern = strings.ToLower(val)
			}
		} else if namePattern == "" {
			namePattern = strings.ToLower(part)
		}
	}

	count := 0
	for _, obj := range g.DB.Objects {
		if obj.IsGoing() {
			continue
		}
		if typeFilter >= 0 && obj.ObjType() != typeFilter {
			continue
		}
		if namePattern != "" && !wildMatchSimple(namePattern, strings.ToLower(obj.Name)) {
			continue
		}
		// Only show objects the player owns (or all if wizard)
		if !g.Controls(d.Player, obj.DBRef) {
			continue
		}
		d.Send(fmt.Sprintf("  %s(#%d%s)", obj.Name, obj.DBRef, typeChar(obj.ObjType())))
		count++
		if count >= 200 {
			d.Send("*** Too many results, truncated ***")
			break
		}
	}
	d.Send(fmt.Sprintf("%d object(s) found.", count))
}

func cmdDecompile(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		d.Send("Decompile what?")
		return
	}
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}

	ref := fmt.Sprintf("#%d", target)

	switch obj.ObjType() {
	case gamedb.TypeRoom:
		d.Send(fmt.Sprintf("@dig %s", obj.Name))
	case gamedb.TypeThing:
		d.Send(fmt.Sprintf("@create %s", obj.Name))
	case gamedb.TypeExit:
		d.Send(fmt.Sprintf("@open %s", obj.Name))
	case gamedb.TypePlayer:
		// Can't recreate players via decompile, just show attrs
	}

	// Description
	desc := g.GetAttrText(target, 6)
	if desc != "" {
		d.Send(fmt.Sprintf("@describe %s=%s", ref, desc))
	}

	// Show user-set attributes
	for _, attr := range obj.Attrs {
		name := g.DB.GetAttrName(attr.Number)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", attr.Number)
		}
		text := eval.StripAttrPrefix(attr.Value)
		// Skip internal/sensitive attrs
		if isInternalAttr(attr.Number) {
			continue
		}
		// Check if there's a known @command for this attr
		switch attr.Number {
		case 6: // A_DESC — already handled above
			continue
		case 4:
			d.Send(fmt.Sprintf("@success %s=%s", ref, text))
		case 1:
			d.Send(fmt.Sprintf("@osuccess %s=%s", ref, text))
		case 3:
			d.Send(fmt.Sprintf("@fail %s=%s", ref, text))
		case 2:
			d.Send(fmt.Sprintf("@ofail %s=%s", ref, text))
		case 9:
			d.Send(fmt.Sprintf("@drop %s=%s", ref, text))
		case 8:
			d.Send(fmt.Sprintf("@odrop %s=%s", ref, text))
		case 7:
			d.Send(fmt.Sprintf("@sex %s=%s", ref, text))
		default:
			d.Send(fmt.Sprintf("@set %s=%s:%s", ref, name, text))
		}
	}

	// Flags
	if obj.HasFlag(gamedb.FlagDark) {
		d.Send(fmt.Sprintf("@set %s=DARK", ref))
	}
	if obj.HasFlag(gamedb.FlagHaven) {
		d.Send(fmt.Sprintf("@set %s=HAVEN", ref))
	}
	if obj.HasFlag(gamedb.FlagQuiet) {
		d.Send(fmt.Sprintf("@set %s=QUIET", ref))
	}
	if obj.HasFlag(gamedb.FlagSafe) {
		d.Send(fmt.Sprintf("@set %s=SAFE", ref))
	}
	if obj.HasFlag(gamedb.FlagEnterOK) {
		d.Send(fmt.Sprintf("@set %s=ENTER_OK", ref))
	}
	if obj.HasFlag(gamedb.FlagVisual) {
		d.Send(fmt.Sprintf("@set %s=VISUAL", ref))
	}
	if obj.HasFlag(gamedb.FlagPuppet) {
		d.Send(fmt.Sprintf("@set %s=PUPPET", ref))
	}
	if obj.HasFlag(gamedb.FlagSticky) {
		d.Send(fmt.Sprintf("@set %s=STICKY", ref))
	}

	// Parent
	if obj.Parent != gamedb.Nothing {
		d.Send(fmt.Sprintf("@parent %s=#%d", ref, obj.Parent))
	}
}

// StartAutoSave starts a periodic auto-save goroutine.
func (g *Game) StartAutoSave(intervalMinutes int) {
	if intervalMinutes < 1 {
		intervalMinutes = 30
	}
	go func() {
		ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if g.DBPath == "" {
				continue
			}
			log.Printf("Auto-saving database...")
			if err := flatfile.Save(g.DBPath, g.DB); err != nil {
				log.Printf("ERROR: Auto-save failed: %v", err)
			} else {
				log.Printf("Auto-save complete: %d objects", len(g.DB.Objects))
			}
		}
	}()
}

// --- @power command ---

// powerEntry maps a power name to its word index and bit.
type powerEntry struct {
	Word int
	Bit  int
}

// powerTable maps power name strings to their (word, bit) pairs.
var powerTable = map[string]powerEntry{
	"change_quotas":  {0, gamedb.PowChgQuotas},
	"chown_anything": {0, gamedb.PowChownAny},
	"announce":       {0, gamedb.PowAnnounce},
	"boot":           {0, gamedb.PowBoot},
	"halt":           {0, gamedb.PowHalt},
	"control_all":    {0, gamedb.PowControlAll},
	"wizard_who":     {0, gamedb.PowWizardWho},
	"see_all":        {0, gamedb.PowExamAll},
	"find_unfindable": {0, gamedb.PowFindUnfind},
	"free_money":     {0, gamedb.PowFreeMoney},
	"free_quota":     {0, gamedb.PowFreeQuota},
	"hide":           {0, gamedb.PowHide},
	"idle":           {0, gamedb.PowIdle},
	"search":         {0, gamedb.PowSearch},
	"long_fingers":   {0, gamedb.PowLongfingers},
	"prog":           {0, gamedb.PowProg},
	"mdark_attr":     {0, gamedb.PowMdarkAttr},
	"wiz_attr":       {0, gamedb.PowWizAttr},
	"comm_all":       {0, gamedb.PowCommAll},
	"see_queue":      {0, gamedb.PowSeeQueue},
	"see_hidden":     {0, gamedb.PowSeeHidden},
	"watch":          {0, gamedb.PowWatch},
	"poll":           {0, gamedb.PowPoll},
	"no_destroy":     {0, gamedb.PowNoDestroy},
	"guest":          {0, gamedb.PowGuest},
	"pass_locks":     {0, gamedb.PowPassLocks},
	"stat_any":       {0, gamedb.PowStatAny},
	"steal":          {0, gamedb.PowSteal},
	"tel_anywhere":   {0, gamedb.PowTelAnywhr},
	"tel_unrestricted": {0, gamedb.PowTelUnrst},
	"unkillable":     {0, gamedb.PowUnkillable},
	"builder":        {1, gamedb.Pow2Builder},
	"link_variable":  {1, gamedb.Pow2LinkVar},
	"link_to_anything": {1, gamedb.Pow2LinkToAny},
	"open_anywhere":  {1, gamedb.Pow2OpenAnyLoc},
	"use_sql":        {1, gamedb.Pow2UseSQL},
	"link_any_home":  {1, gamedb.Pow2LinkHome},
	"cloak":          {1, gamedb.Pow2Cloak},
}

// --- SQL Commands ---

func cmdSQL(g *Game, d *Descriptor, args string, _ []string) {
	// @sql <query> — Wizard-only interactive query tool
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if args == "" {
		d.Send("Usage: @sql <query>")
		return
	}
	if g.SQLDB == nil {
		d.Send("SQL is not configured.")
		return
	}

	trimmed := strings.TrimSpace(args)
	upper := strings.ToUpper(trimmed)

	if strings.HasPrefix(upper, "SELECT") {
		// SELECT: show row-by-row field display
		result, err := g.SQLDB.Query(trimmed, "\n", "\x01")
		if err != nil {
			d.Send(fmt.Sprintf("SQL error: %s", err))
			return
		}
		if result == "" {
			d.Send("No rows returned.")
			return
		}
		rows := strings.Split(result, "\n")
		for i, row := range rows {
			fields := strings.Split(row, "\x01")
			for j, field := range fields {
				d.Send(fmt.Sprintf("Row %d, Field %d: %s", i+1, j+1, field))
			}
		}
		d.Send(fmt.Sprintf("%d row(s) returned.", len(rows)))
	} else {
		// Non-SELECT
		result, err := g.SQLDB.Query(trimmed, " ", " ")
		if err != nil {
			d.Send(fmt.Sprintf("SQL error: %s", err))
			return
		}
		d.Send(fmt.Sprintf("SQL query touched %s row(s).", result))
	}
}

func cmdSQLInit(g *Game, d *Descriptor, _ string, _ []string) {
	// @sqlinit — God-only, re-opens SQL connection
	if !IsGod(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if g.SQLDB == nil {
		d.Send("SQL is not configured.")
		return
	}
	if err := g.SQLDB.Reconnect(); err != nil {
		d.Send(fmt.Sprintf("SQL reconnect failed: %s", err))
		return
	}
	d.Send("SQL connection re-initialized.")
}

func cmdSQLDisconnect(g *Game, d *Descriptor, _ string, _ []string) {
	// @sqldisconnect — God-only, closes SQL connection
	if !IsGod(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if g.SQLDB == nil {
		d.Send("SQL is not configured.")
		return
	}
	if err := g.SQLDB.Close(); err != nil {
		d.Send(fmt.Sprintf("SQL disconnect failed: %s", err))
		return
	}
	g.SQLDB = nil
	d.Send("SQL connection closed.")
}

func cmdPower(g *Game, d *Descriptor, args string, _ []string) {
	// @power obj = [!]powername
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @power object = [!]power")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	powStr := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		d.Send("No such object.")
		return
	}

	// God protection
	if IsGod(g, target) && !IsGod(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	// Parse [!]powername
	negate := false
	if strings.HasPrefix(powStr, "!") {
		negate = true
		powStr = strings.TrimSpace(powStr[1:])
	}
	powName := strings.ToLower(powStr)

	pe, ok := powerTable[powName]
	if !ok {
		d.Send("I don't know that power.")
		return
	}

	obj.SetPower(pe.Word, pe.Bit, !negate)
	g.PersistObject(obj)
	if negate {
		d.Send(fmt.Sprintf("Power %s removed from %s(#%d).", powStr, obj.Name, target))
	} else {
		d.Send(fmt.Sprintf("Power %s granted to %s(#%d).", powStr, obj.Name, target))
	}
}

// cmdFunction implements @function[/privileged][/preserve][/delete] name=obj/attr
// Registers a global softcode-defined function.
func cmdFunction(g *Game, d *Descriptor, args string, switches []string) {
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	// Parse switches
	privileged := false
	preserve := false
	doDelete := false
	for _, sw := range switches {
		switch strings.ToLower(sw) {
		case "privileged", "priv":
			privileged = true
		case "preserve", "pres":
			preserve = true
		case "delete", "del":
			doDelete = true
		}
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		if doDelete {
			// @function/delete name
			funcName := strings.ToUpper(strings.TrimSpace(args))
			if funcName == "" {
				d.Send("Usage: @function/delete <name>")
				return
			}
			if _, ok := g.GameFuncs[funcName]; ok {
				delete(g.GameFuncs, funcName)
				d.Send(fmt.Sprintf("Function %s deleted.", funcName))
			} else {
				d.Send(fmt.Sprintf("No @function named %s.", funcName))
			}
			return
		}
		// List all @functions
		if len(g.GameFuncs) == 0 {
			d.Send("No @functions defined.")
			return
		}
		for name, uf := range g.GameFuncs {
			flags := ""
			if uf.Flags&eval.UfPriv != 0 {
				flags += " privileged"
			}
			if uf.Flags&eval.UfPres != 0 {
				flags += " preserve"
			}
			d.Send(fmt.Sprintf("  %s = #%d/%d%s", name, uf.Obj, uf.Attr, flags))
		}
		return
	}

	funcName := strings.ToUpper(strings.TrimSpace(args[:eqIdx]))
	objAttr := strings.TrimSpace(args[eqIdx+1:])

	if funcName == "" {
		d.Send("Usage: @function[/privileged] <name> = <obj>/<attr>")
		return
	}

	// Handle deletion via empty value
	if objAttr == "" {
		if _, ok := g.GameFuncs[funcName]; ok {
			delete(g.GameFuncs, funcName)
			d.Send(fmt.Sprintf("Function %s deleted.", funcName))
		} else {
			d.Send(fmt.Sprintf("No @function named %s.", funcName))
		}
		return
	}

	// Parse obj/attr
	slashIdx := strings.IndexByte(objAttr, '/')
	if slashIdx < 0 {
		d.Send("Usage: @function[/privileged] <name> = <obj>/<attr>")
		return
	}
	objStr := strings.TrimSpace(objAttr[:slashIdx])
	attrName := strings.ToUpper(strings.TrimSpace(objAttr[slashIdx+1:]))

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}

	// Resolve attr number
	attrNum := g.LookupAttrNum(attrName)
	if attrNum < 0 {
		d.Send(fmt.Sprintf("No such attribute: %s", attrName))
		return
	}

	flags := 0
	if privileged {
		flags |= eval.UfPriv
	}
	if preserve {
		flags |= eval.UfPres
	}

	uf := &eval.UFunction{
		Name:  funcName,
		Obj:   target,
		Attr:  attrNum,
		Flags: flags,
	}
	g.GameFuncs[funcName] = uf
	log.Printf("@function %s = #%d/%s (flags=%d)", funcName, target, attrName, flags)
	d.Send(fmt.Sprintf("Function %s defined.", funcName))
}

// cmdDrain implements @drain <obj>[/<attr>]
// Removes all wait queue entries belonging to the object, and resets its semaphore count.
func cmdDrain(g *Game, d *Descriptor, args string, _ []string) {
	args = strings.TrimSpace(args)
	if args == "" {
		d.Send("Usage: @drain <object>")
		return
	}

	// Parse obj/attr if present
	var objStr, attrName string
	if slashIdx := strings.IndexByte(args, '/'); slashIdx >= 0 {
		objStr = strings.TrimSpace(args[:slashIdx])
		attrName = strings.ToUpper(strings.TrimSpace(args[slashIdx+1:]))
	} else {
		objStr = args
	}

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		d.Send("I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		d.Send("Permission denied.")
		return
	}

	// Drain semaphore entries from the queue
	semAttr := 47 // A_SEMAPHORE = 47
	if attrName != "" {
		num := g.LookupAttrNum(attrName)
		if num >= 0 {
			semAttr = num
		}
	}

	count := g.Queue.DrainObject(target, semAttr)

	// Reset the semaphore count on the object
	if attrName == "" {
		g.SetAttr(target, 47, "") // Clear A_SEMAPHORE = 47
	}

	d.Send(fmt.Sprintf("Drained %d entries from %s.", count, objStr))
}

// --- Archive Commands ---

// cmdArchive implements @archive and @archive/list.
func cmdArchive(g *Game, d *Descriptor, args string, switches []string) {
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	if HasSwitch(switches, "list") {
		cmdArchiveList(g, d)
		return
	}

	archiveDir := g.ArchiveDir
	if archiveDir == "" {
		archiveDir = "backups"
	}

	mudName := "GoTinyMUSH"
	if g.Conf != nil && g.Conf.MudName != "" {
		mudName = g.Conf.MudName
	}

	params := archive.ArchiveParams{
		ArchiveDir:  archiveDir,
		MudName:     mudName,
		ObjectCount: len(g.DB.Objects),
		DictDir:     g.DictDir,
		TextDir:     g.TextDir,
		ConfPath:    g.ConfPath,
		AliasConfs:  g.AliasConfs,
	}

	// Bolt snapshot closure
	if g.Store != nil {
		params.BoltSnapshotFunc = func(dest string) error {
			return g.Store.Backup(dest)
		}
	}

	// SQL checkpoint + path
	if g.SQLDB != nil {
		params.SQLPath = g.SQLDB.Path()
		params.SQLCheckpointFunc = func() error {
			return g.SQLDB.Checkpoint()
		}
	}

	d.Send("Creating archive...")
	go func() {
		archivePath, err := archive.CreateArchive(params)
		if err != nil {
			log.Printf("ERROR: Archive failed: %v", err)
			g.Conns.SendToPlayer(d.Player, fmt.Sprintf("Archive failed: %v", err))
			return
		}
		log.Printf("Archive created: %s", archivePath)
		g.Conns.SendToPlayer(d.Player, fmt.Sprintf("Archive created: %s", archivePath))

		// Prune old archives
		retain := 0
		if g.Conf != nil {
			retain = g.Conf.ArchiveRetain
		}
		if retain > 0 {
			pruneArchives(archiveDir, retain)
		}

		// Run post-archive hook
		if hook := g.archiveHook(); hook != "" {
			runArchiveHook(hook, archivePath)
		}
	}()
}

// cmdArchiveList implements @archive/list.
func cmdArchiveList(g *Game, d *Descriptor) {
	archiveDir := g.ArchiveDir
	if archiveDir == "" {
		archiveDir = "backups"
	}

	archives, err := archive.ListArchives(archiveDir)
	if err != nil {
		d.Send(fmt.Sprintf("Error listing archives: %v", err))
		return
	}
	if len(archives) == 0 {
		d.Send(fmt.Sprintf("No archives found in %s.", archiveDir))
		return
	}

	d.Send(fmt.Sprintf("Archives in %s:", archiveDir))
	for _, ai := range archives {
		sizeMB := float64(ai.Size) / (1024 * 1024)
		if ai.Objects > 0 {
			d.Send(fmt.Sprintf("  %s  %.1f MB  %d objects  %s", ai.Filename, sizeMB, ai.Objects, ai.Timestamp))
		} else {
			d.Send(fmt.Sprintf("  %s  %.1f MB  %s", ai.Filename, sizeMB, ai.Timestamp))
		}
	}
	d.Send(fmt.Sprintf("%d archive(s).", len(archives)))
}

// StartAutoArchive starts a periodic archive goroutine.
func (g *Game) StartAutoArchive(intervalMinutes int) {
	if intervalMinutes < 1 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			archiveDir := g.ArchiveDir
			if archiveDir == "" {
				archiveDir = "backups"
			}

			mudName := "GoTinyMUSH"
			if g.Conf != nil && g.Conf.MudName != "" {
				mudName = g.Conf.MudName
			}

			params := archive.ArchiveParams{
				ArchiveDir:  archiveDir,
				MudName:     mudName,
				ObjectCount: len(g.DB.Objects),
				DictDir:     g.DictDir,
				TextDir:     g.TextDir,
				ConfPath:    g.ConfPath,
				AliasConfs:  g.AliasConfs,
			}
			if g.Store != nil {
				params.BoltSnapshotFunc = func(dest string) error {
					return g.Store.Backup(dest)
				}
			}
			if g.SQLDB != nil {
				params.SQLPath = g.SQLDB.Path()
				params.SQLCheckpointFunc = func() error {
					return g.SQLDB.Checkpoint()
				}
			}

			log.Printf("Auto-archive starting...")
			archivePath, err := archive.CreateArchive(params)
			if err != nil {
				log.Printf("ERROR: Auto-archive failed: %v", err)
				continue
			}
			log.Printf("Auto-archive complete: %s", archivePath)

			retain := 0
			if g.Conf != nil {
				retain = g.Conf.ArchiveRetain
			}
			if retain > 0 {
				pruneArchives(archiveDir, retain)
			}

			if hook := g.archiveHook(); hook != "" {
				runArchiveHook(hook, archivePath)
			}
		}
	}()
}

// pruneArchives deletes old archives beyond the keep count.
func pruneArchives(dir string, keep int) {
	if keep <= 0 {
		return
	}
	archives, err := archive.ListArchives(dir)
	if err != nil {
		log.Printf("WARNING: prune archives: %v", err)
		return
	}
	if len(archives) <= keep {
		return
	}
	for _, ai := range archives[keep:] {
		if err := os.Remove(ai.Path); err != nil {
			log.Printf("WARNING: prune archive %s: %v", ai.Filename, err)
		} else {
			log.Printf("Pruned old archive: %s", ai.Filename)
		}
	}
}

// runArchiveHook runs a shell command after archive creation.
// %f in the command is replaced with the archive path.
func runArchiveHook(command, archivePath string) {
	command = strings.ReplaceAll(command, "%f", archivePath)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("WARNING: archive hook failed: %v (output: %s)", err, string(output))
	} else {
		log.Printf("Archive hook completed: %s", strings.TrimSpace(string(output)))
	}
}

// cmdAdmin implements @admin param=value for runtime configuration.
// Wizard-only. Maps TinyMUSH config param names to GameConf fields.
func cmdAdmin(g *Game, d *Descriptor, args string, _ []string) {
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	if g.Conf == nil {
		d.Send("No game configuration loaded.")
		return
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		// Show a param value
		param := strings.TrimSpace(args)
		if param == "" {
			d.Send("Usage: @admin param=value")
			return
		}
		val, ok := getAdminParam(g.Conf, param)
		if !ok {
			d.Send(fmt.Sprintf("Unknown parameter: %s", param))
			return
		}
		d.Send(fmt.Sprintf("%s = %s", param, val))
		return
	}

	param := strings.TrimSpace(args[:eqIdx])
	value := strings.TrimSpace(args[eqIdx+1:])

	ok := setAdminParam(g.Conf, param, value)
	if !ok {
		d.Send(fmt.Sprintf("Unknown parameter: %s", param))
		return
	}
	d.Send(fmt.Sprintf("Set: %s = %s", param, value))
	log.Printf("@admin: %s set %s = %s", g.DB.Objects[d.Player].Name, param, value)
}

// adminParamMap maps TinyMUSH @admin parameter names to get/set closures.
func getAdminParam(c *GameConf, param string) (string, bool) {
	param = strings.ToLower(strings.TrimSpace(param))
	switch param {
	case "paycheck":
		return strconv.Itoa(c.Paycheck), true
	case "money_name_singular":
		return c.MoneyNameSingular, true
	case "money_name_plural":
		return c.MoneyNamePlural, true
	case "starting_money":
		return strconv.Itoa(c.StartingMoney), true
	case "earn_limit":
		return strconv.Itoa(c.EarnLimit), true
	case "page_cost":
		return strconv.Itoa(c.PageCost), true
	case "wait_cost":
		return strconv.Itoa(c.WaitCost), true
	case "link_cost":
		return strconv.Itoa(c.LinkCost), true
	case "machine_command_cost":
		return strconv.Itoa(c.MachineCommandCost), true
	case "trace_topdown":
		if c.TraceTopdown { return "1", true }
		return "0", true
	case "trace_output_limit":
		return strconv.Itoa(c.TraceOutputLimit), true
	case "idle_timeout":
		return strconv.Itoa(c.IdleTimeout), true
	case "output_limit":
		return strconv.Itoa(c.OutputLimit), true
	case "function_invocation_limit":
		return strconv.Itoa(c.FunctionInvocationLimit), true
	case "queue_idle_chunk":
		return strconv.Itoa(c.QueueIdleChunk), true
	case "mud_name":
		return c.MudName, true
	case "master_room":
		return strconv.Itoa(c.MasterRoom), true
	case "player_starting_room":
		return strconv.Itoa(c.PlayerStartingRoom), true
	case "player_starting_home":
		return strconv.Itoa(c.PlayerStartingHome), true
	case "default_home":
		return strconv.Itoa(c.DefaultHome), true
	case "switch_default_all":
		if c.SwitchDefaultAll { return "1", true }
		return "0", true
	case "pemit_far_players":
		if c.PemitFarPlayers { return "1", true }
		return "0", true
	case "pemit_any_object":
		if c.PemitAnyObject { return "1", true }
		return "0", true
	case "public_flags":
		if c.PublicFlags { return "1", true }
		return "0", true
	case "examine_public_attrs":
		if c.ExaminePublicAttrs { return "1", true }
		return "0", true
	case "read_remote_name":
		if c.ReadRemoteName { return "1", true }
		return "0", true
	case "debug":
		if IsDebug() { return "1", true }
		return "0", true
	default:
		return "", false
	}
}

func setAdminParam(c *GameConf, param, value string) bool {
	param = strings.ToLower(strings.TrimSpace(param))
	// Handle negation: @admin log=!all_commands -> strip ! prefix
	negate := false
	if strings.HasPrefix(value, "!") {
		negate = true
		value = value[1:]
	}

	switch param {
	case "paycheck":
		c.Paycheck, _ = strconv.Atoi(value); return true
	case "money_name_singular":
		c.MoneyNameSingular = value; return true
	case "money_name_plural":
		c.MoneyNamePlural = value; return true
	case "starting_money":
		c.StartingMoney, _ = strconv.Atoi(value); return true
	case "earn_limit":
		c.EarnLimit, _ = strconv.Atoi(value); return true
	case "page_cost":
		c.PageCost, _ = strconv.Atoi(value); return true
	case "wait_cost":
		c.WaitCost, _ = strconv.Atoi(value); return true
	case "link_cost":
		c.LinkCost, _ = strconv.Atoi(value); return true
	case "machine_command_cost":
		c.MachineCommandCost, _ = strconv.Atoi(value); return true
	case "trace_topdown":
		c.TraceTopdown = parseBoolAdmin(value, negate); return true
	case "trace_output_limit":
		c.TraceOutputLimit, _ = strconv.Atoi(value); return true
	case "idle_timeout":
		c.IdleTimeout, _ = strconv.Atoi(value); return true
	case "output_limit":
		c.OutputLimit, _ = strconv.Atoi(value); return true
	case "function_invocation_limit":
		c.FunctionInvocationLimit, _ = strconv.Atoi(value); return true
	case "queue_idle_chunk":
		c.QueueIdleChunk, _ = strconv.Atoi(value); return true
	case "mud_name":
		c.MudName = value; return true
	case "master_room":
		c.MasterRoom, _ = strconv.Atoi(value); return true
	case "player_starting_room":
		c.PlayerStartingRoom, _ = strconv.Atoi(value); return true
	case "player_starting_home":
		c.PlayerStartingHome, _ = strconv.Atoi(value); return true
	case "default_home":
		c.DefaultHome, _ = strconv.Atoi(value); return true
	case "switch_default_all":
		c.SwitchDefaultAll = parseBoolAdmin(value, negate); return true
	case "pemit_far_players":
		c.PemitFarPlayers = parseBoolAdmin(value, negate); return true
	case "pemit_any_object":
		c.PemitAnyObject = parseBoolAdmin(value, negate); return true
	case "public_flags":
		c.PublicFlags = parseBoolAdmin(value, negate); return true
	case "examine_public_attrs":
		c.ExaminePublicAttrs = parseBoolAdmin(value, negate); return true
	case "read_remote_name":
		c.ReadRemoteName = parseBoolAdmin(value, negate); return true
	case "log":
		// @admin log=all_commands / @admin log=!all_commands
		// Currently a no-op placeholder; TinyMUSH uses this for log configuration
		return true
	case "debug":
		SetDebug(parseBoolAdmin(value, negate))
		return true
	default:
		return false
	}
}

func parseBoolAdmin(value string, negate bool) bool {
	if negate { return false }
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return value != ""
}

// archiveHook returns the configured archive hook command, with env override.
func (g *Game) archiveHook() string {
	if v := os.Getenv("MUSH_ARCHIVE_HOOK"); v != "" {
		return v
	}
	if g.Conf != nil {
		return g.Conf.ArchiveHook
	}
	return ""
}

// --- Attribute management ---

// attrAccessNameTable maps flag names (used in @attribute/access and config
// directives) to AF_ flag values. Matches C TinyMUSH's attraccess_nametab.
var attrAccessNameTable = map[string]int{
	"CONST":      gamedb.AFConst,
	"DARK":       gamedb.AFDark,
	"DEFAULT":    gamedb.AFDefault,
	"DELETED":    gamedb.AFDeleted,
	"GOD":        gamedb.AFGod,
	"HIDDEN":     gamedb.AFMDark,
	"IGNORE":     gamedb.AFNoCMD,
	"INTERNAL":   gamedb.AFInternal,
	"IS_LOCK":    gamedb.AFIsLock,
	"LOCKED":     gamedb.AFLock,
	"NO_CLONE":   gamedb.AFNoClone,
	"NO_COMMAND":  gamedb.AFNoProg,
	"NO_INHERIT": gamedb.AFPrivate,
	"VISUAL":     gamedb.AFVisual,
	"WIZARD":     gamedb.AFWizard,
	"PROPAGATE":  gamedb.AFPropagate,
}

// parseAttrAccessFlags parses a space-separated list of flag names (with
// optional ! prefix for negation) and returns (setFlags, clearFlags).
// Matches C TinyMUSH's attraccess_nametab parsing in do_attribute.
func parseAttrAccessFlags(value string) (set, clear int, errs []string) {
	for _, token := range strings.Fields(strings.ToUpper(value)) {
		negate := false
		name := token
		if len(name) > 0 && name[0] == '!' {
			negate = true
			name = name[1:]
		}
		f, ok := attrAccessNameTable[name]
		if !ok {
			errs = append(errs, token)
			continue
		}
		if negate {
			clear |= f
		} else {
			set |= f
		}
	}
	return
}

// cmdAttribute implements @attribute/access, @attribute/rename, @attribute/delete.
// Wizard-only. Matches C TinyMUSH's do_attribute.
func cmdAttribute(g *Game, d *Descriptor, args string, switches []string) {
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}

	if len(switches) == 0 {
		d.Send("Usage: @attribute/access <attr>=<flags>")
		return
	}

	sw := strings.ToLower(switches[0])

	switch sw {
	case "access":
		// @attribute/access <name>=<flags>
		parts := strings.SplitN(args, "=", 2)
		if len(parts) != 2 {
			d.Send("Usage: @attribute/access <attr>=<flags>")
			return
		}
		attrName := strings.TrimSpace(strings.ToUpper(parts[0]))
		flagStr := strings.TrimSpace(parts[1])

		if attrName == "" {
			d.Send("Specify an attribute name.")
			return
		}

		// Look up the attribute definition
		def, ok := g.DB.AttrByName[attrName]
		if !ok {
			// Also check well-known attrs (can't modify their flags)
			for _, wkName := range gamedb.WellKnownAttrs {
				if strings.EqualFold(wkName, attrName) {
					d.Send("Cannot modify access on built-in attributes.")
					return
				}
			}
			d.Send("No such user-named attribute.")
			return
		}

		setFlags, clearFlags, errs := parseAttrAccessFlags(flagStr)
		for _, e := range errs {
			d.Send(fmt.Sprintf("Unknown permission: %s.", e))
		}

		if setFlags != 0 || clearFlags != 0 {
			def.Flags = (def.Flags &^ clearFlags) | setFlags
			// Persist to store
			if g.Store != nil {
				g.Store.PutMeta()
			}
			d.Send("Attribute access changed.")
		}

	case "rename":
		// @attribute/rename <old>=<new>
		parts := strings.SplitN(args, "=", 2)
		if len(parts) != 2 {
			d.Send("Usage: @attribute/rename <old>=<new>")
			return
		}
		oldName := strings.TrimSpace(strings.ToUpper(parts[0]))
		newName := strings.TrimSpace(strings.ToUpper(parts[1]))

		def, ok := g.DB.AttrByName[oldName]
		if !ok {
			d.Send("No such user-named attribute.")
			return
		}
		if _, exists := g.DB.AttrByName[newName]; exists {
			d.Send("An attribute with that name already exists.")
			return
		}

		delete(g.DB.AttrByName, oldName)
		def.Name = newName
		g.DB.AttrByName[newName] = def
		if g.Store != nil {
			g.Store.PutMeta()
		}
		d.Send("Attribute renamed.")

	case "delete":
		attrName := strings.TrimSpace(strings.ToUpper(args))
		if attrName == "" {
			d.Send("Usage: @attribute/delete <attr>")
			return
		}
		def, ok := g.DB.AttrByName[attrName]
		if !ok {
			d.Send("No such user-named attribute.")
			return
		}
		delete(g.DB.AttrByName, attrName)
		delete(g.DB.AttrNames, def.Number)
		if g.Store != nil {
			g.Store.PutMeta()
		}
		d.Send("Attribute deleted.")

	case "propagate":
		cmdAttributePropagate(g, d, args)

	default:
		d.Send("Unknown switch. Use: @attribute/access, @attribute/rename, @attribute/delete, @attribute/propagate")
	}
}

// ApplyAttrAccess applies an @attribute/access directive (from config file).
// Format: "ATTRNAME=FLAGS" or "ATTRNAME FLAGS". Used during startup.
func (g *Game) ApplyAttrAccess(value string) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		log.Printf("gameconf: invalid @attribute/access directive: %s", value)
		return
	}
	attrName := strings.TrimSpace(strings.ToUpper(parts[0]))
	flagStr := strings.TrimSpace(parts[1])

	def, ok := g.DB.AttrByName[attrName]
	if !ok {
		log.Printf("gameconf: @attribute/access: no such attribute %q", attrName)
		return
	}

	setFlags, clearFlags, errs := parseAttrAccessFlags(flagStr)
	for _, e := range errs {
		log.Printf("gameconf: @attribute/access %s: unknown flag %q", attrName, e)
	}
	if setFlags != 0 || clearFlags != 0 {
		def.Flags = (def.Flags &^ clearFlags) | setFlags
		log.Printf("gameconf: @attribute/access %s flags set to 0x%x", attrName, def.Flags)
	}
}

// ApplyAttrType applies an attr_type config directive.
// Format: "pattern flags" — sets flags on all user-defined attrs matching pattern.
func (g *Game) ApplyAttrType(value string) {
	parts := strings.Fields(value)
	if len(parts) < 2 {
		log.Printf("gameconf: invalid attr_type directive: %s", value)
		return
	}
	pattern := strings.ToUpper(parts[0])
	flagStr := strings.Join(parts[1:], " ")

	setFlags, _, errs := parseAttrAccessFlags(flagStr)
	for _, e := range errs {
		log.Printf("gameconf: attr_type %s: unknown flag %q", pattern, e)
	}
	if setFlags == 0 {
		return
	}

	count := 0
	for _, def := range g.DB.AttrNames {
		if wildMatchSimple(strings.ToLower(pattern), strings.ToLower(def.Name)) {
			def.Flags |= setFlags
			count++
		}
	}
	log.Printf("gameconf: attr_type %s applied to %d attributes", pattern, count)
}

// ApplyUserAttrAccess sets the default flags for all user-defined attributes.
// This is the user_attr_access config directive.
func (g *Game) ApplyUserAttrAccess(value string) {
	setFlags, _, errs := parseAttrAccessFlags(value)
	for _, e := range errs {
		log.Printf("gameconf: user_attr_access: unknown flag %q", e)
	}
	if setFlags == 0 {
		return
	}
	count := 0
	for _, def := range g.DB.AttrNames {
		def.Flags |= setFlags
		count++
	}
	log.Printf("gameconf: user_attr_access applied flags 0x%x to %d attributes", setFlags, count)
}

// --- @attlist command ---

// parseTypeFilter extracts a type=<type> qualifier from args string.
// Returns the remaining pattern and the ObjectType filter (-1 if none).
func parseTypeFilter(args string) (string, int) {
	parts := strings.Fields(args)
	typeFilter := -1
	var remaining []string
	for _, p := range parts {
		lower := strings.ToLower(p)
		if strings.HasPrefix(lower, "type=") {
			typeName := strings.ToUpper(p[5:])
			switch typeName {
			case "PLAYER":
				typeFilter = int(gamedb.TypePlayer)
			case "THING", "OBJECT":
				typeFilter = int(gamedb.TypeThing)
			case "ROOM":
				typeFilter = int(gamedb.TypeRoom)
			case "EXIT":
				typeFilter = int(gamedb.TypeExit)
			}
		} else {
			remaining = append(remaining, p)
		}
	}
	return strings.Join(remaining, " "), typeFilter
}

// countAttrsOnObjects counts how many objects have each attribute number.
// If objType >= 0, only counts objects of that type.
// Non-wizards only count objects they control.
func countAttrsOnObjects(g *Game, player gamedb.DBRef, objType int, isWiz bool) map[int]int {
	counts := make(map[int]int)
	for _, obj := range g.DB.Objects {
		if objType >= 0 && int(obj.ObjType()) != objType {
			continue
		}
		if obj.IsGoing() || obj.ObjType() == gamedb.TypeGarbage {
			continue
		}
		// Non-wizards only count objects they control
		if !isWiz && !Controls(g, player, obj.DBRef) {
			continue
		}
		for _, attr := range obj.Attrs {
			counts[attr.Number]++
		}
	}
	return counts
}

// findObjectsWithAttr returns dbrefs of objects that have the given attr number.
// If objType >= 0, only returns objects of that type.
// Non-wizards only see objects they control. Limited to maxResults.
func findObjectsWithAttr(g *Game, player gamedb.DBRef, attrNum int, objType int, isWiz bool, maxResults int) []gamedb.DBRef {
	var results []gamedb.DBRef
	for _, obj := range g.DB.Objects {
		if objType >= 0 && int(obj.ObjType()) != objType {
			continue
		}
		if obj.IsGoing() || obj.ObjType() == gamedb.TypeGarbage {
			continue
		}
		if !isWiz && !Controls(g, player, obj.DBRef) {
			continue
		}
		for _, attr := range obj.Attrs {
			if attr.Number == attrNum {
				results = append(results, obj.DBRef)
				if maxResults > 0 && len(results) >= maxResults {
					return results
				}
				break
			}
		}
	}
	return results
}

// cmdAttlist lists user-defined attribute definitions with their flags and object counts.
// Usage: @attlist [type=<player|thing|room|exit>] [pattern]
//        @attlist/detail <attrname> [type=<player|thing|room|exit>]
// With type= filter, shows configured attrs present on objects of that type.
// Without type=, shows all configured attrs (or pattern-matched attrs).
// /detail shows individual objects that have a specific attribute.
func cmdAttlist(g *Game, d *Descriptor, args string, switches []string) {
	isDetail := false
	for _, sw := range switches {
		if strings.EqualFold(sw, "detail") {
			isDetail = true
		}
	}

	pattern, typeFilter := parseTypeFilter(strings.TrimSpace(args))
	pattern = strings.TrimSpace(pattern)
	isWiz := Wizard(g, d.Player)

	// Detail mode: show objects with a specific attribute
	if isDetail {
		if pattern == "" {
			d.Send("Usage: @attlist/detail <attrname> [type=<player|thing|room|exit>]")
			return
		}
		attrName := strings.ToUpper(pattern)
		def, ok := g.DB.AttrByName[attrName]
		if !ok {
			d.Send("No such attribute definition.")
			return
		}
		results := findObjectsWithAttr(g, d.Player, def.Number, typeFilter, isWiz, 100)
		typeName := ""
		if typeFilter >= 0 {
			typeName = " (" + gamedb.ObjectType(typeFilter).String() + " only)"
		}
		d.Send(fmt.Sprintf("--- Objects with %s%s ---", attrName, typeName))
		for _, ref := range results {
			obj := g.DB.Objects[ref]
			if obj != nil {
				d.Send(fmt.Sprintf("  #%d  %s (%s)", ref, DisplayName(obj.Name), obj.ObjType().String()))
			}
		}
		total := len(results)
		if total >= 100 {
			d.Send(fmt.Sprintf("--- Showing first 100 of possibly more ---"))
		} else {
			d.Send(fmt.Sprintf("--- %d object(s) ---", total))
		}
		return
	}

	// Count attrs across relevant objects
	attrCounts := countAttrsOnObjects(g, d.Player, typeFilter, isWiz)

	// Collect matching definitions
	type entry struct {
		num   int
		name  string
		flags int
		count int
	}
	var results []entry

	for num, def := range g.DB.AttrNames {
		if pattern != "" && !wildMatchSimple(strings.ToLower(pattern), strings.ToLower(def.Name)) {
			continue
		}
		// Without a pattern or type filter, only show attrs that have flags set
		if pattern == "" && typeFilter < 0 && def.Flags == 0 {
			continue
		}
		// With type filter, only show attrs present on that type + must have flags
		if typeFilter >= 0 {
			if attrCounts[num] == 0 {
				continue
			}
			if def.Flags == 0 {
				continue
			}
		}
		// Non-wizards can only see VISUAL attrs (when no type filter)
		if typeFilter < 0 && !isWiz && def.Flags&gamedb.AFVisual == 0 {
			continue
		}
		results = append(results, entry{num: num, name: def.Name, flags: def.Flags, count: attrCounts[num]})
	}

	// Sort by name
	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	if len(results) == 0 {
		if pattern != "" || typeFilter >= 0 {
			d.Send("No matching configured attributes found.")
		} else {
			d.Send("No configured attributes defined.")
		}
		return
	}

	// Header
	typeName := ""
	if typeFilter >= 0 {
		typeName = " on " + gamedb.ObjectType(typeFilter).String() + " objects"
	}
	if pattern == "" && typeFilter < 0 {
		d.Send(fmt.Sprintf("--- Configured Attributes (%d) ---", len(results)))
	} else {
		d.Send(fmt.Sprintf("--- Configured Attributes%s (%d) ---", typeName, len(results)))
	}
	for _, e := range results {
		flagStr := attrFlagString(e.flags)
		if flagStr != "" {
			flagStr = "[" + flagStr + "]"
		} else {
			flagStr = "[-]"
		}
		d.Send(fmt.Sprintf("  %-30s %-8s %d", e.name, flagStr, e.count))
	}
	d.Send(fmt.Sprintf("--- %d attribute(s) listed ---", len(results)))
}

// --- @attribute/propagate command ---

// Propagate adds an attribute with a default value to target objects that
// don't already have it. Wizard-only.
//
// Syntax:
//   @attribute/propagate <attr>=<target>[/<default value>]
//
// Target can be:
//   #dbref    — propagate to all children of that parent object
//   PLAYER    — propagate to all player objects
//   THING     — propagate to all thing objects
//   ROOM      — propagate to all room objects
//   EXIT      — propagate to all exit objects
//   ALL       — propagate to all objects
func cmdAttributePropagate(g *Game, d *Descriptor, args string) {
	parts := strings.SplitN(args, "=", 2)
	if len(parts) != 2 {
		d.Send("Usage: @attribute/propagate <attr>=<target>[/<default value>]")
		return
	}

	attrName := strings.TrimSpace(strings.ToUpper(parts[0]))
	rest := strings.TrimSpace(parts[1])

	// Resolve attribute number
	attrNum := g.ResolveAttrNum(attrName)
	if attrNum < 0 {
		d.Send(fmt.Sprintf("Unknown attribute: %s", attrName))
		return
	}

	// Parse target and optional default value
	var targetStr, defaultVal string
	if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
		targetStr = strings.TrimSpace(rest[:slashIdx])
		defaultVal = rest[slashIdx+1:]
	} else {
		targetStr = rest
	}

	// Determine which objects to propagate to
	var targets []gamedb.DBRef
	upper := strings.ToUpper(targetStr)

	switch upper {
	case "PLAYER", "THING", "ROOM", "EXIT", "ALL":
		var filterType gamedb.ObjectType = -1
		switch upper {
		case "PLAYER":
			filterType = gamedb.TypePlayer
		case "THING":
			filterType = gamedb.TypeThing
		case "ROOM":
			filterType = gamedb.TypeRoom
		case "EXIT":
			filterType = gamedb.TypeExit
		}
		for ref, obj := range g.DB.Objects {
			if obj.Flags[0]&gamedb.FlagGoing != 0 {
				continue
			}
			if filterType >= 0 && obj.ObjType() != filterType {
				continue
			}
			targets = append(targets, ref)
		}
	default:
		// Must be a #dbref — propagate to children of that parent
		parentRef, err := parseDBRef(targetStr)
		if err != nil {
			d.Send("Target must be a #dbref, PLAYER, THING, ROOM, EXIT, or ALL.")
			return
		}
		if _, ok := g.DB.Objects[parentRef]; !ok {
			d.Send(fmt.Sprintf("Parent object #%d not found.", parentRef))
			return
		}
		for ref, obj := range g.DB.Objects {
			if obj.Parent == parentRef {
				targets = append(targets, ref)
			}
		}
	}

	if len(targets) == 0 {
		d.Send("No matching objects found.")
		return
	}

	// Propagate: set attribute only on objects that don't already have it
	set := 0
	skipped := 0
	for _, ref := range targets {
		obj, ok := g.DB.Objects[ref]
		if !ok {
			continue
		}
		// Check if object already has this attribute
		hasIt := false
		for _, attr := range obj.Attrs {
			if attr.Number == attrNum {
				hasIt = true
				break
			}
		}
		if hasIt {
			skipped++
			continue
		}
		// Set the attribute with the default value
		g.SetAttr(ref, attrNum, defaultVal)
		set++
	}

	d.Send(fmt.Sprintf("Propagated %s to %d object(s) (%d already had it, %d total checked).",
		attrName, set, skipped, len(targets)))
}
