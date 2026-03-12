package eval

import (
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// EvalFlags control expression evaluation behavior
const (
	EvEval        = 0x0001 // Evaluate functions
	EvFCheck      = 0x0002 // Check for function invocations
	EvFMand       = 0x0004 // Function evaluation is mandatory (inside [])
	EvStrip       = 0x0008 // Strip {} and leading/trailing spaces
	EvNoCompress  = 0x0010 // Don't compress spaces
	EvStripLS     = 0x0020 // Strip leading spaces
	EvStripTS     = 0x0040 // Strip trailing spaces
	EvStripESC    = 0x0080 // Strip backslash escapes
	EvStripAround = 0x0100 // Strip surrounding {}
	EvNoFCheck    = 0x0200 // Don't check for functions
	EvNoTrace        = 0x0400 // Don't trace
	EvNoLocation     = 0x0800 // Don't resolve %l
	EvFCheckPersist  = 0x1000 // Don't clear EvFCheck after first bare function call
)

const MaxGlobalRegs = 36
const MaxNFArgs = 30

// RegisterData holds the q-register state (%q0-%q9, %qa-%qz, named regs)
type RegisterData struct {
	QRegs  [MaxGlobalRegs]string // %q0-%q9, %qa-%qz
	QLens  [MaxGlobalRegs]int
	QAlloc int
	XRegs  map[string]string // Named registers %q<name>
	Dirty  int
}

// NewRegisterData creates a RegisterData with defaults.
func NewRegisterData() *RegisterData {
	return &RegisterData{
		QAlloc: MaxGlobalRegs,
		XRegs:  make(map[string]string),
	}
}

// Clone returns a deep copy of the RegisterData.
func (r *RegisterData) Clone() *RegisterData {
	if r == nil {
		return nil
	}
	nr := &RegisterData{
		QAlloc: r.QAlloc,
		Dirty:  r.Dirty,
		XRegs:  make(map[string]string),
	}
	copy(nr.QRegs[:], r.QRegs[:])
	copy(nr.QLens[:], r.QLens[:])
	for k, v := range r.XRegs {
		nr.XRegs[k] = v
	}
	return nr
}

// LoopState tracks iter()/parse()/switch() nesting
type LoopState struct {
	InLoop      int
	InSwitch    int
	LoopTokens  []string // ## values per nesting level
	LoopTokens2 []string // #+ values (iter2)
	LoopNumbers []int    // #@ values
	SwitchToken string   // #$ value
	BreakLevel  int      // > 0 means break out of this many loop levels
}

// GameState provides connection/game info to eval functions without importing server.
type GameState interface {
	// ConnectedPlayers returns all connected player dbrefs.
	ConnectedPlayers() []gamedb.DBRef
	// ConnectedPlayersVisible returns connected players visible to viewer.
	ConnectedPlayersVisible(viewer gamedb.DBRef) []gamedb.DBRef
	// ConnTime returns connection time in seconds for a player (-1 if not connected).
	ConnTime(player gamedb.DBRef) float64
	// IdleTime returns idle time in seconds for a player (-1 if not connected).
	IdleTime(player gamedb.DBRef) float64
	// DoingString returns a player's @doing string.
	DoingString(player gamedb.DBRef) string
	// IsConnected returns true if the player is connected.
	IsConnected(player gamedb.DBRef) bool
	// LookupPlayer finds a player by name (partial match).
	LookupPlayer(name string) gamedb.DBRef
	// CreateObject creates a new object, returns its dbref.
	CreateObject(name string, objType gamedb.ObjectType, owner gamedb.DBRef) gamedb.DBRef
	// Controls returns true if player controls target.
	Controls(player, target gamedb.DBRef) bool
	// Teleport moves victim to destination, updating contents chains.
	Teleport(victim, dest gamedb.DBRef)
	// SetAttrByName sets an attribute value on an object by attribute name.
	// Optional executor is the object performing the set — its resolved player-owner
	// is stored as the attribute owner (matching C TinyMUSH's atr_add behavior).
	SetAttrByName(obj gamedb.DBRef, attrName string, value string, executor ...gamedb.DBRef)
	// SetFlag sets or clears a flag on an object.
	// Optional player param enables permission checks (C TinyMUSH fh_* handlers).
	SetFlag(target gamedb.DBRef, flagStr string, player ...gamedb.DBRef) int
	// PlayerLocation returns the location of a player.
	PlayerLocation(player gamedb.DBRef) gamedb.DBRef
	// CreateExit creates a new exit linking source to dest.
	CreateExit(name string, source, dest, owner gamedb.DBRef) gamedb.DBRef
	// RemoveFromContents removes obj from loc's contents chain.
	RemoveFromContents(loc gamedb.DBRef, obj gamedb.DBRef)
	// CouldDoIt checks if player passes the lock on thing for the given lock attribute.
	// Includes wizard/power bypass — use for physical actions (movement, use).
	CouldDoIt(player, thing gamedb.DBRef, lockAttr int) bool
	// EvalObjLock evaluates the lock on thing against player without wizard bypass.
	// Only Pass_Locks power bypasses. Used by elock() function.
	EvalObjLock(player, thing gamedb.DBRef, lockAttr int) bool
	// GetAttrTextGS returns the text of an attribute on an object (with parent walk).
	GetAttrTextGS(obj gamedb.DBRef, attrNum int) string
	// CanReadAttrGS checks if player can read a specific attribute on obj.
	// rawValue is the raw attribute value string (with \x01owner:flags:text prefix).
	CanReadAttrGS(player, obj gamedb.DBRef, attrNum int, rawValue string) bool
	// SpellCheck returns misspelled words in text, considering player's custom dictionary.
	// If grammar is true, also returns grammar issues (requires remote API).
	SpellCheck(player gamedb.DBRef, text string, grammar bool) []string
	// SpellHighlight returns text with misspelled words highlighted.
	// Honors the player's ANSI flag for formatting. If grammar is true, also
	// highlights grammar issues in cyan (requires remote API).
	SpellHighlight(player gamedb.DBRef, text string, grammar bool) string
	// ExecuteSQL executes a SQL query, returning delimited results or an error string.
	// Checks that SQL is configured and player has use_sql power or is God.
	ExecuteSQL(player gamedb.DBRef, query, rowDelim, fieldDelim string) string
	// EscapeSQL escapes a string for safe SQL interpolation (doubles single quotes).
	EscapeSQL(input string) string
	// EvalLockStr parses and evaluates a lock expression string.
	// Returns true if actor passes the lock on thing.
	EvalLockStr(player, thing, actor gamedb.DBRef, lockStr string) bool
	// HelpLookup retrieves help text for a given topic from the named help file.
	// fileID is "help", "wizhelp", "news", "qhelp", or "plushelp".
	// Returns the text or empty string if not found.
	HelpLookup(player gamedb.DBRef, fileID, topic string) string
	// SessionInfo returns session statistics for a connected player.
	// Returns totalCmds, bytesSent, bytesRecv. Returns -1,-1,-1 if not connected.
	SessionInfo(player gamedb.DBRef) (int, int, int)
	// PersistStructDef saves or deletes a structure definition.
	// Pass nil def to delete.
	PersistStructDef(player gamedb.DBRef, name string, def *gamedb.StructDef)
	// PersistStructInstance saves or deletes a structure instance.
	// Pass nil inst to delete.
	PersistStructInstance(player gamedb.DBRef, name string, inst *gamedb.StructInstance)
	// PersistArray saves or deletes an array.
	// Pass nil arr to delete.
	PersistArray(player gamedb.DBRef, name string, arr *gamedb.ArrayData)
	// MailCount returns (total, unread, cleared) for a player's mailbox.
	// Returns (-1, -1, -1) if mail is disabled.
	MailCount(player gamedb.DBRef) (int, int, int)
	// MailFrom returns the sender dbref of message #num for player.
	// Returns gamedb.Nothing if not found or mail disabled.
	MailFrom(player gamedb.DBRef, num int) gamedb.DBRef
	// MailSubject returns the subject of message #num for player.
	// Returns "" if not found or mail disabled.
	MailSubject(player gamedb.DBRef, num int) string
	// ChannelInfo returns a field value for a channel by name.
	// Valid fields: owner, description, header, flags, numsent, subscribers, joinlock, translock, recvlock, charge.
	// Returns "" if channel not found, unknown field, or player lacks permission (must be channel owner or Wizard).
	ChannelInfo(player gamedb.DBRef, name, field string) string
	// ListAttrDefs returns a space-separated list of user-defined attribute names
	// matching the given pattern (wildcard). Empty pattern matches all.
	// objType filters by object type ("player", "thing", "room", "exit", or "" for all).
	// Respects permissions: non-wizards only see VISUAL attr definitions.
	ListAttrDefs(player gamedb.DBRef, pattern string, objType string) string
	// AttrDefFlags returns the flag string for a user-defined attribute definition.
	// Returns "#-1 NO SUCH ATTRIBUTE" if not found.
	// Non-wizards can only query VISUAL attributes.
	AttrDefFlags(player gamedb.DBRef, attrName string) string
	// HasAttrDef returns "1" if a user-defined attribute exists, "0" otherwise.
	HasAttrDef(attrName string) string
	// SetAttrDefFlags modifies flags on a user-defined attribute definition.
	// Returns "" on success, error string on failure. Wizard-only.
	SetAttrDefFlags(player gamedb.DBRef, attrName, flags string) string
	// IsWizard returns true if the player is an effective wizard.
	IsWizard(player gamedb.DBRef) bool
	// IsGod returns true if the player is the God player (#1).
	IsGod(player gamedb.DBRef) bool
	// GetObjLockStr returns the serialized default lock (obj.Lock BoolExp) for an object.
	// Returns "" if no header lock is set. Used as fallback when attr 42 is empty.
	GetObjLockStr(obj gamedb.DBRef) string

	// UnparseLockStr parses a serialized lock string and returns a human-readable
	// representation: players shown as *Name, things as #dbref (matching C's F_FUNCTION format).
	UnparseLockStr(player gamedb.DBRef, lockStr string) string

	// SendOOB sends a GMCP message to a connected player's client(s).
	// Returns true if at least one GMCP-capable descriptor received it.
	SendOOB(player gamedb.DBRef, pkg string, data string) bool

	// HasGMCP returns true if player has at least one GMCP-capable connection.
	HasGMCP(player gamedb.DBRef) bool

	// GMCPPackages returns the GMCP package subscriptions for a player.
	GMCPPackages(player gamedb.DBRef) []string

	// HasMSDP returns true if player has at least one MSDP-capable connection.
	HasMSDP(player gamedb.DBRef) bool

	// HelpSearch searches help file entry contents for a pattern.
	// Returns matching topic names (case-insensitive substring match).
	HelpSearch(player gamedb.DBRef, fileID string, pattern string) []string

	// HasAPIKey returns true if the object has an API key set.
	HasAPIKey(obj gamedb.DBRef) bool

	// ConnLog returns connection log timestamps for a player.
	// Returns space-separated Unix timestamps (newest first).
	ConnLog(player gamedb.DBRef, count int) string

	// AddrLog returns connection log IP addresses for a player.
	// Returns space-separated IP addresses (newest first).
	AddrLog(player gamedb.DBRef, count int) string

	// NextDBRef returns the next available dbref that would be assigned.
	NextDBRef() gamedb.DBRef

	// Event Bus functions
	EventBusPublish(caller gamedb.DBRef, queueName, data string) string
	EventBusSubscribe(obj gamedb.DBRef, attr, queueName string, bind gamedb.DBRef) string
	EventBusUnsubscribe(obj gamedb.DBRef, attr, queueName string) string
	EventBusQueueList() string
	EventBusQueueInfo(queueName, mode string) string

	// Examinable returns true if player can examine target.
	// True if: VISUAL flag, SeeAll, same owner, or zone control.
	Examinable(player, target gamedb.DBRef) bool

	// Comsys softcode function support

	// ChannelList returns visible channel names for player (public, owned, or comm_all).
	ChannelList(player gamedb.DBRef) []string
	// ChannelWho returns connected, listening players on a channel.
	// Respects Hidden flag (only See_Hidden/wizards see hidden players).
	ChannelWho(player gamedb.DBRef, channelName string) []gamedb.DBRef
	// ChannelWhoAll returns all subscribers on a channel (connected or not).
	ChannelWhoAll(player gamedb.DBRef, channelName string) []gamedb.DBRef
	// ChannelOwner returns the owner dbref of a channel, or Nothing.
	ChannelOwner(channelName string) gamedb.DBRef
	// ChannelDesc returns the description of a channel.
	ChannelDesc(channelName string) string
	// ChannelHeader returns the header of a channel.
	ChannelHeader(channelName string) string
	// PlayerComAliases returns space-separated alias names for a player.
	// Requires Controls or Comm_All.
	PlayerComAliases(player, target gamedb.DBRef) string
	// PlayerComInfo returns the channel name for a player's alias.
	// Requires Controls or Comm_All.
	PlayerComInfo(player, target gamedb.DBRef, alias string) string
	// PlayerComTitle returns the title for a player's alias.
	// Requires Controls or Comm_All.
	PlayerComTitle(player, target gamedb.DBRef, alias string) string
	// ChannelEmit sends a message to a channel. Returns "" on success, error on failure.
	ChannelEmit(player gamedb.DBRef, channelName, message string) string

	// IsInstance returns true if obj has the Flag3Instance flag.
	IsInstance(obj gamedb.DBRef) bool

	// InstanceRooms returns the interior rooms of an instance (rooms whose Location = obj).
	InstanceRooms(obj gamedb.DBRef) []gamedb.DBRef

	// InstanceVehicle returns the instance THING this room belongs to, or Nothing.
	InstanceVehicle(room gamedb.DBRef) gamedb.DBRef

	// Side-effect function support

	// ForceCommand queues command to execute as victim, with forcer as cause.
	ForceCommand(forcer, victim gamedb.DBRef, command string)
	// LinkObject sets the link/home/dropto on target to dest.
	// Returns "" on success, error message on failure.
	LinkObject(player, target, dest gamedb.DBRef) string
	// WipeAttrs removes user-defined attributes matching pattern from target.
	// Pattern uses wildcard matching. Returns "" on success, error on failure.
	WipeAttrs(player, target gamedb.DBRef, pattern string) string
	// TriggerAttr triggers obj/attr with optional args (comma-separated after obj/attr).
	// Returns true on success.
	TriggerAttr(player, cause gamedb.DBRef, obj gamedb.DBRef, attr string, args []string) bool
	// WaitCommand queues a delayed command. secs is the delay in seconds.
	WaitCommand(player, cause gamedb.DBRef, secs int, command string)
	// SetParent sets the parent of target to newParent. Checks Controls and circularity.
	// Returns "" on success, error string on failure.
	SetParent(player, target, newParent gamedb.DBRef) string
	// RenameObject sets the name of target. Checks Controls and player name conflicts.
	// Returns "" on success, error string on failure.
	RenameObject(player, target gamedb.DBRef, newName string) string
	// CloneObject clones target, returns the new dbref or Nothing on failure.
	CloneObject(player, target gamedb.DBRef) gamedb.DBRef
}

// EvalContext is the execution context for MUSH expression evaluation.
type EvalContext struct {
	// Database reference
	DB *gamedb.Database

	// Game state for connection queries
	GameState GameState

	// Object context
	Player gamedb.DBRef // Executor (the object running code, %!)
	Caller gamedb.DBRef // Caller (@)
	Cause  gamedb.DBRef // Enactor/trigger cause (%#)

	// Register state
	RData *RegisterData

	// Loop/switch state
	Loop LoopState

	// Function call tracking
	FuncNestLev int
	FuncInvkCtr int
	FuncNestLim int // default 50
	FuncInvkLim int // default 2500

	// Iteration limit — caps iter/parse/map/filter/foreach/etc. element count (default 10000)
	IterLim     int
	IterLimited bool // set when iteration limit was hit

	// Wall-clock deadline for this evaluation context.
	// Zero means no deadline (backward compat for unit tests).
	Deadline time.Time
	TimedOut bool // set when deadline exceeded; all further eval returns ""

	// Output buffer cap (bytes). 0 = unlimited (backward compat for unit tests).
	OutputLimit   int
	OutputLimited bool // set when output cap exceeded

	// Command invocation limit — caps semicolon-separated commands per queue entry.
	CmdInvkLim     int
	CmdInvkCtr     int
	CmdInvkLimited bool

	// Current command text
	CurrCmd string

	// Piped output
	PipeOut string

	// Output buffer for side-effect notifications
	Notifications []Notification

	// Space compression (default true in most configs)
	SpaceCompress bool

	// ANSI colors enabled
	AnsiColors bool

	// User-defined functions (name -> UFun)
	UFunctions map[string]*UFunction

	// Built-in function registry
	Functions map[string]*Function

	// Game identity (set from game config)
	MudName    string
	VersionStr string
	StartTime  int64 // Server start time as Unix timestamp

	// CArgs holds the current command arguments (%0-%9) from the calling context.
	// This allows FnNoEval function handlers (iter, switch, etc.) to propagate
	// parent cargs when they call Exec() internally.
	CArgs []string
}

// NotifyType distinguishes different notification semantics.
type NotifyType int

const (
	NotifyPemit NotifyType = iota // Default: send to target
	NotifyRemit                   // Send to all in room
	NotifyOEmit                   // Send to all in target's room except target
)

// Notification represents a pending pemit/remit/etc
type Notification struct {
	Target  gamedb.DBRef
	Message string
	Type    NotifyType
}

// UFunction is a user-defined (@function) function
type UFunction struct {
	Name  string
	Obj   gamedb.DBRef
	Attr  int
	Flags int
	Perms int
}

// UFunction flags
const (
	UfPriv = 0x0001 // /privileged — runs as object owner
	UfPres = 0x0002 // /preserve — preserves caller registers
)

// FnHandler is the signature for built-in function handlers.
type FnHandler func(ctx *EvalContext, args []string, buff *strings.Builder, caller, cause gamedb.DBRef)

// Function is a registered built-in function.
type Function struct {
	Name    string
	Handler FnHandler
	NArgs   int // Expected args (-N means join rest, 0 with VarArgs means any)
	MinArgs int // Minimum args for VarArgs functions (0 = no minimum)
	MaxArgs int // Maximum args for VarArgs functions (0 = no maximum)
	Flags   int
	Perms   int // Access level: FnAccessPublic, FnAccessWizard, FnAccessGod, FnAccessDisabled
}

// Function flags
const (
	FnVarArgs = 0x0001 // Variable number of args
	FnNoEval  = 0x0002 // Don't evaluate args before calling
	FnPriv    = 0x0004 // Privileged function
	FnNoregs  = 0x0008 // Don't pass registers
	FnPres    = 0x0010 // Preserve registers across call
)

// Function access levels (matching C TinyMUSH CA_PUBLIC/CA_WIZARD/CA_GOD/CA_DISABLED)
const (
	FnAccessPublic   = 0 // Anyone can use (default)
	FnAccessWizard   = 1 // Wizard-only
	FnAccessGod      = 2 // God-only
	FnAccessDisabled = 3 // Completely disabled
)

// CheckFuncAccess checks whether the current player can invoke a function.
// Returns true if access is allowed.
func (ctx *EvalContext) CheckFuncAccess(fn *Function) bool {
	switch fn.Perms {
	case FnAccessPublic:
		return true
	case FnAccessWizard:
		if ctx.GameState != nil {
			return ctx.GameState.IsWizard(ctx.Player)
		}
		return false
	case FnAccessGod:
		if ctx.GameState != nil {
			return ctx.GameState.IsGod(ctx.Player)
		}
		return false
	case FnAccessDisabled:
		return false
	}
	return true
}

// NewEvalContext creates an EvalContext with reasonable defaults.
func NewEvalContext(db *gamedb.Database) *EvalContext {
	ctx := &EvalContext{
		DB:            db,
		Player:        gamedb.Nothing,
		Caller:        gamedb.Nothing,
		Cause:         gamedb.Nothing,
		RData:         NewRegisterData(),
		FuncNestLim:   50,
		FuncInvkLim:   2500,
		IterLim:       10000,
		OutputLimit:   1048576, // 1MB default
		SpaceCompress: false,
		AnsiColors:    true,
		UFunctions:    make(map[string]*UFunction),
		Functions:     make(map[string]*Function),
	}
	return ctx
}

// CheckDeadline tests if the wall-clock deadline has been exceeded.
// Should be called periodically (e.g. every 100 function invocations).
// Returns true if evaluation should stop.
func (ctx *EvalContext) CheckDeadline() bool {
	if ctx.TimedOut {
		return true
	}
	if !ctx.Deadline.IsZero() && time.Now().After(ctx.Deadline) {
		ctx.TimedOut = true
		return true
	}
	return false
}

// ClampIterList returns n clamped to IterLim, setting IterLimited if truncated.
func (ctx *EvalContext) ClampIterList(n int) int {
	if ctx.IterLim > 0 && n > ctx.IterLim {
		ctx.IterLimited = true
		return ctx.IterLim
	}
	return n
}

// CheckOutputLimit returns true if the output buffer has exceeded the cap.
func (ctx *EvalContext) CheckOutputLimit(bufLen int) bool {
	if ctx.OutputLimited {
		return true
	}
	if ctx.OutputLimit > 0 && bufLen > ctx.OutputLimit {
		ctx.OutputLimited = true
		return true
	}
	return false
}

// GetAttrValue fetches an attribute value for an object from the DB.
// Returns the raw value string including owner:flags:data prefix.
func (ctx *EvalContext) GetAttrValue(obj gamedb.DBRef, attrNum int) string {
	dbObj, ok := ctx.DB.Objects[obj]
	if !ok {
		return ""
	}
	for _, attr := range dbObj.Attrs {
		if attr.Number == attrNum {
			return attr.Value
		}
	}
	return ""
}

// GetAttrText fetches the text portion of an attribute (after owner:flags: prefix).
func (ctx *EvalContext) GetAttrText(obj gamedb.DBRef, attrNum int) string {
	raw := ctx.GetAttrValue(obj, attrNum)
	if raw == "" {
		return ""
	}
	return StripAttrPrefix(raw)
}

// GetAttrTextParent fetches the text portion of an attribute, walking the parent
// chain like TinyMUSH's atr_pget. This is the correct method for most attribute
// lookups (%v substitutions, @function invocation, pronoun resolution, etc.).
func (ctx *EvalContext) GetAttrTextParent(obj gamedb.DBRef, attrNum int) string {
	current := obj
	for depth := 0; depth <= 10; depth++ {
		dbObj, ok := ctx.DB.Objects[current]
		if !ok {
			return ""
		}
		for _, attr := range dbObj.Attrs {
			if attr.Number == attrNum {
				return StripAttrPrefix(attr.Value)
			}
		}
		if dbObj.Parent == gamedb.Nothing || dbObj.Parent == current {
			return ""
		}
		current = dbObj.Parent
	}
	return ""
}

// StripAttrPrefix removes the "\x01owner:flags:" prefix from a raw attribute value.
// TinyMUSH stores attributes either as raw text (no prefix) or with a \x01 marker
// followed by "owner:flags:text". If no \x01 marker is present, returns the raw value.
func StripAttrPrefix(raw string) string {
	if len(raw) == 0 {
		return raw
	}
	// Check for ATR_INFO_CHAR (\x01) marker
	if raw[0] != '\x01' {
		return raw
	}
	// Format is "\x01owner:flags:text" — find second colon after the marker
	colonCount := 0
	for i := 1; i < len(raw); i++ {
		if raw[i] == ':' {
			colonCount++
			if colonCount == 2 {
				return raw[i+1:]
			}
		}
	}
	// Malformed prefix — return everything after the marker
	return raw[1:]
}

// parseInstanceFlags extracts per-instance attribute flags from the raw
// "\x01owner:flags:text" format. Returns 0 if no prefix is present.
func parseInstanceFlags(raw string) int {
	if len(raw) == 0 || raw[0] != '\x01' {
		return 0
	}
	colonCount := 0
	start := 0
	for i := 1; i < len(raw); i++ {
		if raw[i] == ':' {
			colonCount++
			if colonCount == 1 {
				start = i + 1
			}
			if colonCount == 2 {
				n := 0
				for _, ch := range raw[start:i] {
					if ch >= '0' && ch <= '9' {
						n = n*10 + int(ch-'0')
					}
				}
				return n
			}
		}
	}
	return 0
}

// RegisterFunction adds a built-in function to the registry.
func (ctx *EvalContext) RegisterFunction(name string, handler FnHandler, nargs int, flags int) {
	ctx.Functions[name] = &Function{
		Name:    name,
		Handler: handler,
		NArgs:   nargs,
		Flags:   flags,
	}
}

// RegisterFunctionV adds a VarArgs function with min/max arg validation.
// maxArgs=0 means no upper limit.
func (ctx *EvalContext) RegisterFunctionV(name string, handler FnHandler, minArgs, maxArgs int, flags int) {
	ctx.Functions[name] = &Function{
		Name:    name,
		Handler: handler,
		NArgs:   0,
		MinArgs: minArgs,
		MaxArgs: maxArgs,
		Flags:   flags | FnVarArgs,
	}
}

// AliasFunction creates an alias for an existing function.
// Both alias and target should be uppercase.
func (ctx *EvalContext) AliasFunction(alias, target string) {
	if fn, ok := ctx.Functions[target]; ok {
		ctx.Functions[alias] = fn
	}
}
