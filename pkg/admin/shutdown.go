package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleServerShutdown initiates a graceful shutdown with @wall warnings.
func (a *Admin) handleServerShutdown(w http.ResponseWriter, r *http.Request) {
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

	// Check if shutdown is already in progress
	if s, _ := a.shutdownStatus.Load().(*ShutdownStatus); s != nil && s.Active {
		writeError(w, http.StatusConflict, "shutdown already in progress")
		return
	}

	var req struct {
		Delay  int    `json:"delay"`  // seconds until shutdown (default 300 = 5 min)
		Reason string `json:"reason"` // optional reason message
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body for default shutdown
		req.Delay = 300
	}
	if req.Delay <= 0 {
		req.Delay = 300
	}
	if req.Reason == "" {
		req.Reason = "Server maintenance"
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.shutdownCancel = cancel

	shutdownAt := time.Now().Add(time.Duration(req.Delay) * time.Second)
	status := &ShutdownStatus{
		Active:     true,
		Reason:     req.Reason,
		StartedAt:  time.Now(),
		ShutdownAt: shutdownAt,
		Remaining:  req.Delay,
		Stage:      "warning",
	}
	a.shutdownStatus.Store(status)

	log.Printf("admin: graceful shutdown initiated, %d seconds delay, reason: %s", req.Delay, req.Reason)

	// Launch the shutdown goroutine
	go a.runShutdownSequence(ctx, req.Delay, req.Reason)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "shutdown_initiated",
		"delay":       req.Delay,
		"reason":      req.Reason,
		"shutdown_at": shutdownAt.Format(time.RFC3339),
	})
}

// handleShutdownStatus returns the current shutdown status.
func (a *Admin) handleShutdownStatus(w http.ResponseWriter, r *http.Request) {
	s, _ := a.shutdownStatus.Load().(*ShutdownStatus)
	if s == nil || !s.Active {
		writeJSON(w, http.StatusOK, &ShutdownStatus{Active: false})
		return
	}
	// Update remaining seconds
	remaining := int(time.Until(s.ShutdownAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	resp := *s // copy
	resp.Remaining = remaining
	writeJSON(w, http.StatusOK, &resp)
}

// handleShutdownCancel cancels a pending shutdown.
func (a *Admin) handleShutdownCancel(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	s, _ := a.shutdownStatus.Load().(*ShutdownStatus)
	if s == nil || !s.Active {
		writeError(w, http.StatusConflict, "no shutdown in progress")
		return
	}

	if a.shutdownCancel != nil {
		a.shutdownCancel()
		a.shutdownCancel = nil
	}
	a.shutdownStatus.Store(&ShutdownStatus{Active: false})

	if a.controller != nil {
		a.controller.WallAll("## SHUTDOWN CANCELLED — server will continue running.")
	}

	log.Printf("admin: shutdown cancelled")
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// runShutdownSequence runs the timed shutdown with @wall warnings.
func (a *Admin) runShutdownSequence(ctx context.Context, delaySec int, reason string) {
	ctrl := a.controller
	if ctrl == nil {
		return
	}

	remaining := delaySec

	// Initial wall
	ctrl.WallAll(fmt.Sprintf("## SHUTDOWN: Server shutting down in %s. Reason: %s",
		formatDuration(remaining), reason))

	// Warning phase: wall every minute until < 60 seconds remain
	for remaining > 60 {
		sleepSec := 60
		// If next wall would overshoot, sleep less
		if remaining-sleepSec < 60 {
			sleepSec = remaining - 60
		}
		if sleepSec <= 0 {
			break
		}

		select {
		case <-ctx.Done():
			log.Printf("admin: shutdown sequence cancelled")
			return
		case <-time.After(time.Duration(sleepSec) * time.Second):
		}

		remaining -= sleepSec
		a.updateShutdownRemaining(remaining, "warning")
		ctrl.WallAll(fmt.Sprintf("## SHUTDOWN: Server shutting down in %s.",
			formatDuration(remaining)))
	}

	// Wait until 10 seconds remain
	if remaining > 10 {
		waitFor := remaining - 10
		select {
		case <-ctx.Done():
			log.Printf("admin: shutdown sequence cancelled")
			return
		case <-time.After(time.Duration(waitFor) * time.Second):
		}
		remaining = 10
	}

	// Countdown phase: wall every second for final 10 seconds
	a.updateShutdownRemaining(remaining, "countdown")
	for remaining > 0 {
		ctrl.WallAll(fmt.Sprintf("## SHUTDOWN IN %d...", remaining))

		select {
		case <-ctx.Done():
			log.Printf("admin: shutdown sequence cancelled during countdown")
			return
		case <-time.After(1 * time.Second):
		}
		remaining--
		a.updateShutdownRemaining(remaining, "countdown")
	}

	// Archive phase
	a.updateShutdownRemaining(0, "archiving")
	ctrl.WallAll("## SERVER SHUTTING DOWN — creating backup archive...")
	log.Printf("admin: creating pre-shutdown archive...")

	archivePath, err := ctrl.CreateArchive()
	if err != nil {
		log.Printf("admin: warning: pre-shutdown archive failed: %v", err)
		ctrl.WallAll("## Warning: backup archive failed, proceeding with shutdown.")
	} else {
		log.Printf("admin: pre-shutdown archive created: %s", archivePath)
	}

	// Disconnect phase
	a.updateShutdownRemaining(0, "disconnecting")
	ctrl.WallAll("## Server is going down NOW. Goodbye!")
	log.Printf("admin: disconnecting all players and stopping server...")

	// Brief pause to let the final wall message flush
	time.Sleep(500 * time.Millisecond)

	ctrl.Shutdown()

	a.updateShutdownRemaining(0, "done")
	log.Printf("admin: graceful shutdown complete")
}

// updateShutdownRemaining updates the shutdown status with current remaining time.
func (a *Admin) updateShutdownRemaining(remaining int, stage string) {
	s, _ := a.shutdownStatus.Load().(*ShutdownStatus)
	if s == nil {
		return
	}
	updated := *s
	updated.Remaining = remaining
	updated.Stage = stage
	a.shutdownStatus.Store(&updated)
}

// formatDuration returns a human-readable duration string.
func formatDuration(seconds int) string {
	if seconds >= 120 {
		return fmt.Sprintf("%d minutes", seconds/60)
	}
	if seconds >= 60 {
		return "1 minute"
	}
	return fmt.Sprintf("%d seconds", seconds)
}
