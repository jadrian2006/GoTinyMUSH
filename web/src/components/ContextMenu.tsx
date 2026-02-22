import { signal } from "@preact/signals";

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

  if (!state.visible || state.items.length === 0) return null;

  return (
    <>
      {/* Invisible backdrop — click to dismiss */}
      <div
        class="fixed inset-0"
        style={{ zIndex: 99 }}
        onClick={() => hideContextMenu()}
        onContextMenu={(e) => { e.preventDefault(); hideContextMenu(); }}
      />
      {/* Menu */}
      <div
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
    </>
  );
}
