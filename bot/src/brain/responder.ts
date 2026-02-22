import type { ParsedEvent, ChannelConfig } from "../types.js";
import type { Config } from "../config.js";
import type { MushClient } from "../mush/client.js";

const SPLIT_LENGTH = 900;
const SPLIT_DELAY_MS = 200;
const KEEPALIVE_INTERVAL_MS = 5 * 60_000; // 5 minutes

export class Responder {
  private readonly channelAliases = new Map<string, string>();
  private keepaliveTimer: ReturnType<typeof setInterval> | null = null;

  constructor(
    private readonly client: MushClient,
    private readonly config: Config
  ) {
    // Build channel name → alias map
    for (const ch of config.bot.channels) {
      this.channelAliases.set(ch.name.toLowerCase(), ch.alias);
    }
  }

  /**
   * Called when WebSocket connects. Joins channels and starts keepalive.
   */
  onConnect(): void {
    // Join configured channels
    for (const ch of this.config.bot.channels) {
      console.log(`[responder] Joining channel: ${ch.name} (alias: ${ch.alias})`);
      this.client.send(`addcom ${ch.alias}=${ch.name}`);
    }

    // Start keepalive
    this.startKeepalive();
  }

  /**
   * Send a response back to the MUSH, split into chunks if needed.
   */
  async respond(event: ParsedEvent, text: string): Promise<void> {
    const parts = splitMessage(text, SPLIT_LENGTH);

    for (let i = 0; i < parts.length; i++) {
      if (i > 0) {
        await sleep(SPLIT_DELAY_MS);
      }
      const part = parts[i]!;
      const cmd = this.formatCommand(event, part);
      this.client.send(cmd);
    }
  }

  stopKeepalive(): void {
    if (this.keepaliveTimer) {
      clearInterval(this.keepaliveTimer);
      this.keepaliveTimer = null;
    }
  }

  private formatCommand(event: ParsedEvent, text: string): string {
    switch (event.kind) {
      case "say":
      case "pose":
      case "whisper":
      case "emit":
        return `say ${text}`;

      case "channel": {
        const alias = event.channel
          ? this.channelAliases.get(event.channel.toLowerCase())
          : undefined;
        if (alias) {
          return `${alias} ${text}`;
        }
        // Fallback: try to use channel name directly
        return `say ${text}`;
      }

      case "page":
        return `page ${event.speaker}=${text}`;

      default:
        return `say ${text}`;
    }
  }

  private startKeepalive(): void {
    this.stopKeepalive();
    this.keepaliveTimer = setInterval(() => {
      this.client.send("think keepalive");
    }, KEEPALIVE_INTERVAL_MS);
  }
}

function splitMessage(text: string, maxLen: number): string[] {
  if (text.length <= maxLen) return [text];

  const parts: string[] = [];
  let remaining = text;

  while (remaining.length > 0) {
    if (remaining.length <= maxLen) {
      parts.push(remaining);
      break;
    }

    // Try to split at a sentence boundary
    let splitIdx = remaining.lastIndexOf(". ", maxLen);
    if (splitIdx < maxLen * 0.5) {
      // Try space
      splitIdx = remaining.lastIndexOf(" ", maxLen);
    }
    if (splitIdx < maxLen * 0.3) {
      // Force split
      splitIdx = maxLen;
    }

    parts.push(remaining.slice(0, splitIdx + 1).trim());
    remaining = remaining.slice(splitIdx + 1).trim();
  }

  return parts;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
