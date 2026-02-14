package server

import (
	"math"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Ensure Game implements eval.GameState.
var _ eval.GameState = (*Game)(nil)

// ConnectedPlayers returns all connected player dbrefs.
func (g *Game) ConnectedPlayers() []gamedb.DBRef {
	return g.Conns.ConnectedPlayers()
}

// ConnectedPlayersVisible returns connected players visible to viewer
// (excludes DARK wizards and UNFINDABLE players unless viewer is wizard).
func (g *Game) ConnectedPlayersVisible(viewer gamedb.DBRef) []gamedb.DBRef {
	all := g.Conns.ConnectedPlayers()
	isWiz := Wizard(g, viewer)
	if isWiz {
		return all
	}
	var visible []gamedb.DBRef
	for _, p := range all {
		if obj, ok := g.DB.Objects[p]; ok {
			if obj.HasFlag(gamedb.FlagDark) && obj.HasFlag(gamedb.FlagWizard) {
				continue
			}
			if obj.HasFlag2(gamedb.Flag2Unfindable) {
				continue
			}
		}
		visible = append(visible, p)
	}
	return visible
}

// ConnTime returns connection time in seconds for a player (-1 if not connected).
func (g *Game) ConnTime(player gamedb.DBRef) float64 {
	descs := g.Conns.GetByPlayer(player)
	if len(descs) == 0 {
		return -1
	}
	// Return the longest connection (first connected descriptor)
	var longest time.Duration
	now := time.Now()
	for _, d := range descs {
		dur := now.Sub(d.ConnTime)
		if dur > longest {
			longest = dur
		}
	}
	return math.Floor(longest.Seconds())
}

// IdleTime returns idle time in seconds for a player (-1 if not connected).
func (g *Game) IdleTime(player gamedb.DBRef) float64 {
	descs := g.Conns.GetByPlayer(player)
	if len(descs) == 0 {
		return -1
	}
	// Return the least idle descriptor
	var leastIdle time.Duration = time.Duration(math.MaxInt64)
	now := time.Now()
	for _, d := range descs {
		dur := now.Sub(d.LastCmd)
		if dur < leastIdle {
			leastIdle = dur
		}
	}
	return math.Floor(leastIdle.Seconds())
}

// DoingString returns a player's @doing string.
func (g *Game) DoingString(player gamedb.DBRef) string {
	descs := g.Conns.GetByPlayer(player)
	if len(descs) == 0 {
		return ""
	}
	return descs[0].DoingStr
}

// IsConnected returns true if the player has at least one active connection.
func (g *Game) IsConnected(player gamedb.DBRef) bool {
	return g.Conns.IsConnected(player)
}

// Teleport moves victim to destination, updating contents chains and persisting.
func (g *Game) Teleport(victim, dest gamedb.DBRef) {
	obj, ok := g.DB.Objects[victim]
	if !ok {
		return
	}
	oldLoc := obj.Location
	if oldLoc != gamedb.Nothing {
		g.RemoveFromContents(oldLoc, victim)
	}
	obj.Location = dest
	persistList := []*gamedb.Object{obj}
	if destObj, ok := g.DB.Objects[dest]; ok {
		obj.Next = destObj.Contents
		destObj.Contents = victim
		persistList = append(persistList, destObj)
	}
	if oldLoc != gamedb.Nothing {
		if oldLocObj, ok := g.DB.Objects[oldLoc]; ok {
			persistList = append(persistList, oldLocObj)
		}
	}
	g.PersistObjects(persistList...)
}

// LookupPlayer finds a player by name (exact and partial match).
func (g *Game) LookupPlayer(name string) gamedb.DBRef {
	name = strings.TrimSpace(name)
	if name == "" {
		return gamedb.Nothing
	}
	// Strip leading * for player matching
	if name[0] == '*' {
		name = name[1:]
	}
	// Try exact match first
	for _, obj := range g.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() && strings.EqualFold(obj.Name, name) {
			return obj.DBRef
		}
	}
	// Try prefix match
	nameLower := strings.ToLower(name)
	var match gamedb.DBRef = gamedb.Nothing
	matchCount := 0
	for _, obj := range g.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() {
			if strings.HasPrefix(strings.ToLower(obj.Name), nameLower) {
				match = obj.DBRef
				matchCount++
			}
		}
	}
	if matchCount == 1 {
		return match
	}
	if matchCount > 1 {
		return gamedb.Ambiguous
	}
	return gamedb.Nothing
}

// CouldDoIt checks if player passes the lock on thing for the given lock attribute.
func (g *Game) CouldDoIt(player, thing gamedb.DBRef, lockAttr int) bool {
	return CouldDoIt(g, player, thing, lockAttr)
}

// GetAttrTextGS returns the text of an attribute on an object (with parent walk).
func (g *Game) GetAttrTextGS(obj gamedb.DBRef, attrNum int) string {
	return g.GetAttrText(obj, attrNum)
}

// CanReadAttrGS checks if player can read a specific attribute on obj.
func (g *Game) CanReadAttrGS(player, obj gamedb.DBRef, attrNum int, rawValue string) bool {
	info := ParseAttrInfo(rawValue)
	def := g.LookupAttrDef(attrNum)
	return CanReadAttr(g, player, obj, def, info.Flags, info.Owner)
}

