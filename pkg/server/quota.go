package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Quota type indices (matching C TinyMUSH QTYPE_* from quota.c)
const (
	QtypeAll    = 0
	QtypeRoom   = 1
	QtypeExit   = 2
	QtypeThing  = 3
	QtypePlayer = 4
	QuotaSlots  = 5
)

// loadQuota parses the space-delimited quota string from an attribute.
// C format: "Q0 Q1 Q2 Q3 Q4" (all room exit thing player)
func loadQuota(g *Game, player gamedb.DBRef, attrNum int) [QuotaSlots]int {
	var q [QuotaSlots]int
	text := g.GetAttrText(player, attrNum)
	if text == "" {
		return q
	}
	parts := strings.Fields(text)
	for i := 0; i < len(parts) && i < QuotaSlots; i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil {
			q[i] = n
		}
	}
	return q
}

// saveQuota writes the space-delimited quota string to an attribute.
func saveQuota(g *Game, player gamedb.DBRef, attrNum int, q [QuotaSlots]int) {
	text := fmt.Sprintf("%d %d %d %d %d", q[0], q[1], q[2], q[3], q[4])
	g.SetAttr(player, attrNum, text)
}

// qtypeForObjType maps object type to quota type index.
func qtypeForObjType(objType gamedb.ObjectType) int {
	switch objType {
	case gamedb.TypeRoom:
		return QtypeRoom
	case gamedb.TypeExit:
		return QtypeExit
	case gamedb.TypeThing:
		return QtypeThing
	case gamedb.TypePlayer:
		return QtypePlayer
	default:
		return QtypeThing
	}
}

// quotaCostForType returns the per-type quota cost from config.
func quotaCostForType(g *Game, objType gamedb.ObjectType) int {
	if g.Conf == nil {
		return 1
	}
	switch objType {
	case gamedb.TypeRoom:
		return g.Conf.RoomQuota
	case gamedb.TypeExit:
		return g.Conf.ExitQuota
	case gamedb.TypeThing:
		return g.Conf.ThingQuota
	case gamedb.TypePlayer:
		return g.Conf.PlayerQuota
	default:
		return 1
	}
}

// CanPayQuota checks if player has enough quota to create an object.
// Returns true if quotas are disabled, player has free_quota, or has enough remaining.
func (g *Game) CanPayQuota(player gamedb.DBRef, cost int, objType gamedb.ObjectType) bool {
	if g.Conf == nil || !g.Conf.Quotas {
		return true
	}
	if CanFreeQuota(g, player) {
		return true
	}
	// Check owner's quota (objects are charged to their owner)
	owner := gamedb.DBRef(ResolveOwner(g, player))
	if CanFreeQuota(g, owner) {
		return true
	}

	rquota := loadQuota(g, owner, 38) // A_RQUOTA
	// Check overall quota
	if rquota[QtypeAll] < cost {
		return false
	}
	// Check typed quota if enabled
	if g.Conf.TypedQuotas {
		qtype := qtypeForObjType(objType)
		if rquota[qtype] < 1 {
			return false
		}
	}
	return true
}

// PayQuota deducts quota for creating an object.
func (g *Game) PayQuota(player gamedb.DBRef, cost int, objType gamedb.ObjectType) {
	if g.Conf == nil || !g.Conf.Quotas {
		return
	}
	owner := gamedb.DBRef(ResolveOwner(g, player))
	if CanFreeQuota(g, owner) {
		return
	}
	rquota := loadQuota(g, owner, 38) // A_RQUOTA
	rquota[QtypeAll] -= cost
	if g.Conf.TypedQuotas {
		qtype := qtypeForObjType(objType)
		rquota[qtype] -= 1
	}
	saveQuota(g, owner, 38, rquota)
}

// RefundQuota returns quota when an object is destroyed.
func (g *Game) RefundQuota(owner gamedb.DBRef, cost int, objType gamedb.ObjectType) {
	if g.Conf == nil || !g.Conf.Quotas {
		return
	}
	rquota := loadQuota(g, owner, 38) // A_RQUOTA
	rquota[QtypeAll] += cost
	if g.Conf.TypedQuotas {
		qtype := qtypeForObjType(objType)
		rquota[qtype] += 1
	}
	saveQuota(g, owner, 38, rquota)
}

// InitPlayerQuota sets initial quota attrs on a newly created player.
func (g *Game) InitPlayerQuota(player gamedb.DBRef) {
	if g.Conf == nil || !g.Conf.Quotas {
		return
	}
	quota := [QuotaSlots]int{
		g.Conf.StartQuota,
		g.Conf.StartRoomQuota,
		g.Conf.StartExitQuota,
		g.Conf.StartThingQuota,
		g.Conf.StartPlayerQuota,
	}
	saveQuota(g, player, 49, quota) // A_QUOTA (absolute limit)
	saveQuota(g, player, 38, quota) // A_RQUOTA (remaining)
}


