import { useEffect, useRef, useState } from "preact/hooks";
import { outputLines } from "../stores/gameStore";
import { panes, matchesFilter, removePane, updatePane, setPoppedOut, draggingPaneId } from "../stores/paneStore";
import { layoutMode } from "../stores/prefsStore";
import { parseAnsiLine } from "../lib/ansiParser";
import { PaneSettings } from "./PaneSettings";
import { PopoutWindow } from "./PopoutWindow";

function typeStyle(type: string): Record<string, string> {
  switch (type) {
    case "system":
      return { color: "#6b7280", fontStyle: "italic" };
    case "error":
      return { color: "#f87171" };
    default:
      return {};
  }
}

interface PaneContainerProps {
  paneId: string;
}

export function PaneContainer({ paneId }: PaneContainerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const pane = panes.value.find((p) => p.id === paneId);
  if (!pane) return null;

  const filteredLines = outputLines.value.filter((line) =>
    matchesFilter(line, pane.filter, paneId),
  );

  useEffect(() => {
    const el = containerRef.current;
    if (el && autoScroll.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [filteredLines.length]);

  function handleScroll() {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    autoScroll.current = atBottom;
  }

  // If popped out, render in a popup window instead
  if (pane.poppedOut) {
    return (
      <PopoutWindow
        paneId={paneId}
        onClose={() => setPoppedOut(paneId, false)}
      />
    );
  }

  if (pane.minimized) {
    return (
      <div class="pane-container pane-minimized">
        <div class="pane-titlebar">
          <span class="pane-title">{pane.title}</span>
          <span class="pane-count">{filteredLines.length}</span>
          <div class="pane-actions">
            <button
              onClick={() => updatePane(paneId, { minimized: false })}
              title="Restore"
              class="pane-btn"
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="15 3 21 3 21 9" />
                <polyline points="9 21 3 21 3 15" />
                <line x1="21" y1="3" x2="14" y2="10" />
                <line x1="3" y1="21" x2="10" y2="14" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    );
  }

  const isMain = paneId === "main";
  const isClassic = isMain && layoutMode.value === "classic";

  return (
    <div
      class="pane-container"
      style={{ backgroundColor: pane.style.bgColor }}
    >
      {/* Title bar — draggable unless locked */}
      <div
        class={`pane-titlebar ${!pane.locked ? "pane-titlebar-draggable" : ""}`}
        draggable={!pane.locked}
        onDragStart={(e) => {
          if (pane.locked) { e.preventDefault(); return; }
          const de = e as DragEvent;
          de.dataTransfer!.effectAllowed = "move";
          de.dataTransfer!.setData("text/plain", paneId);
          draggingPaneId.value = paneId;
        }}
        onDragEnd={() => { draggingPaneId.value = null; }}
      >
        {/* Drag grip icon */}
        {!pane.locked && (
          <span class="pane-drag-grip" title="Drag to reorder">
            <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
              <circle cx="8" cy="4" r="2" /><circle cx="16" cy="4" r="2" />
              <circle cx="8" cy="12" r="2" /><circle cx="16" cy="12" r="2" />
              <circle cx="8" cy="20" r="2" /><circle cx="16" cy="20" r="2" />
            </svg>
          </span>
        )}
        <span class="pane-title">{pane.title}</span>
        <span class="pane-count">{filteredLines.length}</span>
        <div class="pane-actions" draggable={false}>
          {/* Lock/unlock */}
          <button
            draggable={false}
            onClick={() => updatePane(paneId, { locked: !pane.locked })}
            title={pane.locked ? "Unlock position" : "Lock position"}
            class={`pane-btn ${pane.locked ? "pane-btn-locked" : ""}`}
          >
            {pane.locked ? (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
            ) : (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0 1 9.9-1" />
              </svg>
            )}
          </button>
          <button
            draggable={false}
            onClick={() => setSettingsOpen(!settingsOpen)}
            title="Settings"
            class="pane-btn"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
          </button>
          <button
            draggable={false}
            onClick={() => updatePane(paneId, { minimized: true })}
            title="Minimize"
            class="pane-btn"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          </button>
          <button
            draggable={false}
            onClick={() => setPoppedOut(paneId, true)}
            title="Pop out"
            class="pane-btn"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
              <polyline points="15 3 21 3 21 9" />
              <line x1="10" y1="14" x2="21" y2="3" />
            </svg>
          </button>
          {!isMain && (
            <button
              draggable={false}
              onClick={() => removePane(paneId)}
              title="Close"
              class="pane-btn pane-btn-close"
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* Settings dropdown */}
      {settingsOpen && (
        <PaneSettings paneId={paneId} onClose={() => setSettingsOpen(false)} />
      )}

      {/* Output area */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        class="pane-output"
        style={{ fontSize: `${pane.style.fontSize}px`, fontFamily: pane.style.fontFamily }}
      >
        <div
          style={
            isClassic
              ? { width: "80ch", maxWidth: "100%", margin: "0 auto" }
              : { width: "100%" }
          }
          class="whitespace-pre-wrap break-words"
        >
          {filteredLines.length === 0 ? (
            <div class="text-mush-dim italic text-xs">No messages</div>
          ) : (
            filteredLines.map((line) => (
              <div key={line.id} style={typeStyle(line.type)}>
                {pane.style.ansiEnabled
                  ? parseAnsiLine(line.text).map((span, i) => {
                      const hasStyle = Object.keys(span.style).length > 0;
                      return hasStyle ? (
                        <span key={i} style={span.style}>
                          {span.text}
                        </span>
                      ) : (
                        span.text
                      );
                    })
                  : line.text.replace(/\x1b\[[0-9;]*m/g, "")}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
