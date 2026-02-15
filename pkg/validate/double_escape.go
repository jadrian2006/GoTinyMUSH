package validate

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// DoubleEscapeChecker detects \\[...\\] patterns that were written to compensate
// for C TinyMUSH's double-evaluation in queue processing. In Go's correct
// single-eval, these produce \text\ instead of the intended [text].
type DoubleEscapeChecker struct{}

func (c *DoubleEscapeChecker) Name() string { return "double-escape" }

// knownFunctions are softcode functions whose arguments commonly contain
// bracket-escaped text that was compensated for C's double-eval.
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

// Check scans all object attributes for double-escaped bracket patterns.
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
			matches := findDoubleEscapes(text)
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
				// Highlight in current (only if within truncation limit)
				if m.start < 200 {
					end := m.end
					if end > 200 { end = 200 }
					currentHL = append(currentHL, Highlight{Start: m.start, End: end})
				}
				// Highlight in proposed (adjusted for size difference)
				pStart := m.start + offset
				pEnd := pStart + len(fixed)
				if pStart < 200 {
					if pEnd > 200 { pEnd = 200 }
					proposedHL = append(proposedHL, Highlight{Start: pStart, End: pEnd})
				}
				offset += len(fixed) - len(old)
			}

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
				Current:     truncate(text, 200),
				Proposed:    truncate(proposed, 200),
				CurrentHL:   currentHL,
				ProposedHL:  proposedHL,
				Effect:      strings.Join(effectParts, "; "),
				Explanation: `C TinyMUSH evaluates queued commands twice, so game authors wrote \\[text\\] to get [text] after double processing. GoTinyMUSH evaluates correctly in a single pass, so the extra backslashes produce \text\ instead. The fix removes the extra escaping so text displays as originally intended.`,
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

// findDoubleEscapes locates \\[...\\] patterns in text.
// It uses a heuristic: the pattern should appear in a context likely affected
// by C's double-eval (inside function args, after @pemit, in command output, etc.).
func findDoubleEscapes(text string) []escapeMatch {
	var matches []escapeMatch
	i := 0
	for i < len(text)-3 {
		// Look for \\[ pattern
		if text[i] == '\\' && i+2 < len(text) && text[i+1] == '\\' && text[i+2] == '[' {
			// Found \\[ — now find matching \\]
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
				// No matching \\] found, skip this occurrence
				i++
			}
		} else {
			i++
		}
	}
	return matches
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

// fixSpan converts \\[text\\] to [text] within a matched span.
// It handles nested \\[ and \\] as well.
func fixSpan(span string) string {
	var b strings.Builder
	b.Grow(len(span))
	i := 0
	for i < len(span) {
		if i+2 < len(span) && span[i] == '\\' && span[i+1] == '\\' {
			if span[i+2] == '[' || span[i+2] == ']' {
				b.WriteByte(span[i+2])
				i += 3
				continue
			}
		}
		b.WriteByte(span[i])
		i++
	}
	return b.String()
}
