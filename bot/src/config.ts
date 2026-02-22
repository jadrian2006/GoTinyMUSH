import { z } from "zod";
import type { ChannelConfig } from "./types.js";

const ChannelConfigSchema = z.object({
  name: z.string(),
  alias: z.string(),
  triggerAll: z.boolean().optional().default(false),
});

const ConfigSchema = z.object({
  mush: z.object({
    wsUrl: z.string(),
    restUrl: z.string(),
    authMode: z.enum(["password", "apikey"]),
    user: z.string().optional(),
    pass: z.string().optional(),
    dbref: z.string().optional(),
    apikey: z.string().optional(),
  }),
  llm: z.object({
    provider: z.enum(["anthropic", "openai"]),
    apiKey: z.string().min(1, "LLM_API_KEY is required"),
    model: z.string(),
    baseUrl: z.string().optional(),
    maxTokens: z.number().int().positive(),
  }),
  bot: z.object({
    name: z.string(),
    systemPrompt: z.string(),
    triggers: z.array(z.string()),
    triggerAllRoom: z.boolean(),
    channels: z.array(ChannelConfigSchema),
  }),
  context: z.object({
    maxMessages: z.number().int().positive(),
    ttlMinutes: z.number().positive(),
  }),
  cooldownMs: z.number().int().nonnegative(),
  maxResponseLength: z.number().int().positive(),
});

export type Config = z.infer<typeof ConfigSchema>;

function parseChannels(raw: string): ChannelConfig[] {
  if (!raw) return [];
  try {
    return JSON.parse(raw) as ChannelConfig[];
  } catch {
    return [];
  }
}

function parseTriggers(raw: string): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

const DEFAULT_SYSTEM_PROMPT = `You are a friendly character in a text-based multiplayer game (MUSH). Keep responses concise (1-3 sentences). Stay in character. Do not use markdown formatting — plain text only. Do not use asterisks for actions; use the third person narrative style of the game world instead.`;

export function loadConfig(): Config {
  const env = process.env;

  const raw = {
    mush: {
      wsUrl: env.MUSH_WS_URL || "ws://localhost:8443/ws",
      restUrl: env.MUSH_REST_URL || "http://localhost:8443",
      authMode: env.AUTH_MODE || "password",
      user: env.MUSH_USER,
      pass: env.MUSH_PASS,
      dbref: env.MUSH_DBREF,
      apikey: env.MUSH_APIKEY,
    },
    llm: {
      provider: env.LLM_PROVIDER || "anthropic",
      apiKey: env.LLM_API_KEY || "",
      model: env.LLM_MODEL || "claude-sonnet-4-20250514",
      baseUrl: env.LLM_BASE_URL || undefined,
      maxTokens: parseInt(env.LLM_MAX_TOKENS || "300", 10),
    },
    bot: {
      name: env.BOT_NAME || "Assistant",
      systemPrompt: env.BOT_SYSTEM_PROMPT || DEFAULT_SYSTEM_PROMPT,
      triggers: parseTriggers(env.BOT_TRIGGERS || ""),
      triggerAllRoom: env.BOT_TRIGGER_ALL_ROOM === "true",
      channels: parseChannels(env.BOT_CHANNELS || ""),
    },
    context: {
      maxMessages: parseInt(env.CONTEXT_MAX_MESSAGES || "20", 10),
      ttlMinutes: parseInt(env.CONTEXT_TTL_MINUTES || "30", 10),
    },
    cooldownMs: parseInt(env.COOLDOWN_MS || "2000", 10),
    maxResponseLength: parseInt(env.MAX_RESPONSE_LENGTH || "900", 10),
  };

  return ConfigSchema.parse(raw);
}
