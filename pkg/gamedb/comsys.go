package gamedb

// Channel represents a comsys channel definition.
type Channel struct {
	Name           string
	Owner          DBRef
	Flags          int
	Charge         int
	ChargeCollected int
	NumSent        int
	Description    string
	Header         string // ANSI header prefix for messages
	JoinLock       string // Lock expression (unparsed)
	TransLock      string
	RecvLock       string
	Mogrifier      DBRef  // Object that transforms messages before broadcast
}

// ChanAlias represents a player's subscription/alias for a channel.
type ChanAlias struct {
	Player      DBRef
	Channel     string // Channel name
	Alias       string // Player's alias for this channel
	Title       string // Player's title on this channel
	IsListening bool   // Currently tuned in
}

// Channel flag constants — matches C TinyMUSH 3.3 comsys flag values.
const (
	ChanPublic   = 0x00000010 // Anyone can join
	ChanLoud     = 0x00000020 // Show connect/disconnect
	ChanPJoin    = 0x00000040 // Players can join (flag-based)
	ChanPTrans   = 0x00000080 // Players can transmit (flag-based)
	ChanPRecv    = 0x00000100 // Players can receive (flag-based)
	ChanOJoin    = 0x00000200 // Objects can join (flag-based)
	ChanOTrans   = 0x00000400 // Objects can transmit (flag-based)
	ChanORecv    = 0x00000800 // Objects can receive (flag-based)
	ChanSpoof    = 0x00001000 // Allow spoofing on channel
	ChanNoTitles = 0x00010000 // Suppress titles (Go extension, above C range)

	// Legacy alias — ChanObject mapped to ChanOJoin for backwards compatibility
	ChanObject = ChanOJoin
)
