package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/flatfile"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func main() {
	dbPath := flag.String("db", "", "Path to TinyMUSH flatfile (e.g., game.FLAT)")
	showPlayers := flag.Bool("players", false, "List all player objects")
	showRooms := flag.Bool("rooms", false, "List room summary")
	showObj := flag.Int("obj", -1, "Show details for a specific object by dbref")
	showAttrStats := flag.Bool("attrstats", false, "Show attribute usage statistics")
	validate := flag.Bool("validate", false, "Run referential integrity checks")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: dbloader -db <path-to-flatfile> [options]")
		fmt.Fprintln(os.Stderr, "  -players      List all players")
		fmt.Fprintln(os.Stderr, "  -rooms        List rooms summary")
		fmt.Fprintln(os.Stderr, "  -obj <dbref>  Show object details")
		fmt.Fprintln(os.Stderr, "  -attrstats    Show attribute usage stats")
		fmt.Fprintln(os.Stderr, "  -validate     Run integrity checks")
		os.Exit(1)
	}

	fmt.Printf("Loading flatfile: %s\n", *dbPath)
	start := time.Now()

	db, err := flatfile.Load(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("Loaded in %v\n\n", elapsed)

	// Always print summary
	printSummary(db)

	if *showPlayers {
		fmt.Println()
		printPlayers(db)
	}

	if *showRooms {
		fmt.Println()
		printRooms(db)
	}

	if *showObj >= 0 {
		fmt.Println()
		printObject(db, gamedb.DBRef(*showObj))
	}

	if *showAttrStats {
		fmt.Println()
		printAttrStats(db)
	}

	if *validate {
		fmt.Println()
		runValidation(db)
	}
}

func printSummary(db *gamedb.Database) {
	fmt.Println("=== DATABASE SUMMARY ===")
	fmt.Printf("Format:         %d (TinyMUSH 3.0 = 6)\n", db.Format)
	fmt.Printf("Version:        %d\n", db.Version)
	fmt.Printf("Declared size:  %d objects\n", db.Size)
	fmt.Printf("Loaded objects: %d\n", len(db.Objects))
	fmt.Printf("Attr defs:      %d user-defined attributes\n", len(db.AttrNames))
	fmt.Printf("Next attr num:  %d\n", db.NextAttr)
	fmt.Printf("Record players: %d\n", db.RecordPlayers)

	// Count by type
	typeCounts := make(map[gamedb.ObjectType]int)
	totalAttrs := 0
	goingCount := 0
	for _, obj := range db.Objects {
		typeCounts[obj.ObjType()]++
		totalAttrs += len(obj.Attrs)
		if obj.IsGoing() {
			goingCount++
		}
	}

	fmt.Println("\n--- Object Counts by Type ---")
	types := []gamedb.ObjectType{
		gamedb.TypeRoom, gamedb.TypeThing, gamedb.TypeExit,
		gamedb.TypePlayer, gamedb.TypeZone, gamedb.TypeGarbage,
	}
	for _, t := range types {
		if c, ok := typeCounts[t]; ok {
			fmt.Printf("  %-10s %d\n", t.String(), c)
		}
	}
	fmt.Printf("  %-10s %d\n", "GOING", goingCount)
	fmt.Printf("\nTotal attributes across all objects: %d\n", totalAttrs)
}

func printPlayers(db *gamedb.Database) {
	fmt.Println("=== PLAYERS ===")

	type playerInfo struct {
		ref  gamedb.DBRef
		name string
		loc  gamedb.DBRef
		last time.Time
	}

	var players []playerInfo
	for _, obj := range db.Objects {
		if obj.ObjType() == gamedb.TypePlayer && !obj.IsGoing() {
			players = append(players, playerInfo{
				ref:  obj.DBRef,
				name: obj.Name,
				loc:  obj.Location,
				last: obj.LastAccess,
			})
		}
	}

	sort.Slice(players, func(i, j int) bool {
		return players[i].ref < players[j].ref
	})

	fmt.Printf("%-8s %-25s %-10s %s\n", "DBRef", "Name", "Location", "Last Access")
	fmt.Println(strings.Repeat("-", 75))
	for _, p := range players {
		lastStr := "never"
		if !p.last.IsZero() {
			lastStr = p.last.Format("2006-01-02 15:04")
		}
		fmt.Printf("#%-7d %-25s #%-9d %s\n", p.ref, truncate(p.name, 25), p.loc, lastStr)
	}
	fmt.Printf("\nTotal players: %d\n", len(players))
}

