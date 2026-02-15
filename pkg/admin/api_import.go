package admin

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/archive"
	"github.com/crystal-mush/gotinymush/pkg/boltstore"
	"github.com/crystal-mush/gotinymush/pkg/flatfile"
	"github.com/crystal-mush/gotinymush/pkg/validate"
)

// isArchiveFile checks if a filename looks like an archive by extension.
func isArchiveFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".zip")
}

// isTarGz checks if a filename is a tar.gz/tgz archive.
func isTarGz(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

// extractZip extracts a zip archive to destDir.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		// Prevent directory traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid zip entry: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// extractArchiveToDir extracts a tar.gz or zip archive to destDir.
func extractArchiveToDir(srcPath, destDir string) error {
	if isTarGz(srcPath) {
		return archive.ExtractTarGz(srcPath, destDir)
	}
	return extractZip(srcPath, destDir)
}

// checkManifest looks for manifest.json in the staging dir.
func checkManifest(stagingDir string) *archive.Manifest {
	data, err := os.ReadFile(filepath.Join(stagingDir, "manifest.json"))
	if err != nil {
		return nil
	}
	var m archive.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}

// initSession creates a fresh import session.
func (a *Admin) initSession() *ImportSession {
	a.session = &ImportSession{
		Aliases:   make(map[string][]byte),
		TextFiles: make(map[string][]byte),
		DictFiles: make(map[string][]byte),
	}
	return a.session
}

// handleImportUpload accepts a flatfile or archive upload, parses/extracts it,
// and stores the resulting session for further import steps.
func (a *Admin) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var uploadPath string
	var uploadName string

	if r.Header.Get("Content-Type") == "application/json" {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		uploadPath = req.Path
		uploadName = filepath.Base(req.Path)
	} else {
		// Multipart file upload
		r.ParseMultipartForm(256 << 20) // 256MB max
		file, header, err := r.FormFile("flatfile")
		if err != nil {
			writeError(w, http.StatusBadRequest, "no file provided: "+err.Error())
			return
		}
		defer file.Close()

		tmpFile, err := os.CreateTemp("", "gotinymush-import-*"+filepath.Ext(header.Filename))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create temp file")
			return
		}
		defer tmpFile.Close()

		if _, err := io.Copy(tmpFile, file); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save uploaded file")
			return
		}
		uploadPath = tmpFile.Name()
		uploadName = header.Filename
	}

	session := a.initSession()

	if isArchiveFile(uploadName) {
		// Archive upload — extract to staging dir
		stagingDir, err := os.MkdirTemp("", "gotinymush-staging-*")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create staging dir")
			return
		}
		session.StagingDir = stagingDir

		if err := extractArchiveToDir(uploadPath, stagingDir); err != nil {
			writeError(w, http.StatusBadRequest, "failed to extract archive: "+err.Error())
			return
		}

		// Check for GoTinyMUSH manifest
		if m := checkManifest(stagingDir); m != nil {
			session.Source = "gotinymush_archive"
			session.Manifest = m
			session.UploadFile = uploadName

			// Count extracted files
			fileCount := 0
			filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					fileCount++
				}
				return nil
			})

			writeJSON(w, http.StatusOK, map[string]any{
				"status":      "extracted",
				"source":      "gotinymush_archive",
				"file":        uploadName,
				"file_count":  fileCount,
				"manifest":    m,
			})
			return
		}

		// Foreign archive
		session.Source = "foreign_archive"
		session.UploadFile = uploadName

		fileCount := 0
		filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})

		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "extracted",
			"source":     "foreign_archive",
			"file":       uploadName,
			"file_count": fileCount,
		})
		return
	}

	// Bare flatfile upload (existing flow)
	session.Source = "flatfile"
	session.StagingDir = filepath.Dir(uploadPath)

	db, err := flatfile.Load(uploadPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse flatfile: "+err.Error())
		return
	}

	session.FlatfileDB = db

	typeCounts := make(map[string]int)
	totalAttrs := 0
	for _, obj := range db.Objects {
		typeCounts[obj.ObjType().String()]++
		totalAttrs += len(obj.Attrs)
	}

	session.UploadFile = filepath.Base(uploadPath)
	session.ObjectCount = len(db.Objects)
	session.AttrDefs = len(db.AttrNames)
	session.TotalAttrs = totalAttrs
	session.TypeCounts = typeCounts

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "parsed",
		"source":       "flatfile",
		"file":         filepath.Base(uploadPath),
		"object_count": len(db.Objects),
		"attr_defs":    len(db.AttrNames),
		"total_attrs":  totalAttrs,
		"type_counts":  typeCounts,
	})
}

