import type { Config } from "../config.js";
import type { WSMessage, ParsedEvent, ContextKey } from "../types.js";
import type { AuthResult } from "../mush/auth.js";
import type { LLMProvider } from "../ai/provider.js";
import { parseEvent } from "../mush/protocol.js";
import { TriggerMatcher } from "./trigger.js";
import { ContextManager } from "./context.js";
import { Responder } from "./responder.js";
import { RateLimiter } from "../util/rate-limiter.js";

export class Router {
  private readonly botNameLower: string;
  private readonly botRef: number;
  // Track recent outbound messages for self-loop prevention
  private readonly recentOutbound: string[] = [];
  private static readonly MAX_OUTBOUND_TRACK = 5;

  constructor(
    private readonly config: Config,
    private readonly trigger: TriggerMatcher,
    private readonly context: ContextManager,
    private readonly rateLimiter: RateLimiter,
    private readonly llm: LLMProvider,
    private readonly responder: Responder,
    private readonly auth: AuthResult
  ) {
    this.botNameLower = config.bot.name.toLowerCase();
    this.botRef = auth.playerRef;
  }

  async handleEvent(msg: WSMessage): Promise<void> {
    const event = parseEvent(msg);
    if (!event) return;

    // Self-loop prevention layer 1: compare speaker name
    if (event.speaker.toLowerCase() === this.botNameLower) return;

    // Self-loop prevention layer 2: compare source dbref
    if (event.sourceRef !== undefined && event.sourceRef === this.botRef) return;

    // Self-loop prevention layer 3: check recent outbound
    if (this.isRecentOutbound(event.message)) return;

    // Determine context key
    const ctxKey = this.contextKey(event);
    if (!ctxKey) return;

    // Add to conversation context (even non-triggered messages for context)
    this.context.add(ctxKey, {
      speaker: event.speaker,
      message: event.message,
      isBot: false,
      timestamp: Date.now(),
    });

    // Check if we should respond
    if (!this.trigger.shouldTrigger(event)) return;

    // Rate limit check
    const ctxKeyStr = `${ctxKey.type}:${ctxKey.id}`;
    if (!this.rateLimiter.check(ctxKeyStr)) {
      console.log(`[router] Rate limited: ${ctxKeyStr}`);
      return;
    }

    // Build context-aware prompt
    const contextLabel = this.contextLabel(event);
    const messages = this.context.getMessages(ctxKey);

    // Strip bot name from the triggering message for cleaner LLM input
    const cleanMessage = this.trigger.stripBotName(event.message);
    if (messages.length > 0) {
      const last = messages[messages.length - 1]!;
      if (last.role === "user") {
        last.content = `${event.speaker}: ${cleanMessage}`;
      }
    }

    const systemPrompt = this.buildSystemPrompt(event, contextLabel);

    console.log(
      `[router] Responding to ${event.speaker} in ${ctxKeyStr}: "${cleanMessage}"`
    );

    try {
      const response = await this.llm.complete({
        messages,
        systemPrompt,
        maxTokens: this.config.llm.maxTokens,
      });

      if (!response.content.trim()) return;

      let text = response.content.trim();

      // Truncate if needed
      if (text.length > this.config.maxResponseLength) {
        text = text.slice(0, this.config.maxResponseLength - 3) + "...";
      }

      // Track outbound for self-loop prevention
      this.trackOutbound(text);

      // Record bot response in context
      this.context.add(ctxKey, {
        speaker: this.config.bot.name,
        message: text,
        isBot: true,
        timestamp: Date.now(),
      });

      // Mark rate limit
      this.rateLimiter.mark(ctxKeyStr);

      // Send response
      await this.responder.respond(event, text);

      if (response.usage) {
        console.log(
          `[router] LLM usage: ${response.usage.inputTokens}in/${response.usage.outputTokens}out`
        );
      }
    } catch (err) {
      console.error("[router] LLM error:", err);
    }
  }

  private contextKey(event: ParsedEvent): ContextKey | null {
    switch (event.kind) {
      case "say":
      case "pose":
      case "whisper":
      case "emit":
        return { type: "room", id: event.room ? String(event.room) : "current" };
      case "channel":
        return event.channel
          ? { type: "channel", id: event.channel }
          : null;
      case "page":
        return { type: "page", id: event.speaker };
      default:
        return null;
    }
  }

  private contextLabel(event: ParsedEvent): string {
    switch (event.kind) {
      case "say":
      case "pose":
      case "whisper":
        return "a room in a text-based multiplayer game";
      case "channel":
        return `the "${event.channel}" chat channel`;
      case "page":
        return "a private message";
      default:
        return "a conversation";
    }
  }

  private buildSystemPrompt(event: ParsedEvent, contextLabel: string): string {
    let prompt = this.config.bot.systemPrompt;
    prompt += `\n\nYou are ${this.config.bot.name}. You are in ${contextLabel}.`;
    prompt += `\nThe person talking to you is ${event.speaker}.`;
    prompt += `\nKeep responses concise. Do not use markdown. Plain text only.`;

    if (event.kind === "page") {
      prompt += `\nThis is a private conversation.`;
    }

    return prompt;
  }

  private isRecentOutbound(text: string): boolean {
    return this.recentOutbound.some((msg) => text.includes(msg) || msg.includes(text));
  }

  private trackOutbound(text: string): void {
    this.recentOutbound.push(text);
    if (this.recentOutbound.length > Router.MAX_OUTBOUND_TRACK) {
      this.recentOutbound.shift();
    }
  }
}
