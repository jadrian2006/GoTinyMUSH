import { useEffect, useRef } from "preact/hooks";
import { outputLines } from "../stores/gameStore";
import { layoutMode } from "../stores/prefsStore";
import { parseAnsiLine } from "../lib/ansiParser";

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

export function Terminal() {
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);

  useEffect(() => {
    const el = containerRef.current;
    if (el && autoScroll.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [outputLines.value]);

  function handleScroll() {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    autoScroll.current = atBottom;
  }

  const isClassic = layoutMode.value === "classic";

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      class="flex-1 overflow-y-auto p-3 text-sm leading-relaxed"
    >
      <div
        style={
          isClassic
            ? { width: "80ch", maxWidth: "100%", margin: "0 auto" }
            : { width: "100%" }
        }
        class="whitespace-pre-wrap break-words"
      >
        {outputLines.value.map((line) => (
          <div key={line.id} style={typeStyle(line.type)}>
            {parseAnsiLine(line.text).map((span, i) => {
              const hasStyle = Object.keys(span.style).length > 0;
              return hasStyle ? (
                <span key={i} style={span.style}>
                  {span.text}
                </span>
              ) : (
                span.text
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
