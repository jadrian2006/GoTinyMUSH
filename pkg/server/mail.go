package server

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// MailDraft holds a message being composed via @mail/to, @mail/subject, and "- <text>".
type MailDraft struct {
	To      []gamedb.DBRef
	CC      []gamedb.DBRef
	BCC     []gamedb.DBRef
	Subject string
	Body    strings.Builder
}

// Mail manages the in-memory mail store.
type Mail struct {
	mu       sync.RWMutex
	Messages map[gamedb.DBRef]map[int]*gamedb.MailMessage // recipient -> msgID -> message
	NextID   map[gamedb.DBRef]int                         // next ID per player
	Drafts   map[gamedb.DBRef]*MailDraft                  // in-memory only
	Aliases  map[string]*gamedb.MailAlias                  // "ownerRef:lowerName" -> alias
	Expire   int                                          // days before auto-expire, 0 = never
}

// NewMail creates an empty mail manager.
func NewMail(expireDays int) *Mail {
	return &Mail{
		Messages: make(map[gamedb.DBRef]map[int]*gamedb.MailMessage),
		NextID:   make(map[gamedb.DBRef]int),
		Drafts:   make(map[gamedb.DBRef]*MailDraft),
		Aliases:  make(map[string]*gamedb.MailAlias),
		Expire:   expireDays,
	}
}

// LoadMessages populates the mail store from persisted data.
func (m *Mail) LoadMessages(all map[gamedb.DBRef]map[int]*gamedb.MailMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = all
	// Compute NextID for each player
	for player, msgs := range all {
		maxID := 0
		for id := range msgs {
			if id > maxID {
				maxID = id
			}
		}
		m.NextID[player] = maxID + 1
	}
}

// SendMessage delivers a message to all recipients (To + CC + BCC).
// Returns the created messages keyed by recipient.
func (m *Mail) SendMessage(from gamedb.DBRef, to, cc, bcc []gamedb.DBRef, subject, body string, flags int) map[gamedb.DBRef]*gamedb.MailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	result := make(map[gamedb.DBRef]*gamedb.MailMessage)

	allRecipients := make([]gamedb.DBRef, 0, len(to)+len(cc)+len(bcc))
	allRecipients = append(allRecipients, to...)
	allRecipients = append(allRecipients, cc...)
	allRecipients = append(allRecipients, bcc...)

	// Deduplicate
	seen := make(map[gamedb.DBRef]bool)
	for _, r := range allRecipients {
		if seen[r] {
			continue
		}
		seen[r] = true

		if m.Messages[r] == nil {
			m.Messages[r] = make(map[int]*gamedb.MailMessage)
		}
		id := m.NextID[r]
		if id == 0 {
			id = 1
		}
		m.NextID[r] = id + 1

		msg := &gamedb.MailMessage{
			ID:      id,
			From:    from,
			To:      to,
			CC:      cc,
			BCC:     bcc,
			Subject: subject,
			Body:    body,
			Time:    now,
			Flags:   flags,
			Folder:  0,
		}
		m.Messages[r][id] = msg
		result[r] = msg
	}
	return result
}

// GetMessage returns a message by recipient and ID.
func (m *Mail) GetMessage(player gamedb.DBRef, msgID int) *gamedb.MailMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if msgs, ok := m.Messages[player]; ok {
		return msgs[msgID]
	}
	return nil
}

// GetInbox returns all messages for a player, sorted by ID.
func (m *Mail) GetInbox(player gamedb.DBRef) []*gamedb.MailMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs, ok := m.Messages[player]
	if !ok {
		return nil
	}
	result := make([]*gamedb.MailMessage, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, msg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// MarkRead sets the read flag on a message.
func (m *Mail) MarkRead(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Flags |= gamedb.MailIsRead
	return true
}

// MarkCleared sets the cleared flag on a message.
func (m *Mail) MarkCleared(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	if msg.Flags&gamedb.MailSafe != 0 {
		return false
	}
	msg.Flags |= gamedb.MailCleared
	return true
}

// MarkUncleared removes the cleared flag on a message.
func (m *Mail) MarkUncleared(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Flags &^= gamedb.MailCleared
	return true
}

// MarkSafe sets the safe flag on a message.
func (m *Mail) MarkSafe(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Flags |= gamedb.MailSafe
	msg.Flags &^= gamedb.MailCleared // safe implies uncleared
	return true
}

// PurgeCleared removes all cleared messages for a player, returns their IDs.
func (m *Mail) PurgeCleared(player gamedb.DBRef) []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs, ok := m.Messages[player]
	if !ok {
		return nil
	}
	var purged []int
	for id, msg := range msgs {
		if msg.Flags&gamedb.MailCleared != 0 {
			purged = append(purged, id)
			delete(msgs, id)
		}
	}
	return purged
}

