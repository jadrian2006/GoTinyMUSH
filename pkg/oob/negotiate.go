package oob

import (
	"io"
	"log"
	"net"
	"time"
)

// Negotiate performs OOB protocol negotiation with a telnet client.
// It sends WILL for GMCP and MSDP, waits for responses, and returns
// the negotiated capabilities plus any leftover non-IAC bytes (e.g.
// an early "connect" command sent by an autologin client).
func Negotiate(conn net.Conn, timeout time.Duration) (*Capabilities, []byte) {
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
	var leftover []byte
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

		// Parse the buffer: extract IAC sequences, accumulate non-IAC bytes
		data := buf[:n]
		for i := 0; i < len(data); {
			if data[i] == IAC && i+1 < len(data) {
				cmd := data[i+1]
				if cmd == SB {
					// Subnegotiation: IAC SB <opt> ... IAC SE
					// Scan forward to find IAC SE
					found := false
					for j := i + 2; j+1 < len(data); j++ {
						if data[j] == IAC && data[j+1] == SE {
							// Process the subnegotiation content
							if i+2 < len(data) {
								opt := data[i+2]
								_ = opt // subneg content, currently unused
							}
							i = j + 2 // skip past IAC SE
							found = true
							break
						}
					}
					if !found {
						// Incomplete subnegotiation, skip to end
						i = len(data)
					}
					continue
				}
				if i+2 < len(data) {
					// 3-byte IAC command: IAC <cmd> <opt>
					opt := data[i+2]
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
					i += 3
					continue
				}
				// 2-byte IAC sequence (IAC IAC = escaped 0xFF, or incomplete)
				i += 2
				continue
			}
			// Non-IAC byte: preserve as leftover
			leftover = append(leftover, data[i])
			i++
		}

		// If we got responses for all offered protocols, no need to wait longer
		if (caps.GMCP || caps.MSDP) && caps.MSSP {
			break
		}
	}

	// Clear deadline for normal operation
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	return caps, leftover
}
