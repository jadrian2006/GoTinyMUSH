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

// ============================================================================
// Regression: *player lookup checks A_ALIAS (attr 58), semicolon-separated
// Bug: *Dre would fail to find player "Dreu" who had A_ALIAS="Dre;DreAlt"
// C TinyMUSH: load_player_names() in player.c loads semicolon-separated aliases
// ============================================================================

func TestPlayerAliasLookup_MatchObject(t *testing.T) {
	env := newTestEnv(t)

	// Set A_ALIAS (58) on Bob (#3) with semicolon-separated aliases
	env.game.SetAttr(3, 58, "Bobby;Robert;Rob")

	// *Bob should still match by name
	ref := env.game.MatchObject(1, "*Bob")
	if ref != 3 {
		t.Errorf("*Bob: expected #3, got #%d", ref)
	}

	// *Bobby should match via A_ALIAS
	ref = env.game.MatchObject(1, "*Bobby")
	if ref != 3 {
		t.Errorf("*Bobby: expected #3 via A_ALIAS, got #%d", ref)
	}

	// *Robert should match second alias
	ref = env.game.MatchObject(1, "*Robert")
	if ref != 3 {
		t.Errorf("*Robert: expected #3 via A_ALIAS, got #%d", ref)
	}

	// *Rob should match third alias
	ref = env.game.MatchObject(1, "*Rob")
	if ref != 3 {
		t.Errorf("*Rob: expected #3 via A_ALIAS, got #%d", ref)
	}

	// *Nobody should NOT match
	ref = env.game.MatchObject(1, "*Nobody")
	if ref != gamedb.Nothing {
		t.Errorf("*Nobody: expected Nothing, got #%d", ref)
	}
}

func TestPlayerAliasLookup_Pmatch(t *testing.T) {
	env := newTestEnv(t)

	// Set A_ALIAS (58) on Bob (#3) with semicolon-separated aliases
	env.game.SetAttr(3, 58, "Bobby;Robert")

	clearOutput(env.player)

	// pmatch(*Bobby) should find #3 via GameState.LookupPlayer
	DispatchCommand(env.game, env.player, "think [pmatch(*Bobby)]")
	out := getOutput(env.player)
	if !strings.Contains(out, "#3") {
		t.Errorf("pmatch(*Bobby): expected #3, got: %s", out)
	}

	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "think [pmatch(*Robert)]")
	out = getOutput(env.player)
	if !strings.Contains(out, "#3") {
		t.Errorf("pmatch(*Robert): expected #3, got: %s", out)
	}

	// Bare name (no *) should also work
	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "think [pmatch(Bobby)]")
	out = getOutput(env.player)
	if !strings.Contains(out, "#3") {
		t.Errorf("pmatch(Bobby): expected #3, got: %s", out)
	}
}

// ============================================================================
// Regression: @tel triggers OTPORT/OXTPORT/ATPORT/OMOVE/AMOVE attributes
// Bug: @tel only sent "has left"/"has arrived" text, not teleport attributes
// C TinyMUSH: move_via_teleport() fires OXTPORT→LEAVE→move→TPORT/OTPORT/ATPORT→MOVE/OMOVE/AMOVE
// ============================================================================

func TestTeleport_TriggersAttributes(t *testing.T) {
	env := newTestEnv(t)

	// Set OTPORT (80) on Wizard - seen by others in destination room
	env.game.SetAttr(1, 80, "arrives with a whoosh.")

	// Set TPORT (79) on Wizard - seen by teleported player
	env.game.SetAttr(1, 79, "You feel a rush of wind.")

	// Set OXTPORT (81) on Wizard - seen by others in source room
	env.game.SetAttr(1, 81, "vanishes in a puff of smoke.")

	// Create a second player in OtherRoom (#4) to see messages there
	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)

	// Move Bob to OtherRoom
	env.game.DB.Objects[3].Location = 4
	env.game.RemoveFromContents(0, 3)
	env.game.AddToContents(4, 3)
	env.game.DB.Objects[4].Contents = 3

	clearOutput(env.player)
	clearOutput(bobDesc)

	// Teleport Wizard to OtherRoom
	DispatchCommand(env.game, env.player, "@tel me=#4")

	wizOut := getOutput(env.player)
	bobOut := getOutput(bobDesc)

	// Wizard should see TPORT message
	if !strings.Contains(wizOut, "You feel a rush of wind.") {
		t.Errorf("@tel TPORT: wizard should see 'You feel a rush of wind.', got:\n%s", wizOut)
	}

	// Bob (in destination room) should see OTPORT prefixed with name
	if !strings.Contains(bobOut, "Wizard arrives with a whoosh.") {
		t.Errorf("@tel OTPORT: Bob should see 'Wizard arrives with a whoosh.', got:\n%s", bobOut)
	}
}

