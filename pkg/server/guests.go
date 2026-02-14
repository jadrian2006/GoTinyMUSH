package server

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// GuestManager handles guest player creation, tracking, and cleanup.
type GuestManager struct {
	mu     sync.Mutex
	guests map[gamedb.DBRef]time.Time // guest dbref -> creation time
}

// NewGuestManager creates a new guest manager.
func NewGuestManager() *GuestManager {
	return &GuestManager{
		guests: make(map[gamedb.DBRef]time.Time),
	}
}

// IsGuest returns true if the given player is a tracked guest.
func (gm *GuestManager) IsGuest(ref gamedb.DBRef) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	_, ok := gm.guests[ref]
	return ok
}

// Count returns the number of active guests.
func (gm *GuestManager) Count() int {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return len(gm.guests)
}

// Track registers a guest for tracking.
func (gm *GuestManager) Track(ref gamedb.DBRef) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.guests[ref] = time.Now()
}

// Untrack removes a guest from tracking.
func (gm *GuestManager) Untrack(ref gamedb.DBRef) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	delete(gm.guests, ref)
}

// AllGuests returns a copy of all tracked guest dbrefs.
func (gm *GuestManager) AllGuests() []gamedb.DBRef {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	refs := make([]gamedb.DBRef, 0, len(gm.guests))
	for ref := range gm.guests {
		refs = append(refs, ref)
	}
	return refs
}

// GuestsEnabled returns true if the guest system is configured.
func (g *Game) GuestsEnabled() bool {
	return g.Conf != nil && g.Conf.GuestCharNum >= 0
}

// MaxGuests returns the configured max guest count.
func (g *Game) MaxGuests() int {
	if g.Conf != nil && g.Conf.NumberGuests > 0 {
		return g.Conf.NumberGuests
	}
	return 30
}

// GuestPassword returns the configured guest password.
func (g *Game) GuestPassword() string {
	if g.Conf != nil && g.Conf.GuestPassword != "" {
		return g.Conf.GuestPassword
	}
	return "guest"
}

// GuestStartRoom returns the starting room for guests.
func (g *Game) GuestStartRoom() gamedb.DBRef {
	if g.Conf != nil && g.Conf.GuestStartRoom >= 0 {
		return gamedb.DBRef(g.Conf.GuestStartRoom)
	}
	return g.StartingRoom()
}

// GenerateGuestName produces an available guest name using the configured
// prefix/suffix naming scheme. Falls back to basename+number.
func (g *Game) GenerateGuestName() string {
	prefixes := strings.Fields(g.Conf.GuestPrefixes)
	suffixes := strings.Fields(g.Conf.GuestSuffixes)
	basename := g.Conf.GuestBasename
	if basename == "" {
		basename = "Guest"
	}

	// Strategy 1: prefix + suffix combinations
	if len(prefixes) > 0 && len(suffixes) > 0 {
		for _, prefix := range prefixes {
			for _, suffix := range suffixes {
				name := prefix + suffix
				if len(name) <= 22 && !g.IsBadName(name) && LookupPlayer(g.DB, name) == gamedb.Nothing {
					return name
				}
			}
		}
	}

	// Strategy 2: prefixes only or suffixes only
	if len(prefixes) > 0 {
		for _, name := range prefixes {
			if len(name) <= 22 && !g.IsBadName(name) && LookupPlayer(g.DB, name) == gamedb.Nothing {
				return name
			}
		}
	}
	if len(suffixes) > 0 {
		for _, name := range suffixes {
			if len(name) <= 22 && !g.IsBadName(name) && LookupPlayer(g.DB, name) == gamedb.Nothing {
				return name
			}
		}
	}

	// Strategy 3: basename + number
	max := g.MaxGuests()
	for i := 1; i <= max; i++ {
		name := fmt.Sprintf("%s%d", basename, i)
		if LookupPlayer(g.DB, name) == gamedb.Nothing {
			return name
		}
	}

	return "" // All slots exhausted
}

