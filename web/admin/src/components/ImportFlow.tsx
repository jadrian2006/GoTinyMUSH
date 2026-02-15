import { useState } from 'preact/hooks'
import { api } from '../api/client'
import { FindingList } from './FindingList'
import { TabPanel } from './TabPanel'
import { TextFileEditor } from './TextFileEditor'
import { AliasEditor } from './AliasEditor'
import { ConfigEditor } from './ConfigEditor'
import type { Finding } from './FindingCard'
import type { DiscoveredFile, FileRole, ChannelSummary } from '../types/import'

type Step = 'upload' | 'discover' | 'configure' | 'summary' | 'resolve' | 'review' | 'committed'

const ALL_STEPS: Step[] = ['upload', 'discover', 'configure', 'summary', 'resolve', 'review', 'committed']

const ROLE_LABELS: Record<FileRole, string> = {
  flatfile: 'Flatfile',
  comsys: 'Comsys DB',
  main_config: 'Main Config',
  alias_config: 'Alias Config',
  text: 'Text File',
  dict: 'Dictionary',
  discarded: 'Discarded',
  unknown: 'Unknown',
}

const ROLE_OPTIONS: FileRole[] = ['flatfile', 'comsys', 'main_config', 'alias_config', 'text', 'dict', 'discarded', 'unknown']

const CONFIDENCE_DOTS: Record<string, string> = {
  high: 'text-green-400',
  medium: 'text-yellow-400',
  low: 'text-red-400',
  manual: 'text-blue-400',
}

