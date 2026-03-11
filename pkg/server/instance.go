package server

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// cmdInstance implements @instance/create and @instance/destroy for the
// vehicle/container instance system.
//
// @instance/create <template> [= <name>]
//   Clones a template THING and all its interior rooms (rooms whose Location
//   = template dbref) plus exits between those rooms. Sets Flag3Instance on
//   the clone. The new instance is placed at the creator's location.
//
// @instance/destroy <instance>
//   Destroys an instance: ejects all players from interior rooms to the
//   instance's exterior location, destroys interior exits, rooms, and the
//   instance THING itself.
func cmdInstance(g *Game, d *Descriptor, args string, switches []string) {
	if g.Conf != nil && !g.Conf.InstancesEnabled {
		g.Notify(d.Player, "The instance system is not enabled.")
		return
	}
	if HasSwitch(switches, "create") {
		instanceCreate(g, d, args)
	} else if HasSwitch(switches, "destroy") {
		instanceDestroy(g, d, args)
	} else {
		g.Notify(d.Player, "Usage: @instance/create <template> [= <name>]  or  @instance/destroy <instance>")
	}
}

// instanceCreate clones a template THING and its interior rooms/exits.
func instanceCreate(g *Game, d *Descriptor, args string) {
	if args == "" {
		g.Notify(d.Player, "Create an instance of what?")
		return
	}

	parts := strings.SplitN(args, "=", 2)
	templateStr := strings.TrimSpace(parts[0])
	newName := ""
	if len(parts) > 1 {
		newName = strings.TrimSpace(parts[1])
	}

	template := g.MatchObject(d.Player, templateStr)
	if template == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	tmplObj, ok := g.DB.Objects[template]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	if tmplObj.ObjType() != gamedb.TypeThing {
		g.Notify(d.Player, "You can only create instances of THINGs.")
		return
	}
	if !Controls(g, d.Player, template) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	if newName == "" {
		newName = tmplObj.Name
	}

	// 1. Clone the template THING
	instanceRef := g.CreateObject(newName, gamedb.TypeThing, d.Player)
	instanceObj := g.DB.Objects[instanceRef]
	instanceObj.Flags[2] |= gamedb.Flag3Instance
	instanceObj.Flags[0] |= gamedb.FlagEnterOK // Instances are enterable by default
	instanceObj.Parent = tmplObj.Parent
	instanceObj.Link = tmplObj.Link

	// Copy attributes from template
	for _, attr := range tmplObj.Attrs {
		instanceObj.Attrs = append(instanceObj.Attrs, gamedb.Attribute{
			Number: attr.Number,
			Value:  attr.Value,
		})
	}

	// 2. Find interior rooms (rooms whose Location = template)
	var templateRooms []gamedb.DBRef
	for ref, obj := range g.DB.Objects {
		if obj.ObjType() == gamedb.TypeRoom && obj.Location == template && !obj.IsGoing() {
			templateRooms = append(templateRooms, ref)
		}
	}

	// 3. Clone interior rooms, building old->new mapping
	refMap := make(map[gamedb.DBRef]gamedb.DBRef)
	refMap[template] = instanceRef

	for _, oldRoom := range templateRooms {
		oldObj := g.DB.Objects[oldRoom]
		newRoomRef := g.CreateObject(oldObj.Name, gamedb.TypeRoom, d.Player)
		newRoom := g.DB.Objects[newRoomRef]
		newRoom.Location = instanceRef // interior room belongs to instance
		newRoom.Parent = oldObj.Parent
		newRoom.Zone = oldObj.Zone
		g.AddToContents(instanceRef, newRoomRef) // link into THING's contents chain for lcon()

		// Copy attributes
		for _, attr := range oldObj.Attrs {
			newRoom.Attrs = append(newRoom.Attrs, gamedb.Attribute{
				Number: attr.Number,
				Value:  attr.Value,
			})
		}

		refMap[oldRoom] = newRoomRef
		g.PersistObject(newRoom)
	}

	// 4. Clone exits between interior rooms, remapping source/dest
	for _, oldRoom := range templateRooms {
		oldRoomObj := g.DB.Objects[oldRoom]
		exitRef := oldRoomObj.Exits
		seen := make(map[gamedb.DBRef]bool)
		for exitRef != gamedb.Nothing && !seen[exitRef] {
			seen[exitRef] = true
			exitObj, ok := g.DB.Objects[exitRef]
			if !ok {
				break
			}

			// Remap destination
			newDest := exitObj.Location // default: keep original dest
			if mapped, ok := refMap[exitObj.Location]; ok {
				newDest = mapped
			}

			// Remap source
			newSource := refMap[oldRoom]

			newExitRef := g.CreateExit(exitObj.Name, newSource, newDest, d.Player)
			newExitObj := g.DB.Objects[newExitRef]

			// Copy attributes
			for _, attr := range exitObj.Attrs {
				newExitObj.Attrs = append(newExitObj.Attrs, gamedb.Attribute{
					Number: attr.Number,
					Value:  attr.Value,
				})
			}
			g.PersistObject(newExitObj)

			exitRef = exitObj.Next
		}
	}

	// 5. Place instance at creator's location
	loc := g.PlayerLocation(d.Player)
	instanceObj.Location = loc
	g.AddToContents(loc, instanceRef)
	if locObj, ok := g.DB.Objects[loc]; ok {
		g.PersistObjects(instanceObj, locObj)
	} else {
		g.PersistObject(instanceObj)
	}

	g.Notify(d.Player, fmt.Sprintf("Instance created: %s(#%d) from template %s(#%d) with %d interior room(s).",
		newName, instanceRef, tmplObj.Name, template, len(templateRooms)))
}

