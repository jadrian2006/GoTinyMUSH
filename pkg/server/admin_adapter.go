package server

import (
	"fmt"
	"runtime"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"gopkg.in/yaml.v3"
)

// gameServerController adapts the Game to the admin.ServerController interface.
type gameServerController struct {
	game      *Game
	startTime time.Time
	running   bool
}

func (c *gameServerController) IsRunning() bool {
	return c.running
}

func (c *gameServerController) StartGame() error {
	// The game engine is started by the server, not directly controllable here yet.
	// This is a placeholder for future setup-mode support.
	c.running = true
	c.startTime = time.Now()
	return nil
}

func (c *gameServerController) StopGame() error {
	c.running = false
	return nil
}

func (c *gameServerController) PlayerCount() int {
	if c.game == nil || c.game.Conns == nil {
		return 0
	}
	return c.game.Conns.Count()
}

func (c *gameServerController) Uptime() float64 {
	if !c.running || c.startTime.IsZero() {
		return 0
	}
	return time.Since(c.startTime).Seconds()
}

func (c *gameServerController) GetDatabase() *gamedb.Database {
	if c.game == nil {
		return nil
	}
	return c.game.DB
}

func (c *gameServerController) GetConfPath() string {
	if c.game == nil {
		return ""
	}
	return c.game.ConfPath
}

// ConvertLegacyConfig parses a legacy TinyMUSH .conf file and returns YAML bytes.
func (c *gameServerController) ConvertLegacyConfig(confPath string) ([]byte, error) {
	gc, err := LoadGameConf(confPath)
	if err != nil {
		return nil, fmt.Errorf("parse legacy config: %w", err)
	}
	data, err := yaml.Marshal(gc)
	if err != nil {
		return nil, fmt.Errorf("marshal to yaml: %w", err)
	}
	return data, nil
}

func (c *gameServerController) GameName() string {
	if c.game != nil && c.game.Conf != nil && c.game.Conf.MudName != "" {
		return c.game.Conf.MudName
	}
	return "GoTinyMUSH"
}

func (c *gameServerController) GameVersion() string {
	return Version
}

func (c *gameServerController) GamePort() int {
	if c.game != nil && c.game.Conf != nil && c.game.Conf.Port != 0 {
		return c.game.Conf.Port
	}
	return 0
}

func (c *gameServerController) ConnectionStats() map[string]any {
	if c.game == nil || c.game.Conns == nil {
		return map[string]any{
			"total": 0, "tcp": 0, "websocket": 0,
			"login_screen": 0, "connected": 0,
			"bytes_sent": 0, "bytes_recv": 0, "commands": 0,
		}
	}
	return c.game.ConnectionStats()
}

func (c *gameServerController) QueueStats() map[string]any {
	if c.game == nil || c.game.Queue == nil {
		return map[string]any{"immediate": 0, "waiting": 0, "semaphore": 0}
	}
	return c.game.QueueStats()
}

func (c *gameServerController) MemoryStats() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"heap_alloc_mb": float64(m.HeapAlloc) / 1024 / 1024,
		"goroutines":    runtime.NumGoroutine(),
	}
}

func (c *gameServerController) GameStats() map[string]any {
	if c.game == nil || c.game.DB == nil {
		return map[string]any{
			"object_count": 0, "attr_def_count": 0,
			"channels": 0, "mail_enabled": false, "user_functions": 0,
		}
	}
	return c.game.GameStats()
}
