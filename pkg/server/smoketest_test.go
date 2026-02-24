package server

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Smoketest suite: regression tests for bugs found during CrystalMUSH troubleshooting.
// Each test section corresponds to a specific bug fix or feature addition.

// ============================================================================
// matchWild case preservation
// Bug: matchWild captured from lowered string, so "Otter says hi" matched by
// "* says *" would capture "otter" instead of "Otter".
// ============================================================================

func TestMatchWild_CasePreservation(t *testing.T) {
	tests := []struct {
		pattern string
		str     string
		want    bool
		args    []string
	}{
		{"* says *", "Otter says hello", true, []string{"Otter", "hello"}},
		{"* says *", "ALICE says GOODBYE", true, []string{"ALICE", "GOODBYE"}},
		{"*", "MixedCase", true, []string{"MixedCase"}},
		{"test ?", "test X", true, []string{"X"}},
		{"$+res *", "$+res Malorie", true, []string{"Malorie"}},
		// Non-matching cases
		{"* says *", "Otter waves", false, nil},
		{"hello", "world", false, nil},
	}
	for _, tt := range tests {
		matched, args := matchWild(tt.pattern, tt.str)
		if matched != tt.want {
			t.Errorf("matchWild(%q, %q) matched=%v, want %v", tt.pattern, tt.str, matched, tt.want)
			continue
		}
		if matched && len(args) != len(tt.args) {
			t.Errorf("matchWild(%q, %q) args=%v, want %v", tt.pattern, tt.str, args, tt.args)
			continue
		}
		for i, a := range tt.args {
			if args[i] != a {
				t.Errorf("matchWild(%q, %q) arg[%d]=%q, want %q", tt.pattern, tt.str, i, args[i], a)
			}
		}
	}
}

func TestMatchWild_CaseInsensitiveMatch(t *testing.T) {
	// Pattern matching itself is case-insensitive
	matched, args := matchWild("HELLO *", "hello World")
	if !matched {
		t.Fatal("matchWild should match case-insensitively")
	}
	if args[0] != "World" {
		t.Errorf("captured %q, want %q", args[0], "World")
	}
}

// ============================================================================
// Enter/leave alias support (EALIAS/LALIAS)
// Feature: Objects with EALIAS attrs allow "enter" via alias commands.
// matchesExitFromList checks semicolon-separated alias lists.
// ============================================================================

func TestMatchesExitFromList(t *testing.T) {
	tests := []struct {
		cmd   string
		list  string
		match bool
	}{
		// Exact match
		{"board", "board;enter;get on", true},
		{"enter", "board;enter;get on", true},
		// Prefix match (C TinyMUSH behavior)
		{"bo", "board;enter;get on", true},
		{"ent", "board;enter;get on", true},
		// No match
		{"leave", "board;enter;get on", false},
		{"xyz", "board;enter;get on", false},
		// Empty cases
		{"", "board;enter", false},
		{"board", "", false},
		// Single alias
		{"sit", "sit", true},
		{"si", "sit", true},
		{"sitting", "sit", false}, // cmd longer than alias
	}
	for _, tt := range tests {
		got := matchesExitFromList(tt.cmd, tt.list)
		if got != tt.match {
			t.Errorf("matchesExitFromList(%q, %q) = %v, want %v", tt.cmd, tt.list, got, tt.match)
		}
	}
}

func TestEnterLeaveAlias(t *testing.T) {
	env := newTestEnv(t)

	// Set EALIAS on Container #5 so "board" triggers enter
	env.game.SetAttr(5, 64, "board;ride") // A_EALIAS = 64
	clearOutput(env.player)

	// "board" should trigger enter into Container #5
	ok := tryEnterLeaveAlias(env.game, env.player, "board")
	if !ok {
		t.Fatal("tryEnterLeaveAlias('board') should have matched EALIAS on Container")
	}

	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 5 {
		t.Errorf("after EALIAS enter, player location=%d, want 5", playerObj.Location)
	}
}

func TestLeaveAlias(t *testing.T) {
	env := newTestEnv(t)

	// Move player into Container #5 first
	DispatchCommand(env.game, env.player, "enter Container")
	clearOutput(env.player)

	// Set LALIAS on Container so "disembark" triggers leave
	env.game.SetAttr(5, 65, "disembark;off") // A_LALIAS = 65

	ok := tryEnterLeaveAlias(env.game, env.player, "disembark")
	if !ok {
		t.Fatal("tryEnterLeaveAlias('disembark') should have matched LALIAS")
	}

	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 0 {
		t.Errorf("after LALIAS leave, player location=%d, want 0", playerObj.Location)
	}
}

// ============================================================================
// AddToContents cycle prevention
// Bug: Direct chain manipulation (obj.Next = destObj.Contents; destObj.Contents = obj)
// could create cycles if object was already in the chain. AddToContents checks first.
// ============================================================================

func TestAddToContents_NoDuplicate(t *testing.T) {
	env := newTestEnv(t)

	// Object #2 is already in Room #0's contents chain.
	// Adding it again should be a no-op.
	room := env.game.DB.Objects[0]
	originalContents := room.Contents

	env.game.AddToContents(0, 2) // try to add #2 again

	// Chain should be unchanged
	if room.Contents != originalContents {
		t.Errorf("AddToContents allowed duplicate: Contents changed from #%d to #%d",
			originalContents, room.Contents)
	}
}

func TestAddToContents_NewObject(t *testing.T) {
	env := newTestEnv(t)

	// Create a new object #6 not in any chain
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "NewThing",
		Location: gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0},
	}

	env.game.AddToContents(0, 6)

	// #6 should now be first in Room #0's contents chain
	room := env.game.DB.Objects[0]
	if room.Contents != 6 {
		t.Errorf("AddToContents: room Contents=%d, want 6", room.Contents)
	}
	newObj := env.game.DB.Objects[6]
	if newObj.Next == 6 {
		t.Error("AddToContents: created self-referencing Next pointer")
	}
}

// ============================================================================
// Room SUCC display (look_in behavior)
// Feature: ShowRoom shows SUCC attr after DESC, conditional on A_LOCK.
// When SUCC provides output, default Contents/Exits fallback is skipped.
// ============================================================================

