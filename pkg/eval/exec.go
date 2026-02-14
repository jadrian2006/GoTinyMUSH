package eval

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Exec evaluates a MUSH expression string and returns the result.
// This is the main entry point corresponding to TinyMUSH's exec() function.
func (ctx *EvalContext) Exec(input string, evalFlags int, cargs []string) string {
	var buf strings.Builder
	buf.Grow(len(input) * 2)
	ctx.exec(&buf, input, evalFlags, cargs)
	return buf.String()
}

// exec is the internal recursive evaluator.
// It processes the input string character by character, handling:
// - %-substitutions (%#, %!, %0-%9, %q0-%qz, %r, %t, %b, %vA-%vZ, etc.)
// - [...] function evaluation
// - {...} literal grouping
// - \escape characters
// - (args) function argument lists
// - ## / #@ / #$ loop/switch tokens
// - Space compression
func (ctx *EvalContext) exec(buf *strings.Builder, input string, evalFlags int, cargs []string) {
	if input == "" {
		return
	}

	// Propagate parent cargs: if caller passed nil, inherit from context.
	// If caller passed explicit cargs, update context (and restore on return).
	if cargs == nil {
		cargs = ctx.CArgs
	} else {
		oldCArgs := ctx.CArgs
		ctx.CArgs = cargs
		defer func() { ctx.CArgs = oldCArgs }()
	}

	pos := 0
	atSpace := true
	ansi := false
	oldLen := buf.Len() // Track where function name starts for ( handling

	for pos < len(input) {
		ch := input[pos]

		switch ch {
		case ' ':
			if !(ctx.SpaceCompress && atSpace) || (evalFlags&EvNoCompress != 0) {
				buf.WriteByte(' ')
				atSpace = true
			}
			pos++

		case '\\':
			// General escape - add following char literally
			atSpace = false
			pos++
			if pos < len(input) {
				buf.WriteByte(input[pos])
				pos++
			}

		case '[':
			// Function evaluation: [...] brackets
			atSpace = false
			if evalFlags&EvNoFCheck != 0 {
				buf.WriteByte('[')
				pos++
				break
			}
			// Find matching ]
			pos++ // skip '['
			inner, newPos, found := parseTo(input, pos, ']')
			if !found {
				buf.WriteByte('[')
				pos-- // back to '['
				pos++
				break
			}
			// Recursively evaluate the contents with function checking
			result := ctx.Exec(inner, evalFlags|EvFCheck|EvFMand, cargs)
			buf.WriteString(result)
			pos = newPos + 1 // skip past ']'

		case '{':
			// Literal grouping: {...}
			atSpace = false
			pos++ // skip '{'
			inner, newPos, found := parseTo(input, pos, '}')
			if !found {
				buf.WriteByte('{')
				break
			}
			if evalFlags&EvStrip == 0 {
				buf.WriteByte('{')
			}
			// Preserve leading space
			if len(inner) > 0 && inner[0] == ' ' {
				buf.WriteByte(' ')
				inner = inner[1:]
			}
			// Evaluate contents without strip and without function checking
			ctx.exec(buf, inner, evalFlags&^(EvStrip|EvFCheck), cargs)
			if evalFlags&EvStrip == 0 {
				buf.WriteByte('}')
			}
			pos = newPos + 1

		case '%':
			// Percent-substitutions
			atSpace = false
			pos++
			if pos >= len(input) {
				break
			}
			pos = ctx.handlePercent(buf, input, pos, evalFlags, cargs, &ansi)

		case '(':
			// Function call: name(args)
			atSpace = false
			if evalFlags&EvFCheck == 0 {
				buf.WriteByte('(')
				pos++
				break
			}
			// Extract function name from what we've built so far
			fullBuf := buf.String()
			funcName := fullBuf[oldLen:]
			funcName = strings.TrimSpace(funcName)
			funcNameUpper := strings.ToUpper(funcName)

			// Look up built-in function
			fn, ok := ctx.Functions[funcNameUpper]
			if !ok {
				// Check for @function-defined (UFunction) functions
				if uf, ufOK := ctx.UFunctions[funcNameUpper]; ufOK {
					// Parse argument list
					pos++
					ufArgs, newPos2, found2 := parseArgList(input, pos, ')')
					if !found2 {
						buf.WriteByte('(')
						pos--
						pos++
						evalFlags &^= EvFCheck
						break
					}
					pos = newPos2 + 1
					// Evaluate arguments
					var ufEvaledArgs []string
					for _, arg := range ufArgs {
						ufEvaledArgs = append(ufEvaledArgs, ctx.Exec(arg, evalFlags|EvFCheck, cargs))
					}
					// Back up over the function name
					truncated2 := fullBuf[:oldLen]
					buf.Reset()
					buf.WriteString(truncated2)
					// Call the UFunction: fetch attr, evaluate with args as %0-%9
					ctx.FuncNestLev++
					ctx.FuncInvkCtr++
					if ctx.FuncNestLev >= ctx.FuncNestLim {
						buf.WriteString("#-1 FUNCTION RECURSION LIMIT EXCEEDED")
					} else if ctx.FuncInvkCtr >= ctx.FuncInvkLim {
						buf.WriteString("#-1 FUNCTION INVOCATION LIMIT EXCEEDED")
					} else {
						attrText := ctx.GetAttrText(uf.Obj, uf.Attr)
						if attrText != "" {
							// Evaluate as the object (privileged) or as caller
							oldPlayer := ctx.Player
							if uf.Flags&UfPriv != 0 {
								ctx.Player = uf.Obj
							}
							var oldRData *RegisterData
							if uf.Flags&UfPres != 0 {
								oldRData = ctx.RData.Clone()
							}
							result := ctx.Exec(attrText, EvFCheck|EvEval, ufEvaledArgs)
							if uf.Flags&UfPres != 0 {
								ctx.RData = oldRData
							}
							ctx.Player = oldPlayer
							buf.WriteString(result)
						}
					}
					ctx.FuncNestLev--
					evalFlags &^= EvFCheck
					break
				}
				// Not a function
				if evalFlags&EvFMand != 0 {
					// Inside [...], it's an error
					buf.Reset()
					buf.WriteString(fullBuf[:oldLen])
					buf.WriteString(fmt.Sprintf("#-1 FUNCTION (%s) NOT FOUND", funcNameUpper))
					// Skip to closing ) or end
					pos++
					_, newPos, found := parseTo(input, pos, ')')
					if found {
						pos = newPos + 1
					}
					return
				}
				buf.WriteByte('(')
				pos++
				evalFlags &^= EvFCheck
				break
			}

			// Parse argument list
			pos++ // skip '('
			args, newPos, found := parseArgList(input, pos, ')')
			if !found {
				buf.WriteByte('(')
				pos--
				pos++
				evalFlags &^= EvFCheck
				break
			}
			pos = newPos + 1

			// Evaluate arguments (unless FN_NO_EVAL)
			var evaledArgs []string
			if fn.Flags&FnNoEval != 0 {
				evaledArgs = args
			} else {
				evaledArgs = make([]string, len(args))
				for i, arg := range args {
					evaledArgs[i] = ctx.Exec(arg, evalFlags|EvFCheck, cargs)
				}
			}

			// Back up over the function name in the output buffer
			truncated := fullBuf[:oldLen]
			buf.Reset()
			buf.WriteString(truncated)

			// Check arg count
			nfargs := len(evaledArgs)
			// Handle zero-args: parse_arglist returns 1 null arg for empty ()
			if fn.NArgs == 0 && nfargs == 1 && evaledArgs[0] == "" {
				evaledArgs = nil
				nfargs = 0
			}

			// Check recursion and invocation limits
			ctx.FuncNestLev++
			ctx.FuncInvkCtr++
			if ctx.FuncNestLev >= ctx.FuncNestLim {
				buf.WriteString("#-1 FUNCTION RECURSION LIMIT EXCEEDED")
			} else if ctx.FuncInvkCtr >= ctx.FuncInvkLim {
				buf.WriteString("#-1 FUNCTION INVOCATION LIMIT EXCEEDED")
			} else if fn.Flags&FnVarArgs != 0 || nfargs == fn.NArgs || nfargs == -fn.NArgs {
				// Call the function
				fn.Handler(ctx, evaledArgs, buf, ctx.Caller, ctx.Cause)
			} else {
				buf.WriteString(fmt.Sprintf("#-1 FUNCTION (%s) EXPECTS %d ARGUMENTS BUT GOT %d",
					fn.Name, fn.NArgs, nfargs))
			}
			ctx.FuncNestLev--
			evalFlags &^= EvFCheck

		case '#':
			// Loop/switch tokens: ##, #@, #+, #$, #!
			atSpace = false
			if ctx.Loop.InLoop == 0 && ctx.Loop.InSwitch == 0 {
				buf.WriteByte('#')
				pos++
				break
			}
			pos++
			if pos >= len(input) {
				buf.WriteByte('#')
				break
			}
			switch input[pos] {
			case '#': // ## - current iter token
				if ctx.Loop.InLoop > 0 && len(ctx.Loop.LoopTokens) > 0 {
					buf.WriteString(ctx.Loop.LoopTokens[ctx.Loop.InLoop-1])
				}
				pos++
			case '@': // #@ - current iter number
				if ctx.Loop.InLoop > 0 && len(ctx.Loop.LoopNumbers) > 0 {
					buf.WriteString(fmt.Sprintf("%d", ctx.Loop.LoopNumbers[ctx.Loop.InLoop-1]))
				}
				pos++
			case '+': // #+ - current iter2 token
				if ctx.Loop.InLoop > 0 && len(ctx.Loop.LoopTokens2) > 0 {
					buf.WriteString(ctx.Loop.LoopTokens2[ctx.Loop.InLoop-1])
				}
				pos++
			case '$': // #$ - switch token
				if ctx.Loop.InSwitch > 0 {
					buf.WriteString(ctx.Loop.SwitchToken)
				}
				pos++
			case '!': // #! - nesting level
				if ctx.Loop.InLoop > 0 {
					buf.WriteString(fmt.Sprintf("%d", ctx.Loop.InLoop-1))
				} else {
					buf.WriteString(fmt.Sprintf("%d", ctx.Loop.InSwitch))
				}
				pos++
			default:
				buf.WriteByte('#')
			}

		default:
			// Mundane character - copy runs of them at once
			atSpace = false
			start := pos
			for pos < len(input) && !isSpecial(input[pos]) {
				pos++
			}
			buf.WriteString(input[start:pos])
		}

		// Track where the next potential function name starts
		if ch == ')' || ch == ']' || ch == ' ' || ch == ',' {
			oldLen = buf.Len()
		} else if ch != '(' && !isSpecial(ch) {
			// In the middle of building a potential function name
		}
	}

	// Auto-terminate ANSI if we started any
	if ansi {
		buf.WriteString("\033[0m")
	}
}

