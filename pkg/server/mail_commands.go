package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// cmdMail handles the @mail command with switch routing.
func cmdMail(g *Game, d *Descriptor, args string, switches []string) {
	if g.Mail == nil {
		d.Send("The mail system is not enabled.")
		return
	}

	if len(switches) > 0 {
		sw := strings.ToLower(switches[0])
		switch sw {
		case "send":
			mailSend(g, d, args)
		case "to":
			mailTo(g, d, args)
		case "cc":
			mailCC(g, d, args)
		case "subject", "sub":
			mailSubject(g, d, args)
		case "proof":
			mailProof(g, d)
		case "abort":
			mailAbort(g, d)
		case "read":
			mailRead(g, d, args)
		case "list":
			mailList(g, d)
		case "clear":
			mailClear(g, d, args)
		case "unclear":
			mailUnclear(g, d, args)
		case "purge":
			mailPurge(g, d)
		case "reply":
			mailReply(g, d, args)
		case "forward", "fwd":
			mailForward(g, d, args)
		case "stats":
			mailStats(g, d, args)
		case "safe":
			mailSafe(g, d, args)
		default:
			d.Send(fmt.Sprintf("@mail: Unknown switch /%s.", sw))
		}
		return
	}

	args = strings.TrimSpace(args)
	if args == "" {
		// Bare @mail = list inbox
		mailList(g, d)
		return
	}

	// @mail <num> = read message
	if num, err := strconv.Atoi(args); err == nil {
		mailRead(g, d, strconv.Itoa(num))
		return
	}

	// @mail <player>=<subj>/<body> — quick send
	if idx := strings.Index(args, "="); idx > 0 {
		target := strings.TrimSpace(args[:idx])
		rest := args[idx+1:]

		var subject, body string
		if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
			subject = strings.TrimSpace(rest[:slashIdx])
			body = strings.TrimSpace(rest[slashIdx+1:])
		} else {
			subject = rest
			body = ""
		}

		recipients := parseMailRecipients(g, d, target)
		if len(recipients) == 0 {
			return
		}

		deliverMail(g, d, recipients, nil, subject, body)
		return
	}

	d.Send("Usage: @mail [<num>|<player>=<subj>/<body>]")
}

// cmdMailDash handles the "- <text>" prefix command for draft body appending.
func cmdMailDash(g *Game, d *Descriptor, args string, switches []string) {
	if g.Mail == nil {
		d.Send("The mail system is not enabled.")
		return
	}

	if !g.Mail.HasDraft(d.Player) {
		d.Send("You don't have a mail draft in progress. Use @mail/to <players> first.")
		return
	}

	draft := g.Mail.GetDraft(d.Player)
	if draft.Body.Len() > 0 {
		draft.Body.WriteString("\n")
	}
	draft.Body.WriteString(args)
	d.Send("Text added to mail draft.")
}

