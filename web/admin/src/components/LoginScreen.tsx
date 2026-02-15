import { useState } from 'preact/hooks'
import { api } from '../api/client'
import { branding } from '../tokens/branding'

interface LoginScreenProps {
  onLogin: (defaultPassword: boolean) => void
}

export function LoginScreen({ onLogin }: LoginScreenProps) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: Event) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const result = await api.authLogin(password)
      onLogin(result.default_password)
    } catch (e: any) {
      setError(e.message || 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div class="min-h-screen flex items-center justify-center bg-slate-900">
      <div class="bg-slate-800 rounded-lg p-8 w-full max-w-sm">
        <div class="text-center mb-6">
          <h1 class="text-2xl font-bold text-indigo-400">{branding.appName}</h1>
          <p class="text-sm text-slate-400 mt-1">Admin Panel</p>
        </div>

        <form onSubmit={handleSubmit}>
          <div class="mb-4">
            <label class="block text-sm text-slate-400 mb-1">Password</label>
            <input
              type="password"
              value={password}
              onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
              class="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded text-slate-200 focus:outline-none focus:border-indigo-500"
              placeholder="Enter admin password"
              autoFocus
            />
          </div>

          {error && (
            <div class="bg-red-500/10 border border-red-500/30 rounded p-2 mb-4 text-red-300 text-sm">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading || !password}
            class="w-full px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors disabled:opacity-50"
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>

        <p class="text-xs text-slate-500 text-center mt-4">
          Default password: goTinyMush
        </p>
      </div>
    </div>
  )
}
