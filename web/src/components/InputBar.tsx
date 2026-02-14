import { useRef, useCallback } from "preact/hooks";
import {
  commandHistory,
  historyIndex,
  addCommand,
} from "../stores/gameStore";

interface InputBarProps {
  onSubmit: (command: string) => void;
  disabled?: boolean;
}

export function InputBar({ onSubmit, disabled }: InputBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Enter") {
        const input = inputRef.current;
        if (!input) return;
        const cmd = input.value;
        if (cmd.trim()) {
          addCommand(cmd);
          onSubmit(cmd);
        }
        input.value = "";
        e.preventDefault();
      } else if (e.key === "ArrowUp") {
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
        if (inputRef.current) {
          inputRef.current.value = history[idx];
        }
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        const history = commandHistory.value;
        let idx = historyIndex.value;
        if (idx === -1) return;
        if (idx < history.length - 1) {
          idx++;
          historyIndex.value = idx;
          if (inputRef.current) {
            inputRef.current.value = history[idx];
          }
        } else {
          historyIndex.value = -1;
          if (inputRef.current) {
            inputRef.current.value = "";
          }
        }
      }
    },
    [onSubmit],
  );

  return (
    <div class="flex items-center gap-2 border-t border-mush-panel p-2 bg-mush-input">
      <span class="text-mush-accent font-bold">&gt;</span>
      <input
        ref={inputRef}
        type="text"
        disabled={disabled}
        onKeyDown={handleKeyDown}
        class="flex-1 bg-transparent text-mush-text outline-none text-sm font-mono placeholder:text-mush-dim"
        placeholder={disabled ? "Connecting..." : "Enter command..."}
        autofocus
      />
    </div>
  );
}
