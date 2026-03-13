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
		// Fall through to softcode $-commands (e.g. BrandyMail)
		input := "@mail"
		if len(switches) > 0 {
			input += "/" + strings.Join(switches, "/")
		}
		if args != "" {
			input += " " + args
		}
		g.MatchDollarCommands(d.Player, d.Player, input)
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
		case "bcc":
			mailBCC(g, d, args)
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
		case "replyall":
			mailReplyAll(g, d, args)
		case "forward", "fwd":
			mailForward(g, d, args)
		case "stats":
			mailStats(g, d, args)
		case "dstats":
			mailDstats(g, d, args)
		case "fstats":
			mailFstats(g, d, args)
		case "safe":
			mailSafe(g, d, args)
		case "tag":
			mailTag(g, d, args)
		case "untag":
			mailUntag(g, d, args)
		case "urgent":
			mailUrgentFlag(g, d, args)
		case "folder":
			mailFolder(g, d, args)
		case "file":
			mailFile(g, d, args)
		case "edit":
			mailEdit(g, d, args)
		case "quick":
			mailQuick(g, d, args)
		case "review":
			mailReview(g, d, args)
		case "retract":
			mailRetract(g, d, args)
		case "quote":
			mailQuote(g, d, args)
		case "nuke":
			mailNuke(g, d)
		case "debug":
			mailDebug(g, d, args)
		default:
			g.Notify(d.Player, fmt.Sprintf("@mail: Unknown switch /%s.", sw))
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

	// @mail <player>=<subj>/<body> — quick send (Go syntax)
	// Also starts compose mode if no / in the rhs (C compat: @mail player=subject)
	if idx := strings.Index(args, "="); idx > 0 {
		target := strings.TrimSpace(args[:idx])
		rest := args[idx+1:]

		if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
			// Quick send with body
			subject := strings.TrimSpace(rest[:slashIdx])
			body := strings.TrimSpace(rest[slashIdx+1:])
			recipients := parseMailRecipients(g, d, target)
			if len(recipients) == 0 {
				return
			}
			deliverMail(g, d, recipients, nil, nil, subject, body, 0)
		} else {
			// C compat: @mail player=subject starts compose mode
			subject := strings.TrimSpace(rest)
			recipients := parseMailRecipients(g, d, target)
			if len(recipients) == 0 {
				return
			}
			draft := g.Mail.GetDraft(d.Player)
			draft.To = recipients
			draft.Subject = subject
			names := FormatRecipients(g.DB, recipients)
			g.Notify(d.Player, fmt.Sprintf("MAIL: You are sending mail to '%s'. Subject: %s", names, subject))
			g.Notify(d.Player, "Use - <text> to compose body, then @mail/send.")
		}
		return
	}

	g.Notify(d.Player, "Usage: @mail [<num>|<player>=<subj>/<body>]")
}

// cmdMailDash handles the "- <text>" prefix command for draft body appending.
func cmdMailDash(g *Game, d *Descriptor, args string, switches []string) {
	if g.Mail == nil {
		input := "- " + args
		g.MatchDollarCommands(d.Player, d.Player, input)
		return
	}

	if !g.Mail.HasDraft(d.Player) {
		g.Notify(d.Player, "You don't have a mail draft in progress. Use @mail/to <players> first.")
		return
	}

	draft := g.Mail.GetDraft(d.Player)
	if draft.Body.Len() > 0 {
		draft.Body.WriteString("\n")
	}
	draft.Body.WriteString(args)
	g.Notify(d.Player, fmt.Sprintf("Text added to mail draft. (%d/8000 characters)", draft.Body.Len()))
}

