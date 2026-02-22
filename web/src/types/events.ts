export interface WSMessage {
  type: string;
  text?: string;
  data?: Record<string, unknown>;
  channel?: string;
  command?: string;
}

export interface WhoEntry {
  name: string;
  ref: number;
  on_for: string;
  idle: string;
  doing: string;
}

export interface ChannelInfo {
  name: string;
  header: string;
  subscribers: number;
  alias?: string;
  listening?: boolean;
}

export interface ScrollbackMessage {
  id: number;
  channel: string;
  sender_ref: number;
  sender_name: string;
  message: string;
  timestamp: number;
}

export interface PaneFilter {
  types: string[];      // e.g. ["page", "whisper"] — empty = all types
  channels: string[];   // e.g. ["Public"] — empty = all channels
}

export interface PaneStyle {
  bgColor: string;      // hex, default "#1a1a2e"
  ansiEnabled: boolean;  // default true
  fontSize: number;      // px, default 14
  fontFamily: string;    // CSS font-family value
}

export interface PaneConfig {
  id: string;
  title: string;
  filter: PaneFilter;
  style: PaneStyle;
  gridRow: number;
  gridCol: number;
  gridRowSpan: number;
  gridColSpan: number;
  poppedOut: boolean;
  minimized: boolean;
  locked: boolean;       // if true, position cannot be changed by drag
}

export interface AuthResponse {
  token: string;
}

export interface OutputLine {
  id: number;
  text: string;
  type: string;
  channel?: string;
  timestamp: number;
}