// ============================================================================
// Regression: @tel sends departure/arrival messages for non-player objects
// Bug: cmdTeleport gated "has left"/"has arrived" behind isPlayer check,
//      so TYPE_THING objects (cartons, etc.) disappeared silently.
// Fix: removed isPlayer check — all non-dark objects get departure messages.
// ============================================================================

func TestTeleport_ThingDepartureMessage(t *testing.T) {
	env := newTestEnv(t)

	// Bob #3 observes in Room Zero
	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)
	clearOutput(env.player)

	// Teleport TestObject #2 (TYPE_THING) to OtherRoom #4
	DispatchCommand(env.game, env.player, "@tel #2=#4")

	bobOut := getOutput(bobDesc)

	// Bob should see "TestObject has left."
	if !strings.Contains(bobOut, "TestObject has left.") {
		t.Errorf("@tel THING: Bob should see 'TestObject has left.', got:\n%s", bobOut)
	}

	// Create an observer in OtherRoom to check arrival
	// Move Bob to OtherRoom first
	env.game.RemoveFromContents(0, 3)
	env.game.DB.Objects[3].Location = 4
	env.game.AddToContents(4, 3)
	clearOutput(bobDesc)

	// Teleport TestObject back to Room Zero
	DispatchCommand(env.game, env.player, "@tel #2=#0")

	bobOut = getOutput(bobDesc)
	// Bob (in OtherRoom) should see "TestObject has left."
	if !strings.Contains(bobOut, "TestObject has left.") {
		t.Errorf("@tel THING back: Bob should see 'TestObject has left.', got:\n%s", bobOut)
	}
}

func TestTeleport_ThingOLEAVE(t *testing.T) {
	env := newTestEnv(t)

	// Set OLEAVE (51) on Room Zero — custom departure message
	env.game.SetAttr(0, 51, "disappears in a flash.")

	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)
	clearOutput(env.player)

	// Teleport TestObject #2 to OtherRoom #4
	DispatchCommand(env.game, env.player, "@tel #2=#4")

	bobOut := getOutput(bobDesc)

	// Bob should see OLEAVE with object name prefix, not default "has left."
	if !strings.Contains(bobOut, "TestObject disappears in a flash.") {
		t.Errorf("@tel THING OLEAVE: expected 'TestObject disappears in a flash.', got:\n%s", bobOut)
	}
	if strings.Contains(bobOut, "has left") {
		t.Errorf("@tel THING OLEAVE: should NOT show default 'has left' when OLEAVE set, got:\n%s", bobOut)
	}
}

func TestTeleport_DarkThingSilent(t *testing.T) {
	env := newTestEnv(t)

	// Set TestObject #2 to DARK
	env.game.DB.Objects[2].Flags[0] |= gamedb.FlagDark

	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)
	clearOutput(env.player)

	// Teleport dark TestObject #2 to OtherRoom #4
	DispatchCommand(env.game, env.player, "@tel #2=#4")

	bobOut := getOutput(bobDesc)

	// Bob should NOT see any departure message for DARK objects
	if strings.Contains(bobOut, "has left") || strings.Contains(bobOut, "TestObject") {
		t.Errorf("@tel DARK THING: Bob should see nothing, got:\n%s", bobOut)
	}
}

// ============================================================================
// Regression: @destroy sends departure message for non-player objects
// Bug: cmdDestroy silently removed objects with no room announcement.
// Fix: added "has left." message to room when non-dark object is destroyed.
// ============================================================================

func TestDestroy_NoDepartureMessage(t *testing.T) {
	// C TinyMUSH does NOT send departure messages on @destroy.
	env := newTestEnv(t)

	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)
	clearOutput(env.player)

	// Destroy TestObject #2
	DispatchCommand(env.game, env.player, "@dest #2")

	bobOut := getOutput(bobDesc)

	// Bob should NOT see "TestObject has left." — C doesn't send departure on @destroy
	if strings.Contains(bobOut, "has left") {
		t.Errorf("@dest THING: Bob should NOT see departure message (C compat), got:\n%s", bobOut)
	}
}

// ============================================================================
// Regression: DidIt O-messages prefixed with cause's name
// Bug: O-messages were sent without the cause's name, e.g. "arrives." instead
//      of "Wizard arrives."
// C TinyMUSH: did_it() prefixes O-messages with Name(player)
// ============================================================================

