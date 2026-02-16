package validate

import (
	"strings"
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func makeTestDB(objects ...*gamedb.Object) *gamedb.Database {
	db := gamedb.NewDatabase()
	for _, obj := range objects {
		db.Objects[obj.DBRef] = obj
	}
	return db
}

func TestDoubleEscapeChecker(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef: 25,
			Name:  "Test Object",
			Owner: 1,
			Attrs: []gamedb.Attribute{
				// \\[Monitor\\] inside ansi() function argument
				{Number: 39, Value: `@pemit %#=[ansi(c,\\[Monitor\\])] connected.`},
			},
		},
	)

	c := &DoubleEscapeChecker{}
	findings := c.Check(db)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Category != CatDoubleEscape {
		t.Errorf("expected CatDoubleEscape, got %v", f.Category)
	}
	if !f.Fixable {
		t.Error("expected finding to be fixable")
	}
	if f.ObjectRef != 25 {
		t.Errorf("expected object #25, got #%d", f.ObjectRef)
	}
}

func TestDoubleEscapeNotFlaggedOutsideFunction(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef: 25,
			Name:  "Test Object",
			Owner: 1,
			Attrs: []gamedb.Attribute{
				// \\[Monitor\\] at top level — NOT inside a function
				{Number: 39, Value: `@pemit %#=\\[Monitor\\] connected.`},
			},
		},
	)

	c := &DoubleEscapeChecker{}
	findings := c.Check(db)

	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for top-level \\\\[...\\\\], got %d", len(findings))
	}
}

func TestDoubleEscapeFixApply(t *testing.T) {
	obj := &gamedb.Object{
		DBRef: 25,
		Name:  "Test Object",
		Owner: 1,
		Attrs: []gamedb.Attribute{
			{Number: 39, Value: `@pemit %#=[ansi(c,\\[Monitor\\])] connected.`},
		},
	}
	db := makeTestDB(obj)

	v := New(db)
	findings := v.Run()

	// Find the double-escape finding
	var deFindings []Finding
	for _, f := range findings {
		if f.Category == CatDoubleEscape {
			deFindings = append(deFindings, f)
		}
	}
	if len(deFindings) == 0 {
		t.Fatal("expected at least one double-escape finding")
	}

	err := v.ApplyFix(deFindings[0].ID)
	if err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}

	// Check the attr was fixed: \\[Monitor\\] → \[Monitor\]
	got := obj.Attrs[0].Value
	expected := `@pemit %#=[ansi(c,\[Monitor\])] connected.`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDoubleEscapeWithPrefix(t *testing.T) {
	obj := &gamedb.Object{
		DBRef: 100,
		Name:  "Prefixed Object",
		Owner: 1,
		Attrs: []gamedb.Attribute{
			{Number: 39, Value: "\x019118:0:@pemit %#=[ansi(c,\\\\[Monitor\\\\])] connected."},
		},
	}
	db := makeTestDB(obj)

	v := New(db)
	findings := v.Run()

	var deFindings []Finding
	for _, f := range findings {
		if f.Category == CatDoubleEscape {
			deFindings = append(deFindings, f)
		}
	}
	if len(deFindings) == 0 {
		t.Fatal("expected double-escape finding even with \\x01 prefix")
	}

	err := v.ApplyFix(deFindings[0].ID)
	if err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}

	// Check prefix was preserved and fix applied
	got := obj.Attrs[0].Value
	if got[:1] != "\x01" {
		t.Error("expected \\x01 prefix to be preserved")
	}
	if got != "\x019118:0:@pemit %#=[ansi(c,\\[Monitor\\])] connected." {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestPercentChecker(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef: 50,
			Name:  "Percent Test",
			Owner: 1,
			Attrs: []gamedb.Attribute{
				{Number: 39, Value: `say Testing \\%r line break`},
			},
		},
	)

	c := &PercentChecker{}
	findings := c.Check(db)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !findings[0].Fixable {
		t.Error("expected fixable")
	}
}

func TestEscapeSeqChecker(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef: 60,
			Name:  "ANSI Object",
			Owner: 1,
			Attrs: []gamedb.Attribute{
				{Number: 6, Value: "Hello \x1b[31mred\x1b[0m world"},
			},
		},
	)

	c := &EscapeSeqChecker{}
	findings := c.Check(db)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fixable {
		t.Error("escape seq findings should not be fixable")
	}
	if findings[0].Severity != SevInfo {
		t.Error("expected SevInfo severity")
	}
}

