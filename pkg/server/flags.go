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
	FlagPermGod    = 3 // God only
	// C fh_restrict_player: non-wizards may not set/clear this on players
	FlagPermRestrictPlayer = 4
)

// FlagDef maps a flag name to its word index and bit mask.
type FlagDef struct {
	Name     string
	Word     int // 0, 1, or 2 (flag word index)
	Bit      int
	Perm     int // Required permission to SET (C fh_* handler class)
	ListPerm int // Required permission to SEE the flag (C listperm): 0=all, FlagPermWiz, FlagPermGod
}

// FlagTable is the complete flag name -> definition table.
// Perm values match C TinyMUSH fh_* handlers from flags.c.
// FlagTable is derived from gamedb.FlagDefs (the single canonical flag
// inventory) plus Go-side aliases. Keyed by canonical name, alias, and
// single-letter form.
var FlagTable = buildFlagTable()

func buildFlagTable() map[string]*FlagDef {
	permOf := func(p int) int {
		switch p {
		case gamedb.FlagPermWiz:
			return FlagPermWiz
		case gamedb.FlagPermWizRoy:
			return FlagPermWizRoy
		case gamedb.FlagPermGod:
			return FlagPermGod
		case gamedb.FlagPermRestrictPlayer:
			return FlagPermRestrictPlayer
		}
		return FlagPermAny
	}
	t := make(map[string]*FlagDef, len(gamedb.FlagDefs)*2)
	for _, fd := range gamedb.FlagDefs {
		def := &FlagDef{
			Name:     fd.Name,
			Word:     fd.Word,
			Bit:      fd.Bit,
			Perm:     permOf(fd.SetPerm),
			ListPerm: permOf(fd.ListPerm),
		}
		t[fd.Name] = def
		if fd.Letter != 0 {
			t[string(fd.Letter)] = def
		}
	}
	for alias, canon := range gamedb.FlagAliases {
		if def, ok := t[canon]; ok {
			t[alias] = def
		}
	}
	return t
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

	// Permission check (matching C TinyMUSH fh_* handlers).
	// God passes fh_wiz/fh_wizroy in C (fh_wiz tests Wizard(player)||God(player)).
	if def.Perm > FlagPermAny && len(player) > 0 {
		p := player[0]
		switch def.Perm {
		case FlagPermGod:
			if !IsGod(g, p) {
				return SetFlagDenied
			}
		case FlagPermWiz:
			if !Wizard(g, p) && !IsGod(g, p) {
				return SetFlagDenied
			}
		case FlagPermWizRoy:
			if !Wizard(g, p) && !Royalty(g, p) && !IsGod(g, p) {
				return SetFlagDenied
			}
		case FlagPermRestrictPlayer:
			// C fh_restrict_player: non-wizards may not change this on players
			if obj.ObjType() == gamedb.TypePlayer && !Wizard(g, p) && !IsGod(g, p) {
				return SetFlagDenied
			}
		}
	}

	// C fh_any: "Never let God drop his own wizbit." The handler notifies
	// AND returns failure, so the caller also reports Permission denied —
	// both lines appear, exactly as in C.
	if clear && def.Word == 0 && def.Bit == gamedb.FlagWizard && IsGod(g, target) {
		if len(player) > 0 {
			g.Notify(player[0], "You cannot make God mortal.")
		}
		return SetFlagDenied
	}

	// Special flag handlers (matching C TinyMUSH fh_* in flags.c)
	if len(player) > 0 {
		p := player[0]

		// fh_dark_bit: non-wizards can only set DARK on exits
		// or on themselves if they have the hide power (C Can_Hide)
		if def.Bit == gamedb.FlagDark && def.Word == 0 && !clear {
			if !Wizard(g, p) && !IsGod(g, p) && obj.ObjType() != gamedb.TypeExit {
				if !(CanHide(g, p) && target == p) {
					return SetFlagDenied
				}
			}
		}

		// fh_going_bit (C): clearing GOING on an object slated for
		// destruction spares it — allowed for anyone who got this far.
		// Anything else (setting, or clearing on non-going) is God-only.
		if def.Bit == gamedb.FlagGoing && def.Word == 0 {
			sparing := clear && obj.HasFlag(gamedb.FlagGoing) &&
				obj.ObjType() != gamedb.TypeGarbage
			if sparing {
				g.Notify(p, "Your object has been spared from destruction.")
			} else if !IsGod(g, p) {
				return SetFlagDenied
			}
		}

		// fh_inherit: setter must themselves Inherit (be wizard or have INHERIT)
		if def.Bit == gamedb.FlagInherit && def.Word == 0 && !clear {
			if !Inherits(g, p) {
				return SetFlagDenied
			}
		}

		// fh_power_bit (C): wizards can set WATCHER on anything; players
		// with the Watch power only on things they share an owner with.
		if def.Bit == gamedb.Flag2Watcher && def.Word == 1 {
			sameOwner := false
			if pObj, ok1 := g.DB.Objects[p]; ok1 {
				sameOwner = pObj.Owner == obj.Owner
			}
			if !Wizard(g, p) && !IsGod(g, p) &&
				!(sameOwner && HasPow(g, p, 0, gamedb.PowWatch)) {
				return SetFlagDenied
			}
		}

		// fh_player_bit (C): this flag can NOT be changed on players
		if def.Bit == gamedb.FlagRobot && def.Word == 0 {
			if obj.ObjType() == gamedb.TypePlayer {
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

	// Keep per-connection ANSI caches in sync with the flag (output-path
	// stripping reads the cache, not game state).
	if def.Word == 1 && def.Bit == gamedb.Flag2Ansi {
		for _, d := range g.Conns.GetByPlayer(target) {
			d.SetAnsi(!clear)
		}
	}
	return SetFlagOK
}

// GetAttrTextByName returns the text of an attribute by name.
func (g *Game) GetAttrTextByName(obj gamedb.DBRef, attrName string) string {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return ""
	}

	// Try user-defined attrs (AttrByName is keyed by uppercase)
	upper := strings.ToUpper(attrName)
	if def, ok := g.DB.AttrByName[upper]; ok {
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
