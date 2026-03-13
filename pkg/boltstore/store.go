package boltstore

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	bbolt "go.etcd.io/bbolt"
)

// Store wraps a bbolt database and an in-memory cache for ACID persistence.
type Store struct {
	bolt  *bbolt.DB
	cache *gamedb.Database
}

// Open opens or creates a bbolt database file and ensures all buckets exist.
func Open(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("boltstore: open %s: %w", path, err)
	}

	// Ensure all buckets exist.
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{bucketMeta, bucketObjects, bucketAttrDefs, bucketPlayers, bucketChannels, bucketChanAliases, bucketStructDefs, bucketStructInsts, bucketMail, bucketArrays, bucketAPIKeys, bucketConnLog, bucketHooks, bucketEventQueues} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("boltstore: create buckets: %w", err)
	}

	return &Store{
		bolt:  db,
		cache: gamedb.NewDatabase(),
	}, nil
}

// Close closes the underlying bbolt database.
func (s *Store) Close() error {
	if s.bolt != nil {
		return s.bolt.Close()
	}
	return nil
}

// DB returns the in-memory database cache.
func (s *Store) DB() *gamedb.Database {
	return s.cache
}

// Path returns the filesystem path of the underlying bbolt database.
func (s *Store) Path() string {
	if s.bolt != nil {
		return s.bolt.Path()
	}
	return ""
}

// PutObject persists a single object to bbolt (write-through).
func (s *Store) PutObject(obj *gamedb.Object) error {
	data, err := encodeObject(obj)
	if err != nil {
		return fmt.Errorf("boltstore: encode object #%d: %w", obj.DBRef, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketObjects).Put(refToKey(obj.DBRef), data)
	})
}

// PutObjects persists multiple objects in a single bbolt transaction.
func (s *Store) PutObjects(objs ...*gamedb.Object) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketObjects)
		for _, obj := range objs {
			if obj == nil {
				continue
			}
			data, err := encodeObject(obj)
			if err != nil {
				return fmt.Errorf("boltstore: encode object #%d: %w", obj.DBRef, err)
			}
			if err := b.Put(refToKey(obj.DBRef), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteObject removes an object from bbolt.
func (s *Store) DeleteObject(ref gamedb.DBRef) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketObjects).Delete(refToKey(ref))
	})
}

// PutAttrDef persists an attribute definition.
func (s *Store) PutAttrDef(def *gamedb.AttrDef) error {
	data, err := encodeAttrDef(def)
	if err != nil {
		return fmt.Errorf("boltstore: encode attrdef %d: %w", def.Number, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketAttrDefs).Put(intToKey(def.Number), data)
	})
}

// PutMeta persists database metadata (version, nextattr, size, etc.).
func (s *Store) PutMeta() error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		b.Put(keyVersion, intToKey(s.cache.Version))
		b.Put(keyFormat, intToKey(s.cache.Format))
		b.Put(keyFlags, intToKey(s.cache.Flags))
		b.Put(keySize, intToKey(s.cache.Size))
		b.Put(keyNextAttr, intToKey(s.cache.NextAttr))
		b.Put(keyRecordPlayers, intToKey(s.cache.RecordPlayers))
		return nil
	})
}

