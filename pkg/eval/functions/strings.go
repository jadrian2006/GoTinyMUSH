package functions

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"hash/crc32"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func fnCat(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	buf.WriteString(strings.Join(args, " "))
}

func fnStrcat(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	for _, a := range args {
		buf.WriteString(a)
	}
}

func fnStrlen(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { writeInt(buf, 0); return }
	// Strip ANSI for length counting
	writeInt(buf, len(stripAnsiStr(args[0])))
}

func fnMid(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	s := args[0]
	start := toInt(args[1])
	length := toInt(args[2])
	if length < 0 { return }
	// C behavior: negative start reduces length by |start|, clamps start to 0
	if start < 0 {
		length += start // reduces length
		start = 0
	}
	if length <= 0 { return }
	if start >= len(s) { return }
	end := start + length
	if end > len(s) { end = len(s) }
	buf.WriteString(s[start:end])
}

func fnLeft(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	s := args[0]
	n := toInt(args[1])
	if n < 0 { n = 0 }
	if n > len(s) { n = len(s) }
	buf.WriteString(s[:n])
}

func fnRight(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	s := args[0]
	n := toInt(args[1])
	if n < 0 { n = 0 }
	if n > len(s) { n = len(s) }
	buf.WriteString(s[len(s)-n:])
}

func fnLcstr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	buf.WriteString(strings.ToLower(args[0]))
}

func fnUcstr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	buf.WriteString(strings.ToUpper(args[0]))
}

func fnCapstr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { return }
	s := args[0]
	buf.WriteString(strings.ToUpper(s[:1]) + s[1:])
}

func fnPos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("#-1"); return }
	idx := strings.Index(args[1], args[0])
	if idx < 0 {
		buf.WriteString("#-1")
	} else {
		writeInt(buf, idx+1) // C TinyMUSH pos() is 1-based
	}
}

func fnLpos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	s := args[0]
	charset := args[1]
	if charset == "" { return }
	var positions []string
	for i, ch := range s {
		if strings.ContainsRune(charset, ch) {
			positions = append(positions, strconv.Itoa(i))
		}
	}
	buf.WriteString(strings.Join(positions, " "))
}

func fnEdit(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		return
	}
	result := args[0]
	// Process pairs of from/to arguments (C supports multiple pairs)
	for i := 1; i+1 < len(args); i += 2 {
		from := args[i]
		to := args[i+1]
		if from == "" {
			continue
		}
		if from == "^" {
			// Prepend
			result = to + result
		} else if from == "$" {
			// Append
			result = result + to
		} else {
			result = strings.ReplaceAll(result, from, to)
		}
	}
	buf.WriteString(result)
}

func fnReplace(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	delim := " "
	if len(args) > 3 { delim = args[3] }
	words := splitList(args[0], delim)
	positions := splitList(args[1], " ")
	replacements := splitList(args[2], " ")
	for i, posStr := range positions {
		pos := toInt(posStr) - 1
		if pos >= 0 && pos < len(words) && i < len(replacements) {
			words[pos] = replacements[i]
		}
	}
	buf.WriteString(strings.Join(words, delim))
}

func fnTrim(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	side := "b"
	if len(args) > 1 && len(args[1]) > 0 { side = strings.ToLower(args[1]) }
	trimChar := " "
	if len(args) > 2 && len(args[2]) > 0 { trimChar = args[2] }
	switch side {
	case "l":
		buf.WriteString(strings.TrimLeft(s, trimChar))
	case "r":
		buf.WriteString(strings.TrimRight(s, trimChar))
	default:
		buf.WriteString(strings.Trim(s, trimChar))
	}
}

func fnSquish(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	if delim == " " {
		s = strings.TrimSpace(s)
	} else {
		s = strings.Trim(s, delim)
	}
	// Compress runs of delimiter
	prev := false
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], delim) {
			if !prev {
				buf.WriteString(delim)
				prev = true
			}
			i += len(delim)
		} else {
			buf.WriteByte(s[i])
			prev = false
			i++
		}
	}
}

// visualLen returns the display width of a string, ignoring ANSI escape
// sequences. In TinyMUSH, ljust/rjust/center pad based on visible characters,
// not raw byte length, so ANSI color codes don't count toward width.
func visualLen(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ESC [ ... <letter> sequence
			i += 2
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			continue
		}
		n++
	}
	return n
}

func fnLjust(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	s := args[0]
	width := toInt(args[1])
	fill := " "
	if len(args) > 2 && len(args[2]) > 0 { fill = args[2] }
	buf.WriteString(s)
	writePad(buf, width-visualLen(s), fill)
}

