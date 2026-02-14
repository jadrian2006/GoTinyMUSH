package server

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// FlagDef maps a flag name to its word index and bit mask.
type FlagDef struct {
	Name string
	Word int // 0, 1, or 2 (flag word index)
	Bit  int
}

// FlagTable is the complete flag name -> definition table.
var FlagTable = map[string]*FlagDef{
	// Flag word 0
	"WIZARD":     {Name: "WIZARD", Word: 0, Bit: gamedb.FlagWizard},
	"DARK":       {Name: "DARK", Word: 0, Bit: gamedb.FlagDark},
	"HAVEN":      {Name: "HAVEN", Word: 0, Bit: gamedb.FlagHaven},
	"HALT":       {Name: "HALT", Word: 0, Bit: gamedb.FlagHalt},
	"SAFE":       {Name: "SAFE", Word: 0, Bit: gamedb.FlagSafe},
	"INHERIT":    {Name: "INHERIT", Word: 0, Bit: gamedb.FlagInherit},
	"NOSPOOF":    {Name: "NOSPOOF", Word: 0, Bit: gamedb.FlagNoSpoof},
	"VISUAL":     {Name: "VISUAL", Word: 0, Bit: gamedb.FlagVisual},
	"OPAQUE":     {Name: "OPAQUE", Word: 0, Bit: gamedb.FlagOpaque},
	"QUIET":      {Name: "QUIET", Word: 0, Bit: gamedb.FlagQuiet},
	"PUPPET":     {Name: "PUPPET", Word: 0, Bit: gamedb.FlagPuppet},
	"STICKY":     {Name: "STICKY", Word: 0, Bit: gamedb.FlagSticky},
	"MONITOR":    {Name: "MONITOR", Word: 0, Bit: gamedb.FlagMonitor},
	"ROBOT":      {Name: "ROBOT", Word: 0, Bit: gamedb.FlagRobot},
	"ROYALTY":    {Name: "ROYALTY", Word: 0, Bit: gamedb.FlagRoyalty},
	"ENTER_OK":   {Name: "ENTER_OK", Word: 0, Bit: gamedb.FlagEnterOK},
	"LINK_OK":    {Name: "LINK_OK", Word: 0, Bit: gamedb.FlagLinkOK},
	"JUMP_OK":    {Name: "JUMP_OK", Word: 0, Bit: gamedb.FlagJumpOK},
	"VERBOSE":    {Name: "VERBOSE", Word: 0, Bit: gamedb.FlagVerbose},
	"TERSE":      {Name: "TERSE", Word: 0, Bit: gamedb.FlagTerse},
	"TRACE":      {Name: "TRACE", Word: 0, Bit: gamedb.FlagTrace},
	"GOING":      {Name: "GOING", Word: 0, Bit: gamedb.FlagGoing},
	"MYOPIC":     {Name: "MYOPIC", Word: 0, Bit: gamedb.FlagMyopic},
	"CHOWN_OK":   {Name: "CHOWN_OK", Word: 0, Bit: gamedb.FlagChownOK},
	"DESTROY_OK": {Name: "DESTROY_OK", Word: 0, Bit: gamedb.FlagDestroyOK},
	"SEE_THROUGH": {Name: "SEE_THROUGH", Word: 0, Bit: gamedb.FlagSeeThru},
	"HEAR_THROUGH": {Name: "HEAR_THROUGH", Word: 0, Bit: gamedb.FlagHearThru},
	"IMMORTAL":   {Name: "IMMORTAL", Word: 0, Bit: gamedb.FlagImmortal},
	"HAS_STARTUP": {Name: "HAS_STARTUP", Word: 0, Bit: gamedb.FlagHasStartup},

	// Flag word 1
	"ABODE":      {Name: "ABODE", Word: 1, Bit: gamedb.Flag2Abode},
	"FLOATING":   {Name: "FLOATING", Word: 1, Bit: gamedb.Flag2Floating},
	"UNFINDABLE": {Name: "UNFINDABLE", Word: 1, Bit: gamedb.Flag2Unfindable},
	"PARENT_OK":  {Name: "PARENT_OK", Word: 1, Bit: gamedb.Flag2ParentOK},
	"LIGHT":      {Name: "LIGHT", Word: 1, Bit: gamedb.Flag2Light},
	"HAS_LISTEN": {Name: "HAS_LISTEN", Word: 1, Bit: gamedb.Flag2HasListen},
	"CONNECTED":  {Name: "CONNECTED", Word: 1, Bit: gamedb.Flag2Connected},
	"SLAVE":      {Name: "SLAVE", Word: 1, Bit: gamedb.Flag2Slave},
	"HTML":       {Name: "HTML", Word: 1, Bit: gamedb.Flag2HTML},
	"ANSI":       {Name: "ANSI", Word: 1, Bit: gamedb.Flag2Ansi},
	"BLIND":      {Name: "BLIND", Word: 1, Bit: gamedb.Flag2Blind},
	"CONTROL_OK": {Name: "CONTROL_OK", Word: 1, Bit: gamedb.Flag2ControlOK},
	"WATCHER":    {Name: "WATCHER", Word: 1, Bit: gamedb.Flag2Watcher},
	"HAS_COMMANDS": {Name: "HAS_COMMANDS", Word: 1, Bit: gamedb.Flag2HasCommands},
	"STOP":       {Name: "STOP", Word: 1, Bit: gamedb.Flag2StopMatch},
	"BOUNCE":     {Name: "BOUNCE", Word: 1, Bit: gamedb.Flag2Bounce},
	"ZONE_PARENT": {Name: "ZONE_PARENT", Word: 1, Bit: gamedb.Flag2ZoneParent},
	"NO_BLEED":   {Name: "NO_BLEED", Word: 1, Bit: gamedb.Flag2NoBLeed},
	"HAS_DAILY":  {Name: "HAS_DAILY", Word: 1, Bit: gamedb.Flag2HasDaily},
	"GAGGED":     {Name: "GAGGED", Word: 1, Bit: gamedb.Flag2Gagged},
	"STAFF":      {Name: "STAFF", Word: 1, Bit: gamedb.Flag2Staff},
	"FIXED":      {Name: "FIXED", Word: 1, Bit: gamedb.Flag2Fixed},
}

// SetFlag sets or clears a flag on an object.
// flagStr can be "FLAG" (set) or "!FLAG" (clear).
func (g *Game) SetFlag(target gamedb.DBRef, flagStr string) bool {
	obj, ok := g.DB.Objects[target]
	if !ok {
		return false
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
		return false
	}

	if clear {
		obj.Flags[def.Word] &^= def.Bit
	} else {
		obj.Flags[def.Word] |= def.Bit
	}
	g.PersistObject(obj)
	return true
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