func TestIntegrityChecker(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef:    0,
			Name:     "Room Zero",
			Owner:    1,
			Location: gamedb.Nothing,
			Contents: 2,    // points to non-existent object
			Exits:    gamedb.Nothing,
			Next:     gamedb.Nothing,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Link:     gamedb.Nothing,
		},
		&gamedb.Object{
			DBRef:    1,
			Name:     "God",
			Owner:    1,
			Location: 0,
			Contents: gamedb.Nothing,
			Exits:    gamedb.Nothing,
			Next:     gamedb.Nothing,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Link:     gamedb.Nothing,
			Flags:    [3]int{int(gamedb.TypePlayer), 0, 0},
		},
	)

	c := &IntegrityChecker{}
	findings := c.Check(db)

	// Should find that #0's contents head #2 doesn't exist
	found := false
	for _, f := range findings {
		if f.Category == CatIntegrityError && f.ObjectRef == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected integrity error for #0 contents pointing to non-existent #2")
	}
}

func TestValidatorApplyAll(t *testing.T) {
	obj := &gamedb.Object{
		DBRef: 25,
		Name:  "Test Object",
		Owner: 1,
		Attrs: []gamedb.Attribute{
			{Number: 39, Value: `@pemit %#=[ansi(c,\\[Monitor\\])] connected.`},
			{Number: 40, Value: `@pemit %#=[ansi(c,\\[Monitor\\])] disconnected.`},
		},
	}
	db := makeTestDB(obj)

	v := New(db)
	v.Run()
	count := v.ApplyAll(CatDoubleEscape)
	if count != 2 {
		t.Errorf("expected 2 fixes, got %d", count)
	}
}

func TestValidatorSummary(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef:    0,
			Name:     "Test",
			Owner:    1,
			Location: gamedb.Nothing,
			Contents: gamedb.Nothing,
			Exits:    gamedb.Nothing,
			Next:     gamedb.Nothing,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Link:     gamedb.Nothing,
			Attrs: []gamedb.Attribute{
				{Number: 39, Value: `@pemit %#=[switch(1,1,\\[test\\])]`},
			},
		},
	)

	v := New(db)
	v.Run()
	summary := v.Summary()
	if summary[CatDoubleEscape] != 1 {
		t.Errorf("expected 1 double-escape finding, got %d", summary[CatDoubleEscape])
	}
}

func TestSplitAttrPrefix(t *testing.T) {
	tests := []struct {
		input      string
		wantPrefix string
		wantText   string
	}{
		{"hello", "", "hello"},
		{"", "", ""},
		{"\x019118:0:hello world", "\x019118:0:", "hello world"},
		{"\x011:32:$test:action", "\x011:32:", "$test:action"},
	}
	for _, tt := range tests {
		prefix, text := splitAttrPrefix(tt.input)
		if prefix != tt.wantPrefix {
			t.Errorf("splitAttrPrefix(%q): prefix=%q, want %q", tt.input, prefix, tt.wantPrefix)
		}
		if text != tt.wantText {
			t.Errorf("splitAttrPrefix(%q): text=%q, want %q", tt.input, text, tt.wantText)
		}
	}
}

func TestFindAllDoubleEscapePairs(t *testing.T) {
	tests := []struct {
		input string
		count int
	}{
		{`\\[Monitor\\]`, 1},
		{`\\[foo\\] and \\[bar\\]`, 2},
		{`no brackets here`, 0},
		{`\[single escape\]`, 0},
		{`\\[nested \\[inner\\] outer\\]`, 1}, // nested
	}
	for _, tt := range tests {
		matches := findAllDoubleEscapePairs(tt.input)
		if len(matches) != tt.count {
			t.Errorf("findAllDoubleEscapePairs(%q): got %d matches, want %d", tt.input, len(matches), tt.count)
		}
	}
}

func TestFindDoubleEscapesInFuncArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{"inside ansi()", `[ansi(c,\\[Monitor\\])]`, 1},
		{"inside switch()", `switch(1,1,\\[test\\])`, 1},
		{"inside center()", `center(\\[title\\],78)`, 1},
		{"top level - not flagged", `@pemit %#=\\[Monitor\\] connected.`, 0},
		{"bare text - not flagged", `\\[Monitor\\]`, 0},
		{"nested functions", `[ansi(c,center(\\[Monitor\\],20))]`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := findDoubleEscapesInFuncArgs(tt.input)
			if len(matches) != tt.count {
				t.Errorf("findDoubleEscapesInFuncArgs(%q): got %d matches, want %d", tt.input, len(matches), tt.count)
			}
		})
	}
}

func TestFixSpan(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`\\[Monitor\\]`, `\[Monitor\]`},
		{`\\[foo\\]`, `\[foo\]`},
		{`hello`, `hello`},
	}
	for _, tt := range tests {
		got := fixSpan(tt.input)
		if got != tt.want {
			t.Errorf("fixSpan(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsInsideFuncArg(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		pos   int
		want  bool
	}{
		{"inside ansi()", `[ansi(c,\\[Monitor\\])]`, 8, true},
		{"inside switch()", `switch(x,\\[test\\])`, 9, true},
		{"top level", `\\[Monitor\\]`, 0, false},
		{"inside unknown func", `myfunc(\\[test\\])`, 7, false},
		{"nested inside ansi+center", `ansi(c,center(\\[x\\],5))`, 14, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInsideFuncArg(tt.text, tt.pos)
			if got != tt.want {
				t.Errorf("isInsideFuncArg(%q, %d) = %v, want %v", tt.text, tt.pos, got, tt.want)
			}
		})
	}
}