func fnRjust(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	s := args[0]
	width := toInt(args[1])
	fill := " "
	if len(args) > 2 && len(args[2]) > 0 { fill = args[2] }
	writePad(buf, width-visualLen(s), fill)
	buf.WriteString(s)
}

// writePad writes exactly n characters of padding using the fill pattern,
// truncating the last repeat if needed (C behavior for multi-char fill).
func writePad(buf *strings.Builder, n int, fill string) {
	if n <= 0 || fill == "" { return }
	for n > 0 {
		if len(fill) <= n {
			buf.WriteString(fill)
			n -= len(fill)
		} else {
			buf.WriteString(fill[:n])
			n = 0
		}
	}
}

func fnCenter(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	s := args[0]
	width := toInt(args[1])
	fill := " "
	if len(args) > 2 && len(args[2]) > 0 { fill = args[2] }
	padTotal := width - visualLen(s)
	if padTotal <= 0 {
		buf.WriteString(s)
		return
	}
	leftPad := padTotal / 2
	writePad(buf, leftPad, fill)
	buf.WriteString(s)
	writePad(buf, padTotal-leftPad, fill)
}

func fnRepeat(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	n := toInt(args[1])
	if n < 0 { n = 0 }
	if n > 10000 { n = 10000 }
	buf.WriteString(strings.Repeat(args[0], n))
}

func fnSpace(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	n := 1
	if len(args) > 0 { n = toInt(args[0]) }
	// Match C: space(0) returns "", but negative values return 1 space
	if n < 0 { n = 1 }
	if n > 10000 { n = 10000 }
	buf.WriteString(strings.Repeat(" ", n))
}

func fnEscape(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	for i, ch := range s {
		if i == 0 || ch == '%' || ch == '\\' || ch == '[' || ch == ']' || ch == '{' || ch == '}' || ch == ';' {
			buf.WriteByte('\\')
		}
		buf.WriteRune(ch)
	}
}

func fnSecure(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	for _, ch := range args[0] {
		switch ch {
		case '%', '$', '\\', '[', ']', '(', ')', '{', '}', ',', ';':
			buf.WriteByte(' ')
		default:
			buf.WriteRune(ch)
		}
	}
}

func fnAnsi(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	codes := args[0]
	text := args[1]
	i := 0
	for i < len(codes) {
		// Check for extended color spec: <...> or /<...>
		if codes[i] == '/' && i+1 < len(codes) && codes[i+1] == '<' {
			end := strings.IndexByte(codes[i+2:], '>')
			if end >= 0 {
				spec := codes[i+2 : i+2+end]
				code := eval.ParseColorSpec(spec, true)
				if code != "" {
					buf.WriteString(code)
				}
				i = i + 2 + end + 1
				continue
			}
		}
		if codes[i] == '<' {
			end := strings.IndexByte(codes[i+1:], '>')
			if end >= 0 {
				spec := codes[i+1 : i+1+end]
				code := eval.ParseColorSpec(spec, false)
				if code != "" {
					buf.WriteString(code)
				}
				i = i + 1 + end + 1
				continue
			}
		}
		code := eval.AnsiCode(codes[i])
		if code != "" {
			buf.WriteString(code)
		}
		i++
	}
	buf.WriteString(text)
	buf.WriteString("\033[0m")
}

func fnStripansi(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	buf.WriteString(stripAnsiStr(args[0]))
}

var ansiRegexp = regexp.MustCompile(`\033\[[0-9;]*m`)

func stripAnsiStr(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func fnBefore(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	s := args[0]
	// C's before() trims leading delimiters first (trim_space_sep)
	s = strings.TrimLeft(s, delim)
	idx := strings.Index(s, delim)
	if idx < 0 {
		buf.WriteString(s)
		return
	}
	buf.WriteString(s[:idx])
}

func fnAfter(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	s := args[0]
	// C's after() trims leading delimiters first (trim_space_sep)
	s = strings.TrimLeft(s, delim)
	idx := strings.Index(s, delim)
	if idx < 0 { return }
	buf.WriteString(s[idx+len(delim):])
}

func fnReverse(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	runes := []rune(args[0])
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	buf.WriteString(string(runes))
}

func fnScramble(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	runes := []rune(args[0])
	rand.Shuffle(len(runes), func(i, j int) { runes[i], runes[j] = runes[j], runes[i] })
	buf.WriteString(string(runes))
}

func fnStrmatch(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	buf.WriteString(boolToStr(wildMatch(args[1], args[0])))
}

func fnComp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	cmp := strings.Compare(args[0], args[1])
	writeInt(buf, cmp)
}

