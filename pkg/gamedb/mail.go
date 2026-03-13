package gamedb

import "time"

// Mail message flag constants (matching TinyMUSH 3.3 mail module).
const (
	MailIsRead  = 0x0001
	MailCleared = 0x0002
	MailUrgent  = 0x0004
	MailMass    = 0x0008
	MailSafe    = 0x0010
	MailTag     = 0x0040
	MailForward = 0x0080
	MailReply   = 0x4000
)

// MaxAliasMembers is the maximum number of players in a mail alias (matching C).
const MaxAliasMembers = 100

// MailAlias represents a named group of players for mail addressing.
// Referenced as *aliasname in mail recipient lists.
type MailAlias struct {
	Owner   DBRef   // creator/owner
	Name    string  // alias name (stored WITHOUT leading *)
	Desc    string  // description text
	Members []DBRef // player dbrefs (up to MaxAliasMembers)
}

// MailMessage represents a single mail message in a player's mailbox.
// Each recipient gets their own copy with independent read/cleared/folder state.
type MailMessage struct {
	ID      int       // Per-player sequential message number
	From    DBRef     // Sender
	To      []DBRef   // Original recipient list
	CC      []DBRef   // Carbon copy list
	BCC     []DBRef   // Blind carbon copy list (not shown to recipients)
	Subject string
	Body    string
	Time    time.Time
	Flags   int // MailIsRead | MailCleared | etc.
	Folder  int // 0-14
}
