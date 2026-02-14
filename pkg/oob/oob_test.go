package oob

import (
	"encoding/json"
	"strings"
	"testing"

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
