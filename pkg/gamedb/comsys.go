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
}

// ChanAlias represents a player's subscription/alias for a channel.
type ChanAlias struct {
	Player      DBRef
	Channel     string // Channel name
	Alias       string // Player's alias for this channel
	Title       string // Player's title on this channel
	IsListening bool   // Currently tuned in
}

// Channel flag constants (from TinyMUSH comsys).
const (
	ChanPublic  = 0x00000010 // Anyone can join
	ChanLoud    = 0x00000020 // Show connect/disconnect
	ChanPJoin   = 0x00000040 // Per-player join lock
	ChanPTrans  = 0x00000080 // Per-player transmit lock
	ChanPRecv   = 0x00000100 // Per-player receive lock
	ChanObject  = 0x00000200 // Objects can join
	ChanNoTitles = 0x00000400 // Suppress titles
)
