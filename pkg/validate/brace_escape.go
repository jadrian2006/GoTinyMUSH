package validate

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// BraceEscapeChecker detects \{...} patterns that were written for C TinyMUSH's
// double-evaluation in queue processing. In C, \{ in the first eval pass produces
// a literal { which then serves as brace grouping in the second eval pass (with
// EvStrip removing the braces). In Go's single-eval, \{ outputs a literal { character
// that appears in the final output, producing unwanted visible braces.
//
// The fix: change \{ to { so it becomes a real brace group that EvStrip can strip.
type BraceEscapeChecker struct{}

func (c *BraceEscapeChecker) Name() string { return "brace-escape" }

func (c *BraceEscapeChecker) Check(db *gamedb.Database) []Finding {
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

			// Skip attrs with AFRegexp flag (regex patterns may legitimately have \{)
			if def, ok := db.AttrNames[attr.Number]; ok && def.Flags&gamedb.AFRegexp != 0 {
				continue
			}

			_, text := splitAttrPrefix(attr.Value)
			matches := findBraceEscapes(text)
			if len(matches) == 0 {
				continue
			}

			proposed := applyBraceEscapeFixes(text, matches)
			attrName := db.GetAttrName(attr.Number)
			if attrName == "" {
				attrName = fmt.Sprintf("A_%d", attr.Number)
			}

			id := fmt.Sprintf("obj%d-attr%d-be%d", obj.DBRef, attr.Number, seq)
			seq++

			var effectParts []string
			var currentHL, proposedHL []Highlight
			offset := 0
			for _, m := range matches {
				old := text[m.start:m.end]
				fixed := fixBraceSpan(old)
				effectParts = append(effectParts, fmt.Sprintf("%q → %q", truncate(old, 60), truncate(fixed, 60)))
				currentHL = append(currentHL, Highlight{Start: m.start, End: m.end})
				pStart := m.start + offset
				pEnd := pStart + len(fixed)
				proposedHL = append(proposedHL, Highlight{Start: pStart, End: pEnd})
				offset += len(fixed) - len(old)
			}

			const displayMax = 300
			currentText, currentHLAdj := windowAroundHighlights(text, currentHL, displayMax)
			proposedText, proposedHLAdj := windowAroundHighlights(proposed, proposedHL, displayMax)

			capturedObj := obj
			capturedIdx := attrIdx
			capturedProposed := proposed

			f := Finding{
				ID:          id,
				Category:    CatDoubleEscape, // Same root cause as \\[...\\]
				Severity:    SevWarning,
				ObjectRef:   obj.DBRef,
				AttrNum:     attr.Number,
				AttrName:    attrName,
				OwnerRef:    obj.Owner,
				Description: fmt.Sprintf("Escaped brace pattern \\{ in %s on #%d (%s)", attrName, obj.DBRef, truncate(obj.Name, 30)),
				Current:     currentText,
				Proposed:    proposedText,
				CurrentHL:   currentHLAdj,
				ProposedHL:  proposedHLAdj,
				Effect:      strings.Join(effectParts, "; "),
				Explanation: `In C TinyMUSH, \{ was used inside function arguments so that the first eval pass ` +
					`would produce a literal { which then served as brace grouping in the second pass ` +
					`(where EvStrip removes braces). GoTinyMUSH's single-pass evaluation means \{ ` +
					`produces a visible { character in the output. The fix removes the backslash so ` +
					`{ becomes a real brace group that gets properly stripped during function evaluation.`,
				Fixable: true,
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

// braceMatch represents a \{...} span found in text.
type braceMatch struct {
	start int // index of the backslash in \{
	end   int // index after the closing }
}

// findBraceEscapes locates \{...} patterns in text.
// It looks for \{ (single backslash + open brace) and finds the matching }.
// Only flags instances that are likely inside function arguments or command
// output contexts (i.e., not bare \{ at the start of a line which might be
// intentional literal brace output).
func findBraceEscapes(text string) []braceMatch {
	var matches []braceMatch
	i := 0
	for i < len(text)-2 {
		// Look for \{ pattern (single backslash, not \\{)
		if text[i] == '\\' && text[i+1] == '{' {
			// Make sure it's not \\{ (double-backslash before brace)
			if i > 0 && text[i-1] == '\\' {
				i += 2
				continue
			}

			// Found \{ — find matching } respecting nesting
			braceStart := i
			j := i + 2
			depth := 1
			for j < len(text) && depth > 0 {
				switch text[j] {
				case '\\':
					// Skip escaped characters
					j += 2
					continue
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						matches = append(matches, braceMatch{
							start: braceStart,
							end:   j + 1,
						})
					}
				}
				j++
			}
			if depth == 0 {
				i = j
			} else {
				i += 2 // no matching }, skip
			}
		} else {
			i++
		}
	}
	return matches
}

// applyBraceEscapeFixes applies all brace escape fixes to the text.
func applyBraceEscapeFixes(text string, matches []braceMatch) string {
	if len(matches) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for _, m := range matches {
		b.WriteString(text[last:m.start])
		b.WriteString(fixBraceSpan(text[m.start:m.end]))
		last = m.end
	}
	b.WriteString(text[last:])
	return b.String()
}

// fixBraceSpan converts \{text} to {text} by removing the leading backslash.
// It preserves all content inside unchanged.
func fixBraceSpan(span string) string {
	if len(span) >= 2 && span[0] == '\\' && span[1] == '{' {
		return span[1:] // Remove the leading backslash
	}
	return span
}
