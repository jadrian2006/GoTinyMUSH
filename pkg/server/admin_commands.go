package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
		g.Notify(d.Player, "Create what?")
		return
	}
	// @create name [= cost]
	parts := strings.SplitN(args, "=", 2)
	name := strings.TrimSpace(parts[0])

	// Determine cost — clamp to createmin..createmax
	cost := 0
	if len(parts) > 1 {
		cost, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}
	if cost < g.Conf.CreateMinCost {
		cost = g.Conf.CreateMinCost
	}
	if cost > g.Conf.CreateMaxCost {
		cost = g.Conf.CreateMaxCost
	}

	// Check if player can afford it
	playerObj := g.DB.Objects[d.Player]
	if playerObj.Pennies < cost {
		g.Notify(d.Player, fmt.Sprintf("Sorry, you don't have enough %s.", g.MoneyName(2)))
		return
	}

	ref := g.CreateObject(name, gamedb.TypeThing, d.Player)
	obj := g.DB.Objects[ref]
	// Charge the player
	playerObj.Pennies -= cost
	// Set object value (endowment)
	obj.Pennies = g.Conf.ObjectEndowment(cost)
	// Place in player's inventory
	obj.Location = d.Player
	g.AddToContents(d.Player, ref)
	obj.Link = g.PlayerLocation(d.Player) // home = current room
	g.PersistObjects(obj, playerObj)
	g.Notify(d.Player, fmt.Sprintf("%s created as object #%d", name, ref))
}

func cmdDestroy(g *Game, d *Descriptor, args string, switches []string) {
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	// C TinyMUSH: you can destroy DESTROY_OK things in your inventory
	// even if you don't control them (create.c:813-822)
	destroyOKInv := obj.ObjType() == gamedb.TypeThing &&
		obj.HasFlag(gamedb.FlagDestroyOK) && obj.Location == d.Player
	if !g.Controls(d.Player, target) && !destroyOKInv {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// C TinyMUSH: destroyable() — protect #0 and God
	if target == 0 {
		g.Notify(d.Player, "You can't destroy that!")
		return
	}
	if IsGod(g, target) {
		g.Notify(d.Player, "You can't destroy that!")
		return
	}
	// Player destruction checks (matches C TinyMUSH can_destroy_player)
	if obj.ObjType() == gamedb.TypePlayer {
		if !g.IsWizard(d.Player) {
			g.Notify(d.Player, "Sorry, no suicide allowed.")
			return
		}
		if g.IsWizard(target) {
			g.Notify(d.Player, "Even you can't do that!")
			return
		}
	}
	// C TinyMUSH: SAFE check — DESTROY_OK bypasses SAFE (no /override needed)
	destroyOK := obj.HasFlag(gamedb.FlagDestroyOK)
	if obj.HasFlag(gamedb.FlagSafe) && !HasSwitch(switches, "override") &&
		!(obj.ObjType() == gamedb.TypeThing && destroyOK) {
		g.Notify(d.Player, "Sorry, that object is protected. Use @destroy/override to destroy it.")
		return
	}
	instant := HasSwitch(switches, "instant")

	// C TinyMUSH: instant_recycle config — DESTROY_OK things (or things owned
	// by DESTROY_OK owners) skip GOING and destroy immediately
	if !instant && g.Conf.InstantRecycle {
		ownerObj, ownerOK := g.DB.Objects[obj.Owner]
		if destroyOK || (ownerOK && ownerObj.HasFlag(gamedb.FlagDestroyOK)) {
			instant = true
		}
	}

	// Already going?
	if obj.IsGoing() {
		if instant && obj.ObjType() != gamedb.TypeGarbage {
			// @destroy/instant on GOING object — destroy immediately
			g.destroyImmediate(d.Player, target)
			return
		}
		typeName := "thing"
		switch obj.ObjType() {
		case gamedb.TypeRoom:
			typeName = "room"
		case gamedb.TypeExit:
			typeName = "exit"
		case gamedb.TypePlayer:
			typeName = "player"
		}
		g.Notify(d.Player, fmt.Sprintf("That %s has already been destroyed.", typeName))
		return
	}

	// C TinyMUSH @destroy output
	typeName := "thing"
	switch obj.ObjType() {
	case gamedb.TypeRoom:
		typeName = "room"
	case gamedb.TypeExit:
		typeName = "exit"
	case gamedb.TypePlayer:
		typeName = "player"
	}

	// @destroy/instant — bypass the GOING delay, destroy now
	// C TinyMUSH: instant path calls destroy_thing/exit/etc which call
	// destroy_obj(NOTHING, thing) — no "shakes" message, no "Destroyed."
	// The owner still gets the refund notification.
	if instant {
		g.destroyImmediate(gamedb.Nothing, target)
		return
	}

	// Deferred destroy — mark as GOING, reaper handles cleanup
	obj.Flags[0] |= gamedb.FlagGoing
	g.PersistObject(obj)

	if obj.ObjType() != gamedb.TypeRoom {
		g.Notify(d.Player, fmt.Sprintf("The %s shakes and begins to crumble.", typeName))
	} else {
		// C: notify_all — tell everyone in the room
		g.Conns.SendToRoomExcept(g.DB, target, d.Player, "The room shakes and begins to crumble.")
	}
	g.Notify(d.Player, fmt.Sprintf("You will be rewarded shortly for %s(#%d).", DisplayName(obj.Name), target))
}

// PurgeGoing reaps objects marked GOING by @destroy.
// Matches C TinyMUSH's purge_going() in object.c — runs on the dbck interval (10 min).
// Removes objects from location/exit chains, halts queued commands, refunds build credits,
// and converts the object to TYPE_GARBAGE for future reuse.
func (g *Game) PurgeGoing() {
	reaped := 0
	for ref, obj := range g.DB.Objects {
		if !obj.IsGoing() {
			continue
		}
		if obj.ObjType() == gamedb.TypeGarbage {
			continue
		}

		owner := obj.Owner
		ownerObj, ownerOK := g.DB.Objects[owner]

		// Halt any queued commands for this object and clean up semaphore counters
		_, removedSem := g.Queue.HaltPlayer(ref)
		g.cleanupSemaphoreCounters(removedSem)

		// Type-specific cleanup
		switch obj.ObjType() {
		case gamedb.TypeExit:
			// Remove from source room's exit chain
			if obj.Exits != gamedb.Nothing {
				g.RemoveFromExits(obj.Exits, ref)
				if srcObj, ok := g.DB.Objects[obj.Exits]; ok {
					g.PersistObject(srcObj)
				}
			}
			obj.Exits = gamedb.Nothing
			obj.Location = gamedb.Nothing
			obj.Next = gamedb.Nothing
		case gamedb.TypeRoom:
			// Evacuate contents to their homes
			g.evacuateRoom(ref)
		case gamedb.TypeThing:
			// Remove from location's contents chain
			if obj.Location != gamedb.Nothing {
				g.RemoveFromContents(obj.Location, ref)
			}
			obj.Location = gamedb.Nothing
		case gamedb.TypePlayer:
			// Remove from location's contents chain
			if obj.Location != gamedb.Nothing {
				g.RemoveFromContents(obj.Location, ref)
			}
			obj.Location = gamedb.Nothing
		}

		// Refund build credits to owner and notify them
		// C: val = OBJECT_DEPOSIT(Pennies(obj)) = (pennies - sacadjust) * sacfactor
		val := g.Conf.ObjectDeposit(obj.Pennies)
		if ownerOK && owner != ref {
			ownerObj.Pennies += val
			g.PersistObject(ownerObj)
			g.Notify(owner, fmt.Sprintf("You get back your %d %s deposit for %s(#%d).", val, g.MoneyName(val), DisplayName(obj.Name), ref))
		}

		// Convert to garbage
		obj.Name = fmt.Sprintf("Garbage(#%d)", ref)
		obj.Flags = [3]int{int(gamedb.TypeGarbage)} // TYPE_GARBAGE in word 0
		obj.Contents = gamedb.Nothing
		obj.Exits = gamedb.Nothing
		obj.Link = gamedb.Nothing
		obj.Next = gamedb.Nothing
		obj.Parent = gamedb.Nothing
		obj.Owner = gamedb.DBRef(1) // GOD
		obj.Attrs = nil
		g.PersistObject(obj)
		reaped++
	}
	if reaped > 0 {
		log.Printf("PurgeGoing: reaped %d object(s)", reaped)
	}
}

// evacuateRoom sends contents of a GOING room to their homes.
func (g *Game) evacuateRoom(room gamedb.DBRef) {
	roomObj, ok := g.DB.Objects[room]
	if !ok {
		return
	}
	current := roomObj.Contents
	for current != gamedb.Nothing {
		obj, ok := g.DB.Objects[current]
		if !ok {
			break
		}
		next := obj.Next
		if obj.ObjType() == gamedb.TypePlayer || obj.ObjType() == gamedb.TypeThing {
			home := obj.Link
			if home == gamedb.Nothing || home == room {
				home = gamedb.DBRef(0) // Limbo
			}
			g.RemoveFromContents(room, current)
			obj.Location = home
			if homeObj, ok := g.DB.Objects[home]; ok {
				obj.Next = homeObj.Contents
				homeObj.Contents = current
				g.PersistObject(homeObj)
			}
			g.PersistObject(obj)
		}
		current = next
	}
}

// destroyImmediate performs instant destruction of an object (like C's @destroy/instant).
// Removes from chains, refunds credits, converts to garbage.
func (g *Game) destroyImmediate(player, ref gamedb.DBRef) {
	obj, ok := g.DB.Objects[ref]
	if !ok {
		return
	}

	owner := obj.Owner
	ownerObj, ownerOK := g.DB.Objects[owner]

	// Halt queued commands and clean up semaphore counters
	_, removedSem := g.Queue.HaltPlayer(ref)
	g.cleanupSemaphoreCounters(removedSem)

	// Type-specific cleanup
	switch obj.ObjType() {
	case gamedb.TypeExit:
		if obj.Exits != gamedb.Nothing {
			g.RemoveFromExits(obj.Exits, ref)
			if srcObj, ok := g.DB.Objects[obj.Exits]; ok {
				g.PersistObject(srcObj)
			}
		}
		obj.Exits = gamedb.Nothing
		obj.Location = gamedb.Nothing
		obj.Next = gamedb.Nothing
	case gamedb.TypeRoom:
		g.evacuateRoom(ref)
	case gamedb.TypeThing, gamedb.TypePlayer:
		if obj.Location != gamedb.Nothing {
			g.RemoveFromContents(obj.Location, ref)
		}
		obj.Location = gamedb.Nothing
	}

	// Refund build credits and notify owner
	val := g.Conf.ObjectDeposit(obj.Pennies)
	if ownerOK && owner != ref {
		ownerObj.Pennies += val
		g.PersistObject(ownerObj)
		g.Notify(owner, fmt.Sprintf("You get back your %d %s deposit for %s(#%d).", val, g.MoneyName(val), DisplayName(obj.Name), ref))
	}

	// Notify the destroying player (if different from owner, C shows "Destroyed. Owner's Name(#N)")
	if player != gamedb.Nothing {
		g.Notify(player, "Destroyed.")
	}

	// Convert to garbage
	obj.Name = fmt.Sprintf("Garbage(#%d)", ref)
	obj.Flags = [3]int{int(gamedb.TypeGarbage)}
	obj.Contents = gamedb.Nothing
	obj.Exits = gamedb.Nothing
	obj.Link = gamedb.Nothing
	obj.Next = gamedb.Nothing
	obj.Parent = gamedb.Nothing
	obj.Owner = gamedb.DBRef(1)
	obj.Attrs = nil
	g.PersistObject(obj)
}

func cmdLink(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: @link has CS_TWO_ARG|CS_INTERP
	args = evalExpr(g, d.Player, args)
	var targetStr, destStr string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		destStr = strings.TrimSpace(args[eqIdx+1:])
	} else {
		targetStr = strings.TrimSpace(args)
		destStr = ""
	}
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	// C TinyMUSH: @link with empty dest calls do_unlink
	if destStr == "" {
		cmdUnlink(g, d, targetStr, nil)
		return
	}
	dest := g.ResolveRef(d.Player, destStr)
	if dest == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that destination.")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		// C TinyMUSH: Controls() check for player/thing/room; exits check in link_exit
		if !Controls(g, d.Player, target) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		switch obj.ObjType() {
		case gamedb.TypeExit:
			// For exits, destination is stored in Location
			obj.Location = dest
			g.PersistObject(obj)
			g.Notify(d.Player, "Linked.")
		case gamedb.TypeRoom:
			// For rooms, @link sets dropto
			obj.Link = dest
			g.PersistObject(obj)
			g.Notify(d.Player, "Dropto set.")
		default:
			// For players/things, @link sets Home
			obj.Link = dest
			g.PersistObject(obj)
			g.Notify(d.Player, "Home set.")
		}
	}
}

func cmdUnlink(g *Game, d *Descriptor, args string, _ []string) {
	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "Unlink what?")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if obj, ok := g.DB.Objects[target]; ok {
		switch obj.ObjType() {
		case gamedb.TypeExit:
			obj.Location = gamedb.Nothing
			g.PersistObject(obj)
			g.Notify(d.Player, "Unlinked.")
		case gamedb.TypeRoom:
			obj.Link = gamedb.Nothing
			g.PersistObject(obj)
			g.Notify(d.Player, "Dropto removed.")
		default:
			g.Notify(d.Player, "You can't unlink that!")
		}
	}
}

