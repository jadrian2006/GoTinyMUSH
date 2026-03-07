package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eventbus"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// cmdQueue is the main @queue command dispatcher.
// Switches route to subcommands: create, set, destroy, lock, list, info, subs, pubs, stats, drain, bus, alias.
func cmdQueue(g *Game, d *Descriptor, args string, switches []string) {
	if g.EventQueues == nil {
		d.Send("Event bus not initialized.")
		return
	}

	if len(switches) == 0 {
		d.Send("Usage: @queue/<switch> [args]")
		d.Send("Switches: create, set, destroy, lock, list, info, subs, pubs, stats, drain, bus, alias")
		return
	}

	sw := strings.ToLower(switches[0])

	switch sw {
	case "create":
		cmdQueueCreate(g, d, args, switches)
	case "set":
		cmdQueueSet(g, d, args, switches)
	case "destroy":
		cmdQueueDestroy(g, d, args, switches)
	case "lock":
		cmdQueueLock(g, d, args, switches)
	case "list":
		cmdQueueList(g, d, args)
	case "info":
		cmdQueueInfo(g, d, args)
	case "subs":
		cmdQueueSubs(g, d, args)
	case "pubs":
		cmdQueuePubs(g, d, args)
	case "stats":
		if HasSwitch(switches, "reset") {
			cmdQueueStatsReset(g, d, args)
		} else {
			cmdQueueStats(g, d, args)
		}
	case "drain":
		cmdQueueDrain(g, d, args)
	case "bus":
		cmdQueueBus(g, d)
	case "alias":
		cmdQueueAlias(g, d, args)
	default:
		d.Send(fmt.Sprintf("Unknown @queue switch: %s", sw))
	}
}

// cmdQueueCreate handles @queue/create <name>=<options>
func cmdQueueCreate(g *Game, d *Descriptor, args string, _ []string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}

	name, optStr, _ := strings.Cut(args, "=")
	name = strings.TrimSpace(name)
	if name == "" {
		d.Send("Usage: @queue/create <name>=<options>")
		return
	}

	q := eventbus.NewEventQueue(name)

	// Parse options: rate:N scope:type max_subs:N enabled:yes|no
	if optStr != "" {
		for _, opt := range strings.Fields(optStr) {
			key, val, ok := strings.Cut(opt, ":")
			if !ok {
				d.Send(fmt.Sprintf("Invalid option format: %s (use key:value)", opt))
				return
			}
			switch strings.ToLower(key) {
			case "rate":
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					d.Send(fmt.Sprintf("Invalid rate: %s", val))
					return
				}
				q.Rate = n
			case "scope":
				scope, ok := eventbus.ParseScopeType(strings.ToLower(val))
				if !ok {
					d.Send(fmt.Sprintf("Invalid scope: %s (use global, object, or parent)", val))
					return
				}
				q.Scope = scope
			case "max_subs":
				n, err := strconv.Atoi(val)
				if err != nil || n < 1 {
					d.Send(fmt.Sprintf("Invalid max_subs: %s", val))
					return
				}
				q.MaxSubs = n
			case "enabled":
				switch strings.ToLower(val) {
				case "yes", "1", "true":
					q.Enabled = true
				case "no", "0", "false":
					q.Enabled = false
				default:
					d.Send(fmt.Sprintf("Invalid enabled value: %s (use yes or no)", val))
					return
				}
			default:
				d.Send(fmt.Sprintf("Unknown option: %s", key))
				return
			}
		}
	}

	if err := g.EventQueues.CreateQueue(q); err != nil {
		d.Send(fmt.Sprintf("Error: %v", err))
		return
	}

	g.PersistEventQueue(q)
	d.Send(fmt.Sprintf("Queue %q created. Rate:%d Scope:%s MaxSubs:%d Enabled:%v",
		q.Name, q.Rate, q.Scope, q.MaxSubs, q.Enabled))
}

// cmdQueueSet handles @queue/set <name>=<option>:<value>
func cmdQueueSet(g *Game, d *Descriptor, args string, _ []string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}

	name, optStr, ok := strings.Cut(args, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		d.Send("Usage: @queue/set <name>=<option>:<value>")
		return
	}

	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	key, val, ok := strings.Cut(strings.TrimSpace(optStr), ":")
	if !ok {
		d.Send("Usage: @queue/set <name>=<option>:<value>")
		return
	}

	switch strings.ToLower(key) {
	case "rate":
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			d.Send(fmt.Sprintf("Invalid rate: %s", val))
			return
		}
		q.Rate = n
	case "max_subs":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 {
			d.Send(fmt.Sprintf("Invalid max_subs: %s", val))
			return
		}
		q.MaxSubs = n
	case "enabled":
		switch strings.ToLower(val) {
		case "yes", "1", "true":
			q.Enabled = true
		case "no", "0", "false":
			q.Enabled = false
		default:
			d.Send(fmt.Sprintf("Invalid enabled value: %s", val))
			return
		}
	default:
		d.Send(fmt.Sprintf("Unknown option: %s", key))
		return
	}

	g.PersistEventQueue(q)
	d.Send(fmt.Sprintf("Queue %q updated: %s:%s", q.Name, key, val))
}