// CountMessages returns (total, unread, cleared) for a player.
func (m *Mail) CountMessages(player gamedb.DBRef) (total, unread, cleared int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs, ok := m.Messages[player]
	if !ok {
		return 0, 0, 0
	}
	for _, msg := range msgs {
		total++
		if msg.Flags&gamedb.MailIsRead == 0 {
			unread++
		}
		if msg.Flags&gamedb.MailCleared != 0 {
			cleared++
		}
	}
	return
}

// GetDraft returns the current draft for a player, creating one if needed.
func (m *Mail) GetDraft(player gamedb.DBRef) *MailDraft {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.Drafts[player]; ok {
		return d
	}
	d := &MailDraft{}
	m.Drafts[player] = d
	return d
}

// HasDraft returns true if the player has an active draft.
func (m *Mail) HasDraft(player gamedb.DBRef) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.Drafts[player]
	return ok
}

// ClearDraft removes the current draft for a player.
func (m *Mail) ClearDraft(player gamedb.DBRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Drafts, player)
}

// ExpireOld removes messages older than the configured expiration.
// Returns a map of player -> purged message IDs.
func (m *Mail) ExpireOld() map[gamedb.DBRef][]int {
	if m.Expire <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().AddDate(0, 0, -m.Expire)
	result := make(map[gamedb.DBRef][]int)
	for player, msgs := range m.Messages {
		for id, msg := range msgs {
			if msg.Flags&gamedb.MailSafe != 0 {
				continue
			}
			if msg.Time.Before(cutoff) {
				result[player] = append(result[player], id)
				delete(msgs, id)
			}
		}
	}
	return result
}

// MarkTag sets the tag flag on a message.
func (m *Mail) MarkTag(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Flags |= gamedb.MailTag
	return true
}

// MarkUntag clears the tag flag on a message.
func (m *Mail) MarkUntag(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Flags &^= gamedb.MailTag
	return true
}

// MarkUrgent sets the urgent flag on a message.
func (m *Mail) MarkUrgent(player gamedb.DBRef, msgID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Flags |= gamedb.MailUrgent
	return true
}

// FileMessage moves a message to a folder (0-14).
func (m *Mail) FileMessage(player gamedb.DBRef, msgID, folder int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.getMessage(player, msgID)
	if msg == nil {
		return false
	}
	msg.Folder = folder
	return true
}

// GetInboxFolder returns messages in a specific folder for a player, sorted by ID.
func (m *Mail) GetInboxFolder(player gamedb.DBRef, folder int) []*gamedb.MailMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs, ok := m.Messages[player]
	if !ok {
		return nil
	}
	var result []*gamedb.MailMessage
	for _, msg := range msgs {
		if msg.Folder == folder {
			result = append(result, msg)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// NukePlayer removes all mail for a specific player. Returns purged IDs.
func (m *Mail) NukePlayer(player gamedb.DBRef) []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs, ok := m.Messages[player]
	if !ok {
		return nil
	}
	var ids []int
	for id := range msgs {
		ids = append(ids, id)
	}
	delete(m.Messages, player)
	delete(m.NextID, player)
	return ids
}

// NukeAll removes ALL mail from the system. Returns total count.
func (m *Mail) NukeAll() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, msgs := range m.Messages {
		total += len(msgs)
	}
	m.Messages = make(map[gamedb.DBRef]map[int]*gamedb.MailMessage)
	m.NextID = make(map[gamedb.DBRef]int)
	return total
}

// RetractMessage removes an unread message that was sent by 'from' to 'target'.
// Only works on unread messages. Returns true if retracted.
func (m *Mail) RetractMessage(from, target gamedb.DBRef, msgID int) (retracted bool, wasRead bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs, ok := m.Messages[target]
	if !ok {
		return false, false
	}
	msg, ok := msgs[msgID]
	if !ok || msg.From != from {
		return false, false
	}
	if msg.Flags&gamedb.MailIsRead != 0 {
		return false, true
	}
	delete(msgs, msgID)
	return true, false
}

