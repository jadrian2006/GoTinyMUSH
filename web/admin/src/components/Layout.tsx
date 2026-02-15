import { ComponentChildren } from 'preact'
import { branding } from '../tokens/branding'

type Page = 'dashboard' | 'import' | 'config' | 'setup'

interface LayoutProps {
  currentPage: Page
  onNavigate: (page: Page) => void
  setupMode: boolean
  onLogout?: () => void
  children: ComponentChildren
}

const navItems: { page: Page; label: string; icon: string }[] = [
  { page: 'dashboard', label: 'Dashboard', icon: '\u2302' },
  { page: 'import', label: 'Import', icon: '\u21E7' },
  { page: 'config', label: 'Config', icon: '\u2699' },
]

export function Layout({ currentPage, onNavigate, setupMode, onLogout, children }: LayoutProps) {
  return (
    <div class="flex min-h-screen">
      {/* Sidebar */}
      <aside class="w-56 bg-slate-800 border-r border-slate-700 flex flex-col">
        <div class="p-4 border-b border-slate-700">
          <h1 class="text-lg font-bold text-indigo-400">{branding.appName}</h1>
          <p class="text-xs text-slate-400">{branding.tagline}</p>
        </div>
        <nav class="flex-1 p-2">
          {navItems.map(item => (
            <button
              key={item.page}
              onClick={() => onNavigate(item.page)}
              class={`w-full text-left px-3 py-2 rounded text-sm mb-1 flex items-center gap-2 transition-colors ${
                currentPage === item.page
                  ? 'bg-indigo-500/20 text-indigo-300'
                  : 'text-slate-300 hover:bg-slate-700'
              }`}
            >
              <span>{item.icon}</span>
              {item.label}
            </button>
          ))}
        </nav>
        {setupMode && (
          <div class="p-3 m-2 bg-amber-500/10 border border-amber-500/30 rounded text-xs text-amber-300">
            Setup mode active
          </div>
        )}
        {onLogout && (
          <div class="p-2 border-t border-slate-700">
            <button
              onClick={onLogout}
              class="w-full text-left px-3 py-2 rounded text-sm text-slate-400 hover:bg-slate-700 hover:text-slate-300 transition-colors"
            >
              Logout
            </button>
          </div>
        )}
      </aside>

      {/* Main content */}
      <main class="flex-1 p-6 overflow-auto">
        {children}
      </main>
    </div>
  )
}
