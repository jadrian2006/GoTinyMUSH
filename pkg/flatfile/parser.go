package flatfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Version flags from db.h
const (
	VMask        = 0x000000ff
	VZone        = 0x00000100
	VLink        = 0x00000200
	VGDBM        = 0x00000400
	VAtrName     = 0x00000800
	VAtrKey      = 0x00001000
	VParent      = 0x00002000
	VAtrMoney    = 0x00008000
	VXFlags      = 0x00010000
	VPowers      = 0x00020000
	V3Flags      = 0x00040000
	VQuoted      = 0x00080000
	VTQuotas     = 0x00100000
	VTimestamps  = 0x00200000
	VVisualAttrs = 0x00400000
)

// Database format constants
const (
	FUnknown  = 0
	FMush     = 1
	FMuse     = 2
	FMud      = 3
	FMuck     = 4
	FMux      = 5
	FTinyMUSH = 6
)

// Lock expression token characters
const (
	NotToken   = '!'
	AndToken   = '&'
	OrToken    = '|'
	IndirToken = '@'
	CarryToken = '+'
	IsToken    = '='
	OwnerToken = '$'
)

// Parser reads a TinyMUSH flatfile and produces a Database.
type Parser struct {
	reader    *bufio.Reader
	db        *gamedb.Database
	line      int
	format    int
	version   int
	flags     int
	readName  bool
	readZone  bool
	readLink  bool
	readKey   bool
	readParent bool
	readMoney  bool
	readExtFlags bool
	read3Flags   bool
	readTimestamps bool
	readNewStrings bool
	readPowers     bool
	readAttribs    bool
}

// Load reads a flatfile from disk and returns a populated Database.
func Load(path string) (*gamedb.Database, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open flatfile: %w", err)
	}
	defer f.Close()

	return Parse(f)
}

// Parse reads a flatfile from the given reader.
func Parse(r io.Reader) (*gamedb.Database, error) {
	p := &Parser{
		reader:     bufio.NewReaderSize(r, 256*1024),
		db:         gamedb.NewDatabase(),
		format:     FUnknown,
		readName:   true,
		readKey:    true,
		readMoney:  true,
		readAttribs: true,
	}
	if err := p.parse(); err != nil {
		return nil, err
	}
	return p.db, nil
}

func (p *Parser) parse() error {
	for {
		ch, err := p.peekByte()
		if err == io.EOF {
			return fmt.Errorf("unexpected EOF at line %d (no end-of-dump marker)", p.line)
		}
		if err != nil {
			return fmt.Errorf("read error at line %d: %w", p.line, err)
		}

		switch ch {
		case '+':
			if err := p.parseHeader(); err != nil {
				return err
			}
		case '-':
			if err := p.parseMiscTag(); err != nil {
				return err
			}
		case '!':
			if err := p.parseObject(); err != nil {
				return err
			}
		case '*':
			return p.parseEOF()
		case '\n', '\r':
			p.readLine()
			continue
		default:
			return fmt.Errorf("unexpected character '%c' at line %d", ch, p.line)
		}
	}
}

