import { useEffect, useCallback } from "preact/hooks";
import { channels, activeChannel, token, outputLines } from "../stores/gameStore";
import * as api from "../services/api";

export function ChannelPanel() {
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
    fetchChannels();
    const interval = setInterval(fetchChannels, 60000);
    return () => clearInterval(interval);
  }, [fetchChannels]);

  const channelList = channels.value;
  const active = activeChannel.value;

  const filteredCount = active
    ? outputLines.value.filter((l) => l.channel === active).length
    : 0;

  return (
    <div class="bg-mush-surface border-t border-mush-panel">
      <div class="flex items-center gap-1 p-1 overflow-x-auto">
        <button
          onClick={() => (activeChannel.value = null)}
          class={`px-2 py-1 text-xs rounded whitespace-nowrap transition-colors ${
            active === null
              ? "bg-mush-accent text-white"
              : "text-mush-dim hover:text-mush-text"
          }`}
        >
          All
        </button>
        {channelList.map((ch) => (
          <button
            key={ch.name}
            onClick={() => (activeChannel.value = ch.name)}
            class={`px-2 py-1 text-xs rounded whitespace-nowrap transition-colors ${
              active === ch.name
                ? "bg-mush-accent text-white"
                : "text-mush-dim hover:text-mush-text"
            }`}
          >
            {ch.name}
            <span class="ml-1 opacity-60">({ch.subscribers})</span>
          </button>
        ))}
      </div>
      {active && (
        <div class="px-2 pb-1 text-xs text-mush-dim">
          Showing {filteredCount} messages on {active}
        </div>
      )}
    </div>
  );
}
