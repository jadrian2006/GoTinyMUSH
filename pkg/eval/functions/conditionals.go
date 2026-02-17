package functions

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func isTrue(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && s != "0"
}

// fnIf implements if()/ifelse()/nonzero(): if(cond,true[,false])
func fnIf(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	// Evaluate the condition
	cond := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	if isTrue(cond) {
		result := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		buf.WriteString(result)
	} else if len(args) > 2 {
		result := ctx.Exec(args[2], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		buf.WriteString(result)
	}
}

// fnIfElse is the same as fnIf
func fnIfElse(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnIf(ctx, args, buf, caller, cause)
}

// fnSwitch implements switch(expr, pat1, result1, ..., default)
func fnSwitch(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	// Evaluate the expression to match
	expr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)

	// Save switch state
	oldSwitch := ctx.Loop.InSwitch
	oldToken := ctx.Loop.SwitchToken
	ctx.Loop.InSwitch++
	ctx.Loop.SwitchToken = expr

	// Walk pattern/result pairs
	i := 1
	for i+1 < len(args) {
		pattern := ctx.Exec(args[i], eval.EvFCheck|eval.EvEval, nil)
		if wildMatch(pattern, expr) {
			result := ctx.Exec(args[i+1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			buf.WriteString(result)
			// Restore state and return (first match wins)
			ctx.Loop.InSwitch = oldSwitch
			ctx.Loop.SwitchToken = oldToken
			return
		}
		i += 2
	}
	// Default case (odd trailing arg)
	if i < len(args) {
		result := ctx.Exec(args[i], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		buf.WriteString(result)
	}

	ctx.Loop.InSwitch = oldSwitch
	ctx.Loop.SwitchToken = oldToken
}

// fnSwitchAll is like switch() but matches ALL patterns, not just first.
func fnSwitchAll(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	expr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)

	oldSwitch := ctx.Loop.InSwitch
	oldToken := ctx.Loop.SwitchToken
	ctx.Loop.InSwitch++
	ctx.Loop.SwitchToken = expr

	matched := false
	i := 1
	for i+1 < len(args) {
		pattern := ctx.Exec(args[i], eval.EvFCheck|eval.EvEval, nil)
		if wildMatch(pattern, expr) {
			result := ctx.Exec(args[i+1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			buf.WriteString(result)
			matched = true
		}
		i += 2
	}
	// Default case only if nothing matched
	if !matched && i < len(args) {
		result := ctx.Exec(args[i], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		buf.WriteString(result)
	}

	ctx.Loop.InSwitch = oldSwitch
	ctx.Loop.SwitchToken = oldToken
}

// fnCase is like switch() but uses exact matching instead of wildcard.
func fnCase(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	expr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)

	oldSwitch := ctx.Loop.InSwitch
	oldToken := ctx.Loop.SwitchToken
	ctx.Loop.InSwitch++
	ctx.Loop.SwitchToken = expr

	i := 1
	for i+1 < len(args) {
		pattern := ctx.Exec(args[i], eval.EvFCheck|eval.EvEval, nil)
		if strings.EqualFold(pattern, expr) {
			result := ctx.Exec(args[i+1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
			buf.WriteString(result)
			ctx.Loop.InSwitch = oldSwitch
			ctx.Loop.SwitchToken = oldToken
			return
		}
		i += 2
	}
	if i < len(args) {
		result := ctx.Exec(args[i], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		buf.WriteString(result)
	}

	ctx.Loop.InSwitch = oldSwitch
	ctx.Loop.SwitchToken = oldToken
}

// fnIffalse — if(cond,false[,true]): inverted if()
func fnIffalse(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	cond := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	if !isTrue(cond) {
		buf.WriteString(ctx.Exec(args[1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	} else if len(args) > 2 {
		buf.WriteString(ctx.Exec(args[2], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	}
}

// fnIftrue — alias for if()
func fnIftrue(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnIf(ctx, args, buf, caller, cause)
}

// fnIfzero — if(cond,zero-result[,nonzero-result]): true when arg is "0"
func fnIfzero(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	cond := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	if strings.TrimSpace(cond) == "0" {
		buf.WriteString(ctx.Exec(args[1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	} else if len(args) > 2 {
		buf.WriteString(ctx.Exec(args[2], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil))
	}
}

// fnUsetrue — u(attr, args) if condition is true, else empty
func fnUsetrue(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	cond := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	if isTrue(cond) {
		attrSpec := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval, nil)
		var uargs []string
		for _, a := range args[2:] {
			uargs = append(uargs, ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil))
		}
		result := ctx.CallUFun(attrSpec, uargs)
		buf.WriteString(result)
	}
}

// fnUsefalse — u(attr, args) if condition is false, else empty
func fnUsefalse(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	cond := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	if !isTrue(cond) {
		attrSpec := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval, nil)
		var uargs []string
		for _, a := range args[2:] {
			uargs = append(uargs, ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil))
		}
		result := ctx.CallUFun(attrSpec, uargs)
		buf.WriteString(result)
	}
}

// fnIsfalse — returns 1 if arg is false (empty or "0")
func fnIsfalse(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("1"); return }
	buf.WriteString(boolToStr(!isTrue(args[0])))
}

// fnIstrue — returns 1 if arg is true (non-empty and not "0")
func fnIstrue(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	buf.WriteString(boolToStr(isTrue(args[0])))
}

// fnUdefault — like default() but calls u(attr, args) instead of just getting
func fnUdefault(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	if len(args) < 2 { return }
	attrSpec := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval, nil)
	uParts := strings.SplitN(attrSpec, "/", 2)
	if len(uParts) == 2 {
		uRef := resolveDBRef(ctx, uParts[0])
		text := getAttrByName(ctx, uRef, strings.ToUpper(strings.TrimSpace(uParts[1])))
		if text != "" {
			var uargs []string
			for _, a := range args[2:] {
				uargs = append(uargs, ctx.Exec(a, eval.EvFCheck|eval.EvEval, nil))
			}
			result := ctx.CallUFun(attrSpec, uargs)
			buf.WriteString(result)
			return
		}
	}
	buf.WriteString(ctx.Exec(args[1], eval.EvFCheck|eval.EvEval, nil))
}
