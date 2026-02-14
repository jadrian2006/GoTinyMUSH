package flatfile

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// ParseComsys reads a mod_comsys.db file and returns channels and aliases.
func ParseComsys(r io.Reader) ([]gamedb.Channel, []gamedb.ChanAlias, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Expect +V4 header
	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("comsys: empty file")
	}
	line := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(line, "+V") {
		return nil, nil, fmt.Errorf("comsys: expected +V header, got %q", line)
	}

	// Parse channels
	var channels []gamedb.Channel
	for {
		if !scanner.Scan() {
			break
		}
		line = strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Check for alias section header
		if strings.HasPrefix(line, "+V") {
			break
		}
		// Each channel starts with a quoted name
		ch, err := parseChannel(line, scanner)
		if err != nil {
			return nil, nil, fmt.Errorf("comsys: parse channel: %w", err)
		}
		channels = append(channels, ch)
	}

	// Parse aliases (we already consumed the +V1 header)
	var aliases []gamedb.ChanAlias
	for {
		if !scanner.Scan() {
			break
		}
		line = strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "*** END OF DUMP ***" {
			break
		}
		alias, err := parseAlias(line, scanner)
		if err != nil {
			return nil, nil, fmt.Errorf("comsys: parse alias: %w", err)
		}
		aliases = append(aliases, alias)
	}

	return channels, aliases, scanner.Err()
}

// parseChannel parses a single channel record. The first line (quoted name) has already been read.
func parseChannel(nameLine string, scanner *bufio.Scanner) (gamedb.Channel, error) {
	var ch gamedb.Channel
	ch.Name = unquote(nameLine)

	// owner
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel owner")
	}
	owner, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		return ch, fmt.Errorf("bad owner %q: %w", scanner.Text(), err)
	}
	ch.Owner = gamedb.DBRef(owner)

	// flags
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel flags")
	}
	ch.Flags, _ = strconv.Atoi(strings.TrimSpace(scanner.Text()))

	// charge
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel charge")
	}
	ch.Charge, _ = strconv.Atoi(strings.TrimSpace(scanner.Text()))

	// charge_collected
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel charge_collected")
	}
	ch.ChargeCollected, _ = strconv.Atoi(strings.TrimSpace(scanner.Text()))

	// num_sent
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel num_sent")
	}
	ch.NumSent, _ = strconv.Atoi(strings.TrimSpace(scanner.Text()))

	// description
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel description")
	}
	ch.Description = unquote(strings.TrimSpace(scanner.Text()))

	// header (ANSI)
	if !scanner.Scan() {
		return ch, fmt.Errorf("unexpected EOF reading channel header")
	}
	ch.Header = convertANSI(unquote(strings.TrimSpace(scanner.Text())))

	// join_lock: read lines until "-" separator
	ch.JoinLock = readLockUntilDash(scanner)

	// trans_lock
	ch.TransLock = readLockUntilDash(scanner)

	// recv_lock
	ch.RecvLock = readLockUntilDash(scanner)

	// terminator "<"
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "<" {
			break
		}
	}

	return ch, nil
}

// readLockUntilDash reads lines until a "-" separator line, collecting lock text.
func readLockUntilDash(scanner *bufio.Scanner) string {
	var parts []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "-" {
			break
		}
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

// parseAlias parses a single alias record. The first line (player dbref) has already been read.
func parseAlias(dbrefLine string, scanner *bufio.Scanner) (gamedb.ChanAlias, error) {
	var ca gamedb.ChanAlias

	ref, err := strconv.Atoi(strings.TrimSpace(dbrefLine))
	if err != nil {
		return ca, fmt.Errorf("bad player dbref %q: %w", dbrefLine, err)
	}
	ca.Player = gamedb.DBRef(ref)

	// channel name
	if !scanner.Scan() {
		return ca, fmt.Errorf("unexpected EOF reading alias channel")
	}
	ca.Channel = unquote(strings.TrimSpace(scanner.Text()))

	// alias
	if !scanner.Scan() {
		return ca, fmt.Errorf("unexpected EOF reading alias name")
	}
	ca.Alias = unquote(strings.TrimSpace(scanner.Text()))

	// title
	if !scanner.Scan() {
		return ca, fmt.Errorf("unexpected EOF reading alias title")
	}
	ca.Title = unquote(strings.TrimSpace(scanner.Text()))

	// is_listening
	if !scanner.Scan() {
		return ca, fmt.Errorf("unexpected EOF reading alias listening flag")
	}
	ca.IsListening = strings.TrimSpace(scanner.Text()) == "1"

	// terminator "<"
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "<" {
			break
		}
	}

	return ca, nil
}

// unquote removes surrounding double quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// convertANSI converts \e escape sequences to actual ESC characters (\x1b).
func convertANSI(s string) string {
	return strings.ReplaceAll(s, `\e`, "\x1b")
}