// ImportFromDatabase bulk-loads an in-memory Database into bbolt, batching 1000 objects per transaction.
func (s *Store) ImportFromDatabase(db *gamedb.Database) error {
	// Copy the database pointer as our cache.
	s.cache = db

	// Persist metadata.
	if err := s.PutMeta(); err != nil {
		return fmt.Errorf("boltstore: import meta: %w", err)
	}

	// Persist attribute definitions.
	err := s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAttrDefs)
		for _, def := range db.AttrNames {
			data, err := encodeAttrDef(def)
			if err != nil {
				return err
			}
			if err := b.Put(intToKey(def.Number), data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("boltstore: import attrdefs: %w", err)
	}

	// Persist objects in batches of 1000.
	batch := make([]*gamedb.Object, 0, 1000)
	count := 0
	for _, obj := range db.Objects {
		batch = append(batch, obj)
		if len(batch) >= 1000 {
			if err := s.writeBatch(batch); err != nil {
				return err
			}
			count += len(batch)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if err := s.writeBatch(batch); err != nil {
			return err
		}
		count += len(batch)
	}

	// Build player name index.
	if err := s.rebuildPlayerIndex(db); err != nil {
		return fmt.Errorf("boltstore: import player index: %w", err)
	}

	log.Printf("boltstore: imported %d objects, %d attr defs", count, len(db.AttrNames))
	return nil
}

// writeBatch writes a batch of objects in a single transaction.
func (s *Store) writeBatch(objs []*gamedb.Object) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketObjects)
		for _, obj := range objs {
			data, err := encodeObject(obj)
			if err != nil {
				return fmt.Errorf("encode #%d: %w", obj.DBRef, err)
			}
			if err := b.Put(refToKey(obj.DBRef), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// rebuildPlayerIndex writes all player name→DBRef mappings.
func (s *Store) rebuildPlayerIndex(db *gamedb.Database) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketPlayers)
		for _, obj := range db.Objects {
			if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() {
				name := strings.ToLower(obj.Name)
				if err := b.Put([]byte(name), refToKey(obj.DBRef)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// LoadAll reads the entire bbolt database into the in-memory cache.
func (s *Store) LoadAll() error {
	// Load metadata.
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		if v := b.Get(keyVersion); v != nil {
			s.cache.Version = keyToInt(v)
		}
		if v := b.Get(keyFormat); v != nil {
			s.cache.Format = keyToInt(v)
		}
		if v := b.Get(keyFlags); v != nil {
			s.cache.Flags = keyToInt(v)
		}
		if v := b.Get(keySize); v != nil {
			s.cache.Size = keyToInt(v)
		}
		if v := b.Get(keyNextAttr); v != nil {
			s.cache.NextAttr = keyToInt(v)
		}
		if v := b.Get(keyRecordPlayers); v != nil {
			s.cache.RecordPlayers = keyToInt(v)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("boltstore: load meta: %w", err)
	}

	// Load attribute definitions.
	// Skip any that collide with well-known (built-in) attrs by name —
	// the well-known table is authoritative for those attr numbers.
	var staleAttrKeys [][]byte
	err = s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAttrDefs)
		return b.ForEach(func(k, v []byte) error {
			def, err := decodeAttrDef(v)
			if err != nil {
				return fmt.Errorf("decode attrdef: %w", err)
			}
			if wkNum, _, ok := gamedb.ResolveWellKnownAttr(def.Name); ok && wkNum != def.Number {
				// This bbolt def has a different number than the well-known attr.
				// Skip it and mark for cleanup.
				log.Printf("boltstore: skipping stale attr def %q (bbolt #%d vs well-known #%d)", def.Name, def.Number, wkNum)
				staleAttrKeys = append(staleAttrKeys, append([]byte{}, k...))
				return nil
			}
			s.cache.AttrNames[def.Number] = def
			s.cache.AttrByName[strings.ToUpper(def.Name)] = def
			return nil
		})
	})
	// Clean up stale attr defs from bbolt.
	if len(staleAttrKeys) > 0 {
		_ = s.bolt.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketAttrDefs)
			for _, k := range staleAttrKeys {
				_ = b.Delete(k)
			}
			return nil
		})
	}
	if err != nil {
		return fmt.Errorf("boltstore: load attrdefs: %w", err)
	}

	// Load objects.
	count := 0
	err = s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketObjects)
		return b.ForEach(func(k, v []byte) error {
			obj, err := decodeObject(v)
			if err != nil {
				return fmt.Errorf("decode object: %w", err)
			}
			s.cache.Objects[obj.DBRef] = obj
			count++
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("boltstore: load objects: %w", err)
	}

	log.Printf("boltstore: loaded %d objects, %d attr defs from bolt", count, len(s.cache.AttrNames))
	return nil
}

// Backup creates a hot snapshot of the bbolt database using tx.WriteTo().
func (s *Store) Backup(path string) error {
	return s.bolt.View(func(tx *bbolt.Tx) error {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("boltstore: create backup %s: %w", path, err)
		}
		defer f.Close()
		_, err = tx.WriteTo(f)
		if err != nil {
			return fmt.Errorf("boltstore: write backup: %w", err)
		}
		log.Printf("boltstore: backup written to %s", path)
		return nil
	})
}