// cmdMailTilde handles the "~ <text>" prefix command for draft body prepending.
func cmdMailTilde(g *Game, d *Descriptor, args string, switches []string) {
	if g.Mail == nil {
		input := "~ " + args
		g.MatchDollarCommands(d.Player, d.Player, input)
		return
	}

	if !g.Mail.HasDraft(d.Player) {
		g.Notify(d.Player, "You don't have a mail draft in progress. Use @mail/to <players> first.")
		return
	}

	draft := g.Mail.GetDraft(d.Player)
	existing := draft.Body.String()
	draft.Body.Reset()
	draft.Body.WriteString(args)
	if existing != "" {
		draft.Body.WriteString("\n")
		draft.Body.WriteString(existing)
	}
	g.Notify(d.Player, fmt.Sprintf("Text prepended to mail draft. (%d/8000 characters)", draft.Body.Len()))
}

// --- Compose commands ---

// mailTo sets draft recipients.
func mailTo(g *Game, d *Descriptor, args string) {
	if args == "" {
		g.Notify(d.Player, "Usage: @mail/to <player list>")
		return
	}
	recipients := parseMailRecipients(g, d, args)
	if len(recipients) == 0 {
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.To = recipients
	names := FormatRecipients(g.DB, recipients)
	g.Notify(d.Player, fmt.Sprintf("Mail recipients set to: %s", names))
	g.Notify(d.Player, "Use @mail/subject <text>, then - <text> to compose body, then @mail/send.")
}

// mailCC sets draft CC recipients.
func mailCC(g *Game, d *Descriptor, args string) {
	if args == "" {
		g.Notify(d.Player, "Usage: @mail/cc <player list>")
		return
	}
	recipients := parseMailRecipients(g, d, args)
	if len(recipients) == 0 {
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.CC = recipients
	names := FormatRecipients(g.DB, recipients)
	g.Notify(d.Player, fmt.Sprintf("Mail CC set to: %s", names))
}

// mailBCC sets draft BCC recipients (not visible to other recipients).
func mailBCC(g *Game, d *Descriptor, args string) {
	if args == "" {
		g.Notify(d.Player, "Usage: @mail/bcc <player list>")
		return
	}
	recipients := parseMailRecipients(g, d, args)
	if len(recipients) == 0 {
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.BCC = recipients
	names := FormatRecipients(g.DB, recipients)
	g.Notify(d.Player, fmt.Sprintf("Mail BCC set to: %s", names))
}

// mailSubject sets draft subject.
func mailSubject(g *Game, d *Descriptor, args string) {
	if args == "" {
		g.Notify(d.Player, "Usage: @mail/subject <text>")
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	draft.Subject = args
	g.Notify(d.Player, fmt.Sprintf("Mail subject set to: %s", args))
}

// mailProof previews the current draft.
func mailProof(g *Game, d *Descriptor) {
	if !g.Mail.HasDraft(d.Player) {
		g.Notify(d.Player, "MAIL: No message in progress.")
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	g.Notify(d.Player, "--- Mail Draft ---")
	g.Notify(d.Player, fmt.Sprintf("To: %s", FormatRecipients(g.DB, draft.To)))
	if len(draft.CC) > 0 {
		g.Notify(d.Player, fmt.Sprintf("CC: %s", FormatRecipients(g.DB, draft.CC)))
	}
	if len(draft.BCC) > 0 {
		g.Notify(d.Player, fmt.Sprintf("BCC: %s", FormatRecipients(g.DB, draft.BCC)))
	}
	g.Notify(d.Player, fmt.Sprintf("Subject: %s", draft.Subject))
	g.Notify(d.Player, "---")
	body := draft.Body.String()
	if body == "" {
		g.Notify(d.Player, "(empty body)")
	} else {
		g.Notify(d.Player, body)
	}
	g.Notify(d.Player, "--- End Draft ---")
}

// mailAbort discards the current draft.
func mailAbort(g *Game, d *Descriptor) {
	if !g.Mail.HasDraft(d.Player) {
		g.Notify(d.Player, "You have no mail draft to abort.")
		return
	}
	g.Mail.ClearDraft(d.Player)
	g.Notify(d.Player, "Mail draft discarded.")
}

// mailSend sends the current draft.
func mailSend(g *Game, d *Descriptor, args string) {
	if !g.Mail.HasDraft(d.Player) {
		g.Notify(d.Player, "You have no mail draft to send. Use @mail/to <players> first.")
		return
	}
	draft := g.Mail.GetDraft(d.Player)
	if len(draft.To) == 0 {
		g.Notify(d.Player, "Your draft has no recipients. Use @mail/to <players>.")
		return
	}
	if draft.Subject == "" {
		g.Notify(d.Player, "Your draft has no subject. Use @mail/subject <text>.")
		return
	}

	deliverMail(g, d, draft.To, draft.CC, draft.BCC, draft.Subject, draft.Body.String(), 0)
	g.Mail.ClearDraft(d.Player)
}

// mailEdit does search/replace on the current draft body.
func mailEdit(g *Game, d *Descriptor, args string) {
	if !g.Mail.HasDraft(d.Player) {
		g.Notify(d.Player, "MAIL: No message in progress.")
		return
	}
	// args format: old=new
	idx := strings.Index(args, "=")
	if idx < 0 {
		g.Notify(d.Player, "Usage: @mail/edit <old text>=<new text>")
		return
	}
	from := args[:idx]
	to := args[idx+1:]

	draft := g.Mail.GetDraft(d.Player)
	body := draft.Body.String()
	if !strings.Contains(body, from) {
		g.Notify(d.Player, "MAIL: Text not found in message.")
		return
	}
	body = strings.Replace(body, from, to, 1)
	draft.Body.Reset()
	draft.Body.WriteString(body)
	g.Notify(d.Player, "Text edited.")
}

// mailQuick sends a quick message: @mail/quick player/subject=body (C compat syntax)
func mailQuick(g *Game, d *Descriptor, args string) {
	idx := strings.Index(args, "=")
	if idx < 0 {
		g.Notify(d.Player, "Usage: @mail/quick <player>/<subject>=<body>")
		return
	}
	lhs := strings.TrimSpace(args[:idx])
	body := strings.TrimSpace(args[idx+1:])

	slashIdx := strings.Index(lhs, "/")
	if slashIdx < 0 {
		g.Notify(d.Player, "MAIL: No subject.")
		return
	}
	targetStr := strings.TrimSpace(lhs[:slashIdx])
	subject := strings.TrimSpace(lhs[slashIdx+1:])

	if body == "" {
		g.Notify(d.Player, "MAIL: No message.")
		return
	}

	recipients := parseMailRecipients(g, d, targetStr)
	if len(recipients) == 0 {
		return
	}
	deliverMail(g, d, recipients, nil, nil, subject, body, 0)
}

// --- Reading commands ---

// mailRead reads a message by number.
func mailRead(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/read <number>")
		return
	}

	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	g.Mail.MarkRead(d.Player, num)
	persistMailMessage(g, d.Player, msg)

	g.Notify(d.Player, fmt.Sprintf("--- Message %d ---", msg.ID))
	g.Notify(d.Player, fmt.Sprintf("From: %s  Date: %s", playerName(g.DB, msg.From), msg.Time.Format("Mon Jan 02 15:04 2006")))
	g.Notify(d.Player, fmt.Sprintf("To: %s", FormatRecipients(g.DB, msg.To)))
	if len(msg.CC) > 0 {
		g.Notify(d.Player, fmt.Sprintf("CC: %s", FormatRecipients(g.DB, msg.CC)))
	}
	g.Notify(d.Player, fmt.Sprintf("Subject: %s", msg.Subject))
	g.Notify(d.Player, "---")
	if msg.Body != "" {
		g.Notify(d.Player, msg.Body)
	}
	g.Notify(d.Player, "--- End Message ---")
}

// mailList lists inbox messages.
func mailList(g *Game, d *Descriptor) {
	inbox := g.Mail.GetInbox(d.Player)
	if len(inbox) == 0 {
		g.Notify(d.Player, "You have no mail.")
		return
	}

	g.Notify(d.Player, fmt.Sprintf("--- Mailbox for %s (%d messages) ---", playerName(g.DB, d.Player), len(inbox)))
	g.Notify(d.Player, fmt.Sprintf("%-4s %-5s %-16s %-20s %s", "#", "Flags", "From", "Date", "Subject"))
	for _, msg := range inbox {
		from := playerName(g.DB, msg.From)
		if len(from) > 16 {
			from = from[:16]
		}
		subj := msg.Subject
		if len(subj) > 30 {
			subj = subj[:27] + "..."
		}
		g.Notify(d.Player, fmt.Sprintf("%-4d %-5s %-16s %-20s %s",
			msg.ID,
			FormatMailFlags(msg),
			from,
			msg.Time.Format("Jan 02 15:04"),
			subj))
	}
	g.Notify(d.Player, "---")
}

// --- Flag operations ---

// mailClear marks a message for deletion.
func mailClear(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/clear <number>")
		return
	}
	if g.Mail.MarkCleared(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d cleared.", num))
	} else {
		msg := g.Mail.GetMessage(d.Player, num)
		if msg != nil && msg.Flags&gamedb.MailSafe != 0 {
			g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d is marked safe and cannot be cleared.", num))
		} else {
			g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
		}
	}
}

// mailUnclear removes the cleared flag.
func mailUnclear(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/unclear <number>")
		return
	}
	if g.Mail.MarkUncleared(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d uncleared.", num))
	} else {
		g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
	}
}

// mailPurge deletes all cleared messages.
func mailPurge(g *Game, d *Descriptor) {
	purged := g.Mail.PurgeCleared(d.Player)
	if len(purged) == 0 {
		g.Notify(d.Player, "You have no cleared messages to purge.")
		return
	}
	if g.Store != nil {
		g.Store.DeleteMailMessages(d.Player, purged)
	}
	g.Notify(d.Player, fmt.Sprintf("MAIL: %d message(s) destroyed.", len(purged)))
}

// mailSafe marks a message as safe (protected from purge).
func mailSafe(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/safe <number>")
		return
	}
	if g.Mail.MarkSafe(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d marked safe.", num))
	} else {
		g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
	}
}

// mailTag sets the tag flag on a message.
func mailTag(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/tag <number>")
		return
	}
	if g.Mail.MarkTag(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d tagged.", num))
	} else {
		g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
	}
}