func cmdParent(g *Game, d *Descriptor, args string, _ []string) {
	// CS_TWO_ARG: no = means target=args, parent=""
	var targetStr, parentStr string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		parentStr = strings.TrimSpace(args[eqIdx+1:])
	} else {
		targetStr = strings.TrimSpace(args)
		parentStr = ""
	}
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	parent := gamedb.Nothing
	if parentStr != "" {
		parent = g.ResolveRef(d.Player, parentStr)
		if parent == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that parent.")
			return
		}
	}
	// C TinyMUSH: Controls() check
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// C TinyMUSH: Verify no circular parent reference
	if parent != gamedb.Nothing {
		curr := parent
		for i := 0; i < 100; i++ { // depth limit to prevent infinite loops
			if curr == target {
				g.Notify(d.Player, "You can't have yourself as a parent!")
				return
			}
			parentObj, ok := g.DB.Objects[curr]
			if !ok || parentObj.Parent == gamedb.Nothing {
				break
			}
			curr = parentObj.Parent
		}
	}
	if obj, ok := g.DB.Objects[target]; ok {
		obj.Parent = parent
		g.PersistObject(obj)
		if parent == gamedb.Nothing {
			g.Notify(d.Player, "Parent cleared.")
		} else {
			g.Notify(d.Player, "Parent set.")
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

func cmdChown(g *Game, d *Descriptor, args string, switches []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @chown object = player")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	ownerStr := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	tObj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// C TinyMUSH: can't chown God objects
	if IsGod(g, target) && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// C TinyMUSH: can't chown players (players always own themselves)
	if tObj.ObjType() == gamedb.TypePlayer && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Players always own themselves.")
		return
	}

	// Must control the target
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	owner := g.ResolveRef(d.Player, ownerStr)
	if owner == gamedb.Nothing {
		owner = LookupPlayer(g.DB, ownerStr)
	}
	if owner == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that player.")
		return
	}
	// New owner must be a player
	if ownerObj, ok := g.DB.Objects[owner]; !ok || ownerObj.ObjType() != gamedb.TypePlayer {
		g.Notify(d.Player, "Owner must be a player.")
		return
	}

	// Set the new owner
	tObj.Owner = owner

	// C TinyMUSH: strip privilege flags after chown (unless /nostrip by God)
	nostrip := HasSwitch(switches, "nostrip") && IsGod(g, d.Player)
	if !nostrip && tObj.ObjType() != gamedb.TypePlayer {
		// Strip CHOWN_OK
		tObj.Flags[0] &^= gamedb.FlagChownOK
		// Strip privilege flags and powers
		StripPrivFlags(g, target)
		// Set HALT to stop queued commands running under old permissions
		tObj.Flags[0] |= gamedb.FlagHalt
	}

	g.PersistObject(tObj)
	g.Notify(d.Player, fmt.Sprintf("Owner of %s(#%d) changed to %s(#%d).", tObj.Name, target, g.ObjName(owner), owner))
}

func cmdClone(g *Game, d *Descriptor, args string, switches []string) {
	// @clone[/parent][/inventory] obj [= newname]
	parts := strings.SplitN(args, "=", 2)
	target := g.MatchObject(d.Player, strings.TrimSpace(parts[0]))
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	srcObj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	// C TinyMUSH: Examinable() check — lets players clone VISUAL objects
	if !Examinable(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// C TinyMUSH: Cannot clone players
	if srcObj.ObjType() == gamedb.TypePlayer {
		g.Notify(d.Player, "You cannot clone players!")
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
	// C format: "Name cloned, new copy is object #N." or with rename:
	// "Name cloned as NewName, new copy is object #N."
	if newName != DisplayName(srcObj.Name) {
		g.Notify(d.Player, fmt.Sprintf("%s cloned as %s, new copy is object #%d.", DisplayName(srcObj.Name), newName, ref))
	} else {
		g.Notify(d.Player, fmt.Sprintf("%s cloned, new copy is object #%d.", DisplayName(srcObj.Name), ref))
	}
}

func cmdWipe(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Wipe what?")
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
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	// C TinyMUSH: Controls() check
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		return
	}

	if pattern == "*" {
		obj.Attrs = nil
		g.PersistObject(obj)
		g.Notify(d.Player, "Wiped.")
	} else {
		var remaining []gamedb.Attribute
		for _, attr := range obj.Attrs {
			name := g.DB.GetAttrName(attr.Number)
			if name == "" || !wildMatchSimple(pattern, strings.ToUpper(name)) {
				remaining = append(remaining, attr)
			}
		}
		obj.Attrs = remaining
		g.PersistObject(obj)
		g.Notify(d.Player, "Wiped.")
	}
}

// --- @cpattr command ---
// @cpattr obj/attr = obj/attr[,obj/attr,...]
// Copies source attribute to one or more destination object/attrs.
// If dest has no /attr, uses source attr name.
// If source has no /, assumes me/attr.
func cmdCpattr(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Nothing to copy.")
		return
	}

	src := strings.TrimSpace(args[:eqIdx])
	dstList := strings.TrimSpace(args[eqIdx+1:])

	// Parse source: obj/attr (default obj = me)
	srcObj := d.Player
	srcAttr := src
	if slashIdx := strings.IndexByte(src, '/'); slashIdx >= 0 {
		srcObj = g.MatchObject(d.Player, src[:slashIdx])
		srcAttr = strings.TrimSpace(src[slashIdx+1:])
	}
	if srcObj == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// Get source value
	val := g.GetAttrTextByName(srcObj, strings.ToUpper(srcAttr))

	// Parse destinations (comma-separated)
	dests := strings.Split(dstList, ",")
	for _, dst := range dests {
		dst = strings.TrimSpace(dst)
		if dst == "" {
			continue
		}
		dstObj := d.Player
		dstAttr := srcAttr
		if slashIdx := strings.IndexByte(dst, '/'); slashIdx >= 0 {
			dstObj = g.MatchObject(d.Player, dst[:slashIdx])
			dstAttr = strings.TrimSpace(dst[slashIdx+1:])
		} else {
			// No slash: dest is object name, use source attr name
			dstObj = g.MatchObject(d.Player, dst)
		}
		if dstObj == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that here.")
			continue
		}
		if !g.Controls(d.Player, dstObj) {
			g.Notify(d.Player, "Permission denied.")
			continue
		}
		g.SetAttrByName(dstObj, strings.ToUpper(dstAttr), val, d.Player)
		g.Notify(d.Player, "Set.")
	}
}

// --- @mvattr command ---
// @mvattr obj = SRC_ATTR,DST_ATTR[,DST_ATTR2,...]
// Copies source attr to each dest attr on the same object, then clears source.
func cmdMvattr(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Nothing to do.")
		return
	}

	objStr := strings.TrimSpace(args[:eqIdx])
	attrList := strings.TrimSpace(args[eqIdx+1:])

	// In C, nargs < 2 check comes before object lookup
	attrs := strings.Split(attrList, ",")
	if len(attrs) < 2 {
		g.Notify(d.Player, "Nothing to do.")
		return
	}

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !g.Controls(d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	srcAttr := strings.TrimSpace(attrs[0])
	val := g.GetAttrTextByName(target, strings.ToUpper(srcAttr))

	for _, dst := range attrs[1:] {
		dst = strings.TrimSpace(dst)
		if dst == "" {
			continue
		}
		g.SetAttrByName(target, strings.ToUpper(dst), val, d.Player)
		g.Notify(d.Player, fmt.Sprintf("%s: Set.", strings.ToUpper(dst)))
	}

	// Clear source
	g.SetAttrByName(target, strings.ToUpper(srcAttr), "", d.Player)
	g.Notify(d.Player, fmt.Sprintf("%s: Cleared.", strings.ToUpper(srcAttr)))
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
		// C: "I don't see what you want to lock!"
		g.Notify(d.Player, "I don't see what you want to lock!")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	lockStr := strings.TrimSpace(args[eqIdx+1:])
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	lockAttrNum := aLock // A_LOCK = 42
	if HasSwitch(switches, "enter") || HasSwitch(switches, "enterlock") {
		lockAttrNum = aLEnter // A_LENTER = 59
	} else if HasSwitch(switches, "leave") || HasSwitch(switches, "leavelock") {
		lockAttrNum = aLLeave // A_LLEAVE = 60
	} else if HasSwitch(switches, "use") || HasSwitch(switches, "uselock") {
		lockAttrNum = aLUse // A_LUSE = 62
	} else if HasSwitch(switches, "give") || HasSwitch(switches, "givelock") {
		lockAttrNum = aLGive // A_LGIVE = 63
	} else if HasSwitch(switches, "receive") || HasSwitch(switches, "receivelock") {
		lockAttrNum = aLRecv // A_LRECEIVE = 87
	} else if HasSwitch(switches, "drop") || HasSwitch(switches, "droplock") {
		lockAttrNum = aLDrop // A_LDROP = 86
	}
	// Parse lock expression at set time to resolve names (me, here, etc.) to dbrefs.
	// This matches C TinyMUSH behavior where lock keys are stored as parsed boolexps.
	parsed := ParseBoolExp(g, d.Player, lockStr)
	if parsed != nil {
		lockStr = SerializeBoolExp(parsed)
	}
	g.SetAttr(target, lockAttrNum, lockStr)
	g.Notify(d.Player, "Locked.")
}

func cmdUnlock(g *Game, d *Descriptor, args string, switches []string) {
	// @unlock/attr obj/attrname — unlock an attribute (clears AF_LOCK)
	if HasSwitch(switches, "attr") {
		g.lockAttrInstance(d, args, false)
		return
	}

	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	lockAttrNum := aLock // A_LOCK = 42
	if HasSwitch(switches, "enter") || HasSwitch(switches, "enterlock") {
		lockAttrNum = aLEnter // A_LENTER = 59
	} else if HasSwitch(switches, "leave") || HasSwitch(switches, "leavelock") {
		lockAttrNum = aLLeave // A_LLEAVE = 60
	} else if HasSwitch(switches, "use") || HasSwitch(switches, "uselock") {
		lockAttrNum = aLUse // A_LUSE = 62
	} else if HasSwitch(switches, "give") || HasSwitch(switches, "givelock") {
		lockAttrNum = aLGive // A_LGIVE = 63
	} else if HasSwitch(switches, "receive") || HasSwitch(switches, "receivelock") {
		lockAttrNum = aLRecv // A_LRECEIVE = 87
	} else if HasSwitch(switches, "drop") || HasSwitch(switches, "droplock") {
		lockAttrNum = aLDrop // A_LDROP = 86
	}
	g.SetAttr(target, lockAttrNum, "")
	g.Notify(d.Player, "Unlocked.")
}

// lockAttrInstance sets or clears AF_LOCK on an individual attribute instance.
// args should be "obj/attrname".
func (g *Game) lockAttrInstance(d *Descriptor, args string, lock bool) {
	slashIdx := strings.IndexByte(args, '/')
	if slashIdx < 0 {
		g.Notify(d.Player, "Usage: @lock/attr obj/attrname")
		return
	}
	objName := strings.TrimSpace(args[:slashIdx])
	attrName := strings.ToUpper(strings.TrimSpace(args[slashIdx+1:]))

	target := g.MatchObject(d.Player, objName)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
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
		g.Notify(d.Player, fmt.Sprintf("No such attribute: %s", attrName))
		return
	}

	// Get the master attribute definition flags
	masterFlags := 0
	if wkf, ok := gamedb.WellKnownAttrFlags[attrNum]; ok {
		masterFlags = wkf
	} else if def, ok := g.DB.AttrByName[attrName]; ok {
		masterFlags = def.Flags
	}

	// C: Lock_attr check — can't lock internal, lock-type, or constant attrs
	if masterFlags&(gamedb.AFInternal|gamedb.AFIsLock|gamedb.AFConst) != 0 {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// Can't lock attrs on God objects (unless you're God)
	if IsGod(g, target) && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// Can't lock attrs on CONSTANT objects
	if obj.HasFlag2(gamedb.Flag2ConstAttrs) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// AF_WIZARD/AF_GOD attrs: need Wizard (AF_GOD: need God)
	if masterFlags&gamedb.AFGod != 0 && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if masterFlags&gamedb.AFWizard != 0 && !WizRoy(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
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
			// C: must be Wizard or own the attribute
			playerOwner := d.Player
			if po, ok := g.DB.Objects[d.Player]; ok {
				playerOwner = po.Owner
			}
			if !Wizard(g, d.Player) && owner != playerOwner {
				g.Notify(d.Player, "Permission denied.")
				return
			}
			if lock {
				info.Flags |= gamedb.AFLock
			} else {
				info.Flags &^= gamedb.AFLock
			}
			obj.Attrs[i].Value = fmt.Sprintf("\x01%d:%d:%s", owner, info.Flags, text)
			g.PersistObject(obj)
			if lock {
				g.Notify(d.Player, "Attribute locked.")
			} else {
				g.Notify(d.Player, "Attribute unlocked.")
			}
			return
		}
	}
	g.Notify(d.Player, fmt.Sprintf("No such attribute: %s", attrName))
}

// --- Admin/Wizard Commands ---