// mailTo sets draft recipients.
func mailTo(g *Game, d *Descriptor, args string) {
	if args == "" {
		d.Send("Usage: @mail/to <player list>")
		return
	}
	recipients := parseMailRecipients(g, d, args)
	if len(recipients) == 0 {
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.To = recipients
	names := FormatRecipients(g.DB, recipients)
	d.Send(fmt.Sprintf("Mail recipients set to: %s", names))
	d.Send("Use @mail/subject <text>, then - <text> to compose body, then @mail/send.")
}

// mailCC sets draft CC recipients.
func mailCC(g *Game, d *Descriptor, args string) {
	if args == "" {
		d.Send("Usage: @mail/cc <player list>")
		return
	}
	recipients := parseMailRecipients(g, d, args)
	if len(recipients) == 0 {
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.CC = recipients
	names := FormatRecipients(g.DB, recipients)
	d.Send(fmt.Sprintf("Mail CC set to: %s", names))
}

// mailSubject sets draft subject.
func mailSubject(g *Game, d *Descriptor, args string) {
	if args == "" {
		d.Send("Usage: @mail/subject <text>")
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.Subject = args
	d.Send(fmt.Sprintf("Mail subject set to: %s", args))
}

// mailProof previews the current draft.
func mailProof(g *Game, d *Descriptor) {
	if !g.Mail.HasDraft(d.Player) {
		d.Send("You have no mail draft in progress.")
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	d.Send("--- Mail Draft ---")
	d.Send(fmt.Sprintf("To: %s", FormatRecipients(g.DB, draft.To)))
	if len(draft.CC) > 0 {
		d.Send(fmt.Sprintf("CC: %s", FormatRecipients(g.DB, draft.CC)))
	}
	d.Send(fmt.Sprintf("Subject: %s", draft.Subject))
	d.Send("---")
	body := draft.Body.String()
	if body == "" {
		d.Send("(empty body)")
	} else {
		d.Send(body)
	}
	d.Send("--- End Draft ---")
}

// mailAbort discards the current draft.
func mailAbort(g *Game, d *Descriptor) {
	if !g.Mail.HasDraft(d.Player) {
		d.Send("You have no mail draft to abort.")
		return
	}
	g.Mail.ClearDraft(d.Player)
	d.Send("Mail draft discarded.")
}

// mailSend sends the current draft.
func mailSend(g *Game, d *Descriptor, args string) {
	if !g.Mail.HasDraft(d.Player) {
		d.Send("You have no mail draft to send. Use @mail/to <players> first.")
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	if len(draft.To) == 0 {
		d.Send("Your draft has no recipients. Use @mail/to <players>.")
		return
	}
	if draft.Subject == "" {
		d.Send("Your draft has no subject. Use @mail/subject <text>.")
		return
	}

	deliverMail(g, d, draft.To, draft.CC, draft.Subject, draft.Body.String())
	g.Mail.ClearDraft(d.Player)
}

// mailRead reads a message by number.
func mailRead(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		d.Send("Usage: @mail/read <number>")
		return
	}

	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		d.Send(fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	g.Mail.MarkRead(d.Player, num)
	persistMailMessage(g, d.Player, msg)

	d.Send(fmt.Sprintf("--- Message %d ---", msg.ID))
	d.Send(fmt.Sprintf("From: %s  Date: %s", playerName(g.DB, msg.From), msg.Time.Format("Mon Jan 02 15:04 2006")))
	d.Send(fmt.Sprintf("To: %s", FormatRecipients(g.DB, msg.To)))
	if len(msg.CC) > 0 {
		d.Send(fmt.Sprintf("CC: %s", FormatRecipients(g.DB, msg.CC)))
	}
	d.Send(fmt.Sprintf("Subject: %s", msg.Subject))
	d.Send("---")
	if msg.Body != "" {
		d.Send(msg.Body)
	}
	d.Send("--- End Message ---")
}

// mailList lists inbox messages.
func mailList(g *Game, d *Descriptor) {
	inbox := g.Mail.GetInbox(d.Player)
	if len(inbox) == 0 {
		d.Send("You have no mail.")
		return
	}

	d.Send(fmt.Sprintf("--- Mailbox for %s (%d messages) ---", playerName(g.DB, d.Player), len(inbox)))
	d.Send(fmt.Sprintf("%-4s %-5s %-16s %-20s %s", "#", "Flags", "From", "Date", "Subject"))
	for _, msg := range inbox {
		from := playerName(g.DB, msg.From)
		if len(from) > 16 {
			from = from[:16]
		}
		subj := msg.Subject
		if len(subj) > 30 {
			subj = subj[:27] + "..."
		}
		d.Send(fmt.Sprintf("%-4d %-5s %-16s %-20s %s",
			msg.ID,
			FormatMailFlags(msg),
			from,
			msg.Time.Format("Jan 02 15:04"),
			subj))
	}
	d.Send("---")
}

// mailClear marks a message for deletion.
func mailClear(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		d.Send("Usage: @mail/clear <number>")
		return
	}
	if g.Mail.MarkCleared(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		d.Send(fmt.Sprintf("Message %d marked for clearing.", num))
	} else {
		msg := g.Mail.GetMessage(d.Player, num)
		if msg != nil && msg.Flags&gamedb.MailSafe != 0 {
			d.Send(fmt.Sprintf("Message %d is marked safe and cannot be cleared.", num))
		} else {
			d.Send(fmt.Sprintf("You don't have a message #%d.", num))
		}
	}
}

// mailUnclear removes the cleared flag.
func mailUnclear(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		d.Send("Usage: @mail/unclear <number>")
		return
	}
	if g.Mail.MarkUncleared(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		d.Send(fmt.Sprintf("Message %d uncleared.", num))
	} else {
		d.Send(fmt.Sprintf("You don't have a message #%d.", num))
	}
}

// mailPurge deletes all cleared messages.
func mailPurge(g *Game, d *Descriptor) {
	purged := g.Mail.PurgeCleared(d.Player)
	if len(purged) == 0 {
		d.Send("You have no cleared messages to purge.")
		return
	}
	if g.Store != nil {
		g.Store.DeleteMailMessages(d.Player, purged)
	}
	d.Send(fmt.Sprintf("%d message(s) purged.", len(purged)))
}

// mailReply starts a reply to a message.
func mailReply(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		d.Send("Usage: @mail/reply <number>")
		return
	}
	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		d.Send(fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	draft := g.Mail.GetDraft(d.Player)
	draft.To = []gamedb.DBRef{msg.From}
	subj := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subj), "re:") {
		subj = "Re: " + subj
	}
	draft.Subject = subj
	d.Send(fmt.Sprintf("Replying to %s. Subject: %s", playerName(g.DB, msg.From), subj))
	d.Send("Use - <text> to compose body, then @mail/send.")
}

// mailForward forwards a message to other players.
func mailForward(g *Game, d *Descriptor, args string) {
	idx := strings.Index(args, "=")
	if idx < 0 {
		d.Send("Usage: @mail/forward <number>=<player list>")
		return
	}
	numStr := strings.TrimSpace(args[:idx])
	targetStr := strings.TrimSpace(args[idx+1:])

	num, err := strconv.Atoi(numStr)
	if err != nil {
		d.Send("Usage: @mail/forward <number>=<player list>")
		return
	}
	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		d.Send(fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	recipients := parseMailRecipients(g, d, targetStr)
	if len(recipients) == 0 {
		return
	}

	subj := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subj), "fwd:") {
		subj = "Fwd: " + subj
	}
	body := fmt.Sprintf("--- Forwarded message from %s ---\n%s\n--- End forwarded message ---",
		playerName(g.DB, msg.From), msg.Body)

	deliverMail(g, d, recipients, nil, subj, body)
}

