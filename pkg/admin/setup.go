package admin

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"gopkg.in/yaml.v3"
)

// handleSetupStatus returns the current setup wizard state.
func (a *Admin) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	hasDB := false
	hasConfig := false

	if a.controller != nil {
		hasDB = a.controller.GetDatabase() != nil
		hasConfig = a.controller.GetConfPath() != ""
	}

	// Check for bolt file in data dir (committed but not yet launched)
	boltReady := false
	if a.dataDir != "" {
		if _, err := os.Stat(filepath.Join(a.dataDir, "game.bolt")); err == nil {
			boltReady = true
		}
	}

	// Import flow state
	hasImport := a.session != nil && a.session.FlatfileDB != nil
	hasValidation := a.session != nil && a.session.Validator != nil
	importReady := a.session != nil && a.session.ReadyToCommit

	writeJSON(w, http.StatusOK, map[string]any{
		"setup_mode":     a.setupMode,
		"has_database":   hasDB,
		"has_config":     hasConfig,
		"has_import":     hasImport,
		"has_validation": hasValidation,
		"import_ready":   importReady,
		"bolt_ready":     boltReady,
		"steps": map[string]string{
			"config":   stepStatus(hasConfig),
			"import":   stepStatus(hasImport),
			"validate": stepStatus(hasValidation),
			"commit":   stepStatus(importReady && !a.setupMode),
			"launch":   stepStatus(!a.setupMode && a.controller != nil && a.controller.IsRunning()),
		},
	})
}

func stepStatus(done bool) string {
	if done {
		return "complete"
	}
	return "pending"
}

// handleCreateNewDB creates a minimal empty game database with Room Zero and God (#1),
// writes it to bolt, writes a default game.yaml, and marks the system ready to launch.
func (a *Admin) handleCreateNewDB(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	dataDir := a.dataDir
	if dataDir == "" && a.controller != nil {
		confPath := a.controller.GetConfPath()
		if confPath != "" {
			dataDir = filepath.Dir(confPath)
		}
	}
	if dataDir == "" {
		writeError(w, http.StatusInternalServerError, "no data directory configured")
		return
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create data directory: "+err.Error())
		return
	}

	// Create minimal database
	db := createMinimalDB()

	// Write to bolt store
	boltPath := filepath.Join(dataDir, "game.bolt")
	store, err := boltstore.Open(boltPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create bolt store: "+err.Error())
		return
	}
	if err := store.ImportFromDatabase(db); err != nil {
		store.Close()
		writeError(w, http.StatusInternalServerError, "failed to import database: "+err.Error())
		return
	}
	store.Close()
	log.Printf("admin: created new database at %s (%d objects)", boltPath, len(db.Objects))

	// Seed files from MUSH_SEEDDIR (baked into Docker image)
	seedDir := os.Getenv("MUSH_SEEDDIR")
	seeded := seedFromDir(seedDir, dataDir)

	// Write default game.yaml if not seeded
	confPath := filepath.Join(dataDir, "game.yaml")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		defaultConf := map[string]any{
			"mud_name":              "GoTinyMUSH",
			"port":                  6886,
			"player_starting_room":  0,
			"player_starting_home":  0,
			"master_room":           2,
			"default_home":          0,
			"money_name_singular":   "Penny",
			"money_name_plural":     "Pennies",
			"starting_money":        150,
			"paycheck":              50,
			"idle_timeout":          3600,
			"function_invocation_limit": 25000,
			"output_limit":          16384,
			"web_enabled":           true,
			"web_port":              8443,
			"comsys_enabled":        true,
			"mail_enabled":          true,
		}
		yamlData, _ := yaml.Marshal(defaultConf)
		if err := os.WriteFile(confPath, yamlData, 0644); err != nil {
			log.Printf("admin: warning: failed to write config: %v", err)
		} else {
			log.Printf("admin: wrote default config to %s", confPath)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "created",
		"bolt_path":    boltPath,
		"config_path":  confPath,
		"object_count": len(db.Objects),
		"seeded_files": seeded,
	})
}

