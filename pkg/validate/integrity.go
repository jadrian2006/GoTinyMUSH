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

	// objName returns a short name for display
	objName := func(ref gamedb.DBRef) string {
		if o, ok := db.Objects[ref]; ok {
			return truncate(o.Name, 30)
		}
		return "?"
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
					Description: fmt.Sprintf("#%d (%s) location #%d does not exist", ref, objName(ref), obj.Location),
					Explanation: fmt.Sprintf("This object says it is located in #%d, but that object doesn't exist in the database. It may have been in a room or container that was later destroyed.", obj.Location),
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
					Description: fmt.Sprintf("#%d (%s) contents head #%d does not exist", ref, objName(ref), obj.Contents),
					Explanation: fmt.Sprintf("This object's contents list starts with #%d, which doesn't exist. The first item in this room/container was destroyed without being properly removed from the contents chain.", obj.Contents),
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
					Description: fmt.Sprintf("#%d (%s) exits head #%d does not exist", ref, objName(ref), obj.Exits),
					Explanation: fmt.Sprintf("This object's exit list starts with #%d, which doesn't exist. An exit was destroyed without being properly removed from this room's exit chain.", obj.Exits),
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
					Description: fmt.Sprintf("#%d (%s) next #%d does not exist", ref, objName(ref), obj.Next),
					Explanation: fmt.Sprintf("This object's 'next' pointer (used for internal linked lists of room contents/exits) references #%d, which doesn't exist. This is a broken link in the chain.", obj.Next),
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
					Description: fmt.Sprintf("#%d (%s) owner #%d does not exist", ref, objName(ref), obj.Owner),
					Explanation: fmt.Sprintf("This object is owned by #%d, but that player doesn't exist in the database. The owner may have been destroyed.", obj.Owner),
				})
			} else if owner.ObjType() != gamedb.TypePlayer {
				// Expected for God (#1) — warn for others
				if obj.Owner != gamedb.DBRef(1) {
					findings = append(findings, Finding{
						ID:          mkID(),
						Category:    CatIntegrityWarn,
						Severity:    SevWarning,
						ObjectRef:   ref,
						Description: fmt.Sprintf("#%d (%s) owner #%d is not a player (type=%s)", ref, objName(ref), obj.Owner, owner.ObjType()),
						Explanation: fmt.Sprintf("Objects should be owned by player objects. This object is owned by #%d (%s), which is a %s, not a player. This is unusual but may be intentional.", obj.Owner, objName(obj.Owner), owner.ObjType()),
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
					Description: fmt.Sprintf("#%d (%s) parent #%d does not exist", ref, objName(ref), obj.Parent),
					Explanation: fmt.Sprintf("This object inherits from parent #%d, but that parent doesn't exist. The object won't be able to inherit attributes or commands from it.", obj.Parent),
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
					Description: fmt.Sprintf("#%d (%s) zone #%d does not exist", ref, objName(ref), obj.Zone),
					Explanation: fmt.Sprintf("This object belongs to zone #%d, but that zone object doesn't exist. Zone-based permissions and command matching won't work for this object.", obj.Zone),
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
					Description: fmt.Sprintf("#%d (%s) link #%d does not exist", ref, objName(ref), obj.Link),
					Explanation: fmt.Sprintf("This object's link destination is #%d, which doesn't exist. For exits, this means the destination room was destroyed. For other objects, the home/dropto target is missing.", obj.Link),
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
					Description: fmt.Sprintf("#%d (%s) contents chain has loop at #%d", obj.DBRef, objName(obj.DBRef), cur),
					Explanation: fmt.Sprintf("The list of objects inside #%d has a circular reference at #%d — object A points to B which eventually points back to A. This would cause an infinite loop when listing room contents.", obj.DBRef, cur),
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
					Description: fmt.Sprintf("#%d (%s) contents chain exceeds 50000 entries", obj.DBRef, objName(obj.DBRef)),
					Explanation: "This room or container has an impossibly long contents chain (over 50,000 entries), which likely indicates a loop or corruption in the linked list.",
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
					Description: fmt.Sprintf("#%d (%s) exits chain has loop at #%d", obj.DBRef, objName(obj.DBRef), cur),
					Explanation: fmt.Sprintf("The list of exits on #%d has a circular reference at #%d. This would cause an infinite loop when listing available exits.", obj.DBRef, cur),
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
					Description: fmt.Sprintf("#%d (%s) exits chain exceeds 50000 entries", obj.DBRef, objName(obj.DBRef)),
					Explanation: "This room has an impossibly long exit chain (over 50,000 entries), which likely indicates a loop or corruption in the linked list.",
				})
				break
			}
		}
	}

	return findings
}