// cmdQueueDestroy handles @queue/destroy <name>
func cmdQueueDestroy(g *Game, d *Descriptor, args string, _ []string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	if err := g.EventQueues.DestroyQueue(name); err != nil {
		d.Send(fmt.Sprintf("Error: %v", err))
		return
	}
	g.DeleteEventQueue(name)
	d.Send(fmt.Sprintf("Queue %q destroyed.", name))
}

// cmdQueueLock handles @queue/lock/pub, @queue/lock/sub, @queue/lock/admin
func cmdQueueLock(g *Game, d *Descriptor, args string, switches []string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}

	// switches[0] = "lock", switches[1] = "pub"|"sub"|"admin"
	if len(switches) < 2 {
		d.Send("Usage: @queue/lock/pub|sub|admin <name>=<lock_expression>")
		return
	}

	lockType := strings.ToLower(switches[1])
	name, lockStr, _ := strings.Cut(args, "=")
	name = strings.TrimSpace(name)
	lockStr = strings.TrimSpace(lockStr)

	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	switch lockType {
	case "pub":
		q.PubLock = lockStr
	case "sub":
		q.SubLock = lockStr
	case "admin":
		q.AdminLock = lockStr
	default:
		d.Send(fmt.Sprintf("Unknown lock type: %s (use pub, sub, or admin)", lockType))
		return
	}

	g.PersistEventQueue(q)
	if lockStr == "" {
		d.Send(fmt.Sprintf("Queue %q %s lock cleared (wizard-only).", q.Name, lockType))
	} else {
		d.Send(fmt.Sprintf("Queue %q %s lock set to: %s", q.Name, lockType, lockStr))
	}
}

// cmdQueueAlias handles @queue/alias <alias>=<queue_name>
func cmdQueueAlias(g *Game, d *Descriptor, args string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}

	alias, queueName, ok := strings.Cut(args, "=")
	alias = strings.TrimSpace(alias)
	queueName = strings.TrimSpace(queueName)
	if !ok || alias == "" || queueName == "" {
		d.Send("Usage: @queue/alias <alias>=<queue_name>")
		return
	}

	if err := g.EventQueues.AddAlias(alias, queueName); err != nil {
		d.Send(fmt.Sprintf("Error: %v", err))
		return
	}
	d.Send(fmt.Sprintf("Alias %q -> %q created.", alias, queueName))
}

// cmdQueueList handles @queue/list
func cmdQueueList(g *Game, d *Descriptor, _ string) {
	names := g.EventQueues.QueueNames()
	if len(names) == 0 {
		d.Send("No event queues defined.")
		return
	}
	d.Send(fmt.Sprintf("Event Queues (%d):", len(names)))
	for _, name := range names {
		q := g.EventQueues.GetQueue(name)
		if q == nil {
			continue
		}
		enabled := "yes"
		if !q.Enabled {
			enabled = "no"
		}
		aliases := ""
		if len(q.Aliases) > 0 {
			aliases = " (alias: " + strings.Join(q.Aliases, ", ") + ")"
		}
		d.Send(fmt.Sprintf("  %-20s Rate:%-4d Scope:%-7s Subs:%-3d Enabled:%s%s",
			q.Name, q.Rate, q.Scope, q.ActiveSubCount(), enabled, aliases))
	}
}