func TestDidIt_OMessageNamePrefix(t *testing.T) {
	env := newTestEnv(t)

	// Create a second player to observe
	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)

	// Set OSUCC (attr 1) on TestObject #2
	env.game.SetAttr(2, 1, "picks up the shiny object.")

	// Fire DidIt with cause=Wizard(#1), thing=TestObject(#2)
	env.game.DidIt(1, 2, 0, 1, 0) // only O-message

	bobOut := getOutput(bobDesc)
	if !strings.Contains(bobOut, "Wizard picks up the shiny object.") {
		t.Errorf("DidIt O-message: expected 'Wizard picks up the shiny object.', got:\n%s", bobOut)
	}
}

// ============================================================================
// Regression: BoolEval lock falls back from 'from' to 'thing'
// Bug: BoolEval only checked 'from' object for the attribute. If the eval lock
//      was on the exit itself (thing), it wouldn't find the attribute.
// C TinyMUSH: boolexp.c eval_boolexp() BOOLEXP_EVAL case tries 'from' first,
//             falls back to 'thing' (lines 302-310)
// ============================================================================

func TestBoolEval_FallbackToThing(t *testing.T) {
	env := newTestEnv(t)

	// Create an exit #6 with an eval lock that checks attr WEATHER on itself
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Gateway;gate",
		Location: 4, // destination = OtherRoom
		Contents: gamedb.Nothing,
		Exits:    0, // source = Room Zero
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeExit), 0, 0},
	}
	env.game.NextRef = 7

	// Add exit to Room Zero's exits chain
	env.game.DB.Objects[0].Exits = 6

	// Define custom attr WEATHER (attr 400) on the exit itself
	env.game.DB.AddAttrDef(400, "WEATHER", 0)
	env.game.SetAttr(6, 400, "1")

	// Create a BoolEval lock: evaluate attr 400 on thing, match against "1"
	lock := &gamedb.BoolExp{
		Type:   gamedb.BoolEval,
		Thing:  400, // attr number to evaluate
		StrVal: "1", // expected result
	}

	// Test with from=Nothing (so it must fall back to thing=exit #6)
	result := EvalBoolExp(env.game, 1, 6, gamedb.Nothing, lock, 0)
	if !result {
		t.Errorf("BoolEval fallback: expected true when attr is on thing (#6), got false")
	}

	// Change the attr value so it doesn't match
	env.game.SetAttr(6, 400, "0")
	result = EvalBoolExp(env.game, 1, 6, gamedb.Nothing, lock, 0)
	if result {
		t.Errorf("BoolEval fallback: expected false when attr value is '0', got true")
	}
}

func TestBoolEval_PrefersFrom(t *testing.T) {
	env := newTestEnv(t)

	// Attr 400 = "WEATHER"
	env.game.DB.AddAttrDef(400, "WEATHER", 0)

	// Set attr on both from (#2) and thing (#5)
	env.game.SetAttr(2, 400, "good") // from
	env.game.SetAttr(5, 400, "bad")  // thing

	lock := &gamedb.BoolExp{
		Type:   gamedb.BoolEval,
		Thing:  400,
		StrVal: "good",
	}

	// Should match 'from' (#2) first, not fall back to thing (#5)
	result := EvalBoolExp(env.game, 1, 5, 2, lock, 0)
	if !result {
		t.Errorf("BoolEval prefers from: expected true (from has 'good'), got false")
	}

	// Now check that thing's "bad" doesn't interfere
	lock.StrVal = "bad"
	result = EvalBoolExp(env.game, 1, 5, 2, lock, 0)
	if result {
		t.Errorf("BoolEval prefers from: expected false (from has 'good', not 'bad'), got true")
	}
}

// ============================================================================
// Regression: Alias.conf commands have priority over exits
// Bug: typing "l" matched exit "Lifts;lift;lif;li" prefix instead of the
//      "l" -> "look" alias from goTinyAlias.conf
// C TinyMUSH: cf_alias adds aliases to the same command hash table as built-in
//             commands, so they are checked before exits
// ============================================================================

