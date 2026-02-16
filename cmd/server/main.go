package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/admin"
	"github.com/crystal-mush/gotinymush/pkg/archive"
	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	mushcrypt "github.com/crystal-mush/gotinymush/pkg/crypt"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/flatfile"
	"github.com/crystal-mush/gotinymush/pkg/server"
	"gopkg.in/yaml.v3"
)

// envDefault returns the environment variable value if set, otherwise the fallback.
func envDefault(envVar, fallback string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return fallback
}

func main() {
	dbPath := flag.String("db", envDefault("MUSH_DB", ""), "Path to TinyMUSH flatfile database (env: MUSH_DB)")
	boltPath := flag.String("bolt", envDefault("MUSH_BOLT", ""), "Path to bbolt persistent database (env: MUSH_BOLT)")
	forceImport := flag.Bool("import", os.Getenv("MUSH_IMPORT") == "true", "Force re-import from flatfile into bbolt (env: MUSH_IMPORT)")
	port := flag.Int("port", 0, "TCP port to listen on, overrides config (env: MUSH_PORT)")
	textDir := flag.String("textdir", envDefault("MUSH_TEXTDIR", ""), "Path to text files directory (env: MUSH_TEXTDIR)")
	aliasConf := flag.String("aliasconf", envDefault("MUSH_ALIASCONF", ""), "Path to alias config file(s), comma-separated (env: MUSH_ALIASCONF)")
	confFile := flag.String("conf", envDefault("MUSH_CONF", ""), "Path to game config file (env: MUSH_CONF)")
	comsysDB := flag.String("comsysdb", envDefault("MUSH_COMSYSDB", ""), "Path to mod_comsys.db file for channel import (env: MUSH_COMSYSDB)")
	dictDir := flag.String("dictdir", envDefault("MUSH_DICTDIR", ""), "Path to dictionary directory (env: MUSH_DICTDIR)")
	sqlDBPath := flag.String("sqldb", envDefault("MUSH_SQLDB", ""), "Path to SQLite3 database file (env: MUSH_SQLDB)")
	fresh := flag.Bool("fresh", os.Getenv("MUSH_FRESH") == "true", "Delete bolt DB on startup for a clean reimport every restart (env: MUSH_FRESH)")
	tlsCert := flag.String("tls-cert", envDefault("MUSH_TLS_CERT", ""), "Path to TLS certificate file (env: MUSH_TLS_CERT)")
	tlsKey := flag.String("tls-key", envDefault("MUSH_TLS_KEY", ""), "Path to TLS private key file (env: MUSH_TLS_KEY)")
	tlsPort := flag.String("tls-port", envDefault("MUSH_TLS_PORT", ""), "TLS listen port (env: MUSH_TLS_PORT)")
	restoreArchive := flag.String("restore", envDefault("MUSH_RESTORE", ""), "Restore from archive before boot (env: MUSH_RESTORE)")
	godPass := flag.String("godpass", envDefault("MUSH_GODPASS", ""), "Set God (#1) password and exit (env: MUSH_GODPASS)")
	flag.Parse()

	log.Printf("Welcome to %s", server.VersionString())

	// Handle MUSH_PORT env if -port flag not set
	if *port == 0 {
		if envPort := os.Getenv("MUSH_PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil {
				*port = p
			}
		}
	}

	// Load game config early (needed for setup mode and normal mode)
	var gc *server.GameConf
	if *confFile != "" {
		var err error
		gc, err = server.LoadGameConf(*confFile)
		if err != nil {
			log.Printf("Config file not available (%v) — using defaults", err)
			gc = server.DefaultGameConf()
		} else {
			log.Printf("Loaded game config from %s", *confFile)
		}
	} else {
		gc = server.DefaultGameConf()
	}

	// Command-line port override
	if *port != 0 {
		gc.Port = *port
	}

	// Auto-detect existing game.bolt in data directory if not explicitly set
	if *boltPath == "" {
		dataDir := "/game/data"
		if *confFile != "" {
			dataDir = filepath.Dir(*confFile)
		}
		candidate := filepath.Join(dataDir, "game.bolt")
		if _, err := os.Stat(candidate); err == nil {
			*boltPath = candidate
			log.Printf("Auto-detected bolt store: %s", candidate)
		}
	}

	setupMode := *dbPath == "" && *boltPath == ""
	if setupMode {
		log.Printf("No database specified — starting in setup mode (admin panel only)")
		dataDir := "/game/data"
		if *confFile != "" {
			dataDir = filepath.Dir(*confFile)
		}
		startSetupMode(*confFile, *port, gc, dataDir)
		return
	}

	// Pre-boot restore from archive
	if *restoreArchive != "" {
		log.Printf("Restoring from archive: %s", *restoreArchive)
		result, err := archive.RestoreArchive(archive.RestoreParams{
			ArchivePath: *restoreArchive,
			BoltDest:    *boltPath,
			SQLDest:     *sqlDBPath,
			DictDest:    *dictDir,
			TextDest:    *textDir,
			ConfDest:    *confFile,
			AliasDest: func() string {
				if *confFile != "" {
					return filepath.Dir(*confFile)
				}
				return ""
			}(),
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
		})
		if err != nil {
			log.Fatalf("Restore failed: %v", err)
		}
		log.Printf("Restore complete: %d files restored", result.FilesRestored)
		for _, w := range result.Warnings {
			log.Printf("Restore warning: %s", w)
		}
	}

	// (gc already loaded above, before setup mode check)

	// Command-line flags override config file values (non-port, port already handled)


	// TLS cert/key: flags override config
	if *tlsCert != "" {
		gc.TLSCert = *tlsCert
	}
	if *tlsKey != "" {
		gc.TLSKey = *tlsKey
	}
	if *tlsPort != "" {
		if p, err := strconv.Atoi(*tlsPort); err == nil {
			gc.TLSPort = p
		}
	}

	// Env overrides for bool toggles
	if v := os.Getenv("MUSH_TLS"); v != "" {
		gc.TLS = strings.EqualFold(v, "true")
	}
	if v := os.Getenv("MUSH_CLEARTEXT"); v != "" {
		b := strings.EqualFold(v, "true")
		gc.Cleartext = &b
	}

	// Archive env overrides
	if v := os.Getenv("MUSH_ARCHIVE_DIR"); v != "" {
		gc.ArchiveDir = v
	}
	if v := os.Getenv("MUSH_ARCHIVE_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			gc.ArchiveInterval = n
		}
	}
	if v := os.Getenv("MUSH_ARCHIVE_RETAIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			gc.ArchiveRetain = n
		}
	}

	// Default TLS port to main port + 1
	if gc.TLSPort == 0 {
		gc.TLSPort = gc.Port + 1
	}

	// Validate: TLS enabled requires cert and key
	if gc.TLS {
		if gc.TLSCert == "" || gc.TLSKey == "" {
			log.Fatalf("TLS is enabled but tls_cert and/or tls_key are not set. "+
				"Provide certificate and key via -tls-cert/-tls-key flags, "+
				"MUSH_TLS_CERT/MUSH_TLS_KEY env vars, or tls_cert/tls_key in config file.")
		}
	}

	cfg := server.Config{
		Port:        gc.Port,
		IdleTimeout: time.Duration(gc.IdleTimeout) * time.Second,
		MaxRetries:  3,
		WelcomeText: server.WelcomeText,
		Cleartext:   gc.IsCleartext(),
		TLS:         gc.TLS,
		TLSPort:     gc.TLSPort,
		TLSCert:     gc.TLSCert,
		TLSKey:      gc.TLSKey,
	}

	var store *boltstore.Store
	var srv *server.Server

	if *boltPath != "" {
		// Fresh mode: delete old bolt DB so we reimport from flatfile every time
		if *fresh {
			if err := os.Remove(*boltPath); err != nil && !os.IsNotExist(err) {
				log.Fatalf("Error removing bolt database for fresh start: %v", err)
			}
			log.Printf("Fresh mode: removed %s for clean reimport", *boltPath)
		}

		// bbolt mode
		_, boltExists := os.Stat(*boltPath)
		needImport := *forceImport || os.IsNotExist(boltExists)

		var err error
		store, err = boltstore.Open(*boltPath)
		if err != nil {
			log.Fatalf("Error opening bolt database: %v", err)
		}
		defer store.Close()

		if !needImport && store.HasData() {
			// Normal run: load from bbolt
			log.Printf("Loading database from bbolt: %s", *boltPath)
			if err := store.LoadAll(); err != nil {
				log.Fatalf("Error loading from bolt: %v", err)
			}
			log.Printf("Database loaded from bolt: %d objects, %d attribute definitions",
				len(store.DB().Objects), len(store.DB().AttrNames))
		} else {
			// First run or forced import: parse flatfile then import into bbolt
			if *dbPath == "" {
				log.Fatalf("Flatfile path (-db or MUSH_DB) required for initial import into bbolt")
			}
			log.Printf("Importing flatfile %s into bbolt %s...", *dbPath, *boltPath)
			f, err := os.Open(*dbPath)
			if err != nil {
				log.Fatalf("Error opening flatfile: %v", err)
			}
			db, err := flatfile.Parse(f)
			f.Close()
			if err != nil {
				log.Fatalf("Error parsing flatfile: %v", err)
			}
			if err := store.ImportFromDatabase(db); err != nil {
				log.Fatalf("Error importing into bolt: %v", err)
			}
			log.Printf("Import complete: %d objects, %d attribute definitions",
				len(store.DB().Objects), len(store.DB().AttrNames))
		}

		srv = server.NewServer(store.DB(), cfg)
		srv.Game.Store = store
	} else {
		// Flatfile-only mode (no persistence beyond @dump)
		log.Printf("Loading database from %s...", *dbPath)
		f, err := os.Open(*dbPath)
		if err != nil {
			log.Fatalf("Error opening database: %v", err)
		}
		db, err := flatfile.Parse(f)
		f.Close()
		if err != nil {
			log.Fatalf("Error parsing database: %v", err)
		}
		log.Printf("Database loaded: %d objects, %d attribute definitions",
			len(db.Objects), len(db.AttrNames))

		srv = server.NewServer(db, cfg)
	}

	if *dbPath != "" {
		srv.Game.DBPath = *dbPath
	}

	// Apply game config
	srv.Game.ApplyGameConf(gc)

	// Handle -godpass: set God password on startup (continues booting)
	if *godPass != "" {
		godRef := srv.Game.GodPlayer()
		if _, ok := srv.Game.DB.Objects[godRef]; !ok {
			log.Fatalf("God player #%d not found in database", godRef)
		}
		hash := mushcrypt.Crypt(*godPass, "XX")
		srv.Game.SetAttr(godRef, 5, hash) // A_PASS = 5
		log.Printf("God (#%d) password set at startup.", godRef)
	}

	// Load text files if directory specified
	if *textDir != "" {
		srv.Game.TextDir = *textDir
		srv.Game.Texts = server.LoadTextFiles(*textDir)
		srv.Game.WatchTextFiles()
		srv.Game.LoadHelpFiles(*textDir)
	}

	// Load alias configs: explicit -aliasconf flag takes priority,
	// then any "include alias.conf" / "include compat.conf" from the game config.
	var aliasPaths []string
	if *aliasConf != "" {
		for _, p := range strings.Split(*aliasConf, ",") {
			aliasPaths = append(aliasPaths, strings.TrimSpace(p))
		}
	} else if len(gc.IncludedAliasConfs) > 0 {
		aliasPaths = gc.IncludedAliasConfs
	}
	if len(aliasPaths) > 0 {
		ac, err := server.LoadAliasConfig(aliasPaths...)
		if err != nil {
			log.Printf("WARNING: alias config not loaded: %v", err)
		} else {
			srv.Game.ApplyAliasConfig(ac)
		}
	}

	// Initialize spellcheck if enabled
	spellEnabled := gc.SpellcheckEnabled || os.Getenv("MUSH_SPELLCHECK") == "true"
	if spellEnabled {
		dir := *dictDir
		if dir == "" {
			dir = "data/dict"
		}
		spellURL := gc.SpellcheckURL
		if envURL := os.Getenv("MUSH_DICTURL"); envURL != "" {
			spellURL = envURL
		}
		srv.Game.Spell = server.NewSpellChecker(dir, spellURL, true)
		log.Printf("Spellcheck enabled, dict dir: %s", dir)
	}

	// Initialize SQL if enabled
	sqlEnabled := gc.SQLEnabled || os.Getenv("MUSH_SQL") == "true"
	sqlPath := gc.SQLDatabase
	if *sqlDBPath != "" {
		sqlPath = *sqlDBPath
	}
	if sqlEnabled && sqlPath != "" {
		sqlStore, err := server.OpenSQLStore(sqlPath, gc.SQLQueryLimit, gc.SQLTimeout)
		if err != nil {
			log.Printf("WARNING: failed to open SQL database %s: %v", sqlPath, err)
		} else {
			srv.Game.SQLDB = sqlStore
			log.Printf("SQL enabled, database: %s (limit=%d, timeout=%ds)", sqlPath, gc.SQLQueryLimit, gc.SQLTimeout)
		}
	}

	// Load comsys (channel system) if enabled
	if gc.ComsysEnabled {
		loadComsys(srv.Game, store, *comsysDB)
	} else {
		log.Printf("Comsys disabled by config")
	}

	// Load mail system if enabled
	if gc.MailEnabled {
		loadMail(srv.Game, store, gc.MailExpiration)
	} else {
		log.Printf("Mail system disabled by config")
	}

	// Load structures from bbolt
	loadStructures(store)

	// Store paths on Game for archive system
	srv.Game.ConfPath = *confFile
	srv.Game.AliasConfs = aliasPaths
	if *dictDir != "" {
		srv.Game.DictDir = *dictDir
	}
	srv.Game.ArchiveDir = gc.ArchiveDir

	// Start pprof debug endpoint on port 6060
	go func() {
		log.Printf("pprof debug endpoint at http://0.0.0.0:6060/debug/pprof/")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	// Repair any corrupted content chains before startup
	srv.Game.RepairContentChains()

	// Run @startup actions
	srv.Game.RunStartup()

	// Start auto-archive if configured
	if gc.ArchiveInterval > 0 {
		srv.Game.StartAutoArchive(gc.ArchiveInterval)
		log.Printf("Auto-archive enabled: every %d minutes, retain %d, dir %s",
			gc.ArchiveInterval, gc.ArchiveRetain, gc.ArchiveDir)
	}

	if cfg.Cleartext && cfg.TLS {
		log.Printf("Starting %s on port %d (cleartext) and %d (TLS)...", gc.MudName, cfg.Port, cfg.TLSPort)
	} else if cfg.TLS {
		log.Printf("Starting %s on port %d (TLS only)...", gc.MudName, cfg.TLSPort)
	} else {
		log.Printf("Starting %s on port %d...", gc.MudName, cfg.Port)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// loadComsys initializes the channel system from bbolt or mod_comsys.db.
func loadComsys(game *server.Game, store *boltstore.Store, comsysPath string) {
	cs := server.NewComsys()

	// Try loading from bbolt first
	if store != nil && store.HasComsysData() {
		channels, err := store.LoadChannels()
		if err != nil {
			log.Printf("WARNING: failed to load channels from bolt: %v", err)
		}
		aliases, err := store.LoadChanAliases()
		if err != nil {
			log.Printf("WARNING: failed to load chan aliases from bolt: %v", err)
		}
		if len(channels) > 0 {
			cs.LoadChannels(channels, aliases)
			game.Comsys = cs
			return
		}
	}

	// Try importing from mod_comsys.db
	if comsysPath == "" {
		return
	}

	f, err := os.Open(comsysPath)
	if err != nil {
		log.Printf("WARNING: cannot open comsys db %s: %v", comsysPath, err)
		return
	}
	defer f.Close()

	channels, aliases, err := flatfile.ParseComsys(f)
	if err != nil {
		log.Printf("WARNING: failed to parse comsys db %s: %v", comsysPath, err)
		return
	}
	log.Printf("Parsed comsys: %d channels, %d aliases from %s", len(channels), len(aliases), comsysPath)

	// Store in bbolt for future loads
	if store != nil {
		if err := store.ImportComsys(channels, aliases); err != nil {
			log.Printf("WARNING: failed to import comsys into bolt: %v", err)
		}
	}

	cs.LoadChannels(channels, aliases)
	game.Comsys = cs
}

// loadStructures populates the in-memory structure store from bbolt.
func loadStructures(store *boltstore.Store) {
	if store == nil || !store.HasStructData() {
		return
	}
	defs, err := store.LoadStructDefs()
	if err != nil {
		log.Printf("WARNING: failed to load struct defs from bolt: %v", err)
		return
	}
	insts, err := store.LoadStructInstances()
	if err != nil {
		log.Printf("WARNING: failed to load struct instances from bolt: %v", err)
		return
	}
	defCount := 0
	instCount := 0
	for _, m := range defs {
		defCount += len(m)
	}
	for _, m := range insts {
		instCount += len(m)
	}
	functions.LoadStructStore(defs, insts)
	log.Printf("Loaded %d structure defs, %d instances from bolt", defCount, instCount)
}

// loadMail initializes the mail system from bbolt.
func loadMail(game *server.Game, store *boltstore.Store, expireDays int) {
	m := server.NewMail(expireDays)

	if store != nil && store.HasMailData() {
		msgs, err := store.LoadMail()
		if err != nil {
			log.Printf("WARNING: failed to load mail from bolt: %v", err)
		} else {
			m.LoadMessages(msgs)
			total := 0
			for _, inbox := range msgs {
				total += len(inbox)
			}
			log.Printf("Loaded %d mail messages for %d players from bolt", total, len(msgs))
		}
	}

	game.Mail = m
	log.Printf("Mail system enabled (expiration: %d days)", expireDays)
}

// startSetupMode runs the server in setup-only mode: just the admin panel web server,
// no game engine, no telnet listeners. Used when no database is configured yet.
func startSetupMode(confFile string, port int, gc *server.GameConf, dataDir string) {
	webPort := gc.WebPort
	if webPort == 0 {
		webPort = 8443
	}

	// Create admin panel with no controller (setup mode)
	adminPanel := admin.New(nil)
	adminPanel.SetDataDir(dataDir)
	adminPanel.SetConfPath(confFile)
	adminPanel.ConvertLegacyConfigFunc = func(confPath string) ([]byte, error) {
		gc, err := server.LoadGameConf(confPath)
		if err != nil {
			return nil, err
		}
		return yaml.Marshal(gc)
	}
	setupStart := time.Now()
	mux := http.NewServeMux()

	// Health endpoint (no auth, available in setup mode)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"version":        server.Version,
			"uptime_seconds": time.Since(setupStart).Seconds(),
			"game_running":   false,
		})
	})

	mux.Handle("/admin/", adminPanel.Handler("/admin"))

	// Redirect / to /admin
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			http.Redirect(w, r, "/admin/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	addr := fmt.Sprintf(":%d", webPort)
	log.Printf("Setup mode: admin panel at http://0.0.0.0%s/admin/", addr)
	log.Printf("Upload a flatfile or archive to get started.")

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Setup mode web server error: %v", err)
	}
}