// UpdatePlayerIndex updates the player name→DBRef secondary index.
// If oldName is non-empty, the old entry is removed.
func (s *Store) UpdatePlayerIndex(obj *gamedb.Object, oldName string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketPlayers)
		if oldName != "" {
			b.Delete([]byte(strings.ToLower(oldName)))
		}
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() {
			return b.Put([]byte(strings.ToLower(obj.Name)), refToKey(obj.DBRef))
		}
		return nil
	})
}

// HasData returns true if the bbolt database contains any objects.
func (s *Store) HasData() bool {
	hasData := false
	s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketObjects)
		if b.Stats().KeyN > 0 {
			hasData = true
		}
		return nil
	})
	return hasData
}

// --- Comsys (Channel System) Storage ---

// PutChannel persists a channel to bbolt, keyed by lowercase name.
func (s *Store) PutChannel(ch *gamedb.Channel) error {
	data, err := encodeChannel(ch)
	if err != nil {
		return fmt.Errorf("boltstore: encode channel %q: %w", ch.Name, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketChannels).Put([]byte(strings.ToLower(ch.Name)), data)
	})
}

// DeleteChannel removes a channel from bbolt.
func (s *Store) DeleteChannel(name string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketChannels).Delete([]byte(strings.ToLower(name)))
	})
}

// LoadChannels reads all channels from bbolt.
func (s *Store) LoadChannels() ([]gamedb.Channel, error) {
	var channels []gamedb.Channel
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		return b.ForEach(func(k, v []byte) error {
			ch, err := decodeChannel(v)
			if err != nil {
				return fmt.Errorf("decode channel %q: %w", string(k), err)
			}
			channels = append(channels, *ch)
			return nil
		})
	})
	return channels, err
}

// chanAliasKey returns the bbolt key for a channel alias: "playerRef:alias".
func chanAliasKey(player gamedb.DBRef, alias string) []byte {
	return []byte(fmt.Sprintf("%d:%s", player, strings.ToLower(alias)))
}

// PutChanAlias persists a channel alias to bbolt.
func (s *Store) PutChanAlias(ca *gamedb.ChanAlias) error {
	data, err := encodeChanAlias(ca)
	if err != nil {
		return fmt.Errorf("boltstore: encode chan alias: %w", err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketChanAliases).Put(chanAliasKey(ca.Player, ca.Alias), data)
	})
}

// DeleteChanAlias removes a channel alias from bbolt.
func (s *Store) DeleteChanAlias(player gamedb.DBRef, alias string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketChanAliases).Delete(chanAliasKey(player, alias))
	})
}

// DeleteChanAliasesForPlayer removes all channel aliases for a player from bbolt.
func (s *Store) DeleteChanAliasesForPlayer(player gamedb.DBRef) error {
	prefix := []byte(fmt.Sprintf("%d:", player))
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChanAliases)
		c := b.Cursor()
		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// LoadChanAliases reads all channel aliases from bbolt.
func (s *Store) LoadChanAliases() ([]gamedb.ChanAlias, error) {
	var aliases []gamedb.ChanAlias
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChanAliases)
		return b.ForEach(func(k, v []byte) error {
			ca, err := decodeChanAlias(v)
			if err != nil {
				return fmt.Errorf("decode chan alias %q: %w", string(k), err)
			}
			aliases = append(aliases, *ca)
			return nil
		})
	})
	return aliases, err
}

// ImportComsys bulk-loads channels and aliases into bbolt.
func (s *Store) ImportComsys(channels []gamedb.Channel, aliases []gamedb.ChanAlias) error {
	err := s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		for i := range channels {
			data, err := encodeChannel(&channels[i])
			if err != nil {
				return err
			}
			if err := b.Put([]byte(strings.ToLower(channels[i].Name)), data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("boltstore: import channels: %w", err)
	}

	// Batch aliases in groups of 1000
	for i := 0; i < len(aliases); i += 1000 {
		end := i + 1000
		if end > len(aliases) {
			end = len(aliases)
		}
		batch := aliases[i:end]
		err := s.bolt.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketChanAliases)
			for j := range batch {
				data, err := encodeChanAlias(&batch[j])
				if err != nil {
					return err
				}
				if err := b.Put(chanAliasKey(batch[j].Player, batch[j].Alias), data); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("boltstore: import chan aliases: %w", err)
		}
	}

	log.Printf("boltstore: imported %d channels, %d channel aliases", len(channels), len(aliases))
	return nil
}