export function ImportFlow() {
  const [step, setStep] = useState<Step>('upload')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [uploadResult, setUploadResult] = useState<any>(null)
  const [findings, setFindings] = useState<Finding[]>([])
  const [report, setReport] = useState<any>(null)
  const [discoveredFiles, setDiscoveredFiles] = useState<DiscoveredFile[]>([])
  const [configContent, setConfigContent] = useState<string>('')
  const [comsysData, setComsysData] = useState<{ channels: ChannelSummary[], alias_count: number } | null>(null)
  const [textFileNames, setTextFileNames] = useState<string[]>([])
  const [aliasFileNames, setAliasFileNames] = useState<string[]>([])

  const handleFileUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    if (!file) return

    setLoading(true)
    setError('')
    try {
      const result = await api.importUploadFile(file)
      setUploadResult(result)
      // Route to appropriate step based on source
      if (result.source === 'foreign_archive' || result.source === 'gotinymush_archive') {
        setStep('discover')
        // Auto-run discovery
        const discoverResult = await api.importDiscover()
        setDiscoveredFiles(discoverResult.files || [])
      } else {
        setStep('summary')
      }
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handlePathUpload = async (path: string) => {
    setLoading(true)
    setError('')
    try {
      const result = await api.importUpload(path)
      setUploadResult(result)
      if (result.source === 'foreign_archive' || result.source === 'gotinymush_archive') {
        setStep('discover')
        const discoverResult = await api.importDiscover()
        setDiscoveredFiles(discoverResult.files || [])
      } else {
        setStep('summary')
      }
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleReassign = async (path: string, role: FileRole) => {
    try {
      await api.importAssign(path, role)
      setDiscoveredFiles(prev => prev.map(f =>
        f.path === path ? { ...f, role, confidence: 'manual' as const, reason: 'manually assigned' } : f
      ))
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleDiscoverConfirm = async () => {
    setLoading(true)
    setError('')
    try {
      // Check for config files to convert
      const configFile = discoveredFiles.find(f => f.role === 'main_config')
      if (configFile) {
        const ext = configFile.path.split('.').pop()?.toLowerCase()
        if (ext === 'conf') {
          const result = await api.importConvertConfig(configFile.path)
          setConfigContent(result.content || '')
        } else {
          // YAML config, read directly
          const result = await api.importGetFile('main_config', configFile.path.split('/').pop() || '')
          setConfigContent(result.content || '')
        }
      }

      // Parse comsys if found
      const comsysFile = discoveredFiles.find(f => f.role === 'comsys')
      if (comsysFile) {
        const result = await api.importParseComsys(comsysFile.path)
        setComsysData({ channels: result.channels || [], alias_count: result.alias_count || 0 })
      }

      // Get session to find staged file names
      const session = await api.importSession()
      setTextFileNames(session.text_files || [])
      setAliasFileNames(session.alias_files || [])

      // If we have a flatfile loaded, go to configure then summary
      // If not, still show configure for text/config files
      setStep('configure')
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleConfigureDone = () => {
    if (uploadResult?.object_count || discoveredFiles.some(f => f.role === 'flatfile')) {
      setStep('summary')
    } else {
      // No flatfile? Go to summary anyway but with limited info
      setStep('summary')
    }
  }

  const handleValidate = async () => {
    setLoading(true)
    setError('')
    try {
      const result = await api.importValidate()
      setFindings(result.report?.findings || [])
      setReport(result.report)
      setStep('resolve')
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleFix = async (id: string) => {
    try {
      await api.importFix(id)
      const result = await api.importFindings()
      setFindings(result.findings || [])
      setReport(result)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleFixAll = async (category: string) => {
    try {
      await api.importFix(undefined, category)
      const result = await api.importFindings()
      setFindings(result.findings || [])
      setReport(result)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleCommit = async () => {
    setLoading(true)
    setError('')
    try {
      await api.importCommit()
      setStep('committed')
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const handleReset = async () => {
    try {
      await api.importReset()
      setStep('upload')
      setUploadResult(null)
      setDiscoveredFiles([])
      setConfigContent('')
      setComsysData(null)
      setTextFileNames([])
      setAliasFileNames([])
      setFindings([])
      setReport(null)
    } catch (e: any) {
      setError(e.message)
    }
  }

  const refreshFindings = async () => {
    try {
      const result = await api.importFindings()
      setFindings(result.findings || [])
      setReport(result)
    } catch (e: any) {
      setError(e.message)
    }
  }

  // Determine which steps to show based on source
  const visibleSteps = uploadResult?.source === 'flatfile'
    ? ALL_STEPS.filter(s => s !== 'discover' && s !== 'configure')
    : ALL_STEPS

  return (
    <div>
      <div class="flex items-center justify-between mb-6">
        <h2 class="text-2xl font-bold">Import</h2>
        {step !== 'upload' && step !== 'committed' && (
          <button
            onClick={handleReset}
            class="px-3 py-1 bg-slate-700 hover:bg-slate-600 text-slate-300 rounded text-xs transition-colors"
          >
            Start Over
          </button>
        )}
      </div>

      {/* Step indicators */}
      <div class="flex items-center gap-2 mb-6 text-sm flex-wrap">
        {visibleSteps.map((s, i) => (
          <div key={s} class="flex items-center gap-2">
            {i > 0 && <span class="text-slate-600">&rarr;</span>}
            <span class={step === s ? 'text-indigo-400 font-semibold' : 'text-slate-500'}>
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </span>
          </div>
        ))}
      </div>

      {error && (
        <div class="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4 text-red-300 text-sm">
          {error}
          <button onClick={() => setError('')} class="ml-2 text-red-400 hover:text-red-300">&times;</button>
        </div>
      )}

      {step === 'upload' && (
        <UploadStep
          loading={loading}
          onFileUpload={handleFileUpload}
          onPathUpload={handlePathUpload}
        />
      )}

      {step === 'discover' && (
        <DiscoverStep
          files={discoveredFiles}
          loading={loading}
          onReassign={handleReassign}
          onConfirm={handleDiscoverConfirm}
        />
      )}

      {step === 'configure' && (
        <ConfigureStep
          configContent={configContent}
          comsysData={comsysData}
          textFileNames={textFileNames}
          aliasFileNames={aliasFileNames}
          onConfigSave={(content) => {
            setConfigContent(content)
            api.importPutFile('main_config', 'config', content).catch(() => {})
          }}
          onDone={handleConfigureDone}
        />
      )}

      {step === 'summary' && uploadResult && (
        <SummaryStep
          result={uploadResult}
          loading={loading}
          onValidate={handleValidate}
          hasExtraFiles={textFileNames.length > 0 || aliasFileNames.length > 0 || !!configContent}
        />
      )}

      {step === 'resolve' && (
        <div>
          {report?.categories && (
            <div class="grid grid-cols-2 md:grid-cols-3 gap-3 mb-4">
              {Object.entries(report.categories as Record<string, any>).map(([key, cat]) => (
                <div key={key} class="bg-slate-800 rounded p-3">
                  <div class="text-xs text-slate-400">{cat.label || key}</div>
                  <div class="text-lg font-bold text-slate-200">{cat.total}</div>
                  {cat.fixed > 0 && <div class="text-xs text-green-400">{cat.fixed} fixed</div>}
                </div>
              ))}
            </div>
          )}

          <FindingList findings={findings} onFix={handleFix} onFixAll={handleFixAll} />

          <div class="flex gap-3 mt-4">
            <button
              onClick={() => setStep('review')}
              class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors"
            >
              Review & Commit
            </button>
            <button
              onClick={refreshFindings}
              class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-sm transition-colors"
            >
              Refresh
            </button>
          </div>
        </div>
      )}

      {step === 'review' && (
        <div>
          <h3 class="text-lg font-semibold mb-3">Review Changes</h3>
          <div class="bg-slate-800 rounded p-4 mb-4">
            <p class="text-sm text-slate-300">
              {findings.filter(f => f.fixed).length} fixes applied out of {findings.length} total findings.
            </p>
            <p class="text-sm text-slate-400 mt-1">
              {findings.filter(f => f.fixable && !f.fixed).length} fixable findings remain unapplied.
            </p>
            {(textFileNames.length > 0 || !!configContent) && (
              <div class="mt-3 pt-3 border-t border-slate-700">
                <p class="text-sm text-slate-400">Additional files staged for commit:</p>
                <ul class="text-sm text-slate-300 mt-1 list-disc ml-4">
                  {configContent && <li>Game configuration (YAML)</li>}
                  {textFileNames.length > 0 && <li>{textFileNames.length} text file(s)</li>}
                  {aliasFileNames.length > 0 && <li>{aliasFileNames.length} alias config(s)</li>}
                </ul>
              </div>
            )}
          </div>
          <div class="flex gap-3">
            <button
              onClick={handleCommit}
              disabled={loading}
              class="px-4 py-2 bg-green-600 hover:bg-green-500 text-white rounded text-sm transition-colors disabled:opacity-50"
            >
              {loading ? 'Committing...' : 'Commit to Database'}
            </button>
            <button
              onClick={() => setStep('resolve')}
              class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded text-sm transition-colors"
            >
              Back to Resolve
            </button>
          </div>
        </div>
      )}

      {step === 'committed' && (
        <CommittedStep />
      )}
    </div>
  )
}

function UploadStep({ loading, onFileUpload, onPathUpload }: {
  loading: boolean
  onFileUpload: (e: Event) => void
  onPathUpload: (path: string) => void
}) {
  const [path, setPath] = useState('')

  return (
    <div class="max-w-lg">
      <div class="bg-slate-800 border-2 border-dashed border-slate-600 rounded-lg p-8 text-center mb-4">
        <p class="text-slate-400 mb-3">Upload a flatfile, archive (.tar.gz, .zip), or browse</p>
        <input
          type="file"
          accept=".flat,.FLAT,.gz,.tar.gz,.tgz,.zip"
          onChange={onFileUpload}
          disabled={loading}
          class="block w-full text-sm text-slate-400 file:mr-4 file:py-2 file:px-4 file:rounded file:border-0 file:text-sm file:bg-indigo-600 file:text-white hover:file:bg-indigo-500 file:cursor-pointer"
        />
      </div>
      <div class="text-slate-500 text-center text-sm mb-4">or</div>
      <div class="flex gap-2">
        <input
          type="text"
          placeholder="Path to file or directory on server..."
          value={path}
          onInput={(e) => setPath((e.target as HTMLInputElement).value)}
          class="flex-1 px-3 py-2 bg-slate-800 border border-slate-600 rounded text-sm text-slate-200 focus:outline-none focus:border-indigo-500"
        />
        <button
          onClick={() => path && onPathUpload(path)}
          disabled={loading || !path}
          class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors disabled:opacity-50"
        >
          {loading ? 'Loading...' : 'Load'}
        </button>
      </div>
    </div>
  )
}

function DiscoverStep({ files, loading, onReassign, onConfirm }: {
  files: DiscoveredFile[]
  loading: boolean
  onReassign: (path: string, role: FileRole) => void
  onConfirm: () => void
}) {
  const roleGroups = files.reduce((acc, f) => {
    const r = f.role
    acc[r] = (acc[r] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  return (
    <div>
      <h3 class="text-lg font-semibold mb-3">Discovered Files</h3>
      <p class="text-sm text-slate-400 mb-4">
        Found {files.length} files. Review role assignments and adjust if needed.
      </p>

      {/* Role summary badges */}
      <div class="flex flex-wrap gap-2 mb-4">
        {Object.entries(roleGroups).map(([role, count]) => (
          <span key={role} class="px-2 py-1 bg-slate-800 rounded text-xs text-slate-300">
            {ROLE_LABELS[role as FileRole] || role}: {count}
          </span>
        ))}
      </div>

      {/* Discarded files notice */}
      {roleGroups['discarded'] > 0 && (
        <div class="bg-slate-800/50 border border-slate-700 rounded p-3 mb-4 text-sm text-slate-400">
          {roleGroups['discarded']} file(s) marked as discarded (C TinyMUSH artifacts not needed by GoTinyMUSH).
          These will be skipped during import. Your original archive is unchanged.
        </div>
      )}

      {/* File table */}
      <div class="bg-slate-800 rounded overflow-hidden mb-4">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-700">
              <th class="text-left px-3 py-2 text-slate-400 font-medium">File</th>
              <th class="text-left px-3 py-2 text-slate-400 font-medium w-40">Role</th>
              <th class="text-center px-3 py-2 text-slate-400 font-medium w-16">Conf.</th>
              <th class="text-right px-3 py-2 text-slate-400 font-medium w-20">Size</th>
            </tr>
          </thead>
          <tbody>
            {files.map(f => (
              <tr key={f.path} class={`border-b border-slate-700/50 hover:bg-slate-750 ${f.role === 'discarded' ? 'opacity-40' : ''}`}>
                <td class="px-3 py-2">
                  <div class={`font-mono text-xs ${f.role === 'discarded' ? 'text-slate-500 line-through' : 'text-slate-300'}`}>{f.path}</div>
                  <div class="text-xs text-slate-500 mt-0.5">{f.reason}</div>
                </td>
                <td class="px-3 py-2">
                  <select
                    value={f.role}
                    onChange={(e) => onReassign(f.path, (e.target as HTMLSelectElement).value as FileRole)}
                    class="w-full px-2 py-1 bg-slate-900 border border-slate-600 rounded text-xs text-slate-300 focus:outline-none focus:border-indigo-500"
                  >
                    {ROLE_OPTIONS.map(r => (
                      <option key={r} value={r}>{ROLE_LABELS[r]}</option>
                    ))}
                  </select>
                </td>
                <td class="px-3 py-2 text-center">
                  <span class={`text-lg ${CONFIDENCE_DOTS[f.confidence] || 'text-slate-500'}`} title={f.confidence}>
                    &#9679;
                  </span>
                </td>
                <td class="px-3 py-2 text-right text-slate-400 text-xs font-mono">
                  {formatSize(f.size)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <button
        onClick={onConfirm}
        disabled={loading}
        class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors disabled:opacity-50"
      >
        {loading ? 'Processing...' : 'Confirm & Continue'}
      </button>
    </div>
  )
}

function ConfigureStep({ configContent, comsysData, textFileNames, aliasFileNames, onConfigSave, onDone }: {
  configContent: string
  comsysData: { channels: ChannelSummary[], alias_count: number } | null
  textFileNames: string[]
  aliasFileNames: string[]
  onConfigSave: (content: string) => void
  onDone: () => void
}) {
  const tabs = []
  if (configContent) tabs.push({ id: 'config', label: 'Config' })
  if (aliasFileNames.length > 0) tabs.push({ id: 'aliases', label: 'Aliases', badge: aliasFileNames.length })
  if (textFileNames.length > 0) tabs.push({ id: 'text', label: 'Text Files', badge: textFileNames.length })
  if (comsysData) tabs.push({ id: 'comsys', label: 'Comsys' })

  if (tabs.length === 0) {
    return (
      <div>
        <p class="text-slate-400 mb-4">No additional files to configure.</p>
        <button
          onClick={onDone}
          class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors"
        >
          Continue to Summary
        </button>
      </div>
    )
  }

  return (
    <div>
      <h3 class="text-lg font-semibold mb-3">Configure Files</h3>
      <p class="text-sm text-slate-400 mb-4">
        Review and edit staged files before committing.
      </p>

      <TabPanel tabs={tabs}>
        {(activeTab) => (
          <>
            {activeTab === 'config' && (
              <ConfigEditor
                mode="staged"
                stagedContent={configContent}
                onStagedSave={onConfigSave}
              />
            )}

            {activeTab === 'aliases' && (
              <div class="space-y-4">
                {aliasFileNames.map(name => (
                  <AliasEditor key={name} name={name} />
                ))}
              </div>
            )}

            {activeTab === 'text' && (
              <div class="space-y-4">
                {textFileNames.map(name => (
                  <TextFileEditor key={name} name={name} role="text" mode="staged" />
                ))}
              </div>
            )}

            {activeTab === 'comsys' && comsysData && (
              <div class="bg-slate-800 rounded p-4">
                <div class="grid grid-cols-2 gap-4 mb-4">
                  <div>
                    <span class="text-xs text-slate-400">Channels</span>
                    <div class="text-lg font-bold text-slate-200">{comsysData.channels.length}</div>
                  </div>
                  <div>
                    <span class="text-xs text-slate-400">Player Aliases</span>
                    <div class="text-lg font-bold text-slate-200">{comsysData.alias_count}</div>
                  </div>
                </div>
                {comsysData.channels.length > 0 && (
                  <table class="w-full text-sm">
                    <thead>
                      <tr class="border-b border-slate-700">
                        <th class="text-left px-2 py-1 text-slate-400">Channel</th>
                        <th class="text-left px-2 py-1 text-slate-400">Owner</th>
                        <th class="text-left px-2 py-1 text-slate-400">Description</th>
                        <th class="text-right px-2 py-1 text-slate-400">Messages</th>
                      </tr>
                    </thead>
                    <tbody>
                      {comsysData.channels.map(ch => (
                        <tr key={ch.name} class="border-b border-slate-700/50">
                          <td class="px-2 py-1 text-slate-300 font-mono">{ch.name}</td>
                          <td class="px-2 py-1 text-slate-400">#{ch.owner}</td>
                          <td class="px-2 py-1 text-slate-400 truncate max-w-xs">{ch.description}</td>
                          <td class="px-2 py-1 text-slate-400 text-right">{ch.num_sent}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            )}
          </>
        )}
      </TabPanel>

      <div class="mt-4">
        <button
          onClick={onDone}
          class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors"
        >
          Continue to Summary
        </button>
      </div>
    </div>
  )
}

function SummaryStep({ result, loading, onValidate, hasExtraFiles }: {
  result: any
  loading: boolean
  onValidate: () => void
  hasExtraFiles?: boolean
}) {
  return (
    <div>
      <h3 class="text-lg font-semibold mb-3">Import Summary</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
        <div class="bg-slate-800 rounded p-3">
          <div class="text-xs text-slate-400">Objects</div>
          <div class="text-xl font-bold text-slate-200">{result.object_count ?? 0}</div>
        </div>
        <div class="bg-slate-800 rounded p-3">
          <div class="text-xs text-slate-400">Attr Definitions</div>
          <div class="text-xl font-bold text-slate-200">{result.attr_defs ?? 0}</div>
        </div>
        <div class="bg-slate-800 rounded p-3">
          <div class="text-xs text-slate-400">Total Attributes</div>
          <div class="text-xl font-bold text-slate-200">{result.total_attrs ?? 0}</div>
        </div>
        <div class="bg-slate-800 rounded p-3">
          <div class="text-xs text-slate-400">Source</div>
          <div class="text-sm font-mono text-slate-200 truncate">{result.source || result.file}</div>
        </div>
      </div>

      {result.type_counts && (
        <div class="bg-slate-800 rounded p-4 mb-4">
          <h4 class="text-sm text-slate-400 mb-2">Object Types</h4>
          <div class="grid grid-cols-3 md:grid-cols-6 gap-2">
            {Object.entries(result.type_counts as Record<string, number>).map(([type, count]) => (
              <div key={type}>
                <span class="text-xs text-slate-500">{type}: </span>
                <span class="text-sm text-slate-200">{count}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {hasExtraFiles && (
        <div class="bg-slate-800/50 rounded p-3 mb-4 text-sm text-slate-400">
          Additional files (config, text, aliases) are staged and will be committed with the database.
        </div>
      )}

      <button
        onClick={onValidate}
        disabled={loading}
        class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-sm transition-colors disabled:opacity-50"
      >
        {loading ? 'Validating...' : 'Run Validation'}
      </button>
    </div>
  )
}

function CommittedStep() {
  const [launching, setLaunching] = useState(false)
  const [error, setError] = useState('')

  const handleLaunch = async () => {
    setLaunching(true)
    setError('')
    try {
      await api.serverLaunch()
    } catch {
      // Expected — server exits, connection drops
    }
    // Poll until server comes back
    let attempts = 0
    const poll = setInterval(async () => {
      attempts++
      try {
        await api.serverStatus()
        clearInterval(poll)
        window.location.reload()
      } catch {
        if (attempts > 30) {
          clearInterval(poll)
          setError('Server did not come back after restart. Check Docker logs.')
          setLaunching(false)
        }
      }
    }, 2000)
  }

  if (launching) {
    return (
      <div class="text-center py-12">
        <div class="inline-block w-8 h-8 border-4 border-indigo-400 border-t-transparent rounded-full animate-spin mb-4" />
        <h3 class="text-xl font-semibold mb-2">Launching Server...</h3>
        <p class="text-slate-400">The server is restarting with your imported database.</p>
      </div>
    )
  }

  const hostname = window.location.hostname

  return (
    <div class="text-center py-12">
      <div class="text-4xl mb-4 text-green-400">&#10003;</div>
      <h3 class="text-xl font-semibold mb-2">Database Committed</h3>
      <p class="text-slate-400 mb-6">
        The database has been written to disk and is ready to go.
      </p>
      {error && (
        <div class="bg-red-500/10 border border-red-500/30 rounded p-3 mb-4 text-red-300 text-sm inline-block">
          {error}
        </div>
      )}
      <div class="mb-6">
        <button
          onClick={handleLaunch}
          class="px-6 py-2 bg-green-600 hover:bg-green-500 text-white rounded transition-colors"
        >
          Launch Server
        </button>
      </div>
      <div class="bg-slate-800 rounded p-4 text-left max-w-md mx-auto text-sm">
        <h4 class="text-slate-400 text-xs uppercase tracking-wider mb-2">After launch, connect via:</h4>
        <div class="space-y-1">
          <div><span class="text-slate-500">Telnet:</span> <code class="text-cyan-300">telnet {hostname} 6886</code></div>
          <div><span class="text-slate-500">MU* Client:</span> <span class="text-slate-300">{hostname} port 6886</span></div>
          <div><span class="text-slate-500">God Login:</span> <code class="text-cyan-300">connect Wizard &lt;password&gt;</code></div>
          <p class="text-slate-500 text-xs mt-2">Password set via MUSH_GODPASS env variable.</p>
        </div>
      </div>
    </div>
  )
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