func TestAliasConf_PriorityOverExits(t *testing.T) {
	env := newTestEnv(t)

	// Set room description for look output verification
	env.game.SetAttr(0, 6, "A plain test room.")

	// Create an exit named "Lifts;lift;lif;li" in Room Zero
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Lifts;lift;lif;li",
		Location: 4, // destination = OtherRoom
		Contents: gamedb.Nothing,
		Exits:    0, // source = Room Zero
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeExit), 0, 0},
	}
	env.game.DB.Objects[0].Exits = 6
	env.game.NextRef = 7

	// Register "l" as alias for look (simulating goTinyAlias.conf)
	lookCmd := env.game.Commands["look"]
	if lookCmd == nil {
		t.Fatal("look command not found in Commands map")
	}
	env.game.Commands["l"] = &Command{
		Name:    lookCmd.Name,
		Handler: lookCmd.Handler,
		IsAlias: true,
	}

	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "l")
	out := getOutput(env.player)

	// "l" should trigger look (show room name), NOT take exit
	if !strings.Contains(out, "Room Zero") {
		t.Errorf("'l' alias: expected look (Room Zero), but got:\n%s", out)
	}

	// Player should still be in Room Zero, not teleported to OtherRoom
	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 0 {
		t.Errorf("'l' alias: player moved to #%d, should still be in Room Zero (#0)", playerObj.Location)
	}
}

// ============================================================================
// Regression: @emit triggers AUDIBLE inward relay
// Bug: @emit in a room didn't relay messages into AUDIBLE containers in that
//      room, so players inside repair bays (AUDIBLE things) couldn't see
//      @emit messages from the room's $commands.
// C TinyMUSH: speech.c do_say SAY_EMIT uses notify_all_from_inside_speech
//             with MSG_F_CONTENTS flag to relay into AUDIBLE containers
// ============================================================================

func TestEmit_AudibleInwardRelay(t *testing.T) {
	env := newTestEnv(t)

	// Make Container (#5) AUDIBLE (FlagHearThru)
	env.game.DB.Objects[5].Flags[0] |= gamedb.FlagHearThru

	// Set LISTEN (attr 26) on Container to match everything
	env.game.SetAttr(5, 26, "*")

	// Move Bob (#3) inside Container (#5)
	env.game.RemoveFromContents(0, 3)
	env.game.DB.Objects[3].Location = 5
	env.game.AddToContents(5, 3)

	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)
	clearOutput(env.player)

	// @emit from Wizard (in Room Zero) — should relay into AUDIBLE Container
	DispatchCommand(env.game, env.player, "@emit Test broadcast to containers")

	bobOut := getOutput(bobDesc)
	if !strings.Contains(bobOut, "Test broadcast to containers") {
		t.Errorf("@emit AUDIBLE relay: Bob inside Container should see message, got:\n%s", bobOut)
	}
}

func TestEmit_AudibleInwardRelay_WithPrefix(t *testing.T) {
	env := newTestEnv(t)

	// Make Container (#5) AUDIBLE
	env.game.DB.Objects[5].Flags[0] |= gamedb.FlagHearThru

	// Set LISTEN on Container to match everything
	env.game.SetAttr(5, 26, "*")

	// Set INPREFIX (attr 89) on Container — use literal text (no brackets)
	env.game.SetAttr(5, 89, "From outside>")

	// Move Bob inside Container
	env.game.RemoveFromContents(0, 3)
	env.game.DB.Objects[3].Location = 5
	env.game.AddToContents(5, 3)

	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@emit Hello from outside")

	bobOut := getOutput(bobDesc)
	if !strings.Contains(bobOut, "From outside> Hello from outside") {
		t.Errorf("@emit AUDIBLE INPREFIX: expected 'From outside> Hello from outside', got:\n%s", bobOut)
	}
}

// ============================================================================
// Regression: @desc obj (no =) clears the attribute (C TinyMUSH behavior)
// Bug: @desc me (no =) returned usage error instead of clearing DESC.
// This broke sled FIXUP which runs "@desc me;@idesc me;..." to reset desc
// to inherited parent value after repairs.
// Verified on crystalmush.kydance.net: @desc me clears DESC.
// ============================================================================

func TestDescribe_NoEquals_ClearsAttr(t *testing.T) {
	env := newTestEnv(t)

	// Set a DESC on the room
	env.game.SetAttr(0, 6, "A custom description.")
	clearOutput(env.player)

	// Verify it's set
	text := env.game.GetAttrText(0, 6)
	if text != "A custom description." {
		t.Fatalf("setup: expected DESC set, got %q", text)
	}

	// @desc here (no =) should clear it
	DispatchCommand(env.game, env.player, "@describe here")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@describe no-equals: expected 'Set.', got: %s", out)
	}

	// DESC should now be empty (cleared)
	text = env.game.GetAttrText(0, 6)
	if text != "" {
		t.Errorf("@describe no-equals: expected empty DESC, got %q", text)
	}
}

