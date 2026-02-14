package functions

import (
	"fmt"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnLwho returns a space-separated list of connected player dbrefs.
func fnLwho(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	players := ctx.GameState.ConnectedPlayers()
	var refs []string
	for _, p := range players {
		refs = append(refs, fmt.Sprintf("#%d", p))
	}
	buf.WriteString(strings.Join(refs, " "))
}

// fnMwho returns connected players visible to the executor (excludes DARK/UNFINDABLE).
func fnMwho(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		return
	}
	players := ctx.GameState.ConnectedPlayersVisible(ctx.Player)
	var refs []string
	for _, p := range players {
		refs = append(refs, fmt.Sprintf("#%d", p))
	}
	buf.WriteString(strings.Join(refs, " "))
}

// fnConn returns connection time in seconds for a player.
func fnConn(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.GameState == nil {
		buf.WriteString("-1")
		return
	}
	ref := resolveDBRef(ctx, args[0])
	secs := ctx.GameState.ConnTime(ref)
	writeInt(buf, int(secs))
}

// fnIdle returns idle time in seconds for a player.
func fnIdleFn(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.GameState == nil {
		buf.WriteString("-1")
		return
	}
	ref := resolveDBRef(ctx, args[0])
	secs := ctx.GameState.IdleTime(ref)
	writeInt(buf, int(secs))
}

// fnDoingFn returns a player's @doing string.
func fnDoingFn(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 || ctx.GameState == nil {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	buf.WriteString(ctx.GameState.DoingString(ref))
}

// fnPmatch matches a player name (partial) to a dbref.
func fnPmatch(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("#-1 NOT FOUND")
		return
	}
	name := strings.TrimSpace(args[0])
	if name == "" {
		buf.WriteString("#-1 NOT FOUND")
		return
	}
	// Handle #dbref
	if name[0] == '#' {
		ref := resolveDBRef(ctx, name)
		if obj, ok := ctx.DB.Objects[ref]; ok && obj.ObjType() == gamedb.TypePlayer {
			buf.WriteString(fmt.Sprintf("#%d", ref))
		} else {
			buf.WriteString("#-1 NOT FOUND")
		}
		return
	}
	// Handle "me"
	if strings.EqualFold(name, "me") {
		buf.WriteString(fmt.Sprintf("#%d", ctx.Player))
		return
	}
	// Use GameState if available, otherwise fall back to DB scan
	if ctx.GameState != nil {
		ref := ctx.GameState.LookupPlayer(name)
		if ref == gamedb.Ambiguous {
			buf.WriteString("#-2 AMBIGUOUS")
		} else if ref == gamedb.Nothing {
			buf.WriteString("#-1 NOT FOUND")
		} else {
			buf.WriteString(fmt.Sprintf("#%d", ref))
		}
		return
	}
	// Fallback: exact match
	ref := resolveDBRef(ctx, name)
	buf.WriteString(fmt.Sprintf("#%d", ref))
}

// fnHastype checks if an object is of a particular type.
func fnHastype(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("0")
		return
	}
	ref := resolveDBRef(ctx, args[0])
	obj, ok := ctx.DB.Objects[ref]
	if !ok {
		buf.WriteString("0")
		return
	}
	typeName := strings.ToUpper(strings.TrimSpace(args[1]))
	var match bool
	switch typeName {
	case "ROOM":
		match = obj.ObjType() == gamedb.TypeRoom
	case "PLAYER":
		match = obj.ObjType() == gamedb.TypePlayer
	case "EXIT":
		match = obj.ObjType() == gamedb.TypeExit
	case "THING":
		match = obj.ObjType() == gamedb.TypeThing
	}
	buf.WriteString(boolToStr(match))
}

// fnChildren returns a space-separated list of children (objects with this as parent).
func fnChildren(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	var children []string
	for _, obj := range ctx.DB.Objects {
		if obj.Parent == ref && !obj.IsGoing() {
			children = append(children, fmt.Sprintf("#%d", obj.DBRef))
		}
	}
	buf.WriteString(strings.Join(children, " "))
}