// cmdQueueInfo handles @queue/info <name>
func cmdQueueInfo(g *Game, d *Descriptor, args string) {
	name := strings.TrimSpace(args)
	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	d.Send(fmt.Sprintf("Queue: %s", q.Name))
	if len(q.Aliases) > 0 {
		d.Send(fmt.Sprintf("  Aliases: %s", strings.Join(q.Aliases, ", ")))
	}
	d.Send(fmt.Sprintf("  Rate: %d  Scope: %s  MaxSubs: %d  Enabled: %v",
		q.Rate, q.Scope, q.MaxSubs, q.Enabled))
	d.Send(fmt.Sprintf("  Subscribers: %d  Publishers: %d  Pending: %d",
		q.ActiveSubCount(), q.ActivePubCount(), q.PendingCount()))

	pubLock := q.PubLock
	if pubLock == "" {
		pubLock = "(wizard-only)"
	}
	subLock := q.SubLock
	if subLock == "" {
		subLock = "(wizard-only)"
	}
	adminLock := q.AdminLock
	if adminLock == "" {
		adminLock = "(wizard-only)"
	}
	d.Send(fmt.Sprintf("  PubLock: %s", pubLock))
	d.Send(fmt.Sprintf("  SubLock: %s", subLock))
	d.Send(fmt.Sprintf("  AdminLock: %s", adminLock))
}

// cmdQueueSubs handles @queue/subs <name>
func cmdQueueSubs(g *Game, d *Descriptor, args string) {
	name := strings.TrimSpace(args)
	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	if len(q.Subscribers) == 0 {
		d.Send(fmt.Sprintf("Queue %q has no subscribers.", q.Name))
		return
	}

	d.Send(fmt.Sprintf("Subscribers for %s (%d):", q.Name, len(q.Subscribers)))
	for _, sub := range q.Subscribers {
		bind := ""
		if sub.Bind != gamedb.Nothing {
			bind = fmt.Sprintf(" bind:#%d", sub.Bind)
		}
		objName := fmt.Sprintf("#%d", sub.Object)
		if obj, ok := g.DB.Objects[sub.Object]; ok {
			objName = fmt.Sprintf("%s(#%d)", obj.Name, sub.Object)
		}
		d.Send(fmt.Sprintf("  %s/%s%s", objName, sub.Attr, bind))
	}
}

// cmdQueuePubs handles @queue/pubs <name>
func cmdQueuePubs(g *Game, d *Descriptor, args string) {
	name := strings.TrimSpace(args)
	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	if len(q.Publishers) == 0 {
		d.Send(fmt.Sprintf("Queue %q has no publishers.", q.Name))
		return
	}

	d.Send(fmt.Sprintf("Publishers for %s (%d unique):", q.Name, len(q.Publishers)))
	for ref := range q.Publishers {
		objName := fmt.Sprintf("#%d", ref)
		if obj, ok := g.DB.Objects[ref]; ok {
			objName = fmt.Sprintf("%s(#%d)", obj.Name, ref)
		}
		d.Send(fmt.Sprintf("  %s", objName))
	}
}

// cmdQueueStats handles @queue/stats <name>
func cmdQueueStats(g *Game, d *Descriptor, args string) {
	name := strings.TrimSpace(args)
	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	s := &q.Stats
	aliases := ""
	if len(q.Aliases) > 0 {
		aliases = " (alias: " + strings.Join(q.Aliases, ", ") + ")"
	}
	enabled := "yes"
	if !q.Enabled {
		enabled = "no"
	}

	d.Send(fmt.Sprintf("Queue: %s%s", q.Name, aliases))
	d.Send(fmt.Sprintf("Created: %s  Scope: %s  Rate: %d  Enabled: %s",
		s.CreatedAt.Format("2006-01-02 15:04:05"), q.Scope, q.Rate, enabled))
	d.Send("")
	d.Send("--- Publishers ---")
	d.Send(fmt.Sprintf("  Unique publishers:     %d", q.ActivePubCount()))
	if !s.LastPublishTime.IsZero() {
		d.Send(fmt.Sprintf("  Last publish:          %s", fmtAgo(s.LastPublishTime)))
	}
	d.Send("")
	d.Send("--- Subscribers ---")
	d.Send(fmt.Sprintf("  Active subscribers:    %d", q.ActiveSubCount()))
	d.Send(fmt.Sprintf("  Pending events:        %d", q.PendingCount()))
	d.Send("")
	d.Send("--- Volume (since reset) ---")
	d.Send(fmt.Sprintf("  Publishes received:    %d", s.PublishCount))
	d.Send(fmt.Sprintf("  Events fired:          %d", s.FireCount))
	d.Send(fmt.Sprintf("  Deliveries (sub hits): %d", s.DeliverCount))
	d.Send(fmt.Sprintf("  Dropped:               %d", s.DroppedCount))
	d.Send(fmt.Sprintf("  Lock rejects:          %d", s.RejectCount))
	d.Send("")
	d.Send("--- Payload Size ---")
	if s.PublishCount > 0 {
		avg := s.TotalPayloadBytes / s.PublishCount
		d.Send(fmt.Sprintf("  Average:               %d bytes", avg))
		d.Send(fmt.Sprintf("  Min:                   %d bytes", s.MinPayloadBytes))
		d.Send(fmt.Sprintf("  Max:                   %d bytes", s.MaxPayloadBytes))
		d.Send(fmt.Sprintf("  Total:                 %s", fmtBytes(s.TotalPayloadBytes)))
	} else {
		d.Send("  (no events)")
	}
	d.Send("")
	d.Send("--- Handler Performance ---")
	if s.DeliverCount > 0 {
		avgNs := s.TotalEvalNs / s.DeliverCount
		d.Send(fmt.Sprintf("  Avg eval time:         %s", fmtNs(avgNs)))
		d.Send(fmt.Sprintf("  Min eval time:         %s", fmtNs(s.MinEvalNs)))
		d.Send(fmt.Sprintf("  Max eval time:         %s", fmtNs(s.MaxEvalNs)))
		d.Send(fmt.Sprintf("  Total eval time:       %s", fmtNs(s.TotalEvalNs)))
	} else {
		d.Send("  (no deliveries)")
	}
	d.Send("")
	d.Send("--- Budget ---")
	d.Send(fmt.Sprintf("  Budget per tick:       %d", eventbus.MaxBusHandlersPerTick))
	d.Send(fmt.Sprintf("  Budget exhausted:      %d times", s.BudgetExhausted))
	d.Send(fmt.Sprintf("  Gen 1 events:          %d", s.Gen1Count))
	d.Send(fmt.Sprintf("  Gen 2 rejected:        %d", s.Gen2Rejected))
}

