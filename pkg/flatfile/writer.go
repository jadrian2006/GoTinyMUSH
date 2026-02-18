package flatfile

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Write writes the database to the given writer in TinyMUSH 3.0 flatfile format.
func Write(w io.Writer, db *gamedb.Database) error {
	wr := &writer{w: w}

	// Version header: TinyMUSH 3.0 format with all standard flags
	versionFlags := VZone | VLink | VAtrName | VAtrKey | VParent | VXFlags | V3Flags | VPowers | VQuoted | VTimestamps
	version := 1 | versionFlags // version 1 with flags
	wr.writef("+T%d\n", version)

	// Size
	size := 0
	for ref := range db.Objects {
		if int(ref) >= size {
			size = int(ref) + 1
		}
	}
	wr.writef("+S%d\n", size)

	// Next attribute number
	wr.writef("+N%d\n", db.NextAttr)

	// Attribute definitions (sorted by number for consistency)
	var attrNums []int
	for num := range db.AttrNames {
		attrNums = append(attrNums, num)
	}
	sort.Ints(attrNums)
	for _, num := range attrNums {
		def := db.AttrNames[num]
		wr.writef("+A%d\n%d:%s\n", num, def.Flags, def.Name)
	}

	// Record players count
	playerCount := 0
	for _, obj := range db.Objects {
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() {
			playerCount++
		}
	}
	wr.writef("-R%d\n", playerCount)

	// Objects (sorted by dbref for consistency)
	var refs []int
	for ref := range db.Objects {
		refs = append(refs, int(ref))
	}
	sort.Ints(refs)

	for _, ref := range refs {
		obj := db.Objects[gamedb.DBRef(ref)]
		if err := wr.writeObject(obj); err != nil {
			return fmt.Errorf("writing object #%d: %w", ref, err)
		}
	}

	// End marker
	wr.writef("***END OF DUMP***\n")

	return wr.err
}

// Save writes the database to a file path.
func Save(path string, db *gamedb.Database) error {
	// Write to temp file first, then rename for atomicity
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if err := Write(f, db); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, path); err != nil {
		// On Windows, may need to remove target first
		os.Remove(path)
		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("rename temp to final: %w", err)
		}
	}

	return nil
}

type writer struct {
	w   io.Writer
	err error
}

func (wr *writer) writef(format string, args ...interface{}) {
	if wr.err != nil {
		return
	}
	_, wr.err = fmt.Fprintf(wr.w, format, args...)
}

func (wr *writer) writeObject(obj *gamedb.Object) error {
	// Object header
	wr.writef("!%d\n", obj.DBRef)

	// Name (quoted)
	wr.writef("%s\n", quoteString(obj.Name))

	// Location
	wr.writef("%d\n", obj.Location)

	// Zone
	wr.writef("%d\n", obj.Zone)

	// Contents
	wr.writef("%d\n", obj.Contents)

	// Exits
	wr.writef("%d\n", obj.Exits)

	// Link
	wr.writef("%d\n", obj.Link)

	// Next
	wr.writef("%d\n", obj.Next)

	// Owner
	wr.writef("%d\n", obj.Owner)

	// Parent
	wr.writef("%d\n", obj.Parent)

	// Pennies
	wr.writef("%d\n", obj.Pennies)

	// Flags (3 words)
	wr.writef("%d\n", obj.Flags[0])
	wr.writef("%d\n", obj.Flags[1])
	wr.writef("%d\n", obj.Flags[2])

	// Powers (2 words, one per line)
	wr.writef("%d\n", obj.Powers[0])
	wr.writef("%d\n", obj.Powers[1])

	// Timestamps
	access := obj.LastAccess.Unix()
	mod := obj.LastMod.Unix()
	if access <= 0 {
		access = time.Now().Unix()
	}
	if mod <= 0 {
		mod = time.Now().Unix()
	}
	wr.writef("%d\n", access)
	wr.writef("%d\n", mod)

	// Write lock as attribute 42 (A_LOCK) when VAtrKey is set.
	// This preserves boolean expression locks through export/import cycles.
	if obj.Lock != nil {
		lockStr := gamedb.SerializeBoolExp(obj.Lock)
		if lockStr != "" {
			wr.writef(">42\n%s\n", quoteString(lockStr))
		}
	}

	// Attributes
	for _, attr := range obj.Attrs {
		if attr.Number <= 0 {
			continue
		}
		// Skip attr 42 if we already wrote it from obj.Lock above
		if attr.Number == 42 && obj.Lock != nil {
			continue
		}
		wr.writef(">%d\n%s\n", attr.Number, quoteString(attr.Value))
	}

	// End of attributes
	wr.writef("<\n")

	return wr.err
}

// quoteString produces a quoted string with escapes for the flatfile format.
func quoteString(s string) string {
	var buf strings.Builder
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteByte(s[i])
		}
	}
	buf.WriteByte('"')
	return buf.String()
}
