package server

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"gopkg.in/yaml.v3"
)

// GameConf holds game-level configuration parameters.
// Supports both YAML (.yaml/.yml) and legacy TinyMUSH text (.conf) formats.
type GameConf struct {
	// --- Identity ---
	MudName string `yaml:"mud_name"`
	Port    int    `yaml:"port"`

	// --- Key rooms ---
	MasterRoom         int `yaml:"master_room"`
	PlayerStartingRoom int `yaml:"player_starting_room"`
	PlayerStartingHome int `yaml:"player_starting_home"`
	DefaultHome        int `yaml:"default_home"`

	// --- Economy ---
	MoneyNameSingular string `yaml:"money_name_singular"`
	MoneyNamePlural   string `yaml:"money_name_plural"`
	StartingMoney     int    `yaml:"starting_money"`
	Paycheck          int    `yaml:"paycheck"`
	EarnLimit         int    `yaml:"earn_limit"`
	PageCost          int    `yaml:"page_cost"`
	WaitCost          int    `yaml:"wait_cost"`
	LinkCost          int    `yaml:"link_cost"`

	// --- Idle/timeout ---
	IdleTimeout int  `yaml:"idle_timeout"`
	IdleWizDark bool `yaml:"idle_wiz_dark"`

	// --- Queue ---
	QueueIdleChunk          int `yaml:"queue_idle_chunk"`
	FunctionInvocationLimit int `yaml:"function_invocation_limit"`
	MachineCommandCost      int `yaml:"machine_command_cost"`

	// --- Output ---
	OutputLimit int `yaml:"output_limit"`

	// --- Permissions ---
	MatchOwnCommands       bool `yaml:"match_own_commands"`
	PlayerMatchOwnCommands bool `yaml:"player_match_own_commands"`
	PemitFarPlayers        bool `yaml:"pemit_far_players"`
	PemitAnyObject         bool `yaml:"pemit_any_object"`
	ExaminePublicAttrs     bool `yaml:"examine_public_attrs"`
	PublicFlags            bool `yaml:"public_flags"`
	ReadRemoteName         bool `yaml:"read_remote_name"`
	RequireCmdsFlag        bool `yaml:"require_cmds_flag"`
	SwitchDefaultAll       bool `yaml:"switch_default_all"`
	SweepDark              bool `yaml:"sweep_dark"`
	TraceTopdown           bool `yaml:"trace_topdown"`
	TraceOutputLimit       int  `yaml:"trace_output_limit"`

	// --- Guest ---
	GuestCharNum   int    `yaml:"guest_char_num"`
	GuestPrefixes  string `yaml:"guest_prefixes"`
	GuestSuffixes  string `yaml:"guest_suffixes"`
	GuestBasename  string `yaml:"guest_basename"`
	NumberGuests   int    `yaml:"number_guests"`
	GuestPassword  string `yaml:"guest_password"`
	GuestStartRoom int    `yaml:"guest_start_room"`

	// --- Pueblo ---
	PuebloEnabled bool   `yaml:"pueblo_enabled"`
	PuebloVersion string `yaml:"pueblo_version"`

	// --- Module toggles ---
	MailEnabled   bool `yaml:"mail_enabled"`
	ComsysEnabled bool `yaml:"comsys_enabled"`
	MailExpiration int  `yaml:"mail_expiration"` // Days before auto-expire, 0 = never

	// --- Channels (stored for future comsys) ---
	PublicChannel string `yaml:"public_channel"`
	PublicCalias  string `yaml:"public_calias"`
	GuestsChannel string `yaml:"guests_channel"`
	GuestsCalias  string `yaml:"guests_calias"`

	// --- Security ---
	GodDBRef      int `yaml:"god_dbref"`       // The God player dbref (default 1)
	ZoneNestLimit int `yaml:"zone_nest_limit"` // Max zone recursion depth (default 20)

	// --- TLS ---
	Cleartext *bool  `yaml:"cleartext"` // nil = default true; explicitly false disables plaintext
	TLS       bool   `yaml:"tls"`
	TLSPort   int    `yaml:"tls_port"`
	TLSCert   string `yaml:"tls_cert"`
	TLSKey    string `yaml:"tls_key"`

	// --- Spellcheck ---
	SpellcheckEnabled bool   `yaml:"spellcheck_enabled"`
	SpellcheckURL     string `yaml:"spellcheck_url"`

	// --- SQL ---
	SQLEnabled    bool   `yaml:"sql_enabled"`     // Master enable/disable
	SQLDatabase   string `yaml:"sql_database"`    // Path to SQLite3 file
	SQLQueryLimit int    `yaml:"sql_query_limit"` // Max rows returned (default 100)
	SQLTimeout    int    `yaml:"sql_timeout"`     // Query timeout in seconds (default 5)
	SQLReconnect  bool   `yaml:"sql_reconnect"`   // Auto-reconnect on failure

	// --- Archive/Backup ---
	ArchiveDir      string `yaml:"archive_dir"`       // Archive output directory (default: "backups")
	ArchiveInterval int    `yaml:"archive_interval"`  // Auto-archive interval in minutes, 0 = disabled
	ArchiveRetain   int    `yaml:"archive_retain"`    // Keep last N archives, 0 = unlimited
	ArchiveHook     string `yaml:"archive_hook"`      // Shell command to run after archive, %f = archive path

	// --- Web/Security ---
	WebEnabled    bool     `yaml:"web_enabled"`     // Enable HTTPS/WSS server
	WebPort       int      `yaml:"web_port"`        // HTTPS port (default 8443)
	WebHost       string   `yaml:"web_host"`        // Bind address (empty = all interfaces)
	WebDomain     string   `yaml:"web_domain"`      // Let's Encrypt domain (empty = self-signed)
	WebStaticDir  string   `yaml:"web_static_dir"`  // Path to built web client (default "web/dist")
	WebClientURL  string   `yaml:"web_client_url"`  // URL of external web client container (e.g. "http://web-client:80"); if set, / is reverse-proxied to it
	WebCORSOrigins []string `yaml:"web_cors_origins"` // Allowed CORS origins
	WebRateLimit  int      `yaml:"web_rate_limit"`  // Requests per minute per IP (default 60)
	JWTSecret     string   `yaml:"jwt_secret"`      // JWT signing secret (auto-generated if empty)
	JWTExpiry     int      `yaml:"jwt_expiry"`      // JWT expiry in seconds (default 86400)
	CertDir       string   `yaml:"cert_dir"`        // Directory for generated certs (default "certs")
	ScrollbackRetention int `yaml:"scrollback_retention"` // Public scrollback retention in seconds (default 86400)

	// --- Alias config includes (YAML: list of paths; legacy: from "include" directives) ---
	AliasFiles []string `yaml:"alias_files"`

	// --- Attribute access config ---
	UserAttrAccess string   `yaml:"user_attr_access"` // Default flags for user-defined attrs
	AttrTypes      []string `yaml:"attr_types"`       // Pattern-based attr flag assignment
	AttrAccess     []string `yaml:"attr_access"`      // @attribute/access directives (deferred)

	// --- Internal: resolved include paths from legacy .conf parsing ---
	IncludedAliasConfs []string `yaml:"-"`
}

