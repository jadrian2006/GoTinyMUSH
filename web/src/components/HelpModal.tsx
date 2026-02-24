interface HelpModalProps {
  onClose: () => void;
}

export function HelpModal({ onClose }: HelpModalProps) {
  return (
    <>
      {/* Mobile backdrop */}
      <div
        class="help-backdrop"
        onClick={onClose}
      />

      <div class="help-panel bg-mush-surface border-l border-mush-panel flex flex-col">
        {/* Header */}
        <div class="flex items-center justify-between p-2 border-b border-mush-panel">
          <span class="text-xs font-bold text-mush-accent uppercase tracking-wider">Help</span>
          <button
            class="pane-btn pane-btn-close"
            onClick={onClose}
            title="Close"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {/* Scrollable content */}
        <div class="help-panel-body">
          {/* Getting Started */}
          <h2>Getting Started</h2>
          <dl>
            <dt>Connecting</dt>
            <dd>Log in with your character name and password. The green dot in the header shows connection status.</dd>

            <dt>Command Input</dt>
            <dd>Type commands in the bar at the bottom. Press <kbd>Up</kbd>/<kbd>Down</kbd> arrows to recall history. Drag the top edge of the input bar to resize it.</dd>
          </dl>

          {/* Output Panes */}
          <h2>Output Panes</h2>
          <dl>
            <dt>What are panes?</dt>
            <dd>Output is displayed in resizable panes arranged in a grid. The "Main" pane shows all output by default. Additional panes can filter specific message types.</dd>

            <dt>Creating panes</dt>
            <dd>Open the sidebar and click "+ Add Pane" to create a new pane. You can also right-click a channel name to open it in a dedicated pane.</dd>

            <dt>Titlebar buttons</dt>
            <dd>Each pane has controls in its titlebar:</dd>

            <dt>Pause / Play</dt>
            <dd>Freeze auto-scroll to read history. A badge shows how many new messages arrived while paused.</dd>

            <dt>Lock</dt>
            <dd>Prevent a pane from being dragged or reordered.</dd>

            <dt>Settings (gear)</dt>
            <dd>Configure filters, font size, colors, and ANSI rendering for this pane.</dd>

            <dt>Minimize</dt>
            <dd>Collapse the pane to its titlebar only.</dd>

            <dt>Pop out</dt>
            <dd>Open the pane in a separate browser window.</dd>

            <dt>Close</dt>
            <dd>Remove the pane (not available on Main).</dd>

            <dt>Drag grip</dt>
            <dd>Drag to reorder panes in the grid.</dd>
          </dl>

          {/* Split Scroll */}
          <h2>Split Scroll</h2>
          <dl>
            <dt>Splitting a pane</dt>
            <dd>Right-click any output line and choose "Split here". The top half freezes on that line while the bottom half continues showing live output.</dd>

            <dt>Resizing</dt>
            <dd>Drag the divider between the two halves to adjust the split.</dd>

            <dt>Removing a split</dt>
            <dd>Right-click in the pane and choose "Remove split", or click the x button on the divider.</dd>
          </dl>

          {/* Pane Filters */}
          <h2>Pane Filters</h2>
          <dl>
            <dt>Message type filters</dt>
            <dd>In pane settings, filter by message type: say, page, whisper, channel, pose, OOB, etc.</dd>

            <dt>Channel filters</dt>
            <dd>Filter a pane to show only specific channels.</dd>

            <dt>Auto-hide</dt>
            <dd>The Main pane automatically hides messages that are shown in a dedicated pane, so you don't see duplicates.</dd>
          </dl>

          {/* Sidebar */}
          <h2>Sidebar</h2>
          <dl>
            <dt>WHO list</dt>
            <dd>Shows online players. Auto-refreshes periodically.</dd>

            <dt>Channels</dt>
            <dd>Lists your channels. Right-click a channel to open it in its own pane.</dd>

            <dt>Panes</dt>
            <dd>Manage all panes. Right-click for options like rename, minimize, or close.</dd>

            <dt>Grid columns</dt>
            <dd>Slider to set the pane grid from 1 to 4 columns.</dd>

            <dt>Organize</dt>
            <dd>Reset all pane positions to a clean default layout.</dd>
          </dl>

          {/* Layout & Settings */}
          <h2>Layout &amp; Settings</h2>
          <dl>
            <dt>Classic vs Widescreen</dt>
            <dd>Classic mode centers output at 80 characters wide. Widescreen uses the full browser width. Toggle via the gear icon in the header.</dd>
          </dl>

          {/* Connection */}
          <h2>Connection</h2>
          <dl>
            <dt>Auto-reconnect</dt>
            <dd>If the connection drops, the client automatically retries with increasing delays. The header shows "Reconnecting in Ns..." during backoff.</dd>

            <dt>Status indicator</dt>
            <dd>Green dot = connected, red dot = disconnected.</dd>
          </dl>
        </div>
      </div>
    </>
  );
}
