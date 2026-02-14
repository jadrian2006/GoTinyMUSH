import type { WSMessage } from "../types/events";

export type WSHandler = (msg: WSMessage) => void;

export class WebSocketManager {
  private ws: WebSocket | null = null;
  private url: string;
  private token: string | null;
  private handlers: WSHandler[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private closed = false;

  constructor(url: string, token: string | null = null) {
    this.url = url;
    this.token = token;
  }

  onMessage(handler: WSHandler) {
    this.handlers.push(handler);
    return () => {
      this.handlers = this.handlers.filter((h) => h !== handler);
    };
  }

  connect() {
    this.closed = false;
    let wsUrl = this.url;
    if (this.token) {
      wsUrl += `?token=${encodeURIComponent(this.token)}`;
    }

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      this.reconnectDelay = 1000;
      this.dispatch({ type: "connected" });
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        this.dispatch(msg);
      } catch {
        this.dispatch({ type: "text", text: event.data });
      }
    };

    this.ws.onclose = () => {
      this.dispatch({ type: "disconnected" });
      if (!this.closed) {
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  send(msg: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  sendCommand(command: string) {
    this.send({ type: "command", command });
  }

  disconnect() {
    this.closed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private dispatch(msg: WSMessage) {
    for (const handler of this.handlers) {
      handler(msg);
    }
  }

  private scheduleReconnect() {
    this.reconnectTimer = setTimeout(() => {
      this.connect();
    }, this.reconnectDelay);
    this.reconnectDelay = Math.min(
      this.reconnectDelay * 2,
      this.maxReconnectDelay,
    );
  }
}
