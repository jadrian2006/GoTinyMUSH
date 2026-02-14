import { useEffect, useRef } from "preact/hooks";
import { outputLines } from "../stores/gameStore";

const ANSI_REGEX =
  /\x1b\[([0-9;]*)m/g;

interface AnsiSpan {
  text: string;
  classes: string;
}

function parseAnsi(line: string): AnsiSpan[] {
  const spans: AnsiSpan[] = [];
  let lastIndex = 0;
  let currentClasses = "";
  let match: RegExpExecArray | null;

  ANSI_REGEX.lastIndex = 0;
  while ((match = ANSI_REGEX.exec(line)) !== null) {
    if (match.index > lastIndex) {
      spans.push({ text: line.slice(lastIndex, match.index), classes: currentClasses });
    }
    currentClasses = ansiCodesToClasses(match[1]);
    lastIndex = ANSI_REGEX.lastIndex;
  }
  if (lastIndex < line.length) {
    spans.push({ text: line.slice(lastIndex), classes: currentClasses });
  }
  if (spans.length === 0) {
    spans.push({ text: line, classes: "" });
  }
  return spans;
}

function ansiCodesToClasses(codes: string): string {
  if (!codes) return "";
  const parts = codes.split(";").map(Number);
  const cls: string[] = [];
  for (const code of parts) {
    if (code === 0) return "";
    if (code === 1) cls.push("font-bold");
    if (code === 4) cls.push("underline");
    if (code >= 30 && code <= 37) cls.push(`ansi-fg-${code - 30}`);
    if (code >= 40 && code <= 47) cls.push(`ansi-bg-${code - 40}`);
    if (code >= 90 && code <= 97) cls.push(`ansi-fg-${code - 90 + 8}`);
  }
  return cls.join(" ");
}

function typeClass(type: string): string {
  switch (type) {
    case "system":
      return "text-mush-dim italic";
    case "error":
      return "text-red-400";
    case "channel":
      return "text-cyan-300";
    default:
      return "";
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

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      class="flex-1 overflow-y-auto p-3 text-sm leading-relaxed whitespace-pre-wrap break-words"
    >
      {outputLines.value.map((line) => (
        <div key={line.id} class={typeClass(line.type)}>
          {parseAnsi(line.text).map((span, i) =>
            span.classes ? (
              <span key={i} class={span.classes}>
                {span.text}
              </span>
            ) : (
              span.text
            ),
          )}
        </div>
      ))}
    </div>
  );
}
