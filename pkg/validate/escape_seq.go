package validate

import (
	"fmt"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// EscapeSeqChecker detects ESC bytes (0x1b) and other unusual escape sequences
// in attribute values. GoTinyMUSH supports these natively, so this is informational.
type EscapeSeqChecker struct{}

func (c *EscapeSeqChecker) Name() string { return "escape-seq" }

func (c *EscapeSeqChecker) Check(db *gamedb.Database) []Finding {
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
			_, text := splitAttrPrefix(attr.Value)

			escCount := 0
			for i := 0; i < len(text); i++ {
				if text[i] == 0x1b {
					escCount++
				}
			}
			if escCount == 0 {
				continue
			}

			attrName := db.GetAttrName(attr.Number)
			if attrName == "" {
				attrName = fmt.Sprintf("A_%d", attr.Number)
			}
			id := fmt.Sprintf("obj%d-attr%d-esc%d", obj.DBRef, attr.Number, seq)
			seq++

			findings = append(findings, Finding{
				ID:          id,
				Category:    CatEscapeSeq,
				Severity:    SevInfo,
				ObjectRef:   obj.DBRef,
				AttrNum:     attr.Number,
				AttrName:    attrName,
				OwnerRef:    obj.Owner,
				Description: fmt.Sprintf("%d ESC byte(s) in %s on #%d (%s) — ANSI codes, handled natively", escCount, attrName, obj.DBRef, truncate(obj.Name, 30)),
				Current:     truncate(text, 200),
				Fixable:     false,
			})
		}
	}
	return findings
}
