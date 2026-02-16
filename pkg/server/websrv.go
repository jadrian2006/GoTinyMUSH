package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/admin"
	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"github.com/gorilla/websocket"
)

// WebConfig holds configuration for the web server.
type WebConfig struct {
	Port        int
	Host        string
	Domain      string
	CertFile    string
	KeyFile     string
	CertDir     string
	StaticDir   string
	ClientURL   string // URL of external web client container; if set, / is reverse-proxied to it
	CORSOrigins []string
	RateLimit   int
	JWTSecret   string
	JWTExpiry   int
}

// WebServer provides HTTP/WebSocket transport alongside the TCP game server.
type WebServer struct {
	game      *Game
	httpSrv   *http.Server
	mux       *http.ServeMux
	auth      *AuthService
	rl        *rateLimiter
	upgrader  websocket.Upgrader
	admin     *admin.Admin
	ctrl      *gameServerController
	metrics   *Metrics
	startTime time.Time
}

// SetServer gives the admin controller a reference to the Server for shutdown support.
func (ws *WebServer) SetServer(s *Server) {
	if ws.ctrl != nil {
		ws.ctrl.server = s
	}
}

// NewWebServer creates a web server bound to the game.
func NewWebServer(game *Game, cfg WebConfig) *WebServer {
	auth := NewAuthService(game, cfg.JWTSecret, cfg.JWTExpiry)
	rl := newRateLimiter(cfg.RateLimit)

	ws := &WebServer{
		game:      game,
		mux:       http.NewServeMux(),
		auth:      auth,
		rl:        rl,
		startTime: time.Now(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if len(cfg.CORSOrigins) == 0 {
					return true
				}
				origin := r.Header.Get("Origin")
				for _, o := range cfg.CORSOrigins {
					if strings.EqualFold(o, origin) {
						return true
					}
				}
				return false
			},
		},
	}

	ws.registerRoutes(cfg)
	return ws
}

// Auth returns the auth service for external use (e.g., REST handlers).
func (ws *WebServer) Auth() *AuthService {
	return ws.auth
}

// registerRoutes sets up all HTTP routes.
func (ws *WebServer) registerRoutes(cfg WebConfig) {
	// Apply global middleware: CORS -> rate limit
	handler := http.Handler(ws.mux)
	handler = rateLimitMiddleware(ws.rl, handler)
	handler = corsMiddleware(cfg.CORSOrigins, handler)

	ws.httpSrv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: handler,
	}

	// WebSocket endpoint
	ws.mux.HandleFunc("GET /ws", ws.handleWebSocket)

	// Auth endpoints
	ws.mux.HandleFunc("POST /api/v1/auth/login", ws.handleAuthLogin)
	ws.mux.HandleFunc("POST /api/v1/auth/refresh", ws.handleAuthRefresh)

	// REST API endpoints
	ws.RegisterRESTRoutes()

	// Health endpoint (no auth, before admin)
	ws.mux.HandleFunc("GET /health", ws.handleHealth)

	// Prometheus metrics endpoint
	ws.metrics = NewMetrics(ws.game, time.Now())
	ws.mux.Handle("GET /metrics", ws.metrics.Handler())

	// Admin panel
	ctrl := &gameServerController{game: ws.game, running: true, startTime: time.Now()}
	ws.ctrl = ctrl
	ws.admin = admin.New(ctrl)
	if ws.game.ConfPath != "" {
		ws.admin.SetDataDir(filepath.Dir(ws.game.ConfPath))
		ws.admin.SetConfPath(ws.game.ConfPath)
	}
	ws.mux.Handle("/admin/", ws.admin.Handler("/admin"))

	// Root "/" handler: reverse proxy to web client container, serve local SPA, or redirect to /admin.
	// NOTE: Must use method-less pattern "/" (not "GET /") to avoid Go 1.22 mux conflict
	// with the method-less "/admin/" pattern registered above.
	if cfg.ClientURL != "" {
		// Reverse proxy to external web client container
		target, err := url.Parse(cfg.ClientURL)
		if err != nil {
			log.Printf("web: invalid web_client_url %q: %v", cfg.ClientURL, err)
		} else {
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("web: client proxy error: %v", err)
				// Fall back to redirect to admin if client is down
				http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
			}
			ws.mux.Handle("/", proxy)
			log.Printf("web: proxying / to web client at %s", cfg.ClientURL)
		}
	} else if cfg.StaticDir != "" {
		// Serve local static SPA files
		if _, err := os.Stat(cfg.StaticDir); err == nil {
			fsrv := http.FileServer(http.Dir(cfg.StaticDir))
			ws.mux.Handle("/", spaHandler(fsrv, cfg.StaticDir))
		} else {
			// StaticDir configured but doesn't exist — redirect to admin
			ws.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
			})
		}
	} else {
		// No web client configured — redirect to admin panel
		ws.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
		})
	}
}