// fnLparent returns the parent chain of an object (space-separated).
func fnLparent(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	var chain []string
	chain = append(chain, fmt.Sprintf("#%d", ref))
	visited := make(map[gamedb.DBRef]bool)
	visited[ref] = true
	current := ref
	for i := 0; i < 100; i++ {
		obj, ok := ctx.DB.Objects[current]
		if !ok || obj.Parent == gamedb.Nothing {
			break
		}
		if visited[obj.Parent] {
			break
		}
		visited[obj.Parent] = true
		chain = append(chain, fmt.Sprintf("#%d", obj.Parent))
		current = obj.Parent
	}
	buf.WriteString(strings.Join(chain, " "))
}

// fnEntrances returns exits that link to this object.
func fnEntrances(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	ref := resolveDBRef(ctx, args[0])
	var entrances []string
	for _, obj := range ctx.DB.Objects {
		if obj.ObjType() == gamedb.TypeExit && obj.Location == ref && !obj.IsGoing() {
			entrances = append(entrances, fmt.Sprintf("#%d", obj.DBRef))
		}
	}
	buf.WriteString(strings.Join(entrances, " "))
}

// fnLocate does advanced object matching: locate(looker, name, type)
func fnLocate(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 NOT FOUND")
		return
	}
	looker := resolveDBRef(ctx, args[0])
	name := strings.TrimSpace(args[1])
	typeFilter := ""
	if len(args) > 2 {
		typeFilter = strings.ToUpper(strings.TrimSpace(args[2]))
	}

	// Handle special tokens
	if strings.EqualFold(name, "me") {
		buf.WriteString(fmt.Sprintf("#%d", looker))
		return
	}
	if strings.EqualFold(name, "here") {
		if obj, ok := ctx.DB.Objects[looker]; ok {
			buf.WriteString(fmt.Sprintf("#%d", obj.Location))
		} else {
			buf.WriteString("#-1")
		}
		return
	}
	if name[0] == '#' {
		ref := resolveDBRef(ctx, name)
		if _, ok := ctx.DB.Objects[ref]; ok {
			buf.WriteString(fmt.Sprintf("#%d", ref))
		} else {
			buf.WriteString("#-1 NOT FOUND")
		}
		return
	}
	if name[0] == '*' {
		// Player match
		ref := resolveDBRef(ctx, name)
		buf.WriteString(fmt.Sprintf("#%d", ref))
		return
	}

	// Search: inventory, location contents, location exits
	lookerObj, ok := ctx.DB.Objects[looker]
	if !ok {
		buf.WriteString("#-1 NOT FOUND")
		return
	}

	matchType := func(obj *gamedb.Object) bool {
		if typeFilter == "" {
			return true
		}
		for _, ch := range typeFilter {
			switch ch {
			case 'R':
				if obj.ObjType() == gamedb.TypeRoom {
					return true
				}
			case 'E':
				if obj.ObjType() == gamedb.TypeExit {
					return true
				}
			case 'P':
				if obj.ObjType() == gamedb.TypePlayer {
					return true
				}
			case 'T':
				if obj.ObjType() == gamedb.TypeThing {
					return true
				}
			case '*':
				return true
			}
		}
		return false
	}

	// Search inventory
	next := lookerObj.Contents
	for next != gamedb.Nothing {
		if obj, ok := ctx.DB.Objects[next]; ok {
			if strings.EqualFold(obj.Name, name) && matchType(obj) {
				buf.WriteString(fmt.Sprintf("#%d", next))
				return
			}
			next = obj.Next
		} else {
			break
		}
	}

	// Search room contents
	loc := lookerObj.Location
	if locObj, ok := ctx.DB.Objects[loc]; ok {
		next = locObj.Contents
		for next != gamedb.Nothing {
			if obj, ok := ctx.DB.Objects[next]; ok {
				if strings.EqualFold(obj.Name, name) && matchType(obj) {
					buf.WriteString(fmt.Sprintf("#%d", next))
					return
				}
				next = obj.Next
			} else {
				break
			}
		}
		// Search exits
		next = locObj.Exits
		for next != gamedb.Nothing {
			if obj, ok := ctx.DB.Objects[next]; ok {
				exitNames := strings.Split(obj.Name, ";")
				for _, ename := range exitNames {
					if strings.EqualFold(strings.TrimSpace(ename), name) && matchType(obj) {
						buf.WriteString(fmt.Sprintf("#%d", next))
						return
					}
				}
				next = obj.Next
			} else {
				break
			}
		}
	}

	buf.WriteString("#-1 NOT FOUND")
}

