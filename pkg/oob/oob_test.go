package oob

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func TestGMCPPackageMapping(t *testing.T) {
	tests := []struct {
		evType events.EventType
		want   string
	}{
		{events.EvSay, "Comm.Room.Text"},
		{events.EvChannel, "Comm.Channel.Text"},
		{events.EvPage, "Comm.Private.Text"},
		{events.EvRoom, "Room.Info"},
		{events.EvConnect, "Char.Login"},
		{events.EvText, ""},
	}
	for _, tt := range tests {
		got := GMCPPackage(tt.evType)
		if got != tt.want {
			t.Errorf("GMCPPackage(%v) = %q, want %q", tt.evType, got, tt.want)
		}
	}
}

func TestEncodeGMCP(t *testing.T) {
	ev := events.Event{
		Type:    events.EvChannel,
		Channel: "Public",
		Text:    "[Public] Someone says, \"hello\"",
		Data: map[string]any{
			"channel": "Public",
			"text":    "hello",
			"talker":  "Someone",
		},
	}
	buf := EncodeGMCP(ev)
	if buf == nil {
		t.Fatal("expected non-nil GMCP data")
	}
	// Check framing
	if buf[0] != IAC || buf[1] != SB || buf[2] != TeloptGMCP {
		t.Error("bad GMCP prefix")
	}
	if buf[len(buf)-2] != IAC || buf[len(buf)-1] != SE {
		t.Error("bad GMCP suffix")
	}
	// Check payload contains the package name and valid JSON
	payload := string(buf[3 : len(buf)-2])
	if !strings.HasPrefix(payload, "Comm.Channel.Text ") {
		t.Errorf("payload should start with package name, got: %s", payload[:30])
	}
	jsonStr := payload[len("Comm.Channel.Text "):]
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Errorf("GMCP JSON invalid: %v", err)
	}
}

func TestEncodeGMCPNoData(t *testing.T) {
	ev := events.Event{Type: events.EvText, Text: "hello"}
	if buf := EncodeGMCP(ev); buf != nil {
		t.Error("expected nil for event with no GMCP mapping")
	}
}

func TestEncodeGMCPRoomInfo(t *testing.T) {
	room := &gamedb.Object{DBRef: 42, Name: "Town Square"}
	exits := map[string]gamedb.DBRef{"north": 100, "south": 101}
	buf := EncodeGMCPRoomInfo(room, exits)
	if buf == nil {
		t.Fatal("expected non-nil Room.Info")
	}
	payload := string(buf[3 : len(buf)-2])
	if !strings.HasPrefix(payload, "Room.Info ") {
		t.Errorf("expected Room.Info prefix, got: %s", payload)
	}
}

func TestParseGMCPMessage(t *testing.T) {
	pkg, jsonData := ParseGMCPMessage([]byte("Core.Hello {\"client\":\"Mudlet\"}"))
	if pkg != "Core.Hello" {
		t.Errorf("expected Core.Hello, got %q", pkg)
	}
	if string(jsonData) != "{\"client\":\"Mudlet\"}" {
		t.Errorf("unexpected JSON data: %s", jsonData)
	}

	// No JSON
	pkg, jsonData = ParseGMCPMessage([]byte("Core.Ping"))
	if pkg != "Core.Ping" {
		t.Errorf("expected Core.Ping, got %q", pkg)
	}
	if jsonData != nil {
		t.Error("expected nil jsonData for package without data")
	}
}

func TestEncodeMSDP(t *testing.T) {
	pairs := map[string]string{"ROOM": "#42", "ROOM_NAME": "Town Square"}
	buf := EncodeMSDP(pairs)
	if buf[0] != IAC || buf[1] != SB || buf[2] != TeloptMSDP {
		t.Error("bad MSDP prefix")
	}
	if buf[len(buf)-2] != IAC || buf[len(buf)-1] != SE {
		t.Error("bad MSDP suffix")
	}
}

