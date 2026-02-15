package validate

import (
	"fmt"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// IntegrityChecker performs referential integrity checks on the database.
// Ported from cmd/dbloader/main.go runValidation().
type IntegrityChecker struct{}

func (c *IntegrityChecker) Name() string { return "integrity" }

func (c *IntegrityChecker) Check(db *gamedb.Database) []Finding {
	var findings []Finding
	seq := 0

	mkID := func() string {
		id := fmt.Sprintf("integrity-%d", seq)
		seq++
		return id
	}

	// Check that locations, contents, exits, next references are valid
	for _, obj := range db.Objects {
		if obj.IsGoing() {
			continue
		}
		ref := obj.DBRef

		// Location should exist (or be a special value)
		if obj.Location != gamedb.Nothing && obj.Location != gamedb.Ambiguous && obj.Location != gamedb.Home {
			if _, ok := db.Objects[obj.Location]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d location #%d does not exist", ref, obj.Location),
				})
			}
		}

		// Contents chain
		if obj.Contents != gamedb.Nothing {
			if _, ok := db.Objects[obj.Contents]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d contents head #%d does not exist", ref, obj.Contents),
				})
			}
		}

		// Exits chain
		if obj.Exits != gamedb.Nothing {
			if _, ok := db.Objects[obj.Exits]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d exits head #%d does not exist", ref, obj.Exits),
				})
			}
		}

		// Next pointer
		if obj.Next != gamedb.Nothing {
			if _, ok := db.Objects[obj.Next]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d next #%d does not exist", ref, obj.Next),
				})
			}
		}

		// Owner should exist and be a player
		if obj.Owner != gamedb.Nothing {
			if owner, ok := db.Objects[obj.Owner]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d owner #%d does not exist", ref, obj.Owner),
				})
			} else if owner.ObjType() != gamedb.TypePlayer {
				// Expected for God (#1) — warn for others
				if obj.Owner != gamedb.DBRef(1) {
					findings = append(findings, Finding{
						ID:          mkID(),
						Category:    CatIntegrityWarn,
						Severity:    SevWarning,
						ObjectRef:   ref,
						Description: fmt.Sprintf("#%d owner #%d is not a player (type=%s)", ref, obj.Owner, owner.ObjType()),
					})
				}
			}
		}

		// Parent should exist if set
		if obj.Parent != gamedb.Nothing {
			if _, ok := db.Objects[obj.Parent]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d parent #%d does not exist", ref, obj.Parent),
				})
			}
		}

		// Zone should exist if set
		if obj.Zone != gamedb.Nothing {
			if _, ok := db.Objects[obj.Zone]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d zone #%d does not exist", ref, obj.Zone),
				})
			}
		}

		// Link should exist if set (for exits, this is the destination)
		if obj.Link != gamedb.Nothing && obj.Link != gamedb.Home {
			if _, ok := db.Objects[obj.Link]; !ok {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   ref,
					Description: fmt.Sprintf("#%d link #%d does not exist", ref, obj.Link),
				})
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
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   obj.DBRef,
					Description: fmt.Sprintf("#%d contents chain has loop at #%d", obj.DBRef, cur),
				})
				break
			}
			visited[cur] = true
			if o, ok := db.Objects[cur]; ok {
				cur = o.Next
			} else {
				break
			}
			if len(visited) > 50000 {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   obj.DBRef,
					Description: fmt.Sprintf("#%d contents chain exceeds 50000 entries", obj.DBRef),
				})
				break
			}
		}
	}

	// Check exit chains for loops
	for _, obj := range db.Objects {
		if obj.IsGoing() || obj.Exits == gamedb.Nothing {
			continue
		}
		visited := make(map[gamedb.DBRef]bool)
		cur := obj.Exits
		for cur != gamedb.Nothing {
			if visited[cur] {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   obj.DBRef,
					Description: fmt.Sprintf("#%d exits chain has loop at #%d", obj.DBRef, cur),
				})
				break
			}
			visited[cur] = true
			if o, ok := db.Objects[cur]; ok {
				cur = o.Next
			} else {
				break
			}
			if len(visited) > 50000 {
				findings = append(findings, Finding{
					ID:          mkID(),
					Category:    CatIntegrityError,
					Severity:    SevError,
					ObjectRef:   obj.DBRef,
					Description: fmt.Sprintf("#%d exits chain exceeds 50000 entries", obj.DBRef),
				})
				break
			}
		}
	}

	return findings
}