// cmdQuota implements the @quota command.
// @quota [player] — view quota
// @quota player=amount — set overall quota
// @quota/room player=amount — set room quota
// @quota/exit player=amount — set exit quota
// @quota/thing player=amount — set thing quota
// @quota/player player=amount — set player quota
// @quota/fix player — recalculate used quota by scanning DB
func cmdQuota(g *Game, d *Descriptor, args string, switches []string) {
	if g.Conf == nil || !g.Conf.Quotas {
		g.Notify(d.Player, "Quotas are not enabled.")
		return
	}

	// Parse args: player[=amount]
	var targetStr, amountStr string
	if eqIdx := strings.IndexByte(args, '='); eqIdx >= 0 {
		targetStr = strings.TrimSpace(args[:eqIdx])
		amountStr = strings.TrimSpace(args[eqIdx+1:])
	} else {
		targetStr = strings.TrimSpace(args)
	}

	// Default to self
	var target gamedb.DBRef
	if targetStr == "" {
		target = d.Player
	} else {
		target = g.MatchObject(d.Player, targetStr)
		if target == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that player.")
			return
		}
	}

	// Resolve to player (quota is on players/owners)
	targetObj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	if targetObj.ObjType() != gamedb.TypePlayer {
		g.Notify(d.Player, "Quotas are only on players.")
		return
	}

	// Setting quota requires change_quotas power
	if amountStr != "" || HasSwitch(switches, "fix") {
		if !CanChangeQuotas(g, d.Player) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
	}

	// @quota/fix — recalculate used quota by scanning DB
	if HasSwitch(switches, "fix") {
		g.fixQuota(d, target)
		return
	}

	// Setting quota
	if amountStr != "" {
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			g.Notify(d.Player, "Invalid quota amount.")
			return
		}

		quota := loadQuota(g, target, 49)  // A_QUOTA (limits)
		rquota := loadQuota(g, target, 38) // A_RQUOTA (remaining)

		// Determine which quota slot to set
		qtype := QtypeAll
		if HasSwitch(switches, "room") {
			qtype = QtypeRoom
		} else if HasSwitch(switches, "exit") {
			qtype = QtypeExit
		} else if HasSwitch(switches, "thing") {
			qtype = QtypeThing
		} else if HasSwitch(switches, "player") {
			qtype = QtypePlayer
		}

		// Adjust remaining by the difference
		used := quota[qtype] - rquota[qtype]
		quota[qtype] = amount
		rquota[qtype] = amount - used
		if rquota[qtype] < 0 {
			rquota[qtype] = 0
		}

		saveQuota(g, target, 49, quota)
		saveQuota(g, target, 38, rquota)
		g.Notify(d.Player, fmt.Sprintf("Quota set for %s.", DisplayName(targetObj.Name)))
		return
	}

	// Display quota
	quota := loadQuota(g, target, 49)  // A_QUOTA (limits)
	rquota := loadQuota(g, target, 38) // A_RQUOTA (remaining)

	if CanFreeQuota(g, target) {
		g.Notify(d.Player, fmt.Sprintf("%s: Quota: N/A (unlimited)", DisplayName(targetObj.Name)))
		return
	}

	usedAll := quota[QtypeAll] - rquota[QtypeAll]
	g.Notify(d.Player, fmt.Sprintf("%s: Objects: %d  Quota: %d  Remaining: %d",
		DisplayName(targetObj.Name), usedAll, quota[QtypeAll], rquota[QtypeAll]))

	if g.Conf.TypedQuotas {
		for _, entry := range []struct {
			name  string
			qtype int
		}{
			{"Rooms", QtypeRoom},
			{"Exits", QtypeExit},
			{"Things", QtypeThing},
			{"Players", QtypePlayer},
		} {
			used := quota[entry.qtype] - rquota[entry.qtype]
			g.Notify(d.Player, fmt.Sprintf("  %-8s Used: %d  Limit: %d  Remaining: %d",
				entry.name, used, quota[entry.qtype], rquota[entry.qtype]))
		}
	}
}

// fixQuota recalculates a player's used quota by scanning the DB.
func (g *Game) fixQuota(d *Descriptor, target gamedb.DBRef) {
	quota := loadQuota(g, target, 49) // A_QUOTA (limits)
	var counts [QuotaSlots]int

	for _, obj := range g.DB.Objects {
		owner := gamedb.DBRef(ResolveOwner(g, gamedb.DBRef(obj.DBRef)))
		if owner != target {
			continue
		}
		if obj.IsGoing() {
			continue
		}
		qtype := qtypeForObjType(obj.ObjType())
		counts[QtypeAll]++
		counts[qtype]++
	}

	// Remaining = limit - used
	var rquota [QuotaSlots]int
	for i := 0; i < QuotaSlots; i++ {
		rquota[i] = quota[i] - counts[i]
		if rquota[i] < 0 {
			rquota[i] = 0
		}
	}
	saveQuota(g, target, 38, rquota)

	targetObj := g.DB.Objects[target]
	g.Notify(d.Player, fmt.Sprintf("Quota fixed for %s: %d objects counted, %d remaining.",
		DisplayName(targetObj.Name), counts[QtypeAll], rquota[QtypeAll]))
}