// HasComsysData returns true if there are any channels stored in bbolt.
func (s *Store) HasComsysData() bool {
	has := false
	s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		if b.Stats().KeyN > 0 {
			has = true
		}
		return nil
	})
	return has
}

// --- Structure/Instance Persistence ---

// structKey returns "playerRef:name" key for structure storage.
func structKey(player gamedb.DBRef, name string) []byte {
	return []byte(fmt.Sprintf("%d:%s", player, strings.ToLower(name)))
}

// PutStructDef persists a structure definition to bbolt.
func (s *Store) PutStructDef(player gamedb.DBRef, def *gamedb.StructDef) error {
	data, err := encodeStructDef(def)
	if err != nil {
		return fmt.Errorf("boltstore: encode struct def %q: %w", def.Name, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketStructDefs).Put(structKey(player, def.Name), data)
	})
}

// DeleteStructDef removes a structure definition from bbolt.
func (s *Store) DeleteStructDef(player gamedb.DBRef, name string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketStructDefs).Delete(structKey(player, name))
	})
}

// PutStructInstance persists a structure instance to bbolt.
func (s *Store) PutStructInstance(player gamedb.DBRef, name string, inst *gamedb.StructInstance) error {
	data, err := encodeStructInst(inst)
	if err != nil {
		return fmt.Errorf("boltstore: encode struct instance %q: %w", name, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketStructInsts).Put(structKey(player, name), data)
	})
}

// DeleteStructInstance removes a structure instance from bbolt.
func (s *Store) DeleteStructInstance(player gamedb.DBRef, name string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketStructInsts).Delete(structKey(player, name))
	})
}

// LoadStructDefs reads all structure definitions from bbolt.
func (s *Store) LoadStructDefs() (map[gamedb.DBRef]map[string]*gamedb.StructDef, error) {
	result := make(map[gamedb.DBRef]map[string]*gamedb.StructDef)
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStructDefs)
		return b.ForEach(func(k, v []byte) error {
			def, err := decodeStructDef(v)
			if err != nil {
				return fmt.Errorf("decode struct def %q: %w", string(k), err)
			}
			// Parse key "playerRef:name"
			parts := strings.SplitN(string(k), ":", 2)
			if len(parts) != 2 {
				return nil
			}
			var ref int
			fmt.Sscanf(parts[0], "%d", &ref)
			player := gamedb.DBRef(ref)
			if result[player] == nil {
				result[player] = make(map[string]*gamedb.StructDef)
			}
			result[player][def.Name] = def
			return nil
		})
	})
	return result, err
}

// LoadStructInstances reads all structure instances from bbolt.
func (s *Store) LoadStructInstances() (map[gamedb.DBRef]map[string]*gamedb.StructInstance, error) {
	result := make(map[gamedb.DBRef]map[string]*gamedb.StructInstance)
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStructInsts)
		return b.ForEach(func(k, v []byte) error {
			inst, err := decodeStructInst(v)
			if err != nil {
				return fmt.Errorf("decode struct instance %q: %w", string(k), err)
			}
			parts := strings.SplitN(string(k), ":", 2)
			if len(parts) != 2 {
				return nil
			}
			var ref int
			fmt.Sscanf(parts[0], "%d", &ref)
			player := gamedb.DBRef(ref)
			if result[player] == nil {
				result[player] = make(map[string]*gamedb.StructInstance)
			}
			result[player][parts[1]] = inst
			return nil
		})
	})
	return result, err
}

// HasStructData returns true if there are any structure definitions stored in bbolt.
func (s *Store) HasStructData() bool {
	has := false
	s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStructDefs)
		if b.Stats().KeyN > 0 {
			has = true
		}
		return nil
	})
	return has
}

// --- Array Persistence ---

