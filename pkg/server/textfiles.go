package server

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// TextFiles holds cached text file contents served at various connection
// lifecycle points (welcome screen, MOTD, quit message, etc.).
type TextFiles struct {
	mu        sync.RWMutex
	Connect   string // connect.txt — welcome screen
	Motd      string // motd.txt — post-login MOTD
	WizMotd   string // wizmotd.txt — wizard MOTD
	Quit      string // quit.txt — quit message
	NewUser   string // newuser.txt — new character message
	Down      string // down.txt — logins disabled
	Full      string // full.txt — too many connections
	BadSite   string // badsite.txt — banned site
	Guest     string // guest.txt — guest connect
	Register  string // register.txt — registration-only
	CreateReg string // create_reg.txt — create reg fail
	HTMLConn  string // htmlconn.txt — Pueblo HTML welcome
}

// trackedFiles maps filenames to their TextFiles field descriptions.
var trackedFiles = []struct {
	Name string
	Desc string
}{
	{"connect.txt", "welcome screen"},
	{"motd.txt", "post-login MOTD"},
	{"wizmotd.txt", "wizard MOTD"},
	{"quit.txt", "quit message"},
	{"newuser.txt", "new character message"},
	{"down.txt", "logins disabled"},
	{"full.txt", "too many connections"},
	{"badsite.txt", "banned site"},
	{"guest.txt", "guest connect"},
	{"register.txt", "registration-only"},
	{"create_reg.txt", "create reg fail"},
	{"htmlconn.txt", "Pueblo HTML welcome"},
}

// Get returns a snapshot of a text field by reading under the lock.
// Use named accessors below instead of direct field access.
func (tf *TextFiles) GetConnect() string   { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Connect }
func (tf *TextFiles) GetMotd() string      { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Motd }
func (tf *TextFiles) GetWizMotd() string   { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.WizMotd }
func (tf *TextFiles) GetQuit() string      { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Quit }
func (tf *TextFiles) GetNewUser() string   { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.NewUser }
func (tf *TextFiles) GetDown() string      { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Down }
func (tf *TextFiles) GetFull() string      { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Full }
func (tf *TextFiles) GetBadSite() string   { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.BadSite }
func (tf *TextFiles) GetGuest() string     { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Guest }
func (tf *TextFiles) GetRegister() string  { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.Register }
func (tf *TextFiles) GetCreateReg() string { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.CreateReg }
func (tf *TextFiles) GetHTMLConn() string  { tf.mu.RLock(); defer tf.mu.RUnlock(); return tf.HTMLConn }

// loadFile reads a single text file, returning empty string on any error.
func loadFile(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return string(data)
}

// LoadTextFiles reads text files from dir and returns a populated TextFiles.
// Missing or empty files result in empty strings (no error).
func LoadTextFiles(dir string) *TextFiles {
	tf := &TextFiles{}
	tf.loadAll(dir)
	return tf
}

// loadAll populates all fields from the given directory.
func (tf *TextFiles) loadAll(dir string) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	tf.Connect = loadFile(dir, "connect.txt")
	tf.Motd = loadFile(dir, "motd.txt")
	tf.WizMotd = loadFile(dir, "wizmotd.txt")
	tf.Quit = loadFile(dir, "quit.txt")
	tf.NewUser = loadFile(dir, "newuser.txt")
	tf.Down = loadFile(dir, "down.txt")
	tf.Full = loadFile(dir, "full.txt")
	tf.BadSite = loadFile(dir, "badsite.txt")
	tf.Guest = loadFile(dir, "guest.txt")
	tf.Register = loadFile(dir, "register.txt")
	tf.CreateReg = loadFile(dir, "create_reg.txt")
	tf.HTMLConn = loadFile(dir, "htmlconn.txt")

	count := 0
	for _, v := range []string{
		tf.Connect, tf.Motd, tf.WizMotd, tf.Quit, tf.NewUser,
		tf.Down, tf.Full, tf.BadSite, tf.Guest, tf.Register, tf.CreateReg,
		tf.HTMLConn,
	} {
		if v != "" {
			count++
		}
	}
	log.Printf("Loaded %d text files from %s", count, dir)
}

// ReloadTextFiles reloads all cached text files from the configured TextDir.
// Returns the count of non-empty files loaded.
func (g *Game) ReloadTextFiles() int {
	if g.TextDir == "" || g.Texts == nil {
		return 0
	}
	g.Texts.loadAll(g.TextDir)

	g.Texts.mu.RLock()
	count := 0
	for _, v := range []string{
		g.Texts.Connect, g.Texts.Motd, g.Texts.WizMotd, g.Texts.Quit, g.Texts.NewUser,
		g.Texts.Down, g.Texts.Full, g.Texts.BadSite, g.Texts.Guest, g.Texts.Register, g.Texts.CreateReg,
		g.Texts.HTMLConn,
	} {
		if v != "" {
			count++
		}
	}
	g.Texts.mu.RUnlock()
	return count
}

// NotifyWizards sends a message to all connected wizards.
func (g *Game) NotifyWizards(msg string) {
	for _, dd := range g.Conns.AllDescriptors() {
		if dd.State != ConnConnected {
			continue
		}
		if Wizard(g, dd.Player) {
			dd.Send(msg)
		}
	}
}

// WatchTextFiles starts an fsnotify watcher on the text directory.
// When tracked files change, it notifies all connected wizards.
func (g *Game) WatchTextFiles() {
	if g.TextDir == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("WARNING: Could not start text file watcher: %v", err)
		return
	}

	// Build set of tracked filenames for fast lookup
	tracked := make(map[string]bool)
	for _, tf := range trackedFiles {
		tracked[tf.Name] = true
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				name := filepath.Base(event.Name)
				if !tracked[name] {
					continue
				}
				// Find description for the changed file
				desc := name
				for _, tf := range trackedFiles {
					if strings.EqualFold(tf.Name, name) {
						desc = fmt.Sprintf("%s (%s)", name, tf.Desc)
						break
					}
				}
				log.Printf("Text file changed: %s", desc)
				g.NotifyWizards(fmt.Sprintf("GAME: Text file changed on disk: %s — use @readcache to reload.", desc))

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Text file watcher error: %v", err)
			}
		}
	}()

	if err := watcher.Add(g.TextDir); err != nil {
		log.Printf("WARNING: Could not watch text directory %s: %v", g.TextDir, err)
		watcher.Close()
		return
	}
	log.Printf("Watching text directory for changes: %s", g.TextDir)
}
