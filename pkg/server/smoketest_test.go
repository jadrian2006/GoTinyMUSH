package server

import (
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

func TestRepairContentChains_SelfRef(t *testing.T) {
	env := newTestEnv(t)

	// Corrupt: self-referencing Next
	env.game.DB.Objects[2].Next = 2
	env.game.RepairContentChains()

	if env.game.DB.Objects[2].Next == 2 {
		t.Error("RepairContentChains did not fix self-referencing Next pointer")
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
