import { render } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import './styles/globals.css'
import { Layout } from './components/Layout'
import { Dashboard } from './components/Dashboard'
import { ImportFlow } from './components/ImportFlow'
import { ConfigEditor } from './components/ConfigEditor'
import { SetupWizard } from './components/SetupWizard'
import { LoginScreen } from './components/LoginScreen'
import { api } from './api/client'

type Page = 'dashboard' | 'import' | 'config' | 'setup'

function App() {
  const [page, setPage] = useState<Page>('dashboard')
  const [setupMode, setSetupMode] = useState(false)
  const [authenticated, setAuthenticated] = useState<boolean | null>(null) // null = checking
  const [showDefaultWarning, setShowDefaultWarning] = useState(false)

  useEffect(() => {
    // Check auth status on load
    api.authStatus()
      .then(status => {
        setAuthenticated(status.authenticated)
        if (status.authenticated && status.default_password) {
          setShowDefaultWarning(true)
        }
      })
      .catch(() => setAuthenticated(false))
  }, [])

  useEffect(() => {
    if (!authenticated) return
    api.setupStatus().then(status => {
      if (status.setup_mode) {
        setSetupMode(true)
        setPage('setup')
      }
    }).catch(() => {})
  }, [authenticated])

  const handleLogin = (defaultPassword: boolean) => {
    setAuthenticated(true)
    if (defaultPassword) {
      setShowDefaultWarning(true)
    }
  }

  const handleLogout = async () => {
    await api.authLogout().catch(() => {})
    setAuthenticated(false)
  }

  // Still checking auth
  if (authenticated === null) {
    return <div class="min-h-screen flex items-center justify-center bg-slate-900 text-slate-400">Loading...</div>
  }

  // Not authenticated — show login
  if (!authenticated) {
    return <LoginScreen onLogin={handleLogin} />
  }

  const content = () => {
    switch (page) {
      case 'dashboard': return <Dashboard />
      case 'import': return <ImportFlow />
      case 'config': return <ConfigEditor />
      case 'setup': return <SetupWizard onComplete={() => { setSetupMode(false); setPage('dashboard') }} />
      default: return <Dashboard />
    }
  }

  return (
    <Layout currentPage={page} onNavigate={setPage} setupMode={setupMode} onLogout={handleLogout}>
      {showDefaultWarning && (
        <div class="bg-amber-500/10 border border-amber-500/30 rounded p-3 mb-4 text-amber-300 text-sm flex items-center justify-between">
          <span>You are using the default admin password. Change it in the admin settings for security.</span>
          <button onClick={() => setShowDefaultWarning(false)} class="text-amber-400 hover:text-amber-300 ml-2">&times;</button>
        </div>
      )}
      {content()}
    </Layout>
  )
}

render(<App />, document.getElementById('app')!)