func fnStreq(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	buf.WriteString(boolToStr(strings.EqualFold(args[0], args[1])))
}

func fnMatch(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 2 { delim = args[2] }
	words := splitList(args[0], delim)
	pattern := args[1]
	for i, w := range words {
		if wildMatch(pattern, w) {
			writeInt(buf, i+1)
			return
		}
	}
	buf.WriteString("0")
}

func fnMatchall(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 { delim = args[2] }
	outDelim := " "
	if len(args) > 3 { outDelim = args[3] }
	words := splitList(args[0], delim)
	pattern := args[1]
	var results []string
	for i, w := range words {
		if wildMatch(pattern, w) {
			results = append(results, strconv.Itoa(i+1))
		}
	}
	buf.WriteString(strings.Join(results, outDelim))
}

func fnDelete(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	s := args[0]
	start := toInt(args[1])
	length := toInt(args[2])
	if start < 0 { start = 0 }
	if start >= len(s) { buf.WriteString(s); return }
	end := start + length
	if end > len(s) { end = len(s) }
	buf.WriteString(s[:start])
	buf.WriteString(s[end:])
}

func fnChomp(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	buf.WriteString(strings.TrimRight(args[0], "\r\n"))
}

func fnTranslate(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	mode := "0"
	if len(args) > 1 { mode = args[1] }
	if mode == "1" {
		// Substitute returns for spaces
		buf.WriteString(strings.ReplaceAll(s, "\r\n", " "))
	} else {
		buf.WriteString(s)
	}
}

// Type checking
func fnIsnum(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	s := strings.TrimSpace(args[0])
	_, err := strconv.ParseFloat(s, 64)
	buf.WriteString(boolToStr(err == nil && s != ""))
}

func fnIsdbref(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	s := strings.TrimSpace(args[0])
	if len(s) < 2 || s[0] != '#' { buf.WriteString("0"); return }
	n, err := strconv.Atoi(s[1:])
	buf.WriteString(boolToStr(err == nil && n >= 0))
}

func fnIsword(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || args[0] == "" {
		// C returns 1 for empty/no-arg isword()
		buf.WriteString("1")
		return
	}
	s := strings.TrimSpace(args[0])
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
			buf.WriteString("0")
			return
		}
	}
	buf.WriteString("1")
}

// --- Spellcheck functions ---

// fnSpell implements spell(<text>[,g]) - highlights misspelled (and optionally grammar) issues.
// With "g" as second arg, also checks grammar via remote API.
func fnSpell(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.GameState == nil {
		return
	}
	grammar := len(args) > 1 && strings.EqualFold(strings.TrimSpace(args[1]), "g")
	buf.WriteString(ctx.GameState.SpellHighlight(player, args[0], grammar))
}

// fnSpellcheck implements spellcheck(<text>[,g]) - returns space-separated list of issues.
// With "g" as second arg, also returns grammar issues.
func fnSpellcheck(ctx *eval.EvalContext, args []string, buf *strings.Builder, player, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.GameState == nil {
		return
	}
	grammar := len(args) > 1 && strings.EqualFold(strings.TrimSpace(args[1]), "g")
	words := ctx.GameState.SpellCheck(player, args[0], grammar)
	buf.WriteString(strings.Join(words, " "))
}

// --- Additional string functions ---

// fnArt returns the appropriate English article (a/an) for a word.
func fnArt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || args[0] == "" { return }
	first := strings.ToLower(args[0][:1])
	switch first {
	case "a", "e", "i", "o", "u":
		buf.WriteString("an")
	default:
		buf.WriteString("a")
	}
}

// fnNescape escapes characters but NOT the first character (unlike escape()).
func fnNescape(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	for i, ch := range s {
		if i > 0 && (ch == '%' || ch == '\\' || ch == '[' || ch == ']' || ch == '{' || ch == '}' || ch == ';') {
			buf.WriteByte('\\')
		}
		buf.WriteRune(ch)
	}
}

// fnNsecure is like secure() but doesn't escape the first character.
func fnNsecure(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	for i, ch := range args[0] {
		if i > 0 {
			switch ch {
			case '%', '$', '\\', '[', ']', '(', ')', '{', '}', ',', ';':
				buf.WriteByte('\\')
			}
		}
		buf.WriteRune(ch)
	}
}

