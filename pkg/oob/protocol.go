// Package oob implements out-of-band protocol support for MUD clients.
// It supports GMCP (Generic MUD Communication Protocol), MSDP (MUD Server
// Data Protocol), and MCP (MUD Client Protocol) for sending structured
// data alongside normal text output.
package oob

// Protocol identifies which OOB protocols a client supports.
type Protocol int

const (
	ProtoGMCP Protocol = iota
	ProtoMSDP
	ProtoMCP
	ProtoMSSP
)

// Capabilities tracks which OOB protocols a connection has negotiated.
type Capabilities struct {
	GMCP bool // GMCP (telopt 201) negotiated
	MSDP bool // MSDP (telopt 69) negotiated
	MCP  bool // MCP handshake completed
	MSSP bool // MSSP (telopt 70) negotiated

	// GMCP package subscriptions from the client
	GMCPPackages map[string]bool
}

// NewCapabilities returns a zero-value Capabilities (nothing negotiated).
func NewCapabilities() *Capabilities {
	return &Capabilities{
		GMCPPackages: make(map[string]bool),
	}
}

// HasAny returns true if any OOB protocol is negotiated.
func (c *Capabilities) HasAny() bool {
	return c.GMCP || c.MSDP || c.MCP || c.MSSP
}
