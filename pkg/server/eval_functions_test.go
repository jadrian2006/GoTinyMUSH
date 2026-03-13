package server

import (
	"fmt"
	"math"
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
	// VA (attr 100) = "hello from VA"  — v(a) maps to 100 + ('A'-'A') = 100
	db.Objects[1].Attrs = append(db.Objects[1].Attrs,
		gamedb.Attribute{Number: 100, Value: "\x011:0:hello from VA"},
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

// setAttr sets a user-defined attribute on an object for testing.
// Uses a global counter starting at 500 for auto-assigned attr numbers.
var testAttrCounter = 500
var testAttrMap = map[string]int{}

func (e *evalTestEnv) setAttr(dbref gamedb.DBRef, name, value string) {
	attrNum, exists := testAttrMap[name]
	if !exists {
		attrNum = testAttrCounter
		testAttrCounter++
		testAttrMap[name] = attrNum
	}
	// Always register with this game's DB (each test has a fresh DB)
	e.game.DB.AddAttrDef(attrNum, name, 0)
	obj := e.game.DB.Objects[dbref]
	// Replace existing attr or append
	for i, a := range obj.Attrs {
		if a.Number == attrNum {
			obj.Attrs[i].Value = "\x011:0:" + value
			return
		}
	}
	obj.Attrs = append(obj.Attrs, gamedb.Attribute{
		Number: attrNum,
		Value:  "\x011:0:" + value,
	})
}

// toFloatTest parses a string to float64 for test assertions.
func toFloatTest(s string) float64 {
	v := 0.0
	fmt.Sscanf(s, "%f", &v)
	return v
}

// --- Object Functions ---

func TestFnName(t *testing.T) {
	e := newEvalTestEnv(t)
	tests := map[string]string{
		"[name(#1)]":  "Wizard",
		"[name(#0)]":  "Room Zero",
		"[name(#7)]":  "North", // exit returns first alias
		"[name(me)]":  "Wizard",
		"[name(#99)]": "#-1 NO MATCH",
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
	// C TinyMUSH: v(a) with single char → exec("%a") = absolute possessive pronoun.
	// Verified on crystalmush.kydance.net:6886.
	// Wizard (#1) has no @sex set → neuter → "its"
	got := e.eval("[v(a)]")
	if got != "its" {
		t.Errorf("v(a) = %q, want 'its' (absolute possessive pronoun)", got)
	}
	// v(va) with multi-char → attribute lookup for VA
	got = e.eval("[v(va)]")
	if got != "hello from VA" {
		t.Errorf("v(va) = %q, want 'hello from VA'", got)
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
	if got != "x-1 y-2 z-3" {
		t.Errorf("itext/inum = %q, want 'x-1 y-2 z-3'", got)
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
		"[lnum(2,6)]": "2 3 4 5 6",
		"[lnum(5,2)]": "5 4 3 2",
		"[lnum(1)]":   "0",
		"[lnum(0)]":   "",
		"[lnum(20,1)]": "20 19 18 17 16 15 14 13 12 11 10 9 8 7 6 5 4 3 2 1",
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

// --- Topology tests ---

func TestTopologyDeterminism(t *testing.T) {
	e := newEvalTestEnv(t)
	// Add a zone object #8 with topology attrs
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Test Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "12345")
	e.setAttr(8, "TOPO_BASE", "-20")
	e.setAttr(8, "TOPO_GRADIENT", "x -0.1")
	e.setAttr(8, "TOPO_NOISE_AMP", "1.0")
	e.setAttr(8, "TOPO_NOISE_FREQ", "0.1")
	e.setAttr(8, "TOPO_FEATURES", "0")

	// Same coordinates should always return the same value
	v1 := e.eval("[topology(#8,100,150)]")
	v2 := e.eval("[topology(#8,100,150)]")
	if v1 != v2 {
		t.Errorf("topology not deterministic: %q vs %q", v1, v2)
	}

	// Different coordinates should return different values (overwhelmingly likely)
	v3 := e.eval("[topology(#8,200,250)]")
	if v1 == v3 {
		t.Errorf("topology returned same value for different coords: %q", v1)
	}

	// With gradient x -0.1, deeper as x increases
	// At x=100: base(-20) + gradient(-10) + noise ≈ -30 + noise
	// At x=200: base(-20) + gradient(-20) + noise ≈ -40 + noise
	// The x=200 point should generally be more negative (deeper)
	f1 := toFloatTest(v1)
	f3 := toFloatTest(v3)
	if f3 >= f1 {
		t.Errorf("gradient not working: x=100 elev=%v, x=200 elev=%v (expected x=200 deeper)", f1, f3)
	}
}

func TestTopologyIsland(t *testing.T) {
	e := newEvalTestEnv(t)
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Ocean Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.eval("[topoflush(#8)]") // Clear cache from prior tests
	e.setAttr(8, "TOPO_SEED", "99")
	e.setAttr(8, "TOPO_BASE", "-30")
	e.setAttr(8, "TOPO_NOISE_AMP", "0") // No noise for clean test
	e.setAttr(8, "TOPO_FEATURES", "1")
	e.setAttr(8, "TOPO_F_1", "island|100 100|20|150")

	// Center of island should be positive (land)
	center := e.eval("[islandchk(#8,100,100)]")
	if center != "1" {
		t.Errorf("island center should be land: got %q", center)
	}

	// Far from island should be water
	far := e.eval("[islandchk(#8,200,200)]")
	if far != "0" {
		t.Errorf("far from island should be water: got %q", far)
	}

	// Depth at center should be 0 (on land)
	depthCenter := e.eval("[depth(#8,100,100)]")
	if depthCenter != "0" {
		t.Errorf("depth at island center should be 0: got %q", depthCenter)
	}

	// Depth far away should be positive (underwater)
	depthFar := e.eval("[depth(#8,200,200)]")
	if depthFar == "0" {
		t.Errorf("depth far from island should be > 0: got %q", depthFar)
	}
}

func TestTopologyTrench(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]") // Clear cache from prior tests
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Trench Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "42")
	e.setAttr(8, "TOPO_BASE", "-20")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "1")
	// Trench from (50,0) to (50,100), width 20, floor -150
	e.setAttr(8, "TOPO_F_1", "trench|50 0 50 100|20|-150")

	// On the trench centerline: should be much deeper
	onTrench := e.eval("[topology(#8,50,50)]")
	offTrench := e.eval("[topology(#8,100,50)]")

	fOn := toFloatTest(onTrench)
	fOff := toFloatTest(offTrench)
	if fOn >= fOff {
		t.Errorf("trench not deeper: on=%v, off=%v", fOn, fOff)
	}
	// Trench should add -150 at center, so total ≈ -170
	if fOn > -100 {
		t.Errorf("trench not deep enough: %v (expected < -100)", fOn)
	}
}

func TestTopoflush(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]") // Clear cache from prior tests
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Flush Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-10")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")

	v1 := e.eval("[topology(#8,50,50)]")

	// Change base elevation
	e.setAttr(8, "TOPO_BASE", "-50")
	// Without flush, should still return cached value
	v2 := e.eval("[topology(#8,50,50)]")
	if v1 != v2 {
		t.Errorf("cache not working: %q vs %q", v1, v2)
	}

	// After flush, should return new value
	e.eval("[topoflush(#8)]")
	v3 := e.eval("[topology(#8,50,50)]")
	if v2 == v3 {
		t.Errorf("topoflush did not clear cache: still %q", v3)
	}
}

