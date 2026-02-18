package functions

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func splitList(s, delim string) []string {
	if s == "" {
		return nil
	}
	if delim == "" || delim == " " {
		return strings.Fields(s)
	}
	return strings.Split(s, delim)
}

func fnWords(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { writeInt(buf, 0); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	writeInt(buf, len(words))
}

func fnFirst(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) > 0 { buf.WriteString(words[0]) }
}

func fnRest(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) > 1 {
		buf.WriteString(strings.Join(words[1:], delim))
	}
}

func fnLast(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) > 0 { buf.WriteString(words[len(words)-1]) }
}

func fnExtract(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	delim := " "
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	words := splitList(args[0], delim)
	start := toInt(args[1]) - 1 // 1-indexed in MUSH
	count := toInt(args[2])
	if start < 0 { start = 0 }
	if start >= len(words) { return }
	end := start + count
	if end > len(words) { end = len(words) }
	buf.WriteString(strings.Join(words[start:end], delim))
}

func fnElements(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	outDelim := delim
	if len(args) > 3 && args[3] != "" { outDelim = args[3] }
	words := splitList(args[0], delim)
	positions := strings.Fields(args[1])
	var result []string
	for _, posStr := range positions {
		pos := toInt(posStr) - 1
		if pos >= 0 && pos < len(words) {
			result = append(result, words[pos])
		}
	}
	buf.WriteString(strings.Join(result, outDelim))
}

func fnLnum(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	start := 0
	end := toInt(args[0])
	if len(args) > 1 {
		start = toInt(args[0])
		end = toInt(args[1])
	}
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	var nums []string
	if start <= end {
		for i := start; i < end; i++ {
			nums = append(nums, strconv.Itoa(i))
		}
	} else {
		for i := start; i > end; i-- {
			nums = append(nums, strconv.Itoa(i))
		}
	}
	buf.WriteString(strings.Join(nums, delim))
}

func fnMember(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	target := args[1]
	for i, w := range words {
		if strings.EqualFold(w, target) {
			writeInt(buf, i+1)
			return
		}
	}
	buf.WriteString("0")
}

func fnRemove(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	target := args[1]
	found := false
	var result []string
	for _, w := range words {
		if !found && strings.EqualFold(w, target) {
			found = true
			continue
		}
		result = append(result, w)
	}
	buf.WriteString(strings.Join(result, delim))
}

func fnInsert(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	delim := " "
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	words := splitList(args[0], delim)
	pos := toInt(args[1])
	if pos < 0 { pos = 0 }
	newWord := args[2]
	if pos >= len(words) {
		words = append(words, newWord)
	} else {
		words = append(words[:pos], append([]string{newWord}, words[pos:]...)...)
	}
	buf.WriteString(strings.Join(words, delim))
}

func fnLdelete(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	positions := strings.Fields(args[1])
	// Build set of positions to delete (1-indexed)
	deleteSet := make(map[int]bool)
	for _, p := range positions {
		deleteSet[toInt(p)-1] = true
	}
	var result []string
	for i, w := range words {
		if !deleteSet[i] {
			result = append(result, w)
		}
	}
	buf.WriteString(strings.Join(result, delim))
}

func fnSort(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	sortType := "a" // alphabetic
	if len(args) > 2 && args[2] != "" { sortType = strings.ToLower(args[2]) }
	words := splitList(args[0], delim)
	switch sortType {
	case "n", "i": // numeric or integer
		sort.Slice(words, func(i, j int) bool {
			return toFloat(words[i]) < toFloat(words[j])
		})
	case "d": // dbref
		sort.Slice(words, func(i, j int) bool {
			return parseDBRefNum(words[i]) < parseDBRefNum(words[j])
		})
	default: // alphabetic
		sort.Slice(words, func(i, j int) bool {
			return strings.ToLower(words[i]) < strings.ToLower(words[j])
		})
	}
	buf.WriteString(strings.Join(words, delim))
}

