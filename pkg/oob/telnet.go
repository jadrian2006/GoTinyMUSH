package oob

// Telnet protocol constants used by OOB negotiations.
const (
	IAC  byte = 255 // Interpret As Command
	DONT byte = 254
	DO   byte = 253
	WONT byte = 252
	WILL byte = 251
	SB   byte = 250 // Subnegotiation Begin
	SE   byte = 240 // Subnegotiation End
	NOP  byte = 241

	// Telnet options used by OOB protocols
	TeloptGMCP byte = 201 // GMCP option number
	TeloptMSDP byte = 69  // MSDP option number
	TeloptMSSP byte = 70  // MSSP option number
)

// MSDP subnegotiation type bytes
const (
	MSDPVar   byte = 1 // Variable name follows
	MSDPVal   byte = 2 // Variable value follows
	MSDPOpen  byte = 3 // Open table/array
	MSDPClose byte = 4 // Close table/array
	MSDPArray byte = 5 // Array delimiter
)