func printRooms(db *gamedb.Database) {
	fmt.Println("=== ROOMS (first 50) ===")

	type roomInfo struct {
		ref      gamedb.DBRef
		name     string
		contents int
		exits    int
	}

	var rooms []roomInfo
	for _, obj := range db.Objects {
		if obj.ObjType() == gamedb.TypeRoom && !obj.IsGoing() {
			// Count contents
			contentCount := 0
			next := obj.Contents
			for next != gamedb.Nothing {
				contentCount++
				if o, ok := db.Objects[next]; ok {
					next = o.Next
				} else {
					break
				}
				if contentCount > 10000 {
					break // safety
				}
			}
			// Count exits
			exitCount := 0
			next = obj.Exits
			for next != gamedb.Nothing {
				exitCount++
				if o, ok := db.Objects[next]; ok {
					next = o.Next
				} else {
					break
				}
				if exitCount > 10000 {
					break
				}
			}
			rooms = append(rooms, roomInfo{
				ref:      obj.DBRef,
				name:     obj.Name,
				contents: contentCount,
				exits:    exitCount,
			})
		}
	}

	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].ref < rooms[j].ref
	})

	fmt.Printf("%-8s %-40s %8s %8s\n", "DBRef", "Name", "Contents", "Exits")
	fmt.Println(strings.Repeat("-", 68))
	limit := 50
	if len(rooms) < limit {
		limit = len(rooms)
	}
	for _, r := range rooms[:limit] {
		fmt.Printf("#%-7d %-40s %8d %8d\n", r.ref, truncate(r.name, 40), r.contents, r.exits)
	}
	fmt.Printf("\nTotal rooms: %d (showing first %d)\n", len(rooms), limit)
}

func printObject(db *gamedb.Database, ref gamedb.DBRef) {
	obj, ok := db.Objects[ref]
	if !ok {
		fmt.Printf("Object #%d not found in database\n", ref)
		return
	}

	fmt.Printf("=== OBJECT #%d ===\n", ref)
	fmt.Printf("Name:       %s\n", obj.Name)
	fmt.Printf("Type:       %s\n", obj.ObjType())
	fmt.Printf("Location:   #%d\n", obj.Location)
	fmt.Printf("Zone:       #%d\n", obj.Zone)
	fmt.Printf("Contents:   #%d\n", obj.Contents)
	fmt.Printf("Exits:      #%d\n", obj.Exits)
	fmt.Printf("Link/Home:  #%d\n", obj.Link)
	fmt.Printf("Next:       #%d\n", obj.Next)
	fmt.Printf("Owner:      #%d\n", obj.Owner)
	fmt.Printf("Parent:     #%d\n", obj.Parent)
	fmt.Printf("Pennies:    %d\n", obj.Pennies)
	fmt.Printf("Flags:      0x%08x 0x%08x 0x%08x\n", obj.Flags[0], obj.Flags[1], obj.Flags[2])
	fmt.Printf("Powers:     0x%08x 0x%08x\n", obj.Powers[0], obj.Powers[1])
	if !obj.LastAccess.IsZero() {
		fmt.Printf("Last Access: %s\n", obj.LastAccess.Format(time.RFC3339))
	}
	if !obj.LastMod.IsZero() {
		fmt.Printf("Last Mod:    %s\n", obj.LastMod.Format(time.RFC3339))
	}
	fmt.Printf("Going:      %v\n", obj.IsGoing())

	// Print flag names
	fmt.Printf("Flag names: %s\n", flagNames(obj))

	fmt.Printf("\n--- Attributes (%d) ---\n", len(obj.Attrs))
	for _, attr := range obj.Attrs {
		name := db.GetAttrName(attr.Number)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", attr.Number)
		}
		val := attr.Value
		if len(val) > 120 {
			val = val[:120] + "..."
		}
		fmt.Printf("  [%d] %s = %s\n", attr.Number, name, val)
	}
}

func printAttrStats(db *gamedb.Database) {
	fmt.Println("=== ATTRIBUTE STATISTICS ===")

	// Count usage of each attribute number
	attrUsage := make(map[int]int)
	for _, obj := range db.Objects {
		for _, attr := range obj.Attrs {
			attrUsage[attr.Number]++
		}
	}

	// Sort by usage count
	type attrCount struct {
		num   int
		name  string
		count int
	}
	var counts []attrCount
	for num, count := range attrUsage {
		name := db.GetAttrName(num)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", num)
		}
		counts = append(counts, attrCount{num, name, count})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	fmt.Printf("%-8s %-30s %s\n", "AttrNum", "Name", "Usage Count")
	fmt.Println(strings.Repeat("-", 55))
	limit := 50
	if len(counts) < limit {
		limit = len(counts)
	}
	for _, c := range counts[:limit] {
		fmt.Printf("%-8d %-30s %d\n", c.num, truncate(c.name, 30), c.count)
	}
	fmt.Printf("\nTotal unique attributes in use: %d\n", len(attrUsage))
}