// fnWordpos returns the word number containing a given character position.
func fnWordpos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("#-1"); return }
	s := args[0]
	charPos := toInt(args[1])
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	if charPos < 0 || charPos >= len(s) {
		buf.WriteString("#-1")
		return
	}
	words := splitList(s, delim)
	pos := 0
	for i, w := range words {
		end := pos + len(w)
		if charPos >= pos && charPos < end {
			writeInt(buf, i+1)
			return
		}
		pos = end + len(delim)
	}
	buf.WriteString("#-1")
}

// fnIndex extracts a range of delimited fields.
// index(<string>, <delimiter>, <first>, <length>)
func fnIndex(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 4 { return }
	s := args[0]
	delim := args[1]
	if delim == "" { delim = " " }
	first := toInt(args[2]) // 1-indexed
	length := toInt(args[3])
	// C rejects start < 1 or length < 1 or empty string
	if first < 1 || length < 1 || s == "" { return }
	idx := first - 1 // convert to 0-based
	words := splitList(s, delim)
	end := idx + length
	if end > len(words) { end = len(words) }
	if idx >= len(words) { return }
	buf.WriteString(strings.Join(words[idx:end], delim))
}

// fnEncrypt performs C TinyMUSH's encrypt() function.
func fnEncrypt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	cryptHelper(args[0], args[1], true, buf)
}

// fnDecrypt performs C TinyMUSH's decrypt() function.
func fnDecrypt(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	cryptHelper(args[0], args[1], false, buf)
}

// cryptHelper implements C TinyMUSH's crypt_code / encrypt_code / decrypt_code.
// CRYPTCODE_LO=32, CRYPTCODE_HI=126, CRYPTCODE_MOD=95.
// Key is first "crunched" (non-printable ASCII stripped), then each printable
// character in text is shifted modularly within the 32-126 range.
func cryptHelper(text, key string, encrypt bool, buf *strings.Builder) {
	if key == "" {
		buf.WriteString(text)
		return
	}
	// crunch_code: keep only printable ASCII (32-126) from key
	var crunched []byte
	for i := 0; i < len(key); i++ {
		if key[i] >= 32 && key[i] <= 126 {
			crunched = append(crunched, key[i])
		}
	}
	if len(crunched) == 0 {
		buf.WriteString(text)
		return
	}

	ki := 0
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch >= 32 && ch <= 126 {
			if encrypt {
				ch = byte(((int(ch) - 32) + (int(crunched[ki]) - 32)) % 95 + 32)
			} else {
				ch = byte(((int(ch) - int(crunched[ki])) + 2*95) % 95 + 32)
			}
			ki++
			if ki >= len(crunched) {
				ki = 0
			}
		}
		buf.WriteByte(ch)
	}
}

// fnHtmlEscape escapes HTML special characters.
func fnHtmlEscape(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	for _, ch := range args[0] {
		switch ch {
		case '<': buf.WriteString("&lt;")
		case '>': buf.WriteString("&gt;")
		case '&': buf.WriteString("&amp;")
		case '"': buf.WriteString("&quot;")
		default:  buf.WriteRune(ch)
		}
	}
}

// fnHtmlUnescape unescapes HTML entities.
func fnHtmlUnescape(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	buf.WriteString(s)
}

// fnUrlEscape percent-encodes a URL string.
func fnUrlEscape(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	for _, ch := range args[0] {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' || ch == '~' {
			buf.WriteRune(ch)
		} else {
			buf.WriteString(fmt.Sprintf("%%%02X", ch))
		}
	}
}

// fnUrlUnescape decodes percent-encoded URL strings.
func fnUrlUnescape(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	s := args[0]
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			if hi >= 0 && lo >= 0 {
				buf.WriteByte(byte(hi*16 + lo))
				i += 2
				continue
			}
		}
		buf.WriteByte(s[i])
	}
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9': return int(c - '0')
	case c >= 'A' && c <= 'F': return int(c - 'A' + 10)
	case c >= 'a' && c <= 'f': return int(c - 'a' + 10)
	default: return -1
	}
}

// fnAnsipos returns the position of the Nth ANSI-visible character.
func fnAnsipos(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("#-1"); return }
	s := args[0]
	pos := toInt(args[1])
	visible := 0
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' { inEsc = false }
			continue
		}
		if visible == pos {
			writeInt(buf, i)
			return
		}
		visible++
	}
	buf.WriteString("#-1")
}

