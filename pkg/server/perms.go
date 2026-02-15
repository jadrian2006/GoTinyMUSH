package server

import (
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// A_LCONTROL is the attribute number for the control lock.
const aLControl = 129

// IsGod returns true if player is the God player.
func IsGod(g *Game, player gamedb.DBRef) bool {
	return player == g.GodPlayer()
}

// Inherits returns true if obj inherits privilege from its owner.
// Players always inherit. Non-players inherit if they have INHERIT set,
// or their owner has INHERIT set, or they are their own owner.
func Inherits(g *Game, obj gamedb.DBRef) bool {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false
	}
	// Players always inherit their own privileges
	if o.ObjType() == gamedb.TypePlayer {
		return true
	}
	// Object has INHERIT flag
	if o.HasFlag(gamedb.FlagInherit) {
		return true
	}
	// Object is its own owner (shouldn't happen for non-players, but check)
	if o.Owner == obj {
		return true
	}
	// Owner has INHERIT flag
	if ownerObj, ok := g.DB.Objects[o.Owner]; ok {
		return ownerObj.HasFlag(gamedb.FlagInherit)
	}
	return false
}

// Wizard returns true if obj is an effective wizard.
// Matches C TinyMUSH: Wizard(x) = has WIZARD flag directly, OR owner has
// WIZARD and object Inherits. Players always inherit their own privileges.
func Wizard(g *Game, obj gamedb.DBRef) bool {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false
	}
	if o.HasFlag(gamedb.FlagWizard) {
		return true
	}
	// Check if owner has WIZARD and object inherits
	owner, ownerOK := g.DB.Objects[o.Owner]
	if ownerOK && owner.HasFlag(gamedb.FlagWizard) && Inherits(g, obj) {
		return true
	}
	return false
}

// Royalty returns true if obj has the ROYALTY flag.
// Unlike Wizard, Royalty does NOT require Inherits — matches C TinyMUSH.
func Royalty(g *Game, obj gamedb.DBRef) bool {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false
	}
	return o.HasFlag(gamedb.FlagRoyalty)
}

// WizRoy returns true if obj is either an effective wizard or royalty.
func WizRoy(g *Game, obj gamedb.DBRef) bool {
	return Wizard(g, obj) || Royalty(g, obj)
}

// ControlAll returns true if obj has POW_CONTROL_ALL or is an effective wizard.
func ControlAll(g *Game, obj gamedb.DBRef) bool {
	if Wizard(g, obj) {
		return true
	}
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false
	}
	return o.HasPower(0, gamedb.PowControlAll)
}

// SeeAll returns true if obj has POW_EXAM_ALL or is effective WizRoy.
func SeeAll(g *Game, obj gamedb.DBRef) bool {
	if WizRoy(g, obj) {
		return true
	}
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false
	}
	return o.HasPower(0, gamedb.PowExamAll)
}

// CheckZone checks if player passes the zone control lock chain for thing.
// This implements TinyMUSH's recursive zone-based control:
// 1. thing must not be a player
// 2. thing must have CONTROL_OK flag
// 3. thing's zone master object (ZMO) must have A_LCONTROL set
// 4. player must pass the ZMO's control lock
// 5. If fail, recurse on ZMO's zone (up to nesting limit)
func CheckZone(g *Game, player, thing gamedb.DBRef, depth int) bool {
	if depth > g.ZoneNestLimit() {
		return false
	}

	tObj, ok := g.DB.Objects[thing]
	if !ok {
		return false
	}

	// Players can't be zone-controlled
	if tObj.ObjType() == gamedb.TypePlayer {
		return false
	}

	// Must have CONTROL_OK flag
	if !tObj.HasFlag2(gamedb.Flag2ControlOK) {
		return false
	}

	// Must have a zone
	if tObj.Zone == gamedb.Nothing {
		return false
	}

	zmo := tObj.Zone

	// ZMO must have A_LCONTROL set
	lockText := g.GetAttrTextDirect(zmo, aLControl)
	if lockText == "" {
		return false
	}

	// Player must pass the ZMO's control lock
	parsed := ParseBoolExp(g, player, lockText)
	if EvalBoolExp(g, player, zmo, zmo, parsed, 0) {
		return true
	}

	// Recurse on ZMO's zone
	zmoObj, ok := g.DB.Objects[zmo]
	if !ok || zmoObj.Zone == gamedb.Nothing {
		return false
	}
	return CheckZone(g, player, zmo, depth+1)
}