func TestAttrSetter_NoEquals_ClearsAttr(t *testing.T) {
	env := newTestEnv(t)

	// Set SUCC (attr 4) on Wizard
	env.game.SetAttr(1, 4, "You succeed!")
	clearOutput(env.player)

	// @success me (no =) should clear it
	DispatchCommand(env.game, env.player, "@success me")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@success no-equals: expected 'Set.', got: %s", out)
	}

	text := env.game.GetAttrText(1, 4)
	if text != "" {
		t.Errorf("@success no-equals: expected empty attr, got %q", text)
	}
}

func TestIdesc_NoEquals_ClearsAttr(t *testing.T) {
	env := newTestEnv(t)

	// Set IDESC (attr 32) on Container
	env.game.SetAttr(5, 32, "Inside a container.")
	clearOutput(env.player)

	// @idesc #5 (no =) should clear it
	DispatchCommand(env.game, env.player, "@idesc #5")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@idesc no-equals: expected 'Set.', got: %s", out)
	}

	text := env.game.GetAttrText(5, 32)
	if text != "" {
		t.Errorf("@idesc no-equals: expected empty attr, got %q", text)
	}
}

// ============================================================================
// Controls: recursive owner check
// Bug: Objects owned by God/wizard couldn't set attributes on other players
// because Controls() didn't check if the object's OWNER would control the
// target. In C TinyMUSH, Controls() recursively checks Owner(player).
// This broke the economy system where Money Manager (#800, owned by God)
// executes "&credit_balance player = value" via $-commands.
// ============================================================================

func TestControls_OwnerInheritsGodControl(t *testing.T) {
	env := newTestEnv(t)

	// TestObject #2 is already a THING owned by God (#1)
	godThing := gamedb.DBRef(2)

	// Bob #3 is already a player owned by self (#3)
	bob := gamedb.DBRef(3)

	// God's THING should control Bob (via owner inheritance)
	if !Controls(env.game, godThing, bob) {
		t.Errorf("Controls(godThing=#%d, bob=#%d) = false, want true (God-owned thing should control players)", godThing, bob)
	}

	// Verify objSetVAttr works: God's THING can set attrs on Bob
	env.game.objSetVAttr(godThing, fmt.Sprintf("CREDIT_BALANCE #%d = 500", bob))
	attrNum := env.game.ResolveAttrNum("CREDIT_BALANCE")
	if attrNum < 0 {
		t.Fatalf("CREDIT_BALANCE not found in attr table")
	}
	got := env.game.GetAttrText(bob, attrNum)
	if got != "500" {
		t.Errorf("objSetVAttr by God-owned thing: got %q, want '500'", got)
	}
}

func TestControls_NonWizOwnedThingCannotControlOther(t *testing.T) {
	env := newTestEnv(t)

	// Create a THING #6 owned by Bob (#3, a regular player)
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Bob Thing",
		Location: 0,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    3, // owned by Bob
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0},
	}

	// Wizard #1 is a different player from Bob #3
	wizard := gamedb.DBRef(1)

	// A thing owned by Bob should NOT control the Wizard
	if Controls(env.game, 6, wizard) {
		t.Errorf("Controls(bobThing=#6, wizard=#%d) = true, want false (Bob's thing should not control wizard)", wizard)
	}
}

// ============================================================================
// @desc prefix matching with self-aliases
// Bug: "alias @describe @describe" in alias.conf overwrites the built-in
// @describe command with IsAlias=true, which causes prefix matching for
// "@desc" to skip it (prefix matching excludes aliases). Fixed by skipping
// self-aliases that would overwrite built-in commands.
// ============================================================================

func TestDescAbbreviation_PrefixMatch(t *testing.T) {
	env := newTestEnv(t)

	// Load alias config to reproduce the bug: "alias @describe @describe"
	// overwrites the built-in with IsAlias=true, breaking prefix matching.
	ac := &AliasConfig{
		CommandAliases: map[string]string{
			"@descr":    "@describe",
			"@descri":   "@describe",
			"@describ":  "@describe",
			"@describe": "@describe", // self-alias that caused the bug
		},
	}
	env.game.ApplyAliasConfig(ac)

	// "@desc" should prefix-match to "@describe" and set description
	d := makeTestDescriptor(t, env.game.Conns, 1) // Wizard
	DispatchCommand(env.game, d, "@desc #0=A test room description")

	got := env.game.GetAttrText(0, 6) // A_DESC = 6
	if got != "A test room description" {
		t.Errorf("@desc prefix match: got desc %q, want %q", got, "A test room description")
	}
}

