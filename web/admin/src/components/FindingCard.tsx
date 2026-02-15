interface Finding {
  id: string
  category: number
  severity: number
  object_ref: number
  attr_num: number
  attr_name: string
  description: string
  current: string
  proposed: string
  effect: string
  fixable: boolean
  fixed: boolean
}

const severityColors: Record<number, string> = {
  0: 'text-red-400 bg-red-500/10 border-red-500/30',      // error
  1: 'text-amber-400 bg-amber-500/10 border-amber-500/30', // warning
  2: 'text-blue-400 bg-blue-500/10 border-blue-500/30',    // info
}

const severityLabels: Record<number, string> = {
  0: 'ERROR',
  1: 'WARNING',
  2: 'INFO',
}

interface FindingCardProps {
  finding: Finding
  onFix?: (id: string) => void
}

export function FindingCard({ finding, onFix }: FindingCardProps) {
  const color = severityColors[finding.severity] || severityColors[2]

  return (
    <div class={`border rounded p-3 mb-2 ${color}`}>
      <div class="flex items-start justify-between gap-2">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-xs font-mono opacity-70">{severityLabels[finding.severity]}</span>
            <span class="text-xs font-mono opacity-50">#{finding.object_ref}</span>
            {finding.attr_name && (
              <span class="text-xs font-mono opacity-50">{finding.attr_name}</span>
            )}
          </div>
          <p class="text-sm">{finding.description}</p>
        </div>
        {finding.fixable && !finding.fixed && onFix && (
          <button
            onClick={() => onFix(finding.id)}
            class="px-2 py-1 bg-green-600 hover:bg-green-500 text-white text-xs rounded shrink-0 transition-colors"
          >
            Fix
          </button>
        )}
        {finding.fixed && (
          <span class="text-xs text-green-400 shrink-0">Fixed</span>
        )}
      </div>
      {finding.current && (
        <div class="mt-2">
          <div class="text-xs opacity-50 mb-0.5">Current:</div>
          <pre class="text-xs font-mono bg-black/20 rounded p-1.5 overflow-x-auto whitespace-pre-wrap break-all">{finding.current}</pre>
        </div>
      )}
      {finding.proposed && (
        <div class="mt-1">
          <div class="text-xs opacity-50 mb-0.5">Proposed:</div>
          <pre class="text-xs font-mono bg-black/20 rounded p-1.5 overflow-x-auto whitespace-pre-wrap break-all">{finding.proposed}</pre>
        </div>
      )}
      {finding.effect && (
        <div class="mt-1 text-xs opacity-70">
          Effect: {finding.effect}
        </div>
      )}
    </div>
  )
}

export type { Finding }