func TestTopologyDepthWithTide(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]") // Clear cache from prior tests
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Tide Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-1") // 1m underwater at base
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")

	// Base depth should be 1
	d1 := e.eval("[depth(#8,50,50)]")
	if d1 != "1" {
		t.Errorf("base depth: got %q, want 1", d1)
	}

	// High tide +2: effective elev = -1 + 2 = 1 (land!)
	d2 := e.eval("[depth(#8,50,50,2)]")
	if d2 != "0" {
		t.Errorf("high tide should expose land: got %q, want 0", d2)
	}

	// Low tide -2: effective elev = -1 - 2 = -3, depth = 3
	d3 := e.eval("[depth(#8,50,50,-2)]")
	if d3 != "3" {
		t.Errorf("low tide depth: got %q, want 3", d3)
	}
}

func TestTopologyTopomap(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Map Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "42")
	e.setAttr(8, "TOPO_BASE", "-20")
	e.setAttr(8, "TOPO_NOISE_AMP", "3")
	e.setAttr(8, "TOPO_FEATURES", "1")
	e.setAttr(8, "TOPO_F_1", "island|25 25|10|50")

	result := e.eval("[topomap(#8,0,0,50,50,2)]")

	// Should contain newlines (multiple rows)
	lines := strings.Split(result, "\n")
	if len(lines) < 5 {
		t.Errorf("topomap too few lines: %d", len(lines))
	}

	// Should contain ANSI escape codes
	if !strings.Contains(result, "\033[") {
		t.Errorf("topomap missing ANSI codes")
	}

	// Should contain land characters (island at 25,25)
	if !strings.Contains(result, "^") && !strings.Contains(result, ".") {
		t.Errorf("topomap missing land markers for island")
	}

	// Should contain water characters
	if !strings.Contains(result, "~") && !strings.Contains(result, "-") {
		t.Errorf("topomap missing water markers")
	}

	t.Logf("topomap output (%d lines):\n%s", len(lines), result)
}

