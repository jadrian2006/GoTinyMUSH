import { useEffect, useRef, useState, useCallback } from "preact/hooks";
import { outputLines, loginScrollTrigger } from "../stores/gameStore";
import { panes, matchesFilter, removePane, updatePane, setPoppedOut, draggingPaneId } from "../stores/paneStore";
import { layoutMode } from "../stores/prefsStore";
import { parseAnsiLine } from "../lib/ansiParser";
import { PaneSettings } from "./PaneSettings";
import { PopoutWindow } from "./PopoutWindow";
import { showContextMenu } from "./ContextMenu";
import type { OutputLine } from "../types/events";

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

function renderLine(line: OutputLine, ansiEnabled: boolean) {
  return (
    <div key={line.id} data-line-id={line.id} style={typeStyle(line.type)}>
      {ansiEnabled
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
  );
}

interface SplitState {
  splitLineId: number;
  topHeightPercent: number;
}

interface PaneContainerProps {
  paneId: string;
}

export function PaneContainer({ paneId }: PaneContainerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [paused, setPaused] = useState(false);
  const [newLineCount, setNewLineCount] = useState(0);
  const [split, setSplit] = useState<SplitState | null>(null);
  const lastLineCountRef = useRef(0);

  const pane = panes.value.find((p) => p.id === paneId);
  if (!pane) return null;

  const filteredLines = outputLines.value.filter((line) =>
    matchesFilter(line, pane.filter, paneId),
  );

  // Track new lines when paused
  useEffect(() => {
    if (paused) {
      const diff = filteredLines.length - lastLineCountRef.current;
      if (diff > 0) {
        setNewLineCount((c) => c + diff);
      }
    }
    lastLineCountRef.current = filteredLines.length;
  }, [filteredLines.length, paused]);

  // Auto-scroll effect
  useEffect(() => {
    if (paused) return;
    if (split) {
      const el = bottomRef.current;
      if (el && autoScroll.current) {
        el.scrollTop = el.scrollHeight;
      }
    } else {
      const el = containerRef.current;
      if (el && autoScroll.current) {
        el.scrollTop = el.scrollHeight;
      }
    }
  }, [filteredLines.length, paused, split]);

  // Login scroll trigger — reset everything and scroll to bottom
  useEffect(() => {
    if (loginScrollTrigger.value === 0) return;
    autoScroll.current = true;
    setPaused(false);
    setNewLineCount(0);
    setSplit(null);
    requestAnimationFrame(() => {
      const el = split ? bottomRef.current : containerRef.current;
      if (el) el.scrollTop = el.scrollHeight;
    });
  }, [loginScrollTrigger.value]);

  // Auto-remove split if top pane becomes empty
  useEffect(() => {
    if (!split) return;
    const topLines = filteredLines.filter((l) => l.id <= split.splitLineId);
    if (topLines.length === 0) {
      setSplit(null);
    }
  }, [filteredLines, split]);

  function handleScroll() {
    if (paused) return;
    const el = split ? bottomRef.current : containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    autoScroll.current = atBottom;
  }

  function togglePause() {
    if (paused) {
      // Unpause — scroll to bottom
      setPaused(false);
      setNewLineCount(0);
      autoScroll.current = true;
      requestAnimationFrame(() => {
        const el = split ? bottomRef.current : containerRef.current;
        if (el) el.scrollTop = el.scrollHeight;
      });
    } else {
      setPaused(true);
      setNewLineCount(0);
    }
  }

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    if (split) {
      showContextMenu(e.clientX, e.clientY, [
        { label: "Remove split", action: () => setSplit(null) },
      ]);
      return;
    }
    // Walk up from click target to find nearest data-line-id
    let target = e.target as HTMLElement | null;
    let lineId: number | null = null;
    while (target) {
      const attr = target.getAttribute?.("data-line-id");
      if (attr) {
        lineId = parseInt(attr, 10);
        break;
      }
      // Stop at the pane-output container
      if (target === containerRef.current) break;
      target = target.parentElement;
    }
    if (lineId != null) {
      showContextMenu(e.clientX, e.clientY, [
        {
          label: "Split here",
          action: () => setSplit({ splitLineId: lineId!, topHeightPercent: 50 }),
        },
      ]);
    }
  }

  // Divider drag logic
  const handleDividerMouseDown = useCallback(
    (e: MouseEvent) => {
      e.preventDefault();
      const wrapper = (e.target as HTMLElement).closest(".pane-split-wrapper") as HTMLElement | null;
      if (!wrapper) return;
      const startY = e.clientY;
      const startPct = split?.topHeightPercent ?? 50;
      const wrapperHeight = wrapper.getBoundingClientRect().height;

      function onMouseMove(ev: MouseEvent) {
        const delta = ev.clientY - startY;
        const deltaPct = (delta / wrapperHeight) * 100;
        const newPct = Math.min(90, Math.max(10, startPct + deltaPct));
        setSplit((prev) => prev ? { ...prev, topHeightPercent: newPct } : prev);
      }

      function onMouseUp() {
        document.removeEventListener("mousemove", onMouseMove);
        document.removeEventListener("mouseup", onMouseUp);
      }

      document.addEventListener("mousemove", onMouseMove);
      document.addEventListener("mouseup", onMouseUp);
    },
    [split],
  );

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

  const innerStyle = isClassic
    ? { width: "80ch", maxWidth: "100%", margin: "0 auto" }
    : { width: "100%" };

  const outputStyle = { fontSize: `${pane.style.fontSize}px`, fontFamily: pane.style.fontFamily };
  const ansiEnabled = pane.style.ansiEnabled;

  // Split mode rendering
  function renderSplitOutput() {
    if (!split) return null;
    const topLines = filteredLines.filter((l) => l.id <= split.splitLineId);
    const bottomLines = filteredLines.filter((l) => l.id > split.splitLineId);

    return (
      <div class="pane-split-wrapper">
        {/* Top: frozen history */}
        <div
          class="pane-output"
          style={{ ...outputStyle, height: `${split.topHeightPercent}%`, flex: "none" }}
        >
          <div style={innerStyle} class="whitespace-pre-wrap break-words">
            {topLines.length === 0 ? (
              <div class="text-mush-dim italic text-xs">No messages</div>
            ) : (
              topLines.map((line) => renderLine(line, ansiEnabled))
            )}
          </div>
        </div>
        {/* Divider */}
        <div class="pane-split-divider" onMouseDown={handleDividerMouseDown}>
          <button
            class="pane-split-divider-btn"
            onClick={(e) => { e.stopPropagation(); setSplit(null); }}
            title="Remove split"
          >
            ✕
          </button>
        </div>
        {/* Bottom: live output */}
        <div
          ref={bottomRef}
          onScroll={handleScroll}
          class="pane-output"
          style={{ ...outputStyle, flex: 1 }}
        >
          <div style={innerStyle} class="whitespace-pre-wrap break-words">
            {bottomLines.length === 0 ? (
              <div class="text-mush-dim italic text-xs">No messages</div>
            ) : (
              bottomLines.map((line) => renderLine(line, ansiEnabled))
            )}
          </div>
        </div>
      </div>
    );
  }

  // Normal (non-split) rendering
  function renderNormalOutput() {
    return (
      <div
        ref={containerRef}
        onScroll={handleScroll}
        onContextMenu={handleContextMenu}
        class="pane-output"
        style={outputStyle}
      >
        <div style={innerStyle} class="whitespace-pre-wrap break-words">
          {filteredLines.length === 0 ? (
            <div class="text-mush-dim italic text-xs">No messages</div>
          ) : (
            filteredLines.map((line) => renderLine(line, ansiEnabled))
          )}
        </div>
      </div>
    );
  }

  return (
    <div
      class="pane-container"
      style={{ backgroundColor: pane.style.bgColor }}
      onContextMenu={split ? handleContextMenu : undefined}
    >
      {/* Title bar */}
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
          {/* Pause/Play */}
          <button
            draggable={false}
            onClick={togglePause}
            title={paused ? "Resume auto-scroll" : "Pause auto-scroll"}
            class={`pane-btn ${paused ? "pane-btn-pause-active" : ""}`}
          >
            {paused ? (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polygon points="5 3 19 12 5 21 5 3" />
              </svg>
            ) : (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="6" y="4" width="4" height="16" />
                <rect x="14" y="4" width="4" height="16" />
              </svg>
            )}
          </button>
          {paused && newLineCount > 0 && (
            <span class="pane-pause-badge">{newLineCount} new</span>
          )}
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
      {split ? renderSplitOutput() : renderNormalOutput()}
    </div>
  );
}
