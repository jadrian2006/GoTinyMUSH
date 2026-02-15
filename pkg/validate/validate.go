// Package validate provides flatfile import validation for GoTinyMUSH.
// It detects C TinyMUSH quirks (double-escaped brackets, backslash-percent patterns, etc.)
// and referential integrity issues, with optional auto-fix support.
package validate

import (
	"fmt"
	"sort"
	"sync/atomic"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Category classifies the type of finding.
type Category int

const (
	CatDoubleEscape  Category = iota // \\[text\\] C double-eval quirk
	CatAttrFlags                     // Attr flag anomalies (informational)
	CatEscapeSeq                     // Unusual escape sequences (informational)
	CatPercent                       // Backslash-percent issues
	CatIntegrityError                // Broken references
	CatIntegrityWarn                 // Suspicious references
)

func (c Category) String() string {
	switch c {
	case CatDoubleEscape:
		return "double-escape"
	case CatAttrFlags:
		return "attr-flags"
	case CatEscapeSeq:
		return "escape-seq"
	case CatPercent:
		return "percent"
	case CatIntegrityError:
		return "integrity-error"
	case CatIntegrityWarn:
		return "integrity-warning"
	default:
		return "unknown"
	}
}

// Severity indicates how serious a finding is.
type Severity int

const (
	SevError   Severity = iota // Must be fixed for correct behavior
	SevWarning                 // Should be reviewed
	SevInfo                    // Informational only
)

func (s Severity) String() string {
	switch s {
	case SevError:
		return "error"
	case SevWarning:
		return "warning"
	case SevInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Finding represents a single validation issue detected in the database.
type Finding struct {
	ID          string       `json:"id"`
	Category    Category     `json:"category"`
	Severity    Severity     `json:"severity"`
	ObjectRef   gamedb.DBRef `json:"object_ref"`
	AttrNum     int          `json:"attr_num,omitempty"`
	AttrName    string       `json:"attr_name,omitempty"`
	OwnerRef    gamedb.DBRef `json:"owner_ref,omitempty"`
	Description string       `json:"description"`
	Current     string       `json:"current,omitempty"`
	Proposed    string       `json:"proposed,omitempty"`
	Effect      string       `json:"effect,omitempty"`
	Explanation string       `json:"explanation,omitempty"`
	Fixable     bool         `json:"fixable"`
	Fixed       bool         `json:"fixed"`
	fixFunc     func()       // unexported — called via ApplyFix()
}

// Checker is the interface that each validation check implements.
type Checker interface {
	Name() string
	Check(db *gamedb.Database) []Finding
}

// Validator orchestrates running all checkers against a database.
type Validator struct {
	checkers []Checker
	db       *gamedb.Database
	findings []Finding
	idSeq    atomic.Int64
}

// New creates a Validator with all built-in checkers registered.
func New(db *gamedb.Database) *Validator {
	v := &Validator{
		db: db,
		checkers: []Checker{
			&DoubleEscapeChecker{},
			&AttrFlagChecker{},
			&EscapeSeqChecker{},
			&PercentChecker{},
			&IntegrityChecker{},
		},
	}
	return v
}

// Run executes all checkers and returns findings sorted by dbref then attr number.
func (v *Validator) Run() []Finding {
	v.findings = nil
	for _, c := range v.checkers {
		v.findings = append(v.findings, c.Check(v.db)...)
	}
	sort.Slice(v.findings, func(i, j int) bool {
		if v.findings[i].ObjectRef != v.findings[j].ObjectRef {
			return v.findings[i].ObjectRef < v.findings[j].ObjectRef
		}
		return v.findings[i].AttrNum < v.findings[j].AttrNum
	})
	return v.findings
}

// Findings returns the current findings (after Run has been called).
func (v *Validator) Findings() []Finding {
	return v.findings
}

// ApplyFix applies a single fix by finding ID. Returns error if not found or not fixable.
func (v *Validator) ApplyFix(id string) error {
	for i := range v.findings {
		if v.findings[i].ID == id {
			if !v.findings[i].Fixable {
				return fmt.Errorf("finding %s is not fixable", id)
			}
			if v.findings[i].Fixed {
				return fmt.Errorf("finding %s is already fixed", id)
			}
			if v.findings[i].fixFunc != nil {
				v.findings[i].fixFunc()
				v.findings[i].Fixed = true
			}
			return nil
		}
	}
	return fmt.Errorf("finding %s not found", id)
}

// ApplyAll applies all fixable findings in the given category. Returns count of fixes applied.
func (v *Validator) ApplyAll(cat Category) int {
	count := 0
	for i := range v.findings {
		f := &v.findings[i]
		if f.Category == cat && f.Fixable && !f.Fixed && f.fixFunc != nil {
			f.fixFunc()
			f.Fixed = true
			count++
		}
	}
	return count
}

// Summary returns counts of findings per category.
func (v *Validator) Summary() map[Category]int {
	m := make(map[Category]int)
	for _, f := range v.findings {
		m[f.Category]++
	}
	return m
}

// SummaryByStatus returns counts of fixed vs unfixed findings per category.
func (v *Validator) SummaryByStatus() map[Category][2]int {
	m := make(map[Category][2]int) // [0]=unfixed, [1]=fixed
	for _, f := range v.findings {
		counts := m[f.Category]
		if f.Fixed {
			counts[1]++
		} else {
			counts[0]++
		}
		m[f.Category] = counts
	}
	return m
}

// stripAttrPrefix removes the "\x01owner:flags:" prefix from a raw attribute value.
// Returns (prefix, text) where prefix may be empty.
func splitAttrPrefix(raw string) (prefix, text string) {
	if len(raw) == 0 || raw[0] != '\x01' {
		return "", raw
	}
	colonCount := 0
	for i := 1; i < len(raw); i++ {
		if raw[i] == ':' {
			colonCount++
			if colonCount == 2 {
				return raw[:i+1], raw[i+1:]
			}
		}
	}
	return raw[:1], raw[1:]
}

// truncate returns at most max characters of s, adding "..." if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
