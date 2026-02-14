package flatfile

import (
	"os"
	"testing"
)

func TestParseComsysFile(t *testing.T) {
	f, err := os.Open("../../data/mod_comsys.db")
	if err != nil {
		t.Skipf("mod_comsys.db not found: %v", err)
	}
	defer f.Close()

	channels, aliases, err := ParseComsys(f)
	if err != nil {
		t.Fatalf("ParseComsys error: %v", err)
	}

	if len(channels) == 0 {
		t.Fatal("expected channels, got 0")
	}
	t.Logf("Parsed %d channels, %d aliases", len(channels), len(aliases))

	// Verify we got the expected counts
	if len(channels) != 34 {
		t.Errorf("expected 34 channels, got %d", len(channels))
	}

	// Check a known channel
	found := false
	for _, ch := range channels {
		if ch.Name == "Public" {
			found = true
			if ch.Description == "" {
				t.Error("Public channel has empty description")
			}
			t.Logf("Public channel: owner=#%d flags=%d header=%q desc=%q",
				ch.Owner, ch.Flags, ch.Header, ch.Description)
			break
		}
	}
	if !found {
		t.Error("Public channel not found")
	}

	// Check ANSI conversion
	for _, ch := range channels {
		if ch.Name == "marketing" {
			if ch.Header == "" {
				t.Error("marketing header is empty")
			}
			if ch.Header[0] != '\x1b' {
				t.Errorf("marketing header not ANSI-converted: %q", ch.Header)
			}
			break
		}
	}

	// Check aliases have valid data
	if len(aliases) < 100 {
		t.Errorf("expected many aliases, got %d", len(aliases))
	}
	for i, a := range aliases {
		if a.Player < 0 {
			t.Errorf("alias %d: bad player ref %d", i, a.Player)
		}
		if a.Channel == "" {
			t.Errorf("alias %d: empty channel name", i)
		}
		if a.Alias == "" {
			t.Errorf("alias %d: empty alias", i)
		}
	}
}
