package server

import (
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus metric descriptors for the game server.
type Metrics struct {
	game      *Game
	startTime time.Time

	playersConnected   *prometheus.GaugeVec
	objectsTotal       prometheus.Gauge
	connectionsTotal   *prometheus.CounterVec
	commandsTotal      prometheus.Counter
	bytesSentTotal     prometheus.Counter
	bytesRecvTotal     prometheus.Counter
	queueDepth         *prometheus.GaugeVec
	uptimeSeconds      prometheus.Gauge
	memoryHeapBytes    prometheus.Gauge
	goroutines         prometheus.Gauge
}

// NewMetrics creates and registers Prometheus metrics for the game.
func NewMetrics(game *Game, startTime time.Time) *Metrics {
	m := &Metrics{
		game:      game,
		startTime: startTime,
		playersConnected: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gotinymush_players_connected",
			Help: "Number of currently connected players by transport.",
		}, []string{"transport"}),
		objectsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gotinymush_objects_total",
			Help: "Total number of objects in the database.",
		}),
		connectionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gotinymush_connections_total",
			Help: "Total connections since server start.",
		}, []string{"transport"}),
		commandsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gotinymush_commands_processed_total",
			Help: "Total commands processed since server start.",
		}),
		bytesSentTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gotinymush_bytes_sent_total",
			Help: "Total bytes sent to clients.",
		}),
		bytesRecvTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gotinymush_bytes_received_total",
			Help: "Total bytes received from clients.",
		}),
		queueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gotinymush_queue_depth",
			Help: "Current command queue depth by type.",
		}, []string{"queue_type"}),
		uptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gotinymush_uptime_seconds",
			Help: "Server uptime in seconds.",
		}),
		memoryHeapBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gotinymush_memory_heap_bytes",
			Help: "Go heap memory allocated in bytes.",
		}),
		goroutines: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gotinymush_goroutines",
			Help: "Number of active goroutines.",
		}),
	}

	prometheus.MustRegister(
		m.playersConnected,
		m.objectsTotal,
		m.connectionsTotal,
		m.commandsTotal,
		m.bytesSentTotal,
		m.bytesRecvTotal,
		m.queueDepth,
		m.uptimeSeconds,
		m.memoryHeapBytes,
		m.goroutines,
	)

	return m
}

// Update refreshes all gauge metrics from current game state.
func (m *Metrics) Update() {
	stats := m.game.ConnectionStats()

	m.playersConnected.WithLabelValues("tcp").Set(float64(stats["tcp"].(int)))
	m.playersConnected.WithLabelValues("websocket").Set(float64(stats["websocket"].(int)))

	m.objectsTotal.Set(float64(len(m.game.DB.Objects)))

	m.commandsTotal.Add(0) // Counter — incremented elsewhere or snapshot
	m.bytesSentTotal.Add(0)
	m.bytesRecvTotal.Add(0)

	immediate, waiting, semaphore := m.game.Queue.Stats()
	m.queueDepth.WithLabelValues("immediate").Set(float64(immediate))
	m.queueDepth.WithLabelValues("waiting").Set(float64(waiting))
	m.queueDepth.WithLabelValues("semaphore").Set(float64(semaphore))

	m.uptimeSeconds.Set(time.Since(m.startTime).Seconds())

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	m.memoryHeapBytes.Set(float64(mem.HeapAlloc))
	m.goroutines.Set(float64(runtime.NumGoroutine()))
}

// Handler returns an http.Handler that updates metrics before serving them.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Update()
		promhttp.Handler().ServeHTTP(w, r)
	})
}
