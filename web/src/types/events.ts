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
}

export interface ScrollbackMessage {
  id: number;
  channel: string;
  sender_ref: number;
  sender_name: string;
  message: string;
  timestamp: number;
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