// instanceDestroy destroys an instance and all its interior rooms/exits.
func instanceDestroy(g *Game, d *Descriptor, args string) {
	if args == "" {
		g.Notify(d.Player, "Destroy which instance?")
		return
	}

	target := g.MatchObject(d.Player, args)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't see that here.")
		return
	}
	obj, ok := g.DB.Objects[target]
	if !ok {
		g.Notify(d.Player, "No such object.")
		return
	}
	if !obj.HasFlag3(gamedb.Flag3Instance) {
		g.Notify(d.Player, "That is not an instance.")
		return
	}
	if !Controls(g, d.Player, target) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	// Find the exterior location (where the instance THING sits)
	exteriorLoc := obj.Location

	// Find all interior rooms
	var interiorRooms []gamedb.DBRef
	for ref, rObj := range g.DB.Objects {
		if rObj.ObjType() == gamedb.TypeRoom && rObj.Location == target && !rObj.IsGoing() {
			interiorRooms = append(interiorRooms, ref)
		}
	}

	// Eject all players from interior rooms to exterior
	for _, roomRef := range interiorRooms {
		for _, occupant := range g.DB.SafeContents(roomRef) {
			occObj, ok := g.DB.Objects[occupant]
			if !ok {
				continue
			}
			if occObj.ObjType() == gamedb.TypePlayer {
				g.RemoveFromContents(roomRef, occupant)
				occObj.Location = exteriorLoc
				g.AddToContents(exteriorLoc, occupant)
				g.PersistObject(occObj)

				// Show the player their new location
				if descs := g.Conns.GetByPlayer(occupant); len(descs) > 0 {
					descs[0].Send("The instance around you dissolves!")
					g.ShowRoom(descs[0], exteriorLoc)
				}
			}
		}
	}

	// Destroy interior exits and rooms
	for _, roomRef := range interiorRooms {
		roomObj := g.DB.Objects[roomRef]

		// Destroy exits in this room
		exitRef := roomObj.Exits
		seen := make(map[gamedb.DBRef]bool)
		for exitRef != gamedb.Nothing && !seen[exitRef] {
			seen[exitRef] = true
			exitObj, ok := g.DB.Objects[exitRef]
			if !ok {
				break
			}
			nextExit := exitObj.Next
			exitObj.Flags[0] |= gamedb.FlagGoing
			exitObj.Location = gamedb.Nothing
			g.PersistObject(exitObj)
			exitRef = nextExit
		}

		// Destroy the room
		roomObj.Flags[0] |= gamedb.FlagGoing
		roomObj.Location = gamedb.Nothing
		roomObj.Contents = gamedb.Nothing
		roomObj.Exits = gamedb.Nothing
		g.PersistObject(roomObj)
	}

	// Remove instance from its location and destroy it
	if obj.Location != gamedb.Nothing {
		g.RemoveFromContents(obj.Location, target)
	}
	obj.Flags[0] |= gamedb.FlagGoing
	obj.Location = gamedb.Nothing
	obj.Contents = gamedb.Nothing
	g.PersistObject(obj)

	g.Notify(d.Player, fmt.Sprintf("Instance %s(#%d) destroyed with %d interior room(s).",
		obj.Name, target, len(interiorRooms)))
}

// InstanceInteriorRooms returns all interior rooms belonging to an instance THING.
func (g *Game) InstanceInteriorRooms(instance gamedb.DBRef) []gamedb.DBRef {
	var rooms []gamedb.DBRef
	for ref, obj := range g.DB.Objects {
		if obj.ObjType() == gamedb.TypeRoom && obj.Location == instance && !obj.IsGoing() {
			rooms = append(rooms, ref)
		}
	}
	return rooms
}

// InstanceFirstRoom returns the first interior room of an instance (for enter).
// Prefers a room with "entrance" or "entry" in the name, falls back to first found.
func (g *Game) InstanceFirstRoom(instance gamedb.DBRef) gamedb.DBRef {
	rooms := g.InstanceInteriorRooms(instance)
	if len(rooms) == 0 {
		return gamedb.Nothing
	}
	for _, r := range rooms {
		if obj, ok := g.DB.Objects[r]; ok {
			lower := strings.ToLower(obj.Name)
			if strings.Contains(lower, "entrance") || strings.Contains(lower, "entry") {
				return r
			}
		}
	}
	return rooms[0]
}

// MoveInstanceOccupants is called after moving an instance THING to a new location.
// It notifies occupants of the movement and optionally re-renders transparent views.
func (g *Game) MoveInstanceOccupants(instance gamedb.DBRef) {
	obj, ok := g.DB.Objects[instance]
	if !ok || !obj.HasFlag3(gamedb.Flag3Instance) {
		return
	}

	rooms := g.InstanceInteriorRooms(instance)
	for _, roomRef := range rooms {
		for _, occupant := range g.DB.SafeContents(roomRef) {
			occObj, ok := g.DB.Objects[occupant]
			if !ok || occObj.ObjType() != gamedb.TypePlayer {
				continue
			}
			descs := g.Conns.GetByPlayer(occupant)
			if len(descs) > 0 {
				descs[0].Send("You feel the vehicle move.")
			}
		}
	}

	// Fire AMOVE (57) on the instance
	g.QueueAttrAction(instance, instance, 57, nil) // A_AMOVE
}
