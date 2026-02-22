import { useEffect, useRef } from "preact/hooks";
import { outputLines } from "../stores/gameStore";
import { parseAnsiLine } from "../lib/ansiParser";

interface ChannelPaneProps {
  channel: string;
}

export function ChannelPane({ channel }: ChannelPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);

  const lines = outputLines.value.filter((l) => l.channel === channel);

  useEffect(() => {
    const el = containerRef.current;
    if (el && autoScroll.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [lines.length]);

  function handleScroll() {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30;
    autoScroll.current = atBottom;
  }

  return (
    <div class="flex flex-col flex-1 min-h-[150px] border-t border-mush-panel">
      <div class="px-2 py-1 text-xs font-bold text-mush-accent bg-mush-surface border-b border-mush-panel">
        #{channel}
        <span class="text-mush-dim font-normal ml-1">({lines.length})</span>
      </div>
      <div
        ref={containerRef}
        onScroll={handleScroll}
        class="flex-1 overflow-y-auto p-2 text-xs leading-relaxed whitespace-pre-wrap break-words"
      >
        {lines.length === 0 ? (
          <div class="text-mush-dim italic">No messages</div>
        ) : (
          lines.map((line) => (
            <div key={line.id}>
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
          ))
        )}
      </div>
    </div>
  );
}
