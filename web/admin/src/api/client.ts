const BASE = '/admin/api'

// Global callback for 401 responses — set by App to trigger re-login
let onAuthLost: (() => void) | null = null
export function setAuthLostHandler(handler: () => void) {
  onAuthLost = handler
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) {
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(`${BASE}${path}`, opts)
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/auth/')) {
      onAuthLost?.()
    }
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export const api = {
  // Server
  serverStatus: () => request<any>('GET', '/server/status'),
  serverStart: () => request<any>('POST', '/server/start'),
  serverStop: () => request<any>('POST', '/server/stop'),

  // Config
  getConfig: () => request<any>('GET', '/config'),
  putConfig: (config: Record<string, unknown>) => request<any>('PUT', '/config', config),

  // Import — existing
  importUpload: (path: string) => request<any>('POST', '/import/upload', { path }),
  importUploadFile: async (file: File) => {
    const form = new FormData()
    form.append('flatfile', file)
    const res = await fetch(`${BASE}/import/upload`, { method: 'POST', body: form })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(err.error || res.statusText)
    }
    return res.json()
  },
  importValidate: () => request<any>('POST', '/import/validate'),
  importFindings: () => request<any>('GET', '/import/findings'),
  importFix: (findingId?: string, category?: string) =>
    request<any>('POST', '/import/fix', { finding_id: findingId, category }),
  importCommit: () => request<any>('POST', '/import/commit'),
  importCommitProgress: () => request<any>('GET', '/import/commit/progress'),

  // Import — new session management
  importSession: () => request<any>('GET', '/import/session'),
  importReset: () => request<any>('DELETE', '/import/session'),
  importDiscover: () => request<any>('POST', '/import/discover'),
  importAssign: (path: string, role: string) =>
    request<any>('POST', '/import/assign', { path, role }),
  importConvertConfig: (path: string) =>
    request<any>('POST', '/import/convert-config', { path }),
  importParseComsys: (path: string) =>
    request<any>('POST', '/import/parse-comsys', { path }),
  importGetFile: (role: string, name: string) =>
    request<any>('GET', `/import/file/${encodeURIComponent(role)}/${encodeURIComponent(name)}`),
  importPutFile: (role: string, name: string, content: string) =>
    request<any>('PUT', `/import/file/${encodeURIComponent(role)}/${encodeURIComponent(name)}`, { content }),

  // Shutdown
  serverShutdown: (delay?: number, reason?: string) =>
    request<any>('POST', '/server/shutdown', { delay: delay || 300, reason: reason || '' }),
  shutdownStatus: () => request<any>('GET', '/server/shutdown'),
  shutdownCancel: () => request<any>('DELETE', '/server/shutdown'),

  // Setup
  setupStatus: () => request<any>('GET', '/setup/status'),
  createNewDB: () => request<any>('POST', '/import/create-new'),
  serverLaunch: () => request<any>('POST', '/server/launch'),

  // Auth
  authLogin: (password: string) => request<any>('POST', '/auth/login', { password }),
  authLogout: () => request<any>('POST', '/auth/logout'),
  authStatus: () => request<any>('GET', '/auth/status'),
  authChangePassword: (current: string, newPass: string) =>
    request<any>('POST', '/auth/change-password', { current, new: newPass }),
}
