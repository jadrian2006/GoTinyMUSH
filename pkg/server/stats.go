package server

import (
	"runtime"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// ConnectionStats returns a breakdown of current connections.
func (g *Game) ConnectionStats() map[string]any {
	descs := g.Conns.AllDescriptors()

	total := len(descs)
	tcp := 0
	ws := 0
	loginScreen := 0
	connected := 0
	var bytesSent, bytesRecv, cmdCount int

	for _, d := range descs {
		switch d.Transport {
		case TransportTCP:
			tcp++
		case TransportWebSocket:
			ws++
		}
		switch d.State {
		case ConnLogin:
			loginScreen++
		case ConnConnected:
			connected++
		}
		bytesSent += d.BytesSent
		bytesRecv += d.BytesRecv
		cmdCount += d.CmdCount
	}

	return map[string]any{
		"total":        total,
		"tcp":          tcp,
		"websocket":    ws,
		"login_screen": loginScreen,
		"connected":    connected,
		"bytes_sent":   bytesSent,
		"bytes_recv":   bytesRecv,
		"commands":     cmdCount,
	}
}

// QueueStats returns command queue depth info.
func (g *Game) QueueStats() map[string]any {
	immediate, waiting, semaphore := g.Queue.Stats()
	return map[string]any{
		"immediate": immediate,
		"waiting":   waiting,
		"semaphore": semaphore,
	}
}

// MemoryStats returns Go runtime memory statistics.
func (g *Game) MemoryStats() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"heap_alloc_bytes":   m.HeapAlloc,
		"heap_inuse_bytes":   m.HeapInuse,
		"heap_alloc_mb":      float64(m.HeapAlloc) / 1024 / 1024,
		"goroutines":         runtime.NumGoroutine(),
		"gc_cycles":          m.NumGC,
		"gc_pause_total_ns":  m.PauseTotalNs,
	}
}

// GameStats returns object and feature stats.
func (g *Game) GameStats() map[string]any {
	typeCounts := map[string]int{
		"rooms":   0,
		"players": 0,
		"things":  0,
		"exits":   0,
		"garbage": 0,
	}

	for _, obj := range g.DB.Objects {
		switch obj.ObjType() {
		case gamedb.TypeRoom:
			typeCounts["rooms"]++
		case gamedb.TypePlayer:
			typeCounts["players"]++
		case gamedb.TypeThing:
			typeCounts["things"]++
		case gamedb.TypeExit:
			typeCounts["exits"]++
		case gamedb.TypeGarbage:
			typeCounts["garbage"]++
		}
	}

	channelCount := 0
	if g.Comsys != nil {
		channelCount = len(g.Comsys.Channels)
	}

	ufuncCount := 0
	if g.GameFuncs != nil {
		ufuncCount = len(g.GameFuncs)
	}

	return map[string]any{
		"object_count":   len(g.DB.Objects),
		"attr_def_count": len(g.DB.AttrNames),
		"type_counts":    typeCounts,
		"channels":       channelCount,
		"mail_enabled":   g.Mail != nil,
		"user_functions": ufuncCount,
	}
}
