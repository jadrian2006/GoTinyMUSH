package functions

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// --- JSON helper functions ---

// jsonAutoType detects the type of a MUSH string value and converts it
// to the appropriate Go type for JSON marshalling. JSON strings/arrays/objects
// are parsed; null/true/false/numbers are detected; everything else is a string.
func jsonAutoType(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Already JSON object or array?
	if (s[0] == '{' || s[0] == '[') {
		var v any
		if json.Unmarshal([]byte(s), &v) == nil {
			return v
		}
	}
	// JSON string literal (quoted)?
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var v string
		if json.Unmarshal([]byte(s), &v) == nil {
			return v
		}
	}
	// Keywords
	if s == "null" {
		return nil
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// Integer?
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	// Float?
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Plain string
	return s
}

// jsonValueToStr converts a Go value back to MUSH-friendly output.
// Strings return unquoted. Objects/arrays marshal to JSON. Bools return 1/0.
func jsonValueToStr(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "1"
		}
		return "0"
	case float64:
		// Render integers without decimal
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// jsonWalkPath walks a dot-notation path into a parsed JSON structure.
// Supports object keys and numeric array indices (0-based).
// Returns the value and whether the path was found.
func jsonWalkPath(data any, path string) (any, bool) {
	if path == "" {
		return data, true
	}
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]
		default:
			return nil, false
		}
	}
	return current, true
}

// jsonSetPath sets a value at a dot-notation path, creating intermediate objects
// as needed. Returns the modified root.
func jsonSetPath(data any, path string, value any) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return value, nil
	}

	if data == nil {
		data = make(map[string]any)
	}

	// For single-segment paths, set directly
	if len(parts) == 1 {
		switch v := data.(type) {
		case map[string]any:
			v[parts[0]] = value
			return v, nil
		case []any:
			idx, err := strconv.Atoi(parts[0])
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("invalid array index")
			}
			v[idx] = value
			return v, nil
		default:
			return nil, fmt.Errorf("cannot set on scalar")
		}
	}

	// Multi-segment: walk to parent, then set last key
	parent := data
	for i := 0; i < len(parts)-1; i++ {
		switch v := parent.(type) {
		case map[string]any:
			next, ok := v[parts[i]]
			if !ok {
				// Create intermediate object
				newObj := make(map[string]any)
				v[parts[i]] = newObj
				parent = newObj
			} else {
				parent = next
			}
		case []any:
			idx, err := strconv.Atoi(parts[i])
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("invalid array index")
			}
			parent = v[idx]
		default:
			return nil, fmt.Errorf("cannot traverse scalar")
		}
	}

	lastKey := parts[len(parts)-1]
	switch v := parent.(type) {
	case map[string]any:
		v[lastKey] = value
	case []any:
		idx, err := strconv.Atoi(lastKey)
		if err != nil || idx < 0 || idx >= len(v) {
			return nil, fmt.Errorf("invalid array index")
		}
		v[idx] = value
	default:
		return nil, fmt.Errorf("cannot set on scalar")
	}
	return data, nil
}

// jsonRemovePath removes a key/index at the given path.
func jsonRemovePath(data any, path string) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	if len(parts) == 1 {
		switch v := data.(type) {
		case map[string]any:
			delete(v, parts[0])
			return v, nil
		case []any:
			idx, err := strconv.Atoi(parts[0])
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("invalid array index")
			}
			return append(v[:idx], v[idx+1:]...), nil
		default:
			return nil, fmt.Errorf("cannot remove from scalar")
		}
	}

	// Walk to parent
	parent, ok := jsonWalkPath(data, strings.Join(parts[:len(parts)-1], "."))
	if !ok {
		return nil, fmt.Errorf("path not found")
	}

	lastKey := parts[len(parts)-1]
	switch v := parent.(type) {
	case map[string]any:
		delete(v, lastKey)
	case []any:
		idx, err := strconv.Atoi(lastKey)
		if err != nil || idx < 0 || idx >= len(v) {
			return nil, fmt.Errorf("invalid array index")
		}
		// We modified the slice in the parent, but since it's by reference, data is updated
		parentPath := strings.Join(parts[:len(parts)-1], ".")
		newSlice := append(v[:idx], v[idx+1:]...)
		data, _ = jsonSetPath(data, parentPath, newSlice)
	default:
		return nil, fmt.Errorf("cannot remove from scalar")
	}
	return data, nil
}

// jsonTypeName returns the JSON type name for a Go value.
func jsonTypeName(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return "string"
	}
}

// --- JSON function handlers ---