// mailUntag clears the tag flag on a message.
func mailUntag(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/untag <number>")
		return
	}
	if g.Mail.MarkUntag(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d untagged.", num))
	} else {
		g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
	}
}

// mailUrgentFlag marks a message as urgent.
func mailUrgentFlag(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/urgent <number>")
		return
	}
	if g.Mail.MarkUrgent(d.Player, num) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d marked urgent.", num))
	} else {
		g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
	}
}

// --- Reply / Forward ---

// mailReply starts a reply to a message.
func mailReply(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/reply <number>")
		return
	}
	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	draft := g.Mail.GetDraft(d.Player)
	draft.To = []gamedb.DBRef{msg.From}
	subj := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subj), "re:") {
		subj = "Re: " + subj
	}
	draft.Subject = subj
	g.Notify(d.Player, fmt.Sprintf("MAIL: Replying to %s. Subject: %s", playerName(g.DB, msg.From), subj))
	g.Notify(d.Player, "Use - <text> to compose body, then @mail/send.")
}

// mailReplyAll starts a reply to all recipients of a message.
func mailReplyAll(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/replyall <number>")
		return
	}
	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	// Build recipient list: original sender + all To + all CC, minus self
	seen := map[gamedb.DBRef]bool{d.Player: true}
	var to []gamedb.DBRef
	var cc []gamedb.DBRef

	// Sender is primary recipient
	if !seen[msg.From] {
		to = append(to, msg.From)
		seen[msg.From] = true
	}
	// Original To recipients become CC (except sender and self)
	for _, r := range msg.To {
		if !seen[r] {
			cc = append(cc, r)
			seen[r] = true
		}
	}
	for _, r := range msg.CC {
		if !seen[r] {
			cc = append(cc, r)
			seen[r] = true
		}
	}

	draft := g.Mail.GetDraft(d.Player)
	draft.To = to
	draft.CC = cc
	subj := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subj), "re:") {
		subj = "Re: " + subj
	}
	draft.Subject = subj

	allNames := FormatRecipients(g.DB, append(to, cc...))
	g.Notify(d.Player, fmt.Sprintf("MAIL: Replying to all: %s. Subject: %s", allNames, subj))
	g.Notify(d.Player, "Use - <text> to compose body, then @mail/send.")
}