// cmdQueueStatsReset handles @queue/stats/reset <name>
func cmdQueueStatsReset(g *Game, d *Descriptor, args string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}

	name := strings.TrimSpace(args)
	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}

	created := q.Stats.CreatedAt
	q.Stats = eventbus.QueueStats{
		CreatedAt:       created,
		MinPayloadBytes: -1,
		MinEvalNs:       -1,
	}
	d.Send(fmt.Sprintf("Stats for %q reset.", q.Name))
}

// cmdQueueDrain handles @queue/drain <name>
func cmdQueueDrain(g *Game, d *Descriptor, args string) {
	if !g.IsWizard(d.Player) {
		d.Send("Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	q := g.EventQueues.GetQueue(name)
	if q == nil {
		d.Send(fmt.Sprintf("Queue %q not found.", name))
		return
	}
	pending := q.PendingCount()
	if pending == 0 {
		d.Send(fmt.Sprintf("Queue %q has no pending events.", q.Name))
		return
	}
	// Force process this queue's pending events
	d.Send(fmt.Sprintf("Draining %d pending events from %q...", pending, q.Name))
	// The actual drain happens on the next ProcessEventBus tick
}

// cmdQueueBus handles @queue/bus — global bus stats
func cmdQueueBus(g *Game, d *Descriptor) {
	s := &g.EventQueues.Stats
	d.Send("--- Event Bus (Global) ---")
	d.Send(fmt.Sprintf("  Total queues:          %d", len(g.EventQueues.QueueNames())))
	d.Send(fmt.Sprintf("  Total ticks:           %d", s.TotalTicks))
	d.Send(fmt.Sprintf("  Total handlers fired:  %d", s.TotalHandlersFired))
	d.Send(fmt.Sprintf("  Budget exhausted:      %d times", s.BudgetExhausted))
	if !s.LastTickTime.IsZero() {
		d.Send(fmt.Sprintf("  Last tick:             %s", fmtAgo(s.LastTickTime)))
	}
	d.Send(fmt.Sprintf("  Max handlers/tick:     %d", eventbus.MaxBusHandlersPerTick))
	d.Send(fmt.Sprintf("  Max generation:        %d", eventbus.MaxBusGeneration))
}

// --- Formatting helpers ---

func fmtAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Second:
		return fmt.Sprintf("%.1fms ago", float64(d.Nanoseconds())/1e6)
	case d < time.Minute:
		return fmt.Sprintf("%.1fs ago", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%.0fm ago", d.Minutes())
	default:
		return fmt.Sprintf("%.1fh ago", d.Hours())
	}
}

func fmtBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d bytes", b)
	}
}

func fmtNs(ns int64) string {
	switch {
	case ns >= 1_000_000_000:
		return fmt.Sprintf("%.2fs", float64(ns)/1e9)
	case ns >= 1_000_000:
		return fmt.Sprintf("%.2f ms", float64(ns)/1e6)
	case ns >= 1_000:
		return fmt.Sprintf("%.2f µs", float64(ns)/1e3)
	default:
		return fmt.Sprintf("%d ns", ns)
	}
}
