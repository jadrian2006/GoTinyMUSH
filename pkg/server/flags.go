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
	"DARK":         {Name: "DARK", Word: 0, Bit: gamedb.FlagDark},           // fh_dark_bit (special, treated as fh_any for now)
	"HAVEN":        {Name: "HAVEN", Word: 0, Bit: gamedb.FlagHaven},
	"HALT":         {Name: "HALT", Word: 0, Bit: gamedb.FlagHalt},
	"SAFE":         {Name: "SAFE", Word: 0, Bit: gamedb.FlagSafe},
	"INHERIT":      {Name: "INHERIT", Word: 0, Bit: gamedb.FlagInherit},     // fh_inherit (special, treated as fh_any for now)
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
	"SEE_THROUGH":  {Name: "SEE_THROUGH", Word: 0, Bit: gamedb.FlagSeeThru},
	"HEAR_THROUGH": {Name: "HEAR_THROUGH", Word: 0, Bit: gamedb.FlagHearThru},
	"AUDIBLE":      {Name: "HEAR_THROUGH", Word: 0, Bit: gamedb.FlagHearThru}, // alias
	"IMMORTAL":     {Name: "IMMORTAL", Word: 0, Bit: gamedb.FlagImmortal, Perm: FlagPermWiz},
	"HAS_STARTUP":  {Name: "HAS_STARTUP", Word: 0, Bit: gamedb.FlagHasStartup},

	// Flag word 1
	"ABODE":        {Name: "ABODE", Word: 1, Bit: gamedb.Flag2Abode},
	"FLOATING":     {Name: "FLOATING", Word: 1, Bit: gamedb.Flag2Floating},
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
	"ZONE_PARENT":  {Name: "ZONE_PARENT", Word: 1, Bit: gamedb.Flag2ZoneParent},
	"NOBLEED":      {Name: "NOBLEED", Word: 1, Bit: gamedb.Flag2NoBLeed},
	"NO_BLEED":     {Name: "NOBLEED", Word: 1, Bit: gamedb.Flag2NoBLeed},
	"STAFF":        {Name: "STAFF", Word: 1, Bit: gamedb.Flag2Staff, Perm: FlagPermWiz},
	"HAS_DAILY":    {Name: "HAS_DAILY", Word: 1, Bit: gamedb.Flag2HasDaily},
	"GAGGED":       {Name: "GAGGED", Word: 1, Bit: gamedb.Flag2Gagged, Perm: FlagPermWiz},
	"HAS_COMMANDS": {Name: "HAS_COMMANDS", Word: 1, Bit: gamedb.Flag2HasCommands},
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

	// Flag word 2
	"INSTANCE":     {Name: "INSTANCE", Word: 2, Bit: gamedb.Flag3Instance},

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

	// Try well-known
	for num, name := range gamedb.WellKnownAttrs {
		if strings.EqualFold(name, attrName) {
			for _, attr := range o.Attrs {
				if attr.Number == num {
					return eval.StripAttrPrefix(attr.Value)
				}
			}
			break
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
	for num, n := range gamedb.WellKnownAttrs {
		if strings.EqualFold(n, name) {
			return num
		}
	}
	return -1
}
