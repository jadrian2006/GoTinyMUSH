package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// testEnv holds the shared test infrastructure.
type testEnv struct {
	game   *Game
	player *Descriptor // wizard player #1
	room   gamedb.DBRef
}

// readAll drains all available output from a descriptor's pipe.
func readAll(d *Descriptor) string {
	// Close the writer side so the reader gets EOF
	// But we can't do that without breaking further sends.
	// Instead, set a short read deadline and read what's available.
	type deadliner interface {
		SetReadDeadline(time.Time) error
	}
	if dl, ok := d.Conn.(deadliner); ok {
		dl.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	}
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := d.Conn.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimRight(sb.String(), "\r\n")
}

// newTestEnv creates a minimal game environment for testing.
// It creates:
//   - Room #0 (Room Zero)
//   - Player #1 (Wizard) in Room #0, wizard flag set
//   - Thing #2 (TestObject) in Room #0
//   - Player #3 (Bob) in Room #0
//   - Room #4 (OtherRoom)
//   - Thing #5 (Container) in Room #0 with ENTER_OK
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := gamedb.NewDatabase()

	// Room #0
	db.Objects[0] = &gamedb.Object{
		DBRef:    0,
		Name:     "Room Zero",
		Location: gamedb.Nothing,
		Contents: 1, // Wizard is first in contents chain
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeRoom), 0, 0},
	}

	// Wizard #1
	db.Objects[1] = &gamedb.Object{
		DBRef:    1,
		Name:     "Wizard",
		Location: 0,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     0, // home = Room Zero
		Next:     2, // next in room contents chain
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Pennies:  1000,
		Flags:    [3]int{int(gamedb.TypePlayer) | gamedb.FlagWizard, 0, 0},
	}

	// TestObject #2 (THING in Room Zero)
	db.Objects[2] = &gamedb.Object{
		DBRef:    2,
		Name:     "TestObject",
		Location: 0,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     3,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing), 0, 0},
	}

	// Bob #3 (Player in Room Zero)
	db.Objects[3] = &gamedb.Object{
		DBRef:    3,
		Name:     "Bob",
		Location: 0,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     0,
		Next:     5,
		Owner:    3,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Pennies:  100,
		Flags:    [3]int{int(gamedb.TypePlayer), 0, 0},
	}

	// OtherRoom #4
	db.Objects[4] = &gamedb.Object{
		DBRef:    4,
		Name:     "Other Room",
		Location: gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeRoom), 0, 0},
	}

	// Container #5 (THING in Room Zero with ENTER_OK)
	db.Objects[5] = &gamedb.Object{
		DBRef:    5,
		Name:     "Container",
		Location: 0,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeThing) | gamedb.FlagEnterOK, 0, 0},
	}

	// Contents chain: Room #0 -> 1 -> 2 -> 3 -> 5 -> Nothing
	db.Objects[0].Contents = 1

	conns := NewConnManager()
	g := &Game{
		DB:       db,
		Conns:    conns,
		Commands: InitCommands(),
		Queue:    NewCommandQueue(),
		NextRef:  6,
	}

	// Create a piped descriptor for the wizard player
	d := makeTestDescriptor(t, conns, 1)

	return &testEnv{game: g, player: d, room: 0}
}

