import { useState } from 'preact/hooks'
import { api } from '../api/client'
import { branding } from '../tokens/branding'

interface SetupWizardProps {
  onComplete: () => void
}

type WizardStep = 'welcome' | 'config' | 'database' | 'ready' | 'launching'

export function SetupWizard({ onComplete }: SetupWizardProps) {
  const [step, setStep] = useState<WizardStep>('welcome')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [dbCreated, setDbCreated] = useState(false)

  const handleCreateNew = async () => {
    setLoading(true)
    setError('')
    try {
      await api.createNewDB()
      setDbCreated(true)
      setStep('ready')
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleLaunch = async () => {
    setStep('launching')
    setError('')
    try {
      await api.serverLaunch()
      // Server will restart — poll until it comes back
      setTimeout(() => pollUntilReady(), 2000)
    } catch {
      // Expected: connection drops when process exits
      setTimeout(() => pollUntilReady(), 2000)
    }
  }

  const pollUntilReady = () => {
    let attempts = 0
    const poll = setInterval(async () => {
      attempts++
      try {
        const status = await api.serverStatus()
        if (status) {
          clearInterval(poll)
          onComplete()
        }
      } catch {
        if (attempts > 30) {
          clearInterval(poll)
          setError('Server did not come back after restart. Check Docker logs.')
          setStep('ready')
        }
      }
    }, 2000)
  }

  const stepLabels: WizardStep[] = ['welcome', 'config', 'database', 'ready']

  return (
    <div class="max-w-2xl mx-auto">
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-indigo-400 mb-2">{branding.appName}</h1>
        <p class="text-slate-400">Setup Wizard</p>
      </div>

      {/* Progress */}
      <div class="flex items-center justify-center gap-4 mb-8">
        {stepLabels.map((s, i) => (
          <div key={s} class="flex items-center gap-2">
            {i > 0 && <div class="w-8 h-px bg-slate-600" />}
            <div class={`w-8 h-8 rounded-full flex items-center justify-center text-sm ${
              step === s || (step === 'launching' && s === 'ready')
                ? 'bg-indigo-600 text-white'
                : getStepIndex(step) > i
                ? 'bg-green-600 text-white'
                : 'bg-slate-700 text-slate-400'
            }`}>
              {getStepIndex(step) > i ? '\u2713' : i + 1}
            </div>
          </div>
        ))}
      </div>

      {error && (
        <div class="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4 text-red-300 text-sm">
          {error}
          <button onClick={() => setError('')} class="ml-2 text-red-400 hover:text-red-300">&times;</button>
        </div>
      )}

      {step === 'welcome' && (
        <div class="bg-slate-800 rounded-lg p-8 text-center">
          <h2 class="text-xl font-semibold mb-4">Welcome to {branding.appName}</h2>
          <p class="text-slate-400 mb-6">
            This wizard will help you configure your MUSH server and set up your game database.
          </p>
          <button
            onClick={() => setStep('config')}
            class="px-6 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded transition-colors"
          >
            Get Started
          </button>
        </div>
      )}

      {step === 'config' && (
        <div class="bg-slate-800 rounded-lg p-8">
          <h2 class="text-xl font-semibold mb-4">Configuration</h2>
          <p class="text-slate-400 mb-4">
            Your game configuration (game.yaml) contains server settings like port, name, and features.
            A default configuration will be created automatically. You can customize it later from the Config page.
          </p>
          <div class="flex gap-3">
            <button
              onClick={() => setStep('database')}
              class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors"
            >
              Continue
            </button>
            <button
              onClick={() => setStep('database')}
              class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-sm transition-colors"
            >
              Skip
            </button>
          </div>
        </div>
      )}

      {step === 'database' && (
        <div class="bg-slate-800 rounded-lg p-8">
          <h2 class="text-xl font-semibold mb-4">Game Database</h2>
          <p class="text-slate-400 mb-6">
            Choose how to set up your game database:
          </p>

          <div class="space-y-3 mb-6">
            <button
              onClick={handleCreateNew}
              disabled={loading}
              class="w-full text-left p-4 bg-indigo-600/20 border border-indigo-500/40 rounded-lg hover:bg-indigo-600/30 transition-colors disabled:opacity-50"
            >
              <div class="text-indigo-300 font-semibold mb-1">
                {loading ? 'Creating...' : 'New Database (Recommended)'}
              </div>
              <div class="text-slate-400 text-sm">
                Start fresh with an empty world. Creates Room Zero, Master Room, and the God (Wizard) player.
              </div>
            </button>

            <button
              onClick={() => {
                // Navigate to Import page — onComplete switches to dashboard,
                // but we want the Import page. Signal via a special callback or
                // just go to ready and let them navigate.
                setStep('ready')
              }}
              class="w-full text-left p-4 bg-slate-700/50 border border-slate-600 rounded-lg hover:bg-slate-700 transition-colors"
            >
              <div class="text-slate-200 font-semibold mb-1">Import from Backup</div>
              <div class="text-slate-400 text-sm">
                Upload a TinyMUSH flatfile (.FLAT) or archive (.tar.gz, .zip) with your existing game data.
                Use the Import page from the sidebar after setup.
              </div>
            </button>
          </div>
        </div>
      )}

      {step === 'ready' && (
        <div class="bg-slate-800 rounded-lg p-8">
          {dbCreated ? (
            <>
              <div class="text-center">
                <div class="text-4xl mb-4 text-green-400">&#10003;</div>
                <h2 class="text-xl font-semibold mb-2">Database Created</h2>
                <p class="text-slate-400 mb-6">
                  Your new game world is ready with Room Zero, Master Room, and the Wizard player.
                  Seed files (text, dict, config, aliases) have been copied to the data directory.
                </p>
              </div>

              <div class="bg-slate-900 rounded p-4 mb-6 text-sm">
                <h4 class="text-slate-400 text-xs uppercase tracking-wider mb-2">After Launch</h4>
                <div class="space-y-1 text-slate-300">
                  <div><span class="text-slate-500">Telnet:</span> <code class="text-cyan-300">telnet {window.location.hostname} 6886</code></div>
                  <div><span class="text-slate-500">MU* Client:</span> {window.location.hostname} port 6886</div>
                  <div><span class="text-slate-500">Web Client:</span> <code class="text-cyan-300">http://{window.location.hostname}:8443/</code></div>
                  <div class="mt-2 pt-2 border-t border-slate-700">
                    <span class="text-slate-500">God Login:</span> <code class="text-cyan-300">connect Wizard &lt;password&gt;</code>
                  </div>
                  <p class="text-slate-500 text-xs mt-1">
                    God password is set via MUSH_GODPASS in docker-compose.yml.
                    Change it in-game: <code>@password &lt;old&gt;=&lt;new&gt;</code>
                  </p>
                </div>
              </div>

              <div class="flex gap-3 justify-center">
                <button
                  onClick={handleLaunch}
                  class="px-6 py-2 bg-green-600 hover:bg-green-500 text-white rounded transition-colors"
                >
                  Launch Server
                </button>
                <button
                  onClick={onComplete}
                  class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-sm transition-colors"
                >
                  Go to Dashboard First
                </button>
              </div>
            </>
          ) : (
            <>
              <div class="text-center">
                <h2 class="text-xl font-semibold mb-4">Setup Complete</h2>
                <p class="text-slate-400 mb-6">
                  You can import a game database from the Import page, or create a new one.
                  Use the sidebar to navigate to the Import page when ready.
                </p>
                <button
                  onClick={onComplete}
                  class="px-6 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded transition-colors"
                >
                  Go to Dashboard
                </button>
              </div>
            </>
          )}
        </div>
      )}

      {step === 'launching' && (
        <div class="bg-slate-800 rounded-lg p-8 text-center">
          <div class="mb-4">
            <div class="inline-block w-8 h-8 border-4 border-indigo-400 border-t-transparent rounded-full animate-spin" />
          </div>
          <h2 class="text-xl font-semibold mb-4">Launching Server...</h2>
          <p class="text-slate-400">
            The server is restarting with your new database. This may take a few seconds.
          </p>
        </div>
      )}
    </div>
  )
}

function getStepIndex(step: WizardStep): number {
  const steps: WizardStep[] = ['welcome', 'config', 'database', 'ready']
  return steps.indexOf(step)
}
