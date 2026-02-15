package server

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/crystal-mush/gotinymush/pkg/events"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Comsys manages the channel/communication system.
type Comsys struct {
	mu       sync.RWMutex
	Channels map[string]*gamedb.Channel          // lowercase name -> channel
	Aliases  map[gamedb.DBRef][]*gamedb.ChanAlias // player -> their aliases
}

// NewComsys creates an empty comsys manager.
func NewComsys() *Comsys {
	return &Comsys{
		Channels: make(map[string]*gamedb.Channel),
		Aliases:  make(map[gamedb.DBRef][]*gamedb.ChanAlias),
	}
}

// LoadChannels populates the comsys from parsed data.
func (cs *Comsys) LoadChannels(channels []gamedb.Channel, aliases []gamedb.ChanAlias) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range channels {
		cs.Channels[strings.ToLower(channels[i].Name)] = &channels[i]
	}
	for i := range aliases {
		a := aliases[i]
		cs.Aliases[a.Player] = append(cs.Aliases[a.Player], &a)
	}
	log.Printf("comsys: loaded %d channels, %d aliases (%d players)",
		len(cs.Channels), len(aliases), len(cs.Aliases))
}

// LookupAlias finds a player's channel alias by name (case-insensitive).
func (cs *Comsys) LookupAlias(player gamedb.DBRef, alias string) *gamedb.ChanAlias {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	lower := strings.ToLower(alias)
	for _, ca := range cs.Aliases[player] {
		if strings.ToLower(ca.Alias) == lower {
			return ca
		}
	}
	return nil
}

// GetChannel returns a channel by name (case-insensitive).
func (cs *Comsys) GetChannel(name string) *gamedb.Channel {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.Channels[strings.ToLower(name)]
}

// AllChannels returns a snapshot of all channels.
func (cs *Comsys) AllChannels() []*gamedb.Channel {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make([]*gamedb.Channel, 0, len(cs.Channels))
	for _, ch := range cs.Channels {
		result = append(result, ch)
	}
	return result
}

// PlayerAliases returns all aliases for a player.
func (cs *Comsys) PlayerAliases(player gamedb.DBRef) []*gamedb.ChanAlias {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.Aliases[player]
}

// ChannelListeners returns all aliases for a given channel that are listening.
func (cs *Comsys) ChannelListeners(channelName string) []*gamedb.ChanAlias {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	lower := strings.ToLower(channelName)
	var result []*gamedb.ChanAlias
	for _, aliases := range cs.Aliases {
		for _, ca := range aliases {
			if strings.ToLower(ca.Channel) == lower && ca.IsListening {
				result = append(result, ca)
			}
		}
	}
	return result
}

// ChannelSubscribers returns all aliases for a given channel (listening or not).
func (cs *Comsys) ChannelSubscribers(channelName string) []*gamedb.ChanAlias {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	lower := strings.ToLower(channelName)
	var result []*gamedb.ChanAlias
	for _, aliases := range cs.Aliases {
		for _, ca := range aliases {
			if strings.ToLower(ca.Channel) == lower {
				result = append(result, ca)
			}
		}
	}
	return result
}

// AddAlias adds a channel alias for a player. Returns error if alias already exists.
func (cs *Comsys) AddAlias(ca *gamedb.ChanAlias) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	lower := strings.ToLower(ca.Alias)
	for _, existing := range cs.Aliases[ca.Player] {
		if strings.ToLower(existing.Alias) == lower {
			return fmt.Errorf("alias %q already exists", ca.Alias)
		}
	}
	cs.Aliases[ca.Player] = append(cs.Aliases[ca.Player], ca)
	return nil
}

// RemoveAlias removes a channel alias for a player. Returns the removed alias or nil.
func (cs *Comsys) RemoveAlias(player gamedb.DBRef, alias string) *gamedb.ChanAlias {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	lower := strings.ToLower(alias)
	aliases := cs.Aliases[player]
	for i, ca := range aliases {
		if strings.ToLower(ca.Alias) == lower {
			cs.Aliases[player] = append(aliases[:i], aliases[i+1:]...)
			return ca
		}
	}
	return nil
}

// ClearAliases removes all channel aliases for a player.
func (cs *Comsys) ClearAliases(player gamedb.DBRef) []*gamedb.ChanAlias {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	old := cs.Aliases[player]
	delete(cs.Aliases, player)
	return old
}

// AddChannel adds a new channel.
func (cs *Comsys) AddChannel(ch *gamedb.Channel) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	lower := strings.ToLower(ch.Name)
	if _, exists := cs.Channels[lower]; exists {
		return fmt.Errorf("channel %q already exists", ch.Name)
	}
	cs.Channels[lower] = ch
	return nil
}

