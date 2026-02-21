package oob

import (
	"io"
	"log"
	"net"
	"time"
)

// Negotiate performs OOB protocol negotiation with a telnet client.
// It sends WILL for GMCP and MSDP, waits for responses, and returns
// the negotiated capabilities. The timeout controls how long to wait
// for client responses.
func Negotiate(conn net.Conn, timeout time.Duration) *Capabilities {
	caps := NewCapabilities()

	// Send WILL GMCP, WILL MSDP, and WILL MSSP
	willGMCP := []byte{IAC, WILL, TeloptGMCP}
	willMSDP := []byte{IAC, WILL, TeloptMSDP}
	willMSSP := []byte{IAC, WILL, TeloptMSSP}

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	conn.Write(willGMCP)
	conn.Write(willMSDP)
	conn.Write(willMSSP)

	// Read responses within timeout
	conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 256)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break // Timeout — done negotiating
			}
			if err == io.EOF {
				break
			}
			log.Printf("oob negotiate read error: %v", err)
			break
		}

		// Parse IAC sequences in the response
		for i := 0; i < n-2; i++ {
			if buf[i] != IAC {
				continue
			}
			cmd := buf[i+1]
			opt := buf[i+2]
			switch {
			case cmd == DO && opt == TeloptGMCP:
				caps.GMCP = true
				log.Printf("oob: client supports GMCP")
			case cmd == DO && opt == TeloptMSDP:
				caps.MSDP = true
				log.Printf("oob: client supports MSDP")
			case cmd == DONT && opt == TeloptGMCP:
				log.Printf("oob: client declined GMCP")
			case cmd == DONT && opt == TeloptMSDP:
				log.Printf("oob: client declined MSDP")
			case cmd == DO && opt == TeloptMSSP:
				caps.MSSP = true
				log.Printf("oob: client supports MSSP")
			case cmd == DONT && opt == TeloptMSSP:
				log.Printf("oob: client declined MSSP")
			}
			i += 2 // Skip the 3-byte sequence
		}

		// If we got responses for all offered protocols, no need to wait longer
		if (caps.GMCP || caps.MSDP) && caps.MSSP {
			break
		}
	}

	// Clear deadline for normal operation
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	return caps
}