// DefaultGameConf returns a GameConf with TinyMUSH-compatible defaults.
func DefaultGameConf() *GameConf {
	return &GameConf{
		MudName:                 "GoTinyMUSH",
		Port:                    6250,
		MasterRoom:              2,
		PlayerStartingRoom:      0,
		PlayerStartingHome:      0,
		DefaultHome:             0,
		MoneyNameSingular:       "penny",
		MoneyNamePlural:         "pennies",
		StartingMoney:           150,
		Paycheck:                50,
		EarnLimit:               10000,
		PageCost:                0,
		WaitCost:                10,
		LinkCost:                1,
		IdleTimeout:             3600,
		IdleWizDark:             false,
		QueueIdleChunk:          3,
		FunctionInvocationLimit: 2500,
		MachineCommandCost:      64,
		OutputLimit:             16384,
		MatchOwnCommands:        false,
		PlayerMatchOwnCommands:  false,
		PemitFarPlayers:         false,
		PemitAnyObject:          false,
		ExaminePublicAttrs:      true,
		PublicFlags:             true,
		ReadRemoteName:          false,
		RequireCmdsFlag:         true,
		SwitchDefaultAll:        true,
		SweepDark:               false,
		TraceTopdown:            true,
		TraceOutputLimit:        200,
		GuestCharNum:            -1,
		GuestBasename:           "Guest",
		NumberGuests:            30,
		GuestPassword:           "guest",
		GuestStartRoom:          -1,
		GodDBRef:                1,
		ZoneNestLimit:           20,
		MailEnabled:             true,
		ComsysEnabled:           true,
		MailExpiration:          14,
		PuebloEnabled:           false,
		PuebloVersion:           "This world is Pueblo 1.0 enhanced",
		SpellcheckEnabled:       false,
		SpellcheckURL:           "https://api.languagetool.org/v2/check",
		SQLEnabled:              false,
		SQLQueryLimit:           100,
		SQLTimeout:              5,
		SQLReconnect:            true,
		ArchiveDir:              "backups",
		WebEnabled:              true,
		WebPort:                 8443,
		WebStaticDir:            "web/dist",
		WebRateLimit:            60,
		JWTExpiry:               86400,
		CertDir:                 "",
		ScrollbackRetention:     86400,
	}
}

