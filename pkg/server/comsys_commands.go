package server

import (
	"fmt"
	"sort"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// cmdAddcom handles "addcom alias=channel" — subscribe and set alias.
func cmdAddcom(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: addcom <alias>=<channel>")
		return
	}
	alias := strings.TrimSpace(args[:eqIdx])
	chanName := strings.TrimSpace(args[eqIdx+1:])
	if alias == "" || chanName == "" {
		g.Notify(d.Player, "Usage: addcom <alias>=<channel>")
		return
	}

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		g.Notify(d.Player, fmt.Sprintf("Channel %q not found.", chanName))
		return
	}

	// Check if alias already exists for this player
	if existing := g.Comsys.LookupAlias(d.Player, alias); existing != nil {
		g.Notify(d.Player, fmt.Sprintf("You already have an alias %q for channel %s.", alias, existing.Channel))
		return
	}

	ca := &gamedb.ChanAlias{
		Player:      d.Player,
		Channel:     ch.Name,
		Alias:       alias,
		IsListening: true,
	}
	if err := g.Comsys.AddAlias(ca); err != nil {
		g.Notify(d.Player, err.Error())
		return
	}
	if g.Store != nil {
		g.Store.PutChanAlias(ca)
	}
	g.Notify(d.Player, fmt.Sprintf("Channel %s added with alias %s.", ch.Name, alias))
}

// cmdDelcom handles "delcom alias" — remove an alias.
func cmdDelcom(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	alias := strings.TrimSpace(args)
	if alias == "" {
		g.Notify(d.Player, "Usage: delcom <alias>")
		return
	}
	ca := g.Comsys.RemoveAlias(d.Player, alias)
	if ca == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have an alias %q.", alias))
		return
	}
	if g.Store != nil {
		g.Store.DeleteChanAlias(d.Player, alias)
	}
	g.Notify(d.Player, fmt.Sprintf("Alias %s for channel %s removed.", ca.Alias, ca.Channel))
}

// cmdClearcom handles "clearcom" — remove all aliases.
func cmdClearcom(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	old := g.Comsys.ClearAliases(d.Player)
	if len(old) == 0 {
		g.Notify(d.Player, "You have no channel aliases.")
		return
	}
	if g.Store != nil {
		g.Store.DeleteChanAliasesForPlayer(d.Player)
	}
	g.Notify(d.Player, fmt.Sprintf("All %d channel alias(es) removed.", len(old)))
}

// cmdComlist handles "comlist" — list your channel aliases.
func cmdComlist(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	aliases := g.Comsys.PlayerAliases(d.Player)
	if len(aliases) == 0 {
		g.Notify(d.Player, "You have no channel aliases. Use addcom <alias>=<channel> to subscribe.")
		return
	}
	g.Notify(d.Player, fmt.Sprintf("%-12s %-20s %-6s %-20s", "Alias", "Channel", "Status", "Title"))
	g.Notify(d.Player, strings.Repeat("-", 60))
	for _, ca := range aliases {
		status := "Off"
		if ca.IsListening {
			status = "On"
		}
		g.Notify(d.Player, ansiFmtLeft(ca.Alias, 12) + ansiFmtLeft(ca.Channel, 20) + ansiFmtLeft(status, 6) + ansiFmtLeft(ca.Title, 20))
	}
}

// cmdComtitle handles "comtitle alias=title" — set channel title.
func cmdComtitle(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: comtitle <alias>=<title>")
		return
	}
	alias := strings.TrimSpace(args[:eqIdx])
	title := strings.TrimSpace(args[eqIdx+1:])

	ca := g.Comsys.LookupAlias(d.Player, alias)
	if ca == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have an alias %q.", alias))
		return
	}
	ca.Title = title
	if g.Store != nil {
		g.Store.PutChanAlias(ca)
	}
	if title == "" {
		g.Notify(d.Player, fmt.Sprintf("Title cleared on channel %s.", ca.Channel))
	} else {
		g.Notify(d.Player, fmt.Sprintf("Title set to %q on channel %s.", title, ca.Channel))
	}
}