// fnRloc returns the room containing an object (walks up locations).
func fnRloc(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("#-1")
		return
	}
	ref := resolveDBRef(ctx, args[0])
	maxDepth := 20
	if len(args) > 1 {
		maxDepth = toInt(args[1])
		if maxDepth < 1 {
			maxDepth = 1
		}
		if maxDepth > 100 {
			maxDepth = 100
		}
	}
	for i := 0; i < maxDepth; i++ {
		obj, ok := ctx.DB.Objects[ref]
		if !ok {
			buf.WriteString("#-1")
			return
		}
		if obj.ObjType() == gamedb.TypeRoom {
			buf.WriteString(fmt.Sprintf("#%d", ref))
			return
		}
		if obj.Location == gamedb.Nothing {
			break
		}
		ref = obj.Location
	}
	buf.WriteString(fmt.Sprintf("#%d", ref))
}

// fnNearby checks if two objects are near each other (same room or one contains the other).
func fnNearby(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("0")
		return
	}
	ref1 := resolveDBRef(ctx, args[0])
	ref2 := resolveDBRef(ctx, args[1])
	obj1, ok1 := ctx.DB.Objects[ref1]
	obj2, ok2 := ctx.DB.Objects[ref2]
	if !ok1 || !ok2 {
		buf.WriteString("0")
		return
	}

	// Same location, or one is in the other, or one is the other's location
	if obj1.Location == obj2.Location ||
		obj1.Location == ref2 ||
		obj2.Location == ref1 ||
		ref1 == ref2 {
		buf.WriteString("1")
	} else {
		buf.WriteString("0")
	}
}

// visLen returns the visual (display) length of a string, ignoring ANSI
// escape sequences (\033[...m) which occupy zero columns.
func visLen(s string) int {
	n := 0
	inEsc := false
	for i := 0; i < len(s); i++ {
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			inEsc = true
			i++ // skip '['
			continue
		}
		n++
	}
	return n
}

// fnWrap performs word-wrapping at a given width.
//
//	wrap(<text>, <width>[, <left indent>[, <hanging indent>]])
//
// Wraps text to the specified width (minimum 1, default 78), breaking on word
// boundaries when possible. Words longer than the available line width are
// hard-broken. Existing newlines in the input are preserved as paragraph
// breaks. ANSI escape sequences are not counted toward line width.
func fnWrap(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	text := args[0]
	width := 78
	if len(args) >= 2 {
		width = toInt(args[1])
	}
	if width < 1 {
		width = 1
	}

	// Optional indentation
	leftIndent := ""
	hangIndent := ""
	if len(args) >= 3 {
		n := toInt(args[2])
		if n > 0 && n < width {
			leftIndent = strings.Repeat(" ", n)
		}
	}
	if len(args) >= 4 {
		n := toInt(args[3])
		if n > 0 && n < width {
			hangIndent = strings.Repeat(" ", n)
		}
	}

	// Split on existing newlines to preserve paragraph structure
	paragraphs := strings.Split(text, "\n")
	for pi, para := range paragraphs {
		if pi > 0 {
			buf.WriteString("\r\n")
		}
		// Strip trailing \r from \r\n splits
		para = strings.TrimRight(para, "\r")
		wrapParagraph(buf, para, width, leftIndent, hangIndent)
	}
}