// Start begins listening. Uses HTTPS when TLS certs are available,
// falls back to plain HTTP otherwise (development mode).
func (ws *WebServer) Start(cfg WebConfig) error {
	// Rate limiter cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			ws.rl.cleanup()
		}
	}()

	// Try TLS setup; fall back to HTTP if no certs available
	hasTLS := cfg.Domain != "" || (cfg.CertFile != "" && cfg.KeyFile != "") || cfg.CertDir != ""
	if hasTLS {
		result, err := SetupTLS(cfg.Domain, cfg.CertFile, cfg.KeyFile, cfg.CertDir)
		if err != nil {
			log.Printf("web: TLS setup failed (%v), falling back to HTTP", err)
		} else {
			ws.httpSrv.TLSConfig = result.Config

			// If using Let's Encrypt, start HTTP listener on port 80 for ACME challenges
			// and to redirect HTTP -> HTTPS.
			if result.AutocertMgr != nil {
				go func() {
					httpSrv := &http.Server{
						Addr:    ":80",
						Handler: result.AutocertMgr.HTTPHandler(nil),
					}
					log.Printf("ACME HTTP challenge listener on :80")
					if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.Printf("ACME HTTP listener error: %v", err)
					}
				}()
			}

			log.Printf("Web server listening on %s (HTTPS)", ws.httpSrv.Addr)
			err = ws.httpSrv.ListenAndServeTLS("", "")
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		}
	}

	// Plain HTTP fallback
	log.Printf("Web server listening on %s (HTTP)", ws.httpSrv.Addr)
	err := ws.httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Stop gracefully shuts down the web server.
func (ws *WebServer) Stop(ctx context.Context) error {
	return ws.httpSrv.Shutdown(ctx)
}

// --- WebSocket Handler ---

