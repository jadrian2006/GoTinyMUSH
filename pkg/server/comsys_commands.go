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
		d.Send("The channel system is not enabled.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: addcom <alias>=<channel>")
		return
	}
	alias := strings.TrimSpace(args[:eqIdx])
	chanName := strings.TrimSpace(args[eqIdx+1:])
	if alias == "" || chanName == "" {
		d.Send("Usage: addcom <alias>=<channel>")
		return
	}

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		d.Send(fmt.Sprintf("Channel %q not found.", chanName))
		return
	}

	// Check if alias already exists for this player
	if existing := g.Comsys.LookupAlias(d.Player, alias); existing != nil {
		d.Send(fmt.Sprintf("You already have an alias %q for channel %s.", alias, existing.Channel))
		return
	}

	ca := &gamedb.ChanAlias{
		Player:      d.Player,
		Channel:     ch.Name,
		Alias:       alias,
		IsListening: true,
	}
	if err := g.Comsys.AddAlias(ca); err != nil {
		d.Send(err.Error())
		return
	}
	if g.Store != nil {
		g.Store.PutChanAlias(ca)
	}
	d.Send(fmt.Sprintf("Channel %s added with alias %s.", ch.Name, alias))
}

// cmdDelcom handles "delcom alias" — remove an alias.
func cmdDelcom(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	alias := strings.TrimSpace(args)
	if alias == "" {
		d.Send("Usage: delcom <alias>")
		return
	}
	ca := g.Comsys.RemoveAlias(d.Player, alias)
	if ca == nil {
		d.Send(fmt.Sprintf("You don't have an alias %q.", alias))
		return
	}
	if g.Store != nil {
		g.Store.DeleteChanAlias(d.Player, alias)
	}
	d.Send(fmt.Sprintf("Alias %s for channel %s removed.", ca.Alias, ca.Channel))
}

// cmdClearcom handles "clearcom" — remove all aliases.
func cmdClearcom(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	old := g.Comsys.ClearAliases(d.Player)
	if len(old) == 0 {
		d.Send("You have no channel aliases.")
		return
	}
	if g.Store != nil {
		g.Store.DeleteChanAliasesForPlayer(d.Player)
	}
	d.Send(fmt.Sprintf("All %d channel alias(es) removed.", len(old)))
}

// cmdComlist handles "comlist" — list your channel aliases.
func cmdComlist(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	aliases := g.Comsys.PlayerAliases(d.Player)
	if len(aliases) == 0 {
		d.Send("You have no channel aliases. Use addcom <alias>=<channel> to subscribe.")
		return
	}
	d.Send(fmt.Sprintf("%-12s %-20s %-6s %-20s", "Alias", "Channel", "Status", "Title"))
	d.Send(strings.Repeat("-", 60))
	for _, ca := range aliases {
		status := "Off"
		if ca.IsListening {
			status = "On"
		}
		d.Send(ansiFmtLeft(ca.Alias, 12) + ansiFmtLeft(ca.Channel, 20) + ansiFmtLeft(status, 6) + ansiFmtLeft(ca.Title, 20))
	}
}

// cmdComtitle handles "comtitle alias=title" — set channel title.
func cmdComtitle(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: comtitle <alias>=<title>")
		return
	}
	alias := strings.TrimSpace(args[:eqIdx])
	title := strings.TrimSpace(args[eqIdx+1:])

	ca := g.Comsys.LookupAlias(d.Player, alias)
	if ca == nil {
		d.Send(fmt.Sprintf("You don't have an alias %q.", alias))
		return
	}
	ca.Title = title
	if g.Store != nil {
		g.Store.PutChanAlias(ca)
	}
	if title == "" {
		d.Send(fmt.Sprintf("Title cleared on channel %s.", ca.Channel))
	} else {
		d.Send(fmt.Sprintf("Title set to %q on channel %s.", title, ca.Channel))
	}
}