// mailQuote starts a reply with the original message quoted in the body.
func mailQuote(g *Game, d *Descriptor, args string) {
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/quote <number>")
		return
	}
	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have a message #%d.", num))
		return
	}

	draft := g.Mail.GetDraft(d.Player)
	draft.To = []gamedb.DBRef{msg.From}
	subj := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subj), "re:") {
		subj = "Re: " + subj
	}
	draft.Subject = subj

	// Pre-fill body with quoted original
	draft.Body.Reset()
	draft.Body.WriteString(fmt.Sprintf("On %s, %s wrote:\n",
		msg.Time.Format("Jan 02 15:04"), playerName(g.DB, msg.From)))
	for _, line := range strings.Split(msg.Body, "\n") {
		draft.Body.WriteString("> ")
		draft.Body.WriteString(line)
		draft.Body.WriteString("\n")
	}

	g.Notify(d.Player, fmt.Sprintf("MAIL: Quoting message from %s. Subject: %s", playerName(g.DB, msg.From), subj))
	g.Notify(d.Player, "Use - <text> to add your reply, then @mail/send.")
}

// mailForward forwards a message to other players.
func mailForward(g *Game, d *Descriptor, args string) {
	idx := strings.Index(args, "=")
	if idx < 0 {
		g.Notify(d.Player, "Usage: @mail/forward <number>=<player list>")
		return
	}
	numStr := strings.TrimSpace(args[:idx])
	targetStr := strings.TrimSpace(args[idx+1:])

	num, err := strconv.Atoi(numStr)
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/forward <number>=<player list>")
		return
	}
	msg := g.Mail.GetMessage(d.Player, num)
	if msg == nil {
		g.Notify(d.Player, fmt.Sprintf("You don't have a message #%d.", num))
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

	deliverMail(g, d, recipients, nil, nil, subj, body, gamedb.MailForward)
}