// SpellCheck returns misspelled words in text, considering player's custom dictionary.
func (g *Game) SpellCheck(player gamedb.DBRef, text string, grammar bool) []string {
	if g.Spell == nil {
		return nil
	}
	custom := g.gatherCustomWords(player)
	if grammar {
		issues := g.Spell.CheckTextWithGrammar(text, custom)
		var words []string
		for _, issue := range issues {
			words = append(words, issue.Word)
		}
		return words
	}
	return g.Spell.CheckText(text, custom)
}

// SpellHighlight returns text with misspelled words highlighted.
// Honors the player's ANSI flag for formatting.
func (g *Game) SpellHighlight(player gamedb.DBRef, text string, grammar bool) string {
	if g.Spell == nil {
		return text
	}
	custom := g.gatherCustomWords(player)
	useAnsi := g.playerHasAnsi(player)
	if grammar {
		return g.Spell.HighlightTextWithGrammar(text, custom, useAnsi)
	}
	return g.Spell.HighlightText(text, custom, useAnsi)
}

// ExecuteSQL executes a SQL query with permission checking.
func (g *Game) ExecuteSQL(player gamedb.DBRef, query, rowDelim, fieldDelim string) string {
	if g.SQLDB == nil {
		return "#-1 SQL NOT CONFIGURED"
	}
	// Permission: use_sql power or God
	obj, ok := g.DB.Objects[player]
	if !ok {
		return "#-1 PERMISSION DENIED"
	}
	if !obj.HasPower(1, gamedb.Pow2UseSQL) && !IsGod(g, player) {
		return "#-1 PERMISSION DENIED"
	}
	result, err := g.SQLDB.Query(query, rowDelim, fieldDelim)
	if err != nil {
		return "#-1 " + strings.ToUpper(err.Error())
	}
	return result
}

// EscapeSQL escapes a string for safe SQL interpolation.
func (g *Game) EscapeSQL(input string) string {
	return strings.ReplaceAll(input, "'", "''")
}

// playerHasAnsi returns true if the player has the ANSI flag set.
func (g *Game) playerHasAnsi(player gamedb.DBRef) bool {
	obj, ok := g.DB.Objects[player]
	if !ok {
		return false
	}
	return obj.HasFlag2(gamedb.Flag2Ansi)
}

// gatherCustomWords collects custom dictionary words for a player.
func (g *Game) gatherCustomWords(player gamedb.DBRef) map[string]bool {
	custom := make(map[string]bool)

	// 1. Player's DICTIONARY attr (space-separated words)
	dictText := g.GetAttrTextByName(player, "DICTIONARY")
	if dictText != "" {
		for _, w := range strings.Fields(dictText) {
			custom[strings.ToLower(w)] = true
		}
	}

	// 2. Player's name
	if obj, ok := g.DB.Objects[player]; ok {
		custom[strings.ToLower(obj.Name)] = true
	}

	// 3. All connected player names
	for _, p := range g.Conns.ConnectedPlayers() {
		if obj, ok := g.DB.Objects[p]; ok {
			custom[strings.ToLower(obj.Name)] = true
		}
	}

	// 4. Master room's DICTIONARY attr (global custom words)
	masterDict := g.GetAttrTextByName(g.MasterRoomRef(), "DICTIONARY")
	if masterDict != "" {
		for _, w := range strings.Fields(masterDict) {
			custom[strings.ToLower(w)] = true
		}
	}

	return custom
}

// EvalLockStr parses and evaluates a lock expression string.
func (g *Game) EvalLockStr(player, thing, actor gamedb.DBRef, lockStr string) bool {
	parsed := ParseBoolExp(g, player, lockStr)
	if parsed == nil { return false }
	return EvalBoolExp(g, actor, thing, thing, parsed, 0)
}

// HelpLookup retrieves help text for a given topic from the named help file.
func (g *Game) HelpLookup(_ gamedb.DBRef, fileID, topic string) string {
	var hf *HelpFile
	switch strings.ToLower(fileID) {
	case "help":
		hf = g.HelpMain
	case "wizhelp":
		hf = g.HelpWiz
	case "news":
		hf = g.HelpNews
	case "qhelp":
		hf = g.HelpQuick
	case "plushelp", "+help":
		hf = g.HelpPlus
	default:
		return ""
	}
	if hf == nil { return "" }
	return hf.Lookup(topic)
}

// SessionInfo returns session statistics for a connected player.
func (g *Game) SessionInfo(player gamedb.DBRef) (int, int, int) {
	descs := g.Conns.GetByPlayer(player)
	if len(descs) == 0 { return -1, -1, -1 }
	d := descs[0]
	return d.CmdCount, d.BytesSent, d.BytesRecv
}

// PersistStructDef saves or deletes a structure definition in bbolt.
func (g *Game) PersistStructDef(player gamedb.DBRef, name string, def *gamedb.StructDef) {
	if g.Store == nil {
		return
	}
	if def == nil {
		g.Store.DeleteStructDef(player, name)
	} else {
		g.Store.PutStructDef(player, def)
	}
}

// PersistStructInstance saves or deletes a structure instance in bbolt.
func (g *Game) PersistStructInstance(player gamedb.DBRef, name string, inst *gamedb.StructInstance) {
	if g.Store == nil {
		return
	}
	if inst == nil {
		g.Store.DeleteStructInstance(player, name)
	} else {
		g.Store.PutStructInstance(player, name, inst)
	}
}