func TestTopologyCurrentBase(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Current Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-30") // deep open water everywhere
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")
	// Prevailing current: heading 90 (east) at 0.5 units/tick
	e.setAttr(8, "CURRENT_BASE", "90 0.5")

	result := e.eval("[current(#8,100,100)]")
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("current should return 'dx dy': got %q", result)
	}
	dx := toFloatTest(parts[0])
	dy := toFloatTest(parts[1])

	// Heading 90 (east) → dx≈0.5, dy≈0
	if math.Abs(dx-0.5) > 0.01 {
		t.Errorf("east current dx: got %v, want ~0.5", dx)
	}
	if math.Abs(dy) > 0.01 {
		t.Errorf("east current dy: got %v, want ~0", dy)
	}
}

func TestTopologyCurrentDeflection(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Deflect Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-30")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "1")
	e.setAttr(8, "TOPO_F_1", "island|50 50|15|80") // big island
	// Eastward current at 0.5
	e.setAttr(8, "CURRENT_BASE", "90 0.5")

	// In open water far from island: should be ~pure east
	openResult := e.eval("[current(#8,200,200)]")
	openParts := strings.Fields(openResult)
	openDx := toFloatTest(openParts[0])

	// Test west of island: eastward current flows INTO the island, should deflect around
	// Island at (50,50) radius 15, so (35,50) is right at the edge
	nearResult := e.eval("[current(#8,36,50)]") // just inside west edge
	nearParts := strings.Fields(nearResult)
	nearDx := toFloatTest(nearParts[0])
	nearDy := toFloatTest(nearParts[1])

	// Near island with current hitting it, should develop a Y component
	// (deflected around) and/or reduced X component
	deflected := math.Abs(nearDy) > 0.01 || math.Abs(nearDx) < math.Abs(openDx)*0.9
	if !deflected {
		t.Errorf("current not deflected near island: open=(%v,0), near=(%v,%v)",
			openDx, nearDx, nearDy)
	}
	t.Logf("open current: (%v, 0), near-island current: (%v, %v)", openDx, nearDx, nearDy)

	// On the island itself: should be zero (land)
	landResult := e.eval("[current(#8,50,50)]")
	landParts := strings.Fields(landResult)
	landDx := toFloatTest(landParts[0])
	landDy := toFloatTest(landParts[1])
	if landDx != 0 || landDy != 0 {
		t.Errorf("current on land should be 0 0: got %v %v", landDx, landDy)
	}
}

func TestTopologyCurrentTidal(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Tidal Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-20")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")
	e.setAttr(8, "CURRENT_BASE", "90 0.3")         // eastward base
	e.setAttr(8, "CURRENT_TIDAL", "0 0.2")          // north-south tidal component

	// At flood tide (+1): tidal adds northward
	flood := e.eval("[current(#8,100,100,1)]")
	floodParts := strings.Fields(flood)
	floodDy := toFloatTest(floodParts[1])

	// At ebb tide (-1): tidal adds southward
	ebb := e.eval("[current(#8,100,100,-1)]")
	ebbParts := strings.Fields(ebb)
	ebbDy := toFloatTest(ebbParts[1])

	// Flood should push north (positive Y), ebb should push south (negative Y)
	if floodDy <= 0 {
		t.Errorf("flood tide should push north: dy=%v", floodDy)
	}
	if ebbDy >= 0 {
		t.Errorf("ebb tide should push south: dy=%v", ebbDy)
	}

	// Difference should be about 2*0.2 = 0.4
	diff := floodDy - ebbDy
	if math.Abs(diff-0.4) > 0.01 {
		t.Errorf("tidal swing: got %v, want ~0.4", diff)
	}
}

