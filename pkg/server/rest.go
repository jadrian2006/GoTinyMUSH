package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// RegisterRESTRoutes registers all REST API endpoints on the web server's mux.
// Called from WebServer.registerRoutes after the mux is created.
func (ws *WebServer) RegisterRESTRoutes() {
	// WHO list (optional auth)
	ws.mux.Handle("GET /api/v1/who",
		authMiddleware(ws.auth, false, http.HandlerFunc(ws.handleWho)))

	// Command execution (required auth)
	ws.mux.Handle("POST /api/v1/command",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handleCommand)))

	// Object info (required auth)
	ws.mux.Handle("GET /api/v1/objects/{dbref}",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handleGetObject)))

	// Attribute value (required auth)
	ws.mux.Handle("GET /api/v1/objects/{dbref}/attrs/{name}",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handleGetAttr)))

	// Channel list (required auth)
	ws.mux.Handle("GET /api/v1/channels",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handleChannels)))

	// Channel history (required auth)
	ws.mux.Handle("GET /api/v1/channels/{name}/history",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handleChannelHistory)))

	// Personal scrollback (required auth)
	ws.mux.Handle("GET /api/v1/scrollback",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handleGetScrollback)))
	ws.mux.Handle("POST /api/v1/scrollback",
		authMiddleware(ws.auth, true, http.HandlerFunc(ws.handlePostScrollback)))
}

// --- WHO ---

func (ws *WebServer) handleWho(w http.ResponseWriter, r *http.Request) {
	type whoEntry struct {
		Name     string `json:"name"`
		Ref      int    `json:"ref"`
		OnFor    string `json:"on_for"`
		Idle     string `json:"idle"`
		Doing    string `json:"doing"`
	}

	now := time.Now()
	var entries []whoEntry

	descs := ws.game.Conns.AllDescriptors()
	for _, dd := range descs {
		if dd.State != ConnConnected {
			continue
		}
		name := ws.game.PlayerName(dd.Player)
		entries = append(entries, whoEntry{
			Name:  name,
			Ref:   int(dd.Player),
			OnFor: FormatConnTime(now.Sub(dd.ConnTime)),
			Idle:  FormatIdleTime(now.Sub(dd.LastCmd)),
			Doing: dd.DoingStr,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"players": entries,
		"count":   len(entries),
	})
}

// --- Command Execution ---

func (ws *WebServer) handleCommand(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		http.Error(w, `{"error":"command is required"}`, http.StatusBadRequest)
		return
	}

	// Create a capturing descriptor that buffers output
	output := &captureBuffer{}
	d := &Descriptor{
		ID:        -1,
		Conn:      nullConn{},
		State:     ConnConnected,
		Player:    claims.PlayerRef,
		Addr:      r.RemoteAddr,
		ConnTime:  time.Now(),
		LastCmd:   time.Now(),
		Transport: TransportWebSocket,
	}
	d.SendFunc = func(msg string) {
		output.lines = append(output.lines, msg)
	}

	DispatchCommand(ws.game, d, req.Command)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"output": output.lines,
	})
}

type captureBuffer struct {
	lines []string
}

// --- Object Info ---

