import { useState, useEffect } from 'preact/hooks'
import { api } from '../api/client'

export function Dashboard() {
  const [status, setStatus] = useState<any>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [actionError, setActionError] = useState('')
  const [shutdownStatus, setShutdownStatus] = useState<any>(null)
  const [showShutdownDialog, setShowShutdownDialog] = useState(false)
  const [shutdownDelay, setShutdownDelay] = useState(300)
  const [shutdownReason, setShutdownReason] = useState('')

  const refresh = () => {
    setLoading(true)
    api.serverStatus()
      .then(s => { setStatus(s); setError('') })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }

  const refreshShutdown = () => {
    api.shutdownStatus()
      .then(s => setShutdownStatus(s))
      .catch(() => {})
  }

  useEffect(() => {
    refresh()
    refreshShutdown()
    const interval = setInterval(() => {
      refresh()
      refreshShutdown()
    }, 2000)
    return () => clearInterval(interval)
  }, [])

  const handleStart = async () => {
    setActionError('')
    try {
      await api.serverStart()
      refresh()
    } catch (e: any) {
      setActionError(e.message)
    }
  }

  const handleShutdown = async () => {
    setActionError('')
    setShowShutdownDialog(false)
    try {
      await api.serverShutdown(shutdownDelay, shutdownReason || undefined)
      refreshShutdown()
    } catch (e: any) {
      setActionError(e.message)
    }
  }

  const handleCancelShutdown = async () => {
    setActionError('')
    try {
      await api.shutdownCancel()
      refreshShutdown()
    } catch (e: any) {
      setActionError(e.message)
    }
  }

  if (loading && !status) {
    return <div class="text-slate-400">Loading...</div>
  }

  if (error) {
    return (
      <div class="bg-red-500/10 border border-red-500/30 rounded p-4 text-red-300">
        {error}
      </div>
    )
  }

  const isSetup = status?.setup_mode
  const hostname = window.location.hostname
  const conn = status?.connections
  const queue = status?.queue
  const memory = status?.memory

  return (
    <div>
      <h2 class="text-2xl font-bold mb-6">Dashboard</h2>

      {/* Server info row */}
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
        <StatusCard
          label="Server Status"
          value={isSetup ? 'Setup Mode' : status?.running ? 'Running' : 'Stopped'}
          color={isSetup ? 'text-amber-400' : status?.running ? 'text-green-400' : 'text-red-400'}
        />
        <StatusCard
          label="Connected Players"
          value={String(status?.player_count ?? 0)}
          color="text-cyan-400"
        />
        <StatusCard
          label="Objects"
          value={String(status?.object_count ?? 0)}
          color="text-indigo-400"
        />
        <StatusCard
          label="Uptime"
          value={status?.uptime > 0 ? formatUptime(status.uptime) : '-'}
          color="text-slate-300"
        />
      </div>

      {/* Server info bar */}
      {status?.running && !isSetup && (
        <div class="bg-slate-800 rounded p-3 mb-4 flex flex-wrap gap-x-6 gap-y-1 text-sm">
          {status.game_name && (
            <span><span class="text-slate-500">Name:</span> <span class="text-slate-200">{status.game_name}</span></span>
          )}
          {status.version && (
            <span><span class="text-slate-500">Version:</span> <span class="text-slate-200">{status.version}</span></span>
          )}
          {status.port > 0 && (
            <span><span class="text-slate-500">Port:</span> <span class="text-slate-200">{status.port}</span></span>
          )}
          {status.channels != null && (
            <span><span class="text-slate-500">Channels:</span> <span class="text-slate-200">{status.channels}</span></span>
          )}
          {status.mail_enabled != null && (
            <span><span class="text-slate-500">Mail:</span> <span class="text-slate-200">{status.mail_enabled ? 'Enabled' : 'Disabled'}</span></span>
          )}
        </div>
      )}

      {/* Connection, Queue, Memory cards */}
      {status?.running && !isSetup && (conn || queue || memory) && (
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          {conn && (
            <div class="bg-slate-800 rounded p-4">
              <h3 class="text-xs text-slate-400 uppercase tracking-wider mb-2">Connections</h3>
              <div class="space-y-1 text-sm">
                <StatRow label="Total" value={conn.total} />
                <StatRow label="TCP" value={conn.tcp} />
                <StatRow label="WebSocket" value={conn.websocket} />
                <StatRow label="Login Screen" value={conn.login_screen} />
                <StatRow label="Authenticated" value={conn.connected} />
              </div>
            </div>
          )}
          {queue && (
            <div class="bg-slate-800 rounded p-4">
              <h3 class="text-xs text-slate-400 uppercase tracking-wider mb-2">Command Queue</h3>
              <div class="space-y-1 text-sm">
                <StatRow label="Immediate" value={queue.immediate} />
                <StatRow label="Waiting" value={queue.waiting} />
                <StatRow label="Semaphore" value={queue.semaphore} />
              </div>
            </div>
          )}
          {memory && (
            <div class="bg-slate-800 rounded p-4">
              <h3 class="text-xs text-slate-400 uppercase tracking-wider mb-2">Memory</h3>
              <div class="space-y-1 text-sm">
                <StatRow label="Heap" value={`${(memory.heap_alloc_mb ?? 0).toFixed(1)} MB`} />
                <StatRow label="Goroutines" value={memory.goroutines} />
              </div>
            </div>
          )}
        </div>
      )}

      {actionError && (
        <div class="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4 text-red-300 text-sm">
          {actionError}
          <button onClick={() => setActionError('')} class="ml-2 text-red-400 hover:text-red-300">&times;</button>
        </div>
      )}

      {isSetup && (
        <div class="bg-amber-500/10 border border-amber-500/30 rounded p-4 mb-4 text-amber-300 text-sm">
          The server is in setup mode. Use the Setup wizard or Import page to create or import a game database,
          then launch the server.
        </div>
      )}

      {/* Connection info when server is running */}
      {status?.running && !isSetup && (
        <div class="bg-slate-800 rounded p-4 mb-4">
          <h3 class="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-3">How to Connect</h3>
          <div class="space-y-2 text-sm">
            <div class="flex items-start gap-3">
              <span class="text-slate-500 w-24 shrink-0">Telnet:</span>
              <code class="text-cyan-300 bg-slate-900 px-2 py-0.5 rounded">telnet {hostname} {status?.port || 6886}</code>
            </div>
            <div class="flex items-start gap-3">
              <span class="text-slate-500 w-24 shrink-0">MU* Client:</span>
              <span class="text-slate-300">Host: <code class="text-cyan-300 bg-slate-900 px-1 rounded">{hostname}</code> Port: <code class="text-cyan-300 bg-slate-900 px-1 rounded">{status?.port || 6886}</code></span>
            </div>
            <div class="flex items-start gap-3">
              <span class="text-slate-500 w-24 shrink-0">Web Client:</span>
              <a href={`http://${hostname}:8443/`} target="_blank" class="text-indigo-400 hover:text-indigo-300">
                http://{hostname}:8443/
              </a>
            </div>
            <div class="mt-3 pt-3 border-t border-slate-700">
              <div class="flex items-start gap-3">
                <span class="text-slate-500 w-24 shrink-0">God Login:</span>
                <span class="text-slate-300">
                  <code class="text-cyan-300 bg-slate-900 px-1 rounded">connect Wizard &lt;password&gt;</code>
                </span>
              </div>
              <p class="text-slate-500 text-xs mt-1 ml-27">
                Password is set via MUSH_GODPASS env variable. Change it in-game with: @password &lt;old&gt;=&lt;new&gt;
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Shutdown in progress banner */}
      {shutdownStatus?.active && (
        <div class="bg-red-500/10 border border-red-500/30 rounded p-4 mb-4">
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-sm font-semibold text-red-300 uppercase tracking-wider">Shutdown In Progress</h3>
            <button
              onClick={handleCancelShutdown}
              class="px-3 py-1 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-xs transition-colors"
            >
              Cancel Shutdown
            </button>
          </div>
          <div class="space-y-1 text-sm">
            <div><span class="text-slate-500">Reason:</span> <span class="text-slate-300">{shutdownStatus.reason}</span></div>
            <div><span class="text-slate-500">Stage:</span> <span class="text-slate-300">{shutdownStatus.stage}</span></div>
            <div>
              <span class="text-slate-500">Time remaining:</span>{' '}
              <span class="text-red-300 font-mono text-lg">{formatUptime(shutdownStatus.remaining)}</span>
            </div>
          </div>
        </div>
      )}

      {/* Shutdown dialog */}
      {showShutdownDialog && (
        <div class="bg-slate-800 border border-red-500/30 rounded p-4 mb-4">
          <h3 class="text-sm font-semibold text-red-300 mb-3">Graceful Shutdown</h3>
          <p class="text-xs text-slate-400 mb-3">
            Players will receive @wall warnings before the server shuts down. A backup archive will be created automatically.
          </p>
          <div class="space-y-3">
            <div>
              <label class="text-xs text-slate-400 block mb-1">Delay</label>
              <select
                value={shutdownDelay}
                onChange={(e) => setShutdownDelay(Number((e.target as HTMLSelectElement).value))}
                class="w-full px-2 py-1.5 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 focus:outline-none focus:border-indigo-500"
              >
                <option value={30}>30 seconds</option>
                <option value={60}>1 minute</option>
                <option value={120}>2 minutes</option>
                <option value={300}>5 minutes (default)</option>
                <option value={600}>10 minutes</option>
                <option value={900}>15 minutes</option>
              </select>
            </div>
            <div>
              <label class="text-xs text-slate-400 block mb-1">Reason (optional)</label>
              <input
                type="text"
                placeholder="Server maintenance"
                value={shutdownReason}
                onInput={(e) => setShutdownReason((e.target as HTMLInputElement).value)}
                class="w-full px-2 py-1.5 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 focus:outline-none focus:border-indigo-500"
              />
            </div>
            <div class="flex gap-2 pt-1">
              <button
                onClick={handleShutdown}
                class="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded text-sm transition-colors"
              >
                Begin Shutdown
              </button>
              <button
                onClick={() => setShowShutdownDialog(false)}
                class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-sm transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <div class="flex gap-3">
        {!status?.running ? (
          <button
            onClick={handleStart}
            disabled={isSetup}
            class="px-4 py-2 bg-green-600 hover:bg-green-500 text-white rounded text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Start Server
          </button>
        ) : !shutdownStatus?.active && !showShutdownDialog ? (
          <button
            onClick={() => setShowShutdownDialog(true)}
            class="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded text-sm transition-colors"
          >
            Shutdown Server
          </button>
        ) : null}
        <button
          onClick={refresh}
          class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-sm transition-colors"
        >
          Refresh
        </button>
      </div>
    </div>
  )
}

function StatusCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div class="bg-slate-800 rounded p-4">
      <div class="text-xs text-slate-400 uppercase tracking-wider mb-1">{label}</div>
      <div class={`text-2xl font-bold ${color}`}>{value}</div>
    </div>
  )
}

function StatRow({ label, value }: { label: string; value: any }) {
  return (
    <div class="flex justify-between">
      <span class="text-slate-400">{label}</span>
      <span class="text-slate-200">{value}</span>
    </div>
  )
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}