func TestTopologyCurrentStorm(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Storm Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-30")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")
	e.setAttr(8, "CURRENT_BASE", "90 0.5") // east at 0.5

	// Normal conditions (storm_factor=1)
	normal := e.eval("[current(#8,100,100,0,1)]")
	normalParts := strings.Fields(normal)
	normalDx := toFloatTest(normalParts[0])

	// Storm doubles current (storm_factor=2)
	storm := e.eval("[current(#8,100,100,0,2)]")
	stormParts := strings.Fields(storm)
	stormDx := toFloatTest(stormParts[0])

	// Storm current should be ~2x normal
	if math.Abs(stormDx-normalDx*2) > 0.01 {
		t.Errorf("storm not doubling: normal=%v, storm=%v (want %v)", normalDx, stormDx, normalDx*2)
	}

	// Heavy storm (factor=4)
	heavy := e.eval("[current(#8,100,100,0,4)]")
	heavyParts := strings.Fields(heavy)
	heavyDx := toFloatTest(heavyParts[0])
	if math.Abs(heavyDx-normalDx*4) > 0.01 {
		t.Errorf("heavy storm not 4x: normal=%v, heavy=%v (want %v)", normalDx, heavyDx, normalDx*4)
	}
}

func TestTopologyCurrentWind(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Wind Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-30")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")
	// No base current — wind only
	e.setAttr(8, "CURRENT_BASE", "0 0")

	// No wind: should be 0 0
	noWind := e.eval("[current(#8,100,100)]")
	if noWind != "0 0" {
		t.Errorf("no wind should be 0 0: got %q", noWind)
	}

	// Strong north wind (hdg 0, speed 10): Ekman → ~3% at 45° right (NE)
	// wind hdg 0 (N) + 45° deflect → surface current flows NE
	withWind := e.eval("[current(#8,100,100,0,1,0,10)]")
	windParts := strings.Fields(withWind)
	windDx := toFloatTest(windParts[0])
	windDy := toFloatTest(windParts[1])

	// Ekman: 10 * 0.03 = 0.3 at 45° → dx≈0.212, dy≈0.212
	expectedMag := 10 * 0.03
	actualMag := math.Sqrt(windDx*windDx + windDy*windDy)
	if math.Abs(actualMag-expectedMag) > 0.01 {
		t.Errorf("wind current magnitude: got %v, want ~%v", actualMag, expectedMag)
	}

	// Both dx and dy should be positive (NE direction)
	if windDx <= 0 || windDy <= 0 {
		t.Errorf("north wind should create NE current: got (%v, %v)", windDx, windDy)
	}

	// Storm amplifies wind-driven current too
	stormWind := e.eval("[current(#8,100,100,0,3,0,10)]")
	stormParts := strings.Fields(stormWind)
	stormMag := math.Sqrt(toFloatTest(stormParts[0])*toFloatTest(stormParts[0]) +
		toFloatTest(stormParts[1])*toFloatTest(stormParts[1]))
	if math.Abs(stormMag-expectedMag*3) > 0.01 {
		t.Errorf("storm should amplify wind current: got %v, want ~%v", stormMag, expectedMag*3)
	}
}

func TestTopologyCurrentAirMedium(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Air Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-30") // ocean below
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "0")
	e.setAttr(8, "AIR_BASE", "90 1.0") // prevailing east wind at 1.0

	// Air current with no weather wind: should be base wind
	air := e.eval("[current(#8,100,100,0,1,0,0,air)]")
	airParts := strings.Fields(air)
	airDx := toFloatTest(airParts[0])
	if math.Abs(airDx-1.0) > 0.01 {
		t.Errorf("air base wind: got dx=%v, want ~1.0", airDx)
	}

	// Water current at same position: should be 0 (no CURRENT_BASE set)
	water := e.eval("[current(#8,100,100)]")
	if water != "0 0" {
		t.Errorf("water current should be 0 0 with no CURRENT_BASE: got %q", water)
	}

	// Air with weather wind: gets 100% (not 3% Ekman)
	airWind := e.eval("[current(#8,100,100,0,1,90,5,air)]")
	airWindParts := strings.Fields(airWind)
	airWindDx := toFloatTest(airWindParts[0])
	// base=1.0 east + weather wind 90°(east) at 5.0 = 6.0 east total
	if math.Abs(airWindDx-6.0) > 0.01 {
		t.Errorf("air+wind: got dx=%v, want ~6.0", airWindDx)
	}

	// Water with same wind: gets only 3% Ekman (deflected 45°)
	waterWind := e.eval("[current(#8,100,100,0,1,90,5,water)]")
	waterWindParts := strings.Fields(waterWind)
	waterWindMag := math.Sqrt(
		toFloatTest(waterWindParts[0])*toFloatTest(waterWindParts[0]) +
			toFloatTest(waterWindParts[1])*toFloatTest(waterWindParts[1]))
	// 5 * 0.03 = 0.15
	if math.Abs(waterWindMag-0.15) > 0.01 {
		t.Errorf("water Ekman magnitude: got %v, want ~0.15", waterWindMag)
	}
}