// fnSpeak formats speech/pose text with speaker prefix.
// speak(<speaker>, <string>[, <say-string>[, <transform>[, <empty>]]])
func fnSpeak(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	speaker := args[0]
	text := args[1]
	sayStr := "says,"
	if len(args) > 2 && args[2] != "" { sayStr = args[2] }

	if len(text) == 0 {
		if len(args) > 4 { buf.WriteString(args[4]) }
		return
	}

	switch text[0] {
	case ':':
		if len(text) > 1 && text[1] == ':' {
			// ::pose (nospace)
			buf.WriteString(speaker)
			buf.WriteString(text[2:])
		} else {
			buf.WriteString(speaker)
			buf.WriteByte(' ')
			buf.WriteString(text[1:])
		}
	case ';':
		buf.WriteString(speaker)
		buf.WriteString(text[1:])
	case '"':
		buf.WriteString(speaker)
		buf.WriteByte(' ')
		buf.WriteString(sayStr)
		buf.WriteString(" \"")
		buf.WriteString(text[1:])
		buf.WriteByte('"')
	default:
		buf.WriteString(speaker)
		buf.WriteByte(' ')
		buf.WriteString(sayStr)
		buf.WriteString(" \"")
		buf.WriteString(text)
		buf.WriteByte('"')
	}
}

// --- Border functions ---

// fnBorder — left-justified bordered text.
// border(text, width[, fill])
func fnBorder(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	borderHelper(args, buf, "left")
}

// fnCborder — center-justified bordered text.
func fnCborder(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	borderHelper(args, buf, "center")
}

// fnRborder — right-justified bordered text.
func fnRborder(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	borderHelper(args, buf, "right")
}

func borderHelper(args []string, buf *strings.Builder, align string) {
	if len(args) < 2 { return }
	text := args[0]
	width := toInt(args[1])
	if width <= 0 { width = 78 }
	// Optional prefix character (3rd arg) — C prepends a single char, not fill
	if len(args) > 2 && args[2] != "" {
		buf.WriteString(args[2])
	}
	// C behavior: border = ljust, cborder = center, rborder = rjust with spaces
	textLen := len(stripAnsiStr(text))
	remaining := width - textLen
	if remaining < 0 { remaining = 0 }
	switch align {
	case "center":
		leftPad := (remaining + 1) / 2 // C puts extra space on left when odd
		rightPad := remaining - leftPad
		buf.WriteString(strings.Repeat(" ", leftPad))
		buf.WriteString(text)
		buf.WriteString(strings.Repeat(" ", rightPad))
	case "right":
		buf.WriteString(strings.Repeat(" ", remaining))
		buf.WriteString(text)
	default: // left
		buf.WriteString(text)
		buf.WriteString(strings.Repeat(" ", remaining))
	}
}

// fnWildmatch performs wildcard matching with register capture.
// wildmatch(<string>, <pattern>, <register list>)
// C signature: wildmatch(string, pattern, registers) — note arg order differs from wildMatch helper.
// Returns 1 for match, 0 for no match. Captures wildcards into named registers.
func fnWildmatch(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("#-1 FUNCTION (WILDMATCH) EXPECTS 3 ARGUMENTS BUT GOT " + strconv.Itoa(len(args))); return }
	str := args[0]
	pattern := args[1]
	regList := args[2]

	captures := wildMatchCapture(pattern, str)
	if captures == nil {
		buf.WriteString("0")
		return
	}
	buf.WriteString("1")

	// Set captured values into named registers
	if ctx.RData != nil {
		regs := strings.Fields(regList)
		for i, reg := range regs {
			if i < len(captures) {
				regName := strings.TrimSpace(reg)
				if len(regName) == 1 {
					idx := qidxChar(regName[0])
					if idx >= 0 && idx < eval.MaxGlobalRegs {
						ctx.RData.QRegs[idx] = captures[i]
					}
				} else {
					ctx.RData.XRegs[strings.ToLower(regName)] = captures[i]
				}
			}
		}
	}
}

// wildMatch performs glob-style pattern matching (*, ?).
func wildMatch(pattern, str string) bool {
	pattern = strings.ToLower(pattern)
	str = strings.ToLower(str)
	return matchHelper(pattern, str)
}

