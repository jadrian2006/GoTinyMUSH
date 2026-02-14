import { useEffect, useCallback } from "preact/hooks";
import { Terminal } from "./components/Terminal";
import { InputBar } from "./components/InputBar";
import { LoginForm } from "./components/LoginForm";
import { WhoPanel } from "./components/WhoPanel";
import { ChannelPanel } from "./components/ChannelPanel";
import { useAuth } from "./hooks/useAuth";
import { useWebSocket } from "./hooks/useWebSocket";
import {
  isLoggedIn,
  connected,
  token,
  playerName,
  activeChannel,
  outputLines,
} from "./stores/gameStore";

export function App() {
  const { login, logout } = useAuth();
  const { connect, sendCommand, disconnect } = useWebSocket();

  const handleLogin = useCallback(
    async (name: string, password: string) => {
      const t = await login(name, password);
      connect(t);
    },
    [login, connect],
  );

  // Reconnect on page load if we have a stored token
  useEffect(() => {
    const t = token.value;
    if (t) {
      connect(t);
    }
  }, [connect]);

  const handleCommand = useCallback(
    (command: string) => {
      if (command.toLowerCase() === "quit") {
        sendCommand(command);
        disconnect();
        logout();
        return;
      }
      sendCommand(command);
    },
    [sendCommand, disconnect, logout],
  );

  // Filter output by active channel
  const filteredLines = activeChannel.value
    ? outputLines.value.filter(
        (l) => l.channel === activeChannel.value || l.type === "system",
      )
    : outputLines.value;

  if (!isLoggedIn.value) {
    return <LoginForm onLogin={handleLogin} />;
  }

  return (
    <div class="h-full flex flex-col">
      {/* Header */}
      <div class="flex items-center justify-between px-3 py-1 bg-mush-surface border-b border-mush-panel text-xs">
        <div class="flex items-center gap-2">
          <span class="text-mush-accent font-bold">GoTinyMUSH</span>
          <span
            class={`w-2 h-2 rounded-full ${connected.value ? "bg-green-400" : "bg-red-400"}`}
          />
          <span class="text-mush-dim">
            {connected.value ? "Connected" : "Disconnected"}
          </span>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-mush-text">{playerName.value}</span>
          <button
            onClick={() => {
              disconnect();
              logout();
            }}
            class="text-mush-dim hover:text-mush-accent transition-colors"
          >
            Logout
          </button>
        </div>
      </div>

      {/* Main content */}
      <div class="flex flex-1 min-h-0">
        {/* Terminal + input */}
        <div class="flex flex-col flex-1 min-w-0">
          {/* Channel tabs */}
          {isLoggedIn.value && <ChannelPanel />}

          {/* Output area - uses filtered lines via signal override */}
          <Terminal />

          {/* Command input */}
          <InputBar onSubmit={handleCommand} disabled={!connected.value} />
        </div>

        {/* WHO sidebar */}
        <WhoPanel />
      </div>
    </div>
  );
}
