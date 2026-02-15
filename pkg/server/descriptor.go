package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"github.com/crystal-mush/gotinymush/pkg/oob"
)

// TransportType identifies the kind of transport a Descriptor uses.
type TransportType int

const (
	TransportTCP       TransportType = iota // Traditional telnet/TCP
	TransportWebSocket                      // WebSocket (JSON events)
)

// ConnState tracks the state of a connection.
type ConnState int

const (
	ConnLogin     ConnState = iota // Pre-login: awaiting connect/create
	ConnConnected                  // Logged in as a player
)

// Descriptor represents a single client connection.
// It implements events.Subscriber so it can receive events from the bus.
type Descriptor struct {
	ID        int
	Conn      net.Conn
	Reader    *bufio.Reader
	State     ConnState
	Player    gamedb.DBRef
	Addr      string
	ConnTime  time.Time
	LastCmd   time.Time
	Retries   int
	IdleTime  time.Duration
	DoingStr  string // @doing text
	ProgData  *ProgramData // Active @program state (nil = not programmed)
	LastRData *eval.RegisterData // Snapshot of q-registers during queue execution (for @program)
	CmdCount  int    // Total commands entered this session
	BytesSent int    // Total bytes sent to this connection
	BytesRecv int    // Total bytes received from this connection
	Transport TransportType // Transport type (TCP, WebSocket)
	AutoDark  bool         // Wizard connected dark; cleared on first command input
	OOB       *oob.Capabilities // Negotiated OOB protocols (nil = none)

	// SendFunc overrides the default Send behavior (used by WebSocket transport).
	// If nil, the default TCP Send is used.
	SendFunc    func(msg string)
	// ReceiveFunc overrides the default Receive behavior (used by WebSocket transport).
	// If nil, the default event→text→Send path is used.
	ReceiveFunc func(ev events.Event)

	mu        sync.Mutex
	closed    bool
}

// NewDescriptor wraps a net.Conn into a Descriptor.
func NewDescriptor(id int, conn net.Conn) *Descriptor {
	now := time.Now()
	return &Descriptor{
		ID:       id,
		Conn:     conn,
		Reader:   bufio.NewReaderSize(conn, 4096),
		State:    ConnLogin,
		Player:   gamedb.Nothing,
		Addr:     conn.RemoteAddr().String(),
		ConnTime: now,
		LastCmd:  now,
		Retries:  3,
	}
}

// Send writes a string to the client connection.
func (d *Descriptor) Send(msg string) {
	if d.SendFunc != nil {
		d.SendFunc(msg)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	// Ensure lines end with \r\n for telnet
	if !strings.HasSuffix(msg, "\n") {
		msg += "\r\n"
	}
	d.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	n, _ := d.Conn.Write([]byte(msg))
	d.BytesSent += n
}

// SendRaw writes raw bytes to the connection (no newline, no encoding).
// Used for telnet subnegotiation sequences (GMCP, MSDP).
func (d *Descriptor) SendRaw(data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	n, _ := d.Conn.Write(data)
	d.BytesSent += n
}

// SendNoNewline writes a string without appending a newline.
func (d *Descriptor) SendNoNewline(msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	n, _ := d.Conn.Write([]byte(msg))
	d.BytesSent += n
}

// Close shuts down the connection.
func (d *Descriptor) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		d.Conn.Close()
	}
}

// IsClosed returns whether the connection has been closed.
func (d *Descriptor) IsClosed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// Receive implements events.Subscriber. It delivers an event to the client
// using the appropriate encoding for this transport.
func (d *Descriptor) Receive(ev events.Event) {
	if d.ReceiveFunc != nil {
		d.ReceiveFunc(ev)
		return
	}

	// Send text form (universal)
	if ev.Text != "" {
		d.Send(ev.Text)
	}

	// Send OOB side-channel data if negotiated
	if d.OOB != nil && ev.Data != nil {
		if d.OOB.GMCP {
			if gmcpBuf := oob.EncodeGMCP(ev); gmcpBuf != nil {
				d.SendRaw(gmcpBuf)
			}
		}
		if d.OOB.MSDP {
			if msdpBuf := oob.EncodeMSDPEvent(ev); msdpBuf != nil {
				d.SendRaw(msdpBuf)
			}
		}
	}
}

// Closed implements events.Subscriber.
func (d *Descriptor) Closed() bool {
	return d.IsClosed()
}

