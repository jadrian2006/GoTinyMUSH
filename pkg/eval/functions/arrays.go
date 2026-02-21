package functions

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Array system — per-player named mutable variable-length containers.
// Values are always strings. Flat (1D) only.

// arrayInstance is a live array in memory.
type arrayInstance struct {
	Name    string
	MaxSize int
	Values  []string
}

// arrayStore holds all arrays keyed per-player.
type arrayStore struct {
	mu     sync.RWMutex
	Arrays map[gamedb.DBRef]map[string]*arrayInstance // player -> name -> instance
}

var globalArrays = &arrayStore{
	Arrays: make(map[gamedb.DBRef]map[string]*arrayInstance),
}

// LoadArrayStore populates the in-memory array store from bbolt-persisted data.
// Called at server startup after loading from bbolt.
func LoadArrayStore(data map[gamedb.DBRef]map[string]*gamedb.ArrayData) {
	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	for player, playerArrays := range data {
		if globalArrays.Arrays[player] == nil {
			globalArrays.Arrays[player] = make(map[string]*arrayInstance)
		}
		for name, arr := range playerArrays {
			globalArrays.Arrays[player][name] = &arrayInstance{
				Name:    name,
				MaxSize: arr.MaxSize,
				Values:  arr.Values,
			}
		}
	}
}

func getPlayerArrays(player gamedb.DBRef) map[string]*arrayInstance {
	if globalArrays.Arrays[player] == nil {
		globalArrays.Arrays[player] = make(map[string]*arrayInstance)
	}
	return globalArrays.Arrays[player]
}

func toPersistedArray(a *arrayInstance) *gamedb.ArrayData {
	return &gamedb.ArrayData{
		MaxSize: a.MaxSize,
		Values:  a.Values,
	}
}

// fnArray — create a named array.
// array(name[, maxsize])
func fnArray(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("0")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))
	if name == "" {
		buf.WriteString("0")
		return
	}

	maxSize := 0
	if len(args) > 1 && args[1] != "" {
		n, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || n < 0 {
			buf.WriteString("0")
			return
		}
		maxSize = n
	}

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	if _, exists := arrays[name]; exists {
		buf.WriteString("0")
		return
	}

	arr := &arrayInstance{
		Name:    name,
		MaxSize: maxSize,
		Values:  []string{},
	}
	arrays[name] = arr

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	buf.WriteString("1")
}

// fnAdestroy — destroy a named array.
// adestroy(name)
func fnAdestroy(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("0")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	if _, ok := arrays[name]; !ok {
		buf.WriteString("0")
		return
	}

	delete(arrays, name)

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, nil)
	}

	buf.WriteString("1")
}

// fnApush — append value(s) to the end of an array.
// apush(name, value[, value2...])
func fnApush(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("#-1 NO SUCH ARRAY")
		return
	}

	for _, val := range args[1:] {
		if arr.MaxSize > 0 && len(arr.Values) >= arr.MaxSize {
			break
		}
		arr.Values = append(arr.Values, val)
	}

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	writeInt(buf, len(arr.Values))
}

// fnApop — remove and return the last element.
// apop(name)
func fnApop(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok || len(arr.Values) == 0 {
		return
	}

	val := arr.Values[len(arr.Values)-1]
	arr.Values = arr.Values[:len(arr.Values)-1]

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	buf.WriteString(val)
}

// fnAshift — remove and return the first element.
// ashift(name)
func fnAshift(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok || len(arr.Values) == 0 {
		return
	}

	val := arr.Values[0]
	arr.Values = arr.Values[1:]

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	buf.WriteString(val)
}

// fnAunshift — prepend value(s) to the beginning of an array.
// aunshift(name, value[, value2...])
func fnAunshift(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("#-1 NO SUCH ARRAY")
		return
	}

	// Prepend values in order: aunshift(a, x, y) -> [x, y, ...existing...]
	var newVals []string
	for _, val := range args[1:] {
		if arr.MaxSize > 0 && len(newVals)+len(arr.Values) >= arr.MaxSize {
			break
		}
		newVals = append(newVals, val)
	}
	arr.Values = append(newVals, arr.Values...)

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	writeInt(buf, len(arr.Values))
}

// fnAget — read element by 1-based index.
// aget(name, index)
func fnAget(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))
	idx, err := strconv.Atoi(strings.TrimSpace(args[1]))
	if err != nil {
		return
	}

	globalArrays.mu.RLock()
	defer globalArrays.mu.RUnlock()

	arrays := globalArrays.Arrays[ctx.Player]
	if arrays == nil {
		return
	}
	arr, ok := arrays[name]
	if !ok {
		return
	}

	// 1-based index
	if idx < 1 || idx > len(arr.Values) {
		return
	}

	buf.WriteString(arr.Values[idx-1])
}

// fnAset — write element by 1-based index.
// aset(name, index, value)
func fnAset(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		buf.WriteString("0")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))
	idx, err := strconv.Atoi(strings.TrimSpace(args[1]))
	if err != nil {
		buf.WriteString("0")
		return
	}

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("0")
		return
	}

	// 1-based index
	if idx < 1 || idx > len(arr.Values) {
		buf.WriteString("0")
		return
	}

	arr.Values[idx-1] = args[2]

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	buf.WriteString("1")
}

// fnAlen — return current length of an array.
// alen(name)
func fnAlen(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("0")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.RLock()
	defer globalArrays.mu.RUnlock()

	arrays := globalArrays.Arrays[ctx.Player]
	if arrays == nil {
		buf.WriteString("0")
		return
	}
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("0")
		return
	}

	writeInt(buf, len(arr.Values))
}

// fnAlist — convert array to MUSH list.
// alist(name[, delim])
func fnAlist(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.RLock()
	defer globalArrays.mu.RUnlock()

	arrays := globalArrays.Arrays[ctx.Player]
	if arrays == nil {
		return
	}
	arr, ok := arrays[name]
	if !ok {
		return
	}

	delim := " "
	if len(args) > 1 && args[1] != "" {
		delim = args[1]
	}

	buf.WriteString(strings.Join(arr.Values, delim))
}

// fnAload — bulk load from MUSH list (replaces contents).
// aload(name, list[, delim])
func fnAload(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("#-1 NO SUCH ARRAY")
		return
	}

	delim := " "
	if len(args) > 2 && args[2] != "" {
		delim = args[2]
	}

	var parts []string
	if delim == " " {
		parts = strings.Fields(args[1])
	} else {
		parts = strings.Split(args[1], delim)
	}

	// Enforce maxsize
	if arr.MaxSize > 0 && len(parts) > arr.MaxSize {
		parts = parts[:arr.MaxSize]
	}

	arr.Values = parts

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	writeInt(buf, len(arr.Values))
}

// fnLarrays — list all of caller's arrays.
// larrays()
func fnLarrays(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	globalArrays.mu.RLock()
	defer globalArrays.mu.RUnlock()

	arrays := globalArrays.Arrays[ctx.Player]
	if arrays == nil {
		return
	}

	var names []string
	for name := range arrays {
		names = append(names, name)
	}
	sort.Strings(names)
	buf.WriteString(strings.Join(names, " "))
}
