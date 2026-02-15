package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminPass = "goTinyMush"
	adminPassFile    = "admin_pass.hash" // stored in data dir
	sessionCookieKey = "gotinymush_admin"
	sessionMaxAge    = 24 * time.Hour
)

// adminAuth manages admin panel authentication.
type adminAuth struct {
	mu       sync.RWMutex
	dataDir  string
	envPass  string // from MUSH_ADMIN_PASS env var (always wins)
	sessions map[string]time.Time
}

func newAdminAuth(dataDir string) *adminAuth {
	return &adminAuth{
		dataDir:  dataDir,
		envPass:  os.Getenv("MUSH_ADMIN_PASS"),
		sessions: make(map[string]time.Time),
	}
}

// checkPassword verifies a password against the stored/env/default password.
// Priority: env var > stored hash file > default
func (aa *adminAuth) checkPassword(password string) bool {
	aa.mu.RLock()
	defer aa.mu.RUnlock()

	// Env var always takes priority (recovery mechanism)
	if aa.envPass != "" {
		return subtle.ConstantTimeCompare([]byte(password), []byte(aa.envPass)) == 1
	}

	// Check stored hash file
	if aa.dataDir != "" {
		hashPath := filepath.Join(aa.dataDir, adminPassFile)
		if hash, err := os.ReadFile(hashPath); err == nil {
			return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
		}
	}

	// Fall back to default
	return subtle.ConstantTimeCompare([]byte(password), []byte(defaultAdminPass)) == 1
}

// changePassword stores a new bcrypt hash in the data directory.
func (aa *adminAuth) changePassword(newPassword string) error {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if aa.dataDir == "" {
		return nil // nowhere to store it
	}

	hashPath := filepath.Join(aa.dataDir, adminPassFile)
	return os.WriteFile(hashPath, hash, 0600)
}

// createSession generates a new session token.
func (aa *adminAuth) createSession() string {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	// Clean expired sessions
	now := time.Now()
	for tok, exp := range aa.sessions {
		if now.After(exp) {
			delete(aa.sessions, tok)
		}
	}

	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	aa.sessions[token] = now.Add(sessionMaxAge)
	return token
}

// validateSession checks if a session token is valid.
func (aa *adminAuth) validateSession(token string) bool {
	aa.mu.RLock()
	defer aa.mu.RUnlock()

	exp, ok := aa.sessions[token]
	if !ok {
		return false
	}
	return time.Now().Before(exp)
}

// invalidateSession removes a session.
func (aa *adminAuth) invalidateSession(token string) {
	aa.mu.Lock()
	defer aa.mu.Unlock()
	delete(aa.sessions, token)
}

// isUsingDefault returns true if the password is still the default.
func (aa *adminAuth) isUsingDefault() bool {
	aa.mu.RLock()
	defer aa.mu.RUnlock()

	if aa.envPass != "" {
		return false // explicitly configured
	}
	if aa.dataDir != "" {
		hashPath := filepath.Join(aa.dataDir, adminPassFile)
		if _, err := os.Stat(hashPath); err == nil {
			return false // custom password set
		}
	}
	return true // using default
}

// authMiddleware wraps an http.Handler to require admin authentication.
// Exempts the login endpoint and static assets.
func (a *Admin) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Allow auth endpoints without auth
		if strings.HasPrefix(path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow static assets (SPA files) without auth — the SPA handles showing login UI
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check session cookie
		if cookie, err := r.Cookie(sessionCookieKey); err == nil {
			if a.auth.validateSession(cookie.Value) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check Authorization header (for API clients)
		if auth := r.Header.Get("Authorization"); auth != "" {
			token := strings.TrimPrefix(auth, "Bearer ")
			if a.auth.validateSession(token) {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

// handleAuthLogin handles POST /api/auth/login
func (a *Admin) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if !a.auth.checkPassword(req.Password) {
		log.Printf("admin: failed login attempt from %s", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token := a.auth.createSession()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    token,
		Path:     "/admin/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})

	log.Printf("admin: successful login from %s", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"token":          token,
		"default_password": a.auth.isUsingDefault(),
	})
}

// handleAuthLogout handles POST /api/auth/logout
func (a *Admin) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieKey); err == nil {
		a.auth.invalidateSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieKey,
		Value:    "",
		Path:     "/admin/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// handleAuthChangePassword handles POST /api/auth/change-password
func (a *Admin) handleAuthChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if !a.auth.checkPassword(req.Current) {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	if len(req.New) < 6 {
		writeError(w, http.StatusBadRequest, "new password must be at least 6 characters")
		return
	}

	if err := a.auth.changePassword(req.New); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save password: "+err.Error())
		return
	}

	log.Printf("admin: password changed from %s", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]string{"status": "changed"})
}

// handleAuthStatus handles GET /api/auth/status — checks if currently authenticated
func (a *Admin) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false

	if cookie, err := r.Cookie(sessionCookieKey); err == nil {
		authenticated = a.auth.validateSession(cookie.Value)
	}
	if !authenticated {
		if auth := r.Header.Get("Authorization"); auth != "" {
			token := strings.TrimPrefix(auth, "Bearer ")
			authenticated = a.auth.validateSession(token)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":    authenticated,
		"default_password": a.auth.isUsingDefault(),
	})
}