// ReviewSent returns all messages sent by 'from' across all recipients' mailboxes.
// If target != Nothing, only returns messages to that specific player.
func (m *Mail) ReviewSent(from, target gamedb.DBRef) []*gamedb.MailMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*gamedb.MailMessage
	seen := make(map[string]bool) // dedup by subject+time
	for player, msgs := range m.Messages {
		if target != gamedb.Nothing && player != target {
			continue
		}
		for _, msg := range msgs {
			if msg.From != from {
				continue
			}
			key := fmt.Sprintf("%s|%d", msg.Subject, msg.Time.Unix())
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, msg)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.Before(result[j].Time)
	})
	return result
}

// CountFolderMessages returns per-folder message counts for a player.
func (m *Mail) CountFolderMessages(player gamedb.DBRef) map[int]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := make(map[int]int)
	msgs, ok := m.Messages[player]
	if !ok {
		return counts
	}
	for _, msg := range msgs {
		counts[msg.Folder]++
	}
	return counts
}

// DetailedStats returns (total, unread, cleared, tagged, urgent, safe) for a player.
func (m *Mail) DetailedStats(player gamedb.DBRef) (total, unread, cleared, tagged, urgent, safe int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs, ok := m.Messages[player]
	if !ok {
		return
	}
	for _, msg := range msgs {
		total++
		if msg.Flags&gamedb.MailIsRead == 0 {
			unread++
		}
		if msg.Flags&gamedb.MailCleared != 0 {
			cleared++
		}
		if msg.Flags&gamedb.MailTag != 0 {
			tagged++
		}
		if msg.Flags&gamedb.MailUrgent != 0 {
			urgent++
		}
		if msg.Flags&gamedb.MailSafe != 0 {
			safe++
		}
	}
	return
}

// FormatFlags returns a flag string for display (e.g., "UR" for unread+replied).
func FormatMailFlags(msg *gamedb.MailMessage) string {
	var flags []byte
	if msg.Flags&gamedb.MailIsRead == 0 {
		flags = append(flags, 'N') // New/unread
	}
	if msg.Flags&gamedb.MailCleared != 0 {
		flags = append(flags, 'C')
	}
	if msg.Flags&gamedb.MailUrgent != 0 {
		flags = append(flags, 'U')
	}
	if msg.Flags&gamedb.MailSafe != 0 {
		flags = append(flags, 'S')
	}
	if msg.Flags&gamedb.MailTag != 0 {
		flags = append(flags, 'T')
	}
	if msg.Flags&gamedb.MailForward != 0 {
		flags = append(flags, 'F')
	}
	if msg.Flags&gamedb.MailReply != 0 {
		flags = append(flags, 'R')
	}
	if len(flags) == 0 {
		return "-"
	}
	return string(flags)
}

// getMessage is an internal unlocked accessor.
func (m *Mail) getMessage(player gamedb.DBRef, msgID int) *gamedb.MailMessage {
	if msgs, ok := m.Messages[player]; ok {
		return msgs[msgID]
	}
	return nil
}

// FormatRecipients returns a display string of player names for a recipient list.
func FormatRecipients(db *gamedb.Database, refs []gamedb.DBRef) string {
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		if obj, ok := db.Objects[r]; ok {
			names = append(names, obj.Name)
		} else {
			names = append(names, fmt.Sprintf("#%d", r))
		}
	}
	return strings.Join(names, ", ")
}

// --- Mail Alias Methods ---

// aliasKey returns the composite key for an alias: "ownerRef:lowerName".
func aliasKey(owner gamedb.DBRef, name string) string {
	return fmt.Sprintf("%d:%s", owner, strings.ToLower(name))
}

// LoadAliases populates the alias store from persisted data.
func (m *Mail) LoadAliases(aliases []*gamedb.MailAlias) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range aliases {
		m.Aliases[aliasKey(a.Owner, a.Name)] = a
	}
}