// parseHeader handles + prefixed lines: +T (version), +S (size), +A (attr def), +N (next attr), +F (free attr)
func (p *Parser) parseHeader() error {
	p.mustReadByte() // consume '+'
	ch, err := p.mustReadByte()
	if err != nil {
		return err
	}

	switch ch {
	case 'T': // TinyMUSH 3.0 version header
		val, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading version: %w", err)
		}
		p.format = FTinyMUSH
		p.db.Format = FTinyMUSH
		p.applyVersionFlags(val)
		p.version = val & VMask
		p.db.Version = p.version
		p.read3Flags = (val & V3Flags) != 0
		p.readPowers = (val & VPowers) != 0
		p.readNewStrings = (val & VQuoted) != 0

	case 'V': // TinyMUSH 2.x version header
		val, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading version: %w", err)
		}
		p.format = FMush
		p.db.Format = FMush
		p.applyVersionFlags(val)
		p.version = val & VMask
		p.db.Version = p.version

	case 'X': // TinyMUX version header
		val, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading version: %w", err)
		}
		p.format = FMux
		p.db.Format = FMux
		p.applyVersionFlags(val)
		p.version = val & VMask
		p.db.Version = p.version
		p.read3Flags = (val & V3Flags) != 0
		p.readPowers = (val & VPowers) != 0
		p.readNewStrings = (val & VQuoted) != 0

	case 'S': // Size
		val, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading size: %w", err)
		}
		p.db.Size = val

	case 'N': // Next attr number
		val, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading next attr: %w", err)
		}
		p.db.NextAttr = val

	case 'A': // User-defined attribute definition
		num, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading attr def number: %w", err)
		}
		str, err := p.readString(p.readNewStrings)
		if err != nil {
			return fmt.Errorf("reading attr def string: %w", err)
		}
		// Parse "flags:name" format
		aflags := 0
		name := str
		if len(str) > 0 && unicode.IsDigit(rune(str[0])) {
			idx := strings.IndexByte(str, ':')
			if idx > 0 {
				aflags, _ = strconv.Atoi(str[:idx])
				name = str[idx+1:]
			}
		}
		p.db.AddAttrDef(num, name, aflags)

	case 'F': // Free attribute slot
		_, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading free attr: %w", err)
		}

	default:
		// Unknown header type, consume the line
		p.readLine()
	}

	return nil
}

func (p *Parser) applyVersionFlags(val int) {
	p.flags = val & ^VMask
	p.db.Flags = p.flags

	if val&VGDBM != 0 {
		p.readAttribs = true // attrs are still in flatfile for flatfile dumps
		p.readName = (val & VAtrName) == 0
	}
	if val&VZone != 0 {
		p.readZone = true
	}
	if val&VLink != 0 {
		p.readLink = true
	}
	if val&VAtrKey != 0 {
		p.readKey = false
	}
	if val&VParent != 0 {
		p.readParent = true
	}
	if val&VAtrMoney != 0 {
		p.readMoney = false
	}
	if val&VXFlags != 0 {
		p.readExtFlags = true
	}
	if val&VTimestamps != 0 {
		p.readTimestamps = true
	}
}

// parseMiscTag handles - prefixed lines
func (p *Parser) parseMiscTag() error {
	p.mustReadByte() // consume '-'
	ch, err := p.mustReadByte()
	if err != nil {
		return err
	}
	switch ch {
	case 'R': // Record players
		val, err := p.readInt()
		if err != nil {
			return fmt.Errorf("reading record players: %w", err)
		}
		p.db.RecordPlayers = val
	default:
		p.readLine()
	}
	return nil
}