// CreateGuest creates a new guest player object from the template,
// tracking it in the GuestManager.
func (g *Game) CreateGuest() (gamedb.DBRef, string) {
	name := g.GenerateGuestName()
	if name == "" {
		return gamedb.Nothing, ""
	}

	templateRef := gamedb.DBRef(g.Conf.GuestCharNum)
	template, ok := g.DB.Objects[templateRef]
	if !ok {
		log.Printf("guest: template object #%d not found", g.Conf.GuestCharNum)
		return gamedb.Nothing, ""
	}

	// Create the player object
	god := gamedb.DBRef(g.Conf.GodDBRef)
	ref := g.CreateObject(name, gamedb.TypePlayer, god)
	guestObj := g.DB.Objects[ref]

	// Copy key fields from template
	guestObj.Zone = template.Zone
	guestObj.Parent = template.Parent

	// Copy flags from template but ensure Player type is preserved
	guestObj.Flags[0] = (template.Flags[0] & ^0x07) | int(gamedb.TypePlayer)
	guestObj.Flags[1] = template.Flags[1]
	guestObj.Flags[2] = template.Flags[2]

	// Set GUEST power
	guestObj.Powers[0] |= gamedb.PowGuest

	// Set password
	g.SetAttr(ref, aPass, g.GuestPassword())

	// Place in guest start room
	startRoom := g.GuestStartRoom()
	guestObj.Location = startRoom
	guestObj.Link = startRoom // home = start room

	// Add to room contents
	if roomObj, ok := g.DB.Objects[startRoom]; ok {
		guestObj.Next = roomObj.Contents
		roomObj.Contents = ref
		g.PersistObjects(guestObj, roomObj)
	}

	// Copy non-internal attributes from template
	for _, attr := range template.Attrs {
		info := ParseAttrInfo(attr.Value)
		if info.Flags&gamedb.AFInternal != 0 {
			continue
		}
		// Skip password attr
		if attr.Number == aPass {
			continue
		}
		// Copy the raw attribute value (preserves \x01owner:flags:text format)
		guestObj.Attrs = append(guestObj.Attrs, gamedb.Attribute{
			Number: attr.Number,
			Value:  attr.Value,
		})
	}

	// Update player index in bolt store
	if g.Store != nil {
		g.Store.PutMeta()
		g.Store.UpdatePlayerIndex(guestObj, "")
	}

	// Track the guest
	g.Guests.Track(ref)

	log.Printf("guest: created %s(#%d) from template #%d", name, ref, g.Conf.GuestCharNum)
	return ref, name
}

// DestroyGuest destroys a guest player object and cleans up.
func (g *Game) DestroyGuest(ref gamedb.DBRef) {
	obj, ok := g.DB.Objects[ref]
	if !ok {
		return
	}

	// Disconnect any remaining sessions
	for _, dd := range g.Conns.GetByPlayer(ref) {
		dd.Send("Your guest session has ended.")
		dd.Close()
	}

	// Remove from room contents
	g.RemoveFromContents(obj.Location, ref)

	// Mark as GOING
	obj.Flags[0] |= gamedb.FlagGoing

	// Remove from player index — use UpdatePlayerIndex with old name to delete
	if g.Store != nil {
		// Setting oldName removes the old entry; the object is going so
		// we set the obj name empty to avoid re-adding.
		savedName := obj.Name
		obj.Name = ""
		g.Store.UpdatePlayerIndex(obj, savedName)
		obj.Name = savedName // restore for logging
		g.Store.DeleteObject(ref)
	}

	// Untrack
	g.Guests.Untrack(ref)

	// Delete the object from memory
	delete(g.DB.Objects, ref)

	log.Printf("guest: destroyed %s(#%d)", obj.Name, ref)
}

// CleanupDisconnectedGuests destroys any tracked guests that have
// no active connections.
func (g *Game) CleanupDisconnectedGuests() int {
	cleaned := 0
	for _, ref := range g.Guests.AllGuests() {
		descs := g.Conns.GetByPlayer(ref)
		if len(descs) == 0 {
			g.DestroyGuest(ref)
			cleaned++
		}
	}
	return cleaned
}

// handleGuest processes a guest login: cleans up idle guests, creates
// a new guest, and logs them in.
func (s *Server) handleGuest(d *Descriptor) {
	if !s.Game.GuestsEnabled() {
		d.Send("Guest logins are not enabled on this server.")
		return
	}

	// Phase 1: Clean up disconnected guests
	cleaned := s.Game.CleanupDisconnectedGuests()
	if cleaned > 0 {
		log.Printf("guest: cleaned up %d disconnected guest(s)", cleaned)
	}

	// Phase 2: Check guest limit
	if s.Game.Guests.Count() >= s.Game.MaxGuests() {
		d.Send("All guest connections are in use. Please try again later.")
		return
	}

	// Phase 3: Create guest
	ref, name := s.Game.CreateGuest()
	if ref == gamedb.Nothing {
		d.Send("Error creating guest character. Please try again later.")
		return
	}

	// Phase 4: Log in
	s.Game.Conns.Login(d, ref)
	log.Printf("[%d] Guest %s(#%d) connected from %s", d.ID, name, ref, d.Addr)

	d.Send(fmt.Sprintf("Welcome, %s! You are connected as a guest.", name))

	// Show guest text if available
	if s.Game.Texts != nil {
		if txt := s.Game.Texts.GetGuest(); txt != "" {
			d.SendNoNewline(txt)
		}
	}

	// Announce to room
	loc := s.Game.PlayerLocation(ref)
	s.Game.Conns.SendToRoomExcept(s.Game.DB, loc, ref,
		fmt.Sprintf("%s has connected.", name))

	// Show room
	s.Game.ShowRoom(d, loc)

	// Fire ACONNECT
	connCount := len(s.Game.Conns.GetByPlayer(ref))
	s.Game.QueueAttrAction(ref, ref, 35, []string{"connect", fmt.Sprintf("%d", connCount)})
	s.Game.QueueAttrAction(s.Game.MasterRoomRef(), ref, 35, []string{"connect", fmt.Sprintf("%d", connCount)})
}