// createMinimalDB builds a minimal game database with:
// - Room Zero (#0) — the default starting room
// - God/Wizard (#1) — the superuser player
// - Master Room (#2) — for global commands
func createMinimalDB() *gamedb.Database {
	db := gamedb.NewDatabase()
	now := time.Now()

	// #0 Room Zero
	db.Objects[0] = &gamedb.Object{
		DBRef:    0,
		Name:     "Room Zero",
		Location: gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Contents: 1, // God is here
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Pennies:  0,
		Flags:    [3]int{int(gamedb.TypeRoom), 0, 0},
		LastAccess: now,
		LastMod:    now,
		Attrs: []gamedb.Attribute{
			{Number: 28, Value: "Room Zero"},   // A_NAME (display)
			{Number: 27, Value: "You see nothing special."}, // A_DESC
		},
	}

	// #1 God (Wizard)
	db.Objects[1] = &gamedb.Object{
		DBRef:    1,
		Name:     "Wizard",
		Location: 0,
		Zone:     gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     0,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Pennies:  1000,
		Flags:    [3]int{int(gamedb.TypePlayer) | gamedb.FlagWizard | gamedb.FlagInherit, 0, 0},
		LastAccess: now,
		LastMod:    now,
		Attrs: []gamedb.Attribute{
			{Number: 28, Value: "Wizard"}, // A_NAME
			{Number: 27, Value: "You see a powerful wizard."}, // A_DESC
		},
	}

	// #2 Master Room
	db.Objects[2] = &gamedb.Object{
		DBRef:    2,
		Name:     "Master Room",
		Location: gamedb.Nothing,
		Zone:     gamedb.Nothing,
		Contents: gamedb.Nothing,
		Exits:    gamedb.Nothing,
		Link:     gamedb.Nothing,
		Next:     gamedb.Nothing,
		Owner:    1,
		Parent:   gamedb.Nothing,
		Pennies:  0,
		Flags:    [3]int{int(gamedb.TypeRoom), 0, 0},
		LastAccess: now,
		LastMod:    now,
		Attrs: []gamedb.Attribute{
			{Number: 28, Value: "Master Room"}, // A_NAME
		},
	}

	return db
}

// seedFromDir copies seed files (text, dict, config, aliases) from seedDir to dataDir
// if they don't already exist. Returns count of files seeded.
func seedFromDir(seedDir, dataDir string) int {
	if seedDir == "" {
		return 0
	}
	if _, err := os.Stat(seedDir); os.IsNotExist(err) {
		return 0
	}

	count := 0

	// Seed text directory
	seedText := filepath.Join(seedDir, "text")
	destText := filepath.Join(dataDir, "text")
	if entries, err := os.ReadDir(seedText); err == nil {
		os.MkdirAll(destText, 0755)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			dest := filepath.Join(destText, e.Name())
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				data, err := os.ReadFile(filepath.Join(seedText, e.Name()))
				if err == nil {
					os.WriteFile(dest, data, 0644)
					count++
				}
			}
		}
	}

	// Seed dict directory
	seedDict := filepath.Join(seedDir, "dict")
	destDict := filepath.Join(dataDir, "dict")
	if entries, err := os.ReadDir(seedDict); err == nil {
		os.MkdirAll(destDict, 0755)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			dest := filepath.Join(destDict, e.Name())
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				data, err := os.ReadFile(filepath.Join(seedDict, e.Name()))
				if err == nil {
					os.WriteFile(dest, data, 0644)
					count++
				}
			}
		}
	}

	// Seed config file
	seedConf := filepath.Join(seedDir, "game.yaml")
	destConf := filepath.Join(dataDir, "game.yaml")
	if _, err := os.Stat(destConf); os.IsNotExist(err) {
		if data, err := os.ReadFile(seedConf); err == nil {
			os.WriteFile(destConf, data, 0644)
			count++
		}
	}

	// Seed alias config
	seedAlias := filepath.Join(seedDir, "goTinyAlias.conf")
	destAlias := filepath.Join(dataDir, "goTinyAlias.conf")
	if _, err := os.Stat(destAlias); os.IsNotExist(err) {
		if data, err := os.ReadFile(seedAlias); err == nil {
			os.WriteFile(destAlias, data, 0644)
			count++
		}
	}

	if count > 0 {
		log.Printf("admin: seeded %d files from %s to %s", count, seedDir, dataDir)
	}
	return count
}

// handleServerLaunch checks required files exist, then restarts the server process
// so it boots with the new database. In Docker with restart: unless-stopped, this
// triggers a clean restart.
func (a *Admin) handleServerLaunch(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	dataDir := a.dataDir
	a.mu.RUnlock()

	if dataDir == "" {
		writeError(w, http.StatusInternalServerError, "no data directory configured")
		return
	}

	// Check required files
	var missing []string
	required := map[string]string{
		"game.bolt": "Database",
		"game.yaml": "Configuration",
	}
	for file, label := range required {
		if _, err := os.Stat(filepath.Join(dataDir, file)); os.IsNotExist(err) {
			missing = append(missing, label+" ("+file+")")
		}
	}
	if len(missing) > 0 {
		writeError(w, http.StatusBadRequest, "missing required files: "+strings.Join(missing, ", "))
		return
	}

	// Check optional files and warn
	var warnings []string
	optional := map[string]string{
		"text":              "Text files directory",
		"goTinyAlias.conf":  "Alias configuration",
	}
	for file, label := range optional {
		if _, err := os.Stat(filepath.Join(dataDir, file)); os.IsNotExist(err) {
			warnings = append(warnings, label+" ("+file+")")
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "launching",
		"message":  "Server is restarting...",
		"warnings": warnings,
	})

	// Flush the response, then exit after a short delay
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Printf("admin: launching server (exit for restart)...")
		os.Exit(0)
	}()
}