func parseDBRefNum(s string) int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	n, _ := strconv.Atoi(s)
	return n
}

func fnSetunion(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	outDelim := delim
	if len(args) > 3 && args[3] != "" { outDelim = args[3] }
	a := splitList(args[0], delim)
	b := splitList(args[1], delim)
	seen := make(map[string]bool)
	var result []string
	for _, w := range a {
		lw := strings.ToLower(w)
		if !seen[lw] { seen[lw] = true; result = append(result, w) }
	}
	for _, w := range b {
		lw := strings.ToLower(w)
		if !seen[lw] { seen[lw] = true; result = append(result, w) }
	}
	buf.WriteString(strings.Join(result, outDelim))
}

func fnSetdiff(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	outDelim := delim
	if len(args) > 3 && args[3] != "" { outDelim = args[3] }
	b := splitList(args[1], delim)
	bSet := make(map[string]bool)
	for _, w := range b { bSet[strings.ToLower(w)] = true }
	a := splitList(args[0], delim)
	var result []string
	seen := make(map[string]bool)
	for _, w := range a {
		lw := strings.ToLower(w)
		if !bSet[lw] && !seen[lw] { seen[lw] = true; result = append(result, w) }
	}
	buf.WriteString(strings.Join(result, outDelim))
}

func fnSetinter(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	outDelim := delim
	if len(args) > 3 && args[3] != "" { outDelim = args[3] }
	b := splitList(args[1], delim)
	bSet := make(map[string]bool)
	for _, w := range b { bSet[strings.ToLower(w)] = true }
	a := splitList(args[0], delim)
	var result []string
	seen := make(map[string]bool)
	for _, w := range a {
		lw := strings.ToLower(w)
		if bSet[lw] && !seen[lw] { seen[lw] = true; result = append(result, w) }
	}
	buf.WriteString(strings.Join(result, outDelim))
}

func fnRevwords(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	buf.WriteString(strings.Join(words, delim))
}

func fnShuffle(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	buf.WriteString(strings.Join(words, delim))
}

func fnItemize(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	conj := "and"
	if len(args) > 2 && args[2] != "" { conj = args[2] }
	punc := ","
	if len(args) > 3 && args[3] != "" { punc = args[3] }
	words := splitList(args[0], delim)
	switch len(words) {
	case 0:
		return
	case 1:
		buf.WriteString(words[0])
	case 2:
		buf.WriteString(words[0] + " " + conj + " " + words[1])
	default:
		buf.WriteString(strings.Join(words[:len(words)-1], punc+" "))
		buf.WriteString(punc + " " + conj + " " + words[len(words)-1])
	}
}

func fnSplice(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	delim := " "
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	list1 := splitList(args[0], delim)
	list2 := splitList(args[1], delim)
	word := args[2]
	var result []string
	for i, w := range list1 {
		if strings.EqualFold(w, word) && i < len(list2) {
			result = append(result, list2[i])
		} else {
			result = append(result, w)
		}
	}
	buf.WriteString(strings.Join(result, delim))
}

func fnGrab(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	pattern := args[1]
	for _, w := range words {
		if wildMatch(pattern, w) {
			buf.WriteString(w)
			return
		}
	}
}

func fnGraball(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	outDelim := delim
	if len(args) > 3 && args[3] != "" { outDelim = args[3] }
	words := splitList(args[0], delim)
	pattern := args[1]
	var result []string
	for _, w := range words {
		if wildMatch(pattern, w) {
			result = append(result, w)
		}
	}
	buf.WriteString(strings.Join(result, outDelim))
}