// ============================================================================
// MatchObject: exact exit alias wins over word-prefix match in contents
// ============================================================================
// Bug: When room contents had an object with "2" as a word in its name (e.g.
// "Storage Carton Label: 2 L DRos"), it would word-prefix-match before an exit
// with "2" as an exact alias (e.g. "Table 2 Conveyor;Table 2;t2;2"). This
// broke softcode like nearby(me,2) which resolved to the carton instead of
// the exit. Fix: compare match quality across all scopes — exact alias match
// wins over word-prefix match regardless of scope order.
func TestMatchObject_ExitExactOverContentWordPrefix(t *testing.T) {
	env := newTestEnv(t)

	// Create a THING in Room Zero whose name contains "2" as a word.
	// This will get a word-prefix match (quality 1) for search term "2".
	env.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Storage Carton Label: 2 Large",
		Location: 0, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	// Add to room contents linked list
	env.game.DB.Objects[8].Next = env.game.DB.Objects[0].Contents
	env.game.DB.Objects[0].Contents = 8

	// Create an EXIT with "2" as an exact alias.
	// This should get an exact match (quality 2) for search term "2".
	env.game.DB.Objects[9] = &gamedb.Object{
		DBRef: 9, Name: "Table 2 Conveyor;t2;2",
		Location: 4, Contents: gamedb.Nothing, Exits: 0, // dest=#4, source=#0
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeExit), 0, 0},
	}
	// Add to room exits linked list
	oldExits := env.game.DB.Objects[0].Exits
	env.game.DB.Objects[0].Exits = 9
	env.game.DB.Objects[9].Next = oldExits

	// MatchObject("2") should return the exit (#9), not the carton (#8)
	result := env.game.MatchObject(1, "2")
	if result != 9 {
		t.Errorf("MatchObject('2'): got #%d, want #9 (exit with exact alias)", result)
	}
}

// ============================================================================
// Look command: MYOPIC flag hides dbrefs, non-MYOPIC shows dbrefs
// ============================================================================
func TestLook_MyopicHidesDbref(t *testing.T) {
	env := newTestEnv(t)

	// Create a thing the wizard owns
	env.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Test Widget",
		Location: 0, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	env.game.DB.Objects[8].Next = env.game.DB.Objects[0].Contents
	env.game.DB.Objects[0].Contents = 8

	d := makeTestDescriptor(t, env.game.Conns, 1) // Wizard

	// Non-MYOPIC wizard: look should show dbref
	clearOutput(d)
	env.game.ShowObject(d, 8)
	out := getOutput(d)
	if !strings.Contains(out, "(#8") {
		t.Errorf("non-MYOPIC look: expected dbref in output, got: %s", out)
	}

	// Set MYOPIC on wizard
	env.game.DB.Objects[1].Flags[0] |= gamedb.FlagMyopic
	clearOutput(d)
	env.game.ShowObject(d, 8)
	out = getOutput(d)
	if strings.Contains(out, "(#8") {
		t.Errorf("MYOPIC look: expected NO dbref in output, got: %s", out)
	}
	if !strings.Contains(out, "Test Widget") {
		t.Errorf("MYOPIC look: expected name in output, got: %s", out)
	}

	// Clean up
	env.game.DB.Objects[1].Flags[0] &^= gamedb.FlagMyopic
}

// ============================================================================
// resolveDBRef: bare name does NOT global-scan players (only *name does)
// ============================================================================
// In C TinyMUSH, match_player only fires for *name syntax. Bare names
// resolve via contents/exits/inventory, not global player scan.
func TestResolveDBRef_BareNameNoPlayerScan(t *testing.T) {
	env := newEvalTestEnv(t)

	// Give Bob (player #3) an alias "n" which collides with exit "North;n" (#7)
	env.game.DB.Objects[3].Attrs = append(env.game.DB.Objects[3].Attrs,
		gamedb.Attribute{Number: 58, Value: "\x013:0:n"}, // A_ALIAS = 58
	)

	// Bare "n" should match the exit, not the player
	result := env.eval("[num(n)]")
	if result != "#7" {
		t.Errorf("num(n): got %s, want #7 (exit North;n)", result)
	}

	// *Bob should still find the player
	result = env.eval("[num(*Bob)]")
	if result != "#3" {
		t.Errorf("num(*Bob): got %s, want #3 (player Bob)", result)
	}
}

// ============================================================================
// Regression: Exit alias must beat command alias in dispatch
// Bug: "alias sa say" in goTinyAlias.conf made "sa" fire say instead of
//      exit "Sorting Area;sa". C TinyMUSH checks exits BEFORE command table.
// ============================================================================