// fnJson creates JSON values.
// json(type[, args...])
// Types: object, array, string, number, bool, null
func fnJson(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}
	jtype := strings.ToLower(strings.TrimSpace(args[0]))
	switch jtype {
	case "object":
		rest := args[1:]
		if len(rest)%2 != 0 {
			buf.WriteString("#-1 ODD NUMBER OF ARGUMENTS")
			return
		}
		obj := make(map[string]any)
		for i := 0; i+1 < len(rest); i += 2 {
			key := strings.TrimSpace(rest[i])
			obj[key] = jsonAutoType(rest[i+1])
		}
		b, err := json.Marshal(obj)
		if err != nil {
			buf.WriteString("#-1 JSON ERROR")
			return
		}
		buf.Write(b)

	case "array":
		arr := make([]any, 0, len(args)-1)
		for _, v := range args[1:] {
			arr = append(arr, jsonAutoType(v))
		}
		b, err := json.Marshal(arr)
		if err != nil {
			buf.WriteString("#-1 JSON ERROR")
			return
		}
		buf.Write(b)

	case "string":
		s := ""
		if len(args) > 1 {
			s = args[1]
		}
		b, _ := json.Marshal(s)
		buf.Write(b)

	case "number":
		if len(args) < 2 {
			buf.WriteString("0")
			return
		}
		s := strings.TrimSpace(args[1])
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			buf.WriteString(strconv.FormatInt(n, 10))
		} else if f, err := strconv.ParseFloat(s, 64); err == nil {
			buf.WriteString(strconv.FormatFloat(f, 'f', -1, 64))
		} else {
			buf.WriteString("#-1 INVALID NUMBER")
		}

	case "bool", "boolean":
		if len(args) < 2 {
			buf.WriteString("false")
			return
		}
		s := strings.TrimSpace(args[1])
		if s == "1" || strings.EqualFold(s, "true") {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}

	case "null":
		buf.WriteString("null")

	default:
		buf.WriteString("#-1 UNKNOWN TYPE")
	}
}

// fnJsonQuery queries a JSON value.
// json_query(json, op[, path])
func fnJsonQuery(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}
	var data any
	if err := json.Unmarshal([]byte(args[0]), &data); err != nil {
		buf.WriteString("#-1 INVALID JSON")
		return
	}

	op := strings.ToLower(strings.TrimSpace(args[1]))
	path := ""
	if len(args) > 2 {
		path = strings.TrimSpace(args[2])
	}

	// Walk to target if path given
	target := data
	found := true
	if path != "" {
		target, found = jsonWalkPath(data, path)
	}

	switch op {
	case "get":
		if !found {
			return // empty string for missing path
		}
		buf.WriteString(jsonValueToStr(target))

	case "exists":
		buf.WriteString(boolToStr(found))

	case "type":
		if !found {
			buf.WriteString("#-1 PATH NOT FOUND")
			return
		}
		buf.WriteString(jsonTypeName(target))

	case "members", "lkeys", "keys":
		if !found {
			return
		}
		switch v := target.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			buf.WriteString(strings.Join(keys, " "))
		case []any:
			indices := make([]string, len(v))
			for i := range v {
				indices[i] = strconv.Itoa(i)
			}
			buf.WriteString(strings.Join(indices, " "))
		}

	case "count", "size":
		if !found {
			buf.WriteString("0")
			return
		}
		switch v := target.(type) {
		case map[string]any:
			writeInt(buf, len(v))
		case []any:
			writeInt(buf, len(v))
		default:
			buf.WriteString("1")
		}

	case "isnull":
		if !found {
			buf.WriteString("1")
			return
		}
		buf.WriteString(boolToStr(target == nil))

	default:
		buf.WriteString("#-1 UNKNOWN OPERATION")
	}
}

// fnJsonMod modifies a JSON value and returns the result.
// json_mod(json, op, path[, value])
func fnJsonMod(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 3 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}
	var data any
	if err := json.Unmarshal([]byte(args[0]), &data); err != nil {
		buf.WriteString("#-1 INVALID JSON")
		return
	}

	op := strings.ToLower(strings.TrimSpace(args[1]))
	path := strings.TrimSpace(args[2])
	var value any
	if len(args) > 3 {
		value = jsonAutoType(args[3])
	}

	var result any
	var err error

	switch op {
	case "set":
		result, err = jsonSetPath(data, path, value)
		if err != nil {
			buf.WriteString("#-1 " + strings.ToUpper(err.Error()))
			return
		}

	case "insert":
		// Only set if key does NOT exist
		if _, found := jsonWalkPath(data, path); found {
			buf.WriteString("#-1 KEY ALREADY EXISTS")
			return
		}
		result, err = jsonSetPath(data, path, value)
		if err != nil {
			buf.WriteString("#-1 " + strings.ToUpper(err.Error()))
			return
		}

	case "replace":
		// Only set if key DOES exist
		if _, found := jsonWalkPath(data, path); !found {
			buf.WriteString("#-1 KEY NOT FOUND")
			return
		}
		result, err = jsonSetPath(data, path, value)
		if err != nil {
			buf.WriteString("#-1 " + strings.ToUpper(err.Error()))
			return
		}

	case "remove", "delete":
		result, err = jsonRemovePath(data, path)
		if err != nil {
			buf.WriteString("#-1 " + strings.ToUpper(err.Error()))
			return
		}

	case "push", "append":
		target, found := jsonWalkPath(data, path)
		if !found {
			buf.WriteString("#-1 PATH NOT FOUND")
			return
		}
		arr, ok := target.([]any)
		if !ok {
			buf.WriteString("#-1 NOT AN ARRAY")
			return
		}
		arr = append(arr, value)
		result, err = jsonSetPath(data, path, arr)
		if err != nil {
			buf.WriteString("#-1 " + strings.ToUpper(err.Error()))
			return
		}

	case "patch", "merge":
		target, found := jsonWalkPath(data, path)
		if !found {
			buf.WriteString("#-1 PATH NOT FOUND")
			return
		}
		targetObj, ok := target.(map[string]any)
		if !ok {
			buf.WriteString("#-1 NOT AN OBJECT")
			return
		}
		patchObj, ok := value.(map[string]any)
		if !ok {
			buf.WriteString("#-1 PATCH VALUE MUST BE AN OBJECT")
			return
		}
		for k, v := range patchObj {
			targetObj[k] = v
		}
		result = data

	default:
		buf.WriteString("#-1 UNKNOWN OPERATION")
		return
	}

	b, err2 := json.Marshal(result)
	if err2 != nil {
		buf.WriteString("#-1 JSON ERROR")
		return
	}
	buf.Write(b)
}

