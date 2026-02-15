package validate

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// AttrFlagChecker detects attribute flag anomalies:
// - $-command attrs without \x01 prefix that contain colons in the pattern text,
//   which could be misinterpreted by parseAttrFlags.
// This is informational since GoTinyMUSH already handles this at runtime.
type AttrFlagChecker struct{}

func (c *AttrFlagChecker) Name() string { return "attr-flags" }

func (c *AttrFlagChecker) Check(db *gamedb.Database) []Finding {
	var findings []Finding
	seq := 0

	for _, obj := range db.Objects {
		if obj.IsGoing() {
			continue
		}
		for _, attr := range obj.Attrs {
			if attr.Value == "" {
				continue
			}
			// Only check attrs that start with $ (command patterns)
			_, text := splitAttrPrefix(attr.Value)
			if !strings.HasPrefix(text, "$") {
				continue
			}

			// If the attr has NO \x01 prefix but the text contains a colon
			// before the first : that separates pattern from action, it could
			// be misinterpreted.
			if attr.Value[0] != '\x01' && strings.Contains(text, ":") {
				// Check if colon is in the $pattern part (before first unescaped colon)
				colonIdx := strings.Index(text, ":")
				dollarPart := text[:colonIdx]
				// If the $pattern part itself contains what looks like owner:flags format
				if colonIdx > 1 && looksLikeAttrPrefix(dollarPart) {
					attrName := db.GetAttrName(attr.Number)
					if attrName == "" {
						attrName = fmt.Sprintf("A_%d", attr.Number)
					}
					id := fmt.Sprintf("obj%d-attr%d-af%d", obj.DBRef, attr.Number, seq)
					seq++

					findings = append(findings, Finding{
						ID:          id,
						Category:    CatAttrFlags,
						Severity:    SevInfo,
						ObjectRef:   obj.DBRef,
						AttrNum:     attr.Number,
						AttrName:    attrName,
						OwnerRef:    obj.Owner,
						Description: fmt.Sprintf("$-command attr without \\x01 prefix has colon in pattern on #%d %s", obj.DBRef, attrName),
						Current:     truncate(text, 200),
						Explanation: "This $-command attribute's pattern starts with a number followed by a colon, which could be confused with the internal owner:flags:value metadata prefix. GoTinyMUSH handles this correctly at runtime — informational only, no fix needed.",
						Fixable:     false,
					})
				}
			}
		}
	}
	return findings
}

// looksLikeAttrPrefix checks if text before colon could be confused with
// an owner:flags prefix (starts with digits).
func looksLikeAttrPrefix(text string) bool {
	if len(text) < 2 {
		return false
	}
	// Skip the leading $ for $-commands
	s := text
	if s[0] == '$' {
		s = s[1:]
	}
	// If it starts with a digit, it could be confused with "owner:" prefix
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9'
}