// Compile-time check that Descriptor implements events.Subscriber.
var _ events.Subscriber = (*Descriptor)(nil)

// nullConn is a no-op net.Conn used for synthetic descriptors (non-connected objects).
type nullConn struct{}

func (nullConn) Read([]byte) (int, error)         { return 0, fmt.Errorf("no connection") }
func (nullConn) Write(b []byte) (int, error)       { return len(b), nil }
func (nullConn) Close() error                      { return nil }
func (nullConn) LocalAddr() net.Addr               { return nil }
func (nullConn) RemoteAddr() net.Addr              { return &net.TCPAddr{} }
func (nullConn) SetDeadline(time.Time) error       { return nil }
func (nullConn) SetReadDeadline(time.Time) error   { return nil }
func (nullConn) SetWriteDeadline(time.Time) error  { return nil }

// MakeObjDescriptor creates a synthetic Descriptor for a non-connected object.
// Output is discarded (STARTUP commands don't need visible output).
func (g *Game) MakeObjDescriptor(player gamedb.DBRef) *Descriptor {
	return &Descriptor{
		ID:       -1,
		Conn:     nullConn{},
		State:    ConnConnected,
		Player:   player,
		Addr:     "internal",
		ConnTime: time.Now(),
		LastCmd:  time.Now(),
	}
}

// ConnManager tracks all active connections.
type ConnManager struct {
	mu          sync.RWMutex
	descriptors map[int]*Descriptor
	nextID      int
	byPlayer    map[gamedb.DBRef][]*Descriptor // player -> connections (multi-login)
	EventBus    *events.Bus                    // Event bus for pub/sub (nil = disabled)
}

// NewConnManager creates a new connection manager.
func NewConnManager() *ConnManager {
	return &ConnManager{
		descriptors: make(map[int]*Descriptor),
		byPlayer:    make(map[gamedb.DBRef][]*Descriptor),
		nextID:      1,
	}
}

// Add registers a new descriptor.
func (cm *ConnManager) Add(d *Descriptor) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.descriptors[d.ID] = d
}

// Remove unregisters a descriptor and unsubscribes it from the event bus.
func (cm *ConnManager) Remove(d *Descriptor) {
	if cm.EventBus != nil && d.Player != gamedb.Nothing {
		cm.EventBus.Unsubscribe(d.Player, d)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.descriptors, d.ID)
	if d.Player != gamedb.Nothing {
		descs := cm.byPlayer[d.Player]
		for i, dd := range descs {
			if dd.ID == d.ID {
				cm.byPlayer[d.Player] = append(descs[:i], descs[i+1:]...)
				break
			}
		}
		if len(cm.byPlayer[d.Player]) == 0 {
			delete(cm.byPlayer, d.Player)
		}
	}
}

// Login associates a descriptor with a player and subscribes it to the event bus.
func (cm *ConnManager) Login(d *Descriptor, player gamedb.DBRef) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	d.State = ConnConnected
	d.Player = player
	cm.byPlayer[player] = append(cm.byPlayer[player], d)

	if cm.EventBus != nil {
		cm.EventBus.Subscribe(player, d)
	}
}

// NextID returns the next descriptor ID.
func (cm *ConnManager) NextID() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	id := cm.nextID
	cm.nextID++
	return id
}

// GetByPlayer returns all descriptors for a given player.
func (cm *ConnManager) GetByPlayer(player gamedb.DBRef) []*Descriptor {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.byPlayer[player]
}

// IsConnected returns true if the player has at least one active connection.
func (cm *ConnManager) IsConnected(player gamedb.DBRef) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.byPlayer[player]) > 0
}

// ConnectedPlayers returns all currently connected player dbrefs.
func (cm *ConnManager) ConnectedPlayers() []gamedb.DBRef {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	players := make([]gamedb.DBRef, 0, len(cm.byPlayer))
	for p := range cm.byPlayer {
		players = append(players, p)
	}
	return players
}

// AllDescriptors returns a snapshot of all active descriptors.
func (cm *ConnManager) AllDescriptors() []*Descriptor {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	descs := make([]*Descriptor, 0, len(cm.descriptors))
	for _, d := range cm.descriptors {
		descs = append(descs, d)
	}
	return descs
}

// Count returns the number of active connections.
func (cm *ConnManager) Count() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.descriptors)
}