// parseObject reads a single object entry starting with !<dbref>
func (p *Parser) parseObject() error {
	p.mustReadByte() // consume '!'
	ref, err := p.readInt()
	if err != nil {
		return fmt.Errorf("reading object dbref: %w", err)
	}

	obj := &gamedb.Object{
		DBRef:    gamedb.DBRef(ref),
		Location: gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    gamedb.Nothing,
		Parent:   gamedb.Nothing,
	}

	// NAME
	if p.readName {
		name, err := p.readString(p.readNewStrings)
		if err != nil {
			return fmt.Errorf("object #%d name: %w", ref, err)
		}
		obj.Name = name
	}

	// LOCATION
	loc, err := p.readInt()
	if err != nil {
		return fmt.Errorf("object #%d location: %w", ref, err)
	}
	obj.Location = gamedb.DBRef(loc)

	// ZONE (conditional)
	if p.readZone {
		z, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d zone: %w", ref, err)
		}
		obj.Zone = gamedb.DBRef(z)
	}

	// CONTENTS
	c, err := p.readInt()
	if err != nil {
		return fmt.Errorf("object #%d contents: %w", ref, err)
	}
	obj.Contents = gamedb.DBRef(c)

	// EXITS
	e, err := p.readInt()
	if err != nil {
		return fmt.Errorf("object #%d exits: %w", ref, err)
	}
	obj.Exits = gamedb.DBRef(e)

	// LINK (conditional)
	if p.readLink {
		l, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d link: %w", ref, err)
		}
		obj.Link = gamedb.DBRef(l)
	}

	// NEXT
	n, err := p.readInt()
	if err != nil {
		return fmt.Errorf("object #%d next: %w", ref, err)
	}
	obj.Next = gamedb.DBRef(n)

	// LOCK (conditional - only if lock is in header, not as attribute)
	if p.readKey {
		boolexp, err := p.readBoolExp()
		if err != nil {
			return fmt.Errorf("object #%d lock: %w", ref, err)
		}
		obj.Lock = boolexp
	}

	// OWNER
	o, err := p.readInt()
	if err != nil {
		return fmt.Errorf("object #%d owner: %w", ref, err)
	}
	obj.Owner = gamedb.DBRef(o)

	// PARENT (conditional)
	if p.readParent {
		par, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d parent: %w", ref, err)
		}
		obj.Parent = gamedb.DBRef(par)
	}

	// PENNIES (conditional)
	if p.readMoney {
		pen, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d pennies: %w", ref, err)
		}
		obj.Pennies = pen
	}

	// FLAGS (word 1)
	f1, err := p.readInt()
	if err != nil {
		return fmt.Errorf("object #%d flags1: %w", ref, err)
	}
	obj.Flags[0] = f1

	// FLAGS (word 2, conditional)
	if p.readExtFlags {
		f2, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d flags2: %w", ref, err)
		}
		obj.Flags[1] = f2
	}

	// FLAGS (word 3, conditional)
	if p.read3Flags {
		f3, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d flags3: %w", ref, err)
		}
		obj.Flags[2] = f3
	}

	// POWERS (conditional)
	if p.readPowers {
		pw1, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d powers1: %w", ref, err)
		}
		pw2, err := p.readInt()
		if err != nil {
			return fmt.Errorf("object #%d powers2: %w", ref, err)
		}
		obj.Powers[0] = pw1
		obj.Powers[1] = pw2
	}

	// TIMESTAMPS (conditional)
	if p.readTimestamps {
		acc, err := p.readLong()
		if err != nil {
			return fmt.Errorf("object #%d access time: %w", ref, err)
		}
		mod, err := p.readLong()
		if err != nil {
			return fmt.Errorf("object #%d mod time: %w", ref, err)
		}
		obj.LastAccess = time.Unix(acc, 0)
		obj.LastMod = time.Unix(mod, 0)
	}

	// ATTRIBUTES
	if p.readAttribs {
		attrs, err := p.readAttrList()
		if err != nil {
			return fmt.Errorf("object #%d attrs: %w", ref, err)
		}
		obj.Attrs = attrs
	}

	p.db.Objects[obj.DBRef] = obj
	return nil
}

// readAttrList reads the > ... < delimited attribute section.
func (p *Parser) readAttrList() ([]gamedb.Attribute, error) {
	var attrs []gamedb.Attribute

	for {
		ch, err := p.peekByte()
		if err != nil {
			return attrs, fmt.Errorf("unexpected EOF in attr list")
		}

		switch ch {
		case '>':
			p.mustReadByte() // consume '>'
			num, err := p.readInt()
			if err != nil {
				return attrs, fmt.Errorf("reading attr number: %w", err)
			}
			val, err := p.readString(p.readNewStrings)
			if err != nil {
				return attrs, fmt.Errorf("reading attr value: %w", err)
			}
			if num > 0 {
				attrs = append(attrs, gamedb.Attribute{
					Number: num,
					Value:  val,
				})
			}
		case '<':
			p.mustReadByte() // consume '<'
			p.readLine()     // consume trailing newline
			return attrs, nil
		case '\n', '\r':
			p.readLine()
			continue
		default:
			// Bad character, try to skip the value
			p.mustReadByte()
			p.readString(p.readNewStrings)
		}
	}
}

