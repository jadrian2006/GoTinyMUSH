import { useState, useEffect } from 'preact/hooks'
import { api } from '../api/client'

export function Dashboard() {
  const [status, setStatus] = useState<any>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [actionError, setActionError] = useState('')

  const refresh = () => {
    setLoading(true)
    api.serverStatus()
      .then(s => { setStatus(s); setError('') })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, 5000)
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

  const handleStop = async () => {
    setActionError('')
    try {
      await api.serverStop()
      refresh()
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

      <div class="flex gap-3">
        {!status?.running ? (
          <button
            onClick={handleStart}
            disabled={isSetup}
            class="px-4 py-2 bg-green-600 hover:bg-green-500 text-white rounded text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Start Server
          </button>
        ) : (
          <button
            onClick={handleStop}
            class="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded text-sm transition-colors"
          >
            Stop Server
          </button>
        )}
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
