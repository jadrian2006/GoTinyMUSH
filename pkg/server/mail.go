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

// SendMessage delivers a message to all recipients (To + CC).
// Returns the created messages keyed by recipient.
func (m *Mail) SendMessage(from gamedb.DBRef, to, cc []gamedb.DBRef, subject, body string) map[gamedb.DBRef]*gamedb.MailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	result := make(map[gamedb.DBRef]*gamedb.MailMessage)

	allRecipients := make([]gamedb.DBRef, 0, len(to)+len(cc))
	allRecipients = append(allRecipients, to...)
	allRecipients = append(allRecipients, cc...)

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
			Subject: subject,
			Body:    body,
			Time:    now,
			Flags:   0,
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
