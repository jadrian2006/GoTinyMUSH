import { useRef, useCallback, useEffect } from "preact/hooks";
import {
  commandHistory,
  historyIndex,
  addCommand,
} from "../stores/gameStore";
import { inputBarHeight } from "../stores/prefsStore";

interface InputBarProps {
  onSubmit: (command: string) => void;
  disabled?: boolean;
}

const MIN_HEIGHT = 40;
const MAX_HEIGHT = 400;

export function InputBar({ onSubmit, disabled }: InputBarProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const dragState = useRef<{ startY: number; startHeight: number } | null>(
    null,
  );

  const height = inputBarHeight.value;

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const ta = textareaRef.current;
      if (!ta) return;

      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        const cmd = ta.value;
        if (cmd.trim()) {
          addCommand(cmd);
          onSubmit(cmd);
        }
        ta.value = "";
        return;
      }

      // Only do history navigation when content is a single line
      const isMultiLine = ta.value.includes("\n");

      if (e.key === "ArrowUp" && !isMultiLine) {
        e.preventDefault();
        const history = commandHistory.value;
        if (history.length === 0) return;
        let idx = historyIndex.value;
        if (idx === -1) {
          idx = history.length - 1;
        } else if (idx > 0) {
          idx--;
        }
        historyIndex.value = idx;
        ta.value = history[idx];
      } else if (e.key === "ArrowDown" && !isMultiLine) {
        e.preventDefault();
        const history = commandHistory.value;
        let idx = historyIndex.value;
        if (idx === -1) return;
        if (idx < history.length - 1) {
          idx++;
          historyIndex.value = idx;
          ta.value = history[idx];
        } else {
          historyIndex.value = -1;
          ta.value = "";
        }
      }
    },
    [onSubmit],
  );

  // Drag resize handlers
  const onDragStart = useCallback(
    (e: MouseEvent | TouchEvent) => {
      e.preventDefault();
      const clientY =
        "touches" in e ? e.touches[0].clientY : (e as MouseEvent).clientY;
      dragState.current = { startY: clientY, startHeight: height };

      const onMove = (ev: MouseEvent | TouchEvent) => {
        if (!dragState.current) return;
        const currentY =
          "touches" in ev
            ? ev.touches[0].clientY
            : (ev as MouseEvent).clientY;
        const delta = dragState.current.startY - currentY;
        const newHeight = Math.min(
          MAX_HEIGHT,
          Math.max(MIN_HEIGHT, dragState.current.startHeight + delta),
        );
        inputBarHeight.value = newHeight;
      };

      const onEnd = () => {
        dragState.current = null;
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onEnd);
        document.removeEventListener("touchmove", onMove);
        document.removeEventListener("touchend", onEnd);
      };

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onEnd);
      document.addEventListener("touchmove", onMove);
      document.addEventListener("touchend", onEnd);
    },
    [height],
  );

  // Auto-focus on mount
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  return (
    <div class="border-t border-mush-panel bg-mush-input">
      {/* Drag handle */}
      <div
        onMouseDown={onDragStart}
        onTouchStart={onDragStart}
        class="h-1 cursor-ns-resize bg-mush-panel hover:bg-mush-accent transition-colors"
      />
      <div class="flex items-start gap-2 p-2" style={{ height: `${height}px` }}>
        <span class="text-mush-accent font-bold text-sm font-mono" style={{ lineHeight: "1.4" }}>&gt;</span>
        <textarea
          ref={textareaRef}
          disabled={disabled}
          onKeyDown={handleKeyDown}
          class="flex-1 bg-transparent text-mush-text outline-none text-sm font-mono placeholder:text-mush-dim resize-none h-full"
          placeholder={disabled ? "Connecting..." : "Enter command..."}
          style={{ lineHeight: "1.4" }}
        />
      </div>
    </div>
  );
}
