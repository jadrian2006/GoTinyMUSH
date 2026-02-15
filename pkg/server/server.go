package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"github.com/crystal-mush/gotinymush/pkg/oob"
)

// Config holds server configuration.
type Config struct {
	Port        int
	IdleTimeout time.Duration
	MaxRetries  int
	WelcomeText string
	Cleartext   bool
	TLS         bool
	TLSPort     int
	TLSCert     string
	TLSKey      string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Port:        6250,
		IdleTimeout: 3600 * time.Second,
		MaxRetries:  3,
		WelcomeText: WelcomeText,
		Cleartext:   true,
	}
}

// Server is the main TCP game server.
type Server struct {
	Config      Config
	Game        *Game
	listener    net.Listener
	tlsListener net.Listener
	webServer   *WebServer
}

// NewServer creates a new server instance.
func NewServer(db *gamedb.Database, cfg Config) *Server {
	game := NewGame(db)
	return &Server{
		Config: cfg,
		Game:   game,
	}
}

// Start begins listening for connections.
func (s *Server) Start() error {
	if !s.Config.Cleartext && !s.Config.TLS {
		return fmt.Errorf("both cleartext and TLS listeners are disabled; nothing to listen on")
	}

	// Start the command queue processor
	s.Game.StartQueueProcessor()

	// Start scrollback writer if SQL is configured
	if s.Game.SQLDB != nil {
		sw := NewScrollbackWriter(s.Game)
		if sw != nil {
			retention := 24 * time.Hour
			if s.Game.Conf != nil && s.Game.Conf.ScrollbackRetention > 0 {
				retention = time.Duration(s.Game.Conf.ScrollbackRetention) * time.Second
			}
			StartRetentionCleanup(s.Game.SQLDB, retention)
		}
	}

	// Start periodic auto-save (every 30 minutes)
	if s.Game.DBPath != "" {
		s.Game.StartAutoSave(30)
	}

	log.Printf("Database: %d objects, %d attribute definitions",
		len(s.Game.DB.Objects), len(s.Game.DB.AttrNames))

	// Count players in DB
	playerCount := 0
	for _, obj := range s.Game.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() {
			playerCount++
		}
	}
	log.Printf("Players in database: %d", playerCount)

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	if s.Config.Cleartext {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Config.Port))
			if err != nil {
				errCh <- fmt.Errorf("cleartext listener: %w", err)
				return
			}
			s.listener = ln
			log.Printf("Listening (cleartext) on port %d", s.Config.Port)
			s.acceptLoop(ln)
		}()
	}

	if s.Config.TLS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cert, err := tls.LoadX509KeyPair(s.Config.TLSCert, s.Config.TLSKey)
			if err != nil {
				errCh <- fmt.Errorf("TLS cert load: %w", err)
				return
			}
			tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
			ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", s.Config.TLSPort), tlsCfg)
			if err != nil {
				errCh <- fmt.Errorf("TLS listener: %w", err)
				return
			}
			s.tlsListener = ln
			log.Printf("Listening (TLS) on port %d", s.Config.TLSPort)
			s.acceptLoop(ln)
		}()
	}

	// Start web server if enabled
	if s.Game.Conf != nil && s.Game.Conf.WebEnabled {
		cfg := WebConfig{
			Port:        s.Game.Conf.WebPort,
			Host:        s.Game.Conf.WebHost,
			Domain:      s.Game.Conf.WebDomain,
			CertFile:    s.Game.Conf.TLSCert,
			KeyFile:     s.Game.Conf.TLSKey,
			CertDir:     s.Game.Conf.CertDir,
			StaticDir:   s.Game.Conf.WebStaticDir,
			ClientURL:   s.Game.Conf.WebClientURL,
			CORSOrigins: s.Game.Conf.WebCORSOrigins,
			RateLimit:   s.Game.Conf.WebRateLimit,
			JWTSecret:   s.Game.Conf.JWTSecret,
			JWTExpiry:   s.Game.Conf.JWTExpiry,
		}
		s.webServer = NewWebServer(s.Game, cfg)
		s.webServer.SetServer(s)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.webServer.Start(cfg); err != nil {
				errCh <- fmt.Errorf("web server: %w", err)
			}
		}()
	}

	// Check for early startup errors
	select {
	case err := <-errCh:
		return err
	default:
	}

	wg.Wait()
	return nil
}

// acceptLoop accepts connections on the given listener until it is closed.
func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("Accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

// Stop closes all active listeners.
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.tlsListener != nil {
		s.tlsListener.Close()
	}
	if s.webServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.webServer.Stop(ctx)
	}
}