// GetAlias returns an alias visible to player: own alias first, then God-owned.
// name should NOT include the * prefix.
func (m *Mail) GetAlias(player gamedb.DBRef, name string) *gamedb.MailAlias {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lower := strings.ToLower(name)
	// Check player-owned first
	if a, ok := m.Aliases[aliasKey(player, lower)]; ok {
		return a
	}
	// Check God-owned (dbref 1)
	if a, ok := m.Aliases[aliasKey(1, lower)]; ok {
		return a
	}
	return nil
}

// GetAliasExact returns an alias by exact owner, for admin operations.
func (m *Mail) GetAliasExact(owner gamedb.DBRef, name string) *gamedb.MailAlias {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Aliases[aliasKey(owner, name)]
}

// CreateAlias creates a new mail alias. Returns nil if it already exists.
func (m *Mail) CreateAlias(owner gamedb.DBRef, name string, members []gamedb.DBRef) *gamedb.MailAlias {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := aliasKey(owner, name)
	if _, exists := m.Aliases[key]; exists {
		return nil
	}
	a := &gamedb.MailAlias{
		Owner:   owner,
		Name:    name,
		Members: members,
	}
	m.Aliases[key] = a
	return a
}

// DeleteAlias removes an alias. Returns true if found.
func (m *Mail) DeleteAlias(owner gamedb.DBRef, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := aliasKey(owner, name)
	if _, ok := m.Aliases[key]; !ok {
		return false
	}
	delete(m.Aliases, key)
	return true
}

// RenameAlias changes an alias name. Returns false if old doesn't exist or new already exists.
func (m *Mail) RenameAlias(owner gamedb.DBRef, oldName, newName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldKey := aliasKey(owner, oldName)
	newKey := aliasKey(owner, newName)
	a, ok := m.Aliases[oldKey]
	if !ok {
		return false
	}
	if _, exists := m.Aliases[newKey]; exists {
		return false
	}
	delete(m.Aliases, oldKey)
	a.Name = newName
	m.Aliases[newKey] = a
	return true
}

// ChownAlias changes the owner of an alias. Returns false on conflict.
func (m *Mail) ChownAlias(oldOwner gamedb.DBRef, name string, newOwner gamedb.DBRef) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldKey := aliasKey(oldOwner, name)
	newKey := aliasKey(newOwner, name)
	a, ok := m.Aliases[oldKey]
	if !ok {
		return false
	}
	if _, exists := m.Aliases[newKey]; exists {
		return false
	}
	delete(m.Aliases, oldKey)
	a.Owner = newOwner
	m.Aliases[newKey] = a
	return true
}

// SetAliasDesc sets the description of an alias.
func (m *Mail) SetAliasDesc(owner gamedb.DBRef, name, desc string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.Aliases[aliasKey(owner, name)]
	if !ok {
		return false
	}
	a.Desc = desc
	return true
}

// AddAliasMember adds a player to an alias. Returns false if full or duplicate.
func (m *Mail) AddAliasMember(owner gamedb.DBRef, name string, player gamedb.DBRef) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.Aliases[aliasKey(owner, name)]
	if !ok {
		return false
	}
	if len(a.Members) >= gamedb.MaxAliasMembers {
		return false
	}
	for _, p := range a.Members {
		if p == player {
			return false
		}
	}
	a.Members = append(a.Members, player)
	return true
}

// RemoveAliasMember removes a player from an alias. Returns false if not found.
func (m *Mail) RemoveAliasMember(owner gamedb.DBRef, name string, player gamedb.DBRef) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.Aliases[aliasKey(owner, name)]
	if !ok {
		return false
	}
	for i, p := range a.Members {
		if p == player {
			a.Members = append(a.Members[:i], a.Members[i+1:]...)
			return true
		}
	}
	return false
}

// ListAliases returns all aliases visible to a player (own + God-owned).
func (m *Mail) ListAliases(player gamedb.DBRef) []*gamedb.MailAlias {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*gamedb.MailAlias
	for _, a := range m.Aliases {
		if a.Owner == player || a.Owner == 1 {
			result = append(result, a)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

// AllAliases returns every alias in the system (admin use).
func (m *Mail) AllAliases() []*gamedb.MailAlias {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*gamedb.MailAlias, 0, len(m.Aliases))
	for _, a := range m.Aliases {
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Owner != result[j].Owner {
			return result[i].Owner < result[j].Owner
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

// CountAliases returns the total number of aliases.
func (m *Mail) CountAliases() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.Aliases)
}