// cmdAllcom handles "allcom on/off/who" — toggle all channels.
func cmdAllcom(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	aliases := g.Comsys.PlayerAliases(d.Player)
	if len(aliases) == 0 {
		g.Notify(d.Player, "You have no channel aliases.")
		return
	}

	switch strings.ToLower(strings.TrimSpace(args)) {
	case "on":
		for _, ca := range aliases {
			ca.IsListening = true
			if g.Store != nil {
				g.Store.PutChanAlias(ca)
			}
		}
		g.Notify(d.Player, fmt.Sprintf("All %d channel(s) turned on.", len(aliases)))
	case "off":
		for _, ca := range aliases {
			ca.IsListening = false
			if g.Store != nil {
				g.Store.PutChanAlias(ca)
			}
		}
		g.Notify(d.Player, fmt.Sprintf("All %d channel(s) turned off.", len(aliases)))
	case "who":
		for _, ca := range aliases {
			ch := g.Comsys.GetChannel(ca.Channel)
			if ch != nil {
				g.showChannelWho(d, ch)
			}
		}
	default:
		g.Notify(d.Player, "Usage: allcom on|off|who")
	}
}

// cmdCcreate handles "@ccreate channel" — create a channel (wizard).
func cmdCcreate(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		g.Notify(d.Player, "Usage: @ccreate <channel name>")
		return
	}
	ch := &gamedb.Channel{
		Name:      name,
		Owner:     d.Player,
		Flags:     gamedb.ChanPublic,
		Mogrifier: gamedb.Nothing,
	}
	if err := g.Comsys.AddChannel(ch); err != nil {
		g.Notify(d.Player, err.Error())
		return
	}
	if g.Store != nil {
		g.Store.PutChannel(ch)
	}
	g.Notify(d.Player, fmt.Sprintf("Channel %s created.", name))
}

// cmdCdestroy handles "@cdestroy channel" — destroy a channel (wizard).
func cmdCdestroy(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		g.Notify(d.Player, "Usage: @cdestroy <channel name>")
		return
	}
	removed, err := g.Comsys.RemoveChannel(name)
	if err != nil {
		g.Notify(d.Player, err.Error())
		return
	}
	if g.Store != nil {
		g.Store.DeleteChannel(name)
		for _, ca := range removed {
			g.Store.DeleteChanAlias(ca.Player, ca.Alias)
		}
	}
	g.Notify(d.Player, fmt.Sprintf("Channel %s destroyed. %d subscription(s) removed.", name, len(removed)))
}

// cmdClist handles "@clist" — list all channels.
func cmdClist(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	channels := g.Comsys.AllChannels()
	if len(channels) == 0 {
		g.Notify(d.Player, "No channels defined.")
		return
	}
	sort.Slice(channels, func(i, j int) bool {
		return strings.ToLower(channels[i].Name) < strings.ToLower(channels[j].Name)
	})
	g.Notify(d.Player, fmt.Sprintf("%-20s %-6s %-8s %s", "Name", "Msgs", "Owner", "Description"))
	g.Notify(d.Player, strings.Repeat("-", 70))
	for _, ch := range channels {
		owner := g.PlayerName(ch.Owner)
		g.Notify(d.Player, ansiFmtLeft(ch.Name, 20) + ansiFmtLeft(fmt.Sprintf("%d", ch.NumSent), 6) + ansiFmtLeft(owner, 8) + ch.Description)
	}
	g.Notify(d.Player, fmt.Sprintf("-- %d channel(s) --", len(channels)))
}

// cmdCwho handles "@cwho channel" — show who's on a channel.
func cmdCwho(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		g.Notify(d.Player, "Usage: @cwho <channel>")
		return
	}
	ch := g.Comsys.GetChannel(name)
	if ch == nil {
		g.Notify(d.Player, fmt.Sprintf("Channel %q not found.", name))
		return
	}
	g.showChannelWho(d, ch)
}

