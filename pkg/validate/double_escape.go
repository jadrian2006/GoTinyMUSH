package validate

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// DoubleEscapeChecker detects \\[...\\] patterns inside function arguments
// that produce \text\ instead of the intended [text].
// These occur when softcode authors added extra escaping for bracket-containing
// text inside function calls like ansi(c,\\[Monitor\\]).
// The fix reduces \\[ to \[ so the eval engine outputs literal brackets.
type DoubleEscapeChecker struct{}

func (c *DoubleEscapeChecker) Name() string { return "double-escape" }

// knownFunctions are softcode functions whose arguments commonly contain
// bracket-escaped text where \\[ was used to produce literal brackets.
var knownFunctions = map[string]bool{
	"ansi": true, "center": true, "ljust": true, "rjust": true,
	"printf": true, "pemit": true, "remit": true, "oemit": true,
	"u": true, "ulocal": true, "udefault": true, "ufun": true,
	"switch": true, "switchall": true, "ifelse": true, "if": true,
	"iter": true, "list": true, "parse": true, "map": true,
	"fold": true, "filter": true, "step": true, "foreach": true,
	"cat": true, "strcat": true, "set": true, "trigger": true,
	"tel": true, "cemit": true, "say": true, "think": true,
	"whisper": true, "page": true, "emit": true,
	"default": true, "edefault": true, "objeval": true,
	"setq": true, "setr": true, "ldelete": true, "replace": true,
	"edit": true, "mail": true, "create": true, "open": true,
}

// Check scans all object attributes for double-escaped bracket patterns
// that appear inside known function arguments.
func (c *DoubleEscapeChecker) Check(db *gamedb.Database) []Finding {
	var findings []Finding
	seq := 0

	for _, obj := range db.Objects {
		if obj.IsGoing() {
			continue
		}
		for attrIdx, attr := range obj.Attrs {
			if attr.Value == "" {
				continue
			}

			// Skip attrs with AFRegexp flag (regex patterns may legitimately have \\[)
			if def, ok := db.AttrNames[attr.Number]; ok && def.Flags&gamedb.AFRegexp != 0 {
				continue
			}

			_, text := splitAttrPrefix(attr.Value)
			matches := findDoubleEscapesInFuncArgs(text)
			if len(matches) == 0 {
				continue
			}

			proposed := applyDoubleEscapeFixes(text, matches)
			attrName := db.GetAttrName(attr.Number)
			if attrName == "" {
				attrName = fmt.Sprintf("A_%d", attr.Number)
			}

			id := fmt.Sprintf("obj%d-attr%d-de%d", obj.DBRef, attr.Number, seq)
			seq++

			// Build a human-readable effect description and highlight ranges
			var effectParts []string
			var currentHL, proposedHL []Highlight
			offset := 0 // tracks cumulative shift between current and proposed positions
			for _, m := range matches {
				old := text[m.start:m.end]
				fixed := fixSpan(old)
				effectParts = append(effectParts, fmt.Sprintf("%q → %q", old, fixed))
				currentHL = append(currentHL, Highlight{Start: m.start, End: m.end})
				pStart := m.start + offset
				pEnd := pStart + len(fixed)
				proposedHL = append(proposedHL, Highlight{Start: pStart, End: pEnd})
				offset += len(fixed) - len(old)
			}

			// Window the text around the highlighted areas
			const displayMax = 300
			currentText, currentHLAdj := windowAroundHighlights(text, currentHL, displayMax)
			proposedText, proposedHLAdj := windowAroundHighlights(proposed, proposedHL, displayMax)

			// Capture for fix closure
			capturedObj := obj
			capturedIdx := attrIdx
			capturedProposed := proposed

			f := Finding{
				ID:          id,
				Category:    CatDoubleEscape,
				Severity:    SevWarning,
				ObjectRef:   obj.DBRef,
				AttrNum:     attr.Number,
				AttrName:    attrName,
				OwnerRef:    obj.Owner,
				Description: fmt.Sprintf("Double-escaped brackets in %s on #%d (%s)", attrName, obj.DBRef, truncate(obj.Name, 30)),
				Current:     currentText,
				Proposed:    proposedText,
				CurrentHL:   currentHLAdj,
				ProposedHL:  proposedHLAdj,
				Effect:      strings.Join(effectParts, "; "),
				Explanation: `The \\[text\\] pattern inside a function argument like ansi(c,\\[Monitor\\]) causes the eval engine to output \Monitor\ instead of [Monitor]. The extra backslash makes \\ evaluate to a literal \, then [ starts a bracket expression instead of being a literal character. The fix reduces \\[ to \[ so the eval engine treats the bracket as literal text.`,
				Fixable:     true,
				fixFunc: func() {
					prefix, _ := splitAttrPrefix(capturedObj.Attrs[capturedIdx].Value)
					capturedObj.Attrs[capturedIdx].Value = prefix + capturedProposed
				},
			}
			findings = append(findings, f)
		}
	}
	return findings
}

