package server

import (
	"fmt"
	"strings"
	"unicode"
)

// ANSI color constants for softcode pretty-printing
const (
	ansiReset   = "\033[0m"
	ansiCommand = "\033[1;36m" // bold cyan — @commands
	ansiFunc    = "\033[33m"   // yellow — functions
	ansiSub     = "\033[32m"   // green — substitutions (%#, %0, ##)
	ansiTrigger = "\033[35m"   // magenta — $pattern trigger
	ansiBrace   = "\033[2;37m" // dim white — { }
	ansiBracket = "\033[2;37m" // dim white — [ ]
	ansiComma   = "\033[2;37m" // dim white — structural commas
	ansiError   = "\033[1;31m" // bold red — brace errors
)

// scTokenKind classifies softcode tokens for syntax highlighting.
type scTokenKind int

const (
	scLiteral scTokenKind = iota
	scCommand             // @switch, @pemit, @dolist, etc.
	scFunc                // name(
	scSub                 // %#, %0, %va, ##, #@, #+
	scTrigger             // $pattern: prefix
	scBraceOpen           // {
	scBraceClose          // }
	scBracketOpen         // [
	scBracketClose        // ]
	scComma               // , (structural, at brace level)
	scEquals              // = (after @command)
)

type scToken struct {
	Kind scTokenKind
	Text string
}

// PrettyFormatSoftcode formats a softcode attribute value with ANSI colors
// and structural indentation. Returns the formatted lines.
func PrettyFormatSoftcode(attrName, value string) []string {
	// Check for $command or ^listen trigger prefix
	triggerPrefix := ""
	body := value
	if len(value) > 0 && (value[0] == '$' || value[0] == '^') {
		if colonIdx := scFindTriggerColon(value); colonIdx >= 0 {
			triggerPrefix = value[:colonIdx+1]
			body = value[colonIdx+1:]
		}
	}

	tokens := scTokenize(body)
	lines := scFormatTokens(tokens, triggerPrefix)

	// Prepend attribute name
	if len(lines) > 0 {
		lines[0] = attrName + ": " + lines[0]
	}

	return lines
}

// ValidateBraces checks for unmatched braces, brackets, and parens in softcode.
// Returns a list of error descriptions, or empty if balanced.
func ValidateBraces(value string) []string {
	var errors []string
	type stackEntry struct {
		char byte
		pos  int
	}
	var stack []stackEntry
	inEscape := false

	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			inEscape = false
			continue
		}
		if ch == '\\' {
			inEscape = true
			continue
		}

		switch ch {
		case '{':
			stack = append(stack, stackEntry{'{', i})
		case '}':
			if len(stack) > 0 && stack[len(stack)-1].char == '{' {
				stack = stack[:len(stack)-1]
			} else {
				errors = append(errors, fmt.Sprintf("Unmatched '}' at position %d", i+1))
			}
		case '[':
			stack = append(stack, stackEntry{'[', i})
		case ']':
			if len(stack) > 0 && stack[len(stack)-1].char == '[' {
				stack = stack[:len(stack)-1]
			} else {
				errors = append(errors, fmt.Sprintf("Unmatched ']' at position %d", i+1))
			}
		case '(':
			stack = append(stack, stackEntry{'(', i})
		case ')':
			if len(stack) > 0 && stack[len(stack)-1].char == '(' {
				stack = stack[:len(stack)-1]
			} else {
				errors = append(errors, fmt.Sprintf("Unmatched ')' at position %d", i+1))
			}
		}
	}

	// Report unclosed openers
	for i := len(stack) - 1; i >= 0; i-- {
		item := stack[i]
		var closeChar string
		switch item.char {
		case '{':
			closeChar = "}"
		case '[':
			closeChar = "]"
		case '(':
			closeChar = ")"
		}
		errors = append(errors, fmt.Sprintf("Unclosed '%c' at position %d (missing '%s')", item.char, item.pos+1, closeChar))
	}

	return errors
}

