import { useEffect, useRef } from "preact/hooks";
import { outputLines } from "../stores/gameStore";
import { panes, matchesFilter } from "../stores/paneStore";
import { parseAnsiLine } from "../lib/ansiParser";

const POPOUT_CSS = `
  body {
    margin: 0;
    padding: 8px;
    font-family: 'Courier New', monospace;
    color: #eaeaea;
    overflow-y: auto;
  }
  .line { white-space: pre-wrap; word-break: break-word; }
  .line-system { color: #6b7280; font-style: italic; }
  .line-error { color: #f87171; }
`;

interface PopoutWindowProps {
  paneId: string;
  onClose: () => void;
}

export function PopoutWindow({ paneId, onClose }: PopoutWindowProps) {
  const winRef = useRef<Window | null>(null);
  const containerRef = useRef<HTMLElement | null>(null);
  const lastLineId = useRef(0);
  const fallback = useRef(false);

  const pane = panes.value.find((p) => p.id === paneId);

  useEffect(() => {
    if (!pane) return;

    // Try to open a popup window
    const popup = window.open(
      "",
      `mush_pane_${paneId}`,
      `width=600,height=400,menubar=no,toolbar=no,status=no`,
    );

    if (!popup || popup.closed) {
      // Popup blocked — use fallback overlay
      fallback.current = true;
      return;
    }

    winRef.current = popup;
    popup.document.title = pane.title;
    popup.document.head.innerHTML = `<style>${POPOUT_CSS}</style>`;
    popup.document.body.style.backgroundColor = pane.style.bgColor;
    popup.document.body.style.fontSize = `${pane.style.fontSize}px`;
    popup.document.body.innerHTML = '<div id="output"></div>';
    containerRef.current = popup.document.getElementById("output");

    // Render existing lines
    const filtered = outputLines.value.filter((l) => matchesFilter(l, pane.filter));
    for (const line of filtered) {
      appendLine(popup.document, containerRef.current!, line, pane.style.ansiEnabled);
      lastLineId.current = line.id;
    }

    // Listen for new lines via BroadcastChannel
    const bc = new BroadcastChannel("mush_output");
    bc.onmessage = (ev) => {
      const line = ev.data;
      if (!line || !containerRef.current) return;
      const currentPane = panes.value.find((p) => p.id === paneId);
      if (!currentPane) return;
      if (matchesFilter(line, currentPane.filter)) {
        appendLine(popup.document, containerRef.current, line, currentPane.style.ansiEnabled);
        // Auto-scroll
        popup.scrollTo(0, popup.document.body.scrollHeight);
      }
    };

    // Detect child window close
    const checkInterval = setInterval(() => {
      if (popup.closed) {
        clearInterval(checkInterval);
        bc.close();
        onClose();
      }
    }, 500);

    return () => {
      clearInterval(checkInterval);
      bc.close();
      if (!popup.closed) popup.close();
    };
  }, [paneId]);

  // Fallback: floating overlay if popup was blocked
  if (fallback.current && pane) {
    const filtered = outputLines.value.filter((l) => matchesFilter(l, pane.filter));
    return (
      <div class="popout-fallback">
        <div class="popout-fallback-header">
          <span class="text-xs font-bold">{pane.title} (popup blocked)</span>
          <button onClick={onClose} class="pane-btn pane-btn-close">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
        <div
          class="popout-fallback-body"
          style={{
            backgroundColor: pane.style.bgColor,
            fontSize: `${pane.style.fontSize}px`,
          }}
        >
          {filtered.map((line) => (
            <div key={line.id} class="whitespace-pre-wrap break-words">
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
          ))}
        </div>
      </div>
    );
  }

  // Normal popout — nothing renders in the main window
  return null;
}

function appendLine(
  doc: Document,
  container: HTMLElement,
  line: { text: string; type: string },
  ansiEnabled: boolean,
) {
  const div = doc.createElement("div");
  div.className = "line";
  if (line.type === "system") div.className += " line-system";
  if (line.type === "error") div.className += " line-error";

  if (ansiEnabled) {
    const spans = parseAnsiLine(line.text);
    for (const span of spans) {
      if (Object.keys(span.style).length > 0) {
        const el = doc.createElement("span");
        Object.assign(el.style, span.style);
        el.textContent = span.text;
        div.appendChild(el);
      } else {
        div.appendChild(doc.createTextNode(span.text));
      }
    }
  } else {
    div.textContent = line.text.replace(/\x1b\[[0-9;]*m/g, "");
  }

  container.appendChild(div);
}
