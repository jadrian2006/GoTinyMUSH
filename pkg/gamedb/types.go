package gamedb

import "time"

// DBRef is the fundamental object reference type in MUSH.
type DBRef int

const (
	Nothing  DBRef = -1
	Ambiguous DBRef = -2
	Home     DBRef = -3
	NoPerm   DBRef = -4
)

// ObjectType represents the type of a MUSH object.
type ObjectType int

const (
	TypeRoom    ObjectType = 0
	TypeThing   ObjectType = 1
	TypeExit    ObjectType = 2
	TypePlayer  ObjectType = 3
	TypeZone    ObjectType = 4
	TypeGarbage ObjectType = 5
)

func (t ObjectType) String() string {
	switch t {
	case TypeRoom:
		return "ROOM"
	case TypeThing:
		return "THING"
	case TypeExit:
		return "EXIT"
	case TypePlayer:
		return "PLAYER"
	case TypeZone:
		return "ZONE"
	case TypeGarbage:
		return "GARBAGE"
	default:
		return "UNKNOWN"
	}
}

const TypeMask = 0x7

// Flag constants - first word
const (
	FlagSeeThru   = 0x00000008
	FlagWizard    = 0x00000010
	FlagLinkOK    = 0x00000020
	FlagDark      = 0x00000040
	FlagJumpOK    = 0x00000080
	FlagSticky    = 0x00000100
	FlagDestroyOK = 0x00000200
	FlagHaven     = 0x00000400
	FlagQuiet     = 0x00000800
	FlagHalt      = 0x00001000
	FlagTrace     = 0x00002000
	FlagGoing     = 0x00004000
	FlagMonitor   = 0x00008000
	FlagMyopic    = 0x00010000
	FlagPuppet    = 0x00020000
	FlagChownOK   = 0x00040000
	FlagEnterOK   = 0x00080000
	FlagVisual    = 0x00100000
	FlagImmortal  = 0x00200000
	FlagHasStartup = 0x00400000
	FlagOpaque    = 0x00800000
	FlagVerbose   = 0x01000000
	FlagInherit   = 0x02000000
	FlagNoSpoof   = 0x04000000
	FlagRobot     = 0x08000000
	FlagSafe      = 0x10000000
	FlagRoyalty   = 0x20000000
	FlagHearThru  = 0x40000000
	FlagTerse     = 0x80000000
)

// Flag constants - second word
const (
	Flag2Key        = 0x00000001
	Flag2Abode      = 0x00000002
	Flag2Floating   = 0x00000004
	Flag2Unfindable = 0x00000008
	Flag2ParentOK   = 0x00000010
	Flag2Light      = 0x00000020
	Flag2HasListen  = 0x00000040
	Flag2HasFwd     = 0x00000080
	Flag2Connected  = 0x00000200
	Flag2Slave      = 0x00000800
	Flag2HTML       = 0x00001000
	Flag2Ansi       = 0x00002000
	Flag2HadStartup = 0x00004000
	Flag2Blind      = 0x00008000
	Flag2ControlOK  = 0x00010000
	Flag2Watcher    = 0x00080000
	Flag2HasCommands = 0x00200000
	Flag2StopMatch  = 0x00400000
	Flag2Bounce     = 0x00800000
	Flag2ZoneParent = 0x01000000
	Flag2NoBLeed    = 0x02000000
	Flag2HasDaily   = 0x04000000
	Flag2Gagged     = 0x08000000
	Flag2Staff      = 0x10000000
	Flag2HasDarkLock = 0x20000000
	Flag2Fixed      = 0x40000000
)

// Power constants - first word (Powers[0])
const (
	PowChgQuotas   = 0x00000001
	PowChownAny    = 0x00000002
	PowAnnounce    = 0x00000004
	PowBoot        = 0x00000008
	PowHalt        = 0x00000010
	PowControlAll  = 0x00000020
	PowWizardWho   = 0x00000040
	PowExamAll     = 0x00000080 // See_All
	PowFindUnfind  = 0x00000100
	PowFreeMoney   = 0x00000200
	PowFreeQuota   = 0x00000400
	PowHide        = 0x00000800
	PowIdle        = 0x00001000
	PowSearch      = 0x00002000
	PowLongfingers = 0x00004000
	PowProg        = 0x00008000
	PowMdarkAttr   = 0x00010000
	PowWizAttr     = 0x00020000
	PowCommAll     = 0x00080000
	PowSeeQueue    = 0x00100000
	PowSeeHidden   = 0x00200000
	PowWatch       = 0x00400000
	PowPoll        = 0x00800000
	PowNoDestroy   = 0x01000000
	PowGuest       = 0x02000000
	PowPassLocks   = 0x04000000
	PowStatAny     = 0x08000000
	PowSteal       = 0x10000000
	PowTelAnywhr   = 0x20000000
	PowTelUnrst    = 0x40000000
	PowUnkillable  = 0x80000000
)

// Power constants - second word (Powers[1])
const (
	Pow2Builder    = 0x00000001
	Pow2LinkVar    = 0x00000002
	Pow2LinkToAny  = 0x00000004
	Pow2OpenAnyLoc = 0x00000008
	Pow2UseSQL     = 0x00000010
	Pow2LinkHome   = 0x00000020
	Pow2Cloak      = 0x00000040
)

// HasPower checks if a power bit is set in the given power word (0 or 1).
func (o *Object) HasPower(word, bit int) bool {
	if word < 0 || word > 1 {
		return false
	}
	return o.Powers[word]&bit != 0
}

// SetPower sets or clears a power bit in the given power word (0 or 1).
func (o *Object) SetPower(word, bit int, set bool) {
	if word < 0 || word > 1 {
		return
	}
	if set {
		o.Powers[word] |= bit
	} else {
		o.Powers[word] &^= bit
	}
}