// scFindTriggerColon finds the colon separating a $pattern or ^listen from the body.
// Returns -1 if not found. Respects nested brackets and parens.
func scFindTriggerColon(s string) int {
	depth := 0
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '[', '(':
			depth++
		case ']', ')':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// scTokenize splits softcode into typed tokens for highlighting.
func scTokenize(s string) []scToken {
	var tokens []scToken
	i := 0

	for i < len(s) {
		ch := s[i]

		switch {
		case ch == '{':
			tokens = append(tokens, scToken{scBraceOpen, "{"})
			i++
		case ch == '}':
			tokens = append(tokens, scToken{scBraceClose, "}"})
			i++
		case ch == '[':
			tokens = append(tokens, scToken{scBracketOpen, "["})
			i++
		case ch == ']':
			tokens = append(tokens, scToken{scBracketClose, "]"})
			i++
		case ch == ',':
			tokens = append(tokens, scToken{scComma, ","})
			i++
		case ch == '=' && len(tokens) > 0 && tokens[len(tokens)-1].Kind == scCommand:
			tokens = append(tokens, scToken{scEquals, "="})
			i++
		case ch == '%':
			tok, advance := scTokenizeSub(s, i)
			tokens = append(tokens, tok)
			i += advance
		case ch == '#' && i+1 < len(s) && (s[i+1] == '#' || s[i+1] == '@' || s[i+1] == '+' || s[i+1] == '$' || s[i+1] == '!'):
			tokens = append(tokens, scToken{scSub, s[i : i+2]})
			i += 2
		case ch == '@' && scIsCommandStart(s, i):
			tok, advance := scTokenizeCommand(s, i)
			tokens = append(tokens, tok)
			i += advance
		case scIsIdentStart(ch) && scIsFuncCall(s, i):
			tok, advance := scTokenizeFunc(s, i)
			tokens = append(tokens, tok)
			i += advance
		default:
			// Consume literal text up to next interesting char
			j := i + 1
			for j < len(s) && !scIsTokenStart(s, j) {
				j++
			}
			tokens = append(tokens, scToken{scLiteral, s[i:j]})
			i = j
		}
	}

	return tokens
}

// scTokenizeSub reads a %-substitution token.
func scTokenizeSub(s string, i int) (scToken, int) {
	if i+1 >= len(s) {
		return scToken{scLiteral, "%"}, 1
	}
	next := s[i+1]
	switch {
	case next == '%', next == '\\':
		return scToken{scLiteral, s[i : i+2]}, 2
	case next == '#', next == '!', next == 'n', next == 'N',
		next == 's', next == 'S', next == 'o', next == 'O',
		next == 'p', next == 'P', next == 'a', next == 'A',
		next == 'l', next == 'L', next == 'c', next == 'r',
		next == 't', next == 'b', next == 'R', next == 'T',
		next == 'B':
		return scToken{scSub, s[i : i+2]}, 2
	case next >= '0' && next <= '9':
		return scToken{scSub, s[i : i+2]}, 2
	case next == 'q' || next == 'Q':
		if i+2 < len(s) && scIsAlphaNum(s[i+2]) {
			return scToken{scSub, s[i : i+3]}, 3
		}
		return scToken{scSub, s[i : i+2]}, 2
	case next == 'v' || next == 'V':
		if i+2 < len(s) && scIsAlpha(s[i+2]) {
			return scToken{scSub, s[i : i+3]}, 3
		}
		return scToken{scSub, s[i : i+2]}, 2
	case next == 'x' || next == 'X':
		if i+2 < len(s) && scIsAlpha(s[i+2]) {
			return scToken{scSub, s[i : i+3]}, 3
		}
		return scToken{scSub, s[i : i+2]}, 2
	default:
		return scToken{scSub, s[i : i+2]}, 2
	}
}

// scIsCommandStart checks if @ at position i starts a command name.
func scIsCommandStart(s string, i int) bool {
	return i+1 < len(s) && scIsAlpha(s[i+1])
}

// scTokenizeCommand reads an @command token (e.g., @switch, @pemit).
func scTokenizeCommand(s string, i int) (scToken, int) {
	j := i + 1
	for j < len(s) && (scIsAlpha(s[j]) || s[j] == '_') {
		j++
	}
	// Include /switch suffixes
	for j < len(s) && s[j] == '/' {
		j++
		for j < len(s) && (scIsAlpha(s[j]) || s[j] == '_') {
			j++
		}
	}
	return scToken{scCommand, s[i:j]}, j - i
}

// scIsFuncCall checks if position i starts a function call (ident followed by '(').
func scIsFuncCall(s string, i int) bool {
	j := i
	for j < len(s) && (scIsAlphaNum(s[j]) || s[j] == '_') {
		j++
	}
	return j > i && j < len(s) && s[j] == '('
}

// scTokenizeFunc reads a function name including the opening paren.
func scTokenizeFunc(s string, i int) (scToken, int) {
	j := i
	for j < len(s) && (scIsAlphaNum(s[j]) || s[j] == '_') {
		j++
	}
	if j < len(s) && s[j] == '(' {
		j++
		return scToken{scFunc, s[i:j]}, j - i
	}
	return scToken{scLiteral, s[i:j]}, j - i
}

func scIsIdentStart(ch byte) bool { return scIsAlpha(ch) || ch == '_' }
func scIsAlpha(ch byte) bool      { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }
func scIsAlphaNum(ch byte) bool   { return scIsAlpha(ch) || (ch >= '0' && ch <= '9') }

func scIsTokenStart(s string, i int) bool {
	ch := s[i]
	switch ch {
	case '{', '}', '[', ']', ',', '%', '@':
		return true
	case '#':
		return i+1 < len(s) && (s[i+1] == '#' || s[i+1] == '@' || s[i+1] == '+' || s[i+1] == '$' || s[i+1] == '!')
	}
	if scIsIdentStart(ch) && scIsFuncCall(s, i) {
		return true
	}
	return false
}

// scFormatTokens converts a token stream into indented, colorized lines.
func scFormatTokens(tokens []scToken, triggerPrefix string) []string {
	var lines []string
	var cur strings.Builder
	indent := 0

	// Write trigger prefix on first line
	if triggerPrefix != "" {
		cur.WriteString(ansiTrigger)
		cur.WriteString(triggerPrefix)
		cur.WriteString(ansiReset)
	}

	writeIndent := func(b *strings.Builder, level int) {
		for i := 0; i < level; i++ {
			b.WriteString("  ")
		}
	}

	flushLine := func() {
		line := strings.TrimRightFunc(cur.String(), unicode.IsSpace)
		lines = append(lines, line)
		cur.Reset()
	}

	braceDepth := 0

	for ti, tok := range tokens {
		switch tok.Kind {
		case scTrigger:
			cur.WriteString(ansiTrigger)
			cur.WriteString(tok.Text)
			cur.WriteString(ansiReset)

		case scCommand:
			// Line break before @command if we're in a brace block and not at line start
			if cur.Len() > 0 && braceDepth > 0 && strings.TrimSpace(cur.String()) != "" {
				flushLine()
				writeIndent(&cur, indent)
			}
			cur.WriteString(ansiCommand)
			cur.WriteString(tok.Text)
			cur.WriteString(ansiReset)

		case scEquals:
			cur.WriteString(tok.Text)

		case scFunc:
			cur.WriteString(ansiFunc)
			cur.WriteString(tok.Text)
			cur.WriteString(ansiReset)

		case scSub:
			cur.WriteString(ansiSub)
			cur.WriteString(tok.Text)
			cur.WriteString(ansiReset)

		case scBraceOpen:
			braceDepth++
			cur.WriteString(ansiBrace)
			cur.WriteString("{")
			cur.WriteString(ansiReset)
			flushLine()
			indent++
			writeIndent(&cur, indent)

		case scBraceClose:
			braceDepth--
			if braceDepth < 0 {
				braceDepth = 0
			}
			indent--
			if indent < 0 {
				indent = 0
			}
			flushLine()
			writeIndent(&cur, indent)
			cur.WriteString(ansiBrace)
			cur.WriteString("}")
			cur.WriteString(ansiReset)

		case scBracketOpen:
			cur.WriteString(ansiBracket)
			cur.WriteString("[")
			cur.WriteString(ansiReset)

		case scBracketClose:
			cur.WriteString(ansiBracket)
			cur.WriteString("]")
			cur.WriteString(ansiReset)

		case scComma:
			cur.WriteString(ansiComma)
			cur.WriteString(",")
			cur.WriteString(ansiReset)
			// Break after comma if followed by a brace or @command (new case branch)
			if braceDepth > 0 && ti+1 < len(tokens) {
				next := tokens[ti+1]
				if next.Kind == scBraceOpen || next.Kind == scCommand {
					flushLine()
					writeIndent(&cur, indent)
				}
			}

		case scLiteral:
			text := tok.Text
			if strings.TrimSpace(cur.String()) == "" && strings.HasPrefix(text, " ") {
				text = strings.TrimLeft(text, " ")
			}
			cur.WriteString(text)
		}
	}

	// Flush remaining content
	if cur.Len() > 0 {
		flushLine()
	}

	return lines
}

// PrettyExamineAttr formats a single attribute for ex/pretty display.
// Returns colorized, indented lines plus any validation errors.
func PrettyExamineAttr(attrName, value string) ([]string, []string) {
	lines := PrettyFormatSoftcode(attrName, value)
	errors := ValidateBraces(value)
	return lines, errors
}
