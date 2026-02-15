package server

import (
	"strings"
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// evalTestEnv sets up a Game + EvalContext with a richer test database.
// Objects:
//   #0 Room Zero (ROOM) - parent=#6
//   #1 Wizard (PLAYER, WIZARD) in #0, home=#0
//   #2 TestObject (THING) in #0, owner=#1, parent=#6
//   #3 Bob (PLAYER) in #0, home=#0
//   #4 Other Room (ROOM)
//   #5 Container (THING, ENTER_OK) in #0
//   #6 Parent Room (ROOM) - has DESC
//   #7 North Exit (EXIT) in #0, links to #4, name "North;n"
type evalTestEnv struct {
	game *Game
	ctx  *eval.EvalContext
}

func newEvalTestEnv(t *testing.T) *evalTestEnv {
	t.Helper()
	db := gamedb.NewDatabase()

	// Room #0
	db.Objects[0] = &gamedb.Object{
		DBRef: 0, Name: "Room Zero",
		Location: gamedb.Nothing, Contents: 1, Exits: 7,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: 6, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeRoom), 0, 0},
	}

	// Wizard #1
	db.Objects[1] = &gamedb.Object{
		DBRef: 1, Name: "Wizard",
		Location: 0, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: 0, Next: 2,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Pennies: 1000,
		Flags:   [3]int{int(gamedb.TypePlayer) | gamedb.FlagWizard, 0, 0},
	}

	// TestObject #2
	db.Objects[2] = &gamedb.Object{
		DBRef: 2, Name: "TestObject",
		Location: 0, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: 3,
		Owner: 1, Parent: 6, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}

	// Bob #3
	db.Objects[3] = &gamedb.Object{
		DBRef: 3, Name: "Bob",
		Location: 0, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: 0, Next: 5,
		Owner: 3, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Pennies: 100,
		Flags:   [3]int{int(gamedb.TypePlayer), 0, 0},
	}

	// Other Room #4
	db.Objects[4] = &gamedb.Object{
		DBRef: 4, Name: "Other Room",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeRoom), 0, 0},
	}

	// Container #5
	db.Objects[5] = &gamedb.Object{
		DBRef: 5, Name: "Container",
		Location: 0, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing) | gamedb.FlagEnterOK, 0, 0},
	}

	// Parent Room #6
	db.Objects[6] = &gamedb.Object{
		DBRef: 6, Name: "Parent Room",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeRoom), 0, 0},
		Attrs: []gamedb.Attribute{
			{Number: 6, Value: "\x011:0:Inherited desc"},
		},
	}

	// North Exit #7 - from Room Zero to Other Room
	// TinyMUSH exit semantics: Location = destination, Exits = source room
	db.Objects[7] = &gamedb.Object{
		DBRef: 7, Name: "North;n",
		Location: 4, Contents: gamedb.Nothing, Exits: 0,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeExit), 0, 0},
	}

	// Set some attrs on Wizard for testing v(), get(), hasattr()
	// VA (attr 95) = "hello from VA"
	db.Objects[1].Attrs = append(db.Objects[1].Attrs,
		gamedb.Attribute{Number: 95, Value: "\x011:0:hello from VA"},
	)

	// Define a user attr TESTFN on object #2 for u()/map()/filter()/fold()
	db.AddAttrDef(256, "TESTFN", 0)
	db.Objects[2].Attrs = append(db.Objects[2].Attrs,
		gamedb.Attribute{Number: 256, Value: "\x011:0:[ucstr(%0)]"},
	)

	// DOUBLE attr on #2 for fold testing: [add(%0,%1)]
	db.AddAttrDef(257, "ADDFN", 0)
	db.Objects[2].Attrs = append(db.Objects[2].Attrs,
		gamedb.Attribute{Number: 257, Value: "\x011:0:[add(%0,%1)]"},
	)

	// FILTERGT2 on #2: [gt(%0,2)]
	db.AddAttrDef(258, "FILTERGT2", 0)
	db.Objects[2].Attrs = append(db.Objects[2].Attrs,
		gamedb.Attribute{Number: 258, Value: "\x011:0:[gt(%0,2)]"},
	)

	// DESC on Wizard
	db.Objects[1].Attrs = append(db.Objects[1].Attrs,
		gamedb.Attribute{Number: 6, Value: "\x011:0:A powerful wizard."},
	)

	// Custom attr MY_ATTR on Wizard
	db.AddAttrDef(300, "MY_ATTR", 0)
	db.Objects[1].Attrs = append(db.Objects[1].Attrs,
		gamedb.Attribute{Number: 300, Value: "\x011:0:custom value"},
	)

	conns := NewConnManager()
	g := &Game{
		DB:       db,
		Conns:    conns,
		Commands: InitCommands(),
		Queue:    NewCommandQueue(),
		NextRef:  8,
	}

	ctx := MakeEvalContextWithGame(g, 1, func(c *eval.EvalContext) {
		functions.RegisterAll(c)
	})

	return &evalTestEnv{game: g, ctx: ctx}
}