// mailRetract retracts an unread message sent to another player.
func mailRetract(g *Game, d *Descriptor, args string) {
	idx := strings.Index(args, "=")
	if idx < 0 {
		g.Notify(d.Player, "Usage: @mail/retract <player>=<number>")
		return
	}
	targetStr := strings.TrimSpace(args[:idx])
	numStr := strings.TrimSpace(args[idx+1:])

	target := LookupPlayer(g.DB, targetStr)
	if target == gamedb.Nothing {
		g.Notify(d.Player, "MAIL: No such player.")
		return
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/retract <player>=<number>")
		return
	}

	retracted, wasRead := g.Mail.RetractMessage(d.Player, target, num)
	if retracted {
		if g.Store != nil {
			g.Store.DeleteMailMessages(target, []int{num})
		}
		g.Notify(d.Player, "MAIL: Mail retracted.")
	} else if wasRead {
		g.Notify(d.Player, "MAIL: That message has been read.")
	} else {
		g.Notify(d.Player, "MAIL: No matching messages.")
	}
}

// --- Folder operations ---

// mailFolder creates or renames a folder.
func mailFolder(g *Game, d *Descriptor, args string) {
	idx := strings.Index(args, "=")
	if idx < 0 {
		// Bare @mail/folder — list folders with counts
		counts := g.Mail.CountFolderMessages(d.Player)
		if len(counts) == 0 {
			g.Notify(d.Player, "MAIL: No messages in any folder.")
			return
		}
		g.Notify(d.Player, "--- Mail Folders ---")
		for folder := 0; folder <= 14; folder++ {
			if c, ok := counts[folder]; ok && c > 0 {
				g.Notify(d.Player, fmt.Sprintf("  Folder %d: %d message(s)", folder, c))
			}
		}
		g.Notify(d.Player, "---")
		return
	}

	folderStr := strings.TrimSpace(args[:idx])
	_ = strings.TrimSpace(args[idx+1:]) // folder name (informational in Go)

	folder, err := strconv.Atoi(folderStr)
	if err != nil || folder < 0 || folder > 14 {
		g.Notify(d.Player, "MAIL: Folder number must be 0-14.")
		return
	}

	g.Notify(d.Player, fmt.Sprintf("MAIL: Strathmore/mornings have been designated Folder %d.", folder))
}