func TestTopologyTemperature(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Temp Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
		Flags: [3]int{int(gamedb.TypeThing), 0, 0},
	}
	e.setAttr(8, "TOPO_SEED", "1")
	e.setAttr(8, "TOPO_BASE", "-30")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "1")
	e.setAttr(8, "TOPO_F_1", "island|50 50|15|80") // volcanic island
	e.setAttr(8, "TEMP_BASE", "22")
	e.setAttr(8, "TEMP_DEPTH_RATE", "0.2")
	e.setAttr(8, "TEMP_GRADIENT", "y -0.02") // colder as y increases

	// Surface temp at origin
	t0 := toFloatTest(e.eval("[temperature(#8,0,0)]"))
	// base=22, gradient y=0 → 22
	if math.Abs(t0-22.0) > 0.5 {
		t.Errorf("surface temp at origin: got %v, want ~22", t0)
	}

	// Northern position should be colder
	tNorth := toFloatTest(e.eval("[temperature(#8,0,200)]"))
	// base=22 + 200*(-0.02) = 18
	if math.Abs(tNorth-18.0) > 0.5 {
		t.Errorf("north temp: got %v, want ~18", tNorth)
	}

	// Deeper = colder
	tDeep := toFloatTest(e.eval("[temperature(#8,0,0,10)]"))
	// 22 - 10*0.2 = 20
	if math.Abs(tDeep-20.0) > 0.5 {
		t.Errorf("depth 10 temp: got %v, want ~20", tDeep)
	}

	// Near volcanic island = warmer
	tVolcanic := toFloatTest(e.eval("[temperature(#8,50,50)]"))
	// base=22 + gradient(50*-0.02=-1) + volcanic warming(~3) = ~24
	if tVolcanic <= t0 {
		t.Errorf("volcanic area should be warmer: volcanic=%v, open=%v", tVolcanic, t0)
	}

	// Season offset
	tWinter := toFloatTest(e.eval("[temperature(#8,0,0,0,-5)]"))
	// 22 + (-5) = 17
	if math.Abs(tWinter-17.0) > 0.5 {
		t.Errorf("winter temp: got %v, want ~17", tWinter)
	}
}

func TestTopologyShelfY(t *testing.T) {
	e := newEvalTestEnv(t)
	e.eval("[topoflush(#8)]")
	e.game.DB.Objects[8] = &gamedb.Object{
		DBRef: 8, Name: "Bay Zone",
		Location: gamedb.Nothing, Contents: gamedb.Nothing, Exits: gamedb.Nothing,
		Link: gamedb.Nothing, Next: gamedb.Nothing,
		Owner: 1, Parent: gamedb.Nothing, Zone: gamedb.Nothing,
	}

	// Base -20 ocean, no gradient, no noise
	// North shelf_y: y=0..40, slope 25 → beach at y=0, deep by y=40
	// South shelf_y: y=200..160, slope 25 → beach at y=200, deep by y=160
	e.setAttr(8, "TOPO_SEED", "100")
	e.setAttr(8, "TOPO_BASE", "-20")
	e.setAttr(8, "TOPO_NOISE_AMP", "0")
	e.setAttr(8, "TOPO_FEATURES", "2")
	e.setAttr(8, "TOPO_F_1", "shelf_y|0 40|25")
	e.setAttr(8, "TOPO_F_2", "shelf_y|200 160|25")

	// North coast: y=0 should be above water (elev = -20 + 25 = 5)
	dN0 := e.eval("[depth(#8,100,0)]")
	if dN0 != "0" {
		t.Errorf("north beach (y=0): got depth %q, want 0 (land)", dN0)
	}

	// North edge of shelf: y=40 should be full ocean depth
	dN40 := toFloatTest(e.eval("[depth(#8,100,40)]"))
	if dN40 < 19 {
		t.Errorf("north shelf edge (y=40): got depth %v, want ~20", dN40)
	}

	// Midpoint y=20: should have some shelf contribution (cosine halfway ≈ 12.5)
	dN20 := toFloatTest(e.eval("[depth(#8,100,20)]"))
	if dN20 < 2 || dN20 > 15 {
		t.Errorf("north shelf mid (y=20): got depth %v, want 2-15", dN20)
	}

	// South coast: y=200 should be above water (elev = -20 + 25 = 5)
	dS200 := e.eval("[depth(#8,100,200)]")
	if dS200 != "0" {
		t.Errorf("south beach (y=200): got depth %q, want 0 (land)", dS200)
	}

	// South shelf interior: y=160 should be full depth
	dS160 := toFloatTest(e.eval("[depth(#8,100,160)]"))
	if dS160 < 19 {
		t.Errorf("south shelf edge (y=160): got depth %v, want ~20", dS160)
	}

	// Middle of ocean: y=100, no shelf, full depth
	dMid := toFloatTest(e.eval("[depth(#8,100,100)]"))
	if math.Abs(dMid-20.0) > 1 {
		t.Errorf("open ocean (y=100): got depth %v, want ~20", dMid)
	}
}