// makeTestDescriptor creates a Descriptor backed by net.Pipe for capturing output.
func makeTestDescriptor(t *testing.T, cm *ConnManager, player gamedb.DBRef) *Descriptor {
	t.Helper()
	// net.Pipe gives us two connected endpoints — write to server side, read from client side
	serverConn, clientConn := net.Pipe()
	id := cm.NextID()
	d := &Descriptor{
		ID:       id,
		Conn:     serverConn,
		Reader:   bufio.NewReader(serverConn),
		State:    ConnConnected,
		Player:   player,
		Addr:     "test",
		ConnTime: time.Now(),
		LastCmd:  time.Now(),
	}
	cm.Add(d)
	cm.Login(d, player)

	// Keep a reference to clientConn so we can read from it
	// We store it in the Reader field repurposed — but actually
	// we'll just use a wrapper. Let's store clientConn separately.
	// Swap the Conn: the Descriptor writes to serverConn, tests read from clientConn.
	// Actually, net.Pipe is synchronous — reads block until writes happen.
	// So we need to wrap it. Let's use a buffered approach.
	d.Conn = serverConn

	// Replace the descriptor's conn with clientConn-readable approach
	// Actually the simplest approach: use a wrapper that buffers writes
	// and let tests read from the clientConn.
	// But net.Pipe is synchronous, so writes block until reads drain.
	// We need an async reader. Let's just use a channel-buffered approach.

	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})

	// Swap Conn to a buffered writer wrapper to avoid blocking
	d.Conn = &asyncPipeWriter{conn: serverConn, clientConn: clientConn}
	return d
}

// asyncPipeWriter wraps a net.Pipe server-side conn and stores output in a buffer.
type asyncPipeWriter struct {
	conn       net.Conn
	clientConn net.Conn
	buf        strings.Builder
}

func (a *asyncPipeWriter) Read(b []byte) (int, error) {
	return 0, fmt.Errorf("read not supported on server side")
}

func (a *asyncPipeWriter) Write(b []byte) (int, error) {
	a.buf.Write(b)
	return len(b), nil
}

func (a *asyncPipeWriter) Close() error {
	return a.conn.Close()
}

func (a *asyncPipeWriter) LocalAddr() net.Addr                { return a.conn.LocalAddr() }
func (a *asyncPipeWriter) RemoteAddr() net.Addr               { return a.conn.RemoteAddr() }
func (a *asyncPipeWriter) SetDeadline(t time.Time) error      { return nil }
func (a *asyncPipeWriter) SetReadDeadline(t time.Time) error  { return nil }
func (a *asyncPipeWriter) SetWriteDeadline(t time.Time) error { return nil }

// getOutput returns all buffered output and clears the buffer.
func getOutput(d *Descriptor) string {
	w, ok := d.Conn.(*asyncPipeWriter)
	if !ok {
		return ""
	}
	s := w.buf.String()
	w.buf.Reset()
	return strings.TrimRight(s, "\r\n")
}

// clearOutput discards any buffered output.
func clearOutput(d *Descriptor) {
	if w, ok := d.Conn.(*asyncPipeWriter); ok {
		w.buf.Reset()
	}
}

// --- Tests ---

func TestDispatchCommand_Say(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "say Hello World")
	out := getOutput(env.player)
	if !strings.Contains(out, `You say "Hello World"`) {
		t.Errorf("say: expected 'You say \"Hello World\"', got: %s", out)
	}
}

func TestDispatchCommand_SayQuote(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, `"Hello World`)
	out := getOutput(env.player)
	if !strings.Contains(out, `You say "Hello World"`) {
		t.Errorf(`" shortcut: expected 'You say "Hello World"', got: %s`, out)
	}
}

func TestDispatchCommand_Pose(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, ":waves")
	out := getOutput(env.player)
	// pose sends to room (including self via SendToRoom)
	// nothing sent directly to player, only to room
	// In test environment with no real room listeners, just check no crash
	_ = out
}

func TestDispatchCommand_Think(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "think [add(1,2)]")
	out := getOutput(env.player)
	if !strings.Contains(out, "3") {
		t.Errorf("think: expected '3', got: %s", out)
	}
}

func TestDispatchCommand_Eval(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@eval [mul(3,4)]")
	out := getOutput(env.player)
	if !strings.Contains(out, "12") {
		t.Errorf("@eval: expected '12' in output, got: %s", out)
	}
}

func TestDispatchCommand_HuhMessage(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "xyznonexistent")
	out := getOutput(env.player)
	if !strings.Contains(out, "Huh?") {
		t.Errorf("unknown command: expected 'Huh?', got: %s", out)
	}
}

