package functions

import (
	"strconv"
	"strings"
	"sync"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// Structure/instance system — per-player namespaced typed data structures.
// Matches TinyMUSH 3.x's funvars.c structure system.

// structDef defines a named data structure template.
type structDef struct {
	Name       string       // structure name (lowercase)
	Components []string     // component names (lowercase)
	Types      []byte       // type codes: 'a','c','d','i','f','s'
	Defaults   []string     // default values per component
	Delim      string       // output delimiter (default " ")
	Instances  int          // reference count of active instances
}

// structInstance is a live instance of a structure.
type structInstance struct {
	Def    *structDef // pointer to definition
	Values []string   // current component values
}

// structStore holds all structures and instances, keyed per-player.
type structStore struct {
	mu        sync.RWMutex
	Structs   map[gamedb.DBRef]map[string]*structDef      // player -> name -> def
	Instances map[gamedb.DBRef]map[string]*structInstance  // player -> name -> instance
}

var globalStructs = &structStore{
	Structs:   make(map[gamedb.DBRef]map[string]*structDef),
	Instances: make(map[gamedb.DBRef]map[string]*structInstance),
}

// LoadStructStore populates the in-memory structure store from bbolt-persisted data.
// Called at server startup after loading from bbolt.
func LoadStructStore(defs map[gamedb.DBRef]map[string]*gamedb.StructDef, insts map[gamedb.DBRef]map[string]*gamedb.StructInstance) {
	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	// Load definitions
	for player, playerDefs := range defs {
		if globalStructs.Structs[player] == nil {
			globalStructs.Structs[player] = make(map[string]*structDef)
		}
		for name, d := range playerDefs {
			globalStructs.Structs[player][name] = &structDef{
				Name:       d.Name,
				Components: d.Components,
				Types:      d.Types,
				Defaults:   d.Defaults,
				Delim:      d.Delim,
			}
		}
	}

	// Load instances, linking back to their definitions
	for player, playerInsts := range insts {
		if globalStructs.Instances[player] == nil {
			globalStructs.Instances[player] = make(map[string]*structInstance)
		}
		playerDefs := globalStructs.Structs[player]
		if playerDefs == nil {
			continue
		}
		for name, inst := range playerInsts {
			def, ok := playerDefs[inst.DefName]
			if !ok {
				continue // orphaned instance, skip
			}
			def.Instances++
			globalStructs.Instances[player][name] = &structInstance{
				Def:    def,
				Values: inst.Values,
			}
		}
	}
}

// toPersistedDef converts an internal structDef to a persistable StructDef.
func toPersistedDef(d *structDef) *gamedb.StructDef {
	return &gamedb.StructDef{
		Name:       d.Name,
		Components: d.Components,
		Types:      d.Types,
		Defaults:   d.Defaults,
		Delim:      d.Delim,
	}
}

// toPersistedInst converts an internal structInstance to a persistable StructInstance.
func toPersistedInst(inst *structInstance) *gamedb.StructInstance {
	return &gamedb.StructInstance{
		DefName: inst.Def.Name,
		Values:  inst.Values,
	}
}

func getPlayerStructs(player gamedb.DBRef) map[string]*structDef {
	if globalStructs.Structs[player] == nil {
		globalStructs.Structs[player] = make(map[string]*structDef)
	}
	return globalStructs.Structs[player]
}

func getPlayerInstances(player gamedb.DBRef) map[string]*structInstance {
	if globalStructs.Instances[player] == nil {
		globalStructs.Instances[player] = make(map[string]*structInstance)
	}
	return globalStructs.Instances[player]
}

// Type checking functions matching TinyMUSH's type system.
func typeCheck(val string, typeCode byte) bool {
	switch typeCode {
	case 'a': // any
		return true
	case 'c': // single character
		return len(val) == 1
	case 'd': // dbref
		if !strings.HasPrefix(val, "#") { return false }
		_, err := strconv.Atoi(val[1:])
		return err == nil
	case 'i': // integer
		_, err := strconv.Atoi(strings.TrimSpace(val))
		return err == nil
	case 'f': // float
		_, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return err == nil
	case 's': // string (no spaces)
		return !strings.ContainsAny(val, " \t\n\r")
	}
	return true
}

const genericStructDelim = "\f" // form feed, matches TinyMUSH

// fnStructure — define a named structure.
// structure(name, components, types[, defaults[, output-delim]])
func fnStructure(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }

	name := strings.ToLower(strings.TrimSpace(args[0]))
	if name == "" || strings.Contains(name, ".") {
		buf.WriteString("0"); return
	}

	components := strings.Fields(args[1])
	types := strings.Fields(args[2])
	if len(components) == 0 || len(components) != len(types) {
		buf.WriteString("0"); return
	}

	// Validate type codes
	typeCodes := make([]byte, len(types))
	for i, t := range types {
		t = strings.ToLower(t)
		if len(t) != 1 || !strings.ContainsRune("acdifs", rune(t[0])) {
			buf.WriteString("0"); return
		}
		typeCodes[i] = t[0]
	}

	// Parse defaults
	defaults := make([]string, len(components))
	if len(args) > 3 && args[3] != "" {
		delim := " "
		if len(args) > 4 && args[4] != "" { delim = args[4] }
		defVals := splitList(args[3], delim)
		for i := range defaults {
			if i < len(defVals) { defaults[i] = defVals[i] }
		}
	}

	outDelim := " "
	if len(args) > 4 && args[4] != "" { outDelim = args[4] }

	// Lowercase component names
	for i := range components {
		components[i] = strings.ToLower(strings.TrimSpace(components[i]))
	}

	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	structs := getPlayerStructs(ctx.Player)
	if _, exists := structs[name]; exists {
		buf.WriteString("0"); return // can't redefine
	}

	def := &structDef{
		Name:       name,
		Components: components,
		Types:      typeCodes,
		Defaults:   defaults,
		Delim:      outDelim,
	}
	structs[name] = def

	if ctx.GameState != nil {
		ctx.GameState.PersistStructDef(ctx.Player, name, toPersistedDef(def))
	}

	buf.WriteString("1")
}

