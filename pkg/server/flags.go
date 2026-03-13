package server

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Flag permission levels (matching C TinyMUSH fh_* handlers)
const (
	FlagPermAny    = 0 // Anyone who controls the object
	FlagPermWiz    = 1 // Wizards (or God) only
	FlagPermWizRoy = 2 // Wizards, Royalty (or God)
	FlagPermGod    = 3 // God (#1) only
)

// FlagDef maps a flag name to its word index and bit mask.
type FlagDef struct {
	Name string
	Word int // 0, 1, or 2 (flag word index)
	Bit  int
	Perm int // Required permission level (FlagPermAny, FlagPermWiz, FlagPermGod)
}

// FlagTable is the complete flag name -> definition table.
// Perm values match C TinyMUSH fh_* handlers from flags.c.
var FlagTable = map[string]*FlagDef{
	// Flag word 0 — permissions from C flags.c gen_flags[]
	"WIZARD":       {Name: "WIZARD", Word: 0, Bit: gamedb.FlagWizard, Perm: FlagPermGod},
	"DARK":         {Name: "DARK", Word: 0, Bit: gamedb.FlagDark},           // fh_dark_bit: non-wiz can only set on exits
	"HAVEN":        {Name: "HAVEN", Word: 0, Bit: gamedb.FlagHaven},
	"HALT":         {Name: "HALTED", Word: 0, Bit: gamedb.FlagHalt},
	"SAFE":         {Name: "SAFE", Word: 0, Bit: gamedb.FlagSafe},
	"INHERIT":      {Name: "INHERIT", Word: 0, Bit: gamedb.FlagInherit},     // fh_inherit: setter must Inherits()
	"NOSPOOF":      {Name: "NOSPOOF", Word: 0, Bit: gamedb.FlagNoSpoof},
	"VISUAL":       {Name: "VISUAL", Word: 0, Bit: gamedb.FlagVisual},
	"OPAQUE":       {Name: "OPAQUE", Word: 0, Bit: gamedb.FlagOpaque},
	"QUIET":        {Name: "QUIET", Word: 0, Bit: gamedb.FlagQuiet},
	"PUPPET":       {Name: "PUPPET", Word: 0, Bit: gamedb.FlagPuppet},
	"STICKY":       {Name: "STICKY", Word: 0, Bit: gamedb.FlagSticky},
	"MONITOR":      {Name: "MONITOR", Word: 0, Bit: gamedb.FlagMonitor},
	"ROBOT":        {Name: "ROBOT", Word: 0, Bit: gamedb.FlagRobot},
	"ROYALTY":      {Name: "ROYALTY", Word: 0, Bit: gamedb.FlagRoyalty, Perm: FlagPermWiz},
	"ENTER_OK":     {Name: "ENTER_OK", Word: 0, Bit: gamedb.FlagEnterOK},
	"LINK_OK":      {Name: "LINK_OK", Word: 0, Bit: gamedb.FlagLinkOK},
	"JUMP_OK":      {Name: "JUMP_OK", Word: 0, Bit: gamedb.FlagJumpOK},
	"VERBOSE":      {Name: "VERBOSE", Word: 0, Bit: gamedb.FlagVerbose},
	"TERSE":        {Name: "TERSE", Word: 0, Bit: gamedb.FlagTerse},
	"TRACE":        {Name: "TRACE", Word: 0, Bit: gamedb.FlagTrace},
	"GOING":        {Name: "GOING", Word: 0, Bit: gamedb.FlagGoing},
	"MYOPIC":       {Name: "MYOPIC", Word: 0, Bit: gamedb.FlagMyopic},
	"CHOWN_OK":     {Name: "CHOWN_OK", Word: 0, Bit: gamedb.FlagChownOK},
	"DESTROY_OK":   {Name: "DESTROY_OK", Word: 0, Bit: gamedb.FlagDestroyOK},
	"SEE_THROUGH":  {Name: "TRANSPARENT", Word: 0, Bit: gamedb.FlagSeeThru},  // Go alias
	"HEAR_THROUGH": {Name: "AUDIBLE", Word: 0, Bit: gamedb.FlagHearThru},     // Go alias
	"AUDIBLE":      {Name: "AUDIBLE", Word: 0, Bit: gamedb.FlagHearThru},
	"IMMORTAL":     {Name: "IMMORTAL", Word: 0, Bit: gamedb.FlagImmortal, Perm: FlagPermWiz},
	"HAS_STARTUP":  {Name: "HAS_STARTUP", Word: 0, Bit: gamedb.FlagHasStartup},

	// Flag word 1
	"ABODE":        {Name: "ABODE", Word: 1, Bit: gamedb.Flag2Abode},
	"FLOATING":     {Name: "FREE", Word: 1, Bit: gamedb.Flag2Floating},       // Go alias
	"UNFINDABLE":   {Name: "UNFINDABLE", Word: 1, Bit: gamedb.Flag2Unfindable},
	"PARENT_OK":    {Name: "PARENT_OK", Word: 1, Bit: gamedb.Flag2ParentOK},
	"LIGHT":        {Name: "LIGHT", Word: 1, Bit: gamedb.Flag2Light},
	"HAS_LISTEN":   {Name: "HAS_LISTEN", Word: 1, Bit: gamedb.Flag2HasListen},
	"AUDITORIUM":   {Name: "AUDITORIUM", Word: 1, Bit: gamedb.Flag2Auditorium},
	"OOB":          {Name: "OOB", Word: 1, Bit: gamedb.Flag2OOB},
	"ANSI":         {Name: "ANSI", Word: 1, Bit: gamedb.Flag2Ansi},
	"HEAD":         {Name: "HEAD", Word: 1, Bit: gamedb.Flag2HeadFlag},
	"FIXED":        {Name: "FIXED", Word: 1, Bit: gamedb.Flag2Fixed},
	"UNINSPECTED":  {Name: "UNINSPECTED", Word: 1, Bit: gamedb.Flag2Uninspected, Perm: FlagPermWizRoy},
	"ZONE_PARENT":  {Name: "ZONE", Word: 1, Bit: gamedb.Flag2ZoneParent},      // Go alias
	"NOBLEED":      {Name: "NOBLEED", Word: 1, Bit: gamedb.Flag2NoBLeed},
	"NO_BLEED":     {Name: "NOBLEED", Word: 1, Bit: gamedb.Flag2NoBLeed},
	"STAFF":        {Name: "STAFF", Word: 1, Bit: gamedb.Flag2Staff, Perm: FlagPermWiz},
	"HAS_DAILY":    {Name: "HAS_DAILY", Word: 1, Bit: gamedb.Flag2HasDaily},
	"GAGGED":       {Name: "GAGGED", Word: 1, Bit: gamedb.Flag2Gagged, Perm: FlagPermWiz},
	"HAS_COMMANDS": {Name: "COMMANDS", Word: 1, Bit: gamedb.Flag2HasCommands}, // Go alias
	"STOP":         {Name: "STOP", Word: 1, Bit: gamedb.Flag2StopMatch},
	"BOUNCE":       {Name: "BOUNCE", Word: 1, Bit: gamedb.Flag2Bounce},
	"CONTROL_OK":   {Name: "CONTROL_OK", Word: 1, Bit: gamedb.Flag2ControlOK},
	"VACATION":     {Name: "VACATION", Word: 1, Bit: gamedb.Flag2Vacation},
	"HTML":         {Name: "HTML", Word: 1, Bit: gamedb.Flag2HTML},
	"BLIND":        {Name: "BLIND", Word: 1, Bit: gamedb.Flag2Blind, Perm: FlagPermWiz},
	"SUSPECT":      {Name: "SUSPECT", Word: 1, Bit: gamedb.Flag2Suspect, Perm: FlagPermWiz},
	"WATCHER":      {Name: "WATCHER", Word: 1, Bit: gamedb.Flag2Watcher},
	"CONNECTED":    {Name: "CONNECTED", Word: 1, Bit: gamedb.Flag2Connected, Perm: FlagPermGod},
	"SLAVE":        {Name: "SLAVE", Word: 1, Bit: gamedb.Flag2Slave, Perm: FlagPermWiz},

	// C-compatible canonical entries (word 0)
	"HALTED":       {Name: "HALTED", Word: 0, Bit: gamedb.FlagHalt},
	"TRANSPARENT":  {Name: "TRANSPARENT", Word: 0, Bit: gamedb.FlagSeeThru},

	// C-compatible canonical entries (word 1)
	"KEY":          {Name: "KEY", Word: 1, Bit: gamedb.Flag2Key},
	"FREE":         {Name: "FREE", Word: 1, Bit: gamedb.Flag2Floating},
	"ZONE":         {Name: "ZONE", Word: 1, Bit: gamedb.Flag2ZoneParent},
	"COMMANDS":     {Name: "COMMANDS", Word: 1, Bit: gamedb.Flag2HasCommands},
	"CONSTANT":     {Name: "CONSTANT", Word: 1, Bit: gamedb.Flag2ConstAttrs, Perm: FlagPermGod}, // internal
	"HAS_FORWARDLIST": {Name: "HAS_FORWARDLIST", Word: 1, Bit: gamedb.Flag2HasFwd, Perm: FlagPermGod}, // internal
	"PLAYER_MAILS": {Name: "PLAYER_MAILS", Word: 1, Bit: gamedb.Flag2PlayerMails, Perm: FlagPermGod}, // internal

	// Flag word 2
	"INSTANCE":     {Name: "INSTANCE", Word: 2, Bit: gamedb.Flag3Instance},
	"REDIR_OK":     {Name: "REDIR_OK", Word: 2, Bit: gamedb.Flag3RedirOK},
	"HAS_REDIRECT": {Name: "HAS_REDIRECT", Word: 2, Bit: gamedb.Flag3HasRedirect, Perm: FlagPermGod}, // internal
	"ORPHAN":       {Name: "ORPHAN", Word: 2, Bit: gamedb.Flag3Orphan},
	"HAS_DARKLOCK": {Name: "HAS_DARKLOCK", Word: 2, Bit: gamedb.Flag3HasDarkLock, Perm: FlagPermGod}, // internal
	"PRESENCE":     {Name: "PRESENCE", Word: 2, Bit: gamedb.Flag3Presence},
	"SPEECHMOD":    {Name: "SPEECHMOD", Word: 2, Bit: gamedb.Flag3HasSpeechMod, Perm: FlagPermGod}, // internal
	"HAS_SPEECHMOD": {Name: "SPEECHMOD", Word: 2, Bit: gamedb.Flag3HasSpeechMod, Perm: FlagPermGod}, // alias
	"NODEFAULT":     {Name: "NODEFAULT", Word: 2, Bit: gamedb.Flag3NoDefault},

	// Single-letter aliases (matching C TinyMUSH flag letters)
	"W": {Name: "WIZARD", Word: 0, Bit: gamedb.FlagWizard, Perm: FlagPermGod},
	"D": {Name: "DARK", Word: 0, Bit: gamedb.FlagDark},
	"V": {Name: "VISUAL", Word: 0, Bit: gamedb.FlagVisual},
	"I": {Name: "INHERIT", Word: 0, Bit: gamedb.FlagInherit},
	"N": {Name: "NOSPOOF", Word: 0, Bit: gamedb.FlagNoSpoof},
	"J": {Name: "JUMP_OK", Word: 0, Bit: gamedb.FlagJumpOK},
	"L": {Name: "LINK_OK", Word: 0, Bit: gamedb.FlagLinkOK},
	"M": {Name: "MONITOR", Word: 0, Bit: gamedb.FlagMonitor},
	"O": {Name: "OPAQUE", Word: 0, Bit: gamedb.FlagOpaque},
	"Q": {Name: "QUIET", Word: 0, Bit: gamedb.FlagQuiet},
	"S": {Name: "STICKY", Word: 0, Bit: gamedb.FlagSticky},
	"T": {Name: "TRACE", Word: 0, Bit: gamedb.FlagTrace},
	"Z": {Name: "ROYALTY", Word: 0, Bit: gamedb.FlagRoyalty, Perm: FlagPermWiz},
	// C single-letter aliases (word 0)
	"A": {Name: "ABODE", Word: 1, Bit: gamedb.Flag2Abode},
	"B": {Name: "BLIND", Word: 1, Bit: gamedb.Flag2Blind, Perm: FlagPermWiz},
	"C": {Name: "CHOWN_OK", Word: 0, Bit: gamedb.FlagChownOK},
	"E": {Name: "ENTER_OK", Word: 0, Bit: gamedb.FlagEnterOK},
	"F": {Name: "FREE", Word: 1, Bit: gamedb.Flag2Floating},
	"G": {Name: "GOING", Word: 0, Bit: gamedb.FlagGoing},
	"H": {Name: "HAVEN", Word: 0, Bit: gamedb.FlagHaven},
	"K": {Name: "KEY", Word: 1, Bit: gamedb.Flag2Key},
	"U": {Name: "UNFINDABLE", Word: 1, Bit: gamedb.Flag2Unfindable},
	"X": {Name: "ANSI", Word: 1, Bit: gamedb.Flag2Ansi},
	"Y": {Name: "PARENT_OK", Word: 1, Bit: gamedb.Flag2ParentOK},
	// C single-letter aliases (word 1)
	"a": {Name: "AUDIBLE", Word: 0, Bit: gamedb.FlagHearThru},
	"b": {Name: "BOUNCE", Word: 1, Bit: gamedb.Flag2Bounce},
	"c": {Name: "CONNECTED", Word: 1, Bit: gamedb.Flag2Connected, Perm: FlagPermGod},
	"d": {Name: "DESTROY_OK", Word: 0, Bit: gamedb.FlagDestroyOK},
	"e": {Name: "ENTER_OK", Word: 0, Bit: gamedb.FlagEnterOK},
	"f": {Name: "FIXED", Word: 1, Bit: gamedb.Flag2Fixed},
	"g": {Name: "UNINSPECTED", Word: 1, Bit: gamedb.Flag2Uninspected, Perm: FlagPermWizRoy},
	"h": {Name: "HALTED", Word: 0, Bit: gamedb.FlagHalt},
	"i": {Name: "IMMORTAL", Word: 0, Bit: gamedb.FlagImmortal, Perm: FlagPermWiz},
	"j": {Name: "GAGGED", Word: 1, Bit: gamedb.Flag2Gagged, Perm: FlagPermWiz},
	"k": {Name: "CONSTANT", Word: 1, Bit: gamedb.Flag2ConstAttrs, Perm: FlagPermGod},
	"l": {Name: "LIGHT", Word: 1, Bit: gamedb.Flag2Light},
	"m": {Name: "MYOPIC", Word: 0, Bit: gamedb.FlagMyopic},
	"n": {Name: "AUDITORIUM", Word: 1, Bit: gamedb.Flag2Auditorium},
	"o": {Name: "ZONE", Word: 1, Bit: gamedb.Flag2ZoneParent},
	"p": {Name: "PUPPET", Word: 0, Bit: gamedb.FlagPuppet},
	"q": {Name: "TERSE", Word: 0, Bit: gamedb.FlagTerse},
	"r": {Name: "ROBOT", Word: 0, Bit: gamedb.FlagRobot},
	"s": {Name: "SAFE", Word: 0, Bit: gamedb.FlagSafe},
	"t": {Name: "TRANSPARENT", Word: 0, Bit: gamedb.FlagSeeThru},
	"u": {Name: "SUSPECT", Word: 1, Bit: gamedb.Flag2Suspect, Perm: FlagPermWiz},
	"v": {Name: "VERBOSE", Word: 0, Bit: gamedb.FlagVerbose},
	"w": {Name: "STAFF", Word: 1, Bit: gamedb.Flag2Staff, Perm: FlagPermWiz},
	"x": {Name: "SLAVE", Word: 1, Bit: gamedb.Flag2Slave, Perm: FlagPermWiz},
	"y": {Name: "ORPHAN", Word: 2, Bit: gamedb.Flag3Orphan},
	"z": {Name: "CONTROL_OK", Word: 1, Bit: gamedb.Flag2ControlOK},
	"!": {Name: "STOP", Word: 1, Bit: gamedb.Flag2StopMatch},
	"?": {Name: "HEAD", Word: 1, Bit: gamedb.Flag2HeadFlag},
	"+": {Name: "WATCHER", Word: 1, Bit: gamedb.Flag2Watcher},
	"|": {Name: "VACATION", Word: 1, Bit: gamedb.Flag2Vacation},
	"-": {Name: "NOBLEED", Word: 1, Bit: gamedb.Flag2NoBLeed},
	"*": {Name: "HAS_DAILY", Word: 1, Bit: gamedb.Flag2HasDaily},
	"=": {Name: "HAS_STARTUP", Word: 0, Bit: gamedb.FlagHasStartup},
	"$": {Name: "COMMANDS", Word: 1, Bit: gamedb.Flag2HasCommands},
	"&": {Name: "HAS_FORWARDLIST", Word: 1, Bit: gamedb.Flag2HasFwd, Perm: FlagPermGod},
	"@": {Name: "HAS_LISTEN", Word: 1, Bit: gamedb.Flag2HasListen, Perm: FlagPermGod},
	"`": {Name: "PLAYER_MAILS", Word: 1, Bit: gamedb.Flag2PlayerMails, Perm: FlagPermGod},
	"~": {Name: "HTML", Word: 1, Bit: gamedb.Flag2HTML},
	"^": {Name: "PRESENCE", Word: 2, Bit: gamedb.Flag3Presence},
	">": {Name: "REDIR_OK", Word: 2, Bit: gamedb.Flag3RedirOK},
	"<": {Name: "HAS_REDIRECT", Word: 2, Bit: gamedb.Flag3HasRedirect, Perm: FlagPermGod},
	".": {Name: "HAS_DARKLOCK", Word: 2, Bit: gamedb.Flag3HasDarkLock, Perm: FlagPermGod},
	",": {Name: "HAS_PROPDIR", Word: 2, Bit: gamedb.Flag3HasPropdir, Perm: FlagPermGod},

	// Marker flags (word 2, user-defined)
	"MARKER0": {Name: "MARKER0", Word: 2, Bit: gamedb.Flag3Mark0},
	"MARKER1": {Name: "MARKER1", Word: 2, Bit: gamedb.Flag3Mark1},
	"MARKER2": {Name: "MARKER2", Word: 2, Bit: gamedb.Flag3Mark2},
	"MARKER3": {Name: "MARKER3", Word: 2, Bit: gamedb.Flag3Mark3},
	"MARKER4": {Name: "MARKER4", Word: 2, Bit: gamedb.Flag3Mark4},
	"MARKER5": {Name: "MARKER5", Word: 2, Bit: gamedb.Flag3Mark5},
	"MARKER6": {Name: "MARKER6", Word: 2, Bit: gamedb.Flag3Mark6},
	"MARKER7": {Name: "MARKER7", Word: 2, Bit: gamedb.Flag3Mark7},
	"MARKER8": {Name: "MARKER8", Word: 2, Bit: gamedb.Flag3Mark8},
	"MARKER9": {Name: "MARKER9", Word: 2, Bit: gamedb.Flag3Mark9},
	"0": {Name: "MARKER0", Word: 2, Bit: gamedb.Flag3Mark0},
	"1": {Name: "MARKER1", Word: 2, Bit: gamedb.Flag3Mark1},
	"2": {Name: "MARKER2", Word: 2, Bit: gamedb.Flag3Mark2},
	"3": {Name: "MARKER3", Word: 2, Bit: gamedb.Flag3Mark3},
	"4": {Name: "MARKER4", Word: 2, Bit: gamedb.Flag3Mark4},
	"5": {Name: "MARKER5", Word: 2, Bit: gamedb.Flag3Mark5},
	"6": {Name: "MARKER6", Word: 2, Bit: gamedb.Flag3Mark6},
	"7": {Name: "MARKER7", Word: 2, Bit: gamedb.Flag3Mark7},
	"8": {Name: "MARKER8", Word: 2, Bit: gamedb.Flag3Mark8},
	"9": {Name: "MARKER9", Word: 2, Bit: gamedb.Flag3Mark9},

	// HAS_PROPDIR (C internal)
	"HAS_PROPDIR": {Name: "HAS_PROPDIR", Word: 2, Bit: gamedb.Flag3HasPropdir, Perm: FlagPermGod},
}

