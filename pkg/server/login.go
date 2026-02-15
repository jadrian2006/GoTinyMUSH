package server

import (
	"strings"

	mushcrypt "github.com/crystal-mush/gotinymush/pkg/crypt"
	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

const aPass = 5 // A_PASS attribute number

// ParseConnect parses a login-screen command into (command, user, password).
// Handles: "connect name password", "create name password", "connect guest"
func ParseConnect(msg string) (command, user, password string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "", "", ""
	}

	// Split into command and rest
	parts := strings.SplitN(msg, " ", 2)
	command = strings.ToLower(parts[0])
	if len(parts) < 2 {
		return command, "", ""
	}

	rest := strings.TrimSpace(parts[1])
	if rest == "" {
		return command, "", ""
	}

	// Handle quoted names (for names with spaces)
	if rest[0] == '"' {
		end := strings.Index(rest[1:], "\"")
		if end >= 0 {
			user = rest[1 : end+1]
			rest = strings.TrimSpace(rest[end+2:])
			password = rest
			return
		}
	}

	// Standard: name password
	parts = strings.SplitN(rest, " ", 2)
	user = parts[0]
	if len(parts) > 1 {
		password = strings.TrimSpace(parts[1])
	}
	return
}

// LookupPlayer finds a player by name in the database.
func LookupPlayer(db *gamedb.Database, name string) gamedb.DBRef {
	for _, obj := range db.Objects {
		if obj.ObjType() != gamedb.TypePlayer {
			continue
		}
		// Match on player name
		if strings.EqualFold(obj.Name, name) {
			return obj.DBRef
		}
		// Match on ALIAS attribute (A_ALIAS = 58)
		for _, attr := range obj.Attrs {
			if attr.Number == 58 {
				alias := eval.StripAttrPrefix(attr.Value)
				if alias != "" && strings.EqualFold(alias, name) {
					return obj.DBRef
				}
				break
			}
		}
	}
	return gamedb.Nothing
}

// CheckPassword verifies a password against the stored password for a player.
// TinyMUSH stores passwords in A_PASS (attr #5), possibly DES-encrypted with salt "XX".
// For our Go port, we support plaintext comparison and simple hash comparison.
func CheckPassword(db *gamedb.Database, player gamedb.DBRef, password string) bool {
	obj, ok := db.Objects[player]
	if !ok {
		return false
	}

	// Find A_PASS attribute
	for _, attr := range obj.Attrs {
		if attr.Number == aPass {
			stored := eval.StripAttrPrefix(attr.Value)
			if stored == "" {
				return false
			}
			// Direct comparison first (plaintext)
			if stored == password {
				return true
			}
			// Try DES crypt comparison
			if checkCrypt(password, stored) {
				return true
			}
			return false
		}
	}
	return false
}

// checkCrypt checks a password against a DES-encrypted stored password.
// TinyMUSH uses crypt(password, "XX") format.
func checkCrypt(password, stored string) bool {
	return mushcrypt.CheckPassword(password, stored)
}

// WelcomeText is the default welcome screen shown to new connections.
const WelcomeText = `
  ______      _______ _             __  __ _    _  _____ _    _
 / _____|    |__   __(_)           |  \/  | |  | |/ ____| |  | |
| |  __  ___    | |   _ _ __  _   _| \  / | |  | | (___ | |__| |
| | |_ |/ _ \   | |  | | '_ \| | | | |\/| | |  | |\___ \|  __  |
| |__| | (_) |  | |  | | | | | |_| | |  | | |__| |____) | |  | |
 \_____|\___/   |_|  |_|_| |_|\__, |_|  |_|\____/|_____/|_|  |_|
                                __/ |
                               |___/

"connect <name> <password>" to connect to your existing character.
"create <name> <password>" to create a new character.
"WHO" to see who is connected.
"QUIT" to disconnect.

`
