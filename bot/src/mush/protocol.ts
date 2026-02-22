import type { WSMessage, ParsedEvent, EventKind } from "../types.js";

const KNOWN_KINDS = new Set<string>([
  "say",
  "pose",
  "page",
  "channel",
  "whisper",
  "emit",
  "connect",
  "disconnect",
  "text",
  "login",
  "room",
  "move",
]);

/**
 * Parse a raw WSMessage into a normalized ParsedEvent.
 * Returns null for events we don't care about (room, move, connect, etc.)
 */
export function parseEvent(msg: WSMessage): ParsedEvent | null {
  const kind = KNOWN_KINDS.has(msg.type)
    ? (msg.type as EventKind)
    : "text";
  const data = msg.data || {};

  switch (kind) {
    case "say":
      return {
        kind: "say",
        speaker: str(data.speaker),
        message: str(data.message),
        raw: msg.text || "",
        sourceRef: num(data.source_ref),
      };

    case "pose":
      return {
        kind: "pose",
        speaker: str(data.player),
        message: str(data.pose),
        raw: msg.text || "",
        sourceRef: num(data.source_ref),
      };

    case "page":
      return {
        kind: "page",
        speaker: str(data.sender),
        target: str(data.target),
        message: str(data.message),
        raw: msg.text || "",
        sourceRef: num(data.source_ref),
      };

    case "channel":
      return parseChannelEvent(msg, data);

    case "whisper":
      return {
        kind: "whisper",
        speaker: str(data.speaker),
        message: str(data.message),
        raw: msg.text || "",
        sourceRef: num(data.source_ref),
      };

    case "emit":
      return {
        kind: "emit",
        speaker: "",
        message: msg.text || "",
        raw: msg.text || "",
      };

    // Ignore non-chat events
    default:
      return null;
  }
}

// Channel messages are pre-formatted: "[Header] PlayerName says, "text""
// We need to extract the speaker and message from the formatted string.
const CHANNEL_SAY_RE = /^\[.*?\]\s+(\S+)\s+says?,\s*"(.+)"$/s;
const CHANNEL_POSE_RE = /^\[.*?\]\s+(\S+)\s+(.+)$/s;

function parseChannelEvent(
  msg: WSMessage,
  data: Record<string, unknown>
): ParsedEvent {
  const channelName = str(data.channel) || msg.channel || "";
  const rawMessage = str(data.message) || msg.text || "";

  // Try to parse speaker from formatted message
  let speaker = "";
  let message = rawMessage;

  const sayMatch = rawMessage.match(CHANNEL_SAY_RE);
  if (sayMatch) {
    speaker = sayMatch[1]!;
    message = sayMatch[2]!;
  } else {
    const poseMatch = rawMessage.match(CHANNEL_POSE_RE);
    if (poseMatch) {
      speaker = poseMatch[1]!;
      message = poseMatch[2]!;
    }
  }

  return {
    kind: "channel",
    speaker,
    message,
    raw: rawMessage,
    channel: channelName,
  };
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function num(v: unknown): number | undefined {
  return typeof v === "number" ? v : undefined;
}
