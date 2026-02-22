import type { ParsedEvent } from "../types.js";

interface BotConfig {
  name: string;
  triggers: string[];
  triggerAllRoom: boolean;
  channels: Array<{ name: string; triggerAll?: boolean }>;
}

export class TriggerMatcher {
  private readonly nameLower: string;
  private readonly customPatterns: RegExp[];
  private readonly triggerAllChannels: Set<string>;

  constructor(private readonly config: BotConfig) {
    this.nameLower = config.name.toLowerCase();
    this.customPatterns = config.triggers.map(
      (p) => new RegExp(p, "i")
    );
    this.triggerAllChannels = new Set(
      config.channels
        .filter((ch) => ch.triggerAll)
        .map((ch) => ch.name.toLowerCase())
    );
  }

  /**
   * Returns true if the bot should respond to this event.
   */
  shouldTrigger(event: ParsedEvent): boolean {
    // Pages always trigger
    if (event.kind === "page") return true;

    // Room speech: check triggerAllRoom or name/pattern match
    if (event.kind === "say" || event.kind === "whisper") {
      if (this.config.triggerAllRoom) return true;
      return this.matchesNameOrPattern(event.message);
    }

    // Pose: only if mentions bot name
    if (event.kind === "pose") {
      return this.matchesNameOrPattern(event.message);
    }

    // Channel: check triggerAll for channel or name/pattern match
    if (event.kind === "channel") {
      if (
        event.channel &&
        this.triggerAllChannels.has(event.channel.toLowerCase())
      ) {
        return true;
      }
      return this.matchesNameOrPattern(event.message);
    }

    return false;
  }

  private matchesNameOrPattern(text: string): boolean {
    const lower = text.toLowerCase();

    // Bot name appears in message (as prefix, @mention, or anywhere)
    if (lower.includes(this.nameLower)) return true;
    if (lower.startsWith(`@${this.nameLower}`)) return true;

    // Custom regex patterns
    for (const re of this.customPatterns) {
      if (re.test(text)) return true;
    }

    return false;
  }

  /**
   * Strip the bot name prefix from a message if present.
   * "BotName, what is 2+2?" → "what is 2+2?"
   */
  stripBotName(text: string): string {
    const lower = text.toLowerCase();
    const name = this.nameLower;

    // "BotName, ..." or "BotName: ..."
    const prefixes = [`${name},`, `${name}:`, `@${name},`, `@${name}:`];
    for (const prefix of prefixes) {
      if (lower.startsWith(prefix)) {
        return text.slice(prefix.length).trim();
      }
    }

    // "Hey BotName, ..." pattern
    const heyPattern = new RegExp(
      `^(?:hey|hi|hello|yo)\\s+${escapeRegex(this.config.name)}[,:]?\\s*`,
      "i"
    );
    const heyMatch = text.match(heyPattern);
    if (heyMatch) {
      return text.slice(heyMatch[0].length).trim();
    }

    return text;
  }
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
