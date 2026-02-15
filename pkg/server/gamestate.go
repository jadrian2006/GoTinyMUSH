package server

import (
	"fmt"
	"math"
	"sort"
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

// MailCount returns (total, unread, cleared) for a player.
func (g *Game) MailCount(player gamedb.DBRef) (int, int, int) {
	if g.Mail == nil {
		return -1, -1, -1
	}
	return g.Mail.CountMessages(player)
}

// MailFrom returns the sender of message #num for player.
func (g *Game) MailFrom(player gamedb.DBRef, num int) gamedb.DBRef {
	if g.Mail == nil {
		return gamedb.Nothing
	}
	msg := g.Mail.GetMessage(player, num)
	if msg == nil {
		return gamedb.Nothing
	}
	return msg.From
}

// MailSubject returns the subject of message #num for player.
func (g *Game) MailSubject(player gamedb.DBRef, num int) string {
	if g.Mail == nil {
		return ""
	}
	msg := g.Mail.GetMessage(player, num)
	if msg == nil {
		return ""
	}
	return msg.Subject
}

// ChannelInfo returns a field value for a channel by name.
// Requires the caller to be the channel owner or a Wizard.
func (g *Game) ChannelInfo(player gamedb.DBRef, name, field string) string {
	if g.Comsys == nil {
		return ""
	}
	ch := g.Comsys.GetChannel(name)
	if ch == nil {
		return ""
	}
	if !Wizard(g, player) && player != ch.Owner {
		return "#-1 PERMISSION DENIED"
	}
	switch strings.ToLower(field) {
	case "owner":
		return fmt.Sprintf("#%d", ch.Owner)
	case "description", "desc":
		return ch.Description
	case "header":
		return ch.Header
	case "flags":
		var flags []string
		if ch.Flags&gamedb.ChanPublic != 0 {
			flags = append(flags, "Public")
		} else {
			flags = append(flags, "Private")
		}
		if ch.Flags&gamedb.ChanLoud != 0 {
			flags = append(flags, "Loud")
		}
		if ch.Flags&gamedb.ChanObject != 0 {
			flags = append(flags, "Objects")
		}
		if ch.Flags&gamedb.ChanNoTitles != 0 {
			flags = append(flags, "NoTitles")
		}
		return strings.Join(flags, " ")
	case "numsent", "messages":
		return fmt.Sprintf("%d", ch.NumSent)
	case "subscribers", "numusers":
		subs := g.Comsys.ChannelSubscribers(ch.Name)
		return fmt.Sprintf("%d", len(subs))
	case "joinlock":
		return ch.JoinLock
	case "translock":
		return ch.TransLock
	case "recvlock":
		return ch.RecvLock
	case "charge":
		return fmt.Sprintf("%d", ch.Charge)
	default:
		return ""
	}
}

// ListAttrDefs returns a space-separated list of user-defined attribute names
// matching the given pattern. Non-wizards only see VISUAL attr definitions.
// parseObjTypeFilter converts a type name string to an ObjectType int (-1 if none).
func parseObjTypeFilter(objType string) int {
	switch strings.ToUpper(strings.TrimSpace(objType)) {
	case "PLAYER":
		return int(gamedb.TypePlayer)
	case "THING", "OBJECT":
		return int(gamedb.TypeThing)
	case "ROOM":
		return int(gamedb.TypeRoom)
	case "EXIT":
		return int(gamedb.TypeExit)
	}
	return -1
}

func (g *Game) ListAttrDefs(player gamedb.DBRef, pattern string, objType string) string {
	isWiz := Wizard(g, player)
	pat := strings.ToLower(strings.TrimSpace(pattern))
	if pat == "" {
		pat = "*"
	}

	typeFilter := parseObjTypeFilter(objType)

	// Count attrs on relevant objects
	attrCounts := countAttrsOnObjects(g, player, typeFilter, isWiz)

	showAll := pat != "*" // Only show flagged attrs when no specific pattern given
	type entry struct {
		name  string
		flags string
		count int
	}
	var results []entry
	for num, def := range g.DB.AttrNames {
		if !showAll && typeFilter < 0 && def.Flags == 0 {
			continue
		}
		if typeFilter >= 0 {
			if attrCounts[num] == 0 {
				continue
			}
			if def.Flags == 0 {
				continue
			}
		}
		if typeFilter < 0 && !isWiz && def.Flags&gamedb.AFVisual == 0 {
			continue
		}
		if !wildMatchSimple(pat, strings.ToLower(def.Name)) {
			continue
		}
		flagStr := attrFlagString(def.Flags)
		if flagStr == "" {
			flagStr = "-"
		}
		results = append(results, entry{name: def.Name, flags: flagStr, count: attrCounts[num]})
	}
	// Sort by name
	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})
	// Return as space-separated "name:flags:count" tuples
	parts := make([]string, len(results))
	for i, e := range results {
		parts[i] = fmt.Sprintf("%s:%s:%d", e.name, e.flags, e.count)
	}
	return strings.Join(parts, " ")
}

// AttrDefFlags returns the flag display string for a user-defined attribute.
func (g *Game) AttrDefFlags(player gamedb.DBRef, attrName string) string {
	name := strings.ToUpper(strings.TrimSpace(attrName))
	def, ok := g.DB.AttrByName[name]
	if !ok {
		return "#-1 NO SUCH ATTRIBUTE"
	}
	// Non-wizards can only see flags on VISUAL attrs
	if !Wizard(g, player) && def.Flags&gamedb.AFVisual == 0 {
		return "#-1 PERMISSION DENIED"
	}
	return attrFlagString(def.Flags)
}

// HasAttrDef returns "1" if a user-defined attribute exists, "0" otherwise.
func (g *Game) HasAttrDef(attrName string) string {
	name := strings.ToUpper(strings.TrimSpace(attrName))
	if _, ok := g.DB.AttrByName[name]; ok {
		return "1"
	}
	// Also check well-known attrs
	for _, wkName := range gamedb.WellKnownAttrs {
		if strings.EqualFold(wkName, name) {
			return "1"
		}
	}
	return "0"
}

// SetAttrDefFlags modifies flags on a user-defined attribute definition.
// Wizard-only. Returns "" on success, error string on failure.
func (g *Game) SetAttrDefFlags(player gamedb.DBRef, attrName, flags string) string {
	if !Wizard(g, player) {
		return "#-1 PERMISSION DENIED"
	}
	name := strings.ToUpper(strings.TrimSpace(attrName))
	def, ok := g.DB.AttrByName[name]
	if !ok {
		return "#-1 NO SUCH ATTRIBUTE"
	}
	setFlags, clearFlags, errs := parseAttrAccessFlags(flags)
	if len(errs) > 0 {
		return "#-1 UNKNOWN FLAG " + strings.Join(errs, " ")
	}
	def.Flags = (def.Flags &^ clearFlags) | setFlags
	if g.Store != nil {
		g.Store.PutMeta()
	}
	return ""
}

// IsWizard returns true if the player is an effective wizard.
func (g *Game) IsWizard(player gamedb.DBRef) bool {
	return Wizard(g, player)
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
