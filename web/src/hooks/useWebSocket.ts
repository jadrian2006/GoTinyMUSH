import { useEffect, useRef, useCallback } from "preact/hooks";
import { WebSocketManager } from "../services/ws";
import type { WSMessage } from "../types/events";
import {
  addOutput,
  connected,
  setAuth,
  token,
  whoList,
} from "../stores/gameStore";

export function useWebSocket() {
  const wsRef = useRef<WebSocketManager | null>(null);

  const connect = useCallback((authToken: string | null) => {
    if (wsRef.current) {
      wsRef.current.disconnect();
    }

    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${location.host}/ws`;
    const ws = new WebSocketManager(wsUrl, authToken);

    ws.onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case "connected":
          connected.value = true;
          addOutput("--- Connected ---", "system");
          break;
        case "disconnected":
          connected.value = false;
          addOutput("--- Disconnected ---", "system");
          break;
        case "text":
          addOutput(msg.text ?? "", "text");
          break;
        case "login":
          if (msg.data) {
            const name = msg.data.player_name as string;
            const ref = msg.data.player_ref as number;
            const t = token.value;
            if (t) setAuth(t, name, ref);
            addOutput(`Logged in as ${name}`, "system");
          }
          break;
        case "welcome":
          addOutput(msg.text ?? "", "system");
          break;
        case "error":
          addOutput(`Error: ${msg.text}`, "error");
          break;
        case "channel":
          addOutput(msg.text ?? "", "channel", msg.channel);
          break;
        case "who":
          if (msg.data?.players) {
            whoList.value = msg.data.players as typeof whoList.value;
          }
          break;
        default:
          if (msg.text) {
            addOutput(msg.text, msg.type);
          }
          break;
      }
    });

    ws.connect();
    wsRef.current = ws;
  }, []);

  const sendCommand = useCallback((command: string) => {
    wsRef.current?.sendCommand(command);
  }, []);

  const disconnect = useCallback(() => {
    wsRef.current?.disconnect();
    wsRef.current = null;
    connected.value = false;
  }, []);

  useEffect(() => {
    return () => {
      wsRef.current?.disconnect();
    };
  }, []);

  return { connect, sendCommand, disconnect };
}
