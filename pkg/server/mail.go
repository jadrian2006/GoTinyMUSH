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
	Expire   int                                          // days before auto-expire, 0 = never
}

// NewMail creates an empty mail manager.
func NewMail(expireDays int) *Mail {
	return &Mail{
		Messages: make(map[gamedb.DBRef]map[int]*gamedb.MailMessage),
		NextID:   make(map[gamedb.DBRef]int),
		Drafts:   make(map[gamedb.DBRef]*MailDraft),
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