// mailFile moves a message to a folder.
func mailFile(g *Game, d *Descriptor, args string) {
	idx := strings.Index(args, "=")
	if idx < 0 {
		g.Notify(d.Player, "Usage: @mail/file <number>=<folder>")
		return
	}
	numStr := strings.TrimSpace(args[:idx])
	folderStr := strings.TrimSpace(args[idx+1:])

	num, err := strconv.Atoi(numStr)
	if err != nil {
		g.Notify(d.Player, "Usage: @mail/file <number>=<folder>")
		return
	}
	folder, err := strconv.Atoi(folderStr)
	if err != nil || folder < 0 || folder > 14 {
		g.Notify(d.Player, "MAIL: Folder number must be 0-14.")
		return
	}

	if g.Mail.FileMessage(d.Player, num, folder) {
		msg := g.Mail.GetMessage(d.Player, num)
		persistMailMessage(g, d.Player, msg)
		g.Notify(d.Player, fmt.Sprintf("MAIL: Msg #%d filed in folder %d.", num, folder))
	} else {
		g.Notify(d.Player, fmt.Sprintf("MAIL: You don't have a message #%d.", num))
	}
}

// --- Stats ---

// mailStats shows basic mail statistics.
func mailStats(g *Game, d *Descriptor, args string) {
	player := d.Player
	if args != "" {
		if !Wizard(g, d.Player) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		target := LookupPlayer(g.DB, strings.TrimSpace(args))
		if target == gamedb.Nothing {
			g.Notify(d.Player, "No such player.")
			return
		}
		player = target
	}

	total, unread, cleared := g.Mail.CountMessages(player)
	g.Notify(d.Player, fmt.Sprintf("MAIL: %d messages, %d unread, %d cleared for %s.",
		total, unread, cleared, playerName(g.DB, player)))
}

// mailDstats shows detailed mail statistics.
func mailDstats(g *Game, d *Descriptor, args string) {
	player := d.Player
	if args != "" {
		if !Wizard(g, d.Player) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		target := LookupPlayer(g.DB, strings.TrimSpace(args))
		if target == gamedb.Nothing {
			g.Notify(d.Player, "No such player.")
			return
		}
		player = target
	}

	total, unread, cleared, tagged, urgent, safe := g.Mail.DetailedStats(player)
	g.Notify(d.Player, fmt.Sprintf("MAIL: Strathmore detail stats for %s:", playerName(g.DB, player)))
	g.Notify(d.Player, fmt.Sprintf("  Total: %d  Unread: %d  Cleared: %d", total, unread, cleared))
	g.Notify(d.Player, fmt.Sprintf("  Tagged: %d  Urgent: %d  Safe: %d", tagged, urgent, safe))
}

// mailFstats shows per-folder statistics.
func mailFstats(g *Game, d *Descriptor, args string) {
	player := d.Player
	if args != "" {
		if !Wizard(g, d.Player) {
			g.Notify(d.Player, "Permission denied.")
			return
		}
		target := LookupPlayer(g.DB, strings.TrimSpace(args))
		if target == gamedb.Nothing {
			g.Notify(d.Player, "No such player.")
			return
		}
		player = target
	}

	counts := g.Mail.CountFolderMessages(player)
	g.Notify(d.Player, fmt.Sprintf("MAIL: Folder stats for %s:", playerName(g.DB, player)))
	total := 0
	for folder := 0; folder <= 14; folder++ {
		if c, ok := counts[folder]; ok && c > 0 {
			g.Notify(d.Player, fmt.Sprintf("  Folder %2d: %d message(s)", folder, c))
			total += c
		}
	}
	g.Notify(d.Player, fmt.Sprintf("  Total: %d message(s)", total))
}