// Attribute flag constants (from TinyMUSH attrs.h)
const (
	AFODark    = 0x00000001 // Only owner can see
	AFDark     = 0x00000002 // Only God (#1) can see
	AFWizard   = 0x00000004 // Only wizards can change
	AFMDark    = 0x00000008 // Only wizards can see
	AFInternal = 0x00000010 // Don't show even to God
	AFNoCMD    = 0x00000020 // Don't create @ command
	AFLock     = 0x00000040 // Attribute is locked (per-instance)
	AFDeleted  = 0x00000080 // Attribute should be ignored
	AFNoProg   = 0x00000100 // Don't process $-commands
	AFGod      = 0x00000200 // Only God can change
	AFIsLock   = 0x00000400 // Attribute is a lock
	AFVisual   = 0x00000800 // Anyone can see
	AFPrivate  = 0x00001000 // Not inherited by children
	AFHTML     = 0x00002000 // Don't HTML escape
	AFNoParse  = 0x00004000 // Don't evaluate in $-cmd check
	AFRegexp   = 0x00008000 // Regex match for $-commands
	AFNoClone  = 0x00010000 // Don't copy when cloning
	AFConst    = 0x00020000 // No one can change (server-only)
	AFCase      = 0x00040000 // Regexp case-sensitive
	AFStructure = 0x00080000 // Attribute contains a structure
	AFDirty     = 0x00100000 // Attribute number has been modified
	AFDefault   = 0x00200000 // did_it() checks attr_defaults obj
	AFNoName    = 0x00400000 // If used as oattr, no name prepend
	AFRMatch    = 0x00800000 // Set result of match into regs
	AFNow       = 0x01000000 // Execute match immediately
	AFTrace     = 0x02000000 // Trace ufunction
	AFPropagate = 0x04000000 // Auto-copy from parent to child on @parent/@clone (GoTinyMUSH extension)
)

// BoolExpType represents the type of a boolean lock expression node.
type BoolExpType int

const (
	BoolAnd   BoolExpType = 0
	BoolOr    BoolExpType = 1
	BoolNot   BoolExpType = 2
	BoolConst BoolExpType = 3
	BoolAttr  BoolExpType = 4
	BoolIndir BoolExpType = 5
	BoolCarry BoolExpType = 6
	BoolIs    BoolExpType = 7
	BoolOwner BoolExpType = 8
	BoolEval  BoolExpType = 9
)

// BoolExp represents a parsed boolean lock expression.
type BoolExp struct {
	Type    BoolExpType
	Sub1    *BoolExp
	Sub2    *BoolExp
	Thing   int    // dbref or attribute number
	StrVal  string // for ATR/EVAL lock string values
}

// Attribute represents a single attribute on an object.
type Attribute struct {
	Number int
	Value  string
}

// AttrDef represents a user-defined attribute name definition.
type AttrDef struct {
	Number int
	Name   string
	Flags  int
}

// Object represents a MUSH database object.
type Object struct {
	DBRef    DBRef
	Name     string
	Location DBRef
	Zone     DBRef
	Contents DBRef
	Exits    DBRef
	Link     DBRef
	Next     DBRef
	Owner    DBRef
	Parent   DBRef
	Pennies  int
	Flags    [3]int
	Powers   [2]int
	LastAccess time.Time
	LastMod    time.Time
	Attrs    []Attribute
	Lock     *BoolExp // parsed default lock (if in header)
}

// ObjType returns the object type from the flags.
func (o *Object) ObjType() ObjectType {
	return ObjectType(o.Flags[0] & TypeMask)
}

// HasFlag checks if a flag bit is set in the first flag word.
func (o *Object) HasFlag(flag int) bool {
	return o.Flags[0]&flag != 0
}

// HasFlag2 checks if a flag bit is set in the second flag word.
func (o *Object) HasFlag2(flag int) bool {
	return o.Flags[1]&flag != 0
}

// IsGoing returns true if the object is marked for destruction.
func (o *Object) IsGoing() bool {
	return o.HasFlag(FlagGoing)
}

// Database holds the complete in-memory game state.
type Database struct {
	Version       int
	Format        int
	Flags         int
	Size          int
	NextAttr      int
	RecordPlayers int
	Objects       map[DBRef]*Object
	AttrNames     map[int]*AttrDef  // attr number -> definition
	AttrByName    map[string]*AttrDef // attr name -> definition
}

// NewDatabase creates an empty Database.
func NewDatabase() *Database {
	return &Database{
		Objects:    make(map[DBRef]*Object),
		AttrNames:  make(map[int]*AttrDef),
		AttrByName: make(map[string]*AttrDef),
	}
}

// AddAttrDef registers a user-defined attribute.
func (db *Database) AddAttrDef(num int, name string, flags int) {
	def := &AttrDef{Number: num, Name: name, Flags: flags}
	db.AttrNames[num] = def
	db.AttrByName[name] = def
}

// StructDef is a named data structure template (persisted to bbolt).
type StructDef struct {
	Name       string
	Components []string
	Types      []byte
	Defaults   []string
	Delim      string
}

// StructInstance is a live instance of a structure (persisted to bbolt).
type StructInstance struct {
	DefName string
	Values  []string
}

// GetAttrName returns the name for an attribute number, or "" if unknown.
func (db *Database) GetAttrName(num int) string {
	if def, ok := db.AttrNames[num]; ok {
		return def.Name
	}
	// Check well-known attribute names
	if name, ok := WellKnownAttrs[num]; ok {
		return name
	}
	return ""
}