func TestParseMSDP(t *testing.T) {
	// Build a raw MSDP payload
	data := []byte{MSDPVar}
	data = append(data, []byte("ROOM")...)
	data = append(data, MSDPVal)
	data = append(data, []byte("#42")...)
	data = append(data, MSDPVar)
	data = append(data, []byte("NAME")...)
	data = append(data, MSDPVal)
	data = append(data, []byte("Town")...)

	result := ParseMSDP(data)
	if result["ROOM"] != "#42" {
		t.Errorf("ROOM = %q, want #42", result["ROOM"])
	}
	if result["NAME"] != "Town" {
		t.Errorf("NAME = %q, want Town", result["NAME"])
	}
}

func TestMCPInit(t *testing.T) {
	msg := EncodeMCPInit("testkey123")
	if !strings.HasPrefix(msg, "#$#mcp") {
		t.Errorf("MCP init should start with #$#mcp, got: %s", msg)
	}
	if !strings.Contains(msg, "testkey123") {
		t.Error("MCP init should contain auth key")
	}
}

func TestEncodeMCPMessage(t *testing.T) {
	msg := EncodeMCPMessage("authkey", "comm-channel", map[string]string{
		"channel": "Public",
		"text":    "hello",
	})
	if !strings.HasPrefix(msg, "#$#comm-channel authkey") {
		t.Errorf("unexpected MCP message: %s", msg)
	}
}

func TestCapabilities(t *testing.T) {
	caps := NewCapabilities()
	if caps.HasAny() {
		t.Error("new capabilities should not have any protocol")
	}
	caps.GMCP = true
	if !caps.HasAny() {
		t.Error("should have GMCP")
	}
}

func TestNegotiate_PreservesLeftover(t *testing.T) {
	// Create a pipe pair to simulate a client connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan struct{})
	var caps *Capabilities
	var leftover []byte

	go func() {
		caps, leftover = Negotiate(server, 500*time.Millisecond)
		close(done)
	}()

	// Read the WILL commands the server sends
	buf := make([]byte, 64)
	client.SetReadDeadline(time.Now().Add(time.Second))
	client.Read(buf)

	// Send DO GMCP followed by a connect command (simulating autologin)
	response := []byte{IAC, DO, TeloptGMCP}
	response = append(response, []byte("connect Player pass123\r\n")...)
	client.Write(response)

	<-done

	if !caps.GMCP {
		t.Error("expected GMCP to be negotiated")
	}
	want := "connect Player pass123\r\n"
	if string(leftover) != want {
		t.Errorf("leftover = %q, want %q", string(leftover), want)
	}
}

func TestNegotiate_SubnegotiationSkipped(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan struct{})
	var caps *Capabilities
	var leftover []byte

	go func() {
		caps, leftover = Negotiate(server, 500*time.Millisecond)
		close(done)
	}()

	buf := make([]byte, 64)
	client.SetReadDeadline(time.Now().Add(time.Second))
	client.Read(buf)

	// Send DO GMCP + a GMCP subnegotiation + a connect command
	response := []byte{IAC, DO, TeloptGMCP}
	// IAC SB GMCP "Core.Hello {}" IAC SE
	response = append(response, IAC, SB, TeloptGMCP)
	response = append(response, []byte("Core.Hello {}")...)
	response = append(response, IAC, SE)
	response = append(response, []byte("connect Test pw\n")...)
	client.Write(response)

	<-done

	if !caps.GMCP {
		t.Error("expected GMCP negotiated")
	}
	if string(leftover) != "connect Test pw\n" {
		t.Errorf("leftover = %q, want %q", string(leftover), "connect Test pw\n")
	}
}

func TestNegotiate_NoResponse(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan struct{})
	var caps *Capabilities
	var leftover []byte

	go func() {
		caps, leftover = Negotiate(server, 200*time.Millisecond)
		close(done)
	}()

	// Read but send nothing — simulates dumb client
	buf := make([]byte, 64)
	client.SetReadDeadline(time.Now().Add(time.Second))
	client.Read(buf)

	<-done

	if caps.HasAny() {
		t.Error("expected no capabilities for silent client")
	}
	if len(leftover) != 0 {
		t.Errorf("expected no leftover, got %q", leftover)
	}
}