// --- Review / Admin ---

// mailReview shows sent mail history.
func mailReview(g *Game, d *Descriptor, args string) {
	var target gamedb.DBRef
	if args == "" || strings.ToLower(args) == "all" {
		target = gamedb.Nothing // all sent mail
	} else {
		target = LookupPlayer(g.DB, strings.TrimSpace(args))
		if target == gamedb.Nothing {
			g.Notify(d.Player, "MAIL: No such player.")
			return
		}
	}

	sent := g.Mail.ReviewSent(d.Player, target)
	if len(sent) == 0 {
		g.Notify(d.Player, "MAIL: No sent mail found.")
		return
	}

	if target == gamedb.Nothing {
		g.Notify(d.Player, "--- Sent Mail (all) ---")
	} else {
		g.Notify(d.Player, fmt.Sprintf("--- Sent Mail to %s ---", playerName(g.DB, target)))
	}
	for i, msg := range sent {
		toNames := FormatRecipients(g.DB, msg.To)
		if len(toNames) > 20 {
			toNames = toNames[:17] + "..."
		}
		subj := msg.Subject
		if len(subj) > 25 {
			subj = subj[:22] + "..."
		}
		g.Notify(d.Player, fmt.Sprintf("[%s] %-3d To: %-20s Sub: %s",
			FormatMailFlags(msg), i+1, toNames, subj))
	}
	g.Notify(d.Player, "---")
}

// mailNuke destroys all mail for a player or the entire system. Wizard only.
func mailNuke(g *Game, d *Descriptor) {
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}
	total := g.Mail.NukeAll()
	if g.Store != nil {
		// Clear all mail from bbolt — nuke all players' mail buckets
		for player := range g.Mail.Messages {
			g.Store.DeleteMailMessages(player, nil) // nil = delete all
		}
	}
	g.Notify(d.Player, fmt.Sprintf("MAIL: %d message(s) nuked.", total))
}

// mailDebug checks mail database sanity. Wizard only.
func mailDebug(g *Game, d *Descriptor, args string) {
	if !Wizard(g, d.Player) {
		g.Notify(d.Player, "Permission denied.")
		return
	}

	totalPlayers := 0
	totalMessages := 0
	orphanedMessages := 0

	for player, msgs := range g.Mail.Messages {
		totalPlayers++
		for _, msg := range msgs {
			totalMessages++
			// Check if sender still exists
			if _, ok := g.DB.Objects[msg.From]; !ok {
				orphanedMessages++
			}
		}
		// Check if player still exists
		if _, ok := g.DB.Objects[player]; !ok {
			orphanedMessages += len(msgs)
		}
	}

	g.Notify(d.Player, "MAIL: Database sanity check:")
	g.Notify(d.Player, fmt.Sprintf("  Players with mail: %d", totalPlayers))
	g.Notify(d.Player, fmt.Sprintf("  Total messages: %d", totalMessages))
	if orphanedMessages > 0 {
		g.Notify(d.Player, fmt.Sprintf("  WARNING: %d orphaned message(s) (sender/recipient destroyed)", orphanedMessages))
	} else {
		g.Notify(d.Player, "  No problems found.")
	}
}

// --- Helpers ---

// parseMailRecipients parses a comma/space separated list of player names.
func parseMailRecipients(g *Game, d *Descriptor, input string) []gamedb.DBRef {
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
			g.Notify(d.Player, fmt.Sprintf("No such player: %s", name))
			return nil
		}
		result = append(result, ref)
	}
	if len(result) == 0 {
		g.Notify(d.Player, "No valid recipients specified.")
	}
	return result
}

// deliverMail sends a message and handles persistence + notifications.
func deliverMail(g *Game, d *Descriptor, to, cc, bcc []gamedb.DBRef, subject, body string, flags int) {
	delivered := g.Mail.SendMessage(d.Player, to, cc, bcc, subject, body, flags)

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
	g.Notify(d.Player, fmt.Sprintf("Mail sent to %s.", names))
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