// evalExprTest evaluates a MUSH expression and returns the result.
func (e *evalTestEnv) eval(expr string) string {
	e.ctx.FuncInvkCtr = 0
	e.ctx.FuncNestLev = 0
	return e.ctx.Exec(expr, eval.EvFCheck|eval.EvEval, nil)
}

// --- Object Functions ---

func TestFnName(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[name(#1)]":  "Wizard",
		"[name(#0)]":  "Room Zero",
		"[name(#7)]":  "North", // exit returns first alias
		"[name(me)]":  "Wizard",
		"[name(#99)]": "#-1 NOT FOUND",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("name: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnNumLocOwner(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[num(me)]":     "#1",
		"[num(#0)]":     "#0",
		"[loc(#1)]":     "#0",
		"[loc(me)]":     "#0",
		"[owner(#2)]":   "#1",
		"[owner(#3)]":   "#3",
		"[owner(me)]":   "#1",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnType(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[type(#0)]": "ROOM",
		"[type(#1)]": "PLAYER",
		"[type(#2)]": "THING",
		"[type(#7)]": "EXIT",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("type: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnFlags(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[flags(#1)]")
	if !strings.Contains(got, "P") || !strings.Contains(got, "W") {
		t.Errorf("flags(#1) = %q, expected P and W", got)
	}
	got = e.eval("[flags(#0)]")
	if !strings.Contains(got, "R") {
		t.Errorf("flags(#0) = %q, expected R", got)
	}
	got = e.eval("[flags(#5)]")
	if !strings.Contains(got, "e") {
		t.Errorf("flags(#5) = %q, expected 'e' for ENTER_OK", got)
	}
}

func TestFnHasflag(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[hasflag(#1,WIZARD)]":   "1",
		"[hasflag(#1,DARK)]":     "0",
		"[hasflag(#1,PLAYER)]":   "1",
		"[hasflag(#0,ROOM)]":     "1",
		"[hasflag(#2,THING)]":    "1",
		"[hasflag(#5,ENTER_OK)]": "1",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("hasflag: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnHastype(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[hastype(#1,PLAYER)]": "1",
		"[hastype(#1,ROOM)]":   "0",
		"[hastype(#0,ROOM)]":   "1",
		"[hastype(#2,THING)]":  "1",
		"[hastype(#7,EXIT)]":   "1",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("hastype: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnHasattr(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[hasattr(#1,DESC)]":    "1",
		"[hasattr(#1,MY_ATTR)]": "1",
		"[hasattr(#1,NOSUCH)]":  "0",
		"[hasattr(#3,DESC)]":    "0",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("hasattr: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnGetXget(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[get(#1/DESC)]":       "A powerful wizard.",
		"[get(#1/MY_ATTR)]":    "custom value",
		"[xget(#1,DESC)]":      "A powerful wizard.",
		"[xget(#1,MY_ATTR)]":   "custom value",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("get/xget: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnV(t *testing.T) {
	e := newEvalTestEnv(t)
	// v(a) should return VA (attr 95) on the executor (#1)
	got := e.eval("[v(a)]")
	if got != "hello from VA" {
		t.Errorf("v(a) = %q, want 'hello from VA'", got)
	}
	// v(desc) should return DESC
	got = e.eval("[v(desc)]")
	if got != "A powerful wizard." {
		t.Errorf("v(desc) = %q, want 'A powerful wizard.'", got)
	}
}

func TestFnS(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[s(hello world)]")
	if got != "hello world" {
		t.Errorf("s() = %q, want 'hello world'", got)
	}
}

func TestFnConExitNext(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[con(#0)]":  "#1",   // Room Zero's first content is Wizard
		"[exit(#0)]": "#7",   // Room Zero has exit #7
		"[next(#1)]": "#2",   // Wizard.Next = #2
		"[next(#2)]": "#3",   // TestObject.Next = #3
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnLcon(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[lcon(#0)]")
	// Room Zero contents: #1 -> #2 -> #3 -> #5
	if !strings.Contains(got, "#1") || !strings.Contains(got, "#2") ||
		!strings.Contains(got, "#3") || !strings.Contains(got, "#5") {
		t.Errorf("lcon(#0) = %q, expected #1 #2 #3 #5", got)
	}
}

func TestFnLexits(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[lexits(#0)]")
	if got != "#7" {
		t.Errorf("lexits(#0) = %q, want '#7'", got)
	}
}

func TestFnLattr(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[lattr(#1)]")
	// Wizard has attrs: 95 (VA), 6 (DESC), 300 (MY_ATTR)
	if !strings.Contains(got, "MY_ATTR") {
		t.Errorf("lattr(#1) = %q, expected MY_ATTR in list", got)
	}
}

func TestFnNattr(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[nattr(#1)]")
	// Wizard has 3 attrs
	if got != "3" {
		t.Errorf("nattr(#1) = %q, want '3'", got)
	}
}

func TestFnHomeParentZone(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[home(#1)]":   "#0",
		"[parent(#0)]": "#6",  // Room Zero's parent is #6
		"[parent(#1)]": "#-1", // Wizard has no parent
		"[zone(#0)]":   "#-1",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnControls(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[controls(#1,#0)]": "1", // wizard controls everything
		"[controls(#1,#3)]": "1",
		"[controls(#3,#1)]": "0", // Bob doesn't control Wizard
		"[controls(#3,#3)]": "1", // self-control
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("controls: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnRoom(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[room(#1)]": "#0", // Wizard is in Room Zero
		"[room(#0)]": "#0", // Room Zero is itself
		"[room(#2)]": "#0", // TestObject in Room Zero
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("room: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnRloc(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[rloc(#1)]")
	if got != "#0" {
		t.Errorf("rloc(#1) = %q, want '#0'", got)
	}
}

func TestFnNearby(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[nearby(#1,#2)]": "1", // same room
		"[nearby(#1,#0)]": "1", // player is IN #0
		"[nearby(#1,#4)]": "0", // different rooms
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("nearby: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnChildren(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[children(#6)]")
	// #0 and #2 have parent #6
	if !strings.Contains(got, "#0") || !strings.Contains(got, "#2") {
		t.Errorf("children(#6) = %q, expected #0 and #2", got)
	}
}

func TestFnLparent(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[lparent(#0)]")
	// #0 -> parent #6 -> no parent
	if !strings.Contains(got, "#0") || !strings.Contains(got, "#6") {
		t.Errorf("lparent(#0) = %q, expected '#0 #6'", got)
	}
}

func TestFnLocate(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[locate(#1,me)]":         "#1",
		"[locate(#1,here)]":       "#0",
		"[locate(#1,#2)]":         "#2",
		"[locate(#1,TestObject)]": "#2",
		"[locate(#1,nosuch)]":     "#-1 NOT FOUND",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("locate: %s = %q, want %q", expr, got, want)
		}
	}
}

func TestFnEntrances(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[entrances(#4)]")
	// Exit #7 links to #4
	if !strings.Contains(got, "#7") {
		t.Errorf("entrances(#4) = %q, expected '#7'", got)
	}
}

func TestFnElock(t *testing.T) {
	e := newEvalTestEnv(t)
	// Player #1 is a wizard, so CouldDoIt always returns true (wizards pass locks)
	got := e.eval("[elock(#0,#1)]")
	if got != "1" {
		t.Errorf("elock = %q, want '1'", got)
	}
	// Test with non-wizard player #3 (Bob) — no enter lock set, so passes
	got = e.eval("[elock(#0,#3)]")
	if got != "1" {
		t.Errorf("elock(non-wiz, no lock) = %q, want '1'", got)
	}
}

func TestFnDefault(t *testing.T) {
	e := newEvalTestEnv(t)
	// default with existing attr - returns attr value
	got := e.eval("[default(#1/DESC,fallback)]")
	if got != "A powerful wizard." {
		t.Errorf("default existing = %q, want 'A powerful wizard.'", got)
	}
	// default with missing attr - returns fallback
	got = e.eval("[default(#1/NOSUCH,fallback)]")
	if got != "fallback" {
		t.Errorf("default missing = %q, want 'fallback'", got)
	}
}

func TestFnObjeval(t *testing.T) {
	e := newEvalTestEnv(t)
	// objeval evaluates as another object
	got := e.eval("[objeval(#3,[num(me)])]")
	if got != "#3" {
		t.Errorf("objeval = %q, want '#3'", got)
	}
}

// --- U / ULOCAL ---

func TestFnU(t *testing.T) {
	e := newEvalTestEnv(t)
	// u(#2/TESTFN, hello) should run [ucstr(%0)] with %0=hello -> HELLO
	got := e.eval("[u(#2/TESTFN,hello)]")
	if got != "HELLO" {
		t.Errorf("u() = %q, want 'HELLO'", got)
	}
}

func TestFnUlocal(t *testing.T) {
	e := newEvalTestEnv(t)
	// Set a register, call ulocal (which should preserve registers)
	got := e.eval("[setq(0,original)][ulocal(#2/TESTFN,test)][r(0)]")
	if !strings.Contains(got, "original") {
		t.Errorf("ulocal register preservation: got %q, expected 'original' in output", got)
	}
}

// --- Map / Filter / Fold ---

func TestFnMap(t *testing.T) {
	e := newEvalTestEnv(t)
	// map(#2/TESTFN, a b c) should call TESTFN for each element
	got := e.eval("[map(#2/TESTFN,a b c)]")
	if got != "A B C" {
		t.Errorf("map() = %q, want 'A B C'", got)
	}
}

func TestFnFilter(t *testing.T) {
	e := newEvalTestEnv(t)
	// filter(#2/FILTERGT2, 1 2 3 4 5) should keep elements where gt(%0,2)=1
	got := e.eval("[filter(#2/FILTERGT2,1 2 3 4 5)]")
	if got != "3 4 5" {
		t.Errorf("filter() = %q, want '3 4 5'", got)
	}
}

func TestFnFold(t *testing.T) {
	e := newEvalTestEnv(t)
	// fold(#2/ADDFN, 1 2 3 4) should compute 1+2=3, 3+3=6, 6+4=10
	got := e.eval("[fold(#2/ADDFN,1 2 3 4)]")
	if got != "10" {
		t.Errorf("fold() = %q, want '10'", got)
	}
}

// --- Iteration: itext, inum, ilev ---

func TestFnItextInum(t *testing.T) {
	e := newEvalTestEnv(t)
	// iter with itext(0) and inum(0)
	got := e.eval("[iter(x y z,[itext(0)]-[inum(0)])]")
	if got != "x-0 y-1 z-2" {
		t.Errorf("itext/inum = %q, want 'x-0 y-1 z-2'", got)
	}
}

func TestFnIlev(t *testing.T) {
	e := newEvalTestEnv(t)
	// ilev returns current loop nesting level (0-based in the loop)
	got := e.eval("[iter(a,[ilev()])]")
	if got != "0" {
		t.Errorf("ilev = %q, want '0'", got)
	}
}

func TestFnParse(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[parse(a b c,[strlen(##)])]")
	if got != "1 1 1" {
		t.Errorf("parse() = %q, want '1 1 1'", got)
	}
}

// --- Formatting ---

func TestFnWrap(t *testing.T) {
	e := newEvalTestEnv(t)
	// wrap with width 10 should break lines
	got := e.eval("[wrap(hello world how are you,10)]")
	if !strings.Contains(got, "\r\n") || !strings.Contains(got, "hello") {
		t.Errorf("wrap() = %q, expected line breaks", got)
	}
}

func TestFnColumns(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[columns(a b c d,10, ,40)]")
	if !strings.Contains(got, "a") || !strings.Contains(got, "d") {
		t.Errorf("columns() = %q, expected items", got)
	}
}

// --- Side-effect Functions ---

func TestFnPemitNotification(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[pemit(#1,hello)]")
	if len(e.ctx.Notifications) == 0 {
		t.Errorf("pemit: expected notification, got none")
	} else {
		n := e.ctx.Notifications[0]
		if n.Target != 1 || n.Message != "hello" {
			t.Errorf("pemit: notification = %+v", n)
		}
	}
}

func TestFnRemitNotification(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[remit(#0,broadcast)]")
	if len(e.ctx.Notifications) == 0 {
		t.Errorf("remit: expected notification, got none")
	}
}

func TestFnThinkNotification(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[think(thinking...)]")
	if len(e.ctx.Notifications) == 0 {
		t.Errorf("think: expected notification, got none")
	} else {
		if e.ctx.Notifications[0].Target != 1 {
			t.Errorf("think: should target self (#1)")
		}
	}
}

// --- Misc Functions ---

func TestFnRand(t *testing.T) {
	e := newEvalTestEnv(t)
	// rand(10) should produce 0-9
	for i := 0; i < 20; i++ {
		got := e.eval("[rand(10)]")
		if got < "0" || got > "9" {
			t.Errorf("rand(10) = %q, expected 0-9", got)
		}
	}
}

func TestFnDie(t *testing.T) {
	e := newEvalTestEnv(t)
	// die(2,6) = 2d6, should be 2-12
	for i := 0; i < 20; i++ {
		got := e.eval("[die(2,6)]")
		n := 0
		for _, ch := range got {
			if ch >= '0' && ch <= '9' {
				n = n*10 + int(ch-'0')
			}
		}
		if n < 2 || n > 12 {
			t.Errorf("die(2,6) = %q (%d), expected 2-12", got, n)
		}
	}
}

func TestFnTimeSecs(t *testing.T) {
	e := newEvalTestEnv(t)
	// time() should return something non-empty
	got := e.eval("[time()]")
	if got == "" {
		t.Errorf("time() returned empty")
	}
	// secs() should return a number
	got = e.eval("[secs()]")
	if got == "" || got[0] < '0' || got[0] > '9' {
		t.Errorf("secs() = %q, expected number", got)
	}
}

func TestFnConvsecs(t *testing.T) {
	e := newEvalTestEnv(t)
	// convsecs(0) should be epoch time string (may be Dec 31 1969 or Jan 01 1970 depending on timezone)
	got := e.eval("[convsecs(0)]")
	if got == "" || got == "#-1 INVALID ARGUMENT" {
		t.Errorf("convsecs(0) = %q", got)
	}
	if !strings.Contains(got, "1970") && !strings.Contains(got, "1969") {
		t.Errorf("convsecs(0) = %q, expected epoch-era date", got)
	}
}

func TestFnTimefmt(t *testing.T) {
	e := newEvalTestEnv(t)
	// Use %%Y so the MUSH evaluator converts %% to literal %, giving timefmt "%Y"
	// Use epoch 86400 (Jan 2 1970 UTC) to avoid timezone edge cases around epoch 0
	got := e.eval("[timefmt(%%Y,86400)]")
	if got != "1970" {
		t.Errorf("timefmt(%%%%Y,86400) = %q, want '1970'", got)
	}
}

func TestFnVersionMudname(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[version()]")
	if got != VersionString() {
		t.Errorf("version() = %q, want %q", got, VersionString())
	}
	got = e.eval("[mudname()]")
	if got != "GoTinyMUSH" {
		t.Errorf("mudname() = %q", got)
	}
}

func TestFnValid(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[valid(attrname,MY_ATTR)]":   "1",
		"[valid(attrname,bad name)]":  "0",
		"[valid(objectname,Foo)]":     "1",
		"[valid(objectname,#123)]":    "0",
		"[valid(playername,Bob)]":     "1",
		"[valid(playername,has;semi)]": "0",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("valid: %s = %q, want %q", expr, got, want)
		}
	}
}

// --- ANSI ---

func TestFnAnsi(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[ansi(r,hello)]")
	if !strings.Contains(got, "\033[31m") || !strings.Contains(got, "hello") || !strings.Contains(got, "\033[0m") {
		t.Errorf("ansi(r,hello) = %q, expected ANSI red codes", got)
	}
}

func TestFnStripansi(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[stripansi([ansi(r,hello)])]")
	if got != "hello" {
		t.Errorf("stripansi = %q, want 'hello'", got)
	}
}

// --- Ljust / Rjust / Center ---

func TestFnLjust(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[ljust(hi,6)]")
	if len(got) != 6 || !strings.HasPrefix(got, "hi") {
		t.Errorf("ljust(hi,6) = %q (len=%d), want 'hi    '", got, len(got))
	}
}

func TestFnRjust(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[rjust(hi,6)]")
	if len(got) != 6 || !strings.HasSuffix(got, "hi") {
		t.Errorf("rjust(hi,6) = %q (len=%d), want '    hi'", got, len(got))
	}
}

func TestFnCenter(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[center(hi,8)]")
	if len(got) != 8 || !strings.Contains(got, "hi") {
		t.Errorf("center(hi,8) = %q (len=%d)", got, len(got))
	}
}

// --- Escape / Secure ---

func TestFnEscape(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[escape(hello)]")
	// First char always gets backslash prepended
	if !strings.HasPrefix(got, "\\h") {
		t.Errorf("escape(hello) = %q, expected \\h prefix", got)
	}
}

func TestFnSecure(t *testing.T) {
	e := newEvalTestEnv(t)
	// secure() escapes special chars with backslashes; test with plain text (no special chars)
	// should pass through unchanged
	got := e.eval("[secure(hello)]")
	if got != "hello" {
		t.Errorf("secure(hello) = %q, want 'hello'", got)
	}
	// Use strlen to verify escaping adds backslashes (brackets get evaluated by parser,
	// so we test indirectly via the batch tests in eval_basic.txt)
	got = e.eval("[strlen([secure(hello)])]")
	if got != "5" {
		t.Errorf("strlen(secure(hello)) = %q, want '5'", got)
	}
}

// --- Delete ---

func TestFnDelete(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[delete(abcdef,2,3)]": "abf",
		"[delete(abcdef,0,2)]": "cdef",
		"[delete(hello,5,1)]":  "hello", // start >= len
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("delete: %s = %q, want %q", expr, got, want)
		}
	}
}

// --- Scramble / Shuffle (non-deterministic, just test length/membership) ---

func TestFnScramble(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[scramble(abcde)]")
	if len(got) != 5 {
		t.Errorf("scramble: expected 5 chars, got %d: %q", len(got), got)
	}
	// All original chars should be present
	for _, ch := range "abcde" {
		if !strings.ContainsRune(got, ch) {
			t.Errorf("scramble: missing char '%c' in %q", ch, got)
		}
	}
}

func TestFnShuffle(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[shuffle(a b c d e)]")
	words := strings.Fields(got)
	if len(words) != 5 {
		t.Errorf("shuffle: expected 5 words, got %d: %q", len(words), got)
	}
}

// --- Splice ---

func TestFnSplice(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[splice(a b c,x y z,b)]")
	if got != "a y c" {
		t.Errorf("splice = %q, want 'a y c'", got)
	}
}

// --- Itemize ---

func TestFnItemize(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[itemize(a)]":         "a",
		"[itemize(a b)]":       "a and b",
		"[itemize(a b c)]":     "a, b, and c",
		"[itemize(a b c d)]":   "a, b, c, and d",
		"[itemize(a b c, ,or)]": "a, b, or c",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("itemize: %s = %q, want %q", expr, got, want)
		}
	}
}

// --- Lnum ---

func TestFnLnum(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[lnum(5)]":   "0 1 2 3 4",
		"[lnum(2,6)]": "2 3 4 5",
		"[lnum(5,2)]": "5 4 3",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("lnum: %s = %q, want %q", expr, got, want)
		}
	}
}

// --- Replace ---

func TestFnReplace(t *testing.T) {
	e := newEvalTestEnv(t)
	got := e.eval("[replace(a b c d,2 4,X Y)]")
	if got != "a X c Y" {
		t.Errorf("replace() = %q, want 'a X c Y'", got)
	}
}

// --- Complex multi-function expressions ---

func TestComplexNested(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[if([eq([add(1,2)],3)],math works,broken)]":       "math works",
		"[setq(0,hello)][ucstr([r(0)])]":                    "HELLO",
		"[iter(1 2 3,[if([gt(##,1)],big,small)])]":          "small big big",
		"[switch([add(1,1)],1,one,2,two,other)]":            "two",
		"[name(#[add(1,1)])]":                               "TestObject", // #2
		"[words([iter(a b c d,##)])]":                       "4",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("complex: %s = %q, want %q", expr, got, want)
		}
	}
}

// --- Connection functions (without GameState, should return empty/default) ---

func TestFnConnectionNoGameState(t *testing.T) {
	e := newEvalTestEnv(t)
	// Without GameState, these should return empty or -1
	got := e.eval("[lwho()]")
	if got != "" {
		t.Errorf("lwho() without GameState = %q, want empty", got)
	}
	got = e.eval("[mwho()]")
	if got != "" {
		t.Errorf("mwho() without GameState = %q, want empty", got)
	}
	got = e.eval("[conn(#1)]")
	if got != "-1" {
		t.Errorf("conn() without GameState = %q, want '-1'", got)
	}
	got = e.eval("[idle(#1)]")
	if got != "-1" {
		t.Errorf("idle() without GameState = %q, want '-1'", got)
	}
}

func TestFnPmatch(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[pmatch(me)]":     "#1",
		"[pmatch(Wizard)]": "#1",
		"[pmatch(Bob)]":    "#3",
	}
	for expr, want := range tests {
		got := e.eval(expr)
		if got != want {
			t.Errorf("pmatch: %s = %q, want %q", expr, got, want)
		}
	}
}

// --- Parent chain attribute inheritance ---

func TestParentChainAttrInheritance(t *testing.T) {
	e := newEvalTestEnv(t)
	// #2 has parent #6. #6 has DESC. #2 doesn't.
	// get(#2/DESC) should walk to parent and find "Inherited desc"
	got := e.eval("[get(#2/DESC)]")
	if got != "Inherited desc" {
		t.Errorf("parent chain attr: get(#2/DESC) = %q, want 'Inherited desc'", got)
	}
}