// fnChoose — weighted random selection from a list.
// choose(list, weights[, delim])
func fnChoose(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	items := splitList(args[0], delim)
	weights := splitList(args[1], delim)
	if len(items) == 0 { return }
	totalWeight := 0.0
	ws := make([]float64, len(items))
	for i := range items {
		w := 1.0
		if i < len(weights) { w = toFloat(weights[i]); if w <= 0 { w = 1.0 } }
		ws[i] = w
		totalWeight += w
	}
	r := rand.Float64() * totalWeight
	cum := 0.0
	for i, w := range ws {
		cum += w
		if r < cum {
			buf.WriteString(items[i])
			return
		}
	}
	buf.WriteString(items[len(items)-1])
}

// fnGroup — group list elements into N-element groups.
// group(list, n[, delim[, odelim[, gdelim]]])
func fnGroup(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	odelim := delim
	if len(args) > 3 && args[3] != "" { odelim = args[3] }
	gdelim := "|"
	if len(args) > 4 && args[4] != "" { gdelim = args[4] }
	words := splitList(args[0], delim)
	n := toInt(args[1])
	if n <= 0 { n = 1 }
	var groups []string
	for i := 0; i < len(words); i += n {
		end := i + n
		if end > len(words) { end = len(words) }
		groups = append(groups, strings.Join(words[i:end], odelim))
	}
	buf.WriteString(strings.Join(groups, gdelim))
}

// fnWildgrep — grep attrs using wildcard matching (not substring).
// wildgrep(object, attr-pattern, search-wildcard)
func fnWildgrep(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	ref := resolveDBRef(ctx, args[0])
	if ref == gamedb.Nothing { return }
	obj, ok := ctx.DB.Objects[ref]
	if !ok { return }
	attrPattern := args[1]
	searchPattern := args[2]
	var results []string
	for _, attr := range obj.Attrs {
		attrName := ""
		if def, ok := ctx.DB.AttrNames[attr.Number]; ok {
			attrName = def.Name
		} else if wk, ok := gamedb.WellKnownAttrs[attr.Number]; ok {
			attrName = wk
		}
		if attrName == "" { continue }
		if !wildMatch(attrPattern, attrName) { continue }
		text := eval.StripAttrPrefix(attr.Value)
		if wildMatch(searchPattern, text) {
			results = append(results, attrName)
		}
	}
	buf.WriteString(strings.Join(results, " "))
}

// Aggregate list functions

func fnLadd(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	sum := 0.0
	for _, w := range words {
		sum += toFloat(w)
	}
	writeFloat(buf, sum)
}

func fnLand(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("1"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	for _, w := range words {
		if toInt(w) == 0 { buf.WriteString("0"); return }
	}
	buf.WriteString("1")
}

func fnLor(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	for _, w := range words {
		if toInt(w) != 0 { buf.WriteString("1"); return }
	}
	buf.WriteString("0")
}

// fnSortby — sort a list using a user-defined comparison function.
// sortby(sortfn, list[, delim])
func fnSortby(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	sortFn := args[0]
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[1], delim)
	if len(words) <= 1 {
		buf.WriteString(args[1])
		return
	}
	sort.SliceStable(words, func(i, j int) bool {
		result := ctx.CallUFun(sortFn, []string{words[i], words[j]})
		return toInt(result) < 0
	})
	buf.WriteString(strings.Join(words, delim))
}

// fnLreplace — replace element(s) in a list.
// lreplace(list, pos, [count], newelem[, delim])
func fnLreplace(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	delim := " "
	// Determine argument layout:
	// lreplace(list, position, new-elements[, delim])
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	words := splitList(args[0], delim)
	pos := toInt(args[1]) - 1 // 1-indexed
	newElems := splitList(args[2], delim)
	if pos < 0 { pos = 0 }
	if pos >= len(words) {
		words = append(words, newElems...)
	} else {
		end := pos + 1
		if end > len(words) { end = len(words) }
		result := make([]string, 0, len(words)+len(newElems))
		result = append(result, words[:pos]...)
		result = append(result, newElems...)
		result = append(result, words[end:]...)
		words = result
	}
	buf.WriteString(strings.Join(words, delim))
}