// handleImportValidate runs all validators on the imported database.
func (a *Admin) handleImportValidate(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil || a.session.FlatfileDB == nil {
		writeError(w, http.StatusBadRequest, "no flatfile has been uploaded yet")
		return
	}

	a.session.Validator = validate.New(a.session.FlatfileDB)
	findings := a.session.Validator.Run()
	a.session.ValidationDone = true
	a.session.ReadyToCommit = true

	report := validate.GenerateReport(a.session.Validator)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "validated",
		"total":  len(findings),
		"report": report,
	})
}

// handleImportFindings returns the current findings list.
func (a *Admin) handleImportFindings(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.session == nil || a.session.Validator == nil {
		writeError(w, http.StatusBadRequest, "no validation has been run yet")
		return
	}

	report := validate.GenerateReport(a.session.Validator)
	writeJSON(w, http.StatusOK, report)
}

// handleImportFix applies fixes by finding ID or category.
func (a *Admin) handleImportFix(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil || a.session.Validator == nil {
		writeError(w, http.StatusBadRequest, "no validation has been run yet")
		return
	}

	var req struct {
		FindingID string `json:"finding_id,omitempty"`
		Category  string `json:"category,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FindingID != "" {
		if err := a.session.Validator.ApplyFix(req.FindingID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "fixed",
			"id":     req.FindingID,
		})
		return
	}

	if req.Category != "" {
		cat, err := parseCategoryString(req.Category)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		count := a.session.Validator.ApplyAll(cat)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "fixed",
			"category": req.Category,
			"count":    count,
		})
		return
	}

	writeError(w, http.StatusBadRequest, "provide either finding_id or category")
}

// handleImportCommit writes the validated database and all staged files
// to the game's data directory, making it ready for launch.
func (a *Admin) handleImportCommit(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.setCommitProgress("starting", "Preparing commit...", 0)

	if a.session == nil || a.session.FlatfileDB == nil {
		a.setCommitError("no flatfile has been uploaded")
		writeError(w, http.StatusBadRequest, "no flatfile has been uploaded")
		return
	}
	if !a.session.ReadyToCommit {
		a.setCommitError("run validation before committing")
		writeError(w, http.StatusBadRequest, "run validation before committing")
		return
	}

	// Determine data directory
	dataDir := a.dataDir
	if dataDir == "" && a.controller != nil {
		confPath := a.controller.GetConfPath()
		if confPath != "" {
			dataDir = filepath.Dir(confPath)
		}
	}
	if dataDir == "" {
		a.setCommitError("no data directory configured")
		writeError(w, http.StatusInternalServerError, "no data directory configured")
		return
	}

	os.MkdirAll(dataDir, 0755)

	committed := map[string]any{
		"object_count": len(a.session.FlatfileDB.Objects),
	}

	// Write the database to bolt store
	a.setCommitProgress("writing_db", fmt.Sprintf("Writing %d objects to database...", len(a.session.FlatfileDB.Objects)), 10)
	log.Printf("admin: commit: writing %d objects to bolt store...", len(a.session.FlatfileDB.Objects))

	boltPath := filepath.Join(dataDir, "game.bolt")
	store, err := boltstore.Open(boltPath)
	if err != nil {
		a.setCommitError("failed to create bolt store: " + err.Error())
		writeError(w, http.StatusInternalServerError, "failed to create bolt store: "+err.Error())
		return
	}
	if err := store.ImportFromDatabase(a.session.FlatfileDB); err != nil {
		store.Close()
		a.setCommitError("failed to import database: " + err.Error())
		writeError(w, http.StatusInternalServerError, "failed to import database: "+err.Error())
		return
	}
	log.Printf("admin: commit: wrote %d objects to %s", len(a.session.FlatfileDB.Objects), boltPath)
	committed["bolt_path"] = boltPath

	// Write comsys to boltstore if parsed
	if a.session.ComsysParsed && len(a.session.Channels) > 0 {
		a.setCommitProgress("writing_comsys", fmt.Sprintf("Writing %d channels...", len(a.session.Channels)), 50)
		log.Printf("admin: commit: writing %d comsys channels...", len(a.session.Channels))
		if err := store.ImportComsys(a.session.Channels, a.session.ChanAliases); err != nil {
			log.Printf("admin: warning: import comsys: %v", err)
		} else {
			committed["comsys_imported"] = true
			committed["channel_count"] = len(a.session.Channels)
		}
	}
	store.Close()

	// Write staged config if present
	a.setCommitProgress("writing_config", "Writing configuration...", 70)
	confPath := filepath.Join(dataDir, "game.yaml")
	if len(a.session.Config) > 0 {
		log.Printf("admin: commit: writing config to %s", confPath)
		if err := os.WriteFile(confPath, a.session.Config, 0644); err != nil {
			log.Printf("admin: warning: write config: %v", err)
		} else {
			committed["config_written"] = true
		}
	}

	// Write staged text files
	a.setCommitProgress("writing_files", "Writing text and data files...", 80)
	if len(a.session.TextFiles) > 0 {
		textDir := filepath.Join(dataDir, "text")
		os.MkdirAll(textDir, 0755)
		log.Printf("admin: commit: writing %d text files...", len(a.session.TextFiles))
		for name, content := range a.session.TextFiles {
			if err := os.WriteFile(filepath.Join(textDir, name), content, 0644); err != nil {
				log.Printf("admin: warning: write text file %s: %v", name, err)
			}
		}
		committed["text_files_written"] = len(a.session.TextFiles)
	}

	// Write staged dict files
	if len(a.session.DictFiles) > 0 {
		dictDir := filepath.Join(dataDir, "dict")
		os.MkdirAll(dictDir, 0755)
		log.Printf("admin: commit: writing %d dict files...", len(a.session.DictFiles))
		for name, content := range a.session.DictFiles {
			if err := os.WriteFile(filepath.Join(dictDir, name), content, 0644); err != nil {
				log.Printf("admin: warning: write dict file %s: %v", name, err)
			}
		}
		committed["dict_files_written"] = len(a.session.DictFiles)
	}

	// Write staged alias files
	if len(a.session.Aliases) > 0 {
		log.Printf("admin: commit: writing %d alias files...", len(a.session.Aliases))
		for name, content := range a.session.Aliases {
			aliasPath := filepath.Join(dataDir, name)
			if err := os.WriteFile(aliasPath, content, 0644); err != nil {
				log.Printf("admin: warning: write alias file %s: %v", name, err)
			}
		}
		committed["alias_files_written"] = len(a.session.Aliases)
	}

	a.setCommitProgress("done", "Commit complete", 100)
	log.Printf("admin: commit: complete")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "committed",
		"committed": committed,
	})

	// Flush response to ensure the client receives it immediately
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// handleImportSession returns the full session state.
func (a *Admin) handleImportSession(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.session == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"active": false,
		})
		return
	}

	resp := map[string]any{
		"active":          true,
		"source":          a.session.Source,
		"staging_dir":     a.session.StagingDir,
		"files":           a.session.Files,
		"config_ready":    a.session.ConfigReady,
		"comsys_parsed":   a.session.ComsysParsed,
		"validation_done": a.session.ValidationDone,
		"ready_to_commit": a.session.ReadyToCommit,
		"upload_file":     a.session.UploadFile,
	}

	if a.session.FlatfileDB != nil {
		resp["object_count"] = a.session.ObjectCount
		resp["attr_defs"] = a.session.AttrDefs
		resp["total_attrs"] = a.session.TotalAttrs
		resp["type_counts"] = a.session.TypeCounts
	}

	if a.session.Manifest != nil {
		resp["manifest"] = a.session.Manifest
	}

	if a.session.ComsysParsed {
		resp["channel_count"] = len(a.session.Channels)
		resp["alias_count"] = len(a.session.ChanAliases)
	}

	// List staged file names
	if len(a.session.TextFiles) > 0 {
		names := make([]string, 0, len(a.session.TextFiles))
		for k := range a.session.TextFiles {
			names = append(names, k)
		}
		resp["text_files"] = names
	}
	if len(a.session.DictFiles) > 0 {
		names := make([]string, 0, len(a.session.DictFiles))
		for k := range a.session.DictFiles {
			names = append(names, k)
		}
		resp["dict_files"] = names
	}
	if len(a.session.Aliases) > 0 {
		names := make([]string, 0, len(a.session.Aliases))
		for k := range a.session.Aliases {
			names = append(names, k)
		}
		resp["alias_files"] = names
	}
	if len(a.session.Config) > 0 {
		resp["has_config"] = true
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleImportReset clears the import session and cleans up staging files.
func (a *Admin) handleImportReset(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session != nil && a.session.StagingDir != "" {
		// Only remove staging dirs we created in temp
		if strings.Contains(a.session.StagingDir, "gotinymush-staging") {
			os.RemoveAll(a.session.StagingDir)
		}
	}
	a.session = nil

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// handleImportDiscover runs the file discovery engine on the staging dir.
func (a *Admin) handleImportDiscover(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil || a.session.StagingDir == "" {
		writeError(w, http.StatusBadRequest, "no import session with staging directory")
		return
	}

	files, err := DiscoverFiles(a.session.StagingDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "discovery failed: "+err.Error())
		return
	}

	a.session.Files = files

	// Auto-load flatfile if exactly one high-confidence match
	for _, f := range files {
		if f.Role == RoleFlatfile && f.Confidence == "high" && a.session.FlatfileDB == nil {
			fullPath := filepath.Join(a.session.StagingDir, f.Path)
			db, err := flatfile.Load(fullPath)
			if err != nil {
				log.Printf("admin: auto-load flatfile %s failed: %v", f.Path, err)
				continue
			}
			a.session.FlatfileDB = db

			typeCounts := make(map[string]int)
			totalAttrs := 0
			for _, obj := range db.Objects {
				typeCounts[obj.ObjType().String()]++
				totalAttrs += len(obj.Attrs)
			}
			a.session.ObjectCount = len(db.Objects)
			a.session.AttrDefs = len(db.AttrNames)
			a.session.TotalAttrs = totalAttrs
			a.session.TypeCounts = typeCounts
			break
		}
	}

	// Auto-stage text/dict files
	for _, f := range files {
		fullPath := filepath.Join(a.session.StagingDir, f.Path)
		switch f.Role {
		case RoleTextFile:
			data, err := os.ReadFile(fullPath)
			if err == nil {
				a.session.TextFiles[filepath.Base(f.Path)] = data
			}
		case RoleDictFile:
			data, err := os.ReadFile(fullPath)
			if err == nil {
				a.session.DictFiles[filepath.Base(f.Path)] = data
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "discovered",
		"files":  files,
		"flatfile_loaded": a.session.FlatfileDB != nil,
	})
}

// handleImportAssign lets the user reassign a discovered file's role.
func (a *Admin) handleImportAssign(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil {
		writeError(w, http.StatusBadRequest, "no import session")
		return
	}

	var req struct {
		Path string   `json:"path"`
		Role FileRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	found := false
	for i, f := range a.session.Files {
		if f.Path == req.Path {
			a.session.Files[i].Role = req.Role
			a.session.Files[i].Confidence = "manual"
			a.session.Files[i].Reason = "manually assigned"
			found = true
			break
		}
	}

	if !found {
		writeError(w, http.StatusNotFound, "file not found in session")
		return
	}

	// If assigning as flatfile, try to load it
	if req.Role == RoleFlatfile {
		fullPath := filepath.Join(a.session.StagingDir, req.Path)
		db, err := flatfile.Load(fullPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cannot parse as flatfile: "+err.Error())
			return
		}
		a.session.FlatfileDB = db

		typeCounts := make(map[string]int)
		totalAttrs := 0
		for _, obj := range db.Objects {
			typeCounts[obj.ObjType().String()]++
			totalAttrs += len(obj.Attrs)
		}
		a.session.ObjectCount = len(db.Objects)
		a.session.AttrDefs = len(db.AttrNames)
		a.session.TotalAttrs = totalAttrs
		a.session.TypeCounts = typeCounts
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "assigned",
		"path":   req.Path,
		"role":   req.Role,
	})
}

// handleImportConvertConfig converts a legacy .conf to YAML.
func (a *Admin) handleImportConvertConfig(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil {
		writeError(w, http.StatusBadRequest, "no import session")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fullPath := filepath.Join(a.session.StagingDir, req.Path)

	var yamlBytes []byte
	var err error
	if a.controller != nil {
		yamlBytes, err = a.controller.ConvertLegacyConfig(fullPath)
	} else if a.ConvertLegacyConfigFunc != nil {
		yamlBytes, err = a.ConvertLegacyConfigFunc(fullPath)
	} else {
		writeError(w, http.StatusInternalServerError, "no config converter available")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "conversion failed: "+err.Error())
		return
	}

	a.session.Config = yamlBytes
	a.session.ConfigReady = true

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "converted",
		"content": string(yamlBytes),
	})
}

// handleImportParseComsys parses a mod_comsys.db file from the staging dir.
func (a *Admin) handleImportParseComsys(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil {
		writeError(w, http.StatusBadRequest, "no import session")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fullPath := filepath.Join(a.session.StagingDir, req.Path)
	f, err := os.Open(fullPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot open comsys file: "+err.Error())
		return
	}
	defer f.Close()

	channels, aliases, err := flatfile.ParseComsys(f)
	if err != nil {
		writeError(w, http.StatusBadRequest, "comsys parse failed: "+err.Error())
		return
	}

	a.session.Channels = channels
	a.session.ChanAliases = aliases
	a.session.ComsysParsed = true

	// Build channel summaries for response
	channelSummaries := make([]map[string]any, len(channels))
	for i, ch := range channels {
		channelSummaries[i] = map[string]any{
			"name":        ch.Name,
			"owner":       ch.Owner,
			"description": ch.Description,
			"num_sent":    ch.NumSent,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "parsed",
		"channel_count": len(channels),
		"alias_count":   len(aliases),
		"channels":      channelSummaries,
	})
}

// handleImportFileRead returns the content of a staged file.
func (a *Admin) handleImportFileRead(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.session == nil {
		writeError(w, http.StatusBadRequest, "no import session")
		return
	}

	role := FileRole(r.PathValue("role"))
	name := r.PathValue("name")

	var content []byte
	var found bool

	switch role {
	case RoleMainConf:
		if a.session.Config != nil {
			content = a.session.Config
			found = true
		}
	case RoleAliasConf:
		content, found = a.session.Aliases[name]
	case RoleTextFile:
		content, found = a.session.TextFiles[name]
	case RoleDictFile:
		content, found = a.session.DictFiles[name]
	default:
		writeError(w, http.StatusBadRequest, "unsupported role: "+string(role))
		return
	}

	if !found {
		writeError(w, http.StatusNotFound, "file not found in session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"name":    name,
		"role":    role,
		"content": string(content),
	})
}

// handleImportFileWrite updates the content of a staged file.
func (a *Admin) handleImportFileWrite(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil {
		writeError(w, http.StatusBadRequest, "no import session")
		return
	}

	role := FileRole(r.PathValue("role"))
	name := r.PathValue("name")

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	content := []byte(req.Content)

	switch role {
	case RoleMainConf:
		a.session.Config = content
		a.session.ConfigReady = true
	case RoleAliasConf:
		a.session.Aliases[name] = content
	case RoleTextFile:
		a.session.TextFiles[name] = content
	case RoleDictFile:
		a.session.DictFiles[name] = content
	default:
		writeError(w, http.StatusBadRequest, "unsupported role: "+string(role))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "updated",
		"name":   name,
		"role":   role,
		"size":   len(content),
	})
}

// parseCategoryString converts a category string to a Category constant.
func parseCategoryString(s string) (validate.Category, error) {
	switch s {
	case "double-escape":
		return validate.CatDoubleEscape, nil
	case "attr-flags":
		return validate.CatAttrFlags, nil
	case "escape-seq":
		return validate.CatEscapeSeq, nil
	case "percent":
		return validate.CatPercent, nil
	case "integrity-error":
		return validate.CatIntegrityError, nil
	case "integrity-warning":
		return validate.CatIntegrityWarn, nil
	default:
		return 0, fmt.Errorf("unknown category: %s", s)
	}
}