// cmdAllcom handles "allcom on/off/who" — toggle all channels.
func cmdAllcom(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	aliases := g.Comsys.PlayerAliases(d.Player)
	if len(aliases) == 0 {
		d.Send("You have no channel aliases.")
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
		d.Send(fmt.Sprintf("All %d channel(s) turned on.", len(aliases)))
	case "off":
		for _, ca := range aliases {
			ca.IsListening = false
			if g.Store != nil {
				g.Store.PutChanAlias(ca)
			}
		}
		d.Send(fmt.Sprintf("All %d channel(s) turned off.", len(aliases)))
	case "who":
		for _, ca := range aliases {
			ch := g.Comsys.GetChannel(ca.Channel)
			if ch != nil {
				g.showChannelWho(d, ch)
			}
		}
	default:
		d.Send("Usage: allcom on|off|who")
	}
}

// cmdCcreate handles "@ccreate channel" — create a channel (wizard).
func cmdCcreate(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		d.Send("Usage: @ccreate <channel name>")
		return
	}
	ch := &gamedb.Channel{
		Name:  name,
		Owner: d.Player,
		Flags: gamedb.ChanPublic,
	}
	if err := g.Comsys.AddChannel(ch); err != nil {
		d.Send(err.Error())
		return
	}
	if g.Store != nil {
		g.Store.PutChannel(ch)
	}
	d.Send(fmt.Sprintf("Channel %s created.", name))
}

// cmdCdestroy handles "@cdestroy channel" — destroy a channel (wizard).
func cmdCdestroy(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		d.Send("Usage: @cdestroy <channel name>")
		return
	}
	removed, err := g.Comsys.RemoveChannel(name)
	if err != nil {
		d.Send(err.Error())
		return
	}
	if g.Store != nil {
		g.Store.DeleteChannel(name)
		for _, ca := range removed {
			g.Store.DeleteChanAlias(ca.Player, ca.Alias)
		}
	}
	d.Send(fmt.Sprintf("Channel %s destroyed. %d subscription(s) removed.", name, len(removed)))
}

// cmdClist handles "@clist" — list all channels.
func cmdClist(g *Game, d *Descriptor, _ string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	channels := g.Comsys.AllChannels()
	if len(channels) == 0 {
		d.Send("No channels defined.")
		return
	}
	sort.Slice(channels, func(i, j int) bool {
		return strings.ToLower(channels[i].Name) < strings.ToLower(channels[j].Name)
	})
	d.Send(fmt.Sprintf("%-20s %-6s %-8s %s", "Name", "Msgs", "Owner", "Description"))
	d.Send(strings.Repeat("-", 70))
	for _, ch := range channels {
		owner := g.PlayerName(ch.Owner)
		d.Send(ansiFmtLeft(ch.Name, 20) + ansiFmtLeft(fmt.Sprintf("%d", ch.NumSent), 6) + ansiFmtLeft(owner, 8) + ch.Description)
	}
	d.Send(fmt.Sprintf("-- %d channel(s) --", len(channels)))
}

// cmdCwho handles "@cwho channel" — show who's on a channel.
func cmdCwho(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	name := strings.TrimSpace(args)
	if name == "" {
		d.Send("Usage: @cwho <channel>")
		return
	}
	ch := g.Comsys.GetChannel(name)
	if ch == nil {
		d.Send(fmt.Sprintf("Channel %q not found.", name))
		return
	}
	g.showChannelWho(d, ch)
}

// cmdCboot handles "@cboot channel=player" — boot a player from a channel.
func cmdCboot(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @cboot <channel>=<player>")
		return
	}
	chanName := strings.TrimSpace(args[:eqIdx])
	playerName := strings.TrimSpace(args[eqIdx+1:])

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		d.Send(fmt.Sprintf("Channel %q not found.", chanName))
		return
	}
	target := LookupPlayer(g.DB, playerName)
	if target == gamedb.Nothing {
		d.Send("I don't recognize that player.")
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
	d.Send(fmt.Sprintf("Booted %s from channel %s (%d alias(es) removed).", DisplayName(targetObj.Name), ch.Name, removed))
	g.Conns.SendToPlayer(target, fmt.Sprintf("You have been booted from channel %s.", ch.Name))
}

