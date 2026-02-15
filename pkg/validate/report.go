package validate

import (
	"encoding/json"
	"io"
)

// Report is the JSON-serializable validation report consumed by the admin API.
type Report struct {
	TotalFindings int                    `json:"total_findings"`
	Categories    map[string]CategorySum `json:"categories"`
	Findings      []Finding              `json:"findings"`
}

// CategorySum summarizes findings for a single category.
type CategorySum struct {
	Total   int    `json:"total"`
	Fixable int    `json:"fixable"`
	Fixed   int    `json:"fixed"`
	Label   string `json:"label"`
}

var categoryLabels = map[Category]string{
	CatDoubleEscape:  "Double-Escaped Brackets (C quirk)",
	CatAttrFlags:     "Attribute Flag Anomalies",
	CatEscapeSeq:     "Unusual Escape Sequences",
	CatPercent:        "Backslash-Percent Patterns",
	CatIntegrityError: "Referential Integrity Errors",
	CatIntegrityWarn:  "Referential Integrity Warnings",
}

// GenerateReport builds a Report from the validator's current findings.
func GenerateReport(v *Validator) *Report {
	r := &Report{
		TotalFindings: len(v.findings),
		Categories:    make(map[string]CategorySum),
		Findings:      v.findings,
	}

	// Build category summaries
	catCounts := make(map[Category]*CategorySum)
	for _, f := range v.findings {
		cs, ok := catCounts[f.Category]
		if !ok {
			cs = &CategorySum{Label: categoryLabels[f.Category]}
			catCounts[f.Category] = cs
		}
		cs.Total++
		if f.Fixable {
			cs.Fixable++
		}
		if f.Fixed {
			cs.Fixed++
		}
	}
	for cat, cs := range catCounts {
		r.Categories[cat.String()] = *cs
	}

	return r
}

// WriteJSON writes the report as JSON to the given writer.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteJSONFindings writes just the findings array as JSON.
func WriteJSONFindings(w io.Writer, findings []Finding) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(findings)
}
