package events

import "github.com/crystal-mush/gotinymush/pkg/gamedb"

// EventType classifies events for transport-specific encoding.
type EventType int

const (
	EvText       EventType = iota // Raw text (universal fallback)
	EvSay                         // Speech
	EvPose                        // Pose/emote
	EvPage                        // Private message
	EvChannel                     // Channel message
	EvRoom                        // Room description
	EvMove                        // Arrive/depart
	EvConnect                     // Player connected
	EvDisconnect                  // Player disconnected
	EvPrompt                      // Prompt/status update
	EvObjUpdate                   // Object changed
	EvWho                         // WHO data
	EvWhisper                     // Whisper
	EvEmit                        // @emit / @remit / @oemit
)

// String returns a human-readable name for the event type.
func (t EventType) String() string {
	switch t {
	case EvText:
		return "text"
	case EvSay:
		return "say"
	case EvPose:
		return "pose"
	case EvPage:
		return "page"
	case EvChannel:
		return "channel"
	case EvRoom:
		return "room"
	case EvMove:
		return "move"
	case EvConnect:
		return "connect"
	case EvDisconnect:
		return "disconnect"
	case EvPrompt:
		return "prompt"
	case EvObjUpdate:
		return "obj_update"
	case EvWho:
		return "who"
	case EvWhisper:
		return "whisper"
	case EvEmit:
		return "emit"
	default:
		return "unknown"
	}
}

// Event is a structured game event that flows through the event bus.
// Transports decide how to encode each event: telnet uses Text,
// WebSocket/REST use the full structured data.
type Event struct {
	Type    EventType
	Player  gamedb.DBRef   // Recipient (Nothing for broadcast)
	Source  gamedb.DBRef   // Who generated the event
	Room    gamedb.DBRef   // Room context
	Channel string         // Channel name (EvChannel)
	Text    string         // Pre-formatted text (telnet uses this)
	Data    map[string]any // Structured data for OOB/JSON clients
}
