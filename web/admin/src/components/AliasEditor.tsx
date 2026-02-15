import { useState, useEffect } from 'preact/hooks'
import { api } from '../api/client'

interface AliasEditorProps {
  name: string
  initialContent?: string
  onSave?: (content: string) => void
}

export function AliasEditor({ name, initialContent, onSave }: AliasEditorProps) {
  const [content, setContent] = useState(initialContent || '')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(!initialContent)

  useEffect(() => {
    if (!initialContent) {
      setLoading(true)
      api.importGetFile('alias_config', name)
        .then((res: any) => setContent(res.content || ''))
        .catch((e: any) => setError(e.message))
        .finally(() => setLoading(false))
    }
  }, [name])

  const handleSave = async () => {
    setSaving(true)
    setSaved(false)
    setError('')
    try {
      if (onSave) {
        onSave(content)
      } else {
        await api.importPutFile('alias_config', name, content)
      }
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div class="text-slate-400 text-sm">Loading {name}...</div>
  }

  return (
    <div class="bg-slate-800 rounded p-4">
      <div class="flex items-center justify-between mb-2">
        <div>
          <span class="text-sm font-mono text-slate-300">{name}</span>
          <span class="ml-2 text-xs text-slate-500">YAML alias config</span>
        </div>
        <div class="flex items-center gap-2">
          {saved && <span class="text-green-400 text-xs">Saved!</span>}
          {error && <span class="text-red-400 text-xs">{error}</span>}
          <button
            onClick={handleSave}
            disabled={saving}
            class="px-3 py-1 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-xs transition-colors disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
      <textarea
        value={content}
        onInput={(e) => setContent((e.target as HTMLTextAreaElement).value)}
        class="w-full h-80 px-3 py-2 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 font-mono resize-y focus:outline-none focus:border-indigo-500"
        spellcheck={false}
        placeholder="# YAML alias configuration"
      />
    </div>
  )
}