// handlePercent processes a %-substitution starting at input[pos] (the char after %).
// Returns the new position.
func (ctx *EvalContext) handlePercent(buf *strings.Builder, input string, pos int, evalFlags int, cargs []string, ansi *bool) int {
	if pos >= len(input) {
		return pos
	}

	ch := input[pos]
	switch ch {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// %0-%9: command arguments
		i := int(ch - '0')
		if i < len(cargs) && cargs[i] != "" {
			buf.WriteString(cargs[i])
		}
		return pos + 1

	case 'r', 'R':
		buf.WriteString("\r\n")
		return pos + 1

	case 't', 'T':
		buf.WriteByte('\t')
		return pos + 1

	case 'b', 'B':
		buf.WriteByte(' ')
		return pos + 1

	case '%':
		buf.WriteByte('%')
		return pos + 1

	case '#':
		// Cause/enactor dbref
		buf.WriteString(fmt.Sprintf("#%d", ctx.Cause))
		return pos + 1

	case '!':
		// Executor dbref
		buf.WriteString(fmt.Sprintf("#%d", ctx.Player))
		return pos + 1

	case '@':
		// Caller dbref
		buf.WriteString(fmt.Sprintf("#%d", ctx.Caller))
		return pos + 1

	case 'n', 'N':
		// Cause/enactor name
		if obj, ok := ctx.DB.Objects[ctx.Cause]; ok {
			name := obj.Name
			if ch == 'N' && len(name) > 0 {
				name = strings.ToUpper(name[:1]) + name[1:]
			}
			buf.WriteString(name)
		}
		return pos + 1

	case 'l', 'L':
		// Cause/enactor location
		if evalFlags&EvNoLocation == 0 {
			if obj, ok := ctx.DB.Objects[ctx.Cause]; ok {
				buf.WriteString(fmt.Sprintf("#%d", obj.Location))
			}
		}
		return pos + 1

	case 's', 'S':
		// Subjective pronoun
		pronoun := ctx.getPronoun(ctx.Cause, "subj")
		if ch == 'S' && len(pronoun) > 0 {
			pronoun = strings.ToUpper(pronoun[:1]) + pronoun[1:]
		}
		buf.WriteString(pronoun)
		return pos + 1

	case 'o', 'O':
		// Objective pronoun
		pronoun := ctx.getPronoun(ctx.Cause, "obj")
		if ch == 'O' && len(pronoun) > 0 {
			pronoun = strings.ToUpper(pronoun[:1]) + pronoun[1:]
		}
		buf.WriteString(pronoun)
		return pos + 1

	case 'p', 'P':
		// Possessive pronoun
		pronoun := ctx.getPronoun(ctx.Cause, "poss")
		if ch == 'P' && len(pronoun) > 0 {
			pronoun = strings.ToUpper(pronoun[:1]) + pronoun[1:]
		}
		buf.WriteString(pronoun)
		return pos + 1

	case 'a', 'A':
		// Absolute possessive
		pronoun := ctx.getPronoun(ctx.Cause, "aposs")
		if ch == 'A' && len(pronoun) > 0 {
			pronoun = strings.ToUpper(pronoun[:1]) + pronoun[1:]
		}
		buf.WriteString(pronoun)
		return pos + 1

	case 'q', 'Q':
		// Q-register: %q0-%q9, %qa-%qz, %q<name>
		pos++
		if pos >= len(input) {
			return pos
		}
		if input[pos] == '<' {
			// Named register: %q<name>
			pos++
			end := strings.IndexByte(input[pos:], '>')
			if end < 0 {
				return pos
			}
			name := strings.ToLower(input[pos : pos+end])
			if ctx.RData != nil {
				if val, ok := ctx.RData.XRegs[name]; ok {
					buf.WriteString(val)
				}
			}
			return pos + end + 1
		}
		// Single char register
		idx := qidxChar(input[pos])
		if idx >= 0 && idx < MaxGlobalRegs && ctx.RData != nil {
			buf.WriteString(ctx.RData.QRegs[idx])
		}
		return pos + 1

	case 'v', 'V':
		// VA-VZ attribute: %vA through %vZ
		pos++
		if pos >= len(input) {
			return pos
		}
		ch2 := unicode.ToUpper(rune(input[pos]))
		if ch2 >= 'A' && ch2 <= 'Z' {
			attrNum := 95 + int(ch2-'A') // A_VA = 95
			text := ctx.GetAttrText(ctx.Player, attrNum)
			buf.WriteString(text)
		}
		return pos + 1

	case 'x', 'X':
		// ANSI color: %xn, %xr, %x<208>, %x<#FF5733>, %x/<208>, etc.
		pos++
		if pos >= len(input) {
			return pos
		}
		if ctx.AnsiColors {
			// Check for background prefix: %x/<...>
			bg := false
			checkPos := pos
			if input[checkPos] == '/' {
				bg = true
				checkPos++
				if checkPos >= len(input) {
					return checkPos
				}
			}
			// Check for extended color: %x<...> or %x/<...>
			if input[checkPos] == '<' {
				end := strings.IndexByte(input[checkPos:], '>')
				if end < 0 {
					return checkPos + 1
				}
				spec := input[checkPos+1 : checkPos+end]
				code := ParseColorSpec(spec, bg)
				if code != "" {
					buf.WriteString(code)
					*ansi = true
				}
				return checkPos + end + 1
			}
			// Fall back to single-char lookup (only if no / prefix)
			if !bg {
				ansiCode := ansiCharLookup(input[pos])
				if ansiCode != "" {
					buf.WriteString(ansiCode)
					if input[pos] == 'n' || input[pos] == 'N' {
						*ansi = false
					} else {
						*ansi = true
					}
				} else {
					buf.WriteByte(input[pos])
				}
			}
		}
		return pos + 1

	case 'i', 'I', 'j', 'J':
		// itext/itext2: %i0, %i-0, etc.
		pos++
		if pos >= len(input) {
			return pos
		}
		isI := ch == 'i' || ch == 'I'
		if input[pos] == '-' {
			pos++
			if pos >= len(input) || !isDigit(input[pos]) {
				return pos
			}
			i := int(input[pos] - '0')
			if i <= ctx.Loop.InLoop-1 {
				if isI && len(ctx.Loop.LoopTokens) > i {
					buf.WriteString(ctx.Loop.LoopTokens[i])
				} else if !isI && len(ctx.Loop.LoopTokens2) > i {
					buf.WriteString(ctx.Loop.LoopTokens2[i])
				}
			}
			return pos + 1
		}
		if ctx.Loop.InLoop == 0 || !isDigit(input[pos]) {
			return pos
		}
		i := ctx.Loop.InLoop - 1 - int(input[pos]-'0')
		if i >= 0 && i < ctx.Loop.InLoop {
			if isI && len(ctx.Loop.LoopTokens) > i {
				buf.WriteString(ctx.Loop.LoopTokens[i])
			} else if !isI && len(ctx.Loop.LoopTokens2) > i {
				buf.WriteString(ctx.Loop.LoopTokens2[i])
			}
		}
		return pos + 1

	case 'm', 'M':
		// Current command
		buf.WriteString(ctx.CurrCmd)
		return pos + 1

	case '+':
		// Number of function args
		buf.WriteString(fmt.Sprintf("%d", len(cargs)))
		return pos + 1

	case '|':
		// Piped output
		buf.WriteString(ctx.PipeOut)
		return pos + 1

	default:
		buf.WriteByte(ch)
		return pos + 1
	}
}