// readBoolExp reads a boolean lock expression terminated by newline.
func (p *Parser) readBoolExp() (*gamedb.BoolExp, error) {
	b, err := p.readBoolExp1()
	if err != nil {
		return nil, err
	}
	// Consume trailing newline(s)
	for {
		ch, err := p.peekByte()
		if err != nil || ch != '\n' {
			break
		}
		p.mustReadByte()
	}
	return b, nil
}

func (p *Parser) readBoolExp1() (*gamedb.BoolExp, error) {
	ch, err := p.peekByte()
	if err != nil {
		return nil, err
	}

	switch ch {
	case '\n':
		// TRUE_BOOLEXP (null lock = unlocked)
		return nil, nil

	case '(':
		p.mustReadByte() // consume '('
		ch2, _ := p.peekByte()

		switch ch2 {
		case NotToken:
			p.mustReadByte()
			sub, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			p.consumeOptionalNewline()
			p.expectByte(')')
			return &gamedb.BoolExp{Type: gamedb.BoolNot, Sub1: sub}, nil

		case IndirToken:
			p.mustReadByte()
			sub, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			p.consumeOptionalNewline()
			p.expectByte(')')
			return &gamedb.BoolExp{Type: gamedb.BoolIndir, Sub1: sub}, nil

		case IsToken:
			p.mustReadByte()
			sub, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			p.consumeOptionalNewline()
			p.expectByte(')')
			return &gamedb.BoolExp{Type: gamedb.BoolIs, Sub1: sub}, nil

		case CarryToken:
			p.mustReadByte()
			sub, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			p.consumeOptionalNewline()
			p.expectByte(')')
			return &gamedb.BoolExp{Type: gamedb.BoolCarry, Sub1: sub}, nil

		case OwnerToken:
			p.mustReadByte()
			sub, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			p.consumeOptionalNewline()
			p.expectByte(')')
			return &gamedb.BoolExp{Type: gamedb.BoolOwner, Sub1: sub}, nil

		default:
			// Binary expression: sub1 OP sub2
			sub1, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			p.consumeOptionalNewline()
			op, _ := p.mustReadByte()
			b := &gamedb.BoolExp{Sub1: sub1}
			switch op {
			case AndToken:
				b.Type = gamedb.BoolAnd
			case OrToken:
				b.Type = gamedb.BoolOr
			default:
				return nil, fmt.Errorf("unexpected operator '%c' in boolexp", op)
			}
			sub2, err := p.readBoolExp1()
			if err != nil {
				return nil, err
			}
			b.Sub2 = sub2
			p.consumeOptionalNewline()
			p.expectByte(')')
			return b, nil
		}

	case '-':
		// Obsolete NOTHING key, eat it
		p.readLine()
		return nil, nil

	case '"':
		// Quoted attribute lock
		str, err := p.readString(true)
		if err != nil {
			return nil, err
		}
		ch2, _ := p.peekByte()
		if ch2 == ':' || ch2 == '/' {
			p.mustReadByte()
			val := p.readUntilBoolTerminator()
			b := &gamedb.BoolExp{Thing: 0, StrVal: val}
			if ch2 == '/' {
				b.Type = gamedb.BoolEval
			} else {
				b.Type = gamedb.BoolAttr
			}
			// Store the attr name for now; we'd resolve it in a real implementation
			_ = str
			return b, nil
		}
		return &gamedb.BoolExp{Type: gamedb.BoolConst}, nil

	default:
		// dbref number or named attribute lock
		if ch >= '0' && ch <= '9' {
			num := 0
			for {
				ch, err := p.peekByte()
				if err != nil || ch < '0' || ch > '9' {
					break
				}
				p.mustReadByte()
				num = num*10 + int(ch-'0')
			}
			// Check for : or /
			ch, _ = p.peekByte()
			if ch == ':' || ch == '/' {
				p.mustReadByte()
				val := p.readUntilBoolTerminator()
				b := &gamedb.BoolExp{Thing: num, StrVal: val}
				if ch == '/' {
					b.Type = gamedb.BoolEval
				} else {
					b.Type = gamedb.BoolAttr
				}
				return b, nil
			}
			return &gamedb.BoolExp{Type: gamedb.BoolConst, Thing: num}, nil
		} else if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			// Attribute name
			var name strings.Builder
			for {
				ch, err := p.peekByte()
				if err != nil || ch == ':' || ch == '/' || ch == '\n' {
					break
				}
				p.mustReadByte()
				name.WriteByte(ch)
			}
			ch, _ = p.peekByte()
			if ch == ':' || ch == '/' {
				p.mustReadByte()
				val := p.readUntilBoolTerminator()
				b := &gamedb.BoolExp{Thing: 0, StrVal: val}
				if ch == '/' {
					b.Type = gamedb.BoolEval
				} else {
					b.Type = gamedb.BoolAttr
				}
				return b, nil
			}
			return &gamedb.BoolExp{Type: gamedb.BoolConst}, nil
		}
		return nil, nil
	}
}