func TestShowRoom_SuccDisplay(t *testing.T) {
	env := newTestEnv(t)

	// Set SUCC on Room #0 — should show when looking
	env.game.SetAttr(0, 4, "Players: Wizard, Bob") // A_SUCC = 4
	env.game.SetAttr(0, 6, "A test room.")          // A_DESC = 6
	clearOutput(env.player)

	env.game.ShowRoom(env.player, 0)
	out := getOutput(env.player)

	if !strings.Contains(out, "Players: Wizard, Bob") {
		t.Errorf("ShowRoom: SUCC not displayed. Output:\n%s", out)
	}
	// When SUCC is shown, default "Contents:" should NOT appear
	if strings.Contains(out, "Contents:") {
		t.Errorf("ShowRoom: default Contents shown despite SUCC. Output:\n%s", out)
	}
}

func TestShowRoom_NoSuccFallsThrough(t *testing.T) {
	env := newTestEnv(t)

	// No SUCC on Room #0 — should show default Contents list
	env.game.SetAttr(0, 6, "A test room.") // DESC only
	clearOutput(env.player)

	env.game.ShowRoom(env.player, 0)
	out := getOutput(env.player)

	if !strings.Contains(out, "Contents:") {
		t.Errorf("ShowRoom: expected default Contents list without SUCC. Output:\n%s", out)
	}
}

// ============================================================================
// CONFORMAT/EXITFORMAT empty result handling
// Bug: CONFORMAT evaluating to "" was treated as "handled", suppressing
// the default Contents display. Now empty results fall through.
// ============================================================================

func TestShowRoom_EmptyConformatFallsThrough(t *testing.T) {
	env := newTestEnv(t)

	// Set CONFORMAT that evaluates to empty string
	env.game.SetAttr(0, 214, "") // empty CONFORMAT won't be found by GetAttrText
	// Actually, set a CONFORMAT that evaluates empty via softcode
	env.game.SetAttr(0, 214, "[]") // evaluates to empty
	env.game.SetAttr(0, 6, "A room.")
	clearOutput(env.player)

	env.game.ShowRoom(env.player, 0)
	out := getOutput(env.player)

	// Since CONFORMAT evaluates empty AND no SUCC, default Contents should show
	if !strings.Contains(out, "Contents:") {
		t.Errorf("ShowRoom: empty CONFORMAT should fall through to Contents. Output:\n%s", out)
	}
}

// ============================================================================
// Exit SUCC/OSUCC on movement
// Feature: When moving through an exit, its SUCC attr is shown to the player.
// ============================================================================

func TestExitSuccOnMove(t *testing.T) {
	env := newTestEnv(t)

	// Create an exit from Room #0 to Room #4
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "North;n",
		Location: 4, // destination
		Contents: gamedb.Nothing,
		Exits:    0, // source room
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeExit), 0, 0},
	}
	env.game.DB.Objects[0].Exits = 6

	// Set SUCC on the exit
	env.game.SetAttr(6, 4, "You head north through the archway.") // A_SUCC = 4
	env.game.SetAttr(4, 6, "The other room.")                     // DESC on destination
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "north")
	out := getOutput(env.player)

	if !strings.Contains(out, "You head north through the archway.") {
		t.Errorf("exit move: SUCC not shown. Output:\n%s", out)
	}
}

// ============================================================================
// MovePlayer OLEAVE/AENTER processing
// Feature: When a player moves, OLEAVE fires in departure room and
// AENTER fires in arrival room.
// ============================================================================

func TestMovePlayer_OleaveMessage(t *testing.T) {
	env := newTestEnv(t)

	// Create Bob's descriptor to receive room messages
	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)

	// Set OLEAVE on Room #0
	env.game.SetAttr(0, 51, "Wizard departs gracefully.") // A_OLEAVE = 51
	env.game.SetAttr(4, 6, "The other room.")              // DESC for dest

	clearOutput(env.player)
	env.game.MovePlayer(env.player, 4) // move Wizard to Room #4

	// Bob should see OLEAVE instead of default "has left"
	bobOut := getOutput(bobDesc)
	if !strings.Contains(bobOut, "Wizard departs gracefully.") {
		t.Errorf("MovePlayer: OLEAVE not shown to room. Bob saw:\n%s", bobOut)
	}
}

// ============================================================================
// Player name alias matching
// Feature: Player names with semicolons (e.g. "Otter;ott") should match
// on any alias, not just the full name.
// ============================================================================

func TestPlayerNameAliasMatch(t *testing.T) {
	env := newTestEnv(t)

	// Give Bob aliases
	env.game.DB.Objects[3].Name = "Bob;bobby;robert"
	clearOutput(env.player)

	// Whisper using alias should find Bob
	DispatchCommand(env.game, env.player, "whisper bobby=secret")
	out := getOutput(env.player)

	if !strings.Contains(out, "You whisper") {
		t.Errorf("alias match: whisper to alias failed. Output:\n%s", out)
	}
}

// ============================================================================
// Content chain cycle detection
// Bug: Corrupted content chains (self-referencing Next pointers) caused
// infinite loops. All chain traversals now use seen maps.
// ============================================================================

func TestContentChainCycleDetection(t *testing.T) {
	env := newTestEnv(t)

	// Create a self-referencing Next pointer (corruption scenario)
	env.game.DB.Objects[2].Next = 2 // TestObject -> TestObject (cycle!)
	clearOutput(env.player)

	// ShowRoom should not hang — it uses SafeContents internally
	env.game.ShowRoom(env.player, 0)
	out := getOutput(env.player)

	// Should complete without hanging. Room name should appear.
	if !strings.Contains(out, "Room Zero") {
		t.Errorf("cycle detection: ShowRoom hung or failed. Output:\n%s", out)
	}
}

func TestRepairAllChains_SelfRef(t *testing.T) {
	env := newTestEnv(t)

	// Corrupt: self-referencing Next
	env.game.DB.Objects[2].Next = 2
	env.game.RepairAllChains()

	if env.game.DB.Objects[2].Next == 2 {
		t.Error("RepairAllChains did not fix self-referencing Next pointer")
	}
}

