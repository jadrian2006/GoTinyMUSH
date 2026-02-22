import type { ContextEntry, ContextKey, LLMMessage } from "../types.js";

interface ContextConfig {
  maxMessages: number;
  ttlMinutes: number;
}

interface ContextWindow {
  entries: ContextEntry[];
  lastAccess: number;
}

export class ContextManager {
  private windows = new Map<string, ContextWindow>();

  constructor(private readonly config: ContextConfig) {}

  private keyStr(key: ContextKey): string {
    return `${key.type}:${key.id}`;
  }

  add(key: ContextKey, entry: ContextEntry): void {
    const k = this.keyStr(key);
    let win = this.windows.get(k);
    if (!win) {
      win = { entries: [], lastAccess: Date.now() };
      this.windows.set(k, win);
    }

    win.entries.push(entry);
    win.lastAccess = Date.now();

    // Trim to max size
    if (win.entries.length > this.config.maxMessages) {
      win.entries = win.entries.slice(-this.config.maxMessages);
    }
  }

  getMessages(key: ContextKey): LLMMessage[] {
    const k = this.keyStr(key);
    const win = this.windows.get(k);
    if (!win) return [];

    // Check TTL
    const ttlMs = this.config.ttlMinutes * 60_000;
    if (Date.now() - win.lastAccess > ttlMs) {
      this.windows.delete(k);
      return [];
    }

    win.lastAccess = Date.now();

    return win.entries.map((e) => ({
      role: e.isBot ? ("assistant" as const) : ("user" as const),
      content: e.isBot ? e.message : `${e.speaker}: ${e.message}`,
    }));
  }

  /**
   * Periodically clean up expired contexts.
   */
  cleanup(): void {
    const ttlMs = this.config.ttlMinutes * 60_000;
    const now = Date.now();
    for (const [k, win] of this.windows) {
      if (now - win.lastAccess > ttlMs) {
        this.windows.delete(k);
      }
    }
  }
}
