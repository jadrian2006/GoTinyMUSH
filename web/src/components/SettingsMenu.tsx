import { useState } from "preact/hooks";
import { layoutMode } from "../stores/prefsStore";
import type { LayoutMode } from "../stores/prefsStore";

export function SettingsMenu() {
  const [open, setOpen] = useState(false);

  const current = layoutMode.value;

  const toggle = (mode: LayoutMode) => {
    layoutMode.value = mode;
  };

  return (
    <div class="relative">
      <button
        onClick={() => setOpen(!open)}
        class="text-mush-dim hover:text-mush-accent transition-colors"
        title="Settings"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
      </button>

      {open && (
        <>
          {/* Backdrop to close menu */}
          <div
            class="fixed inset-0 z-40"
            onClick={() => setOpen(false)}
          />
          <div class="absolute right-0 top-6 z-50 bg-mush-surface border border-mush-panel rounded shadow-lg p-2 min-w-[160px]">
            <div class="text-xs text-mush-dim uppercase tracking-wider mb-2">
              Layout
            </div>
            <label class="flex items-center gap-2 text-xs text-mush-text cursor-pointer py-1">
              <input
                type="radio"
                name="layout"
                checked={current === "classic"}
                onChange={() => toggle("classic")}
                class="accent-mush-accent"
              />
              Classic (80 col)
            </label>
            <label class="flex items-center gap-2 text-xs text-mush-text cursor-pointer py-1">
              <input
                type="radio"
                name="layout"
                checked={current === "widescreen"}
                onChange={() => toggle("widescreen")}
                class="accent-mush-accent"
              />
              Widescreen
            </label>
          </div>
        </>
      )}
    </div>
  );
}