// Examinable returns true if player can examine target.
// True if: target has VISUAL flag, or player has SeeAll, or same owner, or zone control.
func Examinable(g *Game, player, target gamedb.DBRef) bool {
	// VISUAL flag means anyone can examine
	if tObj, ok := g.DB.Objects[target]; ok {
		if tObj.HasFlag(gamedb.FlagVisual) {
			return true
		}
	}

	// SeeAll (POW_EXAM_ALL or WizRoy)
	if SeeAll(g, player) {
		return true
	}

	// Same owner
	pObj, ok1 := g.DB.Objects[player]
	tObj, ok2 := g.DB.Objects[target]
	if ok1 && ok2 && pObj.Owner == tObj.Owner {
		return true
	}

	// Zone control
	return CheckZone(g, player, target, 0)
}

// Controls returns true if player controls target, using the full TinyMUSH logic:
// 1. God protection: can't control god unless you ARE god
// 2. ControlAll (POW_CONTROL_ALL or Wizard)
// 3. Same owner AND (player Inherits OR target doesn't Inherit)
// 4. Zone-based control
func Controls(g *Game, player, target gamedb.DBRef) bool {
	// Identity always controls
	if player == target {
		return true
	}

	// God protection: nobody controls God except God
	if IsGod(g, target) && !IsGod(g, player) {
		return false
	}

	// ControlAll (wizard or POW_CONTROL_ALL)
	if ControlAll(g, player) {
		return true
	}

	// Ownership-based control
	_, ok1 := g.DB.Objects[player]
	tObj, ok2 := g.DB.Objects[target]
	if ok1 && ok2 && tObj.Owner == player {
		// Owner controls if player Inherits or target doesn't Inherit
		if Inherits(g, player) || !Inherits(g, target) {
			return true
		}
	}

	// Zone-based control
	return CheckZone(g, player, target, 0)
}

// SeesHiddenAttrs returns true if player can see AF_MDARK attributes.
// Requires POW_MDARK_ATTR or WizRoy.
func SeesHiddenAttrs(g *Game, player gamedb.DBRef) bool {
	if WizRoy(g, player) {
		return true
	}
	o, ok := g.DB.Objects[player]
	if !ok {
		return false
	}
	return o.HasPower(0, gamedb.PowMdarkAttr)
}

// SetsWizAttrs returns true if player can modify AF_WIZARD attributes.
// Requires POW_WIZ_ATTR or Wizard.
func SetsWizAttrs(g *Game, player gamedb.DBRef) bool {
	if Wizard(g, player) {
		return true
	}
	o, ok := g.DB.Objects[player]
	if !ok {
		return false
	}
	return o.HasPower(0, gamedb.PowWizAttr)
}

// CanReadAttr checks if player can read an attribute on target.
// Implements C TinyMUSH's See_attr logic:
//  1. AF_INTERNAL → never visible
//  2. AF_IS_LOCK → not visible
//  3. AF_VISUAL → visible to anyone
//  4. Not Examinable AND player doesn't own attr → blocked
//  5. AF_MDARK AND not SeesHiddenAttrs → blocked
//  6. AF_DARK AND not God → blocked
func CanReadAttr(g *Game, player, target gamedb.DBRef, attrDef *gamedb.AttrDef, instFlags int, attrOwner gamedb.DBRef) bool {
	// Merge definition flags and per-instance flags
	defFlags := 0
	if attrDef != nil {
		defFlags = attrDef.Flags
	}
	merged := defFlags | instFlags

	// AF_INTERNAL — never visible to anyone
	if merged&gamedb.AFInternal != 0 {
		return false
	}

	// AF_IS_LOCK — not visible via examine/get
	if merged&gamedb.AFIsLock != 0 {
		return false
	}

	// AF_VISUAL — anyone can see
	if merged&gamedb.AFVisual != 0 {
		return true
	}

	// God can see everything that's not AF_INTERNAL
	if IsGod(g, player) {
		return true
	}

	// Must be examinable OR player owns the attr
	if !Examinable(g, player, target) && attrOwner != player {
		return false
	}

	// AF_MDARK — only SeesHiddenAttrs can see
	if merged&gamedb.AFMDark != 0 && !SeesHiddenAttrs(g, player) {
		return false
	}

	// AF_DARK — only God can see (and we already returned for God above)
	if merged&gamedb.AFDark != 0 {
		return false
	}

	return true
}