// cmdVerb implements @verb: a generic did_it with named attributes.
// C TinyMUSH syntax: @verb obj = actor, what, whatdefault, owhat, owhatdefault, awhat, {args}
// The paired format is: attr_name, default_text (used if attr is empty).
// The +buy system uses: @verb vendor = buyer, MPAY-item, , MOPAY-item, , MAPAY-item, {buyer, cost}
func cmdVerb(g *Game, d *Descriptor, args string, _ []string) {
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @verb obj = actor, what, whatdef, owhat, owhatdef, awhat, {args}")
		return
	}

	// Evaluate the LHS (obj) and RHS
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})
	objStr := ctx.Exec(strings.TrimSpace(args[:eqIdx]), eval.EvFCheck|eval.EvEval, nil)
	rhs := strings.TrimSpace(args[eqIdx+1:])

	obj := g.ResolveRef(d.Player, objStr)
	if obj == gamedb.Nothing {
		obj = g.MatchObject(d.Player, objStr)
	}
	if obj == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// Parse comma-separated args
	parts := splitCommaRespectingBraces(rhs)

	if len(parts) < 1 {
		g.Notify(d.Player, "Usage: @verb obj = actor, what, whatdef, owhat, owhatdef, awhat, {args}")
		return
	}

	actorStr := strings.TrimSpace(parts[0])
	actor := g.ResolveRef(d.Player, actorStr)
	if actor == gamedb.Nothing {
		actor = g.MatchObject(d.Player, actorStr)
	}
	if actor == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	// C TinyMUSH: must control the actor
	if !Controls(g, d.Player, actor) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// Extract attribute names (may be empty)
	// C TinyMUSH paired format: what, whatdef, owhat, owhatdef, awhat, {args}
	getPartStr := func(idx int) string {
		if idx < len(parts) {
			return strings.TrimSpace(parts[idx])
		}
		return ""
	}

	msgAttr := getPartStr(1)    // what: Message attr to actor (like SUCC)
	msgDef := getPartStr(2)     // whatdef: Default msg if attr empty
	omsgAttr := getPartStr(3)   // owhat: Message attr to room (like OSUCC)
	omsgDef := getPartStr(4)    // owhatdef: Default omsg if attr empty
	amsgAttr := getPartStr(5)   // awhat: Action attr (like ASUCC)

	// Args (last element, typically brace-wrapped like {%#, 10})
	var verbArgs []string
	lastPart := getPartStr(len(parts) - 1)
	if len(lastPart) >= 2 && lastPart[0] == '{' && lastPart[len(lastPart)-1] == '}' {
		inner := lastPart[1 : len(lastPart)-1]
		for _, a := range strings.Split(inner, ",") {
			verbArgs = append(verbArgs, strings.TrimSpace(a))
		}
	}

	// Helper to resolve and evaluate a named attribute on obj, with fallback default
	evalAttr := func(attrName, defText string) string {
		if attrName != "" {
			attrNum := g.LookupAttrNum(attrName)
			if attrNum >= 0 {
				if text := g.GetAttrText(obj, attrNum); text != "" {
					evalCtx := MakeEvalContextForObj(g, obj, actor, func(c *eval.EvalContext) {
						functions.RegisterAll(c)
					})
					return evalCtx.Exec(text, eval.EvFCheck|eval.EvEval|eval.EvStrip, verbArgs)
				}
			}
		}
		return defText
	}

	// Show message to actor (what)
	if msg := evalAttr(msgAttr, msgDef); msg != "" {
		g.Conns.SendToPlayer(actor, msg)
	}

	// Show omsg to room (owhat), prefixed with actor name
	if msg := evalAttr(omsgAttr, omsgDef); msg != "" {
		loc := g.PlayerLocation(actor)
		if loc != gamedb.Nothing {
			actorObj, _ := g.DB.Objects[actor]
			prefix := ""
			if actorObj != nil {
				prefix = DisplayName(actorObj.Name) + " "
			}
			g.Conns.SendToRoomExcept(g.DB, loc, actor, prefix+msg)
		}
	}

	// Queue the action attribute (awhat)
	if amsgAttr != "" {
		attrNum := g.LookupAttrNum(amsgAttr)
		if attrNum >= 0 {
			if text := g.GetAttrText(obj, attrNum); text != "" {
				entry := &QueueEntry{
					Player:  obj,
					Cause:   actor,
					Caller:  actor,
					Command: text,
					Args:    verbArgs,
				}
				g.Queue.Add(entry)
			}
		}
	}
}

func cmdTeleport(g *Game, d *Descriptor, args string, _ []string) {
	// @tel dest  OR  @tel victim = dest
	// Evaluate args (C TinyMUSH evaluates function calls before dispatch)
	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})

	var victim gamedb.DBRef
	var destStr string

	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		victimStr := ctx.Exec(strings.TrimSpace(args[:eqIdx]), eval.EvFCheck|eval.EvEval, nil)
		destStr = ctx.Exec(strings.TrimSpace(args[eqIdx+1:]), eval.EvFCheck|eval.EvEval, nil)
		victim = g.MatchObject(d.Player, victimStr)
		if victim == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that here.")
			return
		}
	} else {
		victim = d.Player
		destStr = ctx.Exec(strings.TrimSpace(args), eval.EvFCheck|eval.EvEval, nil)
	}

	if strings.EqualFold(destStr, "home") {
		if obj, ok := g.DB.Objects[victim]; ok {
			destStr = fmt.Sprintf("#%d", obj.Link)
		}
	}

	dest := g.ResolveRef(d.Player, destStr)
	if dest == gamedb.Nothing {
		g.Notify(d.Player, "No match.")
		return
	}

	// Permission checks (matching C TinyMUSH do_teleport):

	// C: FIXED flag blocks teleport for non-wizards
	if pObj, ok := g.DB.Objects[d.Player]; ok {
		if pObj.HasFlag2(gamedb.Flag2Fixed) && !Wizard(g, d.Player) &&
			!pObj.HasPower(0, gamedb.PowTelAnywhr) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
	}

	// 1. Must control the victim OR be wizard OR have TEL_ANYTHING power
	hasTelAnything := false
	if pObj, ok := g.DB.Objects[d.Player]; ok {
		hasTelAnything = pObj.HasPower(0, gamedb.PowTelUnrst)
	}
	if !Controls(g, d.Player, victim) && !Wizard(g, d.Player) && !hasTelAnything {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// 2. Destination must be JUMP_OK or player must control dest or be wizard/TEL_ANYWHERE
	hasTelAnywhere := false
	if pObj, ok := g.DB.Objects[d.Player]; ok {
		hasTelAnywhere = pObj.HasPower(0, gamedb.PowTelAnywhr)
	}
	if destObj, ok := g.DB.Objects[dest]; ok {
		if !Wizard(g, d.Player) && !Controls(g, d.Player, dest) && !hasTelAnywhere {
			if !destObj.HasFlag(gamedb.FlagJumpOK) {
				g.Notify(d.Player, "You can't teleport there!")
				return
			}
			// Check teleport lock on destination
			if !CouldDoIt(g, victim, dest, 97) { // A_TLOCK = 97
				g.Notify(d.Player, "You can't teleport there!")
				return
			}
		}
	}

	// If dest is an EXIT, follow it to its destination room.
	// In C TinyMUSH, @tel obj=exit sends the object through the exit.
	// An exit's Location field holds the destination room.
	if destObj, ok := g.DB.Objects[dest]; ok && destObj.ObjType() == gamedb.TypeExit {
		exitDest := destObj.Location
		if exitDest == gamedb.Nothing {
			g.Notify(d.Player, "That exit doesn't lead anywhere.")
			return
		}
		dest = exitDest
	}

	// Find descriptor for victim (if connected)
	descs := g.Conns.GetByPlayer(victim)

	// C TinyMUSH move_via_teleport sequence:
	// 1. OXTPORT to old room (before move)
	// 2. LEAVE/OLEAVE/ALEAVE on old location
	// 3. Move object
	// 4. TPORT to victim, OTPORT to new room, ATPORT action
	// 5. MOVE to victim, OMOVE to new room, AMOVE action
	// 6. ENTER/OENTER/AENTER on new location
	// 7. "has left"/"has arrived" default messages
	const (
		aOXTPort = 81 // A_OXTPORT
		aTPort   = 79 // A_TPORT
		aOTPort  = 80 // A_OTPORT
		aATPort  = 82 // A_ATPORT
		aMove    = 55 // A_MOVE
		aOMove   = 56 // A_OMOVE
		aAMove   = 57 // A_AMOVE
		aOLeave  = 51 // A_OLEAVE
		aALeave  = 52 // A_ALEAVE
		aOEnter  = 53 // A_OENTER
		aAEnter  = 35 // A_AENTER
	)

	if obj, ok := g.DB.Objects[victim]; ok {
		oldLoc := obj.Location
		isDark := obj.HasFlag(gamedb.FlagDark)

		// Step 1: OXTPORT to old room (before move)
		g.DidIt(victim, victim, 0, aOXTPort, 0)

		// Step 2: LEAVE/OLEAVE/ALEAVE on old location
		if oldLoc != gamedb.Nothing {
			if !isDark {
				g.QueueAttrAction(oldLoc, victim, aALeave, nil)
				if oleave := g.GetAttrText(oldLoc, aOLeave); oleave != "" {
					ctx := MakeEvalContextForObj(g, oldLoc, victim, func(c *eval.EvalContext) {
						functions.RegisterAll(c)
					})
					msg := ctx.Exec(oleave, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
					if msg != "" {
						g.Conns.SendToRoomExcept(g.DB, oldLoc, victim,
							DisplayName(obj.Name)+" "+msg)
					}
				} else {
					g.Conns.SendToRoomExcept(g.DB, oldLoc, victim,
						fmt.Sprintf("%s has left.", DisplayName(obj.Name)))
				}
			}
			g.RemoveFromContents(oldLoc, victim)
		}

		// Step 3: Move object
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

		// Step 4: TPORT to victim, OTPORT to new room, ATPORT action
		g.DidIt(victim, victim, aTPort, aOTPort, aATPort)

		// Step 5: MOVE to victim, OMOVE to new room, AMOVE action
		g.DidIt(victim, victim, aMove, aOMove, aAMove)

		// Step 6+7: "has arrived" + OENTER/AENTER on destination
		if !isDark {
			g.Conns.SendToRoomExcept(g.DB, dest, victim,
				fmt.Sprintf("%s has arrived.", DisplayName(obj.Name)))
			g.QueueAttrAction(dest, victim, aAEnter, nil)
			if oenter := g.GetAttrText(dest, aOEnter); oenter != "" {
				ctx := MakeEvalContextForObj(g, dest, victim, func(c *eval.EvalContext) {
					functions.RegisterAll(c)
				})
				msg := ctx.Exec(oenter, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
				if msg != "" {
					g.Conns.SendToRoomExcept(g.DB, dest, victim,
						DisplayName(obj.Name)+" "+msg)
				}
			}
		}
	}

	if victim == d.Player {
		g.ShowRoom(d, dest)
	} else {
		g.Notify(d.Player, fmt.Sprintf("Teleported %s to %s(#%d).", g.ObjName(victim), g.ObjName(dest), dest))
		if len(descs) > 0 {
			g.ShowRoom(descs[0], dest)
		}
	}

	// Instance movement: if the teleported object is an instance, notify occupants
	if obj, ok := g.DB.Objects[victim]; ok {
		if obj.HasFlag3(gamedb.Flag3Instance) {
			g.MoveInstanceOccupants(victim)
		}
	}
}

func cmdForce(g *Game, d *Descriptor, args string, _ []string) {
	// C: CS_TWO_ARG — splits at =. No = → targetStr=args, command=""
	var targetStr, command string
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		targetStr = strings.TrimSpace(args)
		command = ""
	} else {
		targetStr = strings.TrimSpace(args[:eqIdx])
		command = strings.TrimSpace(args[eqIdx+1:])
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
	if command != "" {
		g.DoForce(d.Player, target, command)
	}
}

func cmdTriggerCmd(g *Game, d *Descriptor, args string, switches []string) {
	var ok bool
	if HasSwitch(switches, "now") {
		g.DoTriggerNow(d.Player, d.Player, args)
		ok = true // DoTriggerNow doesn't return bool; assume success for /now
	} else {
		ok = g.DoTrigger(d.Player, d.Player, args)
	}
	if !ok {
		g.Notify(d.Player, "No match.")
		return
	}
	g.Notify(d.Player, "Triggered.")
}

func cmdWaitCmd(g *Game, d *Descriptor, args string, _ []string) {
	g.DoWait(d.Player, d.Player, args)
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
		// C: noisy_match_result() + explicit "No match."
		g.Notify(d.Player, "I don't see that here.")
		g.Notify(d.Player, "No match.")
		return
	}
	// C TinyMUSH: controls(player, thing) || Link_ok(thing)
	if !Controls(g, d.Player, target) {
		if tObj, ok := g.DB.Objects[target]; !ok || !tObj.HasFlag(gamedb.FlagLinkOK) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
	}

	attr := gamedb.A_SEMAPHORE // Default to A_SEMAPHORE (47), matching C TinyMUSH
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

	g.semaphoreNotify(target, attr, count)
	g.Notify(d.Player, "Notified.")
}

func cmdHalt(g *Game, d *Descriptor, args string, switches []string) {
	if HasSwitch(switches, "all") {
		// C TinyMUSH: Can_Halt (wizard) check for /all
		if !Wizard(g, d.Player) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		// @halt/all - halt all objects' queue entries
		removed, removedSemAll := g.Queue.HaltAll()
		g.cleanupSemaphoreCounters(removedSemAll)
		if removed == 1 {
			g.Notify(d.Player, "1 queue entries removed.")
		} else {
			g.Notify(d.Player, fmt.Sprintf("%d queue entries removed.", removed))
		}
		return
	}
	target := d.Player
	if args != "" {
		target = g.MatchObject(d.Player, args)
		if target == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that here.")
			return
		}
		// C TinyMUSH: Controls() check for halting another object
		if !Controls(g, d.Player, target) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
	}
	removed, removedSemHalt := g.Queue.HaltPlayer(target)
	g.cleanupSemaphoreCounters(removedSemHalt)
	// Note: C TinyMUSH's @halt only clears queue entries — it does NOT set
	// the HALT flag. The HALT flag is only set via @set obj=HALT. This is
	// important because STARTUP patterns like "@halt me; @wait 60=@tr me/loop"
	// rely on the object still being able to queue new commands after @halt.
	if removed == 1 {
		g.Notify(d.Player, "1 queue entries removed.")
	} else {
		g.Notify(d.Player, fmt.Sprintf("%d queue entries removed.", removed))
	}
}