// parseTo finds a delimiter character while respecting nesting of [], (), {}.
// Returns the content before the delimiter, the position of the delimiter, and whether it was found.
func parseTo(input string, pos int, delim byte) (string, int, bool) {
	var stack []byte
	bracketLev := 0
	start := pos

	for pos < len(input) {
		ch := input[pos]

		switch ch {
		case '\\', '%':
			pos++ // skip the escape char
			if pos < len(input) {
				pos++ // skip the escaped char
			}
			continue

		case '{':
			bracketLev = 1
			pos++
			for pos < len(input) && bracketLev > 0 {
				switch input[pos] {
				case '\\', '%':
					pos++
					if pos < len(input) {
						pos++
					}
					continue
				case '{':
					bracketLev++
				case '}':
					bracketLev--
				}
				if bracketLev > 0 {
					pos++
				}
			}
			if bracketLev == 0 {
				pos++ // skip closing }
			}
			continue

		case '[':
			stack = append(stack, ']')
		case '(':
			stack = append(stack, ')')

		case ']', ')':
			// Check if it unwinds the stack
			found := false
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i] == ch {
					stack = stack[:i]
					found = true
					break
				}
			}
			if !found && ch == delim {
				return input[start:pos], pos, true
			}

		default:
			if ch == delim && len(stack) == 0 {
				return input[start:pos], pos, true
			}
		}
		pos++
	}
	return input[start:], pos, false
}

