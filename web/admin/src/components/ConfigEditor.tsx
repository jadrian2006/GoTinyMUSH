import { useState, useEffect } from 'preact/hooks'
import { api } from '../api/client'

interface ConfigEditorProps {
  mode?: 'live' | 'staged'
  stagedContent?: string
  onStagedSave?: (content: string) => void
}

export function ConfigEditor({ mode = 'live', stagedContent, onStagedSave }: ConfigEditorProps) {
  const [config, setConfig] = useState<Record<string, any> | null>(null)
  const [configPath, setConfigPath] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [rawMode, setRawMode] = useState(false)
  const [rawContent, setRawContent] = useState('')

  useEffect(() => {
    if (mode === 'staged' && stagedContent) {
      setRawContent(stagedContent)
      // Try to parse YAML into structured form (best-effort)
      try {
        // Simple YAML-to-object parsing for display (not a full parser)
        const obj: Record<string, any> = {}
        for (const line of stagedContent.split('\n')) {
          const match = line.match(/^(\w+):\s*(.*)$/)
          if (match) {
            const [, key, val] = match
            if (val === 'true') obj[key] = true
            else if (val === 'false') obj[key] = false
            else if (/^\d+$/.test(val)) obj[key] = parseInt(val)
            else if (val.startsWith('"') && val.endsWith('"')) obj[key] = val.slice(1, -1)
            else obj[key] = val
          }
        }
        if (Object.keys(obj).length > 0) setConfig(obj)
      } catch {
        // Fall back to raw mode only
        setRawMode(true)
      }
      return
    }

    if (mode === 'live') {
      api.getConfig()
        .then(result => {
          setConfig(result.config)
          setConfigPath(result.path)
        })
        .catch(e => setError(e.message))
    }
  }, [mode, stagedContent])

  const handleSave = async () => {
    setSaving(true)
    setSaved(false)
    setError('')
    try {
      if (mode === 'staged') {
        if (onStagedSave) {
          onStagedSave(rawMode ? rawContent : rawContent)
        }
      } else if (config) {
        await api.putConfig(config)
      }
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setSaving(false)
    }
  }

  const updateField = (key: string, value: any) => {
    setConfig(prev => prev ? { ...prev, [key]: value } : null)
  }

  if (error && !config && !rawContent) {
    return (
      <div>
        <h2 class="text-2xl font-bold mb-6">Configuration</h2>
        <div class="bg-red-500/10 border border-red-500/30 rounded p-4 text-red-300">{error}</div>
      </div>
    )
  }

  if (!config && !rawContent) {
    return <div class="text-slate-400">Loading configuration...</div>
  }

  // Raw YAML mode
  if (rawMode || mode === 'staged') {
    return (
      <div>
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-2xl font-bold">{mode === 'staged' ? 'Config Preview' : 'Configuration'}</h2>
            {configPath && <p class="text-xs text-slate-400 font-mono">{configPath}</p>}
          </div>
          <div class="flex items-center gap-3">
            {config && (
              <button
                onClick={() => setRawMode(!rawMode)}
                class="px-3 py-1 bg-slate-700 hover:bg-slate-600 text-slate-300 rounded text-xs transition-colors"
              >
                {rawMode ? 'Structured View' : 'Raw YAML'}
              </button>
            )}
            {saved && <span class="text-green-400 text-sm">Saved!</span>}
            {error && <span class="text-red-400 text-sm">{error}</span>}
            <button
              onClick={handleSave}
              disabled={saving}
              class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>

        {rawMode || !config ? (
          <textarea
            value={rawContent}
            onInput={(e) => setRawContent((e.target as HTMLTextAreaElement).value)}
            class="w-full h-96 px-3 py-2 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 font-mono resize-y focus:outline-none focus:border-indigo-500"
            spellcheck={false}
          />
        ) : (
          renderStructured(config, updateField)
        )}
      </div>
    )
  }

  // Structured mode (live)
  const sections = groupConfigKeys(config!)

  return (
    <div>
      <div class="flex items-center justify-between mb-6">
        <div>
          <h2 class="text-2xl font-bold">Configuration</h2>
          <p class="text-xs text-slate-400 font-mono">{configPath}</p>
        </div>
        <div class="flex items-center gap-3">
          <button
            onClick={() => {
              // Convert config to raw YAML-ish display
              const lines = Object.entries(config!).map(([k, v]) => `${k}: ${JSON.stringify(v)}`).join('\n')
              setRawContent(lines)
              setRawMode(true)
            }}
            class="px-3 py-1 bg-slate-700 hover:bg-slate-600 text-slate-300 rounded text-xs transition-colors"
          >
            Raw YAML
          </button>
          {saved && <span class="text-green-400 text-sm">Saved!</span>}
          {error && <span class="text-red-400 text-sm">{error}</span>}
          <button
            onClick={handleSave}
            disabled={saving}
            class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>

      {sections.map(([section, keys]) => (
        <div key={section} class="mb-6">
          <h3 class="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">{section}</h3>
          <div class="bg-slate-800 rounded p-4 space-y-3">
            {keys.map(key => (
              <ConfigField
                key={key}
                name={key}
                value={config![key]}
                onChange={(v) => updateField(key, v)}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

function renderStructured(config: Record<string, any>, updateField: (k: string, v: any) => void) {
  const sections = groupConfigKeys(config)
  return (
    <>
      {sections.map(([section, keys]) => (
        <div key={section} class="mb-6">
          <h3 class="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">{section}</h3>
          <div class="bg-slate-800 rounded p-4 space-y-3">
            {keys.map(key => (
              <ConfigField
                key={key}
                name={key}
                value={config[key]}
                onChange={(v) => updateField(key, v)}
              />
            ))}
          </div>
        </div>
      ))}
    </>
  )
}

function ConfigField({ name, value, onChange }: { name: string; value: any; onChange: (v: any) => void }) {
  const type = typeof value

  if (type === 'boolean') {
    return (
      <label class="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange((e.target as HTMLInputElement).checked)}
          class="w-4 h-4 rounded border-slate-600 text-indigo-600 focus:ring-indigo-500"
        />
        <span class="text-sm text-slate-300">{name}</span>
      </label>
    )
  }

  if (type === 'number') {
    return (
      <div class="flex items-center gap-3">
        <label class="text-sm text-slate-400 w-48 shrink-0">{name}</label>
        <input
          type="number"
          value={value}
          onInput={(e) => onChange(Number((e.target as HTMLInputElement).value))}
          class="flex-1 px-2 py-1 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 focus:outline-none focus:border-indigo-500"
        />
      </div>
    )
  }

  if (Array.isArray(value)) {
    return (
      <div class="flex items-start gap-3">
        <label class="text-sm text-slate-400 w-48 shrink-0 pt-1">{name}</label>
        <input
          type="text"
          value={value.join(', ')}
          onInput={(e) => onChange((e.target as HTMLInputElement).value.split(',').map(s => s.trim()).filter(Boolean))}
          class="flex-1 px-2 py-1 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 focus:outline-none focus:border-indigo-500"
          placeholder="comma-separated values"
        />
      </div>
    )
  }

  return (
    <div class="flex items-center gap-3">
      <label class="text-sm text-slate-400 w-48 shrink-0">{name}</label>
      <input
        type="text"
        value={value ?? ''}
        onInput={(e) => onChange((e.target as HTMLInputElement).value)}
        class="flex-1 px-2 py-1 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 focus:outline-none focus:border-indigo-500"
      />
    </div>
  )
}

function groupConfigKeys(config: Record<string, any>): [string, string[]][] {
  const sections: Record<string, string[]> = {}
  const sectionMap: Record<string, string> = {
    mud_name: 'Identity', port: 'Identity',
    master_room: 'Rooms', player_starting_room: 'Rooms', player_starting_home: 'Rooms', default_home: 'Rooms',
    money_name_singular: 'Economy', money_name_plural: 'Economy', starting_money: 'Economy',
    paycheck: 'Economy', earn_limit: 'Economy', page_cost: 'Economy', wait_cost: 'Economy', link_cost: 'Economy',
    idle_timeout: 'Idle', idle_wiz_dark: 'Idle',
    queue_idle_chunk: 'Queue', function_invocation_limit: 'Queue', machine_command_cost: 'Queue',
    output_limit: 'Output',
    web_enabled: 'Web', web_port: 'Web', web_host: 'Web', web_domain: 'Web', web_static_dir: 'Web',
    web_cors_origins: 'Web', web_rate_limit: 'Web', jwt_secret: 'Web', jwt_expiry: 'Web',
    guest_char_num: 'Guests', guest_prefixes: 'Guests', guest_suffixes: 'Guests', guest_basename: 'Guests',
    number_guests: 'Guests', guest_password: 'Guests', guest_start_room: 'Guests',
    mail_enabled: 'Modules', comsys_enabled: 'Modules', mail_expiration: 'Modules',
    spellcheck_enabled: 'Modules', spellcheck_url: 'Modules',
    sql_enabled: 'SQL', sql_database: 'SQL', sql_query_limit: 'SQL', sql_timeout: 'SQL',
    archive_dir: 'Backup', archive_interval: 'Backup', archive_retain: 'Backup', archive_hook: 'Backup',
  }

  for (const key of Object.keys(config)) {
    const section = sectionMap[key] || 'Other'
    if (!sections[section]) sections[section] = []
    sections[section].push(key)
  }

  return Object.entries(sections)
}