// --- #lambda tests ---

func TestLambdaMap(t *testing.T) {
	e := newEvalTestEnv(t)
	// map passes each element as %0
	got := e.eval("[map(#lambda/[strlen(%0)],abc de f)]")
	if got != "3 2 1" {
		t.Errorf("map(#lambda) = %q, want '3 2 1'", got)
	}
}

func TestLambdaFilter(t *testing.T) {
	e := newEvalTestEnv(t)
	// filter passes each element as %0, keeps elements where result is true
	got := e.eval("[filter(#lambda/[gt(strlen(%0),1)],a bc d ef)]")
	if got != "bc ef" {
		t.Errorf("filter(#lambda) = %q, want 'bc ef'", got)
	}
}

func TestLambdaFold(t *testing.T) {
	e := newEvalTestEnv(t)
	// fold(#lambda/[add(%0,%1)], 1 2 3 4) should return "10"
	got := e.eval("[fold(#lambda/[add(%0,%1)],1 2 3 4)]")
	if got != "10" {
		t.Errorf("fold(#lambda) = %q, want '10'", got)
	}
}

func TestLambdaStep(t *testing.T) {
	e := newEvalTestEnv(t)
	// step passes chunks as %0, %1; step size 2 means pairs
	// cat() joins with space, so "a b" per pair
	got := e.eval("[step(#lambda/[cat(%0,%1)],a b c d,2)]")
	if got != "a b c d" {
		t.Errorf("step(#lambda) = %q, want 'a b c d'", got)
	}
	// Also test with add() for numeric step
	got2 := e.eval("[step(#lambda/[add(%0,%1)],1 2 3 4,2)]")
	if got2 != "3 7" {
		t.Errorf("step(#lambda/add) = %q, want '3 7'", got2)
	}
}

func TestLambdaMix(t *testing.T) {
	e := newEvalTestEnv(t)
	// mix passes corresponding elements as %0, %1
	got := e.eval("[mix(#lambda/[add(%0,%1)],1 2 3,10 20 30)]")
	if got != "11 22 33" {
		t.Errorf("mix(#lambda) = %q, want '11 22 33'", got)
	}
}

func TestLambdaSortby(t *testing.T) {
	e := newEvalTestEnv(t)
	// sortby(#lambda/[sub(strlen(%0),strlen(%1))], cc a bbb) should sort by string length
	got := e.eval("[sortby(#lambda/[sub(strlen(%0),strlen(%1))],cc a bbb)]")
	if got != "a cc bbb" {
		t.Errorf("sortby(#lambda) = %q, want 'a cc bbb'", got)
	}
}

func TestLambdaForeach(t *testing.T) {
	e := newEvalTestEnv(t)
	// foreach(fn, string) — C arg order: function first, string second
	got := e.eval("[foreach(#lambda/[ucstr(%0)],abc)]")
	if got != "ABC" {
		t.Errorf("foreach(#lambda) = %q, want 'ABC'", got)
	}
}

func TestSortbyWithNonexistentAttr(t *testing.T) {
	e := newEvalTestEnv(t)
	// sortby with a nonexistent function should return the list unchanged
	// (comparison function returns "" → 0, so no swaps in stable sort)
	got := e.eval("[sortby(#2/NOSUCHATTR,3 1 4 1 5)]")
	if got != "3 1 4 1 5" {
		t.Errorf("sortby(nonexistent) = %q, want '3 1 4 1 5' (unchanged)", got)
	}
}