func TestExtractFuncNameBefore(t *testing.T) {
	tests := []struct {
		text     string
		parenPos int
		want     string
	}{
		{"ansi(c,test)", 4, "ansi"},
		{"[switch(1,1,test)]", 7, "switch"},
		{"center(text,78)", 6, "center"},
		{"(nofunc)", 0, ""},
		{" (space)", 1, ""},
	}
	for _, tt := range tests {
		got := extractFuncNameBefore(tt.text, tt.parenPos)
		if got != tt.want {
			t.Errorf("extractFuncNameBefore(%q, %d) = %q, want %q", tt.text, tt.parenPos, got, tt.want)
		}
	}
}

func TestBraceEscapeChecker(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef: 25,
			Name:  "Test Object",
			Owner: 1,
			Attrs: []gamedb.Attribute{
				{Number: 39, Value: `switch(test, 1,,\{[ansi(c,Monitor)] %N connected.})`},
			},
		},
	)

	c := &BraceEscapeChecker{}
	findings := c.Check(db)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Category != CatDoubleEscape {
		t.Errorf("expected CatDoubleEscape, got %v", f.Category)
	}
	if !f.Fixable {
		t.Error("expected finding to be fixable")
	}
}

func TestBraceEscapeFixApply(t *testing.T) {
	obj := &gamedb.Object{
		DBRef: 25,
		Name:  "Test Object",
		Owner: 1,
		Attrs: []gamedb.Attribute{
			{Number: 39, Value: `switch(test, 1,,\{[ansi(c,Monitor)] %N connected.})`},
		},
	}
	db := makeTestDB(obj)

	v := New(db)
	findings := v.Run()

	var beFindings []Finding
	for _, f := range findings {
		if f.Description != "" && len(f.Description) > 0 && f.ID != "" {
			if f.Category == CatDoubleEscape && f.AttrNum == 39 {
				// Check if it's a brace-escape finding (has \{ in description)
				if strings.Contains(f.Description, "Escaped brace") {
					beFindings = append(beFindings, f)
				}
			}
		}
	}
	if len(beFindings) == 0 {
		t.Fatal("expected at least one brace-escape finding")
	}

	err := v.ApplyFix(beFindings[0].ID)
	if err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}

	got := obj.Attrs[0].Value
	expected := `switch(test, 1,,{[ansi(c,Monitor)] %N connected.})`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFindBraceEscapes(t *testing.T) {
	tests := []struct {
		input string
		count int
	}{
		{`\{hello}`, 1},
		{`\{foo} and \{bar}`, 2},
		{`no braces here`, 0},
		{`{normal braces}`, 0},
		{`\\{double backslash}`, 0}, // \\{ is not a brace escape
		{`\{nested {inner} outer}`, 1},
	}
	for _, tt := range tests {
		matches := findBraceEscapes(tt.input)
		if len(matches) != tt.count {
			t.Errorf("findBraceEscapes(%q): got %d matches, want %d", tt.input, len(matches), tt.count)
		}
	}
}

func TestFixBraceSpan(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`\{hello}`, `{hello}`},
		{`\{[ansi(c,Monitor)] test}`, `{[ansi(c,Monitor)] test}`},
	}
	for _, tt := range tests {
		got := fixBraceSpan(tt.input)
		if got != tt.want {
			t.Errorf("fixBraceSpan(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNoFindingsOnCleanDB(t *testing.T) {
	db := makeTestDB(
		&gamedb.Object{
			DBRef:    0,
			Name:     "Clean Room",
			Owner:    1,
			Location: gamedb.Nothing,
			Contents: 1,
			Exits:    gamedb.Nothing,
			Next:     gamedb.Nothing,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Link:     gamedb.Nothing,
			Attrs: []gamedb.Attribute{
				{Number: 6, Value: "A clean description with no issues."},
			},
		},
		&gamedb.Object{
			DBRef:    1,
			Name:     "God",
			Owner:    1,
			Location: 0,
			Contents: gamedb.Nothing,
			Exits:    gamedb.Nothing,
			Next:     gamedb.Nothing,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Link:     gamedb.Nothing,
			Flags:    [3]int{int(gamedb.TypePlayer), 0, 0},
		},
	)

	v := New(db)
	findings := v.Run()
	if len(findings) != 0 {
		t.Errorf("expected 0 findings on clean DB, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s: %s", f.ID, f.Description)
		}
	}
}