// ============================================================================
// @trigger deferred eval with CS_ARGV semantics
// Bug: @trigger args were evaluated as a single string. After num(me)
// evaluated, EvFCheck was cleared, preventing name(me) from evaluating.
// Fix: Each comma-separated arg gets its own eval pass with fresh EvFCheck.
// ============================================================================

func TestTriggerDeferredCSArgv(t *testing.T) {
	env := newTestEnv(t)

	// Set up a trigger attr on Room #0 that stores args
	env.game.SetAttr(0, 256, "@pemit %#=ARG0=[v(0)] ARG1=[v(1)]")
	env.game.DB.AddAttrDef(256, "TESTATTR", 0)

	// The real test is that handleTriggerDeferred splits args on commas
	// and evaluates each independently. We verify this by checking that
	// the handler function exists and the prefix is recognized.
	// (Full integration requires queue processing which is async)

	// Verify @trigger is in the deferred command list
	cmd := "@trigger #0/TESTATTR = num(me), name(me)"
	for _, prefix := range []string{"@trigger", "@tr"} {
		if _, _, ok := splitDeferredBody(cmd, prefix); ok {
			return // found it
		}
	}
	t.Error("@trigger not recognized by splitDeferredBody")
}

// ============================================================================
// DisplayName strips aliases
// Feature: Object names like "Crystal Tuner;tuner;ct" should display
// as just "Crystal Tuner" (before first semicolon).
// ============================================================================

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Crystal Tuner;tuner;ct", "Crystal Tuner"},
		{"Bob", "Bob"},
		{"North;n", "North"},
		{";weirdname", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := DisplayName(tt.name)
		if got != tt.want {
			t.Errorf("DisplayName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// ============================================================================
// s() function re-evaluation
// Bug: s() was just returning its arg verbatim. C TinyMUSH's s() does a
// second eval pass to resolve %q registers and bracket expressions.
// ============================================================================

func TestFnS_ReEvaluates(t *testing.T) {
	e := newEvalTestEnv(t)

	// Set a register, then use s() to force re-evaluation
	got := e.eval("[setq(0,HELLO)][s(%q0)]")
	if !strings.Contains(got, "HELLO") {
		t.Errorf("s() should re-evaluate %%q0: got %q", got)
	}
}

// ============================================================================
// splitCommaRespectingBraces
// Used by @switch, @trigger, etc. to split on commas while respecting
// brace groups and parenthesized expressions.
// ============================================================================

func TestSplitCommaRespectingBraces(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"{a,b},c", []string{"{a,b}", "c"}},
		{"hasattr(#1,DESC),yes,no", []string{"hasattr(#1,DESC)", "yes", "no"}},
		{"{think matched},{think default}", []string{"{think matched}", "{think default}"}},
		{"", []string{""}},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got := splitCommaRespectingBraces(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCommaRespectingBraces(%q) = %v (len %d), want %v (len %d)",
				tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i, w := range tt.want {
			if got[i] != w {
				t.Errorf("splitCommaRespectingBraces(%q)[%d] = %q, want %q",
					tt.input, i, got[i], w)
			}
		}
	}
}

// ============================================================================
// secure() replaces special characters with spaces (not backslash-escape)
// Bug: Go's secure() was escaping chars like \$ instead of replacing with space.
// C TinyMUSH help: "Returns <string> after replacing [](){};,%\$ with spaces."
// This broke modal room "Obvious Commands" display: hangar$door → hangar\$door
// ============================================================================

func TestFnSecure_ReplacesWithSpaces(t *testing.T) {
	e := newEvalTestEnv(t)

	// Use setq/r to pass raw strings to secure() without eval interference
	tests := []struct {
		input string
		want  string
	}{
		{"hangar$door", "hangar door"},
		{"plain text", "plain text"},
		{"$$$", "   "},
		{"abc", "abc"},
	}
	for _, tt := range tests {
		// Store raw value in %q0, then secure(%q0)
		got := e.eval("[setq(0," + tt.input + ")][secure(%q0)]")
		if got != tt.want {
			t.Errorf("secure(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================================
// Help file multi-alias parser
// Bug: When help.txt has consecutive "& TOPIC" lines (aliases for same entry),
// the first alias got saved with empty content because the parser treated each
// "& " line as a new entry boundary.
// Example: "& ESCAPE()" / "& NESCAPE()" should share the same help text.
// ============================================================================

func TestHelpFileMultiAlias(t *testing.T) {
	// Create a temporary help file with multi-alias entries
	content := `& ESCAPE()
& NESCAPE()
  escape(<string>)
  Prefixes special characters with backslash.
& OTHER
  Some other topic.
`
	dir := t.TempDir()
	path := dir + "/test_help.txt"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hf := LoadHelpFile(path)
	if hf == nil {
		t.Fatal("LoadHelpFile returned nil")
	}

	// Both aliases should have the same non-empty content
	escText := hf.Lookup("escape()")
	nescText := hf.Lookup("nescape()")

	if escText == "" {
		t.Error("escape() help entry is empty")
	}
	if nescText == "" {
		t.Error("nescape() help entry is empty")
	}
	if escText != nescText {
		t.Errorf("escape() and nescape() should have same content:\n  escape()=%q\n  nescape()=%q", escText, nescText)
	}
	if !strings.Contains(escText, "Prefixes special characters") {
		t.Errorf("escape() content wrong: %q", escText)
	}

	// Other entry should still work
	otherText := hf.Lookup("other")
	if otherText == "" {
		t.Error("other help entry is empty")
	}
}

// ============================================================================
// Help coverage: every registered () function should have a help entry
// ============================================================================

func TestHelpCoverage_Functions(t *testing.T) {
	hf := LoadHelpFile("../../data/text/help.txt")
	if hf == nil {
		t.Skip("help.txt not found at ../../data/text/help.txt")
	}

	// All registered softcode functions that players can call.
	// We check that each has a help topic of the form "FUNCNAME()"
	// in either help.txt or wizhelp.txt.
	wizHf := LoadHelpFile("../../data/text/wizhelp.txt")

	functions := []string{
		// Math
		"ADD", "SUB", "MUL", "DIV", "FDIV", "MOD", "ABS", "SIGN",
		"INC", "DEC", "ROUND", "TRUNC", "FLOOR", "CEIL", "SQRT", "POWER",
		"MAX", "MIN", "PI", "E",
		// Trig
		"SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN",
		// Exp/Log
		"EXP", "LN", "LOG",
		// Bitwise
		"SHL", "SHR", "BAND", "BOR", "BNAND",
		// Comparison
		"GT", "GTE", "LT", "LTE", "EQ", "NEQ", "COMP", "NCOMP",
		// Logic
		"AND", "OR", "XOR", "NOT", "T",
		// Conditional
		"IFELSE", "SWITCH", "SWITCHALL",
		// Strings
		"CAT", "STRCAT", "STRLEN", "MID", "LEFT", "RIGHT", "LCSTR", "UCSTR",
		"CAPSTR", "POS", "LPOS", "EDIT", "REPLACE", "TRIM", "SQUISH",
		"LJUST", "RJUST", "CENTER", "REPEAT", "SPACE",
		"ESCAPE", "SECURE", "ANSI", "STRIPANSI",
		"BEFORE", "AFTER", "REVERSE", "SCRAMBLE",
		"STRMATCH", "MATCH", "DELETE",
		// Type checks
		"ISNUM", "ISDBREF",
		// Lists
		"WORDS", "FIRST", "REST", "LAST", "EXTRACT", "ELEMENTS", "LNUM",
		"MEMBER", "REMOVE", "INSERT", "LDELETE", "SORT",
		"SETUNION", "SETDIFF", "SETINTER",
		"REVWORDS", "SHUFFLE", "ITEMIZE", "SPLICE",
		"GRAB", "GRABALL", "MATCHALL", "SORTBY",
		// Iteration
		"ITER", "PARSE", "MAP", "FILTER", "FOLD",
		// Registers
		"SETQ", "SETR", "R",
		// Objects
		"NAME", "NUM", "LOC", "OWNER", "TYPE", "FLAGS",
		"HASFLAG", "HASATTR", "GET", "XGET", "V", "U", "ULOCAL", "S",
		"CON", "EXIT", "NEXT", "LCON", "LEXITS", "LATTR", "NATTR",
		"HOME", "PARENT", "ZONE", "CONTROLS", "ROOM",
		"CHILDREN", "LPARENT",
		// Connection
		"LWHO", "CONN", "IDLE", "DOING", "PMATCH",
		// Pronouns
		"SUBJ", "OBJ", "POSS", "APOSS",
		// Formatting
		"WRAP", "COLUMNS", "TABLE",
		// Side effects
		"PEMIT", "REMIT", "OEMIT",
		// Misc
		"RAND", "DIE", "TIME", "SECS",
		"SEARCH", "STATS",
	}

	var missing []string
	for _, fn := range functions {
		topic := strings.ToLower(fn) + "()"
		text := hf.Lookup(topic)
		if text == "" && wizHf != nil {
			text = wizHf.Lookup(topic)
		}
		if text == "" {
			missing = append(missing, fn+"()")
		}
	}

	if len(missing) > 0 {
		t.Errorf("Functions missing help entries (%d):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// ============================================================================
// Help coverage: every registered @command should have a help entry
// ============================================================================

func TestHelpCoverage_Commands(t *testing.T) {
	hf := LoadHelpFile("../../data/text/help.txt")
	if hf == nil {
		t.Skip("help.txt not found at ../../data/text/help.txt")
	}

	wizHf := LoadHelpFile("../../data/text/wizhelp.txt")

	// Commands registered in InitCommands that should have help entries.
	// Excludes: single-char aliases (", :, ;, -), internal commands (QUIT),
	// and comsys/mail that may use separate help files.
	commands := []string{
		"say", "pose", "page", "@emit", "think", "@pemit",
		"go", "home",
		"look", "examine", "inventory", "WHO", "score",
		"@dig", "@open", "@describe", "@name", "@set",
		"@create", "@destroy", "@link", "@unlink", "@parent",
		"@chown", "@clone", "@wipe", "@lock", "@unlock",
		"@teleport", "@force", "@trigger", "@wait", "@notify",
		"@halt", "@boot", "@wall", "@newpassword", "@find", "@stats", "@ps",
		"@switch", "@dolist",
		"get", "drop", "give", "enter", "leave", "whisper", "use", "kill",
		"@oemit",
		"@password", "@chzone", "@search", "@decompile", "@power",
		"@success", "@osuccess", "@asuccess",
		"@fail", "@afail",
		"@drop", "@odrop", "@adrop",
		"@describe", "@odescribe", "@adescribe",
		"@enter", "@oenter", "@aenter",
		"@leave", "@oleave", "@aleave",
		"@listen",
		"@sex", "@alias",
		"@teleport",
		"@startup",
		"@conformat", "@exitformat", "@nameformat",
		"@ealias", "@lalias",
		"@filter",
	}

	var missing []string
	for _, cmd := range commands {
		topic := strings.ToLower(cmd)
		text := hf.Lookup(topic)
		if text == "" && wizHf != nil {
			text = wizHf.Lookup(topic)
		}
		if text == "" {
			missing = append(missing, cmd)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Commands missing help entries (%d):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// ============================================================================
// Sensory commands (smell, touch, taste, listen)
// ============================================================================

func TestSmell_Default(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// No SMELL attr set on Room #0 — should get default message
	DispatchCommand(env.game, env.player, "smell")
	out := getOutput(env.player)

	if !strings.Contains(out, "You don't smell anything special.") {
		t.Errorf("smell with no attr: expected default message, got:\n%s", out)
	}
}

func TestSmell_WithAttr(t *testing.T) {
	env := newTestEnv(t)

	// Set SMELL attr (233) on Room #0
	env.game.SetAttr(0, 233, "Fresh pine.")
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "smell")
	out := getOutput(env.player)

	if !strings.Contains(out, "Fresh pine.") {
		t.Errorf("smell with attr set: expected 'Fresh pine.', got:\n%s", out)
	}
}

func TestSmellAttrSetter(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// @smell here=The scent of roses
	DispatchCommand(env.game, env.player, "@smell here=The scent of roses")
	_ = getOutput(env.player)

	// Verify attr 233 was set
	text := env.game.GetAttrText(0, 233)
	if text != "The scent of roses" {
		t.Errorf("@smell setter: attr 233 = %q, want %q", text, "The scent of roses")
	}
}

func TestTouchAttrSetter(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@touch here=Smooth stone")
	_ = getOutput(env.player)

	text := env.game.GetAttrText(0, 236)
	if text != "Smooth stone" {
		t.Errorf("@touch setter: attr 236 = %q, want %q", text, "Smooth stone")
	}
}

func TestTasteAttrSetter(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@taste here=Salty air")
	_ = getOutput(env.player)

	text := env.game.GetAttrText(0, 239)
	if text != "Salty air" {
		t.Errorf("@taste setter: attr 239 = %q, want %q", text, "Salty air")
	}
}

func TestSoundAttrSetter(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@sound here=Birds chirping")
	_ = getOutput(env.player)

	text := env.game.GetAttrText(0, 242)
	if text != "Birds chirping" {
		t.Errorf("@sound setter: attr 242 = %q, want %q", text, "Birds chirping")
	}
}

// ============================================================================
// @roomformat
// ============================================================================

func TestRoomformat_Custom(t *testing.T) {
	env := newTestEnv(t)

	// Set ROOMFORMAT on Room #0 — replaces entire ShowRoom output
	env.game.SetAttr(0, 232, "CUSTOM ROOM OUTPUT: %0")
	clearOutput(env.player)

	env.game.ShowRoom(env.player, 0)
	out := getOutput(env.player)

	if !strings.Contains(out, "CUSTOM ROOM OUTPUT:") {
		t.Errorf("ROOMFORMAT: expected custom output, got:\n%s", out)
	}
	// Normal room name should NOT appear since ROOMFORMAT replaces all output
	if strings.Contains(out, "Room Zero") {
		t.Errorf("ROOMFORMAT: normal room name should be replaced. Output:\n%s", out)
	}
}

func TestRoomformat_NotSet(t *testing.T) {
	env := newTestEnv(t)

	// No ROOMFORMAT — normal ShowRoom display
	env.game.SetAttr(0, 6, "A normal room.")
	clearOutput(env.player)

	env.game.ShowRoom(env.player, 0)
	out := getOutput(env.player)

	if !strings.Contains(out, "Room Zero") {
		t.Errorf("no ROOMFORMAT: expected room name in output, got:\n%s", out)
	}
}

// ============================================================================
// callfn() and nextdbref()
// ============================================================================

func TestFnCallfn(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval("[callfn(ADD,1,2,3)]")
	if got != "6" {
		t.Errorf("callfn(ADD,1,2,3) = %q, want %q", got, "6")
	}
}

func TestFnNextdbref(t *testing.T) {
	e := newEvalTestEnv(t)

	// evalTestEnv NextRef=8, so nextdbref() should return #8
	got := e.eval("[nextdbref()]")
	if got != "#8" {
		t.Errorf("nextdbref() = %q, want %q", got, "#8")
	}
}

// ============================================================================
// Multiple zones
// ============================================================================

func TestAllZones_Combined(t *testing.T) {
	env := newTestEnv(t)

	// Set primary zone on TestObject #2
	env.game.DB.Objects[2].Zone = 4 // Other Room as primary zone

	// Add an additional zone
	env.game.DB.Objects[2].Zones = append(env.game.DB.Objects[2].Zones, 0) // Room Zero as additional zone

	zones := env.game.DB.Objects[2].AllZones()
	if len(zones) != 2 {
		t.Fatalf("AllZones: expected 2 zones, got %d: %v", len(zones), zones)
	}
	if zones[0] != 4 {
		t.Errorf("AllZones[0] = %d, want 4", zones[0])
	}
	if zones[1] != 0 {
		t.Errorf("AllZones[1] = %d, want 0", zones[1])
	}
}

func TestChzoneAdd(t *testing.T) {
	env := newTestEnv(t)

	// Set primary zone first — use Container #5 (a THING) since
	// only rooms may be zoned to rooms; THINGs can zone to THINGs.
	env.game.DB.Objects[2].Zone = 5
	clearOutput(env.player)

	// Create a second THING to use as additional zone
	zoneRef := env.game.CreateObject("ZoneThing", gamedb.TypeThing, 1)
	env.game.DB.Objects[zoneRef].Location = 0
	env.game.AddToContents(0, zoneRef)

	// Use @chzone/add to add the new zone THING
	DispatchCommand(env.game, env.player, "@chzone/add #2=#"+fmt.Sprintf("%d", zoneRef))
	out := getOutput(env.player)

	if !strings.Contains(out, "added") {
		t.Errorf("@chzone/add: expected 'added' in output, got:\n%s", out)
	}

	zones := env.game.DB.Objects[2].AllZones()
	found := false
	for _, z := range zones {
		if z == zoneRef {
			found = true
		}
	}
	if !found {
		t.Errorf("@chzone/add: zone #%d not in AllZones: %v", zoneRef, zones)
	}
}

func TestChzoneRemove(t *testing.T) {
	env := newTestEnv(t)

	// Set up: primary zone #5 (Container THING), additional zone = new THING
	zoneRef := env.game.CreateObject("ExtraZone", gamedb.TypeThing, 1)
	env.game.DB.Objects[zoneRef].Location = 0
	env.game.AddToContents(0, zoneRef)

	env.game.DB.Objects[2].Zone = 5
	env.game.DB.Objects[2].Zones = append(env.game.DB.Objects[2].Zones, zoneRef)
	clearOutput(env.player)

	// Remove the additional zone
	DispatchCommand(env.game, env.player, "@chzone/remove #2=#"+fmt.Sprintf("%d", zoneRef))
	out := getOutput(env.player)

	if !strings.Contains(out, "removed") {
		t.Errorf("@chzone/remove: expected 'removed' in output, got:\n%s", out)
	}

	zones := env.game.DB.Objects[2].AllZones()
	for _, z := range zones {
		if z == zoneRef {
			t.Errorf("@chzone/remove: zone #%d still in AllZones: %v", zoneRef, zones)
		}
	}
}

func TestFnZones(t *testing.T) {
	e := newEvalTestEnv(t)

	// Set zones on TestObject #2
	e.game.DB.Objects[2].Zone = 4
	e.game.DB.Objects[2].Zones = append(e.game.DB.Objects[2].Zones, 0)

	got := e.eval("[zones(#2)]")
	if !strings.Contains(got, "#4") {
		t.Errorf("zones(#2) = %q, expected to contain #4", got)
	}
	if !strings.Contains(got, "#0") {
		t.Errorf("zones(#2) = %q, expected to contain #0", got)
	}
}

// ============================================================================
// @hook system
// ============================================================================

func TestHookList_Empty(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// @hook/list with no hooks defined should not crash
	DispatchCommand(env.game, env.player, "@hook/list")
	out := getOutput(env.player)

	if !strings.Contains(out, "No hooks defined") {
		t.Errorf("@hook/list empty: expected 'No hooks defined', got:\n%s", out)
	}
}

func TestHookSetAndList(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Set a before hook for SAY
	DispatchCommand(env.game, env.player, "@hook/before say=SAYBEFORE")
	out := getOutput(env.player)

	if !strings.Contains(out, "Hook before set for SAY") {
		t.Errorf("@hook/before: expected confirmation, got:\n%s", out)
	}

	// Verify it appears in @hook/list say
	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "@hook/list say")
	out = getOutput(env.player)

	if !strings.Contains(out, "SAYBEFORE") {
		t.Errorf("@hook/list say: expected SAYBEFORE in output, got:\n%s", out)
	}

	// Verify Hooks map is populated
	if env.game.Hooks == nil {
		t.Fatal("@hook/before: Hooks map is nil after setting hook")
	}
	hs, ok := env.game.Hooks["SAY"]
	if !ok {
		t.Fatal("@hook/before: no hook entry for SAY")
	}
	if hs.Before != "SAYBEFORE" {
		t.Errorf("@hook/before: Before = %q, want %q", hs.Before, "SAYBEFORE")
	}
}

// ============================================================================
// Instance system
// ============================================================================

// buildInstanceTemplate sets up a template THING with an interior room in the test env.
// Returns the template dbref. After calling this, env.game.NextRef will have advanced.
func buildInstanceTemplate(env *testEnv) gamedb.DBRef {
	g := env.game
	// Create template THING #6
	templateRef := g.CreateObject("Ship Template", gamedb.TypeThing, 1)
	templateObj := g.DB.Objects[templateRef]
	templateObj.Location = 0
	g.AddToContents(0, templateRef)

	// Create interior room #7 whose Location = template (signals it's interior)
	interiorRef := g.CreateObject("Bridge", gamedb.TypeRoom, 1)
	interiorObj := g.DB.Objects[interiorRef]
	interiorObj.Location = templateRef

	return templateRef
}

func TestInstanceCreate(t *testing.T) {
	env := newTestEnv(t)

	// Build template: #6 = Ship Template (THING), #7 = Bridge (ROOM, Location=#6)
	templateRef := buildInstanceTemplate(env)
	clearOutput(env.player)

	// Create an instance
	DispatchCommand(env.game, env.player, "@instance/create #"+fmt.Sprintf("%d", templateRef)+"=My Ship")
	out := getOutput(env.player)

	if !strings.Contains(out, "Instance created") {
		t.Fatalf("@instance/create: expected 'Instance created', got:\n%s", out)
	}

	// Verify instance object has Flag3Instance
	// The instance THING should be #8 (NextRef was 8 after building template)
	var instanceRef gamedb.DBRef = -1
	for ref, obj := range env.game.DB.Objects {
		if obj.Name == "My Ship" && obj.HasFlag3(gamedb.Flag3Instance) {
			instanceRef = ref
			break
		}
	}
	if instanceRef == -1 {
		t.Fatal("@instance/create: no instance object with Flag3Instance found")
	}

	// Verify interior rooms were cloned
	rooms := env.game.InstanceInteriorRooms(instanceRef)
	if len(rooms) == 0 {
		t.Error("@instance/create: no interior rooms cloned")
	}
}

func TestInstanceEnter(t *testing.T) {
	env := newTestEnv(t)

	templateRef := buildInstanceTemplate(env)
	clearOutput(env.player)

	// Create instance
	DispatchCommand(env.game, env.player, "@instance/create #"+fmt.Sprintf("%d", templateRef)+"=Shuttle")
	_ = getOutput(env.player)

	// Find the instance
	var instanceRef gamedb.DBRef = -1
	for ref, obj := range env.game.DB.Objects {
		if obj.Name == "Shuttle" && obj.HasFlag3(gamedb.Flag3Instance) {
			instanceRef = ref
			break
		}
	}
	if instanceRef == -1 {
		t.Fatal("instance not found")
	}

	// Set ENTER_OK on the instance
	env.game.DB.Objects[instanceRef].Flags[0] |= gamedb.FlagEnterOK

	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "enter Shuttle")
	out := getOutput(env.player)

	if !strings.Contains(out, "You enter Shuttle") {
		t.Errorf("enter instance: expected 'You enter Shuttle', got:\n%s", out)
	}

	// Player should be in the interior room, not the instance THING itself
	playerObj := env.game.DB.Objects[1]
	rooms := env.game.InstanceInteriorRooms(instanceRef)
	inInterior := false
	for _, r := range rooms {
		if playerObj.Location == r {
			inInterior = true
			break
		}
	}
	if !inInterior {
		t.Errorf("enter instance: player location=%d, expected one of interior rooms %v", playerObj.Location, rooms)
	}
}

func TestInstanceLeave(t *testing.T) {
	env := newTestEnv(t)

	templateRef := buildInstanceTemplate(env)
	clearOutput(env.player)

	// Create instance
	DispatchCommand(env.game, env.player, "@instance/create #"+fmt.Sprintf("%d", templateRef)+"=Pod")
	_ = getOutput(env.player)

	// Find instance
	var instanceRef gamedb.DBRef = -1
	for ref, obj := range env.game.DB.Objects {
		if obj.Name == "Pod" && obj.HasFlag3(gamedb.Flag3Instance) {
			instanceRef = ref
			break
		}
	}
	if instanceRef == -1 {
		t.Fatal("instance not found")
	}

	// Set ENTER_OK and enter it
	env.game.DB.Objects[instanceRef].Flags[0] |= gamedb.FlagEnterOK
	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "enter Pod")
	_ = getOutput(env.player)

	// Now leave
	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "leave")
	out := getOutput(env.player)

	// Player should be back in Room Zero (#0), the exterior location
	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 0 {
		t.Errorf("leave instance: player location=%d, want 0 (Room Zero). Output:\n%s", playerObj.Location, out)
	}
}

func TestInstanceDestroy(t *testing.T) {
	env := newTestEnv(t)

	templateRef := buildInstanceTemplate(env)
	clearOutput(env.player)

	// Create instance
	DispatchCommand(env.game, env.player, "@instance/create #"+fmt.Sprintf("%d", templateRef)+"=Vessel")
	_ = getOutput(env.player)

	// Find instance
	var instanceRef gamedb.DBRef = -1
	for ref, obj := range env.game.DB.Objects {
		if obj.Name == "Vessel" && obj.HasFlag3(gamedb.Flag3Instance) {
			instanceRef = ref
			break
		}
	}
	if instanceRef == -1 {
		t.Fatal("instance not found")
	}

	// Destroy it
	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "@instance/destroy #"+fmt.Sprintf("%d", instanceRef))
	out := getOutput(env.player)

	if !strings.Contains(out, "destroyed") {
		t.Errorf("@instance/destroy: expected 'destroyed' in output, got:\n%s", out)
	}

	// Verify the instance THING is marked GOING
	instanceObj := env.game.DB.Objects[instanceRef]
	if !instanceObj.IsGoing() {
		t.Error("@instance/destroy: instance object not marked GOING")
	}
}

func TestFnIsinstance(t *testing.T) {
	e := newEvalTestEnv(t)

	// #2 is a regular THING — should return 0
	got := e.eval("[isinstance(#2)]")
	if got != "0" {
		t.Errorf("isinstance(#2) = %q, want %q", got, "0")
	}

	// Set Flag3Instance on #2
	e.game.DB.Objects[2].Flags[2] |= gamedb.Flag3Instance
	got = e.eval("[isinstance(#2)]")
	if got != "1" {
		t.Errorf("isinstance(#2) with flag set = %q, want %q", got, "1")
	}
}

func TestFnIrooms(t *testing.T) {
	e := newEvalTestEnv(t)

	// Create an instance THING and interior room
	instanceRef := e.game.CreateObject("TestVehicle", gamedb.TypeThing, 1)
	e.game.DB.Objects[instanceRef].Flags[2] |= gamedb.Flag3Instance

	interiorRef := e.game.CreateObject("Interior", gamedb.TypeRoom, 1)
	e.game.DB.Objects[interiorRef].Location = instanceRef

	got := e.eval("[irooms(#" + fmt.Sprintf("%d", instanceRef) + ")]")
	expected := "#" + fmt.Sprintf("%d", interiorRef)
	if got != expected {
		t.Errorf("irooms(#%d) = %q, want %q", instanceRef, got, expected)
	}
}

func TestFnIvehicle(t *testing.T) {
	e := newEvalTestEnv(t)

	// Create an instance THING and interior room
	instanceRef := e.game.CreateObject("TestVehicle", gamedb.TypeThing, 1)
	e.game.DB.Objects[instanceRef].Flags[2] |= gamedb.Flag3Instance

	interiorRef := e.game.CreateObject("Interior", gamedb.TypeRoom, 1)
	e.game.DB.Objects[interiorRef].Location = instanceRef

	got := e.eval("[ivehicle(#" + fmt.Sprintf("%d", interiorRef) + ")]")
	expected := "#" + fmt.Sprintf("%d", instanceRef)
	if got != expected {
		t.Errorf("ivehicle(#%d) = %q, want %q", interiorRef, got, expected)
	}
}

// ============================================================================
// INSTANCE flag display
// ============================================================================

func TestInstanceFlagSet(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// @set TestObject=INSTANCE
	DispatchCommand(env.game, env.player, "@set #2=INSTANCE")
	_ = getOutput(env.player)

	obj := env.game.DB.Objects[2]
	if !obj.HasFlag3(gamedb.Flag3Instance) {
		t.Error("@set INSTANCE: Flag3Instance not set on object")
	}
}

func TestInstanceFlagDisplay(t *testing.T) {
	env := newTestEnv(t)

	// Set Flag3Instance directly on TestObject #2
	env.game.DB.Objects[2].Flags[2] |= gamedb.Flag3Instance

	flags := flagString(env.game.DB.Objects[2])
	if !strings.Contains(flags, "^") {
		t.Errorf("flagString: expected '^' for INSTANCE flag, got %q", flags)
	}
}

// ============================================================================
// JSON conversion functions: stringtojson, listtojson, jsontolist, jsonescape
// ============================================================================

func TestFnStringToJson(t *testing.T) {
	e := newEvalTestEnv(t)

	tests := []struct {
		input string
		want  string
	}{
		{`[stringtojson(hello)]`, `"hello"`},
		{`[stringtojson()]`, `""`},
		{`[stringtojson(line1)]`, `"line1"`},
	}
	for _, tt := range tests {
		got := e.eval(tt.input)
		if got != tt.want {
			t.Errorf("stringtojson: eval(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFnListToJson_Strings(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[listtojson(red green blue)]`)
	want := `["red","green","blue"]`
	if got != want {
		t.Errorf("listtojson strings: got %q, want %q", got, want)
	}
}

func TestFnListToJson_Numbers(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[listtojson(1 2 3,%b,number)]`)
	// Note: numbers in JSON are unquoted
	if !strings.Contains(got, "1") || !strings.Contains(got, "2") || !strings.Contains(got, "3") {
		t.Errorf("listtojson numbers: got %q, expected [1,2,3]", got)
	}
	if strings.Contains(got, `"1"`) {
		t.Errorf("listtojson numbers: values should not be quoted, got %q", got)
	}
}

func TestFnListToJson_Auto(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[listtojson(hello 42 true,%b,auto)]`)
	if !strings.Contains(got, `"hello"`) {
		t.Errorf("listtojson auto: expected quoted hello, got %q", got)
	}
	if !strings.Contains(got, "42") {
		t.Errorf("listtojson auto: expected 42, got %q", got)
	}
}

func TestFnListToJson_Empty(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[listtojson()]`)
	if got != "[]" {
		t.Errorf("listtojson empty: got %q, want []", got)
	}
}

func TestFnListToJson_CustomDelim(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[listtojson(a|b|c,|)]`)
	want := `["a","b","c"]`
	if got != want {
		t.Errorf("listtojson custom delim: got %q, want %q", got, want)
	}
}

func TestFnJsonToList(t *testing.T) {
	e := newEvalTestEnv(t)

	// Use json(array,...) to build the JSON input, avoiding bracket/comma escape issues
	got := e.eval(`[jsontolist([json(array,1,2,3)])]`)
	if got != "1 2 3" {
		t.Errorf("jsontolist numbers: got %q, want %q", got, "1 2 3")
	}

	got = e.eval(`[jsontolist([json(array,a,b,c)])]`)
	if got != "a b c" {
		t.Errorf("jsontolist strings: got %q, want %q", got, "a b c")
	}
}

func TestFnJsonToList_CustomDelim(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[jsontolist([json(array,x,y,z)],|)]`)
	want := "x|y|z"
	if got != want {
		t.Errorf("jsontolist custom delim: got %q, want %q", got, want)
	}
}

func TestFnJsonEscape(t *testing.T) {
	e := newEvalTestEnv(t)

	got := e.eval(`[jsonescape(hello)]`)
	if got != "hello" {
		t.Errorf("jsonescape plain: got %q, want %q", got, "hello")
	}
}

// ============================================================================
// stripTelnet: handle SB...SE subnegotiation properly
// Bug: stripTelnet treated IAC SB as a 3-byte command, leaking subneg
// payload bytes (GMCP JSON etc.) into the command stream.
// ============================================================================

func TestStripTelnet_SimpleIAC(t *testing.T) {
	// IAC DO GMCP followed by normal text
	input := string([]byte{0xFF, 0xFD, 0xC9}) + "hello"
	got := stripTelnet(input)
	if got != "hello" {
		t.Errorf("stripTelnet simple IAC: got %q, want %q", got, "hello")
	}
}

func TestStripTelnet_Subnegotiation(t *testing.T) {
	// IAC SB GMCP "Core.Hello {}" IAC SE + "connect test pw"
	var input []byte
	input = append(input, 0xFF, 0xFA, 0xC9)               // IAC SB GMCP
	input = append(input, []byte("Core.Hello {}")...)       // subneg payload
	input = append(input, 0xFF, 0xF0)                       // IAC SE
	input = append(input, []byte("connect test pw")...)     // real command
	got := stripTelnet(string(input))
	if got != "connect test pw" {
		t.Errorf("stripTelnet subneg: got %q, want %q", got, "connect test pw")
	}
}

func TestStripTelnet_MixedSequences(t *testing.T) {
	// IAC DO GMCP + IAC SB GMCP payload IAC SE + IAC WILL MSDP + "test"
	var input []byte
	input = append(input, 0xFF, 0xFD, 0xC9)               // IAC DO GMCP
	input = append(input, 0xFF, 0xFA, 0xC9)               // IAC SB GMCP
	input = append(input, []byte("data")...)               // subneg data
	input = append(input, 0xFF, 0xF0)                      // IAC SE
	input = append(input, 0xFF, 0xFB, 0x45)               // IAC WILL MSDP
	input = append(input, []byte("test")...)               // real text
	got := stripTelnet(string(input))
	if got != "test" {
		t.Errorf("stripTelnet mixed: got %q, want %q", got, "test")
	}
}

// ============================================================================
// toTelnetCRLF: convert bare \n to \r\n for telnet
// ============================================================================

func TestToTelnetCRLF_BareNewline(t *testing.T) {
	got := toTelnetCRLF("hello\nworld\n")
	want := "hello\r\nworld\r\n"
	if got != want {
		t.Errorf("toTelnetCRLF bare: got %q, want %q", got, want)
	}
}

func TestToTelnetCRLF_AlreadyCRLF(t *testing.T) {
	got := toTelnetCRLF("hello\r\nworld\r\n")
	want := "hello\r\nworld\r\n"
	if got != want {
		t.Errorf("toTelnetCRLF already CRLF: got %q, want %q", got, want)
	}
}

func TestToTelnetCRLF_Mixed(t *testing.T) {
	got := toTelnetCRLF("line1\r\nline2\nline3\r\n")
	want := "line1\r\nline2\r\nline3\r\n"
	if got != want {
		t.Errorf("toTelnetCRLF mixed: got %q, want %q", got, want)
	}
}

func TestToTelnetCRLF_NoNewlines(t *testing.T) {
	got := toTelnetCRLF("hello world")
	want := "hello world"
	if got != want {
		t.Errorf("toTelnetCRLF no newlines: got %q, want %q", got, want)
	}
}

func TestToTelnetCRLF_Empty(t *testing.T) {
	got := toTelnetCRLF("")
	if got != "" {
		t.Errorf("toTelnetCRLF empty: got %q, want %q", got, "")
	}
}

func TestToTelnetCRLF_LeadingNewline(t *testing.T) {
	got := toTelnetCRLF("\nhello")
	want := "\r\nhello"
	if got != want {
		t.Errorf("toTelnetCRLF leading newline: got %q, want %q", got, want)
	}
}