func cmdBoot(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: requires Can_Boot power (wizard-level)
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	target := LookupPlayer(g.DB, strings.TrimSpace(args))
	if target == gamedb.Nothing {
		g.Notify(d.Player, "No such player.")
		return
	}
	// C: God protection — can't boot God
	if IsGod(g, target) && !IsGod(g, d.Player) {
		g.Notify(d.Player, "You cannot boot that player!")
		return
	}
	// C: can't boot yourself
	if target == d.Player {
		g.Notify(d.Player, "You cannot boot yourself!")
		return
	}
	descs := g.Conns.GetByPlayer(target)
	if len(descs) == 0 {
		g.Notify(d.Player, "That player is not connected.")
		return
	}
	for _, dd := range descs {
		dd.Send("You have been booted.")
		g.DisconnectPlayer(dd)
	}
	g.Notify(d.Player, fmt.Sprintf("Booted %s.", g.ObjName(target)))
}

func cmdWall(g *Game, d *Descriptor, args string, _ []string) {
	if !g.IsWizard(d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
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

// cmdFixDB rebuilds all content/exit chains in the database. The argument
// is accepted for backward compatibility but the full rebuild runs regardless.
// Usage: @fixdb #<dbref>
func cmdFixDB(g *Game, d *Descriptor, args string, _ []string) {
	if !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	target := g.ResolveRef(d.Player, strings.TrimSpace(args))
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if _, ok := g.DB.Objects[target]; !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	// Run full chain rebuild (affects all containers, not just this one,
	// but that's fine — it's idempotent and fast)
	containers, objects := g.RepairAllChains()
	g.Notify(d.Player, fmt.Sprintf("Database check complete: %d containers, %d objects updated.", containers, objects))
}

// @fixall — rebuild all content/exit chains across the entire database.
// Usage: @fixall
func cmdFixAll(g *Game, d *Descriptor, args string, _ []string) {
	if !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	containers, objects := g.RepairAllChains()
	g.Notify(d.Player, fmt.Sprintf("Database check complete: %d containers, %d objects updated.", containers, objects))
}

func cmdNewPassword(g *Game, d *Descriptor, args string, _ []string) {
	// C TinyMUSH: CA_WIZARD — only wizards can change other players' passwords
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @newpassword player = password")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	newPass := strings.TrimSpace(args[eqIdx+1:])
	// Use MatchObject to handle #dbref, *player, and name syntax
	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "No such player.")
		return
	}
	// Verify target is a player
	if obj, ok := g.DB.Objects[target]; !ok || obj.ObjType() != gamedb.TypePlayer {
		g.Notify(d.Player, "No such player.")
		return
	}
	// God protection: only God can change God's password
	if IsGod(g, target) && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Only God can change God's password. Use the -godpass flag to reset it externally.")
		return
	}
	// Encrypt and store
	hash := mushcrypt.Crypt(newPass, "XX")
	g.SetAttr(target, aPass, hash)
	g.Notify(d.Player, fmt.Sprintf("Password for %s changed.", g.ObjName(target)))
}

// cmdPcreate implements @pcreate name=password — wizard creates a player
// without logging them in. Matches C TinyMUSH create.c:do_pcreate.
func cmdPcreate(g *Game, d *Descriptor, args string, _ []string) {
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 || args == "" {
		g.Notify(d.Player, "Usage: @pcreate name = password")
		return
	}
	name := strings.TrimSpace(args[:eqIdx])
	pass := strings.TrimSpace(args[eqIdx+1:])
	if name == "" || pass == "" {
		g.Notify(d.Player, "Usage: @pcreate name = password")
		return
	}
	// Check if name already exists
	if LookupPlayer(g.DB, name) != gamedb.Nothing {
		g.Notify(d.Player, "That name is already taken.")
		return
	}
	if len(name) < 2 {
		g.Notify(d.Player, "That name is too short.")
		return
	}
	for _, ch := range name {
		if ch == '"' || ch == ';' {
			g.Notify(d.Player, "That name contains illegal characters.")
			return
		}
	}
	if g.IsBadName(name) {
		g.Notify(d.Player, "That name is not allowed.")
		return
	}
	// Create the player
	ref := g.CreateObject(name, gamedb.TypePlayer, d.Player)
	playerObj := g.DB.Objects[ref]
	playerObj.Owner = ref
	// Set password
	hash := mushcrypt.Crypt(pass, "XX")
	g.SetAttr(ref, aPass, hash)
	// Place at start room
	startRoom := g.StartingRoom()
	startHome := g.StartingHome()
	playerObj.Location = startRoom
	playerObj.Link = startHome
	g.AddToContents(startRoom, ref)
	if roomObj, ok := g.DB.Objects[startRoom]; ok {
		g.PersistObjects(playerObj, roomObj)
	}
	if g.Store != nil {
		g.Store.PutMeta()
		g.Store.UpdatePlayerIndex(playerObj, "")
	}
	log.Printf("PCREATE: %s(#%d) created player %s(#%d)", g.PlayerName(d.Player), d.Player, name, ref)
	g.Notify(d.Player, fmt.Sprintf("Player %s created as #%d.", name, ref))
}

// cmdBotcreate implements @botcreate name — wizard creates a bot player.
// Creates the player with ROBOT flag, generates an API key, and sets no password.
// The bot authenticates only via API key, never interactively.
func cmdBotcreate(g *Game, d *Descriptor, args string, _ []string) {
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		g.Notify(d.Player, "Usage: @botcreate name")
		return
	}
	// Check if name already exists
	if LookupPlayer(g.DB, name) != gamedb.Nothing {
		g.Notify(d.Player, "That name is already taken.")
		return
	}
	if len(name) < 2 {
		g.Notify(d.Player, "That name is too short.")
		return
	}
	for _, ch := range name {
		if ch == '"' || ch == ';' {
			g.Notify(d.Player, "That name contains illegal characters.")
			return
		}
	}
	if g.IsBadName(name) {
		g.Notify(d.Player, "That name is not allowed.")
		return
	}
	// Create the player
	ref := g.CreateObject(name, gamedb.TypePlayer, d.Player)
	playerObj := g.DB.Objects[ref]
	playerObj.Owner = ref
	// Set ROBOT flag
	playerObj.Flags[0] |= gamedb.FlagRobot
	// No password — bot uses API key only
	// Place at creator's location (like C TinyMUSH PCRE_ROBOT)
	creatorLoc := g.PlayerLocation(d.Player)
	playerObj.Location = creatorLoc
	playerObj.Link = creatorLoc // home = creator's location
	g.AddToContents(creatorLoc, ref)
	if roomObj, ok := g.DB.Objects[creatorLoc]; ok {
		g.PersistObjects(playerObj, roomObj)
	}
	if g.Store != nil {
		g.Store.PutMeta()
		g.Store.UpdatePlayerIndex(playerObj, "")
	}
	// Generate API key
	var rawKey string
	if g.Store != nil {
		keyBytes := make([]byte, 32)
		rand.Read(keyBytes)
		rawKey = hex.EncodeToString(keyBytes)
		h := sha256.Sum256([]byte(rawKey))
		keyHash := hex.EncodeToString(h[:])
		if err := g.Store.PutAPIKey(ref, keyHash); err != nil {
			g.Notify(d.Player, fmt.Sprintf("Warning: API key generation failed: %s", err))
		}
	}
	log.Printf("BOTCREATE: %s(#%d) created bot %s(#%d)", g.PlayerName(d.Player), d.Player, name, ref)
	g.Notify(d.Player, fmt.Sprintf("Bot player %s created as #%d with ROBOT flag.", name, ref))
	if rawKey != "" {
		g.Notify(d.Player, fmt.Sprintf("API Key: %s", rawKey))
		g.Notify(d.Player, "Store this key securely - it will not be shown again.")
		g.Notify(d.Player, fmt.Sprintf("Authenticate via: POST /api/v1/auth/apikey with {\"key\":\"%s\",\"dbref\":\"#%d\"}", rawKey, ref))
	}
}

func cmdFind(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Find what?")
		return
	}
	pattern := strings.ToLower(strings.TrimSpace(args))
	count := 0
	for _, obj := range g.DB.Objects {
		if obj.IsGoing() {
			continue
		}
		// C TinyMUSH: only show objects the player controls
		if !Controls(g, d.Player, obj.DBRef) {
			continue
		}
		if obj.ObjType() == gamedb.TypeExit {
			continue // C TinyMUSH: @find skips exits
		}
		if wildMatchSimple(pattern, strings.ToLower(obj.Name)) {
			g.Notify(d.Player, fmt.Sprintf("  %s(#%d%s) Owner: %s(#%d)",
				obj.Name, obj.DBRef, typeChar(obj.ObjType()),
				g.ObjName(obj.Owner), obj.Owner))
			count++
			if count >= 200 {
				g.Notify(d.Player, "*** Too many results, truncated ***")
				break
			}
		}
	}
	g.Notify(d.Player, fmt.Sprintf("%d object(s) found.", count))
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
	g.Notify(d.Player, fmt.Sprintf("Database statistics:"))
	g.Notify(d.Player, fmt.Sprintf("  %d rooms, %d things, %d exits, %d players, %d garbage",
		rooms, things, exits, players, garbage))
	g.Notify(d.Player, fmt.Sprintf("  %d total objects", len(g.DB.Objects)))
	g.Notify(d.Player, fmt.Sprintf("  %d attribute definitions", len(g.DB.AttrNames)))
	imm, wait, sem := g.Queue.Stats()
	g.Notify(d.Player, fmt.Sprintf("  Queue: %d immediate, %d waiting, %d semaphore", imm, wait, sem))
	g.Notify(d.Player, fmt.Sprintf("  %d active connections", g.Conns.Count()))
}

func cmdPs(g *Game, d *Descriptor, _ string, switches []string) {
	// C TinyMUSH: @ps/all requires wizard
	showAll := HasSwitch(switches, "all")
	if showAll && !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	// C format: show entries by queue type with headers, then Totals line
	entries := g.Queue.Peek(200)
	owner := ResolveOwner(g, d.Player)

	// Categorize entries
	var playerQ, objectQ, waitQ, semQ []*QueueEntry
	var pTot, oTot, wTot, sTot int
	for _, e := range entries {
		eOwner := ResolveOwner(g, e.Player)
		isWait := !e.WaitUntil.IsZero() && e.SemObj < 0
		isSem := e.SemObj >= 0
		switch {
		case isSem:
			sTot++
			if showAll || eOwner == owner {
				semQ = append(semQ, e)
			}
		case isWait:
			wTot++
			if showAll || eOwner == owner {
				waitQ = append(waitQ, e)
			}
		default:
			// C separates Player queue and Object queue
			if e.Player == eOwner {
				pTot++
				if showAll || eOwner == owner {
					playerQ = append(playerQ, e)
				}
			} else {
				oTot++
				if showAll || eOwner == owner {
					objectQ = append(objectQ, e)
				}
			}
		}
	}

	// Show entries with headers (C format: "----- X Queue -----")
	showQ := func(label string, q []*QueueEntry) {
		if len(q) == 0 {
			return
		}
		g.Notify(d.Player, fmt.Sprintf("----- %s Queue -----", label))
		for _, e := range q {
			name := g.ObjName(e.Player)
			flags := g.ObjFlags(e.Player)
			objStr := fmt.Sprintf("%s(#%d%s)", name, e.Player, flags)
			if e.SemObj >= 0 && !e.WaitUntil.IsZero() {
				secs := int(time.Until(e.WaitUntil).Seconds())
				if secs < 0 {
					secs = 0
				}
				g.Notify(d.Player, fmt.Sprintf("[#%d/%d] %s:%s", e.SemObj, secs, objStr, e.Command))
			} else if !e.WaitUntil.IsZero() {
				secs := int(time.Until(e.WaitUntil).Seconds())
				if secs < 0 {
					secs = 0
				}
				g.Notify(d.Player, fmt.Sprintf("[%d] %s:%s", secs, objStr, e.Command))
			} else if e.SemObj >= 0 {
				g.Notify(d.Player, fmt.Sprintf("[#%d] %s:%s", e.SemObj, objStr, e.Command))
			} else {
				g.Notify(d.Player, fmt.Sprintf("%s:%s", objStr, e.Command))
			}
		}
	}

	showQ("Player", playerQ)
	showQ("Object", objectQ)
	showQ("Wait", waitQ)
	showQ("Semaphore", semQ)

	// C Totals line
	g.Notify(d.Player, fmt.Sprintf("Totals: Player...%d/%d  Object...%d/%d  Wait...%d/%d  Semaphore...%d/%d",
		len(playerQ), pTot, len(objectQ), oTot, len(waitQ), wTot, len(semQ), sTot))
}

// --- Softcode Commands ---

func cmdSwitch(g *Game, d *Descriptor, args string, switches []string) {
	// @switch expr = pattern1, action1 [, pattern2, action2, ...] [, default]
	// @switch/all fires ALL matching cases (not just first)
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @switch expression = pattern1, action1, ...")
		return
	}

	ctx := MakeEvalContextWithGame(g, d.Player, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})

	exprStr := strings.TrimSpace(args[:eqIdx])
	expr := ctx.Exec(exprStr, eval.EvFCheck|eval.EvEval, nil)

	rest := strings.TrimSpace(args[eqIdx+1:])
	parts := splitCommaRespectingBraces(rest)

	matchAll := HasSwitch(switches, "all")
	matched := false

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
			matched = true
			if !matchAll {
				return
			}
		}
	}
	// Default (odd trailing entry, only if no match)
	if len(parts)%2 == 1 && !matched {
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
		g.Notify(d.Player, "Usage: &ATTR object=value")
		return
	}
	attrName := strings.ToUpper(strings.TrimSpace(args[:spaceIdx]))
	rest := strings.TrimSpace(args[spaceIdx+1:])

	// C TinyMUSH: "&ATTR obj" (no =) clears the attribute, same as "&ATTR obj=".
	var targetStr, value string
	eqIdx := strings.IndexByte(rest, '=')
	if eqIdx < 0 {
		targetStr = rest
		value = ""
	} else {
		targetStr = strings.TrimSpace(rest[:eqIdx])
		value = strings.TrimSpace(rest[eqIdx+1:])
	}

	if attrName == "" {
		g.Notify(d.Player, "Usage: &ATTR object=value")
		return
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	ok, errMsg := g.SetAttrByNameChecked(d.Player, target, attrName, value)
	if !ok {
		g.Notify(d.Player, errMsg)
	} else {
		g.Notify(d.Player, "Set.")
	}
}