// cmdCboot handles "@cboot channel=player" — boot a player from a channel.
func cmdCboot(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @cboot <channel>=<player>")
		return
	}
	chanName := strings.TrimSpace(args[:eqIdx])
	playerName := strings.TrimSpace(args[eqIdx+1:])

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		g.Notify(d.Player, fmt.Sprintf("Channel %q not found.", chanName))
		return
	}
	target := LookupPlayer(g.DB, playerName)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "I don't recognize that player.")
		return
	}

	// Remove all aliases this player has for this channel
	removed := 0
	aliases := g.Comsys.PlayerAliases(target)
	for _, ca := range aliases {
		if strings.EqualFold(ca.Channel, ch.Name) {
			g.Comsys.RemoveAlias(target, ca.Alias)
			if g.Store != nil {
				g.Store.DeleteChanAlias(target, ca.Alias)
			}
			removed++
		}
	}
	targetObj := g.DB.Objects[target]
	g.Notify(d.Player, fmt.Sprintf("Booted %s from channel %s (%d alias(es) removed).", DisplayName(targetObj.Name), ch.Name, removed))
	g.Conns.SendToPlayer(target, fmt.Sprintf("You have been booted from channel %s.", ch.Name))
}

// cmdCemit handles "@cemit channel=message" — emit to a channel.
func cmdCemit(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @cemit <channel>=<message>")
		return
	}
	chanName := strings.TrimSpace(args[:eqIdx])
	message := strings.TrimSpace(args[eqIdx+1:])

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		g.Notify(d.Player, fmt.Sprintf("Channel %q not found.", chanName))
		return
	}

	header := ch.Header
	if header == "" {
		header = fmt.Sprintf("[%s]", ch.Name)
	}
	msg := fmt.Sprintf("%s %s", header, message)
	g.SendToChannel(ch.Name, d.Player, msg)
}

// cmdCset handles "@cset channel=option" — set channel properties.
func cmdCset(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		g.Notify(d.Player, "Usage: @cset <channel>=<option>")
		g.Notify(d.Player, "Options: description <text>, header <text>, public, private, loud, quiet")
		return
	}
	chanName := strings.TrimSpace(args[:eqIdx])
	option := strings.TrimSpace(args[eqIdx+1:])

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		g.Notify(d.Player, fmt.Sprintf("Channel %q not found.", chanName))
		return
	}

	lower := strings.ToLower(option)
	switch {
	case strings.HasPrefix(lower, "description "):
		ch.Description = strings.TrimSpace(option[12:])
		g.Notify(d.Player, fmt.Sprintf("Channel %s description set.", ch.Name))
	case strings.HasPrefix(lower, "header "):
		ch.Header = strings.TrimSpace(option[7:])
		g.Notify(d.Player, fmt.Sprintf("Channel %s header set.", ch.Name))
	case lower == "public":
		ch.Flags |= gamedb.ChanPublic
		g.Notify(d.Player, fmt.Sprintf("Channel %s set public.", ch.Name))
	case lower == "private":
		ch.Flags &^= gamedb.ChanPublic
		g.Notify(d.Player, fmt.Sprintf("Channel %s set private.", ch.Name))
	case lower == "loud":
		ch.Flags |= gamedb.ChanLoud
		g.Notify(d.Player, fmt.Sprintf("Channel %s set loud.", ch.Name))
	case lower == "quiet":
		ch.Flags &^= gamedb.ChanLoud
		g.Notify(d.Player, fmt.Sprintf("Channel %s set quiet.", ch.Name))
	case strings.HasPrefix(lower, "mogrifier "):
		objStr := strings.TrimSpace(option[10:])
		mogRef := g.ResolveRef(d.Player, objStr)
		if mogRef == gamedb.Nothing {
			mogRef = g.MatchObject(d.Player, objStr)
		}
		if mogRef == gamedb.Nothing {
			g.Notify(d.Player, "I don't see that object.")
			return
		}
		ch.Mogrifier = mogRef
		g.Notify(d.Player, fmt.Sprintf("Channel %s mogrifier set to #%d.", ch.Name, mogRef))
	case lower == "nomogrifier":
		ch.Mogrifier = gamedb.Nothing
		g.Notify(d.Player, fmt.Sprintf("Channel %s mogrifier cleared.", ch.Name))
	default:
		g.Notify(d.Player, "Unknown option. Options: description <text>, header <text>, public, private, loud, quiet, mogrifier <obj>, nomogrifier")
		return
	}
	if g.Store != nil {
		g.Store.PutChannel(ch)
	}
}

