import { useEffect, useCallback, useState } from "preact/hooks";
import { whoList, channels, token } from "../stores/gameStore";
import { sidebarOpen } from "../stores/prefsStore";
import {
  panes,
  addPane,
  removePane,
  updatePane,
  setPoppedOut,
  organize,
  gridCols,
} from "../stores/paneStore";
import { showContextMenu } from "./ContextMenu";
import * as api from "../services/api";

export function PaneManagerDrawer() {
  const [newPaneOpen, setNewPaneOpen] = useState(false);
  const [newTitle, setNewTitle] = useState("");

  // Fetch WHO list
  const fetchWho = useCallback(async () => {
    try {
      const res = await api.getWho(token.value ?? undefined);
      whoList.value = res.players ?? [];
    } catch {
      // silently ignore
    }
  }, []);

  // Fetch channel list
  const fetchChannels = useCallback(async () => {
    const t = token.value;
    if (!t) return;
    try {
      const res = await api.getChannels(t);
      channels.value = res.channels ?? [];
    } catch {
      // silently ignore
    }
  }, []);

  useEffect(() => {
    fetchWho();
    fetchChannels();
    const whoInterval = setInterval(fetchWho, 30000);
    const chInterval = setInterval(fetchChannels, 60000);
    return () => {
      clearInterval(whoInterval);
      clearInterval(chInterval);
    };
  }, [fetchWho, fetchChannels]);

  const players = whoList.value;
  const channelList = channels.value;
  const isOpen = sidebarOpen.value;
  const allPanes = panes.value;

  if (!isOpen) return null;

  const handleNewPane = () => {
    const title = newTitle.trim() || "New Pane";
    addPane({ title });
    setNewTitle("");
    setNewPaneOpen(false);
  };

  const handleChannelContext = (e: MouseEvent, channelName: string) => {
    e.preventDefault();
    showContextMenu(e.clientX, e.clientY, [
      {
        label: "Open in new pane",
        action: () => {
          addPane({
            title: `#${channelName}`,
            filter: { types: [], channels: [channelName] },
          });
        },
      },
    ]);
  };

  return (
    <>
      {/* Mobile backdrop */}
      <div
        class="sidebar-backdrop"
        onClick={() => (sidebarOpen.value = false)}
      />

      <div class="sidebar-panel bg-mush-surface border-l border-mush-panel flex flex-col">
        {/* WHO section */}
        <div class="border-b border-mush-panel">
          <div class="p-2 text-xs font-bold text-mush-accent uppercase tracking-wider">
            Online ({players.length})
          </div>
          <div class="max-h-40 overflow-y-auto px-2 pb-2 space-y-1">
            {players.length === 0 ? (
              <div class="text-mush-dim text-xs italic">
                No players online
              </div>
            ) : (
              players.map((p) => (
                <div
                  key={p.ref}
                  class="text-xs flex justify-between items-baseline"
                >
                  <span class="text-mush-text truncate">{p.name}</span>
                  <span class="text-mush-dim ml-1 shrink-0">{p.idle}</span>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Channel list */}
        <div class="border-b border-mush-panel p-2">
          <div class="text-xs font-bold text-mush-accent uppercase tracking-wider mb-1">
            Channels
          </div>
          {channelList.length === 0 ? (
            <div class="text-mush-dim text-xs italic">No channel aliases</div>
          ) : (
            <div class="space-y-1">
              {channelList.map((ch) => (
                <div
                  key={ch.name}
                  class="flex items-center gap-2 text-xs text-mush-text cursor-pointer hover:text-mush-accent"
                  onContextMenu={(e) => handleChannelContext(e as MouseEvent, ch.name)}
                  title="Right-click to open in pane"
                >
                  <span class={ch.listening ? "" : "opacity-50"}>
                    {ch.name}
                    {ch.alias && ch.alias !== ch.name && (
                      <span class="text-mush-dim ml-1">[{ch.alias}]</span>
                    )}
                  </span>
                  {!ch.listening && (
                    <span class="text-mush-dim italic">off</span>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Panes section */}
        <div class="flex-1 flex flex-col min-h-0 overflow-y-auto">
          <div class="p-2">
            <div class="text-xs font-bold text-mush-accent uppercase tracking-wider mb-2">
              Panes
            </div>

            {/* Pane list */}
            <div class="space-y-1 mb-2">
              {allPanes.map((p) => (
                <div
                  key={p.id}
                  class="flex items-center justify-between text-xs text-mush-text cursor-pointer hover:text-mush-accent"
                  onContextMenu={(e) => {
                    e.preventDefault();
                    const items = [];
                    if (p.minimized) {
                      items.push({ label: "Restore", action: () => updatePane(p.id, { minimized: false }) });
                    } else {
                      items.push({ label: "Minimize", action: () => updatePane(p.id, { minimized: true }) });
                    }
                    if (p.poppedOut) {
                      items.push({ label: "Return to grid", action: () => setPoppedOut(p.id, false) });
                    } else {
                      items.push({ label: "Pop out", action: () => setPoppedOut(p.id, true) });
                    }
                    items.push({
                      label: p.locked ? "Unlock" : "Lock",
                      action: () => updatePane(p.id, { locked: !p.locked }),
                    });
                    if (p.id !== "main") {
                      items.push({
                        label: "Duplicate",
                        action: () => addPane({ title: p.title + " (copy)", filter: { ...p.filter }, style: { ...p.style } }),
                      });
                      items.push({ label: "Remove", action: () => removePane(p.id) });
                    }
                    showContextMenu((e as MouseEvent).clientX, (e as MouseEvent).clientY, items);
                  }}
                  title="Right-click for options"
                >
                  <span class="truncate">
                    {p.title}
                    {p.locked && (
                      <span class="text-mush-accent ml-1">&#128274;</span>
                    )}
                    {p.poppedOut && (
                      <span class="text-mush-dim ml-1">(popout)</span>
                    )}
                    {p.minimized && (
                      <span class="text-mush-dim ml-1">(min)</span>
                    )}
                  </span>
                  {p.id !== "main" && (
                    <button
                      onClick={() => removePane(p.id)}
                      class="text-mush-dim hover:text-red-400 ml-1"
                      title="Remove pane"
                    >
                      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <line x1="18" y1="6" x2="6" y2="18" />
                        <line x1="6" y1="6" x2="18" y2="18" />
                      </svg>
                    </button>
                  )}
                </div>
              ))}
            </div>

            {/* New Pane */}
            {newPaneOpen ? (
              <div class="mb-2">
                <input
                  type="text"
                  value={newTitle}
                  onInput={(e) => setNewTitle((e.target as HTMLInputElement).value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleNewPane();
                    if (e.key === "Escape") setNewPaneOpen(false);
                  }}
                  placeholder="Pane title..."
                  class="w-full bg-mush-input border border-mush-panel rounded px-2 py-1 text-xs text-mush-text outline-none focus:border-mush-accent mb-1"
                  autofocus
                />
                <div class="flex gap-1">
                  <button
                    onClick={handleNewPane}
                    class="flex-1 bg-mush-accent text-white text-xs py-1 rounded hover:bg-red-500 transition-colors"
                  >
                    Create
                  </button>
                  <button
                    onClick={() => setNewPaneOpen(false)}
                    class="flex-1 bg-mush-panel text-mush-text text-xs py-1 rounded hover:bg-mush-input transition-colors"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <button
                onClick={() => setNewPaneOpen(true)}
                class="w-full bg-mush-panel hover:bg-mush-input text-mush-text text-xs py-1 rounded transition-colors mb-2"
              >
                + New Pane
              </button>
            )}

            {/* Organize */}
            <button
              onClick={organize}
              class="w-full bg-mush-panel hover:bg-mush-input text-mush-text text-xs py-1 rounded transition-colors mb-2"
            >
              Organize
            </button>

            {/* Grid columns slider */}
            <div>
              <label class="text-xs text-mush-dim">
                Grid Columns: {gridCols.value}
              </label>
              <input
                type="range"
                min="1"
                max="4"
                value={gridCols.value}
                onInput={(e) => {
                  gridCols.value = parseInt((e.target as HTMLInputElement).value, 10);
                }}
                class="w-full accent-mush-accent"
              />
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