// handleConnection manages a single client connection lifecycle.
func (s *Server) handleConnection(conn net.Conn) {
	id := s.Game.Conns.NextID()
	d := NewDescriptor(id, conn)
	s.Game.Conns.Add(d)

	log.Printf("[%d] New connection from %s", d.ID, d.Addr)

	// OOB protocol negotiation (GMCP/MSDP) with 1-second timeout.
	// Non-OOB clients simply don't respond and we move on.
	caps := oob.Negotiate(conn, 1*time.Second)
	if caps.HasAny() {
		d.OOB = caps
		log.Printf("[%d] OOB negotiated: GMCP=%v MSDP=%v", d.ID, caps.GMCP, caps.MSDP)
	}

	defer func() {
		s.Game.DisconnectPlayer(d)
		s.Game.Conns.Remove(d)
		d.Close()
		log.Printf("[%d] Connection closed from %s", d.ID, d.Addr)
	}()

	// Send Pueblo version string if enabled (before welcome screen)
	if s.Game.Conf != nil && s.Game.Conf.PuebloEnabled && s.Game.Conf.PuebloVersion != "" {
		d.Send(s.Game.Conf.PuebloVersion)
	}

	// Send welcome screen
	if s.Game.Texts != nil {
		if txt := s.Game.Texts.GetConnect(); txt != "" {
			d.SendNoNewline(txt)
		} else {
			d.SendNoNewline(s.Config.WelcomeText)
		}
	} else {
		d.SendNoNewline(s.Config.WelcomeText)
	}

	// Main read loop
	scanner := bufio.NewScanner(d.Conn)
	scanner.Buffer(make([]byte, 8192), 8192)

	for scanner.Scan() {
		if d.IsClosed() {
			return
		}

		line := scanner.Text()
		d.BytesRecv += len(line) + 1 // +1 for newline
		// Strip telnet control sequences (IAC sequences)
		line = stripTelnet(line)
		line = strings.TrimRight(line, "\r\n")
		d.LastCmd = time.Now()
		if d.State == ConnConnected {
			d.CmdCount++
		}

		if d.State == ConnLogin {
			s.handleLoginCommand(d, line)
		} else {
			// Undo AutoDark on first command input (matches C DS_AUTODARK behavior)
			if d.AutoDark {
				d.AutoDark = false
				if playerObj, ok := s.Game.DB.Objects[d.Player]; ok {
					playerObj.Flags[0] &^= gamedb.FlagDark
				}
			}
			log.Printf("[%d] CMD state=%d player=#%d input=%q", d.ID, d.State, d.Player, line)
			if d.ProgData != nil {
				if strings.HasPrefix(line, "|") {
					// Pipe escape: execute remainder as normal command
					DispatchCommand(s.Game, d, line[1:])
					// Re-send prompt if still in program mode
					if d.ProgData != nil {
						d.SendNoNewline(progPrompt)
					}
				} else if strings.EqualFold(strings.TrimSpace(line), "@quitprogram") {
					// Allow @quitprogram to work normally
					DispatchCommand(s.Game, d, line)
				} else {
					// Feed input to program handler
					s.Game.HandleProgInput(d, line)
				}
			} else {
				DispatchCommand(s.Game, d, line)
			}
		}

		if d.IsClosed() {
			return
		}
	}
}

// handleLoginCommand processes pre-login commands.
func (s *Server) handleLoginCommand(d *Descriptor, input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Pre-login commands
	upper := strings.ToUpper(input)
	if upper == "QUIT" {
		if s.Game.Texts != nil {
			if txt := s.Game.Texts.GetQuit(); txt != "" {
				d.SendNoNewline(txt)
			} else {
				d.Send("Goodbye!")
			}
		} else {
			d.Send("Goodbye!")
		}
		d.Close()
		return
	}
	if upper == "WHO" {
		s.Game.ShowWho(d)
		return
	}
	if strings.HasPrefix(upper, "PUEBLOCLIENT") {
		if s.Game.Conf != nil && s.Game.Conf.PuebloEnabled {
			d.Pueblo = true
			// Send HTML mode activation and HTML welcome screen
			d.Send("</xch_mudtext><img xch_mode=html><tt>")
			if s.Game.Texts != nil {
				if txt := s.Game.Texts.GetHTMLConn(); txt != "" {
					d.SendNoNewline(txt)
				}
			}
			log.Printf("[%d] Pueblo client detected", d.ID)
		}
		return
	}

	command, user, password := ParseConnect(input)

	switch {
	case strings.HasPrefix(command, "cd"): // connect dark (cd <name> <password>)
		s.handleConnect(d, user, password, true)

	case strings.HasPrefix(command, "co"): // connect
		// Detect guest login: "connect guest", "connect guest guest", etc.
		if strings.EqualFold(user, "guest") {
			s.handleGuest(d)
			return
		}
		s.handleConnect(d, user, password, false)

	case strings.HasPrefix(command, "cr"): // create
		s.handleCreate(d, user, password)

	default:
		d.Send("Welcome to GoTinyMUSH. Commands: connect, create, WHO, QUIT")
	}
}