// cmdCinfo handles "@cinfo <channel>" — show detailed channel configuration.
func cmdCinfo(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		g.Notify(d.Player, "The channel system is not enabled.")
		return
	}
	chanName := strings.TrimSpace(args)
	if chanName == "" {
		g.Notify(d.Player, "Usage: @cinfo <channel>")
		return
	}
	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		g.Notify(d.Player, fmt.Sprintf("Channel %q not found.", chanName))
		return
	}
	if !Wizard(g, d.Player) && d.Player != ch.Owner {
		g.Notify(d.Player, "Permission denied. You must be the channel owner or a Wizard.")
		return
	}
	owner := g.PlayerName(ch.Owner)
	g.Notify(d.Player, fmt.Sprintf("--- Channel: %s ---", ch.Name))
	g.Notify(d.Player, fmt.Sprintf("  Owner:       %s (#%d)", owner, ch.Owner))
	g.Notify(d.Player, fmt.Sprintf("  Description: %s", ch.Description))
	g.Notify(d.Player, fmt.Sprintf("  Header:      %s", ch.Header))
	g.Notify(d.Player, fmt.Sprintf("  Messages:    %d", ch.NumSent))
	// Flags
	var flags []string
	if ch.Flags&gamedb.ChanPublic != 0 {
		flags = append(flags, "Public")
	} else {
		flags = append(flags, "Private")
	}
	if ch.Flags&gamedb.ChanLoud != 0 {
		flags = append(flags, "Loud")
	}
	if ch.Flags&gamedb.ChanObject != 0 {
		flags = append(flags, "Objects")
	}
	if ch.Flags&gamedb.ChanNoTitles != 0 {
		flags = append(flags, "NoTitles")
	}
	g.Notify(d.Player, fmt.Sprintf("  Flags:       %s", strings.Join(flags, " ")))
	// Locks
	joinLock := ch.JoinLock
	if joinLock == "" {
		joinLock = "(none)"
	}
	transLock := ch.TransLock
	if transLock == "" {
		transLock = "(none)"
	}
	recvLock := ch.RecvLock
	if recvLock == "" {
		recvLock = "(none)"
	}
	g.Notify(d.Player, fmt.Sprintf("  Join Lock:   %s", joinLock))
	g.Notify(d.Player, fmt.Sprintf("  Trans Lock:  %s", transLock))
	g.Notify(d.Player, fmt.Sprintf("  Recv Lock:   %s", recvLock))
	// Mogrifier
	if ch.Mogrifier != gamedb.Nothing {
		mogName := g.PlayerName(ch.Mogrifier)
		g.Notify(d.Player, fmt.Sprintf("  Mogrifier:   %s (#%d)", mogName, ch.Mogrifier))
	} else {
		g.Notify(d.Player, "  Mogrifier:   (none)")
	}
	// Charge
	if ch.Charge > 0 || ch.ChargeCollected > 0 {
		g.Notify(d.Player, fmt.Sprintf("  Charge:      %d (collected: %d)", ch.Charge, ch.ChargeCollected))
	}
	// Subscriber count
	subs := g.Comsys.ChannelSubscribers(ch.Name)
	g.Notify(d.Player, fmt.Sprintf("  Subscribers: %d", len(subs)))
}
