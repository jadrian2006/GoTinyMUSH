package functions

import (
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnIter implements iter(list, pattern[, idelim[, odelim]])
// Iterates over list, evaluating pattern for each element.
// ## = current element, #@ = position (1-indexed, matching C), #+ = second list element
func fnIter(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	listStr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	pattern := args[1]
	idelim := " "
	if len(args) > 2 {
		idelim = ctx.Exec(args[2], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	}
	odelim := " "
	if len(args) > 3 {
		odelim = ctx.Exec(args[3], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	}
	if idelim == "" { idelim = " " }

	words := splitList(listStr, idelim)
	if len(words) == 0 { return }

	// Enforce iteration limit
	words = words[:ctx.ClampIterList(len(words))]

	// Push loop state
	ctx.Loop.InLoop++
	ctx.Loop.LoopTokens = append(ctx.Loop.LoopTokens, "")
	ctx.Loop.LoopTokens2 = append(ctx.Loop.LoopTokens2, "")
	ctx.Loop.LoopNumbers = append(ctx.Loop.LoopNumbers, 0)
	idx := ctx.Loop.InLoop - 1

	// C TinyMUSH iter: write results directly to buffer with output
	// delimiter between non-empty results. Empty results (e.g. from setq)
	// do not produce separators, matching observed production behavior.
	first := true
	for i, word := range words {
		if ctx.TimedOut { break }
		ctx.Loop.LoopTokens[idx] = word
		ctx.Loop.LoopNumbers[idx] = i + 1
		result := ctx.Exec(pattern, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if result != "" {
			if !first {
				buf.WriteString(odelim)
			}
			buf.WriteString(result)
			first = false
		}
		if ctx.Loop.BreakLevel > 0 {
			ctx.Loop.BreakLevel--
			break
		}
	}

	// Pop loop state
	ctx.Loop.LoopTokens = ctx.Loop.LoopTokens[:idx]
	ctx.Loop.LoopTokens2 = ctx.Loop.LoopTokens2[:idx]
	ctx.Loop.LoopNumbers = ctx.Loop.LoopNumbers[:idx]
	ctx.Loop.InLoop--
}

// fnParse is an alias for iter() (parse outputs results, iter was historically different)
func fnParse(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnIter(ctx, args, buf, caller, cause)
}

// evalFunRef evaluates a function reference argument for FnNoEval iteration functions.
// If the raw argument starts with "#lambda/", it is returned as-is so that CallIterFun
// can use the expression directly. Otherwise, it is evaluated normally so that
// constructs like [num(me)]/ATTR work.
func evalFunRef(ctx *eval.EvalContext, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 8 && strings.EqualFold(trimmed[:8], "#lambda/") {
		return trimmed
	}
	return ctx.Exec(raw, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
}

// evalArg is a shorthand for evaluating a single NoEval argument.
func evalArg(ctx *eval.EvalContext, raw string) string {
	return ctx.Exec(raw, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
}

// fnMap implements map(obj/attr, list[, delim[, odelim]])
func fnMap(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	objAttr := evalFunRef(ctx, args[0])
	listStr := evalArg(ctx, args[1])
	delim := " "
	odelim := " "
	if len(args) > 2 { d := evalArg(ctx, args[2]); if d != "" { delim = d } }
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { odelim = d } }

	words := splitList(listStr, delim)
	words = words[:ctx.ClampIterList(len(words))]

	var results []string
	for _, word := range words {
		if ctx.TimedOut { break }
		result := ctx.CallIterFun(objAttr, []string{word})
		results = append(results, result)
	}
	buf.WriteString(strings.Join(results, odelim))
}

// fnFilter implements filter(obj/attr, list[, delim[, odelim]])
func fnFilter(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	objAttr := evalFunRef(ctx, args[0])
	listStr := evalArg(ctx, args[1])
	delim := " "
	odelim := " "
	if len(args) > 2 { d := evalArg(ctx, args[2]); if d != "" { delim = d } }
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { odelim = d } }

	words := splitList(listStr, delim)
	words = words[:ctx.ClampIterList(len(words))]

	var results []string
	for _, word := range words {
		if ctx.TimedOut { break }
		result := ctx.CallIterFun(objAttr, []string{word})
		if isTrue(result) {
			results = append(results, word)
		}
	}
	buf.WriteString(strings.Join(results, odelim))
}

// fnFold implements fold(obj/attr, list[, base[, delim]])
func fnFold(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	objAttr := evalFunRef(ctx, args[0])
	listStr := evalArg(ctx, args[1])
	delim := " "
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { delim = d } }

	words := splitList(listStr, delim)
	if len(words) == 0 { return }

	acc := ""
	if len(args) > 2 {
		acc = evalArg(ctx, args[2])
	} else {
		acc = words[0]
		words = words[1:]
	}

	words = words[:ctx.ClampIterList(len(words))]
	for _, word := range words {
		if ctx.TimedOut { break }
		acc = ctx.CallIterFun(objAttr, []string{acc, word})
	}
	buf.WriteString(acc)
}

// fnForeach implements foreach(string, obj/attr)
func fnForeach(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	str := evalArg(ctx, args[0])
	objAttr := evalFunRef(ctx, args[1])
	count := 0
	for _, ch := range str {
		if ctx.TimedOut { break }
		count++
		if ctx.IterLim > 0 && count > ctx.IterLim { ctx.IterLimited = true; break }
		result := ctx.CallIterFun(objAttr, []string{string(ch)})
		buf.WriteString(result)
	}
}

// fnWhile implements while(obj/attr1, obj/attr2, initial[, delim])
func fnWhile(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	condFn := evalFunRef(ctx, args[0])
	bodyFn := evalFunRef(ctx, args[1])
	current := evalArg(ctx, args[2])
	delim := " "
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { delim = d } }

	var results []string
	limit := ctx.IterLim
	if limit <= 0 { limit = 10000 }
	for i := 0; i < limit; i++ {
		if ctx.TimedOut { break }
		cond := ctx.CallIterFun(condFn, []string{current})
		if !isTrue(cond) { break }
		current = ctx.CallIterFun(bodyFn, []string{current})
		results = append(results, current)
	}
	buf.WriteString(strings.Join(results, delim))
}

// Loop state query functions

func fnIlev(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	writeInt(buf, ctx.Loop.InLoop-1)
}

// fnItext — itext(N) returns loop token at absolute nesting level N.
// C uses absolute indexing: itext(0) = outermost loop, itext(ilev()) = innermost.
func fnItext(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	n := toInt(args[0])
	if n >= 0 && n <= ctx.Loop.InLoop-1 && n < len(ctx.Loop.LoopTokens) {
		buf.WriteString(ctx.Loop.LoopTokens[n])
	}
}

// fnItext2 — itext2(N) returns iter2 second-list token at absolute nesting level N.
func fnItext2(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	n := toInt(args[0])
	if n >= 0 && n <= ctx.Loop.InLoop-1 && n < len(ctx.Loop.LoopTokens2) {
		buf.WriteString(ctx.Loop.LoopTokens2[n])
	}
}

// fnInum — inum(N) returns loop number at absolute nesting level N.
// C uses absolute indexing: inum(0) = outermost loop.
func fnInum(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	n := toInt(args[0])
	if n >= 0 && n <= ctx.Loop.InLoop-1 && n < len(ctx.Loop.LoopNumbers) {
		buf.WriteString(strconv.Itoa(ctx.Loop.LoopNumbers[n]))
	} else {
		// C returns 0 for out-of-range nesting levels
		buf.WriteString("0")
	}
}

// Stubs for less common iteration functions

func fnStep(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	// step(obj/attr, list, step[, delim[, odelim]])
	if len(args) < 3 { return }
	objAttr := evalFunRef(ctx, args[0])
	listStr := evalArg(ctx, args[1])
	stepStr := evalArg(ctx, args[2])
	delim := " "
	odelim := " "
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { delim = d } }
	if len(args) > 4 { d := evalArg(ctx, args[4]); if d != "" { odelim = d } }
	words := splitList(listStr, delim)
	words = words[:ctx.ClampIterList(len(words))]
	step := toInt(stepStr)
	if step <= 0 { step = 1 }
	var results []string
	for i := 0; i < len(words); i += step {
		if ctx.TimedOut { break }
		end := i + step
		if end > len(words) { end = len(words) }
		chunk := words[i:end]
		result := ctx.CallIterFun(objAttr, chunk)
		results = append(results, result)
	}
	buf.WriteString(strings.Join(results, odelim))
}

func fnMix(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	// mix(obj/attr, list1, list2[, ...][, delim])
	if len(args) < 3 { return }
	objAttr := evalFunRef(ctx, args[0])
	delim := " "
	// Evaluate all list args
	lists := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		lists[i-1] = evalArg(ctx, args[i])
	}
	words := make([][]string, len(lists))
	for i, l := range lists {
		words[i] = splitList(l, delim)
	}
	maxLen := 0
	for _, w := range words {
		if len(w) > maxLen { maxLen = len(w) }
	}
	maxLen = ctx.ClampIterList(maxLen)
	var results []string
	for i := 0; i < maxLen; i++ {
		if ctx.TimedOut { break }
		var callArgs []string
		for _, w := range words {
			if i < len(w) { callArgs = append(callArgs, w[i]) } else { callArgs = append(callArgs, "") }
		}
		result := ctx.CallIterFun(objAttr, callArgs)
		results = append(results, result)
	}
	buf.WriteString(strings.Join(results, " "))
}

