import { signal } from "@preact/signals";
import { useEffect, useRef } from "preact/hooks";

interface ContextMenuItem {
  label: string;
  action: () => void;
}

interface ContextMenuState {
  x: number;
  y: number;
  items: ContextMenuItem[];
  visible: boolean;
}

const menuState = signal<ContextMenuState>({
  x: 0,
  y: 0,
  items: [],
  visible: false,
});

export function showContextMenu(
  x: number,
  y: number,
  items: ContextMenuItem[],
) {
  menuState.value = { x, y, items, visible: true };
}

export function hideContextMenu() {
  menuState.value = { ...menuState.value, visible: false };
}

export function ContextMenu() {
  const state = menuState.value;
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!state.visible) return;

    const handleClick = (e: MouseEvent) => {
      // Don't dismiss if click is inside the context menu
      if (menuRef.current && menuRef.current.contains(e.target as Node)) return;
      hideContextMenu();
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") hideContextMenu();
    };

    document.addEventListener("click", handleClick, true);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("click", handleClick, true);
      document.removeEventListener("keydown", handleEscape);
    };
  }, [state.visible]);

  if (!state.visible || state.items.length === 0) return null;

  return (
    <div
      ref={menuRef}
      class="context-menu"
      style={{ left: `${state.x}px`, top: `${state.y}px` }}
    >
      {state.items.map((item, i) => (
        <button
          key={i}
          class="context-menu-item"
          onClick={() => {
            item.action();
            hideContextMenu();
          }}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