// WSMessage is the JSON message format for WebSocket communication.
type WSMessage struct {
	Type    string         `json:"type"`
	Text    string         `json:"text,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
	Channel string         `json:"channel,omitempty"`
	Command string         `json:"command,omitempty"`
}

// handleWebSocket upgrades an HTTP connection to a WebSocket and creates
// a game Descriptor for the client.
func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query param or header
	var claims *Claims
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = authHeader[7:]
		}
	}
	if token != "" {
		var err error
		claims, err = ws.auth.ValidateToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
	}

	wsConn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	// Use X-Forwarded-For or X-Real-IP if behind a reverse proxy (e.g. Docker)
	remoteAddr := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can be comma-separated; first entry is the real client
		if idx := strings.Index(xff, ","); idx >= 0 {
			remoteAddr = strings.TrimSpace(xff[:idx])
		} else {
			remoteAddr = strings.TrimSpace(xff)
		}
	} else if xri := r.Header.Get("X-Real-IP"); xri != "" {
		remoteAddr = strings.TrimSpace(xri)
	}
	d, wc := newWSDescriptor(ws.game, wsConn, remoteAddr)
	ws.game.Conns.Add(d)

	if claims != nil {
		// Auto-login
		ws.game.Conns.Login(d, claims.PlayerRef)
		wc.sendJSON(WSMessage{
			Type: "login",
			Data: map[string]any{
				"player_ref":  int(claims.PlayerRef),
				"player_name": claims.PlayerName,
			},
		})
		// Show room
		loc := ws.game.PlayerLocation(claims.PlayerRef)
		ws.game.ShowRoom(d, loc)
	} else {
		wc.sendJSON(WSMessage{Type: "welcome", Text: "Connected. Send {\"type\":\"login\",\"command\":\"connect name password\"} to authenticate."})
	}

	// Read loop
	go wsReadLoop(ws, d, wc)
}

// wsConn holds the WebSocket connection and its write mutex.
type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (wc *wsConn) sendJSON(msg WSMessage) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	wc.conn.WriteJSON(msg)
}

// newWSDescriptor creates a Descriptor configured for WebSocket transport.
// The Descriptor's SendFunc and ReceiveFunc are wired to write JSON to the WS conn.
func newWSDescriptor(game *Game, conn *websocket.Conn, addr string) (*Descriptor, *wsConn) {
	wc := &wsConn{conn: conn}
	id := game.Conns.NextID()
	d := &Descriptor{
		ID:        id,
		Conn:      nullConn{}, // No raw TCP conn for WS
		State:     ConnLogin,
		Player:    gamedb.Nothing,
		Addr:      addr,
		ConnTime:  time.Now(),
		LastCmd:   time.Now(),
		Retries:   3,
		Transport: TransportWebSocket,
	}
	d.SendFunc = func(msg string) {
		wc.sendJSON(WSMessage{Type: "text", Text: msg})
	}
	d.ReceiveFunc = func(ev events.Event) {
		wc.sendJSON(WSMessage{
			Type:    ev.Type.String(),
			Text:    ev.Text,
			Data:    ev.Data,
			Channel: ev.Channel,
		})
	}
	return d, wc
}

func wsReadLoop(ws *WebServer, d *Descriptor, wc *wsConn) {
	defer func() {
		ws.game.DisconnectPlayer(d)
		ws.game.Conns.Remove(d)
		wc.conn.Close()
		log.Printf("[ws:%d] WebSocket closed from %s", d.ID, d.Addr)
	}()

	for {
		_, msgBytes, err := wc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[ws:%d] read error: %v", d.ID, err)
			}
			return
		}

		d.LastCmd = time.Now()

		var msg WSMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			wc.sendJSON(WSMessage{Type: "error", Text: "Invalid JSON message"})
			continue
		}

		switch msg.Type {
		case "command":
			if d.State == ConnLogin {
				handleWSLogin(ws, d, wc, msg.Command)
			} else {
				d.CmdCount++
				DispatchCommand(ws.game, d, msg.Command)
			}
		case "login":
			handleWSLogin(ws, d, wc, msg.Command)
		default:
			wc.sendJSON(WSMessage{Type: "error", Text: fmt.Sprintf("Unknown message type: %s", msg.Type)})
		}
	}
}

func handleWSLogin(ws *WebServer, d *Descriptor, wc *wsConn, input string) {
	command, user, password := ParseConnect(input)
	if strings.HasPrefix(command, "co") {
		player := LookupPlayer(ws.game.DB, user)
		if player == gamedb.Nothing || !CheckPassword(ws.game.DB, player, password) {
			wc.sendJSON(WSMessage{Type: "error", Text: "Invalid credentials"})
			return
		}
		ws.game.Conns.Login(d, player)
		playerName := ws.game.PlayerName(player)
		wc.sendJSON(WSMessage{
			Type: "login",
			Data: map[string]any{
				"player_ref":  int(player),
				"player_name": playerName,
			},
		})
		loc := ws.game.PlayerLocation(player)
		ws.game.ShowRoom(d, loc)
	} else {
		wc.sendJSON(WSMessage{Type: "error", Text: "Use: connect <name> <password>"})
	}
}

// --- Auth HTTP Handlers ---

func (ws *WebServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	token, err := ws.auth.Login(req.Name, req.Password)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (ws *WebServer) handleAuthRefresh(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"error":"authorization required"}`, http.StatusUnauthorized)
		return
	}
	token := authHeader[7:]
	newToken, err := ws.auth.RefreshToken(token)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": newToken})
}

// --- Health Handler ---

func (ws *WebServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"version":        Version,
		"uptime_seconds": time.Since(ws.startTime).Seconds(),
		"game_running":   true,
	})
}

// --- SPA Handler ---

// spaHandler serves static files, falling back to index.html for SPA routing.
func spaHandler(fileServer http.Handler, staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := staticDir + r.URL.Path
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// SPA fallback: serve index.html for non-existent paths
			http.ServeFile(w, r, staticDir+"/index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
