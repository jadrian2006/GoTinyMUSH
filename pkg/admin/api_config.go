package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

func (a *Admin) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	confPath := ""
	if a.controller != nil {
		confPath = a.controller.GetConfPath()
	}
	if confPath == "" {
		writeError(w, http.StatusNotFound, "no config file path configured")
		return
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read config: "+err.Error())
		return
	}

	// Parse YAML to return as JSON
	var conf map[string]any
	if err := yaml.Unmarshal(data, &conf); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":   confPath,
		"config": conf,
	})
}

func (a *Admin) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	confPath := ""
	if a.controller != nil {
		confPath = a.controller.GetConfPath()
	}
	if confPath == "" {
		writeError(w, http.StatusNotFound, "no config file path configured")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Parse the incoming JSON config
	var incoming map[string]any
	if err := json.Unmarshal(body, &incoming); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Convert to YAML and write
	yamlData, err := yaml.Marshal(incoming)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to serialize config: "+err.Error())
		return
	}

	if err := os.WriteFile(confPath, yamlData, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "saved",
		"path":   confPath,
	})
}