// CanSetAttr checks if player can write an attribute on target.
// Implements C TinyMUSH's Set_attr logic:
//  1. AF_INTERNAL, AF_IS_LOCK, AF_CONST → never writable
//  2. God can always write
//  3. Target is God → blocked (for non-God)
//  4. AF_LOCK (per-instance) → blocked (for non-God)
//  5. Controls(player, target) with flag checks
func CanSetAttr(g *Game, player, target gamedb.DBRef, attrDef *gamedb.AttrDef, instFlags int) bool {
	defFlags := 0
	if attrDef != nil {
		defFlags = attrDef.Flags
	}
	merged := defFlags | instFlags

	// Never writable
	if merged&gamedb.AFInternal != 0 || merged&gamedb.AFIsLock != 0 || merged&gamedb.AFConst != 0 {
		return false
	}

	// God can always write
	if IsGod(g, player) {
		return true
	}

	// Cannot modify attrs on God
	if IsGod(g, target) {
		return false
	}

	// AF_GOD — only God can change
	if merged&gamedb.AFGod != 0 {
		return false
	}

	// AF_LOCK (per-instance) — locked against modification
	if instFlags&gamedb.AFLock != 0 {
		return false
	}

	// Must control the target
	if !Controls(g, player, target) {
		return false
	}

	// AF_WIZARD — need SetsWizAttrs
	if merged&gamedb.AFWizard != 0 && !SetsWizAttrs(g, player) {
		return false
	}

	return true
}

// CheckZoneForPlayer checks if player passes the zone control lock chain
// when thing IS a player. Unlike CheckZone, this allows thing to be a player
// and checks CONTROL_OK on the zone master object instead of thing itself.
// Used by @chzone to allow zone controllers to change zones on objects.
func CheckZoneForPlayer(g *Game, player, thing gamedb.DBRef, depth int) bool {
	if depth > g.ZoneNestLimit() {
		return false
	}
	tObj, ok := g.DB.Objects[thing]
	if !ok || tObj.ObjType() != gamedb.TypePlayer {
		return false
	}
	if tObj.Zone == gamedb.Nothing {
		return false
	}
	zmo := tObj.Zone
	zmoObj, ok := g.DB.Objects[zmo]
	if !ok || !zmoObj.HasFlag2(gamedb.Flag2ControlOK) {
		return false
	}
	// ZMO must have A_LCONTROL set
	lockText := g.GetAttrTextDirect(zmo, aLControl)
	if lockText == "" {
		return false
	}
	parsed := ParseBoolExp(g, player, lockText)
	if EvalBoolExp(g, player, zmo, zmo, parsed, 0) {
		return true
	}
	// Recurse via standard CheckZone on the ZMO
	return CheckZone(g, player, zmo, depth+1)
}

// StripPrivFlags strips privilege flags from non-player objects.
// Matches C TinyMUSH's stripped_flags behavior: removes IMMORTAL, INHERIT,
// ROYALTY, WIZARD from word 0; BLIND, CONNECTED, GAGGED, SLAVE, STAFF,
// STOP_MATCH from word 1; and clears all powers.
func StripPrivFlags(g *Game, obj gamedb.DBRef) {
	o, ok := g.DB.Objects[obj]
	if !ok || o.ObjType() == gamedb.TypePlayer {
		return
	}
	o.Flags[0] &^= gamedb.FlagImmortal | gamedb.FlagInherit |
		gamedb.FlagRoyalty | gamedb.FlagWizard
	o.Flags[1] &^= gamedb.Flag2Blind | gamedb.Flag2Connected |
		gamedb.Flag2Gagged | gamedb.Flag2Slave |
		gamedb.Flag2Staff | gamedb.Flag2StopMatch
	o.Powers = [2]int{0, 0}
	g.PersistObject(o)
}

// PassLocks returns true if player has the POW_PASS_LOCKS power.
func PassLocks(g *Game, player gamedb.DBRef) bool {
	o, ok := g.DB.Objects[player]
	if !ok {
		return false
	}
	return o.HasPower(0, gamedb.PowPassLocks)
}