// RemoveChannel removes a channel and all its subscriptions.
// Returns the removed aliases so the caller can clean up bbolt.
func (cs *Comsys) RemoveChannel(name string) ([]*gamedb.ChanAlias, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	lower := strings.ToLower(name)
	if _, exists := cs.Channels[lower]; !exists {
		return nil, fmt.Errorf("channel %q not found", name)
	}
	delete(cs.Channels, lower)

	// Remove all aliases for this channel
	var removed []*gamedb.ChanAlias
	for player, aliases := range cs.Aliases {
		var kept []*gamedb.ChanAlias
		for _, ca := range aliases {
			if strings.ToLower(ca.Channel) == lower {
				removed = append(removed, ca)
			} else {
				kept = append(kept, ca)
			}
		}
		if len(kept) == 0 {
			delete(cs.Aliases, player)
		} else {
			cs.Aliases[player] = kept
		}
	}
	return removed, nil
}

// SendToChannel broadcasts a message to all listening, connected players on a channel.
// It emits structured EvChannel events via the event bus.
func (g *Game) SendToChannel(channelName string, sender gamedb.DBRef, msg string) {
	if g.Comsys == nil {
		return
	}
	listeners := g.Comsys.ChannelListeners(channelName)
	// Deduplicate by player — a player may have multiple aliases for the
	// same channel but should only receive each message once.
	seen := make(map[gamedb.DBRef]bool)
	for _, ca := range listeners {
		if seen[ca.Player] {
			continue
		}
		seen[ca.Player] = true
		if g.Conns.IsConnected(ca.Player) {
			g.EmitEvent(ca.Player, channelName, events.Event{
				Type:    events.EvChannel,
				Source:  sender,
				Channel: channelName,
				Text:    msg,
				Data: map[string]any{
					"channel": channelName,
					"message": msg,
				},
			})
		}
	}
}

// ComsysProcessAlias handles a player using a channel alias to send a message.
func (g *Game) ComsysProcessAlias(d *Descriptor, ca *gamedb.ChanAlias, args string) {
	args = strings.TrimSpace(args)
	ch := g.Comsys.GetChannel(ca.Channel)
	if ch == nil {
		d.Send("That channel no longer exists.")
		return
	}

	header := ch.Header
	if header == "" {
		header = fmt.Sprintf("[%s]", ch.Name)
	}

	playerName := g.PlayerName(d.Player)
	if ca.Title != "" {
		playerName = ca.Title + " " + playerName
	}

	// Meta-commands: on, off, who, last
	lower := strings.ToLower(args)
	switch lower {
	case "on":
		ca.IsListening = true
		if g.Store != nil {
			g.Store.PutChanAlias(ca)
		}
		d.Send(fmt.Sprintf("Channel %s is now on.", ch.Name))
		return
	case "off":
		ca.IsListening = false
		if g.Store != nil {
			g.Store.PutChanAlias(ca)
		}
		d.Send(fmt.Sprintf("Channel %s is now off.", ch.Name))
		return
	case "who":
		g.showChannelWho(d, ch)
		return
	}

	if args == "" {
		d.Send(fmt.Sprintf("Channel %s: what do you want to say?", ch.Name))
		return
	}

	if !ca.IsListening {
		d.Send(fmt.Sprintf("You must turn on channel %s first.", ch.Name))
		return
	}

	ch.NumSent++

	// Format the message
	var msg string
	if strings.HasPrefix(args, ":") {
		// Pose
		pose := strings.TrimSpace(args[1:])
		msg = fmt.Sprintf("%s %s %s", header, playerName, pose)
	} else if strings.HasPrefix(args, ";") {
		// Semipose (no space)
		pose := args[1:]
		msg = fmt.Sprintf("%s %s%s", header, playerName, pose)
	} else {
		// Normal say
		msg = fmt.Sprintf("%s %s says, \"%s\"", header, playerName, args)
	}

	g.SendToChannel(ca.Channel, d.Player, msg)
}

// showChannelWho shows who's on a channel.
func (g *Game) showChannelWho(d *Descriptor, ch *gamedb.Channel) {
	subs := g.Comsys.ChannelSubscribers(ch.Name)
	d.Send(fmt.Sprintf("-- %s --", ch.Name))
	d.Send(fmt.Sprintf("%-25s %-10s %-6s", "Name", "Status", "Title"))
	count := 0
	for _, ca := range subs {
		name := g.PlayerName(ca.Player)
		status := "Off"
		if ca.IsListening {
			status = "On"
		}
		online := ""
		if g.Conns.IsConnected(ca.Player) {
			online = " *"
		}
		d.Send(ansiFmtLeft(name, 25) + ansiFmtLeft(status, 10) + ansiFmtLeft(ca.Title, 6) + online)
		count++
	}
	d.Send(fmt.Sprintf("-- %d subscriber(s) --", count))
}