func (p *Parser) readUntilBoolTerminator() string {
	var s strings.Builder
	for {
		ch, err := p.peekByte()
		if err != nil || ch == '\n' || ch == ')' || ch == OrToken || ch == AndToken {
			break
		}
		p.mustReadByte()
		s.WriteByte(ch)
	}
	return s.String()
}

// parseEOF handles the ***END OF DUMP*** marker.
func (p *Parser) parseEOF() error {
	line, _ := p.readLine()
	if strings.TrimSpace(line) != "***END OF DUMP***" {
		return fmt.Errorf("bad EOF marker: %q", line)
	}
	return nil
}

// --- Low-level I/O helpers ---

func (p *Parser) peekByte() (byte, error) {
	b, err := p.reader.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (p *Parser) mustReadByte() (byte, error) {
	b, err := p.reader.ReadByte()
	if b == '\n' {
		p.line++
	}
	return b, err
}

func (p *Parser) expectByte(expected byte) error {
	b, err := p.mustReadByte()
	if err != nil {
		return err
	}
	if b != expected {
		return fmt.Errorf("expected '%c' got '%c' at line %d", expected, b, p.line)
	}
	return nil
}

func (p *Parser) consumeOptionalNewline() {
	ch, err := p.peekByte()
	if err == nil && ch == '\n' {
		p.mustReadByte()
	}
}

// readLine reads until end of line and returns the content (excluding newline).
func (p *Parser) readLine() (string, error) {
	line, err := p.reader.ReadString('\n')
	p.line++
	return strings.TrimRight(line, "\r\n"), err
}

// readInt reads a line and parses it as an integer.
func (p *Parser) readInt() (int, error) {
	line, err := p.readLine()
	if err != nil && err != io.EOF {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, nil
	}
	return strconv.Atoi(line)
}

// readLong reads a line and parses it as int64.
func (p *Parser) readLong() (int64, error) {
	line, err := p.readLine()
	if err != nil && err != io.EOF {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, nil
	}
	return strconv.ParseInt(line, 10, 64)
}

// readString reads a potentially quoted string.
// If quoted (starts with "), reads the content between quotes.
// Otherwise reads a plain line.
func (p *Parser) readString(newStrings bool) (string, error) {
	ch, err := p.peekByte()
	if err != nil {
		return "", err
	}

	if ch == '"' {
		return p.readQuotedString()
	}
	return p.readLine()
}

// readQuotedString reads a "..." delimited string, handling escapes.
func (p *Parser) readQuotedString() (string, error) {
	p.mustReadByte() // consume opening "

	var buf strings.Builder
	for {
		b, err := p.mustReadByte()
		if err != nil {
			return buf.String(), err
		}
		switch b {
		case '"':
			// End of string — consume trailing newline if present
			ch, err := p.peekByte()
			if err == nil && (ch == '\n' || ch == '\r') {
				p.readLine()
			}
			return buf.String(), nil
		case '\\':
			// Escape sequence
			next, err := p.mustReadByte()
			if err != nil {
				return buf.String(), err
			}
			switch next {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			default:
				buf.WriteByte('\\')
				buf.WriteByte(next)
			}
		default:
			buf.WriteByte(b)
		}
	}
}