func runValidation(db *gamedb.Database) {
	fmt.Println("=== VALIDATION ===")
	errors := 0
	warnings := 0

	// Check that locations, contents, exits, next references are valid
	for _, obj := range db.Objects {
		if obj.IsGoing() {
			continue
		}

		ref := obj.DBRef

		// Location should exist (or be a special value: Nothing, Ambiguous, Home)
		if obj.Location != gamedb.Nothing && obj.Location != gamedb.Ambiguous && obj.Location != gamedb.Home {
			if _, ok := db.Objects[obj.Location]; !ok {
				fmt.Printf("ERROR: #%d location #%d does not exist\n", ref, obj.Location)
				errors++
			}
		}

		// Contents chain
		if obj.Contents != gamedb.Nothing {
			if _, ok := db.Objects[obj.Contents]; !ok {
				fmt.Printf("ERROR: #%d contents head #%d does not exist\n", ref, obj.Contents)
				errors++
			}
		}

		// Exits chain
		if obj.Exits != gamedb.Nothing {
			if _, ok := db.Objects[obj.Exits]; !ok {
				fmt.Printf("ERROR: #%d exits head #%d does not exist\n", ref, obj.Exits)
				errors++
			}
		}

		// Next pointer
		if obj.Next != gamedb.Nothing {
			if _, ok := db.Objects[obj.Next]; !ok {
				fmt.Printf("ERROR: #%d next #%d does not exist\n", ref, obj.Next)
				errors++
			}
		}

		// Owner should exist and be a player
		if obj.Owner != gamedb.Nothing {
			if owner, ok := db.Objects[obj.Owner]; !ok {
				fmt.Printf("ERROR: #%d owner #%d does not exist\n", ref, obj.Owner)
				errors++
			} else if owner.ObjType() != gamedb.TypePlayer {
				// This is expected for God (owner=God) but warn for others
				if obj.Owner != gamedb.DBRef(1) {
					fmt.Printf("WARN: #%d owner #%d is not a player (type=%s)\n",
						ref, obj.Owner, owner.ObjType())
					warnings++
				}
			}
		}

		// Parent should exist if set
		if obj.Parent != gamedb.Nothing {
			if _, ok := db.Objects[obj.Parent]; !ok {
				fmt.Printf("ERROR: #%d parent #%d does not exist\n", ref, obj.Parent)
				errors++
			}
		}

		// Zone should exist if set
		if obj.Zone != gamedb.Nothing {
			if _, ok := db.Objects[obj.Zone]; !ok {
				fmt.Printf("ERROR: #%d zone #%d does not exist\n", ref, obj.Zone)
				errors++
			}
		}
	}

	// Check contents chains for loops
	for _, obj := range db.Objects {
		if obj.IsGoing() || obj.Contents == gamedb.Nothing {
			continue
		}
		visited := make(map[gamedb.DBRef]bool)
		cur := obj.Contents
		for cur != gamedb.Nothing {
			if visited[cur] {
				fmt.Printf("ERROR: #%d contents chain has loop at #%d\n", obj.DBRef, cur)
				errors++
				break
			}
			visited[cur] = true
			if o, ok := db.Objects[cur]; ok {
				cur = o.Next
			} else {
				break
			}
			if len(visited) > 50000 {
				fmt.Printf("ERROR: #%d contents chain exceeds 50000 entries\n", obj.DBRef)
				errors++
				break
			}
		}
	}

	fmt.Printf("\nValidation complete: %d errors, %d warnings\n", errors, warnings)
}

func flagNames(obj *gamedb.Object) string {
	var names []string
	f1 := obj.Flags[0]

	flagMap := map[int]string{
		gamedb.FlagWizard:     "WIZARD",
		gamedb.FlagDark:       "DARK",
		gamedb.FlagHaven:      "HAVEN",
		gamedb.FlagQuiet:      "QUIET",
		gamedb.FlagHalt:       "HALT",
		gamedb.FlagGoing:      "GOING",
		gamedb.FlagMonitor:    "MONITOR",
		gamedb.FlagPuppet:     "PUPPET",
		gamedb.FlagEnterOK:    "ENTER_OK",
		gamedb.FlagVisual:     "VISUAL",
		gamedb.FlagImmortal:   "IMMORTAL",
		gamedb.FlagHasStartup: "HAS_STARTUP",
		gamedb.FlagOpaque:     "OPAQUE",
		gamedb.FlagVerbose:    "VERBOSE",
		gamedb.FlagInherit:    "INHERIT",
		gamedb.FlagNoSpoof:    "NOSPOOF",
		gamedb.FlagRobot:      "ROBOT",
		gamedb.FlagSafe:       "SAFE",
		gamedb.FlagRoyalty:    "ROYALTY",
		gamedb.FlagTerse:      "TERSE",
		gamedb.FlagSticky:     "STICKY",
		gamedb.FlagLinkOK:     "LINK_OK",
		gamedb.FlagJumpOK:     "JUMP_OK",
		gamedb.FlagSeeThru:    "TRANSPARENT",
		gamedb.FlagHearThru:   "HEAR_THRU",
	}

	for flag, name := range flagMap {
		if f1&flag != 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, " ")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
