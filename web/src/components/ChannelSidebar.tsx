import { useEffect, useCallback } from "preact/hooks";
import { whoList, channels, token } from "../stores/gameStore";
import { enabledChannels, sidebarOpen } from "../stores/prefsStore";
import { ChannelPane } from "./ChannelPane";
import * as api from "../services/api";

export function ChannelSidebar() {
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
  const enabled = enabledChannels.value;
  const isOpen = sidebarOpen.value;

  if (!isOpen) return null;

  const toggleChannel = (name: string) => {
    if (enabled.includes(name)) {
      enabledChannels.value = enabled.filter((c) => c !== name);
    } else {
      enabledChannels.value = [...enabled, name];
    }
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

        {/* Channel selector */}
        <div class="border-b border-mush-panel p-2">
          <div class="text-xs font-bold text-mush-accent uppercase tracking-wider mb-1">
            Channels
          </div>
          {channelList.length === 0 ? (
            <div class="text-mush-dim text-xs italic">No channels</div>
          ) : (
            <div class="space-y-1">
              {channelList.map((ch) => (
                <label
                  key={ch.name}
                  class="flex items-center gap-2 text-xs text-mush-text cursor-pointer"
                >
                  <input
                    type="checkbox"
                    checked={enabled.includes(ch.name)}
                    onChange={() => toggleChannel(ch.name)}
                    class="accent-mush-accent"
                  />
                  {ch.name}
                  <span class="text-mush-dim">({ch.subscribers})</span>
                </label>
              ))}
            </div>
          )}
        </div>

        {/* Stacked channel panes */}
        <div class="flex-1 flex flex-col min-h-0 overflow-y-auto">
          {enabled.length === 0 ? (
            <div class="p-2 text-xs text-mush-dim italic">
              Enable channels above to see messages
            </div>
          ) : (
            enabled.map((ch) => <ChannelPane key={ch} channel={ch} />)
          )}
        </div>
      </div>
    </>
  );
}