// SetFlag result codes
const (
	SetFlagOK       = 0
	SetFlagUnknown  = 1
	SetFlagDenied   = 2
	SetFlagAmbiguous = 3
)

// SetFlag sets or clears a flag on an object, enforcing C TinyMUSH permissions.
// flagStr can be "FLAG" (set) or "!FLAG" (clear).
// player is the dbref of the player attempting the change (for permission checks).
func (g *Game) SetFlag(target gamedb.DBRef, flagStr string, player ...gamedb.DBRef) int {
	obj, ok := g.DB.Objects[target]
	if !ok {
		return SetFlagUnknown
	}

	flagStr = strings.TrimSpace(flagStr)
	clear := false
	if strings.HasPrefix(flagStr, "!") {
		clear = true
		flagStr = flagStr[1:]
	}

	flagName := strings.ToUpper(flagStr)
	def, ok := FlagTable[flagName]
	if !ok {
		// Prefix match (C TinyMUSH matches flags by prefix)
		var matches []*FlagDef
		for name, fd := range FlagTable {
			if strings.HasPrefix(name, flagName) {
				matches = append(matches, fd)
			}
		}
		if len(matches) == 1 {
			def = matches[0]
		} else if len(matches) > 1 {
			return SetFlagAmbiguous
		} else {
			return SetFlagUnknown
		}
	}

	// Permission check (matching C TinyMUSH fh_* handlers)
	if def.Perm > FlagPermAny && len(player) > 0 {
		p := player[0]
		switch def.Perm {
		case FlagPermGod:
			if !IsGod(g, p) {
				return SetFlagDenied
			}
		case FlagPermWiz:
			if !Wizard(g, p) {
				return SetFlagDenied
			}
		case FlagPermWizRoy:
			if !Wizard(g, p) && !Royalty(g, p) {
				return SetFlagDenied
			}
		}
	}

	// Special flag handlers (matching C TinyMUSH fh_* in flags.c)
	if len(player) > 0 {
		p := player[0]

		// fh_dark_bit: non-wizards can only set DARK on exits
		// or on themselves if they have the hide power (C Can_Hide)
		if def.Bit == gamedb.FlagDark && def.Word == 0 && !clear {
			if !Wizard(g, p) && obj.ObjType() != gamedb.TypeExit {
				if !(CanHide(g, p) && target == p) {
					return SetFlagDenied
				}
			}
		}

		// fh_going_bit: GOING can only be CLEARED by God, never set directly
		if def.Bit == gamedb.FlagGoing && def.Word == 0 {
			if !clear {
				return SetFlagDenied // nobody can set GOING directly
			}
			if !IsGod(g, p) {
				return SetFlagDenied
			}
		}

		// fh_inherit: setter must themselves Inherit (be wizard or have INHERIT)
		if def.Bit == gamedb.FlagInherit && def.Word == 0 && !clear {
			if !Inherits(g, p) {
				return SetFlagDenied
			}
		}

		// fh_watcher: WATCHER requires watch power or wizard (C Can_Watch)
		if def.Bit == gamedb.Flag2Watcher && def.Word == 1 && !clear {
			if !Wizard(g, p) && !HasPow(g, p, 0, gamedb.PowWatch) {
				return SetFlagDenied
			}
		}

		// fh_player_bit: ROBOT can only be set on players
		if def.Bit == gamedb.FlagRobot && def.Word == 0 && !clear {
			if obj.ObjType() != gamedb.TypePlayer {
				return SetFlagDenied
			}
		}
	}

	if clear {
		obj.Flags[def.Word] &^= def.Bit
	} else {
		obj.Flags[def.Word] |= def.Bit
	}
	g.PersistObject(obj)
	return SetFlagOK
}

// GetAttrTextByName returns the text of an attribute by name.
func (g *Game) GetAttrTextByName(obj gamedb.DBRef, attrName string) string {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return ""
	}

	// Try user-defined attrs
	if def, ok := g.DB.AttrByName[attrName]; ok {
		for _, attr := range o.Attrs {
			if attr.Number == def.Number {
				return eval.StripAttrPrefix(attr.Value)
			}
		}
	}

	// Try well-known (including aliases)
	if num, _, ok := gamedb.ResolveWellKnownAttr(attrName); ok {
		for _, attr := range o.Attrs {
			if attr.Number == num {
				return eval.StripAttrPrefix(attr.Value)
			}
		}
	}
	return ""
}

// ResolveAttrNum resolves an attribute name to its number.
func (g *Game) ResolveAttrNum(name string) int {
	name = strings.ToUpper(strings.TrimSpace(name))
	if def, ok := g.DB.AttrByName[name]; ok {
		return def.Number
	}
	if num, _, ok := gamedb.ResolveWellKnownAttr(name); ok {
		return num
	}
	return -1
}