// mailStats shows mail statistics.
func mailStats(g *Game, d *Descriptor, args string) {
	player := d.Player
	if args != "" {
		// Wizards can see stats for other players
		if !Wizard(g, d.Player) {
			d.Send("Permission denied.")
			return
		}
		target := LookupPlayer(g.DB, strings.TrimSpace(args))
		if target == gamedb.Nothing {
			d.Send("No such player.")
			return
		}
		player = target
	}

	total, unread, cleared := g.Mail.CountMessages(player)
	d.Send(fmt.Sprintf("Mail stats for %s: %d total, %d unread, %d cleared.",
		playerName(g.DB, player), total, unread, cleared))
}

// mailSafe marks a message as safe (protected from purge).
func mailSafe(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		d.Send("Usage: @mail/safe <number>")
		return
	}
	if g.Mail.MarkSafe(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		d.Send(fmt.Sprintf("Message %d marked safe.", num))
	} else {
		d.Send(fmt.Sprintf("You don't have a message #%d.", num))
	}
}

// --- Helpers ---

// parseMailRecipients parses a comma/space separated list of player names.
func parseMailRecipients(g *Game, d *Descriptor, input string) []gamedb.DBRef {
	// Split on commas and spaces
	input = strings.ReplaceAll(input, ",", " ")
	parts := strings.Fields(input)
	var result []gamedb.DBRef
	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		ref := LookupPlayer(g.DB, name)
		if ref == gamedb.Nothing {
			d.Send(fmt.Sprintf("No such player: %s", name))
			return nil
		}
		result = append(result, ref)
	}
	if len(result) == 0 {
		d.Send("No valid recipients specified.")
	}
	return result
}

// deliverMail sends a message and handles persistence + notifications.
func deliverMail(g *Game, d *Descriptor, to, cc []gamedb.DBRef, subject, body string) {
	delivered := g.Mail.SendMessage(d.Player, to, cc, subject, body)

	// Persist all delivered messages
	if g.Store != nil {
		for player, msg := range delivered {
			g.Store.PutMailMessage(player, msg)
		}
	}

	// Notify online recipients
	for player := range delivered {
		if player == d.Player {
			continue
		}
		for _, desc := range g.Conns.GetByPlayer(player) {
			desc.Send(fmt.Sprintf("You have new mail from %s.", playerName(g.DB, d.Player)))
		}
	}

	names := FormatRecipients(g.DB, to)
	d.Send(fmt.Sprintf("Mail sent to %s.", names))
}

// persistMailMessage writes a single message update to bbolt.
func persistMailMessage(g *Game, player gamedb.DBRef, msg *gamedb.MailMessage) {
	if g.Store != nil && msg != nil {
		g.Store.PutMailMessage(player, msg)
	}
}

// playerName returns a player's name or "#<ref>" if not found.
func playerName(db *gamedb.Database, ref gamedb.DBRef) string {
	if obj, ok := db.Objects[ref]; ok {
		return obj.Name
	}
	return fmt.Sprintf("#%d", ref)
}
