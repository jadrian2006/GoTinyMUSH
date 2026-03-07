package eventbus

import (
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// ProcessEventBus executes the bus phase. Called after ProcessQueue() each tick.
// Returns true if any work was done.
func (m *QueueManager) ProcessEventBus(g GameInterface) bool {
	if len(m.queues) == 0 {
		return false
	}

	m.BusPhaseActive = true
	defer func() {
		m.BusPhaseActive = false
		m.CurrentGeneration = 0
	}()

	budget := MaxBusHandlersPerTick
	hadWork := false
	m.Stats.TotalTicks++
	m.Stats.LastTickTime = time.Now()

	// Build rotated iteration order for fairness
	n := len(m.order)
	if n == 0 {
		return false
	}
	startIdx := m.roundIdx % n
	m.roundIdx++

	for i := range n {
		idx := (startIdx + i) % n
		queueName := m.order[idx]
		q := m.queues[queueName]
		if q == nil || !q.Enabled {
			continue
		}

		// Rate check for periodic queues
		if q.Rate > 0 {
			q.Counter++
			if q.Counter < q.Rate {
				continue
			}
			q.Counter = 0
		}

		// Determine events to process
		var events []PendingEvent
		if q.Rate == 0 {
			// Event-driven: drain pending
			if len(q.Pending) == 0 {
				continue
			}
			events = q.Pending
			q.Pending = make([]PendingEvent, 0)
		} else {
			// Rate-driven: if there are pending events, use them; otherwise synthetic tick
			if len(q.Pending) > 0 {
				events = q.Pending
				q.Pending = make([]PendingEvent, 0)
			} else {
				events = []PendingEvent{{
					Publisher:  gamedb.Nothing,
					Data:      "{}",
					Generation: 0,
					Timestamp: time.Now(),
				}}
			}
		}

		q.Stats.FireCount++
		q.Stats.LastFireTime = time.Now()
		hadWork = true

		for _, event := range events {
			m.CurrentGeneration = event.Generation

			for _, sub := range q.Subscribers {
				// Validate subscriber still exists
				obj := g.GetObject(sub.Object)
				if obj == nil {
					continue
				}

				// Scope filtering
				switch q.Scope {
				case ScopeObject:
					if sub.Bind != gamedb.Nothing && sub.Bind != event.Publisher {
						continue
					}
				case ScopeParent:
					if event.Publisher != gamedb.Nothing {
						subParent := g.GetParent(sub.Object)
						pubParent := g.GetParent(event.Publisher)
						if subParent == gamedb.Nothing || subParent != pubParent {
							continue
						}
					}
				case ScopeGlobal:
					// All subscribers receive
				}

				// Execute handler directly in bus phase
				start := time.Now()
				g.TriggerAttr(obj.Owner, sub.Object, sub.Object, sub.Attr, []string{event.Data})
				elapsed := time.Since(start)

				// Update stats
				evalNs := elapsed.Nanoseconds()
				q.Stats.DeliverCount++
				q.Stats.TotalEvalNs += evalNs
				if q.Stats.MinEvalNs < 0 || evalNs < q.Stats.MinEvalNs {
					q.Stats.MinEvalNs = evalNs
				}
				if evalNs > q.Stats.MaxEvalNs {
					q.Stats.MaxEvalNs = evalNs
				}
				m.Stats.TotalHandlersFired++

				budget--
				if budget <= 0 {
					q.Stats.BudgetExhausted++
					m.Stats.BudgetExhausted++
					return true
				}
			}
		}
	}

	return hadWork
}

// CleanDeadSubscribers removes subscriptions for objects that no longer exist.
func (m *QueueManager) CleanDeadSubscribers(g GameInterface) int {
	removed := 0
	for _, q := range m.queues {
		alive := make([]Subscription, 0, len(q.Subscribers))
		for _, sub := range q.Subscribers {
			if g.GetObject(sub.Object) != nil {
				alive = append(alive, sub)
			} else {
				removed++
			}
		}
		q.Subscribers = alive
	}
	return removed
}
