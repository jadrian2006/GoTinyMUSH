import { FindingCard, type Finding } from './FindingCard'

const categoryNames: Record<number, string> = {
  0: 'Double-Escaped Brackets',
  1: 'Attribute Flag Anomalies',
  2: 'Escape Sequences',
  3: 'Backslash-Percent Patterns',
  4: 'Integrity Errors',
  5: 'Integrity Warnings',
}

const categoryKeys: Record<number, string> = {
  0: 'double-escape',
  1: 'attr-flags',
  2: 'escape-seq',
  3: 'percent',
  4: 'integrity-error',
  5: 'integrity-warning',
}

interface FindingListProps {
  findings: Finding[]
  onFix?: (id: string) => void
  onFixAll?: (category: string) => void
}

export function FindingList({ findings, onFix, onFixAll }: FindingListProps) {
  // Group by category
  const groups = new Map<number, Finding[]>()
  for (const f of findings) {
    const existing = groups.get(f.category) || []
    existing.push(f)
    groups.set(f.category, existing)
  }

  const sortedCategories = Array.from(groups.keys()).sort()

  return (
    <div>
      {sortedCategories.map(cat => {
        const items = groups.get(cat)!
        const fixable = items.filter(f => f.fixable && !f.fixed)
        const fixed = items.filter(f => f.fixed)

        return (
          <div key={cat} class="mb-6">
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-lg font-semibold text-slate-200">
                {categoryNames[cat] || `Category ${cat}`}
                <span class="text-sm font-normal text-slate-400 ml-2">
                  ({items.length} total{fixed.length > 0 ? `, ${fixed.length} fixed` : ''})
                </span>
              </h3>
              {fixable.length > 0 && onFixAll && (
                <button
                  onClick={() => onFixAll(categoryKeys[cat])}
                  class="px-3 py-1 bg-green-600 hover:bg-green-500 text-white text-xs rounded transition-colors"
                >
                  Fix All ({fixable.length})
                </button>
              )}
            </div>
            {items.map(f => (
              <FindingCard key={f.id} finding={f} onFix={onFix} />
            ))}
          </div>
        )
      })}
    </div>
  )
}