// handleConnect authenticates and logs in a player.
// If dark is true and the player is a wizard/god, set DARK flag on connect.
func (s *Server) handleConnect(d *Descriptor, user, password string, dark bool) {
	if user == "" {
		d.Send("Usage: connect <name> <password>")
		return
	}

	player := LookupPlayer(s.Game.DB, user)
	if player == gamedb.Nothing {
		d.Send("Either that player does not exist, or has a different password.")
		d.Retries--
		if d.Retries <= 0 {
			d.Send("Too many failed attempts. Disconnecting.")
			d.Close()
		}
		return
	}

	if !CheckPassword(s.Game.DB, player, password) {
		d.Send("Either that player does not exist, or has a different password.")
		d.Retries--
		if d.Retries <= 0 {
			d.Send("Too many failed attempts. Disconnecting.")
			d.Close()
		}
		return
	}

	// Successful login
	s.Game.Conns.Login(d, player)
	playerObj := s.Game.DB.Objects[player]

	// Connect dark: set DARK flag if wizard/god requested it
	if dark && (Wizard(s.Game, player) || player == gamedb.DBRef(1)) {
		playerObj.Flags[0] |= gamedb.FlagDark
		d.AutoDark = true
		log.Printf("[%d] Player %s(#%d) DARK-connected from %s", d.ID, playerObj.Name, player, d.Addr)
	} else {
		log.Printf("[%d] Player %s(#%d) connected from %s", d.ID, playerObj.Name, player, d.Addr)
	}

	d.Send(fmt.Sprintf("Welcome back, %s!", playerObj.Name))

	// Show MOTD if available
	if s.Game.Texts != nil {
		if txt := s.Game.Texts.GetMotd(); txt != "" {
			d.SendNoNewline(txt)
		}
		// Show wizard MOTD if player is a wizard
		if Wizard(s.Game, d.Player) {
			if txt := s.Game.Texts.GetWizMotd(); txt != "" {
				d.SendNoNewline(txt)
			}
		}
	}

	// Announce to room (suppress if dark-connected)
	loc := playerObj.Location
	if !d.AutoDark {
		s.Game.Conns.SendToRoomExcept(s.Game.DB, loc, player,
			fmt.Sprintf("%s has connected.", playerObj.Name))
	}

	// Show current room
	s.Game.ShowRoom(d, loc)

	// Announce unread mail
	if s.Game.Mail != nil {
		total, unread, _ := s.Game.Mail.CountMessages(player)
		if total > 0 && unread > 0 {
			d.Send(fmt.Sprintf("You have %d unread mail message(s). Type @mail to read.", unread))
		}
	}

	// Fire ACONNECT triggers
	connCount := len(s.Game.Conns.GetByPlayer(player))
	s.Game.FireConnectAttr(player, connCount, 39) // A_ACONNECT = 39
}

// handleCreate creates a new player and logs them in.
func (s *Server) handleCreate(d *Descriptor, user, password string) {
	if user == "" || password == "" {
		d.Send("Usage: create <name> <password>")
		return
	}

	// Check if name already exists
	if LookupPlayer(s.Game.DB, user) != gamedb.Nothing {
		d.Send("That name is already taken.")
		return
	}

	// Validate name
	if len(user) < 2 {
		d.Send("That name is too short.")
		return
	}
	for _, ch := range user {
		if ch == '"' || ch == ';' {
			d.Send("That name contains illegal characters.")
			return
		}
	}
	if s.Game.IsBadName(user) {
		d.Send("That name is not allowed.")
		return
	}

	// Create the player object
	ref := s.Game.CreateObject(user, gamedb.TypePlayer, gamedb.Nothing)
	playerObj := s.Game.DB.Objects[ref]
	playerObj.Owner = ref

	// Set password (plaintext for now, TODO: add encryption)
	s.Game.SetAttr(ref, aPass, password)

	// Set start room and home from config
	startRoom := s.Game.StartingRoom()
	startHome := s.Game.StartingHome()
	playerObj.Location = startRoom
	playerObj.Link = startHome // home

	// Add to start room contents
	if roomObj, ok := s.Game.DB.Objects[startRoom]; ok {
		playerObj.Next = roomObj.Contents
		roomObj.Contents = ref
		s.Game.PersistObjects(playerObj, roomObj)
	}
	if s.Game.Store != nil {
		s.Game.Store.PutMeta()
		s.Game.Store.UpdatePlayerIndex(playerObj, "")
	}

	log.Printf("[%d] New player %s(#%d) created from %s", d.ID, user, ref, d.Addr)

	// Log them in
	s.Game.Conns.Login(d, ref)

	d.Send(fmt.Sprintf("Welcome to GoTinyMUSH, %s! Your character has been created as #%d.", user, ref))

	// Show new user text if available
	if s.Game.Texts != nil {
		if txt := s.Game.Texts.GetNewUser(); txt != "" {
			d.SendNoNewline(txt)
		}
	}

	// Show room
	s.Game.ShowRoom(d, startRoom)
}

// stripTelnet removes telnet IAC command sequences from input.
func stripTelnet(s string) string {
	var buf strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0xFF && i+2 < len(s) {
			// IAC command: skip 3 bytes (IAC + cmd + option)
			i += 3
			continue
		}
		if s[i] == 0xFF && i+1 < len(s) {
			i += 2
			continue
		}
		// Skip other control chars except tab and standard whitespace
		if s[i] < 32 && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			i++
			continue
		}
		buf.WriteByte(s[i])
		i++
	}
	return buf.String()
}