// arrayKey returns "playerRef:name" key for array storage.
func arrayKey(player gamedb.DBRef, name string) []byte {
	return []byte(fmt.Sprintf("%d:%s", player, strings.ToLower(name)))
}

// PutArray persists an array to bbolt.
func (s *Store) PutArray(player gamedb.DBRef, name string, arr *gamedb.ArrayData) error {
	data, err := encodeArray(arr)
	if err != nil {
		return fmt.Errorf("boltstore: encode array %q: %w", name, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketArrays).Put(arrayKey(player, name), data)
	})
}

// DeleteArray removes an array from bbolt.
func (s *Store) DeleteArray(player gamedb.DBRef, name string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketArrays).Delete(arrayKey(player, name))
	})
}

// LoadArrays reads all arrays from bbolt.
func (s *Store) LoadArrays() (map[gamedb.DBRef]map[string]*gamedb.ArrayData, error) {
	result := make(map[gamedb.DBRef]map[string]*gamedb.ArrayData)
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketArrays)
		return b.ForEach(func(k, v []byte) error {
			arr, err := decodeArray(v)
			if err != nil {
				return fmt.Errorf("decode array %q: %w", string(k), err)
			}
			parts := strings.SplitN(string(k), ":", 2)
			if len(parts) != 2 {
				return nil
			}
			var ref int
			fmt.Sscanf(parts[0], "%d", &ref)
			player := gamedb.DBRef(ref)
			if result[player] == nil {
				result[player] = make(map[string]*gamedb.ArrayData)
			}
			result[player][parts[1]] = arr
			return nil
		})
	})
	return result, err
}

// HasArrayData returns true if there are any arrays stored in bbolt.
func (s *Store) HasArrayData() bool {
	has := false
	s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketArrays)
		if b.Stats().KeyN > 0 {
			has = true
		}
		return nil
	})
	return has
}

// --- API Key Storage ---

// PutAPIKey stores a SHA-256 hash of an API key for an object.
func (s *Store) PutAPIKey(ref gamedb.DBRef, hash string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketAPIKeys).Put(refToKey(ref), []byte(hash))
	})
}

// GetAPIKey returns the stored SHA-256 hash for an object's API key.
// Returns "" if no key exists.
func (s *Store) GetAPIKey(ref gamedb.DBRef) (string, error) {
	var hash string
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketAPIKeys).Get(refToKey(ref))
		if v != nil {
			hash = string(v)
		}
		return nil
	})
	return hash, err
}

// DeleteAPIKey removes the API key for an object.
func (s *Store) DeleteAPIKey(ref gamedb.DBRef) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketAPIKeys).Delete(refToKey(ref))
	})
}

// HasAPIKey returns true if an object has an API key stored.
func (s *Store) HasAPIKey(ref gamedb.DBRef) bool {
	has := false
	s.bolt.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketAPIKeys).Get(refToKey(ref))
		has = v != nil
		return nil
	})
	return has
}

// --- Connection Log Storage ---

// ConnLogEntry records a single connection event.
type ConnLogEntry struct {
	Player    gamedb.DBRef
	Addr      string
	ConnectAt int64 // Unix timestamp
	DisconnAt int64 // 0 if still connected
}

// connLogKey builds a composite key: refToKey(player) + bigEndian(timestamp).
func connLogKey(player gamedb.DBRef, timestamp int64) []byte {
	key := make([]byte, 16)
	copy(key[:8], refToKey(player))
	binary.BigEndian.PutUint64(key[8:], uint64(timestamp))
	return key
}

// AppendConnLog appends a connection log entry and prunes to maxEntries per player.
func (s *Store) AppendConnLog(entry ConnLogEntry, maxEntries int) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConnLog)

		// Encode the entry
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(entry); err != nil {
			return fmt.Errorf("encode connlog: %w", err)
		}

		key := connLogKey(entry.Player, entry.ConnectAt)
		if err := b.Put(key, buf.Bytes()); err != nil {
			return err
		}

		// Prune old entries beyond maxEntries
		prefix := refToKey(entry.Player)
		c := b.Cursor()
		// Count entries for this player
		var keys [][]byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			keys = append(keys, keyCopy)
		}
		// Delete oldest entries beyond maxEntries
		if len(keys) > maxEntries {
			for _, k := range keys[:len(keys)-maxEntries] {
				b.Delete(k)
			}
		}
		return nil
	})
}