// LoadGameConf loads a game config file. Format is auto-detected by extension:
//   - .yaml / .yml  -> YAML format
//   - .conf / other -> legacy TinyMUSH text format
func LoadGameConf(path string) (*GameConf, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return loadGameConfYAML(path)
	default:
		return loadGameConfLegacy(path)
	}
}

// --- YAML loader ---

func loadGameConfYAML(path string) (*GameConf, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	gc := DefaultGameConf()
	if err := yaml.Unmarshal(data, gc); err != nil {
		return nil, fmt.Errorf("parsing YAML %s: %w", path, err)
	}

	// Resolve alias_files paths relative to config dir
	baseDir := filepath.Dir(path)
	for i, af := range gc.AliasFiles {
		if !filepath.IsAbs(af) {
			gc.AliasFiles[i] = filepath.Join(baseDir, af)
		}
	}
	gc.IncludedAliasConfs = gc.AliasFiles

	return gc, nil
}

// --- Legacy TinyMUSH text loader ---

func loadGameConfLegacy(path string) (*GameConf, error) {
	gc := DefaultGameConf()
	if err := gc.loadLegacyFile(path, 0); err != nil {
		return nil, err
	}
	return gc, nil
}

func (gc *GameConf) loadLegacyFile(path string, depth int) error {
	if depth > 10 {
		return fmt.Errorf("include depth exceeded (circular include?)")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	baseDir := filepath.Dir(path)

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		// Lines starting with @ are runtime directives — capture @attribute/access
		if line[0] == '@' {
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "@attribute/access ") {
				gc.AttrAccess = append(gc.AttrAccess, strings.TrimSpace(line[len("@attribute/access "):]))
			}
			continue
		}

		// Split on first whitespace (space or tab)
		key, val := splitKeyVal(line)
		if key == "" {
			continue
		}
		key = strings.ToLower(key)

		switch key {
		// --- Include ---
		case "include":
			includePath := val
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(baseDir, includePath)
			}
			// Track alias/compat conf includes for later loading
			baseName := strings.ToLower(filepath.Base(val))
			if strings.Contains(baseName, "alias") || strings.Contains(baseName, "compat") {
				gc.IncludedAliasConfs = append(gc.IncludedAliasConfs, includePath)
			} else {
				// Generic include - load recursively
				if err := gc.loadLegacyFile(includePath, depth+1); err != nil {
					log.Printf("gameconf: warning: include %s: %v", val, err)
				}
			}

		// --- Identity ---
		case "mud_name":
			gc.MudName = val
		case "port":
			gc.Port = atoi(val, gc.Port)

		// --- Key rooms ---
		case "master_room":
			gc.MasterRoom = atoi(val, gc.MasterRoom)
		case "player_starting_room":
			gc.PlayerStartingRoom = atoi(val, gc.PlayerStartingRoom)
		case "player_starting_home":
			gc.PlayerStartingHome = atoi(val, gc.PlayerStartingHome)
		case "default_home":
			gc.DefaultHome = atoi(val, gc.DefaultHome)

		// --- Economy ---
		case "money_name_singular":
			gc.MoneyNameSingular = val
		case "money_name_plural":
			gc.MoneyNamePlural = val
		case "starting_money":
			gc.StartingMoney = atoi(val, gc.StartingMoney)
		case "paycheck":
			gc.Paycheck = atoi(val, gc.Paycheck)
		case "earn_limit":
			gc.EarnLimit = atoi(val, gc.EarnLimit)
		case "page_cost":
			gc.PageCost = atoi(val, gc.PageCost)
		case "wait_cost":
			gc.WaitCost = atoi(val, gc.WaitCost)
		case "link_cost":
			gc.LinkCost = atoi(val, gc.LinkCost)

		// --- Idle/timeout ---
		case "idle_timeout":
			gc.IdleTimeout = atoi(val, gc.IdleTimeout)
		case "idle_wiz_dark":
			gc.IdleWizDark = parseBool(val)

		// --- Queue ---
		case "queue_idle_chunk":
			gc.QueueIdleChunk = atoi(val, gc.QueueIdleChunk)
		case "function_invocation_limit":
			gc.FunctionInvocationLimit = atoi(val, gc.FunctionInvocationLimit)
		case "machine_command_cost":
			gc.MachineCommandCost = atoi(val, gc.MachineCommandCost)

		// --- Output ---
		case "output_limit":
			gc.OutputLimit = atoi(val, gc.OutputLimit)

		// --- Permissions ---
		case "match_own_commands":
			gc.MatchOwnCommands = parseBool(val)
		case "player_match_own_commands":
			gc.PlayerMatchOwnCommands = parseBool(val)
		case "pemit_far_players":
			gc.PemitFarPlayers = parseBool(val)
		case "pemit_any_object":
			gc.PemitAnyObject = parseBool(val)
		case "examine_public_attrs":
			gc.ExaminePublicAttrs = parseBool(val)
		case "public_flags":
			gc.PublicFlags = parseBool(val)
		case "read_remote_name":
			gc.ReadRemoteName = parseBool(val)
		case "require_cmds_flag":
			gc.RequireCmdsFlag = parseBool(val)
		case "switch_default_all":
			gc.SwitchDefaultAll = parseBool(val)
		case "sweep_dark":
			gc.SweepDark = parseBool(val)
		case "trace_topdown":
			gc.TraceTopdown = parseBool(val)
		case "trace_output_limit":
			gc.TraceOutputLimit = atoi(val, gc.TraceOutputLimit)

		// --- Guest ---
		case "guest_char_num":
			gc.GuestCharNum = atoi(val, gc.GuestCharNum)
		case "guest_prefixes":
			gc.GuestPrefixes = val
		case "guest_suffixes":
			gc.GuestSuffixes = val
		case "guest_basename":
			gc.GuestBasename = val

		// --- Pueblo ---
		case "have_pueblo", "pueblo_enabled":
			gc.PuebloEnabled = parseBool(val)
		case "pueblo_version":
			gc.PuebloVersion = val

		// --- Module toggles ---
		case "mail_enabled":
			gc.MailEnabled = parseBool(val)
		case "comsys_enabled":
			gc.ComsysEnabled = parseBool(val)
		case "mail_expiration":
			gc.MailExpiration = atoi(val, gc.MailExpiration)

		// --- Channels ---
		case "public_channel":
			gc.PublicChannel = val
		case "public_calias":
			gc.PublicCalias = val
		case "guests_channel":
			gc.GuestsChannel = val
		case "guests_calias":
			gc.GuestsCalias = val

		// --- Security ---
		case "god_dbref":
			gc.GodDBRef = atoi(val, gc.GodDBRef)
		case "zone_nest_limit":
			gc.ZoneNestLimit = atoi(val, gc.ZoneNestLimit)

		// --- SQL ---
		case "sql_enabled":
			gc.SQLEnabled = parseBool(val)
		case "sql_database":
			gc.SQLDatabase = val
		case "sql_query_limit":
			gc.SQLQueryLimit = atoi(val, gc.SQLQueryLimit)
		case "sql_timeout":
			gc.SQLTimeout = atoi(val, gc.SQLTimeout)
		case "sql_reconnect":
			gc.SQLReconnect = parseBool(val)

		// --- Archive ---
		case "archive_dir":
			gc.ArchiveDir = val
		case "archive_interval":
			gc.ArchiveInterval = atoi(val, gc.ArchiveInterval)
		case "archive_retain":
			gc.ArchiveRetain = atoi(val, gc.ArchiveRetain)
		case "archive_hook":
			gc.ArchiveHook = val

		// --- TLS ---
		case "cleartext":
			v := parseBool(val)
			gc.Cleartext = &v
		case "tls":
			gc.TLS = parseBool(val)
		case "tls_port":
			gc.TLSPort = atoi(val, gc.TLSPort)
		case "tls_cert":
			gc.TLSCert = val
		case "tls_key":
			gc.TLSKey = val

		// --- Web/Security ---
		case "web_enabled":
			gc.WebEnabled = parseBool(val)
		case "web_port":
			gc.WebPort = atoi(val, gc.WebPort)
		case "web_host":
			gc.WebHost = val
		case "web_domain":
			gc.WebDomain = val
		case "web_static_dir":
			gc.WebStaticDir = val
		case "web_cors_origins":
			gc.WebCORSOrigins = strings.Split(val, ",")
			for i := range gc.WebCORSOrigins {
				gc.WebCORSOrigins[i] = strings.TrimSpace(gc.WebCORSOrigins[i])
			}
		case "web_rate_limit":
			gc.WebRateLimit = atoi(val, gc.WebRateLimit)
		case "jwt_secret":
			gc.JWTSecret = val
		case "jwt_expiry":
			gc.JWTExpiry = atoi(val, gc.JWTExpiry)
		case "cert_dir":
			gc.CertDir = val
		case "scrollback_retention":
			gc.ScrollbackRetention = atoi(val, gc.ScrollbackRetention)

		// --- Attribute access config ---
		case "user_attr_access":
			gc.UserAttrAccess = val
		case "attr_type":
			gc.AttrTypes = append(gc.AttrTypes, val)
		case "attr_access":
			gc.AttrAccess = append(gc.AttrAccess, val)

		// --- Directives handled elsewhere ---
		case "alias", "flag_alias", "function_alias", "attr_alias", "power_alias", "bad_name":
			// Handled by LoadAliasConfig

		// --- Known but not-yet-implemented ---
		case "module", "helpfile", "raw_helpfile", "access", "register_site":
			log.Printf("gameconf: noted directive %q (not yet implemented): %s", key, val)

		default:
			// Unknown directives silently ignored for forward compatibility
		}
	}
	return scanner.Err()
}