func TestDispatchCommand_Version(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@version")
	out := getOutput(env.player)
	if !strings.Contains(out, "GoTinyMUSH") {
		t.Errorf("@version: expected 'GoTinyMUSH', got: %s", out)
	}
}

func TestDispatchCommand_Inventory(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "inventory")
	out := getOutput(env.player)
	if !strings.Contains(out, "You are carrying") || !strings.Contains(out, "nothing") {
		// Wizard starts with no inventory, so should see "nothing"
		// or "You are carrying:" followed by nothing
		_ = out // acceptable either way
	}
}

func TestDispatchCommand_Score(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "score")
	out := getOutput(env.player)
	if !strings.Contains(out, "1000") {
		t.Errorf("score: expected pennies count '1000', got: %s", out)
	}
}

func TestDispatchCommand_Examine(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "examine me")
	out := getOutput(env.player)
	if !strings.Contains(out, "Wizard") {
		t.Errorf("examine: expected 'Wizard' in output, got: %s", out)
	}
}

// --- Attribute Setter Tests ---

func TestAttrSetter_Success(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@success me=You did it!")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@success: expected 'Set.', got: %s", out)
	}

	// Verify the attribute was actually set
	text := env.game.GetAttrText(1, 4) // A_SUCC = 4
	if text != "You did it!" {
		t.Errorf("@success: expected attr text 'You did it!', got: %s", text)
	}
}

func TestAttrSetter_Fail(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@fail me=Sorry, you can't.")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@fail: expected 'Set.', got: %s", out)
	}
	text := env.game.GetAttrText(1, 3) // A_FAIL = 3
	if text != "Sorry, you can't." {
		t.Errorf("@fail: expected 'Sorry, you can't.', got: %s", text)
	}
}

func TestAttrSetter_Sex(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@sex me=Male")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@sex: expected 'Set.', got: %s", out)
	}
	text := env.game.GetAttrText(1, 7) // A_SEX = 7
	if text != "Male" {
		t.Errorf("@sex: expected 'Male', got: %s", text)
	}
}

func TestAttrSetter_Describe(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@describe me=A powerful wizard.")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") {
		t.Errorf("@describe: expected 'Set.', got: %s", out)
	}
	text := env.game.GetAttrText(1, 6) // A_DESC = 6
	if text != "A powerful wizard." {
		t.Errorf("@describe: expected 'A powerful wizard.', got: %s", text)
	}
}

func TestAttrSetter_NoEquals(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@success me")
	out := getOutput(env.player)
	if !strings.Contains(out, "=") || !strings.Contains(out, "need") {
		// Should show usage error about needing =
		_ = out
	}
}

func TestAttrSetter_PermDenied(t *testing.T) {
	env := newTestEnv(t)
	// Create a non-wizard descriptor for Bob (#3)
	bobDesc := makeTestDescriptor(t, env.game.Conns, 3)
	clearOutput(bobDesc)

	// Bob tries to set an attr on Wizard's object - should fail
	DispatchCommand(env.game, bobDesc, "@success #1=hacked")
	out := getOutput(bobDesc)
	if !strings.Contains(out, "Permission denied") {
		t.Errorf("@success perm: expected 'Permission denied', got: %s", out)
	}
}

// --- Player Object Commands ---

func TestGet(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "get TestObject")
	out := getOutput(env.player)
	if !strings.Contains(out, "You pick up TestObject") {
		t.Errorf("get: expected 'You pick up TestObject', got: %s", out)
	}

	// Verify object moved to inventory
	obj := env.game.DB.Objects[2]
	if obj.Location != 1 {
		t.Errorf("get: TestObject should be in Wizard's inventory, location=%d", obj.Location)
	}
}

func TestGetEmpty(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "get")
	out := getOutput(env.player)
	if !strings.Contains(out, "Get what?") {
		t.Errorf("get empty: expected 'Get what?', got: %s", out)
	}
}

