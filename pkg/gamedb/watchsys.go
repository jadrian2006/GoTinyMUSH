package gamedb

// WatchRoom represents a room registered in the watch system.
type WatchRoom struct {
	Dbref  DBRef  // Room dbref
	Format string // Display format — "|" replaced with room name at display time
	Color  string // ANSI color code for room name (e.g. "c", "hr"). Empty = no color
	Owner  DBRef  // Wizard who registered it
	Flags  int    // Permission flags (see below)
}

// WatchSub represents a player's subscription to a watched room.
type WatchSub struct {
	Player    DBRef // Player dbref
	Room      DBRef // Watched room dbref
	IsActive  bool  // Currently receiving notifications
	QuietMode bool  // Suppress notifications temporarily (like allcom off)
}

// DefaultWatchFormat is the default format for new watch rooms.
const DefaultWatchFormat = "[|]> "

// Watch room flag constants.
const (
	WatchPublic = 0x01 // Anyone can subscribe (default)
	WatchGuild  = 0x02 // Guild members only (lock-based)
	WatchAdmin  = 0x04 // Admin-only visibility
)
