package gamedb

// Flag permission classes, mirroring C TinyMUSH's fh_* handlers and listperm
// masks (flags.c). Shared by the server's set/display logic and the softcode
// flag functions so there is exactly ONE source of truth.
const (
	FlagPermAny    = 0 // anyone who controls the object
	FlagPermWiz    = 1 // wizards (or God)
	FlagPermWizRoy = 2 // wizards or royalty (or God)
	FlagPermGod    = 3 // God only
	// C fh_restrict_player: non-wizards may not change this on players
	FlagPermRestrictPlayer = 4
)

// FlagDefEntry describes one flag exactly as C's gen_flags[] does.
type FlagDefEntry struct {
	Name     string
	Letter   byte // display letter; 0 = no letter (Go-internal flags)
	Word     int  // flag word index: 0, 1, 2
	Bit      int
	SetPerm  int // permission class to set/clear (C fh_* handler)
	ListPerm int // permission class to SEE the flag (C listperm)
}

// FlagDefs is the canonical flag inventory in C gen_flags[] order. The order
// is significant: flags() and examine render letters in exactly this order.
var FlagDefs = []FlagDefEntry{
	{"ABODE", 'A', 1, Flag2Abode, FlagPermAny, FlagPermAny},
	{"BLIND", 'B', 1, Flag2Blind, FlagPermWiz, FlagPermAny},
	{"CHOWN_OK", 'C', 0, FlagChownOK, FlagPermAny, FlagPermAny},
	{"DARK", 'D', 0, FlagDark, FlagPermAny, FlagPermAny}, // fh_dark_bit special-cased in SetFlag
	{"FREE", 'F', 2, Flag3NoDefault, FlagPermWiz, FlagPermAny},
	{"GOING", 'G', 0, FlagGoing, FlagPermAny, FlagPermAny}, // fh_going_bit special-cased
	{"HAVEN", 'H', 0, FlagHaven, FlagPermAny, FlagPermAny},
	{"INHERIT", 'I', 0, FlagInherit, FlagPermAny, FlagPermAny}, // fh_inherit special-cased
	{"JUMP_OK", 'J', 0, FlagJumpOK, FlagPermAny, FlagPermAny},
	{"KEY", 'K', 1, Flag2Key, FlagPermAny, FlagPermAny},
	{"LINK_OK", 'L', 0, FlagLinkOK, FlagPermAny, FlagPermAny},
	{"MONITOR", 'M', 0, FlagMonitor, FlagPermAny, FlagPermAny},
	{"NOSPOOF", 'N', 0, FlagNoSpoof, FlagPermAny, FlagPermWiz},
	{"OPAQUE", 'O', 0, FlagOpaque, FlagPermAny, FlagPermAny},
	{"QUIET", 'Q', 0, FlagQuiet, FlagPermAny, FlagPermAny},
	{"STICKY", 'S', 0, FlagSticky, FlagPermAny, FlagPermAny},
	{"TRACE", 'T', 0, FlagTrace, FlagPermAny, FlagPermAny},
	{"UNFINDABLE", 'U', 1, Flag2Unfindable, FlagPermAny, FlagPermAny},
	{"VISUAL", 'V', 0, FlagVisual, FlagPermAny, FlagPermAny},
	{"WIZARD", 'W', 0, FlagWizard, FlagPermGod, FlagPermAny},
	{"ANSI", 'X', 1, Flag2Ansi, FlagPermAny, FlagPermAny},
	{"PARENT_OK", 'Y', 1, Flag2ParentOK, FlagPermAny, FlagPermAny},
	{"ROYALTY", 'Z', 0, FlagRoyalty, FlagPermWiz, FlagPermAny},
	{"AUDIBLE", 'a', 0, FlagHearThru, FlagPermAny, FlagPermAny},
	{"BOUNCE", 'b', 1, Flag2Bounce, FlagPermAny, FlagPermAny},
	{"CONNECTED", 'c', 1, Flag2Connected, FlagPermGod, FlagPermAny},
	{"DESTROY_OK", 'd', 0, FlagDestroyOK, FlagPermAny, FlagPermAny},
	{"ENTER_OK", 'e', 0, FlagEnterOK, FlagPermAny, FlagPermAny},
	{"FIXED", 'f', 1, Flag2Fixed, FlagPermRestrictPlayer, FlagPermAny},
	{"UNINSPECTED", 'g', 1, Flag2Uninspected, FlagPermWizRoy, FlagPermAny},
	{"HALTED", 'h', 0, FlagHalt, FlagPermAny, FlagPermAny},
	{"IMMORTAL", 'i', 0, FlagImmortal, FlagPermWiz, FlagPermAny},
	{"GAGGED", 'j', 1, Flag2Gagged, FlagPermWiz, FlagPermAny},
	{"CONSTANT", 'k', 1, Flag2ConstAttrs, FlagPermWiz, FlagPermAny},
	{"LIGHT", 'l', 1, Flag2Light, FlagPermAny, FlagPermAny},
	{"MYOPIC", 'm', 0, FlagMyopic, FlagPermAny, FlagPermAny},
	{"AUDITORIUM", 'n', 1, Flag2Auditorium, FlagPermAny, FlagPermAny},
	{"ZONE", 'o', 1, Flag2ZoneParent, FlagPermAny, FlagPermAny},
	{"PUPPET", 'p', 0, FlagPuppet, FlagPermAny, FlagPermAny}, // fh_hear_bit
	{"TERSE", 'q', 0, FlagTerse, FlagPermAny, FlagPermAny},
	{"ROBOT", 'r', 0, FlagRobot, FlagPermAny, FlagPermAny}, // fh_player_bit special-cased
	{"SAFE", 's', 0, FlagSafe, FlagPermAny, FlagPermAny},
	{"TRANSPARENT", 't', 0, FlagSeeThru, FlagPermAny, FlagPermAny},
	{"SUSPECT", 'u', 1, Flag2Suspect, FlagPermWiz, FlagPermWiz},
	{"VERBOSE", 'v', 0, FlagVerbose, FlagPermAny, FlagPermAny},
	{"STAFF", 'w', 1, Flag2Staff, FlagPermWiz, FlagPermAny},
	{"SLAVE", 'x', 1, Flag2Slave, FlagPermWiz, FlagPermWiz},
	{"ORPHAN", 'y', 2, Flag3Orphan, FlagPermAny, FlagPermAny},
	{"CONTROL_OK", 'z', 1, Flag2ControlOK, FlagPermAny, FlagPermAny},
	{"STOP", '!', 1, Flag2StopMatch, FlagPermWiz, FlagPermAny},
	{"COMMANDS", '$', 1, Flag2HasCommands, FlagPermAny, FlagPermAny},
	{"PRESENCE", '^', 2, Flag3Presence, FlagPermWiz, FlagPermAny},
	{"NOBLEED", '-', 1, Flag2NoBLeed, FlagPermAny, FlagPermAny},
	{"VACATION", '|', 1, Flag2Vacation, FlagPermRestrictPlayer, FlagPermAny},
	{"HEAD", '?', 1, Flag2HeadFlag, FlagPermWiz, FlagPermAny},
	{"WATCHER", '+', 1, Flag2Watcher, FlagPermAny, FlagPermAny}, // fh_power_bit special-cased
	{"HAS_DAILY", '*', 1, Flag2HasDaily, FlagPermGod, FlagPermGod},
	{"HAS_STARTUP", '=', 0, FlagHasStartup, FlagPermGod, FlagPermGod},
	{"HAS_FORWARDLIST", '&', 1, Flag2HasFwd, FlagPermGod, FlagPermGod},
	{"HAS_LISTEN", '@', 1, Flag2HasListen, FlagPermGod, FlagPermGod},
	{"HAS_PROPDIR", ',', 2, Flag3HasPropdir, FlagPermGod, FlagPermGod},
	{"PLAYER_MAILS", '`', 1, Flag2PlayerMails, FlagPermGod, FlagPermGod},
	{"HTML", '~', 1, Flag2HTML, FlagPermAny, FlagPermAny},
	{"REDIR_OK", '>', 2, Flag3RedirOK, FlagPermAny, FlagPermAny},
	{"HAS_REDIRECT", '<', 2, Flag3HasRedirect, FlagPermGod, FlagPermGod},
	{"HAS_DARKLOCK", '.', 2, Flag3HasDarkLock, FlagPermGod, FlagPermGod},
	{"SPEECHMOD", '"', 2, Flag3HasSpeechMod, FlagPermAny, FlagPermAny},
	{"MARKER0", '0', 2, Flag3Mark0, FlagPermGod, FlagPermAny},
	{"MARKER1", '1', 2, Flag3Mark1, FlagPermGod, FlagPermAny},
	{"MARKER2", '2', 2, Flag3Mark2, FlagPermGod, FlagPermAny},
	{"MARKER3", '3', 2, Flag3Mark3, FlagPermGod, FlagPermAny},
	{"MARKER4", '4', 2, Flag3Mark4, FlagPermGod, FlagPermAny},
	{"MARKER5", '5', 2, Flag3Mark5, FlagPermGod, FlagPermAny},
	{"MARKER6", '6', 2, Flag3Mark6, FlagPermGod, FlagPermAny},
	{"MARKER7", '7', 2, Flag3Mark7, FlagPermGod, FlagPermAny},
	{"MARKER8", '8', 2, Flag3Mark8, FlagPermGod, FlagPermAny},
	{"MARKER9", '9', 2, Flag3Mark9, FlagPermGod, FlagPermAny},
	// Go extensions — no display letter (C parity: every C letter is taken)
	{"INSTANCE", 0, 2, Flag3Instance, FlagPermAny, FlagPermAny},
}

// FlagAliases maps alternate names accepted by @set/hasflag to canonical
// names. C gets these from alias.conf; Go-original synonyms also live here.
var FlagAliases = map[string]string{
	"HALT":         "HALTED",
	"SEE_THROUGH":  "TRANSPARENT",
	"HEAR_THROUGH": "AUDIBLE",
	"FLOATING":     "FREE",
	"NODEFAULT":    "FREE",
	"ZONE_PARENT":  "ZONE",
	"HAS_COMMANDS": "COMMANDS",
	"NO_BLEED":     "NOBLEED",
	"OOB":          "AUDITORIUM",
	"HAS_SPEECHMOD": "SPEECHMOD",
}

// FlagByName returns the canonical entry for a (possibly aliased,
// case-insensitive, possibly single-letter) flag name.
func FlagByName(name string) (FlagDefEntry, bool) {
	if len(name) == 1 {
		for _, fd := range FlagDefs {
			if fd.Letter != 0 && fd.Letter == name[0] {
				return fd, true
			}
		}
	}
	if canon, ok := FlagAliases[name]; ok {
		name = canon
	}
	for _, fd := range FlagDefs {
		if fd.Name == name {
			return fd, true
		}
	}
	return FlagDefEntry{}, false
}
