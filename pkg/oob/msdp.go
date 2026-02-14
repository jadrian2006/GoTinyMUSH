package oob

import (
	"fmt"

	"github.com/crystal-mush/gotinymush/pkg/events"
)

// EncodeMSDP encodes key-value pairs as an MSDP telnet subnegotiation sequence.
// Format: IAC SB 69 MSDP_VAR "key" MSDP_VAL "value" ... IAC SE
func EncodeMSDP(pairs map[string]string) []byte {
	buf := []byte{IAC, SB, TeloptMSDP}
	for k, v := range pairs {
		buf = append(buf, MSDPVar)
		buf = append(buf, []byte(k)...)
		buf = append(buf, MSDPVal)
		buf = append(buf, []byte(v)...)
	}
	buf = append(buf, IAC, SE)
	return buf
}

// EncodeMSDPEvent converts an event to MSDP key-value pairs and encodes them.
// Returns nil if no MSDP mapping exists for this event type.
func EncodeMSDPEvent(ev events.Event) []byte {
	pairs := make(map[string]string)

	switch ev.Type {
	case events.EvRoom:
		pairs["ROOM"] = fmt.Sprintf("#%d", ev.Room)
		if name, ok := ev.Data["name"].(string); ok {
			pairs["ROOM_NAME"] = name
		}
	case events.EvChannel:
		pairs["CHANNEL"] = ev.Channel
		pairs["CHANNEL_TEXT"] = ev.Text
	case events.EvConnect:
		if name, ok := ev.Data["name"].(string); ok {
			pairs["CHARACTER_NAME"] = name
		}
	default:
		return nil
	}

	if len(pairs) == 0 {
		return nil
	}
	return EncodeMSDP(pairs)
}

// ParseMSDP parses an incoming MSDP subnegotiation into key-value pairs.
// The data is the raw bytes between SB 69 and IAC SE.
func ParseMSDP(data []byte) map[string]string {
	result := make(map[string]string)
	var key, val string
	inKey := false
	inVal := false

	for _, b := range data {
		switch b {
		case MSDPVar:
			if inVal && key != "" {
				result[key] = val
			}
			key = ""
			val = ""
			inKey = true
			inVal = false
		case MSDPVal:
			inKey = false
			inVal = true
		default:
			if inKey {
				key += string(b)
			} else if inVal {
				val += string(b)
			}
		}
	}
	if inVal && key != "" {
		result[key] = val
	}
	return result
}