// parseArgList splits a function argument string on commas, respecting nesting.
// Returns the list of argument strings, the position of the closing delimiter, and whether it was found.
func parseArgList(input string, pos int, closingDelim byte) ([]string, int, bool) {
	var args []string
	start := pos
	depth := 0
	bracketLev := 0

	for pos < len(input) {
		ch := input[pos]

		switch ch {
		case '\\', '%':
			pos++
			if pos < len(input) {
				pos++
			}
			continue

		case '{':
			bracketLev = 1
			pos++
			for pos < len(input) && bracketLev > 0 {
				switch input[pos] {
				case '\\', '%':
					pos++
					if pos < len(input) {
						pos++
					}
					continue
				case '{':
					bracketLev++
				case '}':
					bracketLev--
				}
				if bracketLev > 0 {
					pos++
				}
			}
			if bracketLev == 0 {
				pos++
			}
			continue

		case '[':
			depth++
		case ']':
			depth--
		case '(':
			depth++
		case ')':
			if depth == 0 && ch == closingDelim {
				args = append(args, input[start:pos])
				return args, pos, true
			}
			depth--

		case ',':
			if depth == 0 {
				args = append(args, input[start:pos])
				start = pos + 1
			}
		}
		pos++
	}

	// No closing delimiter found
	return nil, pos, false
}