// cmdCemit handles "@cemit channel=message" — emit to a channel.
func cmdCemit(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @cemit <channel>=<message>")
		return
	}
	chanName := strings.TrimSpace(args[:eqIdx])
	message := strings.TrimSpace(args[eqIdx+1:])

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		d.Send(fmt.Sprintf("Channel %q not found.", chanName))
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
		d.Send("The channel system is not enabled.")
		return
	}
	if !Wizard(g, d.Player) {
		d.Send("Permission denied.")
		return
	}
	eqIdx := strings.IndexByte(args, '=')
	if eqIdx < 0 {
		d.Send("Usage: @cset <channel>=<option>")
		d.Send("Options: description <text>, header <text>, public, private, loud, quiet")
		return
	}
	chanName := strings.TrimSpace(args[:eqIdx])
	option := strings.TrimSpace(args[eqIdx+1:])

	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		d.Send(fmt.Sprintf("Channel %q not found.", chanName))
		return
	}

	lower := strings.ToLower(option)
	switch {
	case strings.HasPrefix(lower, "description "):
		ch.Description = strings.TrimSpace(option[12:])
		d.Send(fmt.Sprintf("Channel %s description set.", ch.Name))
	case strings.HasPrefix(lower, "header "):
		ch.Header = strings.TrimSpace(option[7:])
		d.Send(fmt.Sprintf("Channel %s header set.", ch.Name))
	case lower == "public":
		ch.Flags |= gamedb.ChanPublic
		d.Send(fmt.Sprintf("Channel %s set public.", ch.Name))
	case lower == "private":
		ch.Flags &^= gamedb.ChanPublic
		d.Send(fmt.Sprintf("Channel %s set private.", ch.Name))
	case lower == "loud":
		ch.Flags |= gamedb.ChanLoud
		d.Send(fmt.Sprintf("Channel %s set loud.", ch.Name))
	case lower == "quiet":
		ch.Flags &^= gamedb.ChanLoud
		d.Send(fmt.Sprintf("Channel %s set quiet.", ch.Name))
	default:
		d.Send("Unknown option. Options: description <text>, header <text>, public, private, loud, quiet")
		return
	}
	if g.Store != nil {
		g.Store.PutChannel(ch)
	}
}

// cmdCinfo handles "@cinfo <channel>" — show detailed channel configuration.
func cmdCinfo(g *Game, d *Descriptor, args string, _ []string) {
	if g.Comsys == nil {
		d.Send("The channel system is not enabled.")
		return
	}
	chanName := strings.TrimSpace(args)
	if chanName == "" {
		d.Send("Usage: @cinfo <channel>")
		return
	}
	ch := g.Comsys.GetChannel(chanName)
	if ch == nil {
		d.Send(fmt.Sprintf("Channel %q not found.", chanName))
		return
	}
	if !Wizard(g, d.Player) && d.Player != ch.Owner {
		d.Send("Permission denied. You must be the channel owner or a Wizard.")
		return
	}
	owner := g.PlayerName(ch.Owner)
	d.Send(fmt.Sprintf("--- Channel: %s ---", ch.Name))
	d.Send(fmt.Sprintf("  Owner:       %s (#%d)", owner, ch.Owner))
	d.Send(fmt.Sprintf("  Description: %s", ch.Description))
	d.Send(fmt.Sprintf("  Header:      %s", ch.Header))
	d.Send(fmt.Sprintf("  Messages:    %d", ch.NumSent))
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
	d.Send(fmt.Sprintf("  Flags:       %s", strings.Join(flags, " ")))
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
	d.Send(fmt.Sprintf("  Join Lock:   %s", joinLock))
	d.Send(fmt.Sprintf("  Trans Lock:  %s", transLock))
	d.Send(fmt.Sprintf("  Recv Lock:   %s", recvLock))
	// Charge
	if ch.Charge > 0 || ch.ChargeCollected > 0 {
		d.Send(fmt.Sprintf("  Charge:      %d (collected: %d)", ch.Charge, ch.ChargeCollected))
	}
	// Subscriber count
	subs := g.Comsys.ChannelSubscribers(ch.Name)
	d.Send(fmt.Sprintf("  Subscribers: %d", len(subs)))
}