// cmdEdit implements @edit obj/attr=search,replace
// Special search patterns: $ = append to end, ^ = prepend to start
// Escaped: \$ or \^ searches for literal $ or ^
func cmdEdit(g *Game, d *Descriptor, args string, _ []string) {
	// Parse obj/attr = search,replace
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @edit obj/attr = search, replace")
		return
	}
	objAttr := strings.TrimSpace(args[:eqIdx])
	rest := args[eqIdx+1:]

	slashIdx := strings.IndexByte(objAttr, '/')
	if slashIdx < 0 {
		g.Notify(d.Player, "Usage: @edit obj/attr = search, replace")
		return
	}
	objStr := strings.TrimSpace(objAttr[:slashIdx])
	attrName := strings.TrimSpace(objAttr[slashIdx+1:])

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
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
		g.Notify(d.Player, fmt.Sprintf("No such attribute: %s", attrName))
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

	g.Notify(d.Player, fmt.Sprintf("Set - %s: %s", strings.ToUpper(attrName), result))
}

// parseEditArgs splits "search,replace" respecting brace quoting.
// Returns (from, to). If only one part, to is empty.
func parseEditArgs(s string) (string, string) {
	parts := splitEditComma(s)
	from := stripBraces(strings.TrimSpace(parts[0]))
	to := ""
	if len(parts) > 1 {
		to = stripBraces(strings.TrimSpace(parts[1]))
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
		// C: CS_TWO_ARG — no = means match against full args (usually empty → "I don't see that here.")
		g.Notify(d.Player, "I don't see that here.")
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
			g.Notify(d.Player, "I don't see that here.")
			return
		}
		if !Controls(g, d.Player, target) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		g.setAttrFlag(d, target, attrName, value)
		return
	}

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// Check for attr:value format
	if colonIdx := strings.IndexByte(value, ':'); colonIdx >= 0 {
		attrName := strings.ToUpper(strings.TrimSpace(value[:colonIdx]))
		attrValue := strings.TrimSpace(value[colonIdx+1:])
		if !Controls(g, d.Player, target) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		ok, errMsg := g.SetAttrByNameChecked(d.Player, target, attrName, attrValue)
		if !ok {
			g.Notify(d.Player, errMsg)
		} else {
			g.Notify(d.Player, "Set.")
		}
		return
	}

	// Flag setting
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	switch g.SetFlag(target, value, d.Player) {
	case SetFlagOK:
		if strings.HasPrefix(value, "!") {
			g.Notify(d.Player, "Cleared.")
		} else {
			g.Notify(d.Player, "Set.")
		}
	case SetFlagDenied:
		g.Notify(d.Player, "Permission denied.")
	case SetFlagAmbiguous:
		g.Notify(d.Player, "Ambiguous flag name.")
	default:
		g.Notify(d.Player, "I don't know that flag.")
	}
}

// setAttrFlag handles @set obj/attr = [!]flagname — sets or clears an attribute flag.
func (g *Game) setAttrFlag(d *Descriptor, target gamedb.DBRef, attrName string, flagStr string) {
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
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
		g.Notify(d.Player, fmt.Sprintf("No such attribute: %s", attrName))
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

	// Try exact match first, then prefix match (C TinyMUSH matches by prefix)
	bit, ok2 := attrFlagNames[fname]
	if !ok2 {
		var matches []string
		for name := range attrFlagNames {
			if strings.HasPrefix(name, fname) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			bit = attrFlagNames[matches[0]]
		} else if len(matches) > 1 {
			g.Notify(d.Player, fmt.Sprintf("Ambiguous attribute flag: %s", fname))
			return
		} else {
			g.Notify(d.Player, fmt.Sprintf("Unknown attribute flag: %s", fname))
			return
		}
	}

	// AF_GOD and AF_WIZARD flags require special permissions
	if bit == gamedb.AFGod && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if bit == gamedb.AFWizard && !SetsWizAttrs(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
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
			g.Notify(d.Player, "Set.")
			return
		}
	}
	g.Notify(d.Player, fmt.Sprintf("No such attribute: %s", attrName))
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
		DebugLog("SETATTR_NEW player=#%d obj=#%d attr=%s value=%q (new attr)", player, obj, attrName, truncDebug(value, 100))
		g.SetAttrByName(obj, attrName, value, player)
		return true, ""
	}
	ok, msg := g.SetAttrChecked(player, obj, attrNum, value)
	if !ok {
		DebugLog("SETATTR_DENIED player=#%d obj=#%d attr=%s(#%d) msg=%q", player, obj, attrName, attrNum, msg)
	} else {
		DebugLog("SETATTR_OK player=#%d obj=#%d attr=%s(#%d) value=%q", player, obj, attrName, attrNum, truncDebug(value, 100))
	}
	return ok, msg
}

// --- Helper methods on Game ---

// Controls returns true if the player controls the target.
// Delegates to the full permission model in perms.go.
func (g *Game) Controls(player, target gamedb.DBRef) bool {
	return Controls(g, player, target)
}

func (g *Game) Examinable(player, target gamedb.DBRef) bool {
	return Examinable(g, player, target)
}

// --- Utility ---

// wildMatchSimple is a simple glob matcher for internal use.
func wildMatchSimple(pattern, str string) bool {
	// C TinyMUSH's wild_match supports numeric comparison operators
	// in @switch patterns: >N, <N, >=N, <=N
	if len(pattern) > 0 && (pattern[0] == '>' || pattern[0] == '<') {
		op := string(pattern[0])
		rest := pattern[1:]
		if len(rest) > 0 && rest[0] == '=' {
			op += "="
			rest = rest[1:]
		}
		pVal := toFloat(rest)
		sVal := toFloat(str)
		switch op {
		case ">":
			return sVal > pVal
		case ">=":
			return sVal >= pVal
		case "<":
			return sVal < pVal
		case "<=":
			return sVal <= pVal
		}
	}
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

func cmdDump(g *Game, d *Descriptor, args string, switches []string) {
	// @dump is an alias for @archive
	cmdArchive(g, d, args, switches)
}

// --- @backup command ---

func cmdBackup(g *Game, d *Descriptor, args string, _ []string) {
	// Only wizards can backup
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	if g.Store == nil {
		g.Notify(d.Player, "No bolt database configured. Use -bolt flag to enable.")
		return
	}

	path := args
	if path == "" {
		path = fmt.Sprintf("game-backup-%s.bolt", time.Now().Format("20060102-150405"))
	}

	g.Notify(d.Player, fmt.Sprintf("Backing up database to %s...", path))
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
		g.Notify(d.Player, "Usage: @dolist <list> = <command>")
		return
	}

	listStr := strings.TrimSpace(args[:eqIdx])
	command := strings.TrimSpace(args[eqIdx+1:])

	if listStr == "" || command == "" {
		g.Notify(d.Player, "Usage: @dolist <list> = <command>")
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
	// CS_TWO_ARG: no = means target=args, msg=""
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
		g.Notify(d.Player, "Emit except to whom?")
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
	// @remit room = message (CS_TWO_ARG: no = means target=args, msg="")
	var roomStr, message string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		roomStr = strings.TrimSpace(args[:eqIdx])
		message = stripEqSep(args[eqIdx+1:])
	} else {
		roomStr = strings.TrimSpace(args)
		message = ""
	}

	room := g.ResolveRef(d.Player, roomStr)
	if room == gamedb.Nothing {
		g.Notify(d.Player, "Emit to whom?")
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
		g.Notify(d.Player, "Usage: @password old = new")
		return
	}
	oldPass := strings.TrimSpace(args[:eqIdx])
	newPass := strings.TrimSpace(args[eqIdx+1:])

	if oldPass == "" || newPass == "" {
		g.Notify(d.Player, "You must specify both old and new passwords.")
		return
	}

	// Verify old password
	currentHash := g.GetAttrText(d.Player, aPass)
	if currentHash == "" {
		g.Notify(d.Player, "You don't have a password set.")
		return
	}
	check := mushcrypt.Crypt(oldPass, currentHash[:2])
	if check != currentHash {
		g.Notify(d.Player, "Sorry.")
		return
	}

	// Set new password
	hash := mushcrypt.Crypt(newPass, "XX")
	g.SetAttr(d.Player, aPass, hash)
	g.Notify(d.Player, "Password changed.")
}

func cmdVersion(g *Game, d *Descriptor, _ string, _ []string) {
	g.Notify(d.Player, VersionString())
	// Show uptime
	if !g.StartTime.IsZero() {
		g.Notify(d.Player, formatUptime(g.StartTime))
	}
	// Show enabled features
	var features []string
	if g.Comsys != nil {
		features = append(features, "Comsys")
	}
	if g.Mail != nil {
		features = append(features, "Mail")
	}
	if g.SQLDB != nil {
		features = append(features, "SQL")
	}
	if g.Spell != nil {
		features = append(features, "Spellcheck")
	}
	if g.Conf != nil && g.Conf.PuebloEnabled {
		features = append(features, "Pueblo")
	}
	if g.EventBus != nil {
		features = append(features, "GMCP/MSDP")
	}
	features = append(features, "MSSP")
	if len(features) > 0 {
		g.Notify(d.Player, "Features: " + strings.Join(features, ", "))
	}
}

func cmdUptime(g *Game, d *Descriptor, _ string, _ []string) {
	if g.StartTime.IsZero() {
		g.Notify(d.Player, "Server start time not available.")
		return
	}
	g.Notify(d.Player, formatUptime(g.StartTime))
}

// formatUptime returns a human-readable uptime string.
func formatUptime(start time.Time) string {
	dur := time.Since(start)
	days := int(dur.Hours()) / 24
	hours := int(dur.Hours()) % 24
	mins := int(dur.Minutes()) % 60
	secs := int(dur.Seconds()) % 60
	if days > 0 {
		return fmt.Sprintf("Uptime: %dd %dh %dm %ds (since %s)", days, hours, mins, secs, start.Format("2006-01-02 15:04:05"))
	}
	return fmt.Sprintf("Uptime: %dh %dm %ds (since %s)", hours, mins, secs, start.Format("2006-01-02 15:04:05"))
}

func cmdMotd(g *Game, d *Descriptor, args string, switches []string) {
	if HasSwitch(switches, "wizard") {
		if !Wizard(g, d.Player) { g.Notify(d.Player, "Permission denied."); return }
		if args == "" {
			if g.WizMOTD != "" { g.Notify(d.Player, g.WizMOTD) } else { g.Notify(d.Player, "No wizard MOTD set.") }
		} else {
			g.WizMOTD = args
			g.Notify(d.Player, "Wizard MOTD set.")
		}
		return
	}
	if HasSwitch(switches, "down") {
		if !Wizard(g, d.Player) { g.Notify(d.Player, "Permission denied."); return }
		if args == "" {
			if g.DownMOTD != "" { g.Notify(d.Player, g.DownMOTD) } else { g.Notify(d.Player, "No down MOTD set.") }
		} else {
			g.DownMOTD = args
			g.Notify(d.Player, "Down MOTD set.")
		}
		return
	}
	if HasSwitch(switches, "full") {
		if !Wizard(g, d.Player) { g.Notify(d.Player, "Permission denied."); return }
		if args == "" {
			if g.FullMOTD != "" { g.Notify(d.Player, g.FullMOTD) } else { g.Notify(d.Player, "No full MOTD set.") }
		} else {
			g.FullMOTD = args
			g.Notify(d.Player, "Full MOTD set.")
		}
		return
	}

	if args == "" {
		// Show current MOTD
		if g.MOTD != "" {
			g.Notify(d.Player, g.MOTD)
		} else if g.Texts != nil {
			motd := g.Texts.GetMotd()
			if motd != "" {
				g.Notify(d.Player, motd)
			} else {
				g.Notify(d.Player, "No message of the day.")
			}
		} else {
			g.Notify(d.Player, "No message of the day.")
		}
		return
	}
	// Wizard-only: set MOTD
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	g.MOTD = args
	g.Notify(d.Player, "MOTD set.")
}

