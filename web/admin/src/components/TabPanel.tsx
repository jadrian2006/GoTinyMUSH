import { useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'

interface Tab {
  id: string
  label: string
  badge?: string | number
}

interface TabPanelProps {
  tabs: Tab[]
  children: (activeTab: string) => ComponentChildren
}

export function TabPanel({ tabs, children }: TabPanelProps) {
  const [activeTab, setActiveTab] = useState(tabs[0]?.id || '')

  return (
    <div>
      <div class="flex border-b border-slate-700 mb-4">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            class={`px-4 py-2 text-sm transition-colors border-b-2 -mb-px ${
              activeTab === tab.id
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-400 hover:text-slate-300'
            }`}
          >
            {tab.label}
            {tab.badge !== undefined && (
              <span class="ml-1.5 px-1.5 py-0.5 text-xs bg-slate-700 rounded-full">{tab.badge}</span>
            )}
          </button>
        ))}
      </div>
      {children(activeTab)}
    </div>
  )
}
