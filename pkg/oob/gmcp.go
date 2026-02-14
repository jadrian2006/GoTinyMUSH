package oob

import (
	"encoding/json"
	"fmt"

	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// GMCPPackage maps event types to GMCP package names.
func GMCPPackage(evType events.EventType) string {
	switch evType {
	case events.EvSay, events.EvPose, events.EvEmit:
		return "Comm.Room.Text"
	case events.EvChannel:
		return "Comm.Channel.Text"
	case events.EvPage, events.EvWhisper:
		return "Comm.Private.Text"
	case events.EvRoom:
		return "Room.Info"
	case events.EvMove:
		return "Room.Info"
	case events.EvConnect:
		return "Char.Login"
	case events.EvDisconnect:
		return "Char.Logout"
	case events.EvWho:
		return "Char.Group"
	default:
		return ""
	}
}

// EncodeGMCP encodes an event as a GMCP telnet subnegotiation sequence.
// Format: IAC SB 201 <package> <space> <json> IAC SE
// Returns nil if the event has no GMCP mapping or no structured data.
func EncodeGMCP(ev events.Event) []byte {
	pkg := GMCPPackage(ev.Type)
	if pkg == "" || ev.Data == nil {
		return nil
	}

	jsonData, err := json.Marshal(ev.Data)
	if err != nil {
		return nil
	}

	payload := fmt.Sprintf("%s %s", pkg, string(jsonData))
	buf := make([]byte, 0, len(payload)+4)
	buf = append(buf, IAC, SB, TeloptGMCP)
	buf = append(buf, []byte(payload)...)
	buf = append(buf, IAC, SE)
	return buf
}

// EncodeGMCPRoomInfo builds a GMCP Room.Info message for a room.
func EncodeGMCPRoomInfo(room *gamedb.Object, exits map[string]gamedb.DBRef) []byte {
	data := map[string]any{
		"num":  int(room.DBRef),
		"name": room.Name,
	}
	if exits != nil {
		exitMap := make(map[string]string)
		for dir, ref := range exits {
			exitMap[dir] = fmt.Sprintf("#%d", ref)
		}
		data["exits"] = exitMap
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	payload := fmt.Sprintf("Room.Info %s", string(jsonData))
	buf := make([]byte, 0, len(payload)+4)
	buf = append(buf, IAC, SB, TeloptGMCP)
	buf = append(buf, []byte(payload)...)
	buf = append(buf, IAC, SE)
	return buf
}

// ParseGMCPMessage parses an incoming GMCP message from client subnegotiation.
// The data is the raw bytes between SB 201 and IAC SE.
// Returns package name and JSON data.
func ParseGMCPMessage(data []byte) (pkg string, jsonData []byte) {
	// Find first space separator
	for i, b := range data {
		if b == ' ' {
			return string(data[:i]), data[i+1:]
		}
	}
	return string(data), nil
}