func cmdChzone(g *Game, d *Descriptor, args string, switches []string) {
	// @chzone obj = zone
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @chzone object = zone")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	zoneStr := strings.TrimSpace(args[eqIdx+1:])

	target := g.MatchObject(d.Player, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}

	zone := gamedb.Nothing
	if zoneStr != "" && !strings.EqualFold(zoneStr, "none") {
		zone = g.ResolveRef(d.Player, zoneStr)
		if zone == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that zone.")
			return
		}

		// Validate zone type: must be THING or ROOM
		zoneObj, zOk := g.DB.Objects[zone]
		if !zOk {
			g.Notify(d.Player, "No such zone object.")
			return
		}
		zoneType := zoneObj.ObjType()
		if zoneType != gamedb.TypeThing && zoneType != gamedb.TypeRoom {
			g.Notify(d.Player, "Invalid zone object type.")
			return
		}

		// Room-to-room restriction: only rooms may be zoned to rooms
		if zoneType == gamedb.TypeRoom && targetObj.ObjType() != gamedb.TypeRoom {
			g.Notify(d.Player, "Only rooms may be zoned to parent rooms.")
			return
		}
	}

	// Permission check on target:
	// Wizard, Controls, CheckZoneForPlayer, or same owner
	if !Wizard(g, d.Player) &&
		!Controls(g, d.Player, target) &&
		!CheckZoneForPlayer(g, d.Player, target, 0) &&
		targetObj.Owner != d.Player {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// Permission check on new zone (if setting, not clearing)
	if zone != gamedb.Nothing {
		zoneObj := g.DB.Objects[zone]
		if !Wizard(g, d.Player) &&
			!Controls(g, d.Player, zone) &&
			zoneObj.Owner != d.Player {
			g.Notify(d.Player, "Permission denied.")
			return
		}
	}

	// Handle /add and /remove switches for multi-zone
	if HasSwitch(switches, "add") {
		if g.Conf != nil && !g.Conf.MultizoneEnabled {
			g.Notify(d.Player, "The multi-zone system is not enabled.")
			return
		}
		if zone == gamedb.Nothing {
			g.Notify(d.Player, "You must specify a zone to add.")
			return
		}
		// Check not already in zones list
		for _, z := range targetObj.Zones {
			if z == zone {
				g.Notify(d.Player, fmt.Sprintf("%s(#%d) is already in zone %s(#%d).", targetObj.Name, target, g.ObjName(zone), zone))
				return
			}
		}
		if targetObj.Zone == zone {
			g.Notify(d.Player, fmt.Sprintf("%s(#%d) is already in zone %s(#%d).", targetObj.Name, target, g.ObjName(zone), zone))
			return
		}
		targetObj.Zones = append(targetObj.Zones, zone)
		g.PersistObject(targetObj)
		g.Notify(d.Player, fmt.Sprintf("Zone %s(#%d) added to %s(#%d).", g.ObjName(zone), zone, targetObj.Name, target))
		return
	}

	if HasSwitch(switches, "remove") {
		if g.Conf != nil && !g.Conf.MultizoneEnabled {
			g.Notify(d.Player, "The multi-zone system is not enabled.")
			return
		}
		if zone == gamedb.Nothing {
			g.Notify(d.Player, "You must specify a zone to remove.")
			return
		}
		removed := false
		// Check primary zone
		if targetObj.Zone == zone {
			targetObj.Zone = gamedb.Nothing
			removed = true
		}
		// Check additional zones
		for i, z := range targetObj.Zones {
			if z == zone {
				targetObj.Zones = append(targetObj.Zones[:i], targetObj.Zones[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			g.Notify(d.Player, fmt.Sprintf("%s(#%d) is not in zone %s(#%d).", targetObj.Name, target, g.ObjName(zone), zone))
			return
		}
		g.PersistObject(targetObj)
		g.Notify(d.Player, fmt.Sprintf("Zone %s(#%d) removed from %s(#%d).", g.ObjName(zone), zone, targetObj.Name, target))
		return
	}

	// Set the primary zone (existing behavior)
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
		g.Notify(d.Player, fmt.Sprintf("Zone of %s(#%d) cleared.", targetObj.Name, target))
	} else {
		g.Notify(d.Player, fmt.Sprintf("Zone of %s(#%d) set to %s(#%d).", targetObj.Name, target, g.ObjName(zone), zone))
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
				case "garbage":
					typeFilter = gamedb.TypeGarbage
				}
			case "name":
				namePattern = strings.ToLower(val)
			}
		} else if namePattern == "" {
			namePattern = strings.ToLower(part)
		}
	}

	// Collect matching objects sorted by dbref
	var matches []gamedb.DBRef
	for ref, obj := range g.DB.Objects {
		if typeFilter >= 0 && obj.ObjType() != typeFilter {
			continue
		}
		if namePattern != "" && !wildMatchSimple(namePattern, strings.ToLower(obj.Name)) {
			continue
		}
		if !g.Controls(d.Player, ref) {
			continue
		}
		matches = append(matches, ref)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i] < matches[j] })

	// C TinyMUSH groups by type: ROOMS, EXITS, OBJECTS, PLAYERS, GARBAGE
	rcount, ecount, tcount, pcount, gcount := 0, 0, 0, 0, 0

	// Rooms
	if typeFilter == gamedb.TypeRoom || typeFilter < 0 {
		first := true
		for _, ref := range matches {
			obj := g.DB.Objects[ref]
			if obj.ObjType() != gamedb.TypeRoom {
				continue
			}
			if first {
				g.Notify(d.Player, "\nROOMS:")
				first = false
			}
			g.Notify(d.Player, g.unparseObject(d.Player, ref))
			rcount++
		}
	}

	// Exits
	if typeFilter == gamedb.TypeExit || typeFilter < 0 {
		first := true
		for _, ref := range matches {
			obj := g.DB.Objects[ref]
			if obj.ObjType() != gamedb.TypeExit {
				continue
			}
			if first {
				g.Notify(d.Player, "\nEXITS:")
				first = false
			}
			from := obj.Exits // exit's "from" is stored in Exits field (source room)
			to := obj.Location
			fromStr := "NOWHERE"
			if from != gamedb.Nothing {
				fromStr = g.unparseObject(d.Player, from)
			}
			toStr := "NOWHERE"
			if to != gamedb.Nothing {
				toStr = g.unparseObject(d.Player, to)
			}
			g.Notify(d.Player, fmt.Sprintf("%s [from %s to %s]",
				g.unparseObject(d.Player, ref), fromStr, toStr))
			ecount++
		}
	}

	// Objects (things)
	if typeFilter == gamedb.TypeThing || typeFilter < 0 {
		first := true
		for _, ref := range matches {
			obj := g.DB.Objects[ref]
			if obj.ObjType() != gamedb.TypeThing {
				continue
			}
			if first {
				g.Notify(d.Player, "\nOBJECTS:")
				first = false
			}
			ownerStr := g.unparseObject(d.Player, obj.Owner)
			g.Notify(d.Player, fmt.Sprintf("%s [owner: %s]",
				g.unparseObject(d.Player, ref), ownerStr))
			tcount++
		}
	}

	// Players
	if typeFilter == gamedb.TypePlayer || typeFilter < 0 {
		first := true
		for _, ref := range matches {
			obj := g.DB.Objects[ref]
			if obj.ObjType() != gamedb.TypePlayer {
				continue
			}
			if first {
				g.Notify(d.Player, "\nPLAYERS:")
				first = false
			}
			g.Notify(d.Player, g.unparseObject(d.Player, ref))
			pcount++
		}
	}

	// Garbage
	if typeFilter == gamedb.TypeGarbage || typeFilter < 0 {
		for _, ref := range matches {
			obj := g.DB.Objects[ref]
			if obj.ObjType() != gamedb.TypeGarbage {
				continue
			}
			gcount++
		}
	}

	total := rcount + ecount + tcount + pcount + gcount
	if total == 0 {
		g.Notify(d.Player, "Nothing found.")
	} else {
		g.Notify(d.Player, fmt.Sprintf("\nFound:  Rooms...%d  Exits...%d  Objects...%d  Players...%d  Garbage...%d",
			rcount, ecount, tcount, pcount, gcount))
	}
}

// --- @entrances command ---
// Lists all objects that link/home/dropto/parent to the target.
// C TinyMUSH format: exits show "SourceRoom(#N) (exitname)", things/players show "Name(#N) [home]",
// rooms show "Name(#N) [dropto]", parents show "Name(#N) [parent]".
func cmdEntrances(g *Game, d *Descriptor, args string, _ []string) {
	var thing gamedb.DBRef
	if args == "" {
		// Default: current location
		if pObj, ok := g.DB.Objects[d.Player]; ok {
			thing = pObj.Location
		}
	} else {
		thing = g.MatchObject(d.Player, args)
	}
	if thing == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// Collect all dbrefs sorted (C iterates by dbref order)
	var refs []gamedb.DBRef
	for ref := range g.DB.Objects {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i] < refs[j] })

	count := 0
	for _, ref := range refs {
		obj := g.DB.Objects[ref]
		if obj.IsGoing() {
			continue
		}
		// Must be examinable by player or target examinable
		if !Examinable(g, d.Player, thing) && !Examinable(g, d.Player, ref) {
			continue
		}

		switch obj.ObjType() {
		case gamedb.TypeExit:
			// Exit destination (Location) == thing
			if obj.Location == thing {
				source := obj.Exits // exit source room
				srcStr := g.unparseObject(d.Player, source)
				g.Notify(d.Player, fmt.Sprintf("%s (%s)", srcStr, DisplayName(obj.Name)))
				count++
			}
		case gamedb.TypeRoom:
			// Room dropto (Link) == thing
			if obj.Link == thing {
				g.Notify(d.Player, fmt.Sprintf("%s [dropto]", g.unparseObject(d.Player, ref)))
				count++
			}
		case gamedb.TypeThing, gamedb.TypePlayer:
			// Home (Link) == thing
			if obj.Link == thing {
				g.Notify(d.Player, fmt.Sprintf("%s [home]", g.unparseObject(d.Player, ref)))
				count++
			}
		}

		// Check parent
		if obj.Parent == thing {
			g.Notify(d.Player, fmt.Sprintf("%s [parent]", g.unparseObject(d.Player, ref)))
			count++
		}
	}

	if count == 1 {
		g.Notify(d.Player, "1 entrance found.")
	} else {
		g.Notify(d.Player, fmt.Sprintf("%d entrances found.", count))
	}
}

// --- @quota command ---
// C TinyMUSH: if quotas are not enabled, says "Quotas are not enabled."
// We don't implement quota enforcement, so match C's disabled behavior.
func cmdQuota(g *Game, d *Descriptor, args string, _ []string) {
	g.Notify(d.Player, "Quotas are not enabled.")
}

// decompileAttrCmd maps well-known attribute numbers to their @-command names.
// Attrs listed here are output as "@Command obj=value" in @decompile.
// Attrs not listed here and >= A_USER_START use "&ATTR obj=value" format.
var decompileAttrCmd = map[int]string{
	1:   "Osuccess",
	2:   "Ofail",
	3:   "Fail",
	4:   "Success",
	6:   "Describe",
	7:   "Sex",
	8:   "Odrop",
	9:   "Drop",
	10:  "Okill",
	11:  "Kill",
	12:  "Asucc",
	13:  "Afail",
	14:  "Adrop",
	15:  "Akill",
	16:  "Ause",
	17:  "Charges",
	18:  "Runout",
	19:  "Startup",
	20:  "Aclone",
	21:  "Apay",
	22:  "Opay",
	23:  "Pay",
	24:  "Cost",
	26:  "Listen",
	27:  "Aahear",
	28:  "Amhear",
	29:  "Ahear",
	32:  "Idescribe",
	33:  "Enter",
	34:  "Oxenter",
	35:  "Aenter",
	36:  "Adescribe",
	37:  "Odescribe",
	39:  "Aconnect",
	40:  "Adisconnect",
	45:  "Use",
	46:  "Ouse",
	50:  "Leave",
	51:  "Oleave",
	52:  "Aleave",
	53:  "Oenter",
	54:  "Oxleave",
	55:  "Move",
	56:  "Omove",
	57:  "Amove",
	58:  "Alias",
	64:  "Ealias",
	65:  "Lalias",
	66:  "Efail",
	67:  "Oefail",
	68:  "Aefail",
	69:  "Lfail",
	70:  "Olfail",
	71:  "Alfail",
	72:  "Reject",
	73:  "Away",
	74:  "Idle",
	75:  "Ufail",
	76:  "Oufail",
	77:  "Aufail",
	79:  "Tport",
	80:  "Otport",
	81:  "Oxtport",
	82:  "Atport",
	89:  "Inprefix",
	90:  "Prefix",
	91:  "Infilter",
	92:  "Filter",
	95:  "Forwardlist",
	129: "Gfail",
	130: "Ogfail",
	131: "Agfail",
	132: "Rfail",
	133: "Orfail",
	134: "Arfail",
	135: "Dfail",
	136: "Odfail",
	137: "Adfail",
	138: "Tfail",
	139: "Otfail",
	140: "Atfail",
	141: "Tofail",
	142: "Otofail",
	143: "Atofail",
	214: "Conformat",
	215: "Exitformat",
	222: "Nameformat",
}