// fnConstruct — create an instance of a structure.
// construct(instance, structure[, components, values[, input-delim]])
func fnConstruct(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { buf.WriteString("0"); return }

	instName := strings.ToLower(strings.TrimSpace(args[0]))
	structName := strings.ToLower(strings.TrimSpace(args[1]))

	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	structs := getPlayerStructs(ctx.Player)
	def, ok := structs[structName]
	if !ok { buf.WriteString("0"); return }

	instances := getPlayerInstances(ctx.Player)
	if _, exists := instances[instName]; exists {
		buf.WriteString("0"); return // can't recreate
	}

	// Start with defaults
	values := make([]string, len(def.Components))
	copy(values, def.Defaults)

	// Apply overrides
	if len(args) >= 4 {
		overrideComps := strings.Fields(args[2])
		delim := " "
		if len(args) > 4 && args[4] != "" { delim = args[4] }
		overrideVals := splitList(args[3], delim)

		for i, comp := range overrideComps {
			comp = strings.ToLower(strings.TrimSpace(comp))
			idx := -1
			for j, c := range def.Components {
				if c == comp { idx = j; break }
			}
			if idx < 0 { buf.WriteString("0"); return }
			val := ""
			if i < len(overrideVals) { val = overrideVals[i] }
			if !typeCheck(val, def.Types[idx]) { buf.WriteString("0"); return }
			values[idx] = val
		}
	}

	def.Instances++
	inst := &structInstance{
		Def:    def,
		Values: values,
	}
	instances[instName] = inst

	if ctx.GameState != nil {
		ctx.GameState.PersistStructInstance(ctx.Player, instName, toPersistedInst(inst))
	}

	buf.WriteString("1")
}

// fnDestruct — destroy an instance.
func fnDestruct(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }

	instName := strings.ToLower(strings.TrimSpace(args[0]))

	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	instances := getPlayerInstances(ctx.Player)
	inst, ok := instances[instName]
	if !ok { buf.WriteString("0"); return }

	inst.Def.Instances--
	delete(instances, instName)

	if ctx.GameState != nil {
		ctx.GameState.PersistStructInstance(ctx.Player, instName, nil)
	}

	buf.WriteString("1")
}

