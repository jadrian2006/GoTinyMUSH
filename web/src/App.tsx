import { useEffect, useCallback } from "preact/hooks";
import { WindowGrid } from "./components/WindowGrid";
import { InputBar } from "./components/InputBar";
import { LoginForm } from "./components/LoginForm";
import { PaneManagerDrawer } from "./components/PaneManagerDrawer";
import { ContextMenu } from "./components/ContextMenu";
import { SettingsMenu } from "./components/SettingsMenu";
import { useAuth } from "./hooks/useAuth";
import { useWebSocket } from "./hooks/useWebSocket";
import {
  isLoggedIn,
  connected,
  token,
  playerName,
} from "./stores/gameStore";
import { sidebarOpen } from "./stores/prefsStore";

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
          <SettingsMenu />
          <button
            onClick={() => (sidebarOpen.value = !sidebarOpen.value)}
            class="text-mush-dim hover:text-mush-accent transition-colors"
            title="Toggle sidebar"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
              <line x1="15" y1="3" x2="15" y2="21" />
            </svg>
          </button>
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
        {/* Grid + input */}
        <div class="flex flex-col flex-1 min-w-0">
          <WindowGrid />
          <InputBar onSubmit={handleCommand} disabled={!connected.value} />
        </div>

        {/* Pane manager sidebar */}
        <PaneManagerDrawer />
      </div>

      {/* Global context menu */}
      <ContextMenu />
    </div>
  );
}
