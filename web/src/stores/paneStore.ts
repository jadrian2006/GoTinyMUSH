import { signal, effect, computed } from "@preact/signals";
import type { PaneConfig, PaneFilter, PaneStyle, OutputLine } from "../types/events";

const STORAGE_KEY = "mush_panes";

const DEFAULT_STYLE: PaneStyle = {
  bgColor: "#1a1a2e",
  ansiEnabled: true,
  fontSize: 14,
  fontFamily: "'Courier New', monospace",
};

const DEFAULT_FILTER: PaneFilter = {
  types: [],
  channels: [],
};

/** Generate a unique ID that works in non-secure contexts (plain HTTP). */
function generateId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    try { return crypto.randomUUID(); } catch { /* non-secure context */ }
  }
  // Fallback: random hex string
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

function makeMainPane(): PaneConfig {
  return {
    id: "main",
    title: "Main",
    filter: { types: [], channels: [] },
    style: { ...DEFAULT_STYLE },
    gridRow: 0,
    gridCol: 0,
    gridRowSpan: 1,
    gridColSpan: 1,
    poppedOut: false,
    minimized: false,
    locked: false,
  };
}

function loadPanes(): PaneConfig[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as PaneConfig[];
      if (Array.isArray(parsed) && parsed.length > 0) {
        // Ensure main pane exists
        if (!parsed.find((p) => p.id === "main")) {
          parsed.unshift(makeMainPane());
        }
        return parsed;
      }
    }
  } catch {
    // ignore corrupt data
  }
  return [makeMainPane()];
}

export const panes = signal<PaneConfig[]>(loadPanes());
export const gridCols = signal<number>(loadGridConfig().cols);
export const rowHeights = signal<number[]>(loadGridConfig().rowHeights);

const GRID_STORAGE_KEY = "mush_grid";

interface GridConfig {
  cols: number;
  rowHeights: number[];
}

function loadGridConfig(): GridConfig {
  try {
    const raw = localStorage.getItem(GRID_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      return { cols: parsed.cols ?? 2, rowHeights: parsed.rowHeights ?? [] };
    }
  } catch { /* ignore */ }
  return { cols: 2, rowHeights: [] };
}

export const gridRows = computed(() => {
  const maxRow = panes.value.reduce(
    (max, p) => Math.max(max, p.gridRow + p.gridRowSpan),
    0,
  );
  return Math.max(1, maxRow);
});

// Auto-persist panes
effect(() => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(panes.value));
});

// Auto-persist grid config
effect(() => {
  localStorage.setItem(GRID_STORAGE_KEY, JSON.stringify({
    cols: gridCols.value,
    rowHeights: rowHeights.value,
  }));
});

function nextGridSlot(): { gridRow: number; gridCol: number } {
  const cols = gridCols.value;
  const occupied = new Set<string>();
  // Track which columns have locked panes (avoid stacking under them)
  const lockedCols = new Set<number>();
  for (const p of panes.value) {
    for (let r = p.gridRow; r < p.gridRow + p.gridRowSpan; r++) {
      for (let c = p.gridCol; c < p.gridCol + p.gridColSpan; c++) {
        occupied.add(`${r},${c}`);
        if (p.locked) lockedCols.add(c);
      }
    }
  }
  // First pass: find empty cell in unlocked columns
  for (let r = 0; r < 100; r++) {
    for (let c = 0; c < cols; c++) {
      if (!occupied.has(`${r},${c}`) && !lockedCols.has(c)) {
        return { gridRow: r, gridCol: c };
      }
    }
  }
  // Fallback: any empty cell (all columns locked or full)
  for (let r = 0; r < 100; r++) {
    for (let c = 0; c < cols; c++) {
      if (!occupied.has(`${r},${c}`)) {
        return { gridRow: r, gridCol: c };
      }
    }
  }
  return { gridRow: 0, gridCol: 0 };
}

