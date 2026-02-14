package oob

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/events"
)

// MCP (MUD Client Protocol) sends structured data in-band using #$# prefixed lines.
// Reference: http://www.moo.mud.org/mcp/mcp2.html

// MCPVersion is the MCP version we support.
const MCPVersion = "2.1"

// EncodeMCPInit returns the MCP initialization handshake message.
// This is sent in-band at the start of a connection to advertise MCP support.
func EncodeMCPInit(authKey string) string {
	return fmt.Sprintf("#$#mcp version: %s to: %s authentication-key: %s", MCPVersion, MCPVersion, authKey)
}

// EncodeMCPMessage encodes a key-value MCP message.
// Format: #$#package-name auth-key key: value key: value ...
func EncodeMCPMessage(authKey, pkg string, data map[string]string) string {
	var b strings.Builder
	b.WriteString("#$#")
	b.WriteString(pkg)
	b.WriteString(" ")
	b.WriteString(authKey)
	for k, v := range data {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
	}
	return b.String()
}

// EncodeMCPEvent converts an event to an MCP in-band message.
// Returns empty string if no MCP mapping exists.
func EncodeMCPEvent(authKey string, ev events.Event) string {
	switch ev.Type {
	case events.EvChannel:
		return EncodeMCPMessage(authKey, "comm-channel", map[string]string{
			"channel": ev.Channel,
			"text":    ev.Text,
		})
	case events.EvPage:
		return EncodeMCPMessage(authKey, "comm-private", map[string]string{
			"text": ev.Text,
		})
	default:
		return ""
	}
}

// ParseMCPMessage parses an incoming MCP line (must start with "#$#").
// Returns the package name, auth key, and key-value data.
func ParseMCPMessage(line string) (pkg, authKey string, data map[string]string) {
	data = make(map[string]string)
	if !strings.HasPrefix(line, "#$#") {
		return "", "", data
	}
	line = line[3:]
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", data
	}
	pkg = parts[0]
	authKey = parts[1]

	// Parse key: value pairs
	rest := line[len(pkg)+1+len(authKey):]
	for _, segment := range strings.Split(rest, " ") {
		segment = strings.TrimSpace(segment)
		if idx := strings.Index(segment, ": "); idx >= 0 {
			data[segment[:idx]] = segment[idx+2:]
		}
	}
	return pkg, authKey, data
}
