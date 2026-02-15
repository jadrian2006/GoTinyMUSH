package gamedb

import "time"

// Mail message flag constants (matching TinyMUSH 3.3 mail module).
const (
	MailIsRead  = 0x0001
	MailCleared = 0x0002
	MailUrgent  = 0x0004
	MailSafe    = 0x0008
	MailForward = 0x0010
	MailReply   = 0x0020
)

// MailMessage represents a single mail message in a player's mailbox.
// Each recipient gets their own copy with independent read/cleared/folder state.
type MailMessage struct {
	ID      int       // Per-player sequential message number
	From    DBRef     // Sender
	To      []DBRef   // Original recipient list
	CC      []DBRef   // Carbon copy list
	Subject string
	Body    string
	Time    time.Time
	Flags   int // MailIsRead | MailCleared | etc.
	Folder  int // 0-14
}