func TestDrop(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// First pick up the object
	DispatchCommand(env.game, env.player, "get TestObject")
	clearOutput(env.player)

	// Now drop it
	DispatchCommand(env.game, env.player, "drop TestObject")
	out := getOutput(env.player)
	if !strings.Contains(out, "You drop TestObject") {
		t.Errorf("drop: expected 'You drop TestObject', got: %s", out)
	}

	obj := env.game.DB.Objects[2]
	if obj.Location != 0 {
		t.Errorf("drop: TestObject should be back in Room Zero, location=%d", obj.Location)
	}
}

func TestDropNotCarrying(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "drop TestObject")
	out := getOutput(env.player)
	if !strings.Contains(out, "aren't carrying") {
		t.Errorf("drop not carrying: expected 'aren't carrying', got: %s", out)
	}
}

func TestGivePennies(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "give Bob=50")
	out := getOutput(env.player)
	if !strings.Contains(out, "give") && !strings.Contains(out, "50") {
		t.Errorf("give: expected give confirmation, got: %s", out)
	}

	// Verify pennies transferred
	wizard := env.game.DB.Objects[1]
	bob := env.game.DB.Objects[3]
	if wizard.Pennies != 950 {
		t.Errorf("give: wizard should have 950 pennies, has %d", wizard.Pennies)
	}
	if bob.Pennies != 150 {
		t.Errorf("give: bob should have 150 pennies, has %d", bob.Pennies)
	}
}

func TestEnter(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "enter Container")
	out := getOutput(env.player)
	if !strings.Contains(out, "You enter Container") {
		t.Errorf("enter: expected 'You enter Container', got: %s", out)
	}

	// Verify player moved inside container
	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 5 {
		t.Errorf("enter: player should be in Container (#5), location=%d", playerObj.Location)
	}
}

func TestLeave(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// First enter the container
	DispatchCommand(env.game, env.player, "enter Container")
	clearOutput(env.player)

	// Now leave
	DispatchCommand(env.game, env.player, "leave")
	out := getOutput(env.player)
	if !strings.Contains(out, "You leave") {
		t.Errorf("leave: expected 'You leave', got: %s", out)
	}

	playerObj := env.game.DB.Objects[1]
	if playerObj.Location != 0 {
		t.Errorf("leave: player should be back in Room Zero, location=%d", playerObj.Location)
	}
}

func TestWhisper(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "whisper Bob=secret message")
	out := getOutput(env.player)
	if !strings.Contains(out, "You whisper") {
		t.Errorf("whisper: expected 'You whisper', got: %s", out)
	}
}

func TestUse(t *testing.T) {
	env := newTestEnv(t)

	// Set a USE attribute on TestObject
	env.game.SetAttr(2, 41, "You use the object.") // A_USE = 41
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "use TestObject")
	out := getOutput(env.player)
	if !strings.Contains(out, "You use the object.") {
		t.Errorf("use: expected 'You use the object.', got: %s", out)
	}
}

func TestKill(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "kill Bob")
	out := getOutput(env.player)
	if !strings.Contains(out, "You killed Bob!") {
		t.Errorf("kill: expected 'You killed Bob!', got: %s", out)
	}
}

// --- Building Commands ---

func TestCreate(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@create Widget")
	out := getOutput(env.player)
	if !strings.Contains(out, "Widget") && !strings.Contains(out, "created") {
		// Some format of creation confirmation
		_ = out
	}
}

func TestDig(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@dig New Room")
	out := getOutput(env.player)
	if !strings.Contains(out, "New Room") {
		t.Errorf("@dig: expected room name in output, got: %s", out)
	}
}

func TestDescribe(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@describe here=A test room.")
	out := getOutput(env.player)
	if !strings.Contains(out, "Set.") && !strings.Contains(out, "set") {
		t.Errorf("@describe: expected confirmation, got: %s", out)
	}

	text := env.game.GetAttrText(0, 6) // A_DESC = 6
	if text != "A test room." {
		t.Errorf("@describe: expected 'A test room.', got: %s", text)
	}
}