// fnJsonPP pretty-prints JSON with 2-space indentation.
func fnJsonPP(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		return
	}
	var data any
	if err := json.Unmarshal([]byte(args[0]), &data); err != nil {
		buf.WriteString("#-1 INVALID JSON")
		return
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		buf.WriteString("#-1 JSON ERROR")
		return
	}
	buf.Write(b)
}

// fnJsonTest validates JSON. Returns "1" if valid, error description if not.
func fnJsonTest(_ *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}
	var data any
	if err := json.Unmarshal([]byte(args[0]), &data); err != nil {
		buf.WriteString(err.Error())
		return
	}
	buf.WriteString("1")
}

// --- JSON / Array bridge functions ---

// fnJsonToArray loads a JSON array into a named MUSH array.
// json_to_array(json, arrayname)
// If the JSON is an array of scalars, loads them directly.
// If the JSON is an array of objects, loads each object as a JSON string (nested stanza).
// If the JSON is an object, loads key:value pairs as "key:value" strings.
func fnJsonToArray(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}

	var data any
	if err := json.Unmarshal([]byte(args[0]), &data); err != nil {
		buf.WriteString("#-1 INVALID JSON")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[1]))

	globalArrays.mu.Lock()
	defer globalArrays.mu.Unlock()

	arrays := getPlayerArrays(ctx.Player)
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("#-1 NO SUCH ARRAY")
		return
	}

	var values []string
	switch v := data.(type) {
	case []any:
		for _, elem := range v {
			values = append(values, jsonValueToStr(elem))
		}
	case map[string]any:
		// Load as key:value pairs, sorted by key for determinism
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			values = append(values, k+":"+jsonValueToStr(v[k]))
		}
	default:
		values = append(values, jsonValueToStr(data))
	}

	if arr.MaxSize > 0 && len(values) > arr.MaxSize {
		values = values[:arr.MaxSize]
	}
	arr.Values = values

	if ctx.GameState != nil {
		ctx.GameState.PersistArray(ctx.Player, name, toPersistedArray(arr))
	}

	writeInt(buf, len(arr.Values))
}

// fnArrayToJson converts a named MUSH array to a JSON string.
// array_to_json(arrayname[, mode])
// mode "array" (default): produces a JSON array of values.
// mode "object": if values contain "key:value" pairs, produces a JSON object.
// mode "nested": each value that is valid JSON is preserved as-is (nested stanza).
func fnArrayToJson(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 1 {
		buf.WriteString("#-1 TOO FEW ARGUMENTS")
		return
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))

	globalArrays.mu.RLock()
	defer globalArrays.mu.RUnlock()

	arrays := globalArrays.Arrays[ctx.Player]
	if arrays == nil {
		buf.WriteString("#-1 NO SUCH ARRAY")
		return
	}
	arr, ok := arrays[name]
	if !ok {
		buf.WriteString("#-1 NO SUCH ARRAY")
		return
	}

	mode := "array"
	if len(args) > 1 && args[1] != "" {
		mode = strings.ToLower(strings.TrimSpace(args[1]))
	}

	switch mode {
	case "object":
		obj := make(map[string]any)
		for _, val := range arr.Values {
			if idx := strings.Index(val, ":"); idx > 0 {
				key := val[:idx]
				v := val[idx+1:]
				obj[key] = jsonAutoType(v)
			}
		}
		b, err := json.Marshal(obj)
		if err != nil {
			buf.WriteString("#-1 JSON ERROR")
			return
		}
		buf.Write(b)

	case "nested":
		// Each value that is valid JSON is included as parsed, else as string
		result := make([]any, 0, len(arr.Values))
		for _, val := range arr.Values {
			result = append(result, jsonAutoType(val))
		}
		b, err := json.Marshal(result)
		if err != nil {
			buf.WriteString("#-1 JSON ERROR")
			return
		}
		buf.Write(b)

	default: // "array"
		result := make([]any, 0, len(arr.Values))
		for _, val := range arr.Values {
			result = append(result, val)
		}
		b, err := json.Marshal(result)
		if err != nil {
			buf.WriteString("#-1 JSON ERROR")
			return
		}
		buf.Write(b)
	}
}
