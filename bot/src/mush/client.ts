import { EventEmitter } from "node:events";
import WebSocket from "ws";
import type { WSMessage } from "../types.js";
import { Backoff } from "../util/backoff.js";

export interface MushClientEvents {
  event: [WSMessage];
  connected: [];
  disconnected: [];
  login: [{ playerRef: number; playerName: string }];
}

export class MushClient extends EventEmitter {
  private ws: WebSocket | null = null;
  private backoff = new Backoff();
  private closed = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly wsUrl: string,
    private token: string
  ) {
    super();
  }

  connect(): void {
    if (this.closed) return;

    const url = `${this.wsUrl}?token=${encodeURIComponent(this.token)}`;
    const ws = new WebSocket(url);

    ws.on("open", () => {
      this.backoff.reset();
      this.emit("connected");
    });

    ws.on("message", (raw: WebSocket.RawData) => {
      try {
        const msg = JSON.parse(raw.toString()) as WSMessage;

        // Handle login confirmation
        if (msg.type === "login" && msg.data) {
          this.emit("login", {
            playerRef: msg.data.player_ref as number,
            playerName: msg.data.player_name as string,
          });
        }

        this.emit("event", msg);
      } catch {
        // Ignore malformed messages
      }
    });

    ws.on("close", () => {
      this.ws = null;
      this.emit("disconnected");
      this.scheduleReconnect();
    });

    ws.on("error", (err: Error) => {
      console.error("[ws] Error:", err.message);
      // close event will follow
    });

    this.ws = ws;
  }

  send(command: string): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(command);
    }
  }

  updateToken(token: string): void {
    this.token = token;
  }

  close(): void {
    this.closed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  private scheduleReconnect(): void {
    if (this.closed) return;
    const delay = this.backoff.next();
    console.log(`[ws] Reconnecting in ${delay}ms...`);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }
}