// wrapParagraph wraps a single paragraph (no embedded newlines) to width.
func wrapParagraph(buf *strings.Builder, para string, width int, leftIndent, hangIndent string) {
	words := strings.Fields(para)
	if len(words) == 0 {
		return
	}

	leftW := visLen(leftIndent)
	hangW := visLen(hangIndent)
	firstLine := true
	lineLen := 0

	for _, word := range words {
		wl := visLen(word)

		// Determine current indent
		indent := hangIndent
		indentW := hangW
		if firstLine {
			indent = leftIndent
			indentW = leftW
		}
		avail := width - indentW

		if lineLen == 0 {
			// Start of a new line
			buf.WriteString(indent)
			// If word fits, write it
			if wl <= avail {
				buf.WriteString(word)
				lineLen = wl
			} else {
				// Hard-break long word
				hardBreak(buf, word, avail, width, hangIndent, hangW)
				lineLen = 0 // hardBreak ends mid-line or at a break
			}
			firstLine = false
		} else if lineLen+1+wl <= avail {
			// Word fits on current line
			buf.WriteByte(' ')
			buf.WriteString(word)
			lineLen += 1 + wl
		} else {
			// Wrap to next line
			buf.WriteString("\r\n")
			buf.WriteString(hangIndent)
			if wl <= width-hangW {
				buf.WriteString(word)
				lineLen = wl
			} else {
				hardBreak(buf, word, width-hangW, width, hangIndent, hangW)
				lineLen = 0
			}
		}
	}
}

// hardBreak writes a word that's longer than the available width, splitting
// it across multiple lines. It is ANSI-aware: escape sequences are never
// split and don't count toward width.
func hardBreak(buf *strings.Builder, word string, firstAvail, width int, indent string, indentW int) {
	avail := firstAvail
	col := 0
	inEsc := false
	for i := 0; i < len(word); i++ {
		ch := word[i]
		if inEsc {
			buf.WriteByte(ch)
			if ch == 'm' {
				inEsc = false
			}
			continue
		}
		if ch == '\033' && i+1 < len(word) && word[i+1] == '[' {
			buf.WriteByte(ch)
			inEsc = true
			continue
		}
		if col >= avail {
			buf.WriteString("\r\n")
			buf.WriteString(indent)
			col = 0
			avail = width - indentW
		}
		buf.WriteByte(ch)
		col++
	}
}

// fnColumns formats text into columns.
func fnColumns(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		return
	}
	delim := " "
	if len(args) > 2 && args[2] != "" {
		delim = args[2]
	}
	items := splitList(args[0], delim)
	colWidth := toInt(args[1])
	if colWidth < 1 {
		colWidth = 20
	}
	lineWidth := 78
	if len(args) > 3 {
		lineWidth = toInt(args[3])
		if lineWidth < 1 {
			lineWidth = 78
		}
	}

	colsPerRow := lineWidth / colWidth
	if colsPerRow < 1 {
		colsPerRow = 1
	}

	for i, item := range items {
		if i > 0 && i%colsPerRow == 0 {
			buf.WriteString("\r\n")
		}
		// Left-justify within column
		buf.WriteString(item)
		padding := colWidth - len(item)
		if padding > 0 && (i+1)%colsPerRow != 0 && i+1 < len(items) {
			buf.WriteString(strings.Repeat(" ", padding))
		}
	}
}

// fnTable is an alias for columns.
func fnTable(ctx *eval.EvalContext, args []string, buf *strings.Builder, caller, cause gamedb.DBRef) {
	fnColumns(ctx, args, buf, caller, cause)
}

// fnTables implements tables(list, field_widths[, lead_str[, trail_str[, list_sep[, field_sep[, pad]]]]])
// Formats a list into a table with variable column widths, left-justified.
func fnTables(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	processTables(args, buf, 0) // 0 = left justify
}