// fnUnstructure — delete a structure definition (must have 0 instances).
func fnUnstructure(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }

	structName := strings.ToLower(strings.TrimSpace(args[0]))

	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	structs := getPlayerStructs(ctx.Player)
	def, ok := structs[structName]
	if !ok { buf.WriteString("0"); return }
	if def.Instances > 0 { buf.WriteString("0"); return }

	delete(structs, structName)

	if ctx.GameState != nil {
		ctx.GameState.PersistStructDef(ctx.Player, structName, nil)
	}

	buf.WriteString("1")
}

// fnZ — read a component value from an instance.
// z(instance, component)
func fnZ(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }

	instName := strings.ToLower(strings.TrimSpace(args[0]))
	compName := strings.ToLower(strings.TrimSpace(args[1]))

	globalStructs.mu.RLock()
	defer globalStructs.mu.RUnlock()

	instances := globalStructs.Instances[ctx.Player]
	if instances == nil { return }
	inst, ok := instances[instName]
	if !ok { return }

	for i, c := range inst.Def.Components {
		if c == compName {
			buf.WriteString(inst.Values[i])
			return
		}
	}
}

// fnModify — update component values in an instance.
// modify(instance, components, values[, input-delim])
func fnModify(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }

	instName := strings.ToLower(strings.TrimSpace(args[0]))
	comps := strings.Fields(args[1])
	delim := " "
	if len(args) > 3 && args[3] != "" { delim = args[3] }
	vals := splitList(args[2], delim)

	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	instances := getPlayerInstances(ctx.Player)
	inst, ok := instances[instName]
	if !ok { buf.WriteString("0"); return }

	modified := 0
	for i, comp := range comps {
		comp = strings.ToLower(strings.TrimSpace(comp))
		idx := -1
		for j, c := range inst.Def.Components {
			if c == comp { idx = j; break }
		}
		if idx < 0 { continue }
		val := ""
		if i < len(vals) { val = vals[i] }
		if !typeCheck(val, inst.Def.Types[idx]) { continue }
		inst.Values[idx] = val
		modified++
	}

	if modified > 0 && ctx.GameState != nil {
		ctx.GameState.PersistStructInstance(ctx.Player, instName, toPersistedInst(inst))
	}

	writeInt(buf, modified)
}

// fnLoadStruct — parse delimited text and create instance.
// load(instance, structure, text[, input-delim])
func fnLoadStruct(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }

	instName := strings.ToLower(strings.TrimSpace(args[0]))
	structName := strings.ToLower(strings.TrimSpace(args[1]))
	text := args[2]

	globalStructs.mu.Lock()
	defer globalStructs.mu.Unlock()

	structs := getPlayerStructs(ctx.Player)
	def, ok := structs[structName]
	if !ok { buf.WriteString("0"); return }

	instances := getPlayerInstances(ctx.Player)
	if _, exists := instances[instName]; exists {
		buf.WriteString("0"); return
	}

	delim := def.Delim
	if len(args) > 3 && args[3] != "" { delim = args[3] }

	var parts []string
	if delim == " " {
		parts = strings.Fields(text)
	} else {
		parts = strings.Split(text, delim)
	}

	if len(parts) != len(def.Components) {
		buf.WriteString("0"); return
	}

	// Type check all values
	for i, val := range parts {
		if !typeCheck(val, def.Types[i]) { buf.WriteString("0"); return }
	}

	def.Instances++
	values := make([]string, len(parts))
	copy(values, parts)
	newInst := &structInstance{
		Def:    def,
		Values: values,
	}
	instances[instName] = newInst

	if ctx.GameState != nil {
		ctx.GameState.PersistStructInstance(ctx.Player, instName, toPersistedInst(newInst))
	}

	buf.WriteString("1")
}

// fnUnload — serialize instance to delimited text.
// unload(instance[, output-delim])
func fnUnload(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { return }

	instName := strings.ToLower(strings.TrimSpace(args[0]))

	globalStructs.mu.RLock()
	defer globalStructs.mu.RUnlock()

	instances := globalStructs.Instances[ctx.Player]
	if instances == nil { return }
	inst, ok := instances[instName]
	if !ok { return }

	delim := inst.Def.Delim
	if len(args) > 1 && args[1] != "" { delim = args[1] }

	buf.WriteString(strings.Join(inst.Values, delim))
}

