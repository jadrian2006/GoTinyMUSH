package admin

import (
	"net/http"
)

func (a *Admin) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := map[string]any{
		"setup_mode": a.setupMode,
	}

	if a.controller != nil {
		status["running"] = a.controller.IsRunning()
		status["player_count"] = a.controller.PlayerCount()
		status["uptime"] = a.controller.Uptime()
		status["game_name"] = a.controller.GameName()
		status["version"] = a.controller.GameVersion()
		status["port"] = a.controller.GamePort()

		db := a.controller.GetDatabase()
		if db != nil {
			status["object_count"] = len(db.Objects)
			status["attr_def_count"] = len(db.AttrNames)
		}

		status["connections"] = a.controller.ConnectionStats()
		status["queue"] = a.controller.QueueStats()
		status["memory"] = a.controller.MemoryStats()

		gameStats := a.controller.GameStats()
		status["channels"] = gameStats["channels"]
		status["mail_enabled"] = gameStats["mail_enabled"]
		status["user_functions"] = gameStats["user_functions"]
	} else {
		status["running"] = false
		status["player_count"] = 0
		status["uptime"] = 0
	}

	writeJSON(w, http.StatusOK, status)
}

func (a *Admin) handleServerStart(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.controller == nil {
		writeError(w, http.StatusServiceUnavailable, "no server controller available")
		return
	}

	if a.controller.IsRunning() {
		writeError(w, http.StatusConflict, "server is already running")
		return
	}

	if err := a.controller.StartGame(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.setupMode = false
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (a *Admin) handleServerStop(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.controller == nil {
		writeError(w, http.StatusServiceUnavailable, "no server controller available")
		return
	}

	if !a.controller.IsRunning() {
		writeError(w, http.StatusConflict, "server is not running")
		return
	}

	if err := a.controller.StopGame(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