func TestExitAlias_BeatsCommandAlias(t *testing.T) {
	env := newTestEnv(t)

	// Create exit "Sorting;sa" from Room Zero (#0) to OtherRoom (#4)
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Sorting;sa",
		Location: 4, // destination
		Contents: gamedb.Nothing,
		Exits:    0, // source
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeExit), 0, 0},
	}
	env.game.DB.Objects[0].Exits = 6
	env.game.NextRef = 7

	// Register "sa" as alias for say (simulating goTinyAlias.conf)
	sayCmd := env.game.Commands["say"]
	if sayCmd == nil {
		t.Fatal("say command not found")
	}
	env.game.Commands["sa"] = &Command{
		Name:    sayCmd.Name,
		Handler: sayCmd.Handler,
		IsAlias: true,
	}

	clearOutput(env.player)
	DispatchCommand(env.game, env.player, "sa")
	out := getOutput(env.player)

	// Player should have moved to OtherRoom, not said anything
	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 4 {
		t.Errorf("'sa' should take exit to OtherRoom (#4), but player is at #%d; output: %s",
			playerObj.Location, out)
	}
}

// ============================================================================
// Brace-wrapped @force body: {cmd1;cmd2} must dispatch both commands correctly.
// Bug: EvStrip stripped braces from evaluated result, then the brace-wrapped
// group handler did evaluated[1:len-1] which double-stripped, losing the first
// and last characters of the actual content.
// ============================================================================

func TestForce_BraceWrappedBody(t *testing.T) {
	env := newTestEnv(t)

	// Queue the forced command — @force puts {cmd1;cmd2} in the queue
	env.game.DoForce(1, 1, "{think ALPHA;think BETA}")

	// Process one tick to execute the forced entry
	env.game.ProcessQueue()

	out := getOutput(env.player)
	if !strings.Contains(out, "ALPHA") {
		t.Errorf("expected 'ALPHA' in output, got: %s", out)
	}
	if !strings.Contains(out, "BETA") {
		t.Errorf("expected 'BETA' in output, got: %s", out)
	}
}

// ============================================================================
// Bug: "enter" matches inventory before room contents
// C TinyMUSH do_enter uses match_neighbor() (room contents only).
// Go was using MatchObject which searched inventory AND room contents,
// so "enter nyki's" would match "Nyki's Crystal Cutter" in inventory
// before "Nyki's Sled" in the room.
// ============================================================================

func TestEnter_RoomOnlyNotInventory(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Create an inventory item named "Nyki's Crystal Cutter" carried by wizard
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Nyki's Crystal Cutter",
		Location: 1, // carried by Wizard #1
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0},
	}
	// Add to wizard's inventory
	env.game.DB.Objects[1].Contents = 6

	// Create a room object named "Nyki's Sled" in Room #0 with ENTER_OK
	env.game.DB.Objects[7] = &gamedb.Object{
		DBRef:    7,
		Name:     "Nyki's Sled",
		Location: 0, // in Room Zero
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing) | gamedb.FlagEnterOK, 0, 0},
	}
	// Add to room contents
	roomContents := env.game.DB.SafeContents(0)
	env.game.AddToContents(0, 7)
	_ = roomContents

	// "enter Nyki's" should match the room object (sled), NOT the inventory item
	cmdEnter(env.game, env.player, "Nyki's", nil)
	out := getOutput(env.player)

	// Should enter the sled (room object), not complain about inventory item
	if strings.Contains(out, "can't enter") {
		t.Errorf("enter matched inventory instead of room: %s", out)
	}
	if !strings.Contains(out, "You enter Nyki's Sled") {
		t.Errorf("expected to enter Nyki's Sled, got: %s", out)
	}
}

func TestEnter_InventoryItemsIgnored(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Create ONLY an inventory item, no matching room object
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Gadget",
		Location: 1, // carried by Wizard #1
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing) | gamedb.FlagEnterOK, 0, 0},
	}
	env.game.DB.Objects[1].Contents = 6

	// "enter Gadget" should NOT find inventory items
	cmdEnter(env.game, env.player, "Gadget", nil)
	out := getOutput(env.player)

	if strings.Contains(out, "You enter") {
		t.Errorf("enter should not match inventory items, got: %s", out)
	}
	if !strings.Contains(out, "don't see that") {
		t.Errorf("expected 'don't see that here', got: %s", out)
	}
}

// ============================================================================
// Bug: drop does not check A_LDROP (drop lock)
// C TinyMUSH checks could_doit(player, thing, A_LDROP) before allowing drop.
// Go was skipping this check entirely, so locked cartons would show the wrong
// failure message (secured_type instead of "carton is full").
// ============================================================================