// fnRtables is right-justified tables.
func fnRtables(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	processTables(args, buf, 1) // 1 = right justify
}

// fnCtables is center-justified tables.
func fnCtables(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	processTables(args, buf, 2) // 2 = center justify
}

// processTables is the shared implementation for tables/rtables/ctables.
// just: 0=left, 1=right, 2=center
func processTables(args []string, buf *strings.Builder, just int) {
	if len(args) < 2 {
		return
	}

	// Parse column widths (space-separated)
	widthStrs := strings.Fields(args[1])
	if len(widthStrs) == 0 {
		return
	}
	colWidths := make([]int, len(widthStrs))
	for i, ws := range widthStrs {
		w := toInt(ws)
		if w < 1 {
			w = 1
		}
		colWidths[i] = w
	}
	nCols := len(colWidths)

	// Optional parameters
	leadStr := ""
	if len(args) > 2 {
		leadStr = args[2]
	}
	trailStr := ""
	if len(args) > 3 {
		trailStr = args[3]
	}
	listSep := " "
	if len(args) > 4 && args[4] != "" {
		listSep = args[4]
	}
	fieldSep := " "
	if len(args) > 5 {
		fieldSep = args[5]
	}
	padChar := " "
	if len(args) > 6 && args[6] != "" {
		padChar = string(args[6][0])
	}

	// Split the list
	words := splitList(args[0], listSep)
	if len(words) == 0 {
		return
	}

	// Format into rows
	col := 0
	for i, word := range words {
		if col == 0 && leadStr != "" {
			buf.WriteString(leadStr)
		}

		// Calculate visible length (strip ANSI for width calculation)
		visLen := ansiStrLen(word)
		width := colWidths[col%nCols]

		// Justify the word within the column
		padding := width - visLen
		if padding < 0 {
			padding = 0
			// Truncate to column width
			word = ansiTruncate(word, width)
		}

		switch just {
		case 1: // right
			if padding > 0 {
				buf.WriteString(strings.Repeat(padChar, padding))
			}
			buf.WriteString(word)
		case 2: // center
			leftPad := padding / 2
			rightPad := padding - leftPad
			if leftPad > 0 {
				buf.WriteString(strings.Repeat(padChar, leftPad))
			}
			buf.WriteString(word)
			if rightPad > 0 && col+1 < nCols && i+1 < len(words) {
				buf.WriteString(strings.Repeat(padChar, rightPad))
			}
		default: // left
			buf.WriteString(word)
			if padding > 0 && col+1 < nCols && i+1 < len(words) {
				buf.WriteString(strings.Repeat(padChar, padding))
			}
		}

		col++
		if col >= nCols {
			// End of row
			if trailStr != "" {
				buf.WriteString(trailStr)
			}
			if i+1 < len(words) {
				buf.WriteString("\r\n")
			}
			col = 0
		} else if i+1 < len(words) {
			// Field separator between columns
			buf.WriteString(fieldSep)
		}
	}
	// Handle trailing partial row
	if col > 0 && trailStr != "" {
		buf.WriteString(trailStr)
	}
}

// ansiStrLen returns the visible length of a string, not counting ANSI escape sequences.
func ansiStrLen(s string) int {
	n := 0
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ANSI escape sequence: ESC [ ... letter
			i += 2
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			if i < len(s) {
				i++ // skip the final letter
			}
			continue
		}
		n++
		i++
	}
	return n
}

// ansiTruncate truncates a string to maxVisible visible characters, preserving ANSI sequences.
func ansiTruncate(s string, maxVisible int) string {
	var result strings.Builder
	vis := 0
	i := 0
	for i < len(s) && vis < maxVisible {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Copy entire ANSI escape sequence
			start := i
			i += 2
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			if i < len(s) {
				i++
			}
			result.WriteString(s[start:i])
			continue
		}
		result.WriteByte(s[i])
		vis++
		i++
	}
	return result.String()
}