// fnLedit — apply edit() to every element in a list.
// ledit(list, from, to[, delim])
func fnLedit(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { return }
	delim := " "
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	words := splitList(args[0], delim)
	from := args[1]
	to := args[2]
	for i, w := range words {
		words[i] = strings.ReplaceAll(w, from, to)
	}
	buf.WriteString(strings.Join(words, delim))
}

// fnIsort — case-insensitive alphabetic sort.
// isort(list[, delim])
func fnIsort(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	sort.SliceStable(words, func(i, j int) bool {
		return strings.ToLower(words[i]) < strings.ToLower(words[j])
	})
	buf.WriteString(strings.Join(words, delim))
}

// fnMerge — character-level string merge (C TinyMUSH merge()).
// merge(str1, str2, char) — walk both strings (must be equal length),
// where str1 has <char>, output the corresponding character from str2;
// otherwise output str1's character.
func fnMerge(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		return
	}
	s1 := args[0]
	s2 := args[1]
	if len(s1) != len(s2) {
		buf.WriteString("#-1 STRING LENGTHS MUST BE EQUAL")
		return
	}
	carg := args[2]
	if len(carg) > 1 {
		buf.WriteString("#-1 TOO MANY CHARACTERS")
		return
	}
	// Empty char arg treated as space
	c := byte(' ')
	if len(carg) == 1 {
		c = carg[0]
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] == c {
			buf.WriteByte(s2[i])
		} else {
			buf.WriteByte(s1[i])
		}
	}
}

// --- RhostMUSH extension list functions ---

// fnLavg — average of a list of numbers.
// lavg(list[, delim]) → float average
func fnLavg(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { buf.WriteString("0"); return }
	sum := 0.0
	for _, w := range words { sum += toFloat(w) }
	writeFloat(buf, sum/float64(len(words)))
}

// fnLsub — subtract all elements in a list from the first.
// lsub(list[, delim]) → first - second - third - ...
func fnLsub(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { buf.WriteString("0"); return }
	result := toFloat(words[0])
	for i := 1; i < len(words); i++ { result -= toFloat(words[i]) }
	writeFloat(buf, result)
}

// fnLmul — multiply all elements in a list.
// lmul(list[, delim]) → product
func fnLmul(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { buf.WriteString("0"); return }
	result := toFloat(words[0])
	for i := 1; i < len(words); i++ { result *= toFloat(words[i]) }
	writeFloat(buf, result)
}

// fnLdiv — divide first element by all subsequent elements.
// ldiv(list[, delim]) → first / second / third / ...
func fnLdiv(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 1 && args[1] != "" { delim = args[1] }
	words := splitList(args[0], delim)
	if len(words) == 0 { buf.WriteString("0"); return }
	result := toFloat(words[0])
	for i := 1; i < len(words); i++ {
		d := toFloat(words[i])
		if d != 0 { result /= d }
	}
	writeFloat(buf, result)
}

// fnListmatch — filter list elements by wildcard pattern.
// listmatch(list, pattern[, delim]) → matching elements
func fnListmatch(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	pattern := args[1]
	var results []string
	for _, w := range words {
		if wildMatch(pattern, w) {
			results = append(results, w)
		}
	}
	buf.WriteString(strings.Join(results, delim))
}

// fnNummatch — count list elements matching a wildcard pattern.
// nummatch(list, pattern[, delim]) → count
func fnNummatch(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	pattern := args[1]
	count := 0
	for _, w := range words {
		if wildMatch(pattern, w) { count++ }
	}
	buf.WriteString(strconv.Itoa(count))
}

// fnNummember — count exact occurrences of a value in a list.
// nummember(list, value[, delim]) → count
func fnNummember(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }
	delim := " "
	if len(args) > 2 && args[2] != "" { delim = args[2] }
	words := splitList(args[0], delim)
	target := args[1]
	count := 0
	for _, w := range words {
		if strings.EqualFold(w, target) { count++ }
	}
	buf.WriteString(strconv.Itoa(count))
}