func matchHelper(pattern, str string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Try matching rest of pattern with every suffix of str
			for i := len(str); i >= 0; i-- {
				if matchHelper(pattern[1:], str[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(str) == 0 { return false }
			pattern = pattern[1:]
			str = str[1:]
		default:
			if len(str) == 0 || pattern[0] != str[0] { return false }
			pattern = pattern[1:]
			str = str[1:]
		}
	}
	return len(str) == 0
}

// wildMatchCapture and captureHelper are defined in misc.go

// --- New string functions from RhostMUSH audit ---

// fnPrintf — formatted string output.
// printf(format, arg1, arg2, ...) — supports %s (string), %d (integer), %f (float), %% (literal %).
func fnPrintf(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	format := args[0]
	argIdx := 1
	i := 0
	for i < len(format) {
		if format[i] == '%' && i+1 < len(format) {
			i++
			switch format[i] {
			case 's':
				if argIdx < len(args) {
					buf.WriteString(args[argIdx])
					argIdx++
				}
			case 'd':
				if argIdx < len(args) {
					buf.WriteString(strconv.Itoa(toInt(args[argIdx])))
					argIdx++
				}
			case 'f':
				if argIdx < len(args) {
					f, _ := strconv.ParseFloat(strings.TrimSpace(args[argIdx]), 64)
					buf.WriteString(fmt.Sprintf("%g", f))
					argIdx++
				}
			case '%':
				buf.WriteByte('%')
			default:
				buf.WriteByte('%')
				buf.WriteByte(format[i])
			}
		} else {
			buf.WriteByte(format[i])
		}
		i++
	}
}

// fnTr — transliterate characters, like Unix tr.
// tr(string, from, to) — each char in 'from' is replaced by corresponding char in 'to'.
func fnTr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	s := args[0]
	from := args[1]
	to := args[2]
	for _, ch := range s {
		idx := strings.IndexRune(from, ch)
		if idx >= 0 && idx < len(to) {
			buf.WriteByte(to[idx])
		} else {
			buf.WriteRune(ch)
		}
	}
}

// fnStrdistance — Levenshtein edit distance between two strings.
func fnStrdistance(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	a := strings.ToLower(args[0])
	b := strings.ToLower(args[1])
	la, lb := len(a), len(b)
	if la == 0 { writeInt(buf, lb); return }
	if lb == 0 { writeInt(buf, la); return }

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ { prev[j] = j }

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] { cost = 0 }
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m { m = ins }
			if sub < m { m = sub }
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	writeInt(buf, prev[lb])
}

// fnStrlenvis — visual length of a string (stripping ANSI escape codes).
func fnStrlenvis(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { writeInt(buf, 0); return }
	writeInt(buf, len(stripAnsiStr(args[0])))
}

// Character class testing functions (from RhostMUSH)

