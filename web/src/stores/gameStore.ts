import { signal, computed } from "@preact/signals";
import type { OutputLine, WhoEntry, ChannelInfo } from "../types/events";

let nextLineId = 1;

export const token = signal<string | null>(
  localStorage.getItem("mush_token"),
);
export const playerName = signal<string | null>(null);
export const playerRef = signal<number | null>(null);
export const connected = signal(false);

export const outputLines = signal<OutputLine[]>([]);
export const commandHistory = signal<string[]>([]);
export const historyIndex = signal(-1);

export const whoList = signal<WhoEntry[]>([]);
export const channels = signal<ChannelInfo[]>([]);
export const activeChannel = signal<string | null>(null);

export const isLoggedIn = computed(() => token.value !== null);

export function addOutput(text: string, type = "text", channel?: string) {
  const line: OutputLine = {
    id: nextLineId++,
    text,
    type,
    channel,
    timestamp: Date.now(),
  };
  outputLines.value = [...outputLines.value.slice(-2000), line];
}

export function addCommand(cmd: string) {
  commandHistory.value = [...commandHistory.value.slice(-500), cmd];
  historyIndex.value = -1;
}

export function setAuth(t: string, name: string, ref: number) {
  token.value = t;
  playerName.value = name;
  playerRef.value = ref;
  localStorage.setItem("mush_token", t);
}

export function clearAuth() {
  token.value = null;
  playerName.value = null;
  playerRef.value = null;
  localStorage.removeItem("mush_token");
}
