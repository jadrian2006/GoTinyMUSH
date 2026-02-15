// Package admin provides the web admin panel for GoTinyMUSH.
// It serves the admin SPA and API endpoints for server management,
// configuration, flatfile import/validation, and setup wizard flows.
package admin

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/crystal-mush/gotinymush/pkg/archive"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"github.com/crystal-mush/gotinymush/pkg/validate"
)

//go:embed dist/*
var adminDist embed.FS

// ServerController is the interface the admin panel uses to control the game server.
// This avoids a direct import cycle with the server package.
type ServerController interface {
	// IsRunning returns whether the game engine (telnet listener) is running.
	IsRunning() bool
	// StartGame starts the game engine. Returns error if already running or config issue.
	StartGame() error
	// StopGame stops the game engine gracefully.
	StopGame() error
	// PlayerCount returns the number of connected players.
	PlayerCount() int
	// Uptime returns the server uptime in seconds, or 0 if not running.
	Uptime() float64
	// GetDatabase returns the current in-memory database (nil if not loaded).
	GetDatabase() *gamedb.Database
	// GetConfPath returns the path to the game configuration file.
	GetConfPath() string
	// ConvertLegacyConfig parses a legacy .conf file and returns YAML bytes.
	ConvertLegacyConfig(confPath string) ([]byte, error)

	// Extended stats for the admin dashboard.
	GameName() string
	GameVersion() string
	GamePort() int
	ConnectionStats() map[string]any
	QueueStats() map[string]any
	MemoryStats() map[string]any
	GameStats() map[string]any
}

// FileRole describes what role a discovered file plays in an import.
type FileRole string

const (
	RoleFlatfile  FileRole = "flatfile"
	RoleComsys    FileRole = "comsys"
	RoleMainConf  FileRole = "main_config"
	RoleAliasConf FileRole = "alias_config"
	RoleTextFile  FileRole = "text"
	RoleDictFile   FileRole = "dict"
	RoleDiscarded  FileRole = "discarded"
	RoleUnknown    FileRole = "unknown"
)

// DiscoveredFile represents a file found during import discovery.
type DiscoveredFile struct {
	Path       string   `json:"path"`
	Role       FileRole `json:"role"`
	Confidence string   `json:"confidence"` // "high", "medium", "low"
	Size       int64    `json:"size"`
	Reason     string   `json:"reason"`
}

// ImportSession holds all state for an in-progress import operation.
type ImportSession struct {
	StagingDir string `json:"staging_dir"`
	Source     string `json:"source"` // "flatfile", "gotinymush_archive", "foreign_archive"

	Files []DiscoveredFile `json:"files"`

	FlatfileDB  *gamedb.Database   `json:"-"`
	Validator   *validate.Validator `json:"-"`
	Channels    []gamedb.Channel   `json:"-"`
	ChanAliases []gamedb.ChanAlias `json:"-"`

	// Staged file contents (editable before commit)
	Config    []byte            `json:"-"` // YAML bytes
	Aliases   map[string][]byte `json:"-"` // filename → content
	TextFiles map[string][]byte `json:"-"` // filename → content
	DictFiles map[string][]byte `json:"-"` // filename → content

	// Manifest (if GoTinyMUSH archive)
	Manifest *archive.Manifest `json:"manifest,omitempty"`

	// State flags
	ConfigReady    bool `json:"config_ready"`
	ComsysParsed   bool `json:"comsys_parsed"`
	ValidationDone bool `json:"validation_done"`
	ReadyToCommit  bool `json:"ready_to_commit"`

	// Legacy compat: upload result info
	UploadFile  string         `json:"upload_file,omitempty"`
	ObjectCount int            `json:"object_count,omitempty"`
	AttrDefs    int            `json:"attr_defs,omitempty"`
	TotalAttrs  int            `json:"total_attrs,omitempty"`
	TypeCounts  map[string]int `json:"type_counts,omitempty"`
}

