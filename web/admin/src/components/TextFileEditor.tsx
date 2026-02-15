import { useState, useEffect } from 'preact/hooks'
import { api } from '../api/client'
import type { FileRole } from '../types/import'

interface TextFileEditorProps {
  name: string
  role: FileRole
  mode: 'staged' | 'live'
  initialContent?: string
  onSave?: (content: string) => void
}

export function TextFileEditor({ name, role, mode, initialContent, onSave }: TextFileEditorProps) {
  const [content, setContent] = useState(initialContent || '')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(!initialContent)

  useEffect(() => {
    if (!initialContent && mode === 'staged') {
      setLoading(true)
      api.importGetFile(role, name)
        .then((res: any) => setContent(res.content || ''))
        .catch((e: any) => setError(e.message))
        .finally(() => setLoading(false))
    }
  }, [name, role])

  const lineCount = content.split('\n').length

  const handleSave = async () => {
    setSaving(true)
    setSaved(false)
    setError('')
    try {
      if (onSave) {
        onSave(content)
      } else {
        await api.importPutFile(role, name, content)
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
          <span class="ml-2 text-xs text-slate-500">{lineCount} lines</span>
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
        class="w-full h-64 px-3 py-2 bg-slate-900 border border-slate-600 rounded text-sm text-slate-200 font-mono resize-y focus:outline-none focus:border-indigo-500"
        spellcheck={false}
      />
    </div>
  )
}