// splitKeyVal splits a line on the first whitespace (space or tab).
func splitKeyVal(line string) (string, string) {
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			return line[:i], strings.TrimSpace(line[i+1:])
		}
	}
	return line, ""
}

// --- Apply config to Game ---

// ApplyGameConf applies a parsed game config to the Game.
func (g *Game) ApplyGameConf(gc *GameConf) {
	g.Conf = gc

	log.Printf("Game config applied: mud_name=%q master_room=#%d start_room=#%d start_home=#%d",
		gc.MudName, gc.MasterRoom, gc.PlayerStartingRoom, gc.PlayerStartingHome)
	log.Printf("  economy: %s/%s starting=%d paycheck=%d",
		gc.MoneyNameSingular, gc.MoneyNamePlural, gc.StartingMoney, gc.Paycheck)

	// Apply deferred attribute access directives (requires DB to be loaded)
	if gc.UserAttrAccess != "" {
		g.ApplyUserAttrAccess(gc.UserAttrAccess)
	}
	for _, at := range gc.AttrTypes {
		g.ApplyAttrType(at)
	}
	for _, aa := range gc.AttrAccess {
		g.ApplyAttrAccess(aa)
	}
}

// MasterRoomRef returns the configured master room dbref.
func (g *Game) MasterRoomRef() gamedb.DBRef {
	if g.Conf != nil {
		return gamedb.DBRef(g.Conf.MasterRoom)
	}
	return gamedb.DBRef(2)
}