// escapeMatch represents a \\[...\\] span found in text.
type escapeMatch struct {
	start int // index of first backslash in \\[
	end   int // index after the ] in \\]
}

// findDoubleEscapesInFuncArgs locates \\[...\\] patterns that appear inside
// known function arguments. Only flags patterns where a known function name
// precedes the parenthesized argument containing the double-escaped brackets.
func findDoubleEscapesInFuncArgs(text string) []escapeMatch {
	var matches []escapeMatch

	// Find all \\[...\\] pairs first
	allPairs := findAllDoubleEscapePairs(text)
	if len(allPairs) == 0 {
		return nil
	}

	// For each pair, check if it's inside a known function's argument list
	for _, pair := range allPairs {
		if isInsideFuncArg(text, pair.start) {
			matches = append(matches, pair)
		}
	}

	return matches
}

// findAllDoubleEscapePairs finds all \\[...\\] paired patterns in text.
func findAllDoubleEscapePairs(text string) []escapeMatch {
	var matches []escapeMatch
	i := 0
	for i < len(text)-3 {
		// Look for \\[ pattern (two backslash chars followed by [)
		if text[i] == '\\' && i+2 < len(text) && text[i+1] == '\\' && text[i+2] == '[' {
			bracketStart := i
			j := i + 3
			depth := 1
			for j < len(text)-1 && depth > 0 {
				if text[j] == '\\' && j+1 < len(text) && text[j+1] == '\\' && j+2 < len(text) {
					if text[j+2] == '[' {
						depth++
						j += 3
						continue
					} else if text[j+2] == ']' {
						depth--
						if depth == 0 {
							matches = append(matches, escapeMatch{
								start: bracketStart,
								end:   j + 3,
							})
							i = j + 3
							break
						}
						j += 3
						continue
					}
				}
				j++
			}
			if depth > 0 {
				i++
			}
		} else {
			i++
		}
	}
	return matches
}

// isInsideFuncArg checks whether position pos in text falls inside
// the argument list of a known function call like funcname(...).
// Scans backwards from pos to find the nearest unmatched '(' and checks
// if a known function name precedes it.
func isInsideFuncArg(text string, pos int) bool {
	// Scan backwards from pos, tracking paren depth
	depth := 0
	for i := pos - 1; i >= 0; i-- {
		switch text[i] {
		case ')':
			depth++
		case '(':
			if depth > 0 {
				depth--
			} else {
				// Found an unmatched '(' — check for function name before it
				funcName := extractFuncNameBefore(text, i)
				if funcName != "" && knownFunctions[strings.ToLower(funcName)] {
					return true
				}
				// Not a known function, but could be nested; keep scanning
			}
		}
	}
	return false
}

// extractFuncNameBefore extracts the function name immediately before position
// parenPos (which points to '('). Returns empty string if no valid name found.
func extractFuncNameBefore(text string, parenPos int) string {
	end := parenPos
	i := end - 1
	// Skip trailing whitespace
	for i >= 0 && text[i] == ' ' {
		i--
	}
	if i < 0 {
		return ""
	}
	nameEnd := i + 1
	// Collect alphanumeric/underscore chars (function name)
	for i >= 0 && (unicode.IsLetter(rune(text[i])) || unicode.IsDigit(rune(text[i])) || text[i] == '_') {
		i--
	}
	nameStart := i + 1
	if nameStart >= nameEnd {
		return ""
	}
	return text[nameStart:nameEnd]
}

// applyDoubleEscapeFixes applies all fixes to the text.
func applyDoubleEscapeFixes(text string, matches []escapeMatch) string {
	if len(matches) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for _, m := range matches {
		b.WriteString(text[last:m.start])
		b.WriteString(fixSpan(text[m.start:m.end]))
		last = m.end
	}
	b.WriteString(text[last:])
	return b.String()
}

// fixSpan converts \\[text\\] to \[text\] within a matched span.
// Removes one backslash so \[ is treated as a literal bracket by the eval engine.
func fixSpan(span string) string {
	var b strings.Builder
	b.Grow(len(span))
	i := 0
	for i < len(span) {
		if i+2 < len(span) && span[i] == '\\' && span[i+1] == '\\' {
			if span[i+2] == '[' || span[i+2] == ']' {
				b.WriteByte('\\')      // keep one backslash
				b.WriteByte(span[i+2]) // then the bracket
				i += 3
				continue
			}
		}
		b.WriteByte(span[i])
		i++
	}
	return b.String()
}