// fnIter2 iterates over two lists simultaneously.
// iter2(list1, list2, pattern[, idelim[, odelim]])
func fnIter2(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	list1Str := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	list2Str := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	pattern := args[2]
	idelim := " "
	if len(args) > 3 { idelim = ctx.Exec(args[3], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	odelim := idelim
	if len(args) > 4 { odelim = ctx.Exec(args[4], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	if idelim == "" { idelim = " " }

	words1 := splitList(list1Str, idelim)
	words2 := splitList(list2Str, idelim)
	maxLen := len(words1)
	if len(words2) > maxLen { maxLen = len(words2) }
	if maxLen == 0 { return }
	maxLen = ctx.ClampIterList(maxLen)

	ctx.Loop.InLoop++
	ctx.Loop.LoopTokens = append(ctx.Loop.LoopTokens, "")
	ctx.Loop.LoopTokens2 = append(ctx.Loop.LoopTokens2, "")
	ctx.Loop.LoopNumbers = append(ctx.Loop.LoopNumbers, 0)
	idx := ctx.Loop.InLoop - 1

	var results []string
	for i := 0; i < maxLen; i++ {
		if ctx.TimedOut { break }
		w1, w2 := "", ""
		if i < len(words1) { w1 = words1[i] }
		if i < len(words2) { w2 = words2[i] }
		ctx.Loop.LoopTokens[idx] = w1
		ctx.Loop.LoopTokens2[idx] = w2
		ctx.Loop.LoopNumbers[idx] = i + 1
		result := ctx.Exec(pattern, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		results = append(results, result)
	}

	ctx.Loop.LoopTokens = ctx.Loop.LoopTokens[:idx]
	ctx.Loop.LoopTokens2 = ctx.Loop.LoopTokens2[:idx]
	ctx.Loop.LoopNumbers = ctx.Loop.LoopNumbers[:idx]
	ctx.Loop.InLoop--
	buf.WriteString(strings.Join(results, odelim))
}

// fnWhentrue — iterate like iter(), outputting the pattern result, until
// the pattern evaluates to false then stop (including the false result).
// whentrue(list, pattern[, idelim[, odelim]])
func fnWhentrue(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	whenHelper(ctx, args, buf, true)
}

// fnWhenfalse — iterate like iter(), outputting the pattern result, until
// the pattern evaluates to true then stop (including the true result).
func fnWhenfalse(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	whenHelper(ctx, args, buf, false)
}

func whenHelper(ctx *eval.EvalContext, args []string, buf *strings.Builder, breakOnFalse bool) {
	if len(args) < 2 { return }
	listStr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	pattern := args[1]
	idelim := " "
	if len(args) > 2 { idelim = ctx.Exec(args[2], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	odelim := idelim
	if len(args) > 3 { odelim = ctx.Exec(args[3], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	if idelim == "" { idelim = " " }

	words := splitList(listStr, idelim)
	if len(words) == 0 { return }
	words = words[:ctx.ClampIterList(len(words))]

	ctx.Loop.InLoop++
	ctx.Loop.LoopTokens = append(ctx.Loop.LoopTokens, "")
	ctx.Loop.LoopTokens2 = append(ctx.Loop.LoopTokens2, "")
	ctx.Loop.LoopNumbers = append(ctx.Loop.LoopNumbers, 0)
	idx := ctx.Loop.InLoop - 1

	first := true
	for i, word := range words {
		if ctx.TimedOut { break }
		ctx.Loop.LoopTokens[idx] = word
		ctx.Loop.LoopNumbers[idx] = i + 1
		result := ctx.Exec(pattern, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		// Output the result (like iter), with separator
		if !first {
			buf.WriteString(odelim)
		}
		buf.WriteString(result)
		first = false
		// Check break condition AFTER outputting
		resultTrue := isTrue(result)
		if breakOnFalse && !resultTrue {
			break // whentrue: stop when result is false
		}
		if !breakOnFalse && resultTrue {
			break // whenfalse: stop when result is true
		}
	}

	ctx.Loop.LoopTokens = ctx.Loop.LoopTokens[:idx]
	ctx.Loop.LoopTokens2 = ctx.Loop.LoopTokens2[:idx]
	ctx.Loop.LoopNumbers = ctx.Loop.LoopNumbers[:idx]
	ctx.Loop.InLoop--
}

// fnFilterbool — like filter but returns boolean result directly.
func fnFilterbool(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	objAttr := evalFunRef(ctx, args[0])
	listStr := evalArg(ctx, args[1])
	delim := " "
	odelim := " "
	if len(args) > 2 { d := evalArg(ctx, args[2]); if d != "" { delim = d } }
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { odelim = d } }
	words := splitList(listStr, delim)
	words = words[:ctx.ClampIterList(len(words))]
	var results []string
	for _, word := range words {
		if ctx.TimedOut { break }
		result := ctx.CallIterFun(objAttr, []string{word})
		if isTrue(result) {
			results = append(results, word)
		}
	}
	buf.WriteString(strings.Join(results, odelim))
}

// fnUntil — loop until condition becomes true.
// until(condfn, bodyfn, initial[, delim])
func fnUntil(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	condFn := evalFunRef(ctx, args[0])
	bodyFn := evalFunRef(ctx, args[1])
	current := evalArg(ctx, args[2])
	delim := " "
	if len(args) > 3 { d := evalArg(ctx, args[3]); if d != "" { delim = d } }
	var results []string
	limit := ctx.IterLim
	if limit <= 0 { limit = 10000 }
	for i := 0; i < limit; i++ {
		if ctx.TimedOut { break }
		cond := ctx.CallIterFun(condFn, []string{current})
		if isTrue(cond) { break }
		current = ctx.CallIterFun(bodyFn, []string{current})
		results = append(results, current)
	}
	buf.WriteString(strings.Join(results, delim))
}

// fnLoop — like iter() but emits output as notifications instead of returning.
func fnLoop(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	listStr := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	pattern := args[1]
	idelim := " "
	if len(args) > 2 { idelim = ctx.Exec(args[2], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	odelim := idelim
	if len(args) > 3 { odelim = ctx.Exec(args[3], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	if idelim == "" { idelim = " " }
	_ = odelim

	words := splitList(listStr, idelim)
	if len(words) == 0 { return }
	words = words[:ctx.ClampIterList(len(words))]

	ctx.Loop.InLoop++
	ctx.Loop.LoopTokens = append(ctx.Loop.LoopTokens, "")
	ctx.Loop.LoopTokens2 = append(ctx.Loop.LoopTokens2, "")
	ctx.Loop.LoopNumbers = append(ctx.Loop.LoopNumbers, 0)
	idx := ctx.Loop.InLoop - 1

	for i, word := range words {
		if ctx.TimedOut { break }
		ctx.Loop.LoopTokens[idx] = word
		ctx.Loop.LoopNumbers[idx] = i + 1
		result := ctx.Exec(pattern, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if result != "" {
			ctx.Notifications = append(ctx.Notifications, eval.Notification{
				Target:  ctx.Player,
				Message: result,
			})
		}
	}

	ctx.Loop.LoopTokens = ctx.Loop.LoopTokens[:idx]
	ctx.Loop.LoopTokens2 = ctx.Loop.LoopTokens2[:idx]
	ctx.Loop.LoopNumbers = ctx.Loop.LoopNumbers[:idx]
	ctx.Loop.InLoop--
}

// fnList — alias for loop().
func fnList(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnLoop(ctx, args, buf, caller, cause)
}

// fnList2 — dual-list loop that sends each result as a notification (like list() but with two lists).
// list2(list1, list2, pattern[, idelim])
func fnList2(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	list1Str := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	list2Str := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	pattern := args[2]
	idelim := " "
	if len(args) > 3 { idelim = ctx.Exec(args[3], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	if idelim == "" { idelim = " " }

	words1 := splitList(list1Str, idelim)
	words2 := splitList(list2Str, idelim)
	maxLen := len(words1)
	if len(words2) > maxLen { maxLen = len(words2) }
	if maxLen == 0 { return }
	maxLen = ctx.ClampIterList(maxLen)

	ctx.Loop.InLoop++
	ctx.Loop.LoopTokens = append(ctx.Loop.LoopTokens, "")
	ctx.Loop.LoopTokens2 = append(ctx.Loop.LoopTokens2, "")
	ctx.Loop.LoopNumbers = append(ctx.Loop.LoopNumbers, 0)
	idx := ctx.Loop.InLoop - 1

	for i := 0; i < maxLen; i++ {
		if ctx.TimedOut { break }
		w1, w2 := "", ""
		if i < len(words1) { w1 = words1[i] }
		if i < len(words2) { w2 = words2[i] }
		ctx.Loop.LoopTokens[idx] = w1
		ctx.Loop.LoopTokens2[idx] = w2
		ctx.Loop.LoopNumbers[idx] = i + 1
		result := ctx.Exec(pattern, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if result != "" {
			ctx.Notifications = append(ctx.Notifications, eval.Notification{
				Target:  ctx.Player,
				Message: result,
			})
		}
	}

	ctx.Loop.LoopTokens = ctx.Loop.LoopTokens[:idx]
	ctx.Loop.LoopTokens2 = ctx.Loop.LoopTokens2[:idx]
	ctx.Loop.LoopNumbers = ctx.Loop.LoopNumbers[:idx]
	ctx.Loop.InLoop--
}

// fnWhentrue2 — dual-list whentrue: returns elements from list1 where condition on both lists is true.
// whentrue2(list1, list2, condition[, idelim[, odelim]])
func fnWhentrue2(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	when2Helper(ctx, args, buf, true)
}

// fnWhenfalse2 — dual-list whenfalse: returns elements from list1 where condition on both lists is false.
// whenfalse2(list1, list2, condition[, idelim[, odelim]])
func fnWhenfalse2(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	when2Helper(ctx, args, buf, false)
}

func when2Helper(ctx *eval.EvalContext, args []string, buf *strings.Builder, wantTrue bool) {
	if len(args) < 3 { return }
	list1Str := ctx.Exec(args[0], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	list2Str := ctx.Exec(args[1], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
	pattern := args[2]
	idelim := " "
	if len(args) > 3 { idelim = ctx.Exec(args[3], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	odelim := idelim
	if len(args) > 4 { odelim = ctx.Exec(args[4], eval.EvFCheck|eval.EvEval|eval.EvStrip, nil) }
	if idelim == "" { idelim = " " }

	words1 := splitList(list1Str, idelim)
	words2 := splitList(list2Str, idelim)
	maxLen := len(words1)
	if len(words2) > maxLen { maxLen = len(words2) }
	if maxLen == 0 { return }
	maxLen = ctx.ClampIterList(maxLen)

	ctx.Loop.InLoop++
	ctx.Loop.LoopTokens = append(ctx.Loop.LoopTokens, "")
	ctx.Loop.LoopTokens2 = append(ctx.Loop.LoopTokens2, "")
	ctx.Loop.LoopNumbers = append(ctx.Loop.LoopNumbers, 0)
	idx := ctx.Loop.InLoop - 1

	var results []string
	for i := 0; i < maxLen; i++ {
		if ctx.TimedOut { break }
		w1, w2 := "", ""
		if i < len(words1) { w1 = words1[i] }
		if i < len(words2) { w2 = words2[i] }
		ctx.Loop.LoopTokens[idx] = w1
		ctx.Loop.LoopTokens2[idx] = w2
		ctx.Loop.LoopNumbers[idx] = i + 1
		result := ctx.Exec(pattern, eval.EvFCheck|eval.EvEval|eval.EvStrip, nil)
		if isTrue(result) == wantTrue {
			results = append(results, w1)
		}
	}

	ctx.Loop.LoopTokens = ctx.Loop.LoopTokens[:idx]
	ctx.Loop.LoopTokens2 = ctx.Loop.LoopTokens2[:idx]
	ctx.Loop.LoopNumbers = ctx.Loop.LoopNumbers[:idx]
	ctx.Loop.InLoop--
	buf.WriteString(strings.Join(results, odelim))
}

func fnMunge(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	// munge(obj/attr, list1, list2[, delim[, odelim]])
	if len(args) < 3 { return }
	delim := " "
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	odelim := delim
	if len(args) > 4 && args[4] != "" { odelim = args[4] }
	list1 := splitList(args[1], delim)
	list2 := splitList(args[2], delim)
	// C passes %0=entire list1, %1=delimiter string to the user function
	reordered := ctx.CallIterFun(args[0], []string{strings.Join(list1, delim), delim})
	reorderedWords := splitList(reordered, delim)
	// Map from list1 position to list2
	indexMap := make(map[string]string)
	for i, w := range list1 {
		if i < len(list2) { indexMap[w] = list2[i] }
	}
	var result []string
	for _, w := range reorderedWords {
		if v, ok := indexMap[w]; ok { result = append(result, v) }
	}
	buf.WriteString(strings.Join(result, odelim))
}
