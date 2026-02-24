package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/flatfile"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Merge two FLAT databases:
// - Structural fields from the BACKUP (correct chain data)
// - Attributes from the CURRENT (has conformat fixes and other game changes)
// - Objects only in CURRENT are included as-is (new objects)
// - Attr definitions from CURRENT (superset of backup)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: flatmerge <backup.FLAT> <current.FLAT> <output.FLAT>\n")
		os.Exit(1)
	}

	backupPath := os.Args[1]
	currentPath := os.Args[2]
	outputPath := os.Args[3]

	log.Printf("Parsing backup: %s", backupPath)
	f1, err := os.Open(backupPath)
	if err != nil {
		log.Fatal(err)
	}
	backup, err := flatfile.Parse(f1)
	f1.Close()
	if err != nil {
		log.Fatalf("Backup parse error: %v", err)
	}
	log.Printf("Backup: %d objects, %d attr defs", len(backup.Objects), len(backup.AttrNames))

	log.Printf("Parsing current: %s", currentPath)
	f2, err := os.Open(currentPath)
	if err != nil {
		log.Fatal(err)
	}
	current, err := flatfile.Parse(f2)
	f2.Close()
	if err != nil {
		log.Fatalf("Current parse error: %v", err)
	}
	log.Printf("Current: %d objects, %d attr defs", len(current.Objects), len(current.AttrNames))

	// Merge: backup structure + current attributes
	merged := gamedb.NewDatabase()

	// Use current's attr definitions (superset)
	for num, def := range current.AttrNames {
		merged.AddAttrDef(num, def.Name, def.Flags)
	}
	for num, def := range backup.AttrNames {
		if _, exists := merged.AttrNames[num]; !exists {
			merged.AddAttrDef(num, def.Name, def.Flags)
		}
	}

	merged.Size = current.Size
	if current.NextAttr > backup.NextAttr {
		merged.NextAttr = current.NextAttr
	} else {
		merged.NextAttr = backup.NextAttr
	}
	merged.Flags = current.Flags
	merged.Version = current.Version
	merged.Format = current.Format

	// Merge objects
	allRefs := make(map[gamedb.DBRef]bool)
	for ref := range backup.Objects {
		allRefs[ref] = true
	}
	for ref := range current.Objects {
		allRefs[ref] = true
	}

	fromBackup, fromCurrent, mergedCount := 0, 0, 0
	for ref := range allRefs {
		bObj, inBackup := backup.Objects[ref]
		cObj, inCurrent := current.Objects[ref]

		if inBackup && inCurrent {
			obj := &gamedb.Object{
				DBRef:      ref,
				Name:       bObj.Name,
				Location:   bObj.Location,
				Zone:       bObj.Zone,
				Contents:   bObj.Contents,
				Exits:      bObj.Exits,
				Link:       bObj.Link,
				Next:       bObj.Next,
				Owner:      bObj.Owner,
				Parent:     bObj.Parent,
				Pennies:    bObj.Pennies,
				Lock:       bObj.Lock,
				LastAccess: bObj.LastAccess,
				LastMod:    bObj.LastMod,
			}
			obj.Flags = bObj.Flags
			obj.Powers = bObj.Powers
			obj.Attrs = cObj.Attrs
			merged.Objects[ref] = obj
			mergedCount++
		} else if inCurrent {
			merged.Objects[ref] = cObj
			fromCurrent++
		} else {
			merged.Objects[ref] = bObj
			fromBackup++
		}
	}

	log.Printf("Merged: %d objects (%d merged, %d backup-only, %d current-only)",
		len(merged.Objects), mergedCount, fromBackup, fromCurrent)

	// Write output FLAT
	out, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	w := bufio.NewWriter(out)

	fmt.Fprintf(w, "+T%d\n", current.Flags|current.Version)
	fmt.Fprintf(w, "+S%d\n", merged.Size)
	fmt.Fprintf(w, "+N%d\n", merged.NextAttr)

	var attrNums []int
	for num := range merged.AttrNames {
		attrNums = append(attrNums, num)
	}
	sort.Ints(attrNums)
	for _, num := range attrNums {
		def := merged.AttrNames[num]
		fmt.Fprintf(w, "+A%d\n%d:%s\n", num, def.Flags, def.Name)
	}

	var refs []int
	for ref := range merged.Objects {
		refs = append(refs, int(ref))
	}
	sort.Ints(refs)

	for _, ref := range refs {
		obj := merged.Objects[gamedb.DBRef(ref)]
		fmt.Fprintf(w, "!%d\n", ref)
		fmt.Fprintf(w, "%s\n", obj.Name)
		fmt.Fprintf(w, "%d\n", obj.Location)
		fmt.Fprintf(w, "%d\n", obj.Zone)
		fmt.Fprintf(w, "%d\n", obj.Contents)
		fmt.Fprintf(w, "%d\n", obj.Exits)
		fmt.Fprintf(w, "%d\n", obj.Link)
		fmt.Fprintf(w, "%d\n", obj.Next)
		fmt.Fprintf(w, "%d\n", obj.Owner)
		fmt.Fprintf(w, "%d\n", obj.Parent)
		fmt.Fprintf(w, "%d\n", obj.Pennies)
		fmt.Fprintf(w, "%d\n", obj.Flags[0])
		fmt.Fprintf(w, "%d\n", obj.Flags[1])
		fmt.Fprintf(w, "%d\n", obj.Flags[2])
		fmt.Fprintf(w, "%d\n", obj.Powers[0])
		fmt.Fprintf(w, "%d\n", obj.Powers[1])
		fmt.Fprintf(w, "%d\n", obj.LastAccess.Unix())
		fmt.Fprintf(w, "%d\n", obj.LastMod.Unix())

		for _, a := range obj.Attrs {
			val := a.Value
			// New FLAT format (unquoted): readLine() reads as-is, so only
			// escape characters that would break line-based parsing.
			// Do NOT escape backslashes or quotes — they are literal in
			// the unquoted format.
			val = strings.ReplaceAll(val, "\n", "\\n")
			val = strings.ReplaceAll(val, "\r", "\\r")
			fmt.Fprintf(w, ">%d\n%s\n", a.Number, val)
		}
		fmt.Fprintf(w, "<\n")
	}

	fmt.Fprintf(w, "***END OF DUMP***\n")
	w.Flush()
	out.Close()

	log.Printf("Written golden FLAT: %s", outputPath)

	// Verify by re-parsing
	vf, _ := os.Open(outputPath)
	vdb, err := flatfile.Parse(vf)
	vf.Close()
	if err != nil {
		log.Fatalf("Verification FAILED: %v", err)
	}
	log.Printf("Verified: %d objects, %d attr defs", len(vdb.Objects), len(vdb.AttrNames))

	if obj, ok := vdb.Objects[1814]; ok {
		log.Printf("Exit #1814: source=%d (want 2915), dest=%d, next=%d", obj.Exits, obj.Location, obj.Next)
	}
	exitCount := 0
	for _, obj := range vdb.Objects {
		if obj.ObjType() == gamedb.TypeExit && obj.Exits == 2915 {
			exitCount++
		}
	}
	log.Printf("Exits sourced from #2915: %d (want 5)", exitCount)
}
