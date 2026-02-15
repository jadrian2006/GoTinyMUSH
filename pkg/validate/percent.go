package validate

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// PercentChecker detects backslash-percent patterns (\\% and \\\\%) that may
// produce incorrect output due to evaluation differences between C TinyMUSH
// and GoTinyMUSH.
type PercentChecker struct{}

func (c *PercentChecker) Name() string { return "percent" }

func (c *PercentChecker) Check(db *gamedb.Database) []Finding {
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
			_, text := splitAttrPrefix(attr.Value)

			// Look for \\% patterns (double-backslash before percent substitution)
			issues := findPercentIssues(text)
			if len(issues) == 0 {
				continue
			}

			attrName := db.GetAttrName(attr.Number)
			if attrName == "" {
				attrName = fmt.Sprintf("A_%d", attr.Number)
			}

			proposed := applyPercentFixes(text, issues)
			id := fmt.Sprintf("obj%d-attr%d-pct%d", obj.DBRef, attr.Number, seq)
			seq++

			var effectParts []string
			var currentHL, proposedHL []Highlight
			offset := 0
			for _, iss := range issues {
				effectParts = append(effectParts, fmt.Sprintf("%q at pos %d", iss.match, iss.pos))
				currentHL = append(currentHL, Highlight{Start: iss.pos, End: iss.pos + len(iss.match)})
				pStart := iss.pos + offset
				pEnd := pStart + len(iss.fix)
				proposedHL = append(proposedHL, Highlight{Start: pStart, End: pEnd})
				offset += len(iss.fix) - len(iss.match)
			}

			// Window the text around the highlighted areas
			const displayMax = 300
			currentText, currentHLAdj := windowAroundHighlights(text, currentHL, displayMax)
			proposedText, proposedHLAdj := windowAroundHighlights(proposed, proposedHL, displayMax)

			capturedObj := obj
			capturedIdx := attrIdx
			capturedProposed := proposed

			findings = append(findings, Finding{
				ID:          id,
				Category:    CatPercent,
				Severity:    SevWarning,
				ObjectRef:   obj.DBRef,
				AttrNum:     attr.Number,
				AttrName:    attrName,
				OwnerRef:    obj.Owner,
				Description: fmt.Sprintf("Backslash-percent pattern in %s on #%d (%s)", attrName, obj.DBRef, truncate(obj.Name, 30)),
				Current:     currentText,
				Proposed:    proposedText,
				CurrentHL:   currentHLAdj,
				ProposedHL:  proposedHLAdj,
				Effect:      strings.Join(effectParts, "; "),
				Explanation: `Similar to bracket escaping, \\% was used in C TinyMUSH to produce a literal % after double-evaluation. GoTinyMUSH's single-pass evaluation means \\% now produces \% instead. The fix removes the extra backslash so percent-substitutions like %r (newline) and %t (tab) work correctly.`,
				Fixable:     true,
				fixFunc: func() {
					prefix, _ := splitAttrPrefix(capturedObj.Attrs[capturedIdx].Value)
					capturedObj.Attrs[capturedIdx].Value = prefix + capturedProposed
				},
			})
		}
	}
	return findings
}

type percentIssue struct {
	pos   int
	match string // the problematic substring
	fix   string // the replacement
}

// findPercentIssues finds \\% patterns that are likely double-escaped for C's double-eval.
func findPercentIssues(text string) []percentIssue {
	var issues []percentIssue
	for i := 0; i < len(text)-2; i++ {
		if text[i] == '\\' && text[i+1] == '\\' && text[i+2] == '%' {
			issues = append(issues, percentIssue{
				pos:   i,
				match: "\\\\%",
				fix:   "\\%",
			})
			i += 2 // skip past \\%
		}
	}
	return issues
}

// applyPercentFixes applies all percent fixes to the text.
func applyPercentFixes(text string, issues []percentIssue) string {
	if len(issues) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for _, iss := range issues {
		b.WriteString(text[last:iss.pos])
		b.WriteString(iss.fix)
		last = iss.pos + len(iss.match)
	}
	b.WriteString(text[last:])
	return b.String()
}