func cmdDecompile(g *Game, d *Descriptor, args string, _ []string) {
	if args == "" {
		g.Notify(d.Player, "Decompile what?")
		return
	}

	// Parse object/attrpattern — split on '/' for attribute wildcard filter
	objStr := args
	attrPattern := ""
	if slashIdx := strings.IndexByte(args, '/'); slashIdx >= 0 {
		objStr = args[:slashIdx]
		attrPattern = strings.ToUpper(args[slashIdx+1:])
	}

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}

	// Collect all output lines, then send with marker wrapping
	var lines []string

	// Use object name for ref (C TinyMUSH style), "me" for self
	ref := DisplayName(obj.Name)
	if target == d.Player {
		ref = "me"
	}

	attrOnly := attrPattern != ""

	// Object creation line (skip for players; show even with attr filter for non-players)
	switch obj.ObjType() {
	case gamedb.TypeRoom:
		lines = append(lines, fmt.Sprintf("@dig %s", obj.Name))
	case gamedb.TypeThing:
		lines = append(lines, fmt.Sprintf("@create %s=10", obj.Name))
	case gamedb.TypeExit:
		lines = append(lines, fmt.Sprintf("@open %s", obj.Name))
	case gamedb.TypePlayer:
		// Can't recreate players via decompile
	}

	// Show attributes
	for _, attr := range obj.Attrs {
		name := g.DB.GetAttrName(attr.Number)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", attr.Number)
		}
		// If filtering by attribute pattern, check match
		if attrOnly && !wildMatchCI(attrPattern, name) {
			continue
		}
		text := eval.StripAttrPrefix(attr.Value)
		// Skip internal/sensitive attrs
		if isInternalAttr(attr.Number) {
			continue
		}
		// Use @Command format for well-known attrs, &ATTR for user attrs
		if cmd, ok := decompileAttrCmd[attr.Number]; ok {
			lines = append(lines, fmt.Sprintf("@%s %s=%s", cmd, ref, text))
		} else if attr.Number >= gamedb.A_USER_START {
			lines = append(lines, fmt.Sprintf("&%s %s=%s", name, ref, text))
		} else {
			// Built-in attr without a known @-command — use &ATTR format
			lines = append(lines, fmt.Sprintf("&%s %s=%s", name, ref, text))
		}
	}

	// Flags (skip when filtering by attribute)
	if !attrOnly {
		if obj.HasFlag(gamedb.FlagDark) {
			lines = append(lines, fmt.Sprintf("@set %s=DARK", ref))
		}
		if obj.HasFlag(gamedb.FlagHaven) {
			lines = append(lines, fmt.Sprintf("@set %s=HAVEN", ref))
		}
		if obj.HasFlag(gamedb.FlagQuiet) {
			lines = append(lines, fmt.Sprintf("@set %s=QUIET", ref))
		}
		if obj.HasFlag(gamedb.FlagSafe) {
			lines = append(lines, fmt.Sprintf("@set %s=SAFE", ref))
		}
		if obj.HasFlag(gamedb.FlagEnterOK) {
			lines = append(lines, fmt.Sprintf("@set %s=ENTER_OK", ref))
		}
		if obj.HasFlag(gamedb.FlagVisual) {
			lines = append(lines, fmt.Sprintf("@set %s=VISUAL", ref))
		}
		if obj.HasFlag(gamedb.FlagPuppet) {
			lines = append(lines, fmt.Sprintf("@set %s=PUPPET", ref))
		}
		if obj.HasFlag(gamedb.FlagSticky) {
			lines = append(lines, fmt.Sprintf("@set %s=STICKY", ref))
		}

		// Parent
		if obj.Parent != gamedb.Nothing {
			lines = append(lines, fmt.Sprintf("@parent %s=#%d", ref, obj.Parent))
		}
	}

	// Send with DECOMPILE marker wrapping (block mode: open before, close after)
	markerVal := g.GetAttrTextByName(d.Player, "MARKER_DECOMPILE")
	openMarker, closeMarker := "", ""
	if markerVal != "" {
		if idx := strings.IndexByte(markerVal, '|'); idx >= 0 {
			openMarker = markerVal[:idx]
			closeMarker = markerVal[idx+1:]
		} else {
			openMarker = markerVal
		}
	}

	if openMarker != "" {
		g.Notify(d.Player, openMarker)
	}
	for _, line := range lines {
		g.Notify(d.Player, line)
	}
	if closeMarker != "" {
		g.Notify(d.Player, closeMarker)
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
	Word    int
	Bit     int
	GodOnly bool // ph_god in C: only God may set/clear
}

// powerTable maps power name strings to their (word, bit) pairs.
// GodOnly matches C's ph_god handler — only God can set/clear these powers.
var powerTable = map[string]powerEntry{
	"change_quotas":    {Word: 0, Bit: gamedb.PowChgQuotas},
	"chown_anything":   {Word: 0, Bit: gamedb.PowChownAny},
	"announce":         {Word: 0, Bit: gamedb.PowAnnounce},
	"boot":             {Word: 0, Bit: gamedb.PowBoot},
	"halt":             {Word: 0, Bit: gamedb.PowHalt},
	"control_all":      {Word: 0, Bit: gamedb.PowControlAll, GodOnly: true},
	"wizard_who":       {Word: 0, Bit: gamedb.PowWizardWho},
	"see_all":          {Word: 0, Bit: gamedb.PowExamAll},
	"find_unfindable":  {Word: 0, Bit: gamedb.PowFindUnfind},
	"free_money":       {Word: 0, Bit: gamedb.PowFreeMoney},
	"free_quota":       {Word: 0, Bit: gamedb.PowFreeQuota},
	"hide":             {Word: 0, Bit: gamedb.PowHide},
	"idle":             {Word: 0, Bit: gamedb.PowIdle},
	"search":           {Word: 0, Bit: gamedb.PowSearch},
	"long_fingers":     {Word: 0, Bit: gamedb.PowLongfingers},
	"prog":             {Word: 0, Bit: gamedb.PowProg},
	"mdark_attr":       {Word: 0, Bit: gamedb.PowMdarkAttr},
	"wiz_attr":         {Word: 0, Bit: gamedb.PowWizAttr},
	"comm_all":         {Word: 0, Bit: gamedb.PowCommAll},
	"see_queue":        {Word: 0, Bit: gamedb.PowSeeQueue},
	"see_hidden":       {Word: 0, Bit: gamedb.PowSeeHidden},
	"watch":            {Word: 0, Bit: gamedb.PowWatch},
	"poll":             {Word: 0, Bit: gamedb.PowPoll},
	"no_destroy":       {Word: 0, Bit: gamedb.PowNoDestroy},
	"guest":            {Word: 0, Bit: gamedb.PowGuest, GodOnly: true},
	"pass_locks":       {Word: 0, Bit: gamedb.PowPassLocks},
	"stat_any":         {Word: 0, Bit: gamedb.PowStatAny},
	"steal":            {Word: 0, Bit: gamedb.PowSteal},
	"tel_anywhere":     {Word: 0, Bit: gamedb.PowTelAnywhr},
	"tel_unrestricted": {Word: 0, Bit: gamedb.PowTelUnrst},
	"unkillable":       {Word: 0, Bit: gamedb.PowUnkillable},
	"builder":          {Word: 1, Bit: gamedb.Pow2Builder},
	"link_variable":    {Word: 1, Bit: gamedb.Pow2LinkVar},
	"link_to_anything": {Word: 1, Bit: gamedb.Pow2LinkToAny},
	"open_anywhere":    {Word: 1, Bit: gamedb.Pow2OpenAnyLoc},
	"use_sql":          {Word: 1, Bit: gamedb.Pow2UseSQL, GodOnly: true},
	"link_any_home":    {Word: 1, Bit: gamedb.Pow2LinkHome},
	"cloak":            {Word: 1, Bit: gamedb.Pow2Cloak, GodOnly: true},
	"bot":              {Word: 1, Bit: gamedb.Pow2Bot},
}

// --- @apikey command ---

// cmdApikey implements @apikey generate|revoke <object>
// Generates or revokes an API key for a Player or Thing.
// Permission: Wizard OR (caller has bot power AND owns target).
func cmdApikey(g *Game, d *Descriptor, args string, _ []string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 || args == "" {
		g.Notify(d.Player, "Usage: @apikey generate|revoke <object>")
		return
	}
	action := strings.ToLower(strings.TrimSpace(parts[0]))
	targetStr := strings.TrimSpace(parts[1])

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

	// Only Player and Thing types can have API keys
	if obj.ObjType() != gamedb.TypePlayer && obj.ObjType() != gamedb.TypeThing {
		g.Notify(d.Player, "Only players and things can have API keys.")
		return
	}

	// Permission: Wizard OR (caller has bot power AND owns target)
	callerObj, cOK := g.DB.Objects[d.Player]
	isWiz := Wizard(g, d.Player)
	hasBotPower := cOK && callerObj.HasPower(1, gamedb.Pow2Bot)
	ownsTarget := cOK && (obj.Owner == d.Player || d.Player == target)

	if !isWiz && !(hasBotPower && ownsTarget) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	if g.Store == nil {
		g.Notify(d.Player, "Storage not available.")
		return
	}

	switch action {
	case "generate":
		// Generate a 64-char hex key (32 random bytes)
		keyBytes := make([]byte, 32)
		rand.Read(keyBytes)
		rawKey := hex.EncodeToString(keyBytes)

		// Store SHA-256 hash
		h := sha256.Sum256([]byte(rawKey))
		hash := hex.EncodeToString(h[:])

		if err := g.Store.PutAPIKey(target, hash); err != nil {
			g.Notify(d.Player, fmt.Sprintf("Error storing API key: %s", err))
			return
		}

		// Warn if player doesn't have ROBOT flag
		if obj.ObjType() == gamedb.TypePlayer && !obj.HasFlag(gamedb.FlagRobot) {
			g.Notify(d.Player, fmt.Sprintf("Warning: %s(#%d) does not have the ROBOT flag set.", obj.Name, target))
		}

		g.Notify(d.Player, fmt.Sprintf("API key generated for %s(#%d).", obj.Name, target))
		g.Notify(d.Player, fmt.Sprintf("Key: %s", rawKey))
		g.Notify(d.Player, "Store this key securely - it will not be shown again.")
		g.Notify(d.Player, fmt.Sprintf("Authenticate via: POST /api/v1/auth/apikey with {\"key\":\"%s\",\"dbref\":\"#%d\"}", rawKey, target))

	case "revoke":
		if !g.Store.HasAPIKey(target) {
			g.Notify(d.Player, fmt.Sprintf("%s(#%d) does not have an API key.", obj.Name, target))
			return
		}
		if err := g.Store.DeleteAPIKey(target); err != nil {
			g.Notify(d.Player, fmt.Sprintf("Error revoking API key: %s", err))
			return
		}
		g.Notify(d.Player, fmt.Sprintf("API key revoked for %s(#%d).", obj.Name, target))

	default:
		g.Notify(d.Player, "Usage: @apikey generate|revoke <object>")
	}
}

// --- SQL Commands ---

func cmdSQL(g *Game, d *Descriptor, args string, _ []string) {
	// @sql <query> — Wizard-only interactive query tool
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if args == "" {
		g.Notify(d.Player, "Usage: @sql <query>")
		return
	}
	if g.SQLDB == nil {
		g.Notify(d.Player, "SQL is not configured.")
		return
	}

	trimmed := strings.TrimSpace(args)
	upper := strings.ToUpper(trimmed)

	if strings.HasPrefix(upper, "SELECT") {
		// SELECT: show row-by-row field display
		result, err := g.SQLDB.Query(trimmed, "\n", "\x01")
		if err != nil {
			g.Notify(d.Player, fmt.Sprintf("SQL error: %s", err))
			return
		}
		if result == "" {
			g.Notify(d.Player, "No rows returned.")
			return
		}
		rows := strings.Split(result, "\n")
		for i, row := range rows {
			fields := strings.Split(row, "\x01")
			for j, field := range fields {
				g.Notify(d.Player, fmt.Sprintf("Row %d, Field %d: %s", i+1, j+1, field))
			}
		}
		g.Notify(d.Player, fmt.Sprintf("%d row(s) returned.", len(rows)))
	} else {
		// Non-SELECT
		result, err := g.SQLDB.Query(trimmed, " ", " ")
		if err != nil {
			g.Notify(d.Player, fmt.Sprintf("SQL error: %s", err))
			return
		}
		g.Notify(d.Player, fmt.Sprintf("SQL query touched %s row(s).", result))
	}
}

func cmdSQLInit(g *Game, d *Descriptor, _ string, _ []string) {
	// @sqlinit — God-only, re-opens SQL connection
	if !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if g.SQLDB == nil {
		g.Notify(d.Player, "SQL is not configured.")
		return
	}
	if err := g.SQLDB.Reconnect(); err != nil {
		g.Notify(d.Player, fmt.Sprintf("SQL reconnect failed: %s", err))
		return
	}
	g.Notify(d.Player, "SQL connection re-initialized.")
}

func cmdSQLDisconnect(g *Game, d *Descriptor, _ string, _ []string) {
	// @sqldisconnect — God-only, closes SQL connection
	if !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if g.SQLDB == nil {
		g.Notify(d.Player, "SQL is not configured.")
		return
	}
	if err := g.SQLDB.Close(); err != nil {
		g.Notify(d.Player, fmt.Sprintf("SQL disconnect failed: %s", err))
		return
	}
	g.SQLDB = nil
	g.Notify(d.Player, "SQL connection closed.")
}

func cmdPower(g *Game, d *Descriptor, args string, _ []string) {
	// @power obj = [!]powername
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @power object = [!]power")
		return
	}
	targetStr := strings.TrimSpace(args[:eqIdx])
	powStr := strings.TrimSpace(args[eqIdx+1:])

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

	// God protection
	if IsGod(g, target) && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
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
		g.Notify(d.Player, "I don't know that power.")
		return
	}

	// God-only powers (ph_god in C): control_all, cloak, guest, use_sql
	if pe.GodOnly && !IsGod(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	obj.SetPower(pe.Word, pe.Bit, !negate)
	g.PersistObject(obj)
	if negate {
		g.Notify(d.Player, fmt.Sprintf("Power %s removed from %s(#%d).", powStr, obj.Name, target))
	} else {
		g.Notify(d.Player, fmt.Sprintf("Power %s granted to %s(#%d).", powStr, obj.Name, target))
	}
}

// cmdFunction implements @function[/privileged][/preserve][/delete] name=obj/attr
// Registers a global softcode-defined function.
func cmdFunction(g *Game, d *Descriptor, args string, switches []string) {
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
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
				g.Notify(d.Player, "Usage: @function/delete <name>")
				return
			}
			if _, ok := g.GameFuncs[funcName]; ok {
				delete(g.GameFuncs, funcName)
				g.Notify(d.Player, fmt.Sprintf("Function %s deleted.", funcName))
			} else {
				g.Notify(d.Player, fmt.Sprintf("No @function named %s.", funcName))
			}
			return
		}
		// List all @functions
		if len(g.GameFuncs) == 0 {
			g.Notify(d.Player, "No @functions defined.")
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
			g.Notify(d.Player, fmt.Sprintf("  %s = #%d/%d%s", name, uf.Obj, uf.Attr, flags))
		}
		return
	}

	funcName := strings.ToUpper(strings.TrimSpace(args[:eqIdx]))
	objAttr := strings.TrimSpace(args[eqIdx+1:])

	if funcName == "" {
		g.Notify(d.Player, "Usage: @function[/privileged] <name> = <obj>/<attr>")
		return
	}

	// Handle deletion via empty value
	if objAttr == "" {
		if _, ok := g.GameFuncs[funcName]; ok {
			delete(g.GameFuncs, funcName)
			g.Notify(d.Player, fmt.Sprintf("Function %s deleted.", funcName))
		} else {
			g.Notify(d.Player, fmt.Sprintf("No @function named %s.", funcName))
		}
		return
	}

	// Parse obj/attr
	slashIdx := strings.IndexByte(objAttr, '/')
	if slashIdx < 0 {
		g.Notify(d.Player, "Usage: @function[/privileged] <name> = <obj>/<attr>")
		return
	}
	objStr := strings.TrimSpace(objAttr[:slashIdx])
	attrName := strings.ToUpper(strings.TrimSpace(objAttr[slashIdx+1:]))

	target := g.MatchObject(d.Player, objStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}

	// Resolve attr number
	attrNum := g.LookupAttrNum(attrName)
	if attrNum < 0 {
		g.Notify(d.Player, fmt.Sprintf("No such attribute: %s", attrName))
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
	g.Notify(d.Player, fmt.Sprintf("Function %s defined.", funcName))
}