// fnReadStruct — load instance from an attribute.
// read(obj/attr, instance, structure)
func fnReadStruct(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 { buf.WriteString("0"); return }

	attrSpec := args[0]
	parts := strings.SplitN(attrSpec, "/", 2)
	if len(parts) != 2 { buf.WriteString("0"); return }
	ref := resolveDBRef(ctx, parts[0])
	text := getAttrByName(ctx, ref, strings.ToUpper(strings.TrimSpace(parts[1])))
	if text == "" { buf.WriteString("0"); return }

	// Replace with load using form-feed delimiter
	loadArgs := []string{args[1], args[2], text, genericStructDelim}
	fnLoadStruct(ctx, loadArgs, buf, 0, 0)
}

// fnWriteStruct — save instance to an attribute.
// write(obj/attr, instance)
func fnWriteStruct(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.GameState == nil { return }

	instName := strings.ToLower(strings.TrimSpace(args[1]))

	globalStructs.mu.RLock()
	instances := globalStructs.Instances[ctx.Player]
	var serialized string
	if instances != nil {
		if inst, ok := instances[instName]; ok {
			serialized = strings.Join(inst.Values, genericStructDelim)
		}
	}
	globalStructs.mu.RUnlock()

	if serialized == "" { return }

	// Parse obj/attr
	parts := strings.SplitN(args[0], "/", 2)
	if len(parts) != 2 { return }
	ref := resolveDBRef(ctx, parts[0])
	if ref == gamedb.Nothing { return }
	if !ctx.GameState.Controls(ctx.Player, ref) { return }
	ctx.GameState.SetAttrByName(ref, parts[1], serialized)
}

// fnDelimit — convert structure-attribute delimiter to a new delimiter.
// delimit(obj/attr, new-delim[, input-delim])
func fnDelimit(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 { return }

	dparts := strings.SplitN(args[0], "/", 2)
	if len(dparts) != 2 { return }
	dref := resolveDBRef(ctx, dparts[0])
	text := getAttrByName(ctx, dref, strings.ToUpper(strings.TrimSpace(dparts[1])))
	if text == "" { return }

	inputDelim := genericStructDelim
	if len(args) > 2 && args[2] != "" { inputDelim = args[2] }

	parts := strings.Split(text, inputDelim)
	buf.WriteString(strings.Join(parts, args[1]))
}

// fnLstructures — list player's defined structures.
func fnLstructures(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	globalStructs.mu.RLock()
	defer globalStructs.mu.RUnlock()

	structs := globalStructs.Structs[ctx.Player]
	if structs == nil { return }

	var names []string
	for name := range structs {
		names = append(names, name)
	}
	buf.WriteString(strings.Join(names, " "))
}

// fnLinstances — list player's active instances.
func fnLinstances(ctx *eval.EvalContext, _ []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	globalStructs.mu.RLock()
	defer globalStructs.mu.RUnlock()

	instances := globalStructs.Instances[ctx.Player]
	if instances == nil { return }

	var names []string
	for name := range instances {
		names = append(names, name)
	}
	buf.WriteString(strings.Join(names, " "))
}

// fnStore — set and return a variable value (combines setx + x).
// store(name, value)
func fnStore(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.RData == nil { return }
	name := strings.ToLower(strings.TrimSpace(args[0]))
	ctx.RData.XRegs[name] = args[1]
	buf.WriteString(args[1])
}

// fnItems — return number of components in a structure.
// items(structure)
func fnItems(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 { buf.WriteString("0"); return }

	structName := strings.ToLower(strings.TrimSpace(args[0]))

	globalStructs.mu.RLock()
	defer globalStructs.mu.RUnlock()

	structs := globalStructs.Structs[ctx.Player]
	if structs == nil { buf.WriteString("0"); return }
	def, ok := structs[structName]
	if !ok { buf.WriteString("0"); return }

	writeInt(buf, len(def.Components))
}