// SendToPlayer sends a message to all connections of a player.
func (cm *ConnManager) SendToPlayer(player gamedb.DBRef, msg string) {
	cm.mu.RLock()
	descs := cm.byPlayer[player]
	cm.mu.RUnlock()
	for _, d := range descs {
		d.Send(msg)
	}
}

// SendToRoom sends a message to all connected players in a room.
func (cm *ConnManager) SendToRoom(db *gamedb.Database, room gamedb.DBRef, msg string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Walk the contents chain of the room
	roomObj, ok := db.Objects[room]
	if !ok {
		return
	}
	next := roomObj.Contents
	for next != gamedb.Nothing {
		if descs, ok := cm.byPlayer[next]; ok {
			for _, d := range descs {
				d.Send(msg)
			}
		}
		obj, ok := db.Objects[next]
		if !ok {
			break
		}
		next = obj.Next
	}
}

// SendToRoomExcept sends a message to all connected players in a room except one.
func (cm *ConnManager) SendToRoomExcept(db *gamedb.Database, room gamedb.DBRef, except gamedb.DBRef, msg string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	roomObj, ok := db.Objects[room]
	if !ok {
		return
	}
	next := roomObj.Contents
	for next != gamedb.Nothing {
		if next != except {
			if descs, ok := cm.byPlayer[next]; ok {
				for _, d := range descs {
					d.Send(msg)
				}
			}
		}
		obj, ok := db.Objects[next]
		if !ok {
			break
		}
		next = obj.Next
	}
}

// MakeEvalContext creates an EvalContext for a connected player.
func MakeEvalContext(db *gamedb.Database, player gamedb.DBRef, registerFn func(*eval.EvalContext)) *eval.EvalContext {
	ctx := eval.NewEvalContext(db)
	ctx.Player = player
	ctx.Cause = player
	ctx.Caller = player
	if registerFn != nil {
		registerFn(ctx)
	}
	return ctx
}

// MakeEvalContextWithGame creates an EvalContext with GameState for connection queries.
func MakeEvalContextWithGame(g *Game, player gamedb.DBRef, registerFn func(*eval.EvalContext)) *eval.EvalContext {
	ctx := eval.NewEvalContext(g.DB)
	ctx.Player = player
	ctx.Cause = player
	ctx.Caller = player
	ctx.GameState = g
	ctx.VersionStr = VersionString()
	if g.Conf != nil {
		ctx.MudName = g.Conf.MudName
		ctx.FuncInvkLim = g.Conf.FunctionInvocationLimit
	}
	if registerFn != nil {
		registerFn(ctx)
	}
	applyGameFuncs(g, ctx)
	return ctx
}

// MakeEvalContextForObj creates an EvalContext where the executor (%!) is the
// given object and the enactor (%#) is the player who triggered the evaluation.
// This is the correct context for evaluating an object's attributes (DESC, etc.)
// where v(), get(me/...), etc. should resolve attributes on the object, not the player.
func MakeEvalContextForObj(g *Game, executor gamedb.DBRef, enactor gamedb.DBRef, registerFn func(*eval.EvalContext)) *eval.EvalContext {
	ctx := eval.NewEvalContext(g.DB)
	ctx.Player = executor
	ctx.Cause = enactor
	ctx.Caller = enactor
	ctx.GameState = g
	ctx.VersionStr = VersionString()
	if g.Conf != nil {
		ctx.MudName = g.Conf.MudName
		ctx.FuncInvkLim = g.Conf.FunctionInvocationLimit
	}
	if registerFn != nil {
		registerFn(ctx)
	}
	applyGameFuncs(g, ctx)
	return ctx
}

// applyGameFuncs copies @function-defined functions from Game to an EvalContext.
func applyGameFuncs(g *Game, ctx *eval.EvalContext) {
	if g == nil || g.GameFuncs == nil {
		return
	}
	for name, uf := range g.GameFuncs {
		ctx.UFunctions[name] = uf
	}
}

// FormatIdleTime formats a duration as a human-readable idle time.
func FormatIdleTime(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm", secs/60)
	}
	if secs < 86400 {
		return fmt.Sprintf("%dh", secs/3600)
	}
	return fmt.Sprintf("%dd", secs/86400)
}

// FormatConnTime formats a duration as connection time.
func FormatConnTime(d time.Duration) string {
	secs := int(d.Seconds())
	hours := secs / 3600
	mins := (secs % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}
