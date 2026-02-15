package boltstore

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	bbolt "go.etcd.io/bbolt"
)

// mailKey returns "playerRef:msgID" key for mail storage.
func mailKey(player gamedb.DBRef, msgID int) []byte {
	return []byte(fmt.Sprintf("%d:%d", player, msgID))
}

// PutMailMessage persists a single mail message to bbolt.
func (s *Store) PutMailMessage(player gamedb.DBRef, msg *gamedb.MailMessage) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return fmt.Errorf("boltstore: encode mail msg: %w", err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketMail).Put(mailKey(player, msg.ID), buf.Bytes())
	})
}

// DeleteMailMessage removes a single mail message from bbolt.
func (s *Store) DeleteMailMessage(player gamedb.DBRef, msgID int) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketMail).Delete(mailKey(player, msgID))
	})
}

// DeleteMailMessages removes multiple mail messages in a single transaction.
func (s *Store) DeleteMailMessages(player gamedb.DBRef, msgIDs []int) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMail)
		for _, id := range msgIDs {
			if err := b.Delete(mailKey(player, id)); err != nil {
				return err
			}
		}
		return nil
	})
}

// LoadMail reads all mail messages from bbolt, grouped by recipient.
func (s *Store) LoadMail() (map[gamedb.DBRef]map[int]*gamedb.MailMessage, error) {
	result := make(map[gamedb.DBRef]map[int]*gamedb.MailMessage)
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMail)
		return b.ForEach(func(k, v []byte) error {
			var msg gamedb.MailMessage
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&msg); err != nil {
				return fmt.Errorf("decode mail %q: %w", string(k), err)
			}
			// Parse key "playerRef:msgID"
			parts := strings.SplitN(string(k), ":", 2)
			if len(parts) != 2 {
				return nil
			}
			var ref int
			fmt.Sscanf(parts[0], "%d", &ref)
			player := gamedb.DBRef(ref)
			if result[player] == nil {
				result[player] = make(map[int]*gamedb.MailMessage)
			}
			result[player][msg.ID] = &msg
			return nil
		})
	})
	return result, err
}

// HasMailData returns true if there are any mail messages stored in bbolt.
func (s *Store) HasMailData() bool {
	has := false
	s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMail)
		if b.Stats().KeyN > 0 {
			has = true
		}
		return nil
	})
	return has
}

// PutMailMessages persists multiple mail messages in a single transaction.
func (s *Store) PutMailMessages(player gamedb.DBRef, msgs []*gamedb.MailMessage) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMail)
		for _, msg := range msgs {
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
				return err
			}
			if err := b.Put(mailKey(player, msg.ID), buf.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})
}

// ImportMail bulk-loads mail messages into bbolt.
func (s *Store) ImportMail(all map[gamedb.DBRef]map[int]*gamedb.MailMessage) error {
	total := 0
	for player, msgs := range all {
		batch := make([]*gamedb.MailMessage, 0, len(msgs))
		for _, msg := range msgs {
			batch = append(batch, msg)
		}
		if err := s.PutMailMessages(player, batch); err != nil {
			return fmt.Errorf("boltstore: import mail for #%d: %w", player, err)
		}
		total += len(msgs)
	}
	log.Printf("boltstore: imported %d mail messages", total)
	return nil
}
