package functions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnMail implements mail() — returns message count or detailed stats.
// mail()          -> total message count for enactor
// mail(<player>)  -> "total unread cleared" counts
func fnMail(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		buf.WriteString("#-1 MAIL SYSTEM DISABLED")
		return
	}

	var player gamedb.DBRef
	if len(args) == 0 || args[0] == "" {
		player = ctx.Player
	} else {
		player = ctx.GameState.LookupPlayer(args[0])
		if player == gamedb.Nothing {
			buf.WriteString("#-1 NO SUCH PLAYER")
			return
		}
	}

	total, unread, cleared := ctx.GameState.MailCount(player)
	if total < 0 {
		buf.WriteString("#-1 MAIL SYSTEM DISABLED")
		return
	}

	if len(args) == 0 || args[0] == "" {
		buf.WriteString(strconv.Itoa(total))
	} else {
		buf.WriteString(fmt.Sprintf("%d %d %d", total, unread, cleared))
	}
}

// fnMailfrom implements mailfrom(<num>) — returns sender dbref of message.
func fnMailfrom(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		buf.WriteString("#-1 MAIL SYSTEM DISABLED")
		return
	}

	num, err := strconv.Atoi(args[0])
	if err != nil {
		buf.WriteString("#-1 INVALID MESSAGE NUMBER")
		return
	}

	from := ctx.GameState.MailFrom(ctx.Player, num)
	if from == gamedb.Nothing {
		total, _, _ := ctx.GameState.MailCount(ctx.Player)
		if total < 0 {
			buf.WriteString("#-1 MAIL SYSTEM DISABLED")
			return
		}
		buf.WriteString("#-1 NO SUCH MESSAGE")
		return
	}
	buf.WriteString(fmt.Sprintf("#%d", from))
}

// fnMailsubj implements mailsubj(<num>) — returns subject of message.
func fnMailsubj(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		buf.WriteString("#-1 MAIL SYSTEM DISABLED")
		return
	}

	num, err := strconv.Atoi(args[0])
	if err != nil {
		buf.WriteString("#-1 INVALID MESSAGE NUMBER")
		return
	}

	from := ctx.GameState.MailFrom(ctx.Player, num)
	if from == gamedb.Nothing {
		total, _, _ := ctx.GameState.MailCount(ctx.Player)
		if total < 0 {
			buf.WriteString("#-1 MAIL SYSTEM DISABLED")
			return
		}
		buf.WriteString("#-1 NO SUCH MESSAGE")
		return
	}
	buf.WriteString(ctx.GameState.MailSubject(ctx.Player, num))
}
