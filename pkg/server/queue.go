package server

import (
	"log"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// QueueEntry represents a queued command to be executed.
type QueueEntry struct {
	Player  gamedb.DBRef   // Object executing the command
	Cause   gamedb.DBRef   // Enactor who triggered this
	Caller  gamedb.DBRef   // Caller context
	Command string         // Command string to execute
	Args    []string       // Captured args from $ matching (%0-%9)
	RData   *eval.RegisterData // Saved register state
	WaitUntil time.Time    // When to execute (zero = immediate)
	SemObj  gamedb.DBRef   // Semaphore object (Nothing = none)
	SemAttr int            // Semaphore attribute number
}

// CommandQueue manages queued commands for execution.
type CommandQueue struct {
	mu        sync.Mutex
	immediate []*QueueEntry // Execute ASAP
	waitQueue []*QueueEntry // Delayed execution
	semQueue  []*QueueEntry // Waiting on semaphores
	maxPerObj int           // Max queued commands per owner
}

// NewCommandQueue creates a new command queue.
func NewCommandQueue() *CommandQueue {
	return &CommandQueue{
		maxPerObj: 1000,
	}
}

// Add queues a command for immediate execution.
func (q *CommandQueue) Add(entry *QueueEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Enforce per-object queue limit to prevent runaway objects
	if q.maxPerObj > 0 {
		count := 0
		for _, e := range q.immediate {
			if e.Player == entry.Player {
				count++
			}
		}
		if count >= q.maxPerObj {
			log.Printf("QUEUE: dropping entry for #%d — per-object limit (%d) reached", entry.Player, q.maxPerObj)
			return
		}
	}
	q.immediate = append(q.immediate, entry)
}

// AddWait queues a command for delayed execution.
func (q *CommandQueue) AddWait(entry *QueueEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Insert sorted by WaitUntil
	inserted := false
	for i, e := range q.waitQueue {
		if entry.WaitUntil.Before(e.WaitUntil) {
			q.waitQueue = append(q.waitQueue[:i+1], q.waitQueue[i:]...)
			q.waitQueue[i] = entry
			inserted = true
			break
		}
	}
	if !inserted {
		q.waitQueue = append(q.waitQueue, entry)
	}
}

// AddSemaphore queues a command waiting on a semaphore.
func (q *CommandQueue) AddSemaphore(entry *QueueEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.semQueue = append(q.semQueue, entry)
}

// NotifySemaphore wakes up commands waiting on a semaphore.
// Returns the number of commands woken.
func (q *CommandQueue) NotifySemaphore(obj gamedb.DBRef, attr int, count int) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	woken := 0
	var remaining []*QueueEntry
	for _, e := range q.semQueue {
		if e.SemObj == obj && e.SemAttr == attr && woken < count {
			q.immediate = append(q.immediate, e)
			woken++
		} else {
			remaining = append(remaining, e)
		}
	}
	q.semQueue = remaining
	return woken
}

// DrainSemaphore removes all commands waiting on a semaphore.
func (q *CommandQueue) DrainSemaphore(obj gamedb.DBRef, attr int) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	removed := 0
	var remaining []*QueueEntry
	for _, e := range q.semQueue {
		if e.SemObj == obj && e.SemAttr == attr {
			removed++
		} else {
			remaining = append(remaining, e)
		}
	}
	q.semQueue = remaining
	return removed
}

// DrainObject removes all semaphore and wait queue entries for an object.
// Returns the number of entries removed.
func (q *CommandQueue) DrainObject(obj gamedb.DBRef, semAttr int) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	removed := 0
	// Drain semaphore queue entries for this object
	var remSem []*QueueEntry
	for _, e := range q.semQueue {
		if e.SemObj == obj && (semAttr <= 0 || e.SemAttr == semAttr) {
			removed++
		} else {
			remSem = append(remSem, e)
		}
	}
	q.semQueue = remSem

	// Also drain wait queue entries belonging to this object
	var remWait []*QueueEntry
	for _, e := range q.waitQueue {
		if e.Player == obj {
			removed++
		} else {
			remWait = append(remWait, e)
		}
	}
	q.waitQueue = remWait

	return removed
}

// PromoteReady moves entries from the wait queue whose time has come.
// Returns the number of entries promoted.
func (q *CommandQueue) PromoteReady() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	cutoff := 0
	for i, e := range q.waitQueue {
		if e.WaitUntil.After(now) {
			break
		}
		cutoff = i + 1
	}
	if cutoff > 0 {
		q.immediate = append(q.immediate, q.waitQueue[:cutoff]...)
		q.waitQueue = q.waitQueue[cutoff:]
	}
	return cutoff
}

// PopImmediate returns and removes the next immediate command, or nil.
func (q *CommandQueue) PopImmediate() *QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.immediate) == 0 {
		return nil
	}
	entry := q.immediate[0]
	q.immediate = q.immediate[1:]
	return entry
}

// HaltPlayer removes all queued commands for a player/object.
func (q *CommandQueue) HaltPlayer(player gamedb.DBRef) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	removed := 0
	filter := func(entries []*QueueEntry) []*QueueEntry {
		var result []*QueueEntry
		for _, e := range entries {
			if e.Player == player {
				removed++
			} else {
				result = append(result, e)
			}
		}
		return result
	}
	q.immediate = filter(q.immediate)
	q.waitQueue = filter(q.waitQueue)
	q.semQueue = filter(q.semQueue)
	return removed
}

// HaltAll removes all queued commands from all queues.
func (q *CommandQueue) HaltAll() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	removed := len(q.immediate) + len(q.waitQueue) + len(q.semQueue)
	q.immediate = nil
	q.waitQueue = nil
	q.semQueue = nil
	return removed
}

// CountByOwner returns how many commands are queued for a given owner.
func (q *CommandQueue) CountByOwner(db *gamedb.Database, owner gamedb.DBRef) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := 0
	countFor := func(entries []*QueueEntry) {
		for _, e := range entries {
			if obj, ok := db.Objects[e.Player]; ok {
				if obj.Owner == owner {
					count++
				}
			}
		}
	}
	countFor(q.immediate)
	countFor(q.waitQueue)
	countFor(q.semQueue)
	return count
}

// Stats returns queue size info.
func (q *CommandQueue) Stats() (immediate, waiting, semaphore int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.immediate), len(q.waitQueue), len(q.semQueue)
}

// Peek returns up to n entries from all queues for inspection (does not remove them).
func (q *CommandQueue) Peek(n int) []*QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	var result []*QueueEntry
	for _, e := range q.immediate {
		if len(result) >= n {
			break
		}
		result = append(result, e)
	}
	for _, e := range q.waitQueue {
		if len(result) >= n {
			break
		}
		result = append(result, e)
	}
	for _, e := range q.semQueue {
		if len(result) >= n {
			break
		}
		result = append(result, e)
	}
	return result
}
