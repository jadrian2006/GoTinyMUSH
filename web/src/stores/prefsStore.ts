import { signal, effect } from "@preact/signals";

const STORAGE_KEY = "mush_prefs";

export type LayoutMode = "classic" | "widescreen";

interface StoredPrefs {
  layoutMode: LayoutMode;
  inputBarHeight: number;
  sidebarOpen: boolean;
}

const DEFAULTS: StoredPrefs = {
  layoutMode: "classic",
  inputBarHeight: 40,
  sidebarOpen: false,
};

function loadPrefs(): StoredPrefs {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      return { ...DEFAULTS, ...parsed };
    }
  } catch {
    // ignore corrupt data
  }
  return { ...DEFAULTS };
}

const saved = loadPrefs();

export const layoutMode = signal<LayoutMode>(saved.layoutMode);
export const inputBarHeight = signal<number>(saved.inputBarHeight);
export const sidebarOpen = signal<boolean>(saved.sidebarOpen);

// Auto-persist whenever any pref changes
effect(() => {
  const prefs: StoredPrefs = {
    layoutMode: layoutMode.value,
    inputBarHeight: inputBarHeight.value,
    sidebarOpen: sidebarOpen.value,
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
});
