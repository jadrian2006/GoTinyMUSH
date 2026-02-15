package boltstore

import (
	"encoding/binary"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Bucket name constants for bbolt storage.
var (
	bucketMeta       = []byte("meta")
	bucketObjects    = []byte("objects")
	bucketAttrDefs   = []byte("attrdefs")
	bucketPlayers    = []byte("players")
	bucketChannels    = []byte("channels")
	bucketChanAliases = []byte("chanaliases")
	bucketStructDefs  = []byte("structdefs")
	bucketStructInsts = []byte("structinsts")
	bucketMail        = []byte("mail")
)

// Meta key constants.
var (
	keyVersion       = []byte("version")
	keyFormat        = []byte("format")
	keyFlags         = []byte("flags")
	keySize          = []byte("size")
	keyNextAttr      = []byte("nextattr")
	keyRecordPlayers = []byte("recordplayers")
)

// refToKey converts a DBRef to an 8-byte big-endian key.
// We offset by a large constant so negative DBRefs (Nothing=-1, etc.) sort correctly.
func refToKey(ref gamedb.DBRef) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(int64(ref)+1<<32))
	return buf
}

// keyToRef converts an 8-byte big-endian key back to a DBRef.
func keyToRef(b []byte) gamedb.DBRef {
	v := binary.BigEndian.Uint64(b)
	return gamedb.DBRef(int64(v) - 1<<32)
}

// intToKey converts an int to an 8-byte big-endian key.
func intToKey(n int) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(n))
	return buf
}

// keyToInt converts an 8-byte big-endian key back to an int.
func keyToInt(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}