// cmdDrain implements @drain <obj>[/<attr>]
// Removes all wait queue entries belonging to the object, and resets its semaphore count.
func cmdDrain(g *Game, d *Descriptor, args string, _ []string) {
	args = strings.TrimSpace(args)
	if args == "" {
		// C: noisy_match_result("") + explicit "No match."
		g.Notify(d.Player, "I don't see that here.")
		g.Notify(d.Player, "No match.")
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
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// Drain semaphore entries from the queue
	semAttr := gamedb.A_SEMAPHORE
	if attrName != "" {
		num := g.LookupAttrNum(attrName)
		if num >= 0 {
			semAttr = num
		}
	}

	g.Queue.DrainObject(target, semAttr)

	// Reset the semaphore count on the object (clear attr)
	g.SetAttr(target, semAttr, "")

	// C: just "Drained." with no details
	g.Notify(d.Player, "Drained.")
}

// --- Archive Commands ---

// cmdArchive implements @archive and @archive/list.
func cmdArchive(g *Game, d *Descriptor, args string, switches []string) {
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
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

	g.Notify(d.Player, "Creating archive...")
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
		g.Notify(d.Player, fmt.Sprintf("Error listing archives: %v", err))
		return
	}
	if len(archives) == 0 {
		g.Notify(d.Player, fmt.Sprintf("No archives found in %s.", archiveDir))
		return
	}

	g.Notify(d.Player, fmt.Sprintf("Archives in %s:", archiveDir))
	for _, ai := range archives {
		sizeMB := float64(ai.Size) / (1024 * 1024)
		if ai.Objects > 0 {
			g.Notify(d.Player, fmt.Sprintf("  %s  %.1f MB  %d objects  %s", ai.Filename, sizeMB, ai.Objects, ai.Timestamp))
		} else {
			g.Notify(d.Player, fmt.Sprintf("  %s  %.1f MB  %s", ai.Filename, sizeMB, ai.Timestamp))
		}
	}
	g.Notify(d.Player, fmt.Sprintf("%d archive(s).", len(archives)))
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
		g.Notify(d.Player, "Permission denied.")
		return
	}
	if g.Conf == nil {
		g.Notify(d.Player, "No game configuration loaded.")
		return
	}

	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		// Show a param value
		param := strings.TrimSpace(args)
		if param == "" {
			g.Notify(d.Player, "Usage: @admin param=value")
			return
		}
		val, ok := getAdminParam(g.Conf, param)
		if !ok {
			g.Notify(d.Player, fmt.Sprintf("Unknown parameter: %s", param))
			return
		}
		g.Notify(d.Player, fmt.Sprintf("%s = %s", param, val))
		return
	}

	param := strings.TrimSpace(args[:eqIdx])
	value := strings.TrimSpace(args[eqIdx+1:])

	ok := setAdminParam(g.Conf, param, value)
	if !ok {
		g.Notify(d.Player, fmt.Sprintf("Unknown parameter: %s", param))
		return
	}
	g.Notify(d.Player, fmt.Sprintf("Set: %s = %s", param, value))
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
	case "create_min_cost":
		return strconv.Itoa(c.CreateMinCost), true
	case "create_max_cost":
		return strconv.Itoa(c.CreateMaxCost), true
	case "dig_cost":
		return strconv.Itoa(c.DigCost), true
	case "open_cost":
		return strconv.Itoa(c.OpenCost), true
	case "robot_cost":
		return strconv.Itoa(c.RobotCost), true
	case "sacrifice_adjust":
		return strconv.Itoa(c.SacrificeAdjust), true
	case "sacrifice_factor":
		return strconv.Itoa(c.SacrificeFactor), true
	case "machine_command_cost":
		return strconv.Itoa(c.MachineCommandCost), true
	case "trace_topdown":
		if c.TraceTopdown { return "1", true }
		return "0", true
	case "trace_output_limit":
		return strconv.Itoa(c.TraceOutputLimit), true
	case "idle_timeout":
		return strconv.Itoa(c.IdleTimeout), true
	case "keepalive_interval":
		return strconv.Itoa(c.KeepaliveInterval), true
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
	case "function_access":
		// Show all configured function access overrides
		if c.FunctionAccess == nil || len(c.FunctionAccess) == 0 {
			return "(none)", true
		}
		var parts []string
		for name, level := range c.FunctionAccess {
			parts = append(parts, strings.ToUpper(name)+":"+level)
		}
		return strings.Join(parts, " "), true
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
	case "create_min_cost":
		c.CreateMinCost, _ = strconv.Atoi(value); return true
	case "create_max_cost":
		c.CreateMaxCost, _ = strconv.Atoi(value); return true
	case "dig_cost":
		c.DigCost, _ = strconv.Atoi(value); return true
	case "open_cost":
		c.OpenCost, _ = strconv.Atoi(value); return true
	case "robot_cost":
		c.RobotCost, _ = strconv.Atoi(value); return true
	case "sacrifice_adjust":
		c.SacrificeAdjust, _ = strconv.Atoi(value); return true
	case "sacrifice_factor":
		c.SacrificeFactor, _ = strconv.Atoi(value); return true
	case "machine_command_cost":
		c.MachineCommandCost, _ = strconv.Atoi(value); return true
	case "trace_topdown":
		c.TraceTopdown = parseBoolAdmin(value, negate); return true
	case "trace_output_limit":
		c.TraceOutputLimit, _ = strconv.Atoi(value); return true
	case "idle_timeout":
		c.IdleTimeout, _ = strconv.Atoi(value); return true
	case "keepalive_interval":
		c.KeepaliveInterval, _ = strconv.Atoi(value); return true
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
	case "function_access":
		// @admin function_access=FUNCNAME LEVEL
		// e.g.: @admin function_access=FORCE wizard
		// e.g.: @admin function_access=BEEP disabled
		// e.g.: @admin function_access=FORCE public  (reset to default)
		parts := strings.Fields(value)
		if len(parts) != 2 {
			return false
		}
		funcName := strings.ToUpper(parts[0])
		level := strings.ToLower(parts[1])
		if level != "public" && level != "wizard" && level != "god" && level != "disabled" {
			return false
		}
		if c.FunctionAccess == nil {
			c.FunctionAccess = make(map[string]string)
		}
		if level == "public" {
			delete(c.FunctionAccess, funcName)
		} else {
			c.FunctionAccess[funcName] = level
		}
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
		g.Notify(d.Player, "Permission denied.")
		return
	}

	if len(switches) == 0 {
		g.Notify(d.Player, "Usage: @attribute/access <attr>=<flags>")
		return
	}

	sw := strings.ToLower(switches[0])

	switch sw {
	case "access":
		// @attribute/access <name>=<flags>
		parts := strings.SplitN(args, "=", 2)
		if len(parts) != 2 {
			g.Notify(d.Player, "Usage: @attribute/access <attr>=<flags>")
			return
		}
		attrName := strings.TrimSpace(strings.ToUpper(parts[0]))
		flagStr := strings.TrimSpace(parts[1])

		if attrName == "" {
			g.Notify(d.Player, "Specify an attribute name.")
			return
		}

		// Look up the attribute definition
		def, ok := g.DB.AttrByName[attrName]
		if !ok {
			// Also check well-known attrs (can't modify their flags)
			for _, wkName := range gamedb.WellKnownAttrs {
				if strings.EqualFold(wkName, attrName) {
					g.Notify(d.Player, "Cannot modify access on built-in attributes.")
					return
				}
			}
			g.Notify(d.Player, "No such user-named attribute.")
			return
		}

		setFlags, clearFlags, errs := parseAttrAccessFlags(flagStr)
		for _, e := range errs {
			g.Notify(d.Player, fmt.Sprintf("Unknown permission: %s.", e))
		}

		if setFlags != 0 || clearFlags != 0 {
			def.Flags = (def.Flags &^ clearFlags) | setFlags
			// Persist to store
			if g.Store != nil {
				g.Store.PutMeta()
			}
			g.Notify(d.Player, "Attribute access changed.")
		}

	case "rename":
		// @attribute/rename <old>=<new>
		parts := strings.SplitN(args, "=", 2)
		if len(parts) != 2 {
			g.Notify(d.Player, "Usage: @attribute/rename <old>=<new>")
			return
		}
		oldName := strings.TrimSpace(strings.ToUpper(parts[0]))
		newName := strings.TrimSpace(strings.ToUpper(parts[1]))

		def, ok := g.DB.AttrByName[oldName]
		if !ok {
			g.Notify(d.Player, "No such user-named attribute.")
			return
		}
		if _, exists := g.DB.AttrByName[newName]; exists {
			g.Notify(d.Player, "An attribute with that name already exists.")
			return
		}

		delete(g.DB.AttrByName, oldName)
		def.Name = newName
		g.DB.AttrByName[newName] = def
		if g.Store != nil {
			g.Store.PutMeta()
		}
		g.Notify(d.Player, "Attribute renamed.")

	case "delete":
		attrName := strings.TrimSpace(strings.ToUpper(args))
		if attrName == "" {
			g.Notify(d.Player, "Usage: @attribute/delete <attr>")
			return
		}
		def, ok := g.DB.AttrByName[attrName]
		if !ok {
			g.Notify(d.Player, "No such user-named attribute.")
			return
		}
		delete(g.DB.AttrByName, attrName)
		delete(g.DB.AttrNames, def.Number)
		if g.Store != nil {
			g.Store.PutMeta()
		}
		g.Notify(d.Player, "Attribute deleted.")

	case "propagate":
		cmdAttributePropagate(g, d, args)

	default:
		g.Notify(d.Player, "Unknown switch. Use: @attribute/access, @attribute/rename, @attribute/delete, @attribute/propagate")
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
			g.Notify(d.Player, "Usage: @attlist/detail <attrname> [type=<player|thing|room|exit>]")
			return
		}
		attrName := strings.ToUpper(pattern)
		def, ok := g.DB.AttrByName[attrName]
		if !ok {
			g.Notify(d.Player, "No such attribute definition.")
			return
		}
		results := findObjectsWithAttr(g, d.Player, def.Number, typeFilter, isWiz, 100)
		typeName := ""
		if typeFilter >= 0 {
			typeName = " (" + gamedb.ObjectType(typeFilter).String() + " only)"
		}
		g.Notify(d.Player, fmt.Sprintf("--- Objects with %s%s ---", attrName, typeName))
		for _, ref := range results {
			obj := g.DB.Objects[ref]
			if obj != nil {
				g.Notify(d.Player, fmt.Sprintf("  #%d  %s (%s)", ref, DisplayName(obj.Name), obj.ObjType().String()))
			}
		}
		total := len(results)
		if total >= 100 {
			g.Notify(d.Player, fmt.Sprintf("--- Showing first 100 of possibly more ---"))
		} else {
			g.Notify(d.Player, fmt.Sprintf("--- %d object(s) ---", total))
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
			g.Notify(d.Player, "No matching configured attributes found.")
		} else {
			g.Notify(d.Player, "No configured attributes defined.")
		}
		return
	}

	// Header
	typeName := ""
	if typeFilter >= 0 {
		typeName = " on " + gamedb.ObjectType(typeFilter).String() + " objects"
	}
	if pattern == "" && typeFilter < 0 {
		g.Notify(d.Player, fmt.Sprintf("--- Configured Attributes (%d) ---", len(results)))
	} else {
		g.Notify(d.Player, fmt.Sprintf("--- Configured Attributes%s (%d) ---", typeName, len(results)))
	}
	for _, e := range results {
		flagStr := attrFlagString(e.flags)
		if flagStr != "" {
			flagStr = "[" + flagStr + "]"
		} else {
			flagStr = "[-]"
		}
		g.Notify(d.Player, fmt.Sprintf("  %-30s %-8s %d", e.name, flagStr, e.count))
	}
	g.Notify(d.Player, fmt.Sprintf("--- %d attribute(s) listed ---", len(results)))
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
		g.Notify(d.Player, "Usage: @attribute/propagate <attr>=<target>[/<default value>]")
		return
	}

	attrName := strings.TrimSpace(strings.ToUpper(parts[0]))
	rest := strings.TrimSpace(parts[1])

	// Resolve attribute number
	attrNum := g.ResolveAttrNum(attrName)
	if attrNum < 0 {
		g.Notify(d.Player, fmt.Sprintf("Unknown attribute: %s", attrName))
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
			g.Notify(d.Player, "Target must be a #dbref, PLAYER, THING, ROOM, EXIT, or ALL.")
			return
		}
		if _, ok := g.DB.Objects[parentRef]; !ok {
			g.Notify(d.Player, fmt.Sprintf("Parent object #%d not found.", parentRef))
			return
		}
		for ref, obj := range g.DB.Objects {
			if obj.Parent == parentRef {
				targets = append(targets, ref)
			}
		}
	}

	if len(targets) == 0 {
		g.Notify(d.Player, "No matching objects found.")
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

	g.Notify(d.Player, fmt.Sprintf("Propagated %s to %d object(s) (%d already had it, %d total checked).",
		attrName, set, skipped, len(targets)))
}