func TestDrop_ChecksDropLock(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Give the wizard an object with a drop lock that will FAIL
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Locked Carton",
		Location: 1, // carried by Wizard #1
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0},
		Attrs: []gamedb.Attribute{
			// A_LDROP = 86 — lock to #-1 (nobody) so it always fails
			{Number: 86, Value: "1:#-1"},
			// A_DFAIL = 135 — custom fail message
			{Number: 135, Value: "1:The carton is full!"},
		},
	}
	env.game.DB.Objects[1].Contents = 6

	cmdDrop(env.game, env.player, "Locked Carton", nil)
	out := getOutput(env.player)

	// Should show the drop fail message, NOT actually drop it
	if strings.Contains(out, "You drop") {
		t.Errorf("drop should have been blocked by drop lock, got: %s", out)
	}
	if !strings.Contains(out, "carton is full") {
		t.Errorf("expected drop fail message 'The carton is full!', got: %s", out)
	}

	// Object should still be in inventory (location unchanged)
	if env.game.DB.Objects[6].Location != 1 {
		t.Errorf("object should still be in wizard's inventory, location=%d", env.game.DB.Objects[6].Location)
	}
}

// ============================================================================
// Bug: $commands on parent objects don't fire for child objects in room
// C TinyMUSH sets HAS_COMMANDS on child objects during db load when a parent
// has $-commands. Go was only checking the object's own HAS_COMMANDS flag,
// so objects like cartons (parent has $commands, child doesn't have flag)
// would not respond to commands like "show packed crystals".
// ============================================================================

func TestDollarCommand_InheritedFromParent(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Create parent object #6 with HAS_COMMANDS and a $command
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Parent Carton",
		Location: gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), int(gamedb.Flag2HasCommands), 0},
		Attrs: []gamedb.Attribute{
			// $show packed crystals: @pemit %#=PACKED_OUTPUT
			{Number: 200, Value: "\x011:0:$show packed crystals:@pemit %#=PACKED_OUTPUT"},
		},
	}

	// Create child object #7 in room — has parent #6, but NO HAS_COMMANDS flag itself
	env.game.DB.Objects[7] = &gamedb.Object{
		DBRef:    7,
		Name:     "Carton 1",
		Location: 0, // in Room Zero
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   6, // parent is #6 which has HAS_COMMANDS
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0}, // NO HAS_COMMANDS
	}
	env.game.AddToContents(0, 7)

	// Verify parent chain is correct
	child := env.game.DB.Objects[7]
	if child.Parent != 6 {
		t.Fatalf("child parent=%d, want 6", child.Parent)
	}
	parent := env.game.DB.Objects[6]
	if !parent.HasFlag2(gamedb.Flag2HasCommands) {
		t.Fatal("parent should have HAS_COMMANDS flag")
	}

	// Verify child is in room contents
	contents := env.game.DB.SafeContents(0)
	found7 := false
	for _, ref := range contents {
		if ref == 7 {
			found7 = true
		}
	}
	if !found7 {
		t.Fatalf("child #7 not in room contents: %v", contents)
	}

	// Verify hasCommandsFlag works for child
	if !env.game.hasCommandsFlag(7) {
		t.Fatal("hasCommandsFlag(7) should be true via parent inheritance")
	}

	// "show packed crystals" should match via parent's $command
	matched := env.game.MatchDollarCommands(1, 1, "show packed crystals")
	if !matched {
		t.Error("$command on parent should match for child object without HAS_COMMANDS flag")
	}

	// Process queue to execute the matched command
	env.game.ProcessQueue()
	out := getOutput(env.player)

	if !strings.Contains(out, "PACKED_OUTPUT") {
		t.Errorf("expected PACKED_OUTPUT from parent $command, got: %s", out)
	}
}

func TestDrop_NoDropLockAllowsDrop(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Give the wizard an object with NO drop lock
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Normal Item",
		Location: 1, // carried by Wizard #1
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0},
	}
	env.game.DB.Objects[1].Contents = 6

	cmdDrop(env.game, env.player, "Normal Item", nil)
	out := getOutput(env.player)

	// Should drop normally
	if !strings.Contains(out, "You drop Normal Item") {
		t.Errorf("expected normal drop, got: %s", out)
	}

	// Object should now be in room
	if env.game.DB.Objects[6].Location != 0 {
		t.Errorf("object should be in room #0 after drop, location=%d", env.game.DB.Objects[6].Location)
	}
}

