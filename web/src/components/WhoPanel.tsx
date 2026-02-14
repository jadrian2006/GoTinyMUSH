import { useEffect, useCallback } from "preact/hooks";
import { whoList, token } from "../stores/gameStore";
import * as api from "../services/api";

export function WhoPanel() {
  const fetchWho = useCallback(async () => {
    try {
      const res = await api.getWho(token.value ?? undefined);
      whoList.value = res.players ?? [];
    } catch {
      // silently ignore
    }
  }, []);

  useEffect(() => {
    fetchWho();
    const interval = setInterval(fetchWho, 30000);
    return () => clearInterval(interval);
  }, [fetchWho]);

  const players = whoList.value;

  return (
    <div class="bg-mush-surface border-l border-mush-panel w-56 flex flex-col">
      <div class="p-2 border-b border-mush-panel text-xs font-bold text-mush-accent uppercase tracking-wider">
        Online ({players.length})
      </div>
      <div class="flex-1 overflow-y-auto p-2 space-y-1">
        {players.length === 0 ? (
          <div class="text-mush-dim text-xs italic">No players online</div>
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
  );
}