// StartingRoom returns the configured player starting room.
func (g *Game) StartingRoom() gamedb.DBRef {
	if g.Conf != nil {
		return gamedb.DBRef(g.Conf.PlayerStartingRoom)
	}
	return gamedb.DBRef(0)
}

// StartingHome returns the configured player starting home.
func (g *Game) StartingHome() gamedb.DBRef {
	if g.Conf != nil {
		return gamedb.DBRef(g.Conf.PlayerStartingHome)
	}
	return g.StartingRoom()
}

// MoneyName returns the singular or plural money name.
func (g *Game) MoneyName(amount int) string {
	if g.Conf != nil {
		if amount == 1 {
			return g.Conf.MoneyNameSingular
		}
		return g.Conf.MoneyNamePlural
	}
	if amount == 1 {
		return "penny"
	}
	return "pennies"
}

// IsCleartext returns whether the cleartext listener is enabled.
// Defaults to true if not explicitly set.
func (gc *GameConf) IsCleartext() bool {
	if gc.Cleartext == nil {
		return true
	}
	return *gc.Cleartext
}

// --- Helper functions ---

func atoi(s string, fallback int) int {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "yes" || s == "true" || s == "1" || s == "on"
}

// GodPlayer returns the configured God player dbref.
func (g *Game) GodPlayer() gamedb.DBRef {
	if g.Conf != nil {
		return gamedb.DBRef(g.Conf.GodDBRef)
	}
	return gamedb.DBRef(1)
}

// ZoneNestLimit returns the configured max zone recursion depth.
func (g *Game) ZoneNestLimit() int {
	if g.Conf != nil && g.Conf.ZoneNestLimit > 0 {
		return g.Conf.ZoneNestLimit
	}
	return 20
}