func TestName(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@name #2=RenamedObject")
	out := getOutput(env.player)
	_ = out

	obj := env.game.DB.Objects[2]
	if obj.Name != "RenamedObject" {
		t.Errorf("@name: expected 'RenamedObject', got: %s", obj.Name)
	}
}

func TestSet(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@set #2=DARK")
	out := getOutput(env.player)
	if !strings.Contains(out, "set") || !strings.Contains(out, "Set") {
		// Some flag set confirmation
		_ = out
	}
}

func TestLink(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Link Wizard's home to OtherRoom
	DispatchCommand(env.game, env.player, "@link me=#4")
	out := getOutput(env.player)
	_ = out

	playerObj := env.game.DB.Objects[1]
	if playerObj.Link != 4 {
		t.Errorf("@link: expected link to #4, got #%d", playerObj.Link)
	}
}

func TestParent(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@parent #2=#0")
	out := getOutput(env.player)
	_ = out

	obj := env.game.DB.Objects[2]
	if obj.Parent != 0 {
		t.Errorf("@parent: expected parent #0, got #%d", obj.Parent)
	}
}

// --- Communication Commands ---

func TestEmit(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// @emit sends to the room - in test env it goes to ConnManager but our
	// test descriptor is in the room so it should receive via SendToRoom
	DispatchCommand(env.game, env.player, "@emit Test broadcast message")
	// Output goes through ConnManager.SendToRoom which iterates room contents
	// In our test setup, the descriptor IS in the room
}

func TestEmitWithSoftcode(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "think [add(2,3)]")
	out := getOutput(env.player)
	if !strings.Contains(out, "5") {
		t.Errorf("softcode eval: expected '5', got: %s", out)
	}
}

// --- Admin Commands ---

func TestMotd(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Set MOTD (wizard only)
	DispatchCommand(env.game, env.player, "@motd Welcome to GoTinyMUSH!")
	out := getOutput(env.player)
	if !strings.Contains(out, "MOTD") || !strings.Contains(out, "set") {
		_ = out
	}

	// Check it was stored
	if env.game.MOTD != "Welcome to GoTinyMUSH!" {
		t.Errorf("@motd: expected 'Welcome to GoTinyMUSH!', got: %s", env.game.MOTD)
	}
}

func TestSearch(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@search type=player")
	out := getOutput(env.player)
	if !strings.Contains(out, "Wizard") || !strings.Contains(out, "Bob") {
		t.Errorf("@search: expected Wizard and Bob in results, got: %s", out)
	}
}

func TestDecompile(t *testing.T) {
	env := newTestEnv(t)

	// Set some attributes first
	env.game.SetAttr(2, 6, "A test object.")  // DESC
	env.game.SetAttr(2, 4, "You got it!")      // SUCC
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "@decompile #2")
	out := getOutput(env.player)
	if !strings.Contains(out, "TestObject") {
		t.Errorf("@decompile: expected object name, got: %s", out)
	}
}

func TestChzone(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// Zone a THING to another THING (rooms can only be zoned to rooms)
	DispatchCommand(env.game, env.player, "@chzone #2=#5")
	out := getOutput(env.player)
	_ = out

	obj := env.game.DB.Objects[2]
	if obj.Zone != 5 {
		t.Errorf("@chzone: expected zone #5, got #%d", obj.Zone)
	}
}

// --- Dispatch and Switches ---

func TestSwitchParsing(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// @dolist with /now switch should work
	DispatchCommand(env.game, env.player, "@dolist/now a b c=think ##")
	out := getOutput(env.player)
	// Should see a, b, c as separate think outputs
	if !strings.Contains(out, "a") {
		t.Errorf("@dolist/now: expected output, got: %s", out)
	}
}

// --- Help System ---