export function addPane(partial: Partial<PaneConfig> = {}): PaneConfig {
  const slot = nextGridSlot();
  const pane: PaneConfig = {
    id: generateId(),
    title: partial.title ?? "New Pane",
    filter: partial.filter ?? { ...DEFAULT_FILTER, types: [...DEFAULT_FILTER.types], channels: [...DEFAULT_FILTER.channels] },
    style: partial.style ?? { ...DEFAULT_STYLE },
    gridRow: partial.gridRow ?? slot.gridRow,
    gridCol: partial.gridCol ?? slot.gridCol,
    gridRowSpan: partial.gridRowSpan ?? 1,
    gridColSpan: partial.gridColSpan ?? 1,
    poppedOut: false,
    minimized: false,
    locked: false,
  };
  panes.value = [...panes.value, pane];
  return pane;
}

export function removePane(id: string) {
  if (id === "main") return; // Cannot remove main
  panes.value = panes.value.filter((p) => p.id !== id);
}

export function updatePane(id: string, partial: Partial<PaneConfig>) {
  panes.value = panes.value.map((p) =>
    p.id === id ? { ...p, ...partial } : p,
  );
}

export function setPoppedOut(id: string, val: boolean) {
  updatePane(id, { poppedOut: val });
}

export function organize() {
  const cols = gridCols.value;
  const sorted = panes.value
    .filter((p) => !p.poppedOut)
    .sort((a, b) => {
      // Main always first
      if (a.id === "main") return -1;
      if (b.id === "main") return 1;
      return 0;
    });

  const updated = panes.value.map((p) => {
    if (p.poppedOut) return p;
    const idx = sorted.indexOf(p);
    if (idx === -1) return p;
    const r = Math.floor(idx / cols);
    const c = idx % cols;
    return { ...p, gridRow: r, gridCol: c, gridRowSpan: 1, gridColSpan: 1 };
  });

  panes.value = updated;
}

/**
 * Compute the set of channels and types that are captured by
 * dedicated (non-main) panes, so the main pane can suppress them.
 */
function getSuppressed(): { channels: Set<string>; types: Set<string> } {
  const channels = new Set<string>();
  const types = new Set<string>();
  for (const p of panes.value) {
    if (p.id === "main") continue;
    for (const ch of p.filter.channels) channels.add(ch);
    // Only suppress types if the pane filters *exclusively* by type
    // (no channels), to avoid suppressing broadly
    if (p.filter.channels.length === 0) {
      for (const t of p.filter.types) types.add(t);
    }
  }
  return { channels, types };
}

export function matchesFilter(line: OutputLine, filter: PaneFilter, paneId?: string): boolean {
  const hasTypes = filter.types.length > 0;
  const hasChannels = filter.channels.length > 0;

  // Main pane (no filters) — show everything except what other panes capture
  if (!hasTypes && !hasChannels) {
    if (paneId === "main") {
      const suppressed = getSuppressed();
      // Suppress lines whose channel is captured by a dedicated pane
      if (line.channel && suppressed.channels.has(line.channel)) return false;
      // Suppress lines whose type is captured by a type-only pane
      if (suppressed.types.has(line.type)) return false;
    }
    return true;
  }

  const typeMatch = hasTypes && filter.types.includes(line.type);
  const channelMatch =
    hasChannels && line.channel != null && filter.channels.includes(line.channel);

  // If channels are specified but no types, only show matching channel messages
  if (hasChannels && !hasTypes) return channelMatch;

  // If types are specified but no channels, match by type only
  if (hasTypes && !hasChannels) return typeMatch;

  // Both specified: match either
  return typeMatch || channelMatch;
}

export function swapPanes(idA: string, idB: string) {
  if (idA === idB) return;
  const a = panes.value.find((p) => p.id === idA);
  const b = panes.value.find((p) => p.id === idB);
  if (!a || !b) return;
  // Refuse to move locked panes
  if (a.locked || b.locked) return;
  panes.value = panes.value.map((p) => {
    if (p.id === idA) {
      return { ...p, gridRow: b.gridRow, gridCol: b.gridCol, gridRowSpan: b.gridRowSpan, gridColSpan: b.gridColSpan };
    }
    if (p.id === idB) {
      return { ...p, gridRow: a.gridRow, gridCol: a.gridCol, gridRowSpan: a.gridRowSpan, gridColSpan: a.gridColSpan };
    }
    return p;
  });
}

// Signal for drag-in-progress pane ID (used for drop target highlighting)
export const draggingPaneId = signal<string | null>(null);

export function getPane(id: string): PaneConfig | undefined {
  return panes.value.find((p) => p.id === id);
}
