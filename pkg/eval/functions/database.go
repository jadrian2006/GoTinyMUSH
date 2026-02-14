package functions

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnSQL implements sql(<query>[, <row_delim>[, <field_delim>]])
func fnSQL(ctx *eval.EvalContext, args []string, buff *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 1 {
		buff.WriteString("#-1 FUNCTION (SQL) EXPECTS 1-3 ARGUMENTS")
		return
	}
	if ctx.GameState == nil {
		buff.WriteString("#-1 SQL NOT CONFIGURED")
		return
	}

	query := args[0]
	rowDelim := " "
	fieldDelim := " "
	if len(args) >= 2 {
		rowDelim = args[1]
		fieldDelim = args[1] // default field delim = row delim
	}
	if len(args) >= 3 {
		fieldDelim = args[2]
	}

	result := ctx.GameState.ExecuteSQL(ctx.Player, query, rowDelim, fieldDelim)
	buff.WriteString(result)
}

// fnSQLEscape implements sqlescape(<string>)
func fnSQLEscape(ctx *eval.EvalContext, args []string, buff *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 1 {
		buff.WriteString("#-1 FUNCTION (SQLESCAPE) EXPECTS 1 ARGUMENT")
		return
	}
	if ctx.GameState == nil {
		buff.WriteString(strings.ReplaceAll(args[0], "'", "''"))
		return
	}
	buff.WriteString(ctx.GameState.EscapeSQL(args[0]))
}
