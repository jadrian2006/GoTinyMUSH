package server

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"github.com/crystal-mush/gotinymush/pkg/oob"
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
			// Only filter DARK wizards — C TinyMUSH does NOT filter UNFINDABLE from lwho()
			if obj.HasFlag(gamedb.FlagDark) && obj.HasFlag(gamedb.FlagWizard) {
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

// RepairAllChains rebuilds ALL Contents and Exits chains from authoritative
// Location data.  This fixes both orphans (missing from chain) and intruders
// (present in chain but Location points elsewhere).  Safe to run on startup
// and via @fixall.  Returns (containers fixed, exits fixed).
func (g *Game) RepairAllChains() (int, int) {
	// --- Phase 1: Rebuild Contents chains from Location fields ---
	// contentsOf[loc] = ordered list of non-exit dbrefs whose Location == loc
	contentsOf := make(map[gamedb.DBRef][]gamedb.DBRef)
	for ref, obj := range g.DB.Objects {
		if obj.IsGoing() || obj.Location == gamedb.Nothing {
			continue
		}
		if obj.ObjType() != gamedb.TypeExit {
			contentsOf[obj.Location] = append(contentsOf[obj.Location], ref)
		}
	}

	// Track all modified objects across all phases
	modified := make(map[gamedb.DBRef]bool)

	// --- Phase 1b: Migrate exit source fields from existing chains ---
	// FLAT-imported exits may not have Exits set to their source room.
	// Walk each room/thing's existing exit chain to discover which exits
	// belong to which container, then stamp exitObj.Exits = source.
	for ref, obj := range g.DB.Objects {
		if obj.IsGoing() || obj.Exits == gamedb.Nothing {
			continue
		}
		// Only rooms and things can own exit chains
		if obj.ObjType() == gamedb.TypeExit {
			continue
		}
		// Walk this container's exit chain
		cur := obj.Exits
		seen := make(map[gamedb.DBRef]bool)
		for cur != gamedb.Nothing && !seen[cur] {
			seen[cur] = true
			exitObj, ok := g.DB.Objects[cur]
			if !ok {
				break
			}
			if exitObj.ObjType() == gamedb.TypeExit && exitObj.Exits != ref {
				exitObj.Exits = ref
				modified[cur] = true
			}
			cur = exitObj.Next
		}
	}

	// --- Phase 2: Rebuild Exits chains from exit source field ---
	// exitsOf[source] = ordered list of exit dbrefs whose Exits == source
	exitsOf := make(map[gamedb.DBRef][]gamedb.DBRef)
	for ref, obj := range g.DB.Objects {
		if obj.IsGoing() {
			continue
		}
		if obj.ObjType() == gamedb.TypeExit && obj.Exits != gamedb.Nothing {
			exitsOf[obj.Exits] = append(exitsOf[obj.Exits], ref)
		}
	}

	// --- Phase 3: Apply rebuilt chains and detect changes ---
	containersFixed := 0

	// All containers that might have contents or exits
	containers := make(map[gamedb.DBRef]bool)
	for loc := range contentsOf {
		containers[loc] = true
	}
	for src := range exitsOf {
		containers[src] = true
	}
	// Also include any non-exit container whose existing chain is non-empty.
	// Exit objects use Exits to store their source room, NOT a chain head,
	// so they must never be treated as containers here.
	for _, obj := range g.DB.Objects {
		if obj.ObjType() == gamedb.TypeExit {
			continue
		}
		if obj.Contents != gamedb.Nothing || obj.Exits != gamedb.Nothing {
			containers[obj.DBRef] = true
		}
	}

	for cRef := range containers {
		cObj, ok := g.DB.Objects[cRef]
		if !ok || cObj.ObjType() == gamedb.TypeExit {
			continue
		}
		changed := false

		// Rebuild Contents chain
		members := contentsOf[cRef]
		newHead := gamedb.Nothing
		if len(members) > 0 {
			newHead = members[0]
		}
		if cObj.Contents != newHead {
			cObj.Contents = newHead
			changed = true
		}
		for i, ref := range members {
			obj := g.DB.Objects[ref]
			newNext := gamedb.Nothing
			if i < len(members)-1 {
				newNext = members[i+1]
			}
			if obj.Next != newNext {
				obj.Next = newNext
				modified[ref] = true
			}
		}

		// Rebuild Exits chain
		exitMembers := exitsOf[cRef]
		newExitHead := gamedb.Nothing
		if len(exitMembers) > 0 {
			newExitHead = exitMembers[0]
		}
		if cObj.Exits != newExitHead {
			cObj.Exits = newExitHead
			changed = true
		}
		for i, ref := range exitMembers {
			obj := g.DB.Objects[ref]
			newNext := gamedb.Nothing
			if i < len(exitMembers)-1 {
				newNext = exitMembers[i+1]
			}
			if obj.Next != newNext {
				obj.Next = newNext
				modified[ref] = true
			}
		}

		if changed {
			modified[cRef] = true
			containersFixed++
		}
	}

	// Persist all changed objects
	if len(modified) > 0 {
		var batch []*gamedb.Object
		for ref := range modified {
			if obj, ok := g.DB.Objects[ref]; ok {
				batch = append(batch, obj)
			}
		}
		g.PersistObjects(batch...)
		log.Printf("[REPAIR] Rebuilt chains: %d containers touched, %d objects updated",
			containersFixed, len(modified))
	}

	return containersFixed, len(modified)
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
	g.AddToContents(dest, victim)
	persistList := []*gamedb.Object{obj}
	if destObj, ok := g.DB.Objects[dest]; ok {
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
	// Try exact match first (name then alias)
	for _, obj := range g.DB.Objects {
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() && strings.EqualFold(obj.Name, name) {
			return obj.DBRef
		}
	}
	for _, obj := range g.DB.Objects {
		if obj.ObjType() != gamedb.TypePlayer || obj.IsGoing() {
			continue
		}
		for _, attr := range obj.Attrs {
			if attr.Number == 58 { // A_ALIAS
				aliasStr := eval.StripAttrPrefix(attr.Value)
				if aliasStr != "" {
					for _, a := range strings.Split(aliasStr, ";") {
						if strings.EqualFold(strings.TrimSpace(a), name) {
							return obj.DBRef
						}
					}
				}
				break
			}
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

// EvalObjLock evaluates the lock on thing against player without wizard bypass.
// Only Pass_Locks power bypasses. Matches C's fun_elock behavior.
func (g *Game) EvalObjLock(player, thing gamedb.DBRef, lockAttr int) bool {
	// Only Pass_Locks bypasses (matches C's fun_elock)
	if PassLocks(g, player) {
		return true
	}

	// Check attribute-stored lock
	lockText := g.GetAttrText(thing, lockAttr)
	if lockText != "" {
		parsed := ParseBoolExp(g, player, lockText)
		return EvalBoolExp(g, player, thing, thing, parsed, 0)
	}

	// Check header-based lock
	if lockAttr == aLock {
		if tObj, ok := g.DB.Objects[thing]; ok && tObj.Lock != nil {
			return EvalBoolExp(g, player, thing, thing, tObj.Lock, 0)
		}
	}

	// No lock = unlocked
	return true
}

// GetAttrTextGS returns the text of an attribute on an object (with parent walk).
func (g *Game) GetAttrTextGS(obj gamedb.DBRef, attrNum int) string {
	return g.GetAttrText(obj, attrNum)
}

// GetObjLockStr returns the serialized default lock (obj.Lock BoolExp) for an object.
// Returns "" if no header lock is set. Used as fallback when attr 42 is empty.
func (g *Game) GetObjLockStr(obj gamedb.DBRef) string {
	o, ok := g.DB.Objects[obj]
	if !ok || o.Lock == nil {
		return ""
	}
	return gamedb.SerializeBoolExp(o.Lock)
}

// UnparseLockStr parses a serialized lock string and returns a human-readable
// representation using F_FUNCTION format: players shown as *Name, else #dbref.
func (g *Game) UnparseLockStr(player gamedb.DBRef, lockStr string) string {
	if lockStr == "" {
		return ""
	}
	parsed := ParseBoolExp(g, player, lockStr)
	if parsed == nil {
		return lockStr
	}
	return UnparseBoolExpFunction(g, parsed)
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
	case "man", "mushman":
		hf = g.HelpMan
	case "wiznews":
		hf = g.HelpWizNews
	case "jhelp", "+jhelp":
		hf = g.HelpJobs
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

// PersistArray saves or deletes an array in bbolt.
func (g *Game) PersistArray(player gamedb.DBRef, name string, arr *gamedb.ArrayData) {
	if g.Store == nil {
		return
	}
	if arr == nil {
		g.Store.DeleteArray(player, name)
	} else {
		g.Store.PutArray(player, name, arr)
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
		if ch.Flags&gamedb.ChanSpoof != 0 {
			flags = append(flags, "Spoof")
		}
		if ch.Flags&gamedb.ChanPJoin != 0 {
			flags = append(flags, "P_Join")
		}
		if ch.Flags&gamedb.ChanPTrans != 0 {
			flags = append(flags, "P_Trans")
		}
		if ch.Flags&gamedb.ChanPRecv != 0 {
			flags = append(flags, "P_Recv")
		}
		if ch.Flags&gamedb.ChanOJoin != 0 {
			flags = append(flags, "O_Join")
		}
		if ch.Flags&gamedb.ChanOTrans != 0 {
			flags = append(flags, "O_Trans")
		}
		if ch.Flags&gamedb.ChanORecv != 0 {
			flags = append(flags, "O_Recv")
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

// SendOOB sends a GMCP message to a connected player's client(s).
// Format: IAC SB 201 <pkg> <space> <data> IAC SE
// Returns true if at least one GMCP-capable descriptor received it.
func (g *Game) SendOOB(player gamedb.DBRef, pkg string, data string) bool {
	descs := g.Conns.GetByPlayer(player)
	if len(descs) == 0 {
		return false
	}
	payload := fmt.Sprintf("%s %s", pkg, data)
	buf := make([]byte, 0, len(payload)+4)
	buf = append(buf, oob.IAC, oob.SB, oob.TeloptGMCP)
	buf = append(buf, []byte(payload)...)
	buf = append(buf, oob.IAC, oob.SE)

	sent := false
	for _, d := range descs {
		if d.OOB != nil && d.OOB.GMCP {
			d.SendRaw(buf)
			sent = true
		}
	}
	return sent
}

// HasGMCP returns true if player has at least one GMCP-capable connection.
func (g *Game) HasGMCP(player gamedb.DBRef) bool {
	descs := g.Conns.GetByPlayer(player)
	for _, d := range descs {
		if d.OOB != nil && d.OOB.GMCP {
			return true
		}
	}
	return false
}

// GMCPPackages returns the GMCP package subscriptions for a player.
func (g *Game) GMCPPackages(player gamedb.DBRef) []string {
	descs := g.Conns.GetByPlayer(player)
	for _, d := range descs {
		if d.OOB != nil && d.OOB.GMCP && d.OOB.GMCPPackages != nil {
			var pkgs []string
			for pkg := range d.OOB.GMCPPackages {
				pkgs = append(pkgs, pkg)
			}
			sort.Strings(pkgs)
			return pkgs
		}
	}
	return nil
}

// HasMSDP returns true if player has at least one MSDP-capable connection.
func (g *Game) HasMSDP(player gamedb.DBRef) bool {
	descs := g.Conns.GetByPlayer(player)
	for _, d := range descs {
		if d.OOB != nil && d.OOB.MSDP {
			return true
		}
	}
	return false
}

// HelpSearch searches help file entry contents for a pattern.
// Returns matching topic names (case-insensitive substring match).
func (g *Game) HelpSearch(player gamedb.DBRef, fileID string, pattern string) []string {
	var hf *HelpFile
	switch strings.ToLower(fileID) {
	case "help":
		hf = g.HelpMain
	case "wizhelp":
		if !Wizard(g, player) {
			return nil
		}
		hf = g.HelpWiz
	case "news":
		hf = g.HelpNews
	case "wiznews":
		if !Wizard(g, player) {
			return nil
		}
		hf = g.HelpWizNews
	case "qhelp":
		hf = g.HelpQuick
	case "plushelp", "+help":
		hf = g.HelpPlus
	case "man", "mushman":
		hf = g.HelpMan
	default:
		return nil
	}
	if hf == nil {
		return nil
	}

	patLower := strings.ToLower(pattern)
	seen := make(map[string]bool)
	var results []string
	for topic, content := range hf.Entries {
		if strings.Contains(strings.ToLower(content), patLower) {
			if !seen[topic] {
				seen[topic] = true
				results = append(results, topic)
			}
		}
	}
	sort.Strings(results)
	return results
}

// HasAPIKey returns true if the object has an API key set.
func (g *Game) HasAPIKey(obj gamedb.DBRef) bool {
	if g.Store == nil {
		return false
	}
	return g.Store.HasAPIKey(obj)
}

// ConnLog returns connection log timestamps for a player.
func (g *Game) ConnLog(player gamedb.DBRef, count int) string {
	if g.Store == nil {
		return ""
	}
	entries, err := g.Store.GetConnLog(player, count)
	if err != nil || len(entries) == 0 {
		return ""
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = fmt.Sprintf("%d", e.ConnectAt)
	}
	return strings.Join(parts, " ")
}

// NextDBRef returns the next available dbref that would be assigned.
func (g *Game) NextDBRef() gamedb.DBRef {
	return g.NextRef
}

// IsInstance returns true if obj has the Flag3Instance flag.
func (g *Game) IsInstance(obj gamedb.DBRef) bool {
	o, ok := g.DB.Objects[obj]
	if !ok {
		return false
	}
	return o.HasFlag3(gamedb.Flag3Instance)
}

// InstanceRooms returns the interior rooms of an instance (rooms whose Location = obj).
func (g *Game) InstanceRooms(obj gamedb.DBRef) []gamedb.DBRef {
	if !g.IsInstance(obj) {
		return nil
	}
	var rooms []gamedb.DBRef
	for ref, o := range g.DB.Objects {
		if o.ObjType() == gamedb.TypeRoom && o.Location == obj {
			rooms = append(rooms, ref)
		}
	}
	return rooms
}

// InstanceVehicle returns the instance THING this room belongs to, or Nothing.
func (g *Game) InstanceVehicle(room gamedb.DBRef) gamedb.DBRef {
	o, ok := g.DB.Objects[room]
	if !ok || o.ObjType() != gamedb.TypeRoom {
		return gamedb.Nothing
	}
	if o.Location == gamedb.Nothing {
		return gamedb.Nothing
	}
	loc, ok := g.DB.Objects[o.Location]
	if !ok {
		return gamedb.Nothing
	}
	if loc.HasFlag3(gamedb.Flag3Instance) {
		return o.Location
	}
	return gamedb.Nothing
}

// ChannelList returns visible channel names for player (public, owned, or comm_all).
func (g *Game) ChannelList(player gamedb.DBRef) []string {
	if g.Comsys == nil {
		return nil
	}
	hasCommAll := false
	if pObj, ok := g.DB.Objects[player]; ok {
		hasCommAll = pObj.HasPower(0, gamedb.PowCommAll) || Wizard(g, player)
	}
	var result []string
	for _, ch := range g.Comsys.AllChannels() {
		if ch.Flags&gamedb.ChanPublic != 0 || hasCommAll || ch.Owner == player {
			result = append(result, ch.Name)
		}
	}
	return result
}

// ChannelWho returns connected, listening players on a channel.
// Respects Hidden flag (only See_Hidden/wizards see hidden players).
func (g *Game) ChannelWho(player gamedb.DBRef, channelName string) []gamedb.DBRef {
	if g.Comsys == nil {
		return nil
	}
	ch := g.Comsys.GetChannel(channelName)
	if ch == nil {
		return nil // signals channel not found
	}
	canSeeHidden := Wizard(g, player)
	if !canSeeHidden {
		if pObj, ok := g.DB.Objects[player]; ok {
			canSeeHidden = pObj.HasPower(0, gamedb.PowSeeHidden)
		}
	}
	listeners := g.Comsys.ChannelListeners(channelName)
	seen := make(map[gamedb.DBRef]bool)
	var result []gamedb.DBRef
	for _, ca := range listeners {
		if seen[ca.Player] {
			continue
		}
		seen[ca.Player] = true
		if !g.Conns.IsConnected(ca.Player) {
			continue
		}
		// Hidden player check
		if pObj, ok := g.DB.Objects[ca.Player]; ok {
			if pObj.HasFlag(gamedb.FlagDark) && !canSeeHidden {
				continue
			}
		}
		result = append(result, ca.Player)
	}
	return result
}

// ChannelWhoAll returns all subscribers on a channel (connected or not).
func (g *Game) ChannelWhoAll(player gamedb.DBRef, channelName string) []gamedb.DBRef {
	if g.Comsys == nil {
		return nil
	}
	ch := g.Comsys.GetChannel(channelName)
	if ch == nil {
		return nil
	}
	subs := g.Comsys.ChannelSubscribers(channelName)
	seen := make(map[gamedb.DBRef]bool)
	var result []gamedb.DBRef
	for _, ca := range subs {
		if seen[ca.Player] {
			continue
		}
		seen[ca.Player] = true
		result = append(result, ca.Player)
	}
	_ = ch
	return result
}

// ChannelOwner returns the owner dbref of a channel, or Nothing.
func (g *Game) ChannelOwner(channelName string) gamedb.DBRef {
	if g.Comsys == nil {
		return gamedb.Nothing
	}
	ch := g.Comsys.GetChannel(channelName)
	if ch == nil {
		return gamedb.Nothing
	}
	return ch.Owner
}

// ChannelDesc returns the description of a channel.
func (g *Game) ChannelDesc(channelName string) string {
	if g.Comsys == nil {
		return ""
	}
	ch := g.Comsys.GetChannel(channelName)
	if ch == nil {
		return ""
	}
	return ch.Description
}

// ChannelHeader returns the header of a channel.
func (g *Game) ChannelHeader(channelName string) string {
	if g.Comsys == nil {
		return ""
	}
	ch := g.Comsys.GetChannel(channelName)
	if ch == nil {
		return ""
	}
	return ch.Header
}

// PlayerComAliases returns space-separated alias names for a player.
// Requires Controls or Comm_All (C: Comsys_User macro).
func (g *Game) PlayerComAliases(player, target gamedb.DBRef) string {
	if g.Comsys == nil {
		return "#-1 CHANNEL SYSTEM DISABLED"
	}
	if !g.canComsysUser(player, target) {
		return "#-1 NO PERMISSION TO USE"
	}
	aliases := g.Comsys.PlayerAliases(target)
	parts := make([]string, len(aliases))
	for i, a := range aliases {
		parts[i] = a.Alias
	}
	return strings.Join(parts, " ")
}

// PlayerComInfo returns the channel name for a player's alias.
// Requires Controls or Comm_All.
func (g *Game) PlayerComInfo(player, target gamedb.DBRef, alias string) string {
	if g.Comsys == nil {
		return "#-1 CHANNEL SYSTEM DISABLED"
	}
	if !g.canComsysUser(player, target) {
		return "#-1 NO PERMISSION TO USE"
	}
	ca := g.Comsys.LookupAlias(target, alias)
	if ca == nil {
		return "#-1 ALIAS NOT FOUND"
	}
	return ca.Channel
}

// PlayerComTitle returns the title for a player's alias.
// Requires Controls or Comm_All.
func (g *Game) PlayerComTitle(player, target gamedb.DBRef, alias string) string {
	if g.Comsys == nil {
		return "#-1 CHANNEL SYSTEM DISABLED"
	}
	if !g.canComsysUser(player, target) {
		return "#-1 NO PERMISSION TO USE"
	}
	ca := g.Comsys.LookupAlias(target, alias)
	if ca == nil {
		return "#-1 ALIAS NOT FOUND"
	}
	return ca.Title
}

// ChannelEmit sends a message to a channel. Returns "" on success, error on failure.
func (g *Game) ChannelEmit(player gamedb.DBRef, channelName, message string) string {
	if g.Comsys == nil {
		return "#-1 CHANNEL SYSTEM DISABLED"
	}
	ch := g.Comsys.GetChannel(channelName)
	if ch == nil {
		return "#-1 CHANNEL NOT FOUND"
	}
	header := ch.Header
	if header == "" {
		header = fmt.Sprintf("[%s]", ch.Name)
	}
	msg := fmt.Sprintf("%s %s", header, message)
	ch.NumSent++
	g.SendToChannel(channelName, player, msg)
	return ""
}

// canComsysUser checks the C Comsys_User macro equivalent:
// Controls(player, target) || Comm_All(player)
func (g *Game) canComsysUser(player, target gamedb.DBRef) bool {
	if Controls(g, player, target) {
		return true
	}
	if pObj, ok := g.DB.Objects[player]; ok {
		return pObj.HasPower(0, gamedb.PowCommAll) || Wizard(g, player)
	}
	return false
}

// AddrLog returns connection log IP addresses for a player.
func (g *Game) AddrLog(player gamedb.DBRef, count int) string {
	if g.Store == nil {
		return ""
	}
	entries, err := g.Store.GetConnLog(player, count)
	if err != nil || len(entries) == 0 {
		return ""
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = e.Addr
	}
	return strings.Join(parts, " ")
}