// Admin is the admin panel HTTP handler.
type Admin struct {
	mu         sync.RWMutex
	controller ServerController
	setupMode  bool

	// Import session (persists across requests during an import flow)
	session *ImportSession

	// Setup mode paths (set when running without a controller)
	dataDir  string // data directory (e.g. /game/data)
	confPath string // config file path (e.g. /game/data/game.yaml)

	// Authentication
	auth *adminAuth
}

// New creates an Admin handler. If controller is nil, the admin panel
// starts in setup mode (no game running, limited API).
func New(controller ServerController) *Admin {
	a := &Admin{
		controller: controller,
		setupMode:  controller == nil || controller.GetDatabase() == nil,
		auth:       newAdminAuth(""),
	}
	return a
}

// SetDataDir sets the data directory for setup mode operations.
func (a *Admin) SetDataDir(dir string) {
	a.dataDir = dir
	a.auth = newAdminAuth(dir)
}

// SetConfPath sets the config file path for setup mode operations.
func (a *Admin) SetConfPath(path string) {
	a.confPath = path
}

// Handler returns an http.Handler that serves the admin panel at the given prefix.
// The prefix should be "/admin" (without trailing slash).
func (a *Admin) Handler(prefix string) http.Handler {
	mux := http.NewServeMux()

	// Auth routes (must be before auth middleware)
	mux.HandleFunc("POST /api/auth/login", a.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleAuthLogout)
	mux.HandleFunc("POST /api/auth/change-password", a.handleAuthChangePassword)
	mux.HandleFunc("GET /api/auth/status", a.handleAuthStatus)

	// API routes
	mux.HandleFunc("GET /api/server/status", a.handleServerStatus)
	mux.HandleFunc("POST /api/server/start", a.handleServerStart)
	mux.HandleFunc("POST /api/server/stop", a.handleServerStop)

	mux.HandleFunc("GET /api/config", a.handleGetConfig)
	mux.HandleFunc("PUT /api/config", a.handlePutConfig)

	// Import routes (existing)
	mux.HandleFunc("POST /api/import/upload", a.handleImportUpload)
	mux.HandleFunc("POST /api/import/validate", a.handleImportValidate)
	mux.HandleFunc("GET /api/import/findings", a.handleImportFindings)
	mux.HandleFunc("POST /api/import/fix", a.handleImportFix)
	mux.HandleFunc("POST /api/import/commit", a.handleImportCommit)

	// Import routes (new: session management, discovery, file editing)
	mux.HandleFunc("GET /api/import/session", a.handleImportSession)
	mux.HandleFunc("DELETE /api/import/session", a.handleImportReset)
	mux.HandleFunc("POST /api/import/discover", a.handleImportDiscover)
	mux.HandleFunc("POST /api/import/assign", a.handleImportAssign)
	mux.HandleFunc("POST /api/import/convert-config", a.handleImportConvertConfig)
	mux.HandleFunc("POST /api/import/parse-comsys", a.handleImportParseComsys)
	mux.HandleFunc("GET /api/import/file/{role}/{name}", a.handleImportFileRead)
	mux.HandleFunc("PUT /api/import/file/{role}/{name}", a.handleImportFileWrite)

	mux.HandleFunc("GET /api/setup/status", a.handleSetupStatus)
	mux.HandleFunc("POST /api/import/create-new", a.handleCreateNewDB)
	mux.HandleFunc("POST /api/server/launch", a.handleServerLaunch)

	// SPA static files
	distFS, err := fs.Sub(adminDist, "dist")
	if err != nil {
		log.Printf("admin: embedded dist not found, serving API only: %v", err)
	} else {
		fileServer := http.FileServer(http.FS(distFS))
		mux.Handle("/", adminSPAHandler(fileServer, distFS))
	}

	// Strip the prefix, then apply auth middleware
	return http.StripPrefix(prefix, a.authMiddleware(mux))
}

// adminSPAHandler serves static files, falling back to index.html for SPA routing.
func adminSPAHandler(fileServer http.Handler, fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// Check if the file exists in the embedded FS
		if _, err := fs.Stat(fsys, path); os.IsNotExist(err) {
			// SPA fallback
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// readJSON decodes a JSON request body.
func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