// isSpecial returns true for characters that need special processing in the eval loop.
func isSpecial(ch byte) bool {
	switch ch {
	case 0, '\033', ' ', '\\', '[', '{', '(', '%', '#':
		return true
	}
	return false
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// qidxChar converts a register character (0-9, a-z) to an index (0-35).
func qidxChar(ch byte) int {
	if ch >= '0' && ch <= '9' {
		return int(ch - '0')
	}
	if ch >= 'a' && ch <= 'z' {
		return int(ch-'a') + 10
	}
	if ch >= 'A' && ch <= 'Z' {
		return int(ch-'A') + 10
	}
	return -1
}

// getPronoun returns a pronoun based on the object's SEX attribute.
func (ctx *EvalContext) getPronoun(obj gamedb.DBRef, ptype string) string {
	sex := ctx.GetAttrText(obj, 7) // A_SEX = 7
	gender := 1                     // neuter
	if len(sex) > 0 {
		switch sex[0] {
		case 'M', 'm':
			gender = 3
		case 'F', 'f', 'W', 'w':
			gender = 2
		case 'P', 'p':
			gender = 4
		}
	}

	subj := []string{"", "it", "she", "he", "they"}
	poss := []string{"", "its", "her", "his", "their"}
	objp := []string{"", "it", "her", "him", "them"}
	aposs := []string{"", "its", "hers", "his", "theirs"}

	switch ptype {
	case "subj":
		if gender == 0 {
			if o, ok := ctx.DB.Objects[obj]; ok {
				return o.Name
			}
		}
		return subj[gender]
	case "poss":
		if gender == 0 {
			if o, ok := ctx.DB.Objects[obj]; ok {
				return o.Name + "s"
			}
		}
		return poss[gender]
	case "obj":
		if gender == 0 {
			if o, ok := ctx.DB.Objects[obj]; ok {
				return o.Name
			}
		}
		return objp[gender]
	case "aposs":
		if gender == 0 {
			if o, ok := ctx.DB.Objects[obj]; ok {
				return o.Name + "s"
			}
		}
		return aposs[gender]
	}
	return ""
}

// ansiCharLookup maps a character from %x? to an ANSI escape code.
func ansiCharLookup(ch byte) string {
	switch ch {
	case 'n', 'N':
		return "\033[0m"
	case 'h', 'H':
		return "\033[1m"
	case 'i', 'I':
		return "\033[7m"
	case 'f', 'F':
		return "\033[5m"
	case 'u', 'U':
		return "\033[4m"
	case 'x', 'X':
		return "\033[30m"
	case 'r', 'R':
		return "\033[31m"
	case 'g', 'G':
		return "\033[32m"
	case 'y', 'Y':
		return "\033[33m"
	case 'b', 'B':
		return "\033[34m"
	case 'm', 'M':
		return "\033[35m"
	case 'c', 'C':
		return "\033[36m"
	case 'w', 'W':
		return "\033[37m"
	}
	return ""
}