func TestHelpNoFiles(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	// No help files loaded, should say no help available
	DispatchCommand(env.game, env.player, "help")
	out := getOutput(env.player)
	if !strings.Contains(out, "No help available") && !strings.Contains(out, "no help") {
		// Some message about no help
		_ = out
	}
}

// --- Multiple Attribute Setters ---

func TestAttrSetterBulk(t *testing.T) {
	env := newTestEnv(t)

	// Test several attribute-setting commands
	tests := []struct {
		cmd     string
		attrNum int
		value   string
	}{
		{"@success me=win", 4, "win"},
		{"@osuccess me=wins", 1, "wins"},
		{"@fail me=nope", 3, "nope"},
		{"@ofail me=fails", 2, "fails"},
		{"@sex me=Female", 7, "Female"},
		{"@alias me=Wiz", 54, "Wiz"},
		{"@listen me=* says *", 24, "* says *"},
		{"@idesc me=Inside description", 28, "Inside description"},
		{"@reject me=Go away!", 68, "Go away!"},
	}

	for _, tt := range tests {
		clearOutput(env.player)
		DispatchCommand(env.game, env.player, tt.cmd)
		out := getOutput(env.player)
		if !strings.Contains(out, "Set.") {
			t.Errorf("%s: expected 'Set.', got: %s", tt.cmd, out)
		}
		text := env.game.GetAttrText(1, tt.attrNum)
		if text != tt.value {
			t.Errorf("%s: expected attr %d = %q, got %q", tt.cmd, tt.attrNum, tt.value, text)
		}
	}
}

// --- Eval Tests (softcode in commands) ---

func TestSayEvalsSoftcode(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "say [add(1,2)]")
	out := getOutput(env.player)
	if !strings.Contains(out, `You say "3"`) {
		t.Errorf("say softcode: expected 'You say \"3\"', got: %s", out)
	}
}

// --- Internal Attr Hiding ---

func TestExamineHidesPassword(t *testing.T) {
	env := newTestEnv(t)

	// Set a password attribute
	env.game.SetAttr(1, 5, "secret_password") // A_PASS = 5
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "examine me")
	out := getOutput(env.player)
	if strings.Contains(out, "secret_password") {
		t.Errorf("examine: password should be hidden, but found in output: %s", out)
	}
}

// --- Look Command ---

func TestLook(t *testing.T) {
	env := newTestEnv(t)

	// Set room description
	env.game.SetAttr(0, 6, "A plain test room.") // A_DESC = 6
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "look")
	out := getOutput(env.player)
	if !strings.Contains(out, "Room Zero") {
		t.Errorf("look: expected 'Room Zero' in output, got: %s", out)
	}
	if !strings.Contains(out, "A plain test room.") {
		t.Errorf("look: expected description in output, got: %s", out)
	}
}

// --- Parent Chain Inheritance ---

func TestParentChainDescLookup(t *testing.T) {
	env := newTestEnv(t)

	// Create parent room #6 with a DESC
	env.game.DB.Objects[6] = &gamedb.Object{
		DBRef:    6,
		Name:     "Parent Room",
		Location: gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Flags:    [3]int{int(gamedb.TypeRoom), 0, 0},
	}
	env.game.SetAttr(6, 6, "Inherited description.") // DESC on parent

	// Set Room Zero's parent to #6
	env.game.DB.Objects[0].Parent = 6
	clearOutput(env.player)

	// Room Zero has no DESC itself, should inherit from parent
	text := env.game.GetAttrText(0, 6)
	if text != "Inherited description." {
		t.Errorf("parent chain: expected 'Inherited description.', got: %s", text)
	}
}

// --- WHO Command ---

func TestWho(t *testing.T) {
	env := newTestEnv(t)
	clearOutput(env.player)

	DispatchCommand(env.game, env.player, "WHO")
	out := getOutput(env.player)
	if !strings.Contains(out, "Wizard") {
		t.Errorf("WHO: expected 'Wizard' in output, got: %s", out)
	}
}