// UpdateConnLogDisconnect updates the most recent connlog entry for a player
// with a disconnect timestamp.
func (s *Store) UpdateConnLogDisconnect(player gamedb.DBRef, disconnAt int64) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConnLog)
		prefix := refToKey(player)
		c := b.Cursor()

		// Find the last entry for this player (cursor to end of prefix range)
		var lastKey []byte
		var lastVal []byte
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			lastKey = make([]byte, len(k))
			copy(lastKey, k)
			lastVal = make([]byte, len(v))
			copy(lastVal, v)
		}
		if lastKey == nil {
			return nil // no entries
		}

		// Decode, update, re-encode
		var entry ConnLogEntry
		if err := gob.NewDecoder(bytes.NewReader(lastVal)).Decode(&entry); err != nil {
			return err
		}
		if entry.DisconnAt != 0 {
			return nil // already disconnected
		}
		entry.DisconnAt = disconnAt
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(entry); err != nil {
			return err
		}
		return b.Put(lastKey, buf.Bytes())
	})
}

// GetConnLog retrieves the most recent count connection log entries for a player.
// Returns entries in reverse chronological order (newest first).
func (s *Store) GetConnLog(player gamedb.DBRef, count int) ([]ConnLogEntry, error) {
	var entries []ConnLogEntry
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConnLog)
		prefix := refToKey(player)
		c := b.Cursor()

		// Collect all entries for this player
		var all [][]byte
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			valCopy := make([]byte, len(v))
			copy(valCopy, v)
			all = append(all, valCopy)
		}

		// Take last 'count' entries in reverse order
		start := 0
		if len(all) > count {
			start = len(all) - count
		}
		for i := len(all) - 1; i >= start; i-- {
			var entry ConnLogEntry
			if err := gob.NewDecoder(bytes.NewReader(all[i])).Decode(&entry); err != nil {
				continue
			}
			entries = append(entries, entry)
		}
		return nil
	})
	return entries, err
}

// HookSet holds hook attribute names for a command.
type HookSet struct {
	Before   string
	After    string
	Override string
	Ignore   string
}

// PutHooks persists the entire hooks map to bbolt.
func (s *Store) PutHooks(hooks map[string]*HookSet) {
	if hooks == nil {
		return
	}
	s.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHooks)
		// Clear existing
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			b.Delete(k)
		}
		// Write new
		for name, hs := range hooks {
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(hs); err != nil {
				continue
			}
			b.Put([]byte(name), buf.Bytes())
		}
		return nil
	})
}

// GetHooks loads all hooks from bbolt.
func (s *Store) GetHooks() map[string]*HookSet {
	hooks := make(map[string]*HookSet)
	s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHooks)
		if b == nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			var hs HookSet
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&hs); err != nil {
				return nil
			}
			hooks[string(k)] = &hs
			return nil
		})
		return nil
	})
	return hooks
}

// --- Event Queue persistence ---

// PutEventQueue persists an event queue definition to bbolt.
func (s *Store) PutEventQueue(q *StoredEventQueue) error {
	data, err := encodeEventQueue(q)
	if err != nil {
		return fmt.Errorf("boltstore: encode event queue %q: %w", q.Name, err)
	}
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketEventQueues).Put([]byte(strings.ToLower(q.Name)), data)
	})
}

// DeleteEventQueue removes an event queue from bbolt.
func (s *Store) DeleteEventQueue(name string) error {
	return s.bolt.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketEventQueues).Delete([]byte(strings.ToLower(name)))
	})
}

// LoadEventQueues reads all event queues from bbolt.
func (s *Store) LoadEventQueues() ([]StoredEventQueue, error) {
	var queues []StoredEventQueue
	err := s.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEventQueues)
		return b.ForEach(func(k, v []byte) error {
			q, err := decodeEventQueue(v)
			if err != nil {
				return fmt.Errorf("decode event queue %q: %w", string(k), err)
			}
			queues = append(queues, *q)
			return nil
		})
	})
	return queues, err
}
