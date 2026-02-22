// WebSocket message from goTinyMUSH server
export interface WSMessage {
  type: string;
  text?: string;
  data?: Record<string, unknown>;
  channel?: string;
  command?: string;
}

// Known event types from server
export type EventKind =
  | "say"
  | "pose"
  | "page"
  | "channel"
  | "whisper"
  | "emit"
  | "connect"
  | "disconnect"
  | "text"
  | "login"
  | "room"
  | "move";

// Parsed/normalized event for brain processing
export interface ParsedEvent {
  kind: EventKind;
  speaker: string;
  message: string;
  raw: string;
  sourceRef?: number;
  room?: number;
  channel?: string;
  target?: string;
}

// Login event data from server
export interface LoginEventData {
  player_ref: number;
  player_name: string;
}

// LLM message for conversation history
export interface LLMMessage {
  role: "user" | "assistant";
  content: string;
}

// LLM provider request
export interface LLMRequest {
  messages: LLMMessage[];
  systemPrompt: string;
  maxTokens: number;
}

// LLM provider response
export interface LLMResponse {
  content: string;
  usage?: {
    inputTokens: number;
    outputTokens: number;
  };
}

// Context tracking
export type ContextType = "room" | "channel" | "page";

export interface ContextKey {
  type: ContextType;
  id: string;
}

export interface ContextEntry {
  speaker: string;
  message: string;
  isBot: boolean;
  timestamp: number;
}

// Channel configuration
export interface ChannelConfig {
  name: string;
  alias: string;
  triggerAll?: boolean;
}