func (ws *WebServer) handleGetObject(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	dbrefStr := r.PathValue("dbref")
	ref, err := parseDBRef(dbrefStr)
	if err != nil {
		http.Error(w, `{"error":"invalid dbref"}`, http.StatusBadRequest)
		return
	}

	obj, ok := ws.game.DB.Objects[ref]
	if !ok {
		http.Error(w, `{"error":"object not found"}`, http.StatusNotFound)
		return
	}

	// Permission check: must be examinable
	if !Examinable(ws.game, claims.PlayerRef, ref) {
		// Return basic info only
		result := map[string]any{
			"ref":  int(ref),
			"name": obj.Name,
			"type": obj.ObjType().String(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// Full info
	result := map[string]any{
		"ref":      int(ref),
		"name":     obj.Name,
		"type":     obj.ObjType().String(),
		"location": int(obj.Location),
		"owner":    int(obj.Owner),
		"parent":   int(obj.Parent),
		"zone":     int(obj.Zone),
		"flags":    flagString(obj),
	}

	// Include readable attributes
	attrs := make(map[string]string)
	for _, attr := range obj.Attrs {
		info := ParseAttrInfo(attr.Value)
		def := ws.game.LookupAttrDef(attr.Number)
		if !CanReadAttr(ws.game, claims.PlayerRef, ref, def, info.Flags, info.Owner) {
			continue
		}
		name := ws.game.DB.GetAttrName(attr.Number)
		if name == "" {
			name = fmt.Sprintf("ATTR_%d", attr.Number)
		}
		attrs[name] = eval.StripAttrPrefix(attr.Value)
	}
	result["attrs"] = attrs

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --- Attribute Value ---

func (ws *WebServer) handleGetAttr(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	dbrefStr := r.PathValue("dbref")
	ref, err := parseDBRef(dbrefStr)
	if err != nil {
		http.Error(w, `{"error":"invalid dbref"}`, http.StatusBadRequest)
		return
	}

	attrName := strings.ToUpper(r.PathValue("name"))

	obj, ok := ws.game.DB.Objects[ref]
	if !ok {
		http.Error(w, `{"error":"object not found"}`, http.StatusNotFound)
		return
	}

	attrNum := ws.game.LookupAttrNum(attrName)
	if attrNum < 0 {
		http.Error(w, `{"error":"attribute not found"}`, http.StatusNotFound)
		return
	}

	for _, attr := range obj.Attrs {
		if attr.Number == attrNum {
			info := ParseAttrInfo(attr.Value)
			def := ws.game.LookupAttrDef(attrNum)
			if !CanReadAttr(ws.game, claims.PlayerRef, ref, def, info.Flags, info.Owner) {
				http.Error(w, `{"error":"permission denied"}`, http.StatusForbidden)
				return
			}
			text := eval.StripAttrPrefix(attr.Value)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"name":  attrName,
				"value": text,
			})
			return
		}
	}

	http.Error(w, `{"error":"attribute not found"}`, http.StatusNotFound)
}

// --- Channels ---

func (ws *WebServer) handleChannels(w http.ResponseWriter, r *http.Request) {
	if ws.game.Comsys == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"channels": []any{}})
		return
	}

	type chanInfo struct {
		Name        string `json:"name"`
		Header      string `json:"header"`
		Subscribers int    `json:"subscribers"`
	}

	channels := ws.game.Comsys.AllChannels()
	var result []chanInfo
	for _, ch := range channels {
		subs := ws.game.Comsys.ChannelSubscribers(ch.Name)
		result = append(result, chanInfo{
			Name:        ch.Name,
			Header:      ch.Header,
			Subscribers: len(subs),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"channels": result})
}

// --- Channel History ---

func (ws *WebServer) handleChannelHistory(w http.ResponseWriter, r *http.Request) {
	channelName := r.PathValue("name")
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr != "" {
		if t, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			since = time.Unix(t, 0)
		}
	}

	if ws.game.SQLDB == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"messages": []any{}, "channel": channelName})
		return
	}

	messages, err := ws.game.SQLDB.GetScrollback(channelName, since, 500)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"channel":  channelName,
		"messages": messages,
	})
}

// --- Personal Scrollback ---

func (ws *WebServer) handleGetScrollback(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr != "" {
		if t, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			since = time.Unix(t, 0)
		}
	}

	if ws.game.SQLDB == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"entries": []any{}})
		return
	}

	entries, err := ws.game.SQLDB.GetPersonalScrollback(claims.PlayerRef, since, 500)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"entries": entries})
}

func (ws *WebServer) handlePostScrollback(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		EncryptedData []byte `json:"encrypted_data"`
		IV            []byte `json:"iv"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if ws.game.SQLDB == nil {
		http.Error(w, `{"error":"storage not available"}`, http.StatusServiceUnavailable)
		return
	}

	err := ws.game.SQLDB.InsertPersonalScrollback(claims.PlayerRef, req.EncryptedData, req.IV)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Helpers ---

func parseDBRef(s string) (gamedb.DBRef, error) {
	s = strings.TrimPrefix(s, "#")
	n, err := strconv.Atoi(s)
	if err != nil {
		return gamedb.Nothing, err
	}
	return gamedb.DBRef(n), nil
}

// Suppress unused import warning — functions is used for future eval contexts
var _ = functions.RegisterAll