func fnIsalnum(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

func fnIsalpha(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

func fnIsdigit(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if ch < '0' || ch > '9' {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

func fnIsupper(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if ch < 'A' || ch > 'Z' {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

func fnIslower(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if ch < 'a' || ch > 'z' {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

func fnIsspace(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

func fnIspunct(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("0"); return }
	for _, ch := range args[0] {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == ' ' {
			buf.WriteString("0"); return
		}
	}
	buf.WriteString("1")
}

// --- Encoding/hashing functions ---

// fnEncode64 — base64 encode a string.
func fnEncode64(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(args[0])))
}

// fnDecode64 — base64 decode a string.
func fnDecode64(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	decoded, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		buf.WriteString("#-1 INVALID BASE64")
		return
	}
	buf.Write(decoded)
}

// fnDigest — hash a string using SHA256 (default) or specified algorithm.
// digest(string[, algorithm]) — supports sha256, sha1, md5, sha512.
func fnDigest(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	// digest(algorithm, text) — C TinyMUSH convention: algo first, text second
	algo := strings.ToLower(strings.TrimSpace(args[0]))
	text := args[1]
	var h hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New()
	case "sha1":
		h = sha1.New()
	case "md5":
		h = md5.New()
	case "sha512":
		h = sha512.New()
	default:
		buf.WriteString("#-1 UNKNOWN ALGORITHM")
		return
	}
	h.Write([]byte(text))
	buf.WriteString(fmt.Sprintf("%x", h.Sum(nil)))
}

// fnHmac — compute HMAC of a message with a key.
// hmac(algorithm, key, message) — supports sha256, sha1, md5, sha512.
func fnHmac(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("#-1 FUNCTION EXPECTS 3 ARGUMENTS"); return }
	algo := strings.ToLower(strings.TrimSpace(args[0]))
	key := args[1]
	message := args[2]
	var h func() hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New
	case "sha1":
		h = sha1.New
	case "md5":
		h = md5.New
	case "sha512":
		h = sha512.New
	default:
		buf.WriteString("#-1 UNKNOWN ALGORITHM")
		return
	}
	mac := hmac.New(h, []byte(key))
	mac.Write([]byte(message))
	buf.WriteString(fmt.Sprintf("%x", mac.Sum(nil)))
}

// fnCrc32 — CRC32 checksum of a string.
func fnCrc32(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	csum := crc32.ChecksumIEEE([]byte(args[0]))
	buf.WriteString(strconv.FormatUint(uint64(csum), 10))
}

// --- RhostMUSH extension string functions ---

// fnAsc — return ASCII value of the first character.
// asc(string) → integer
func fnAsc(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { buf.WriteString("-1"); return }
	buf.WriteString(strconv.Itoa(int(args[0][0])))
}

// fnChr — return character for an ASCII code.
// chr(code) → character
func fnChr(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	code := toInt(args[0])
	if code < 32 || code > 126 {
		buf.WriteString("#-1 ARGUMENT OUT OF RANGE")
		return
	}
	buf.WriteByte(byte(code))
}

// fnStrip — remove specified characters from a string.
// strip(string, chars) → string with those characters removed
func fnStrip(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { if len(args) > 0 { buf.WriteString(args[0]) }; return }
	str := args[0]
	chars := args[1]
	for _, c := range str {
		if !strings.ContainsRune(chars, c) {
			buf.WriteRune(c)
		}
	}
}

// fnCaplist — capitalize each word in a list.
// caplist(list[, delim]) → list with each word capitalized
func fnCaplist(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	for i, w := range words {
		if i > 0 { buf.WriteString(delim) }
		if len(w) > 0 {
			buf.WriteString(strings.ToUpper(w[:1]))
			if len(w) > 1 { buf.WriteString(strings.ToLower(w[1:])) }
		}
	}
}

// fnSpellnum — convert an integer to English words.
// spellnum(number) → "forty-two"
func fnSpellnum(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	n := toInt(args[0])
	buf.WriteString(intToEnglish(n))
}

var onesWords = [20]string{
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
	"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen",
}
var tensWords = [10]string{
	"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety",
}

func intToEnglish(n int) string {
	if n < 0 { return "negative " + intToEnglish(-n) }
	if n < 20 { return onesWords[n] }
	if n < 100 {
		s := tensWords[n/10]
		if n%10 != 0 { s += "-" + onesWords[n%10] }
		return s
	}
	if n < 1000 {
		s := onesWords[n/100] + " hundred"
		if n%100 != 0 { s += " " + intToEnglish(n%100) }
		return s
	}
	type mag struct { val int; name string }
	mags := []mag{{1000000000, "billion"}, {1000000, "million"}, {1000, "thousand"}}
	for _, m := range mags {
		if n >= m.val {
			s := intToEnglish(n/m.val) + " " + m.name
			if n%m.val != 0 { s += " " + intToEnglish(n%m.val) }
			return s
		}
	}
	return strconv.Itoa(n)
}

// fnSoundex — return the Soundex code of a word.
// soundex(word) → 4-character Soundex code
func fnSoundex(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || len(args[0]) == 0 { return }
	buf.WriteString(soundexCode(args[0]))
}

// fnSoundlike — check if two words have the same Soundex code.
// soundlike(word1, word2) → 1 or 0
func fnSoundlike(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	if soundexCode(args[0]) == soundexCode(args[1]) {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

func soundexCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) == 0 { return "" }
	// Skip non-alpha prefix
	start := 0
	for start < len(s) && (s[start] < 'A' || s[start] > 'Z') { start++ }
	if start >= len(s) { return "" }
	s = s[start:]

	// Soundex coding table
	codes := map[byte]byte{
		'B': '1', 'F': '1', 'P': '1', 'V': '1',
		'C': '2', 'G': '2', 'J': '2', 'K': '2', 'Q': '2', 'S': '2', 'X': '2', 'Z': '2',
		'D': '3', 'T': '3',
		'L': '4',
		'M': '5', 'N': '5',
		'R': '6',
	}

	result := []byte{s[0]}
	lastCode := codes[s[0]]
	for i := 1; i < len(s) && len(result) < 4; i++ {
		c := s[i]
		if c < 'A' || c > 'Z' { continue }
		code, ok := codes[c]
		if ok && code != lastCode {
			result = append(result, code)
			lastCode = code
		} else if !ok {
			lastCode = '0'
		}
	}
	for len(result) < 4 { result = append(result, '0') }
	return string(result)
}

// --- Column alignment ---

// colSpec describes a single column for align().
type colSpec struct {
	width int
	align byte // '<' left (default), '>' right, '-' center
	pad   byte // padding character (default ' ')
}

// parseColSpecs parses the align() spec string into column descriptors.
// Each token: leading digits = width, optional </>/- for alignment, optional pad char.
// Example: "20< 10> 15-." → [{20,'<',' '}, {10,'>','.'}, {15,'-',' '}]
func parseColSpecs(spec string) []colSpec {
	tokens := strings.Fields(spec)
	cols := make([]colSpec, 0, len(tokens))
	for _, tok := range tokens {
		var cs colSpec
		cs.align = '<'
		cs.pad = ' '
		// Parse width digits from start
		i := 0
		for i < len(tok) && tok[i] >= '0' && tok[i] <= '9' {
			i++
		}
		if i > 0 {
			cs.width, _ = strconv.Atoi(tok[:i])
		}
		// Parse alignment char
		if i < len(tok) {
			switch tok[i] {
			case '<', '>', '-':
				cs.align = tok[i]
				i++
			}
		}
		// Parse optional pad char
		if i < len(tok) {
			cs.pad = tok[i]
		}
		if cs.width > 0 {
			cols = append(cols, cs)
		}
	}
	return cols
}

// fnAlign formats columns with alignment.
// align(spec, col1[, col2...][, filler][, linesep][, colsep])
func fnAlign(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}

	cols := parseColSpecs(args[0])
	if len(cols) == 0 {
		return
	}

	n := len(cols) // number of columns
	// Column values are args[1] through args[n]
	colValues := make([]string, n)
	for i := 0; i < n && i+1 < len(args); i++ {
		colValues[i] = args[i+1]
	}

	// Optional trailing args after column values
	nextArg := n + 1
	// linesep (default \n)
	linesep := "\n"
	if nextArg+1 < len(args) && args[nextArg+1] != "" {
		linesep = args[nextArg+1]
	}
	// colsep (default empty)
	colsep := ""
	if nextArg+2 < len(args) && args[nextArg+2] != "" {
		colsep = args[nextArg+2]
	}

	// Split each column value on \n for multi-row support
	splitCols := make([][]string, n)
	maxRows := 0
	for i := 0; i < n; i++ {
		if colValues[i] != "" {
			splitCols[i] = strings.Split(colValues[i], "\n")
		} else {
			splitCols[i] = []string{""}
		}
		if len(splitCols[i]) > maxRows {
			maxRows = len(splitCols[i])
		}
	}
	if maxRows == 0 {
		maxRows = 1
	}

	// Render each row
	for row := 0; row < maxRows; row++ {
		if row > 0 {
			buf.WriteString(linesep)
		}
		for col := 0; col < n; col++ {
			if col > 0 {
				buf.WriteString(colsep)
			}
			cell := ""
			if row < len(splitCols[col]) {
				cell = splitCols[col][row]
			}
			cs := cols[col]
			vLen := ansiStrLen(cell)
			padChar := string(cs.pad)

			if vLen > cs.width {
				// Truncate
				buf.WriteString(ansiTruncate(cell, cs.width))
			} else {
				padNeeded := cs.width - vLen
				switch cs.align {
				case '>': // right
					for i := 0; i < padNeeded; i++ {
						buf.WriteString(padChar)
					}
					buf.WriteString(cell)
				case '-': // center
					leftPad := padNeeded / 2
					rightPad := padNeeded - leftPad
					for i := 0; i < leftPad; i++ {
						buf.WriteString(padChar)
					}
					buf.WriteString(cell)
					for i := 0; i < rightPad; i++ {
						buf.WriteString(padChar)
					}
				default: // '<' left
					buf.WriteString(cell)
					for i := 0; i < padNeeded; i++ {
						buf.WriteString(padChar)
					}
				}
			}
		}
	}
}

// fnGarble — garble/corrupt text with random character substitution.
// garble(text[, percent]) → garbled text
// percent defaults to 50. Each character has that % chance of being replaced.
func fnGarble(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	text := args[0]
	pct := 50
	if len(args) > 1 { pct = toInt(args[1]) }
	if pct < 0 { pct = 0 }
	if pct > 100 { pct = 100 }
	for _, c := range text {
		if c == ' ' || rand.IntN(100) >= pct {
			buf.WriteRune(c)
		} else {
			// Replace with random printable ASCII
			buf.WriteByte(byte(rand.IntN(94) + 33))
		}
	}
}
