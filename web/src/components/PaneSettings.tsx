import { panes, updatePane } from "../stores/paneStore";
import { channels } from "../stores/gameStore";

// Server EventType values — these are the actual type values
// sent in WSMessage.type by the GoTinyMUSH server
const MESSAGE_TYPES: { value: string; label: string; desc: string }[] = [
  { value: "say", label: "Say", desc: "Speech" },
  { value: "pose", label: "Pose", desc: "Emotes" },
  { value: "page", label: "Page", desc: "Private messages" },
  { value: "whisper", label: "Whisper", desc: "Whispers" },
  { value: "channel", label: "Channel", desc: "Channel messages" },
  { value: "emit", label: "Emit", desc: "@emit/@remit" },
  { value: "room", label: "Room", desc: "Room descriptions" },
  { value: "move", label: "Move", desc: "Arrive/depart" },
  { value: "connect", label: "Connect", desc: "Player connected" },
  { value: "disconnect", label: "Disconnect", desc: "Player disconnected" },
  { value: "text", label: "Text", desc: "Generic output" },
];

const FONT_CHOICES = [
  { label: "Courier New", value: "'Courier New', monospace", mono: true },
  { label: "Consolas", value: "Consolas, monospace", mono: true },
  { label: "Source Code Pro", value: "'Source Code Pro', monospace", mono: true },
  { label: "Lucida Console", value: "'Lucida Console', monospace", mono: true },
  { label: "Menlo", value: "Menlo, monospace", mono: true },
  { label: "monospace", value: "monospace", mono: true },
  { label: "Arial", value: "Arial, sans-serif", mono: false },
  { label: "Verdana", value: "Verdana, sans-serif", mono: false },
  { label: "Georgia", value: "Georgia, serif", mono: false },
];

interface PaneSettingsProps {
  paneId: string;
  onClose: () => void;
}

export function PaneSettings({ paneId, onClose }: PaneSettingsProps) {
  const pane = panes.value.find((p) => p.id === paneId);
  if (!pane) return null;

  const channelList = channels.value;

  const toggleType = (type: string) => {
    const types = pane.filter.types.includes(type)
      ? pane.filter.types.filter((t) => t !== type)
      : [...pane.filter.types, type];
    updatePane(paneId, { filter: { ...pane.filter, types } });
  };

  const toggleChannel = (ch: string) => {
    const chans = pane.filter.channels.includes(ch)
      ? pane.filter.channels.filter((c) => c !== ch)
      : [...pane.filter.channels, ch];
    updatePane(paneId, { filter: { ...pane.filter, channels: chans } });
  };

  return (
    <div class="pane-settings">
      {/* Backdrop */}
      <div class="fixed inset-0 z-40" onClick={onClose} />

      <div class="pane-settings-dropdown">
        {/* Title */}
        <div class="mb-2">
          <label class="text-xs text-mush-dim uppercase tracking-wider">Title</label>
          <input
            type="text"
            value={pane.title}
            onInput={(e) => updatePane(paneId, { title: (e.target as HTMLInputElement).value })}
            class="w-full bg-mush-input border border-mush-panel rounded px-2 py-1 text-xs text-mush-text outline-none focus:border-mush-accent mt-1"
          />
        </div>

        {/* Message type filters */}
        <div class="mb-2">
          <label class="text-xs text-mush-dim uppercase tracking-wider">Message Types</label>
          <div class="text-xs text-mush-dim italic mb-1">(empty = all)</div>
          <div class="grid grid-cols-2 gap-1 mt-1">
            {MESSAGE_TYPES.map((t) => (
              <label key={t.value} class="flex items-center gap-1 text-xs text-mush-text cursor-pointer" title={t.desc}>
                <input
                  type="checkbox"
                  checked={pane.filter.types.includes(t.value)}
                  onChange={() => toggleType(t.value)}
                  class="accent-mush-accent"
                />
                {t.label}
              </label>
            ))}
          </div>
        </div>

        {/* Channel filters */}
        {channelList.length > 0 && (
          <div class="mb-2">
            <label class="text-xs text-mush-dim uppercase tracking-wider">Channels</label>
            <div class="text-xs text-mush-dim italic mb-1">(empty = all)</div>
            <div class="space-y-1 mt-1 max-h-24 overflow-y-auto">
              {channelList.map((ch) => (
                <label key={ch.name} class="flex items-center gap-1 text-xs text-mush-text cursor-pointer">
                  <input
                    type="checkbox"
                    checked={pane.filter.channels.includes(ch.name)}
                    onChange={() => toggleChannel(ch.name)}
                    class="accent-mush-accent"
                  />
                  {ch.name}
                </label>
              ))}
            </div>
          </div>
        )}

        {/* Font family */}
        <div class="mb-2">
          <label class="text-xs text-mush-dim uppercase tracking-wider">Font</label>
          <div class="text-xs text-mush-dim italic mb-1">Fixed-width recommended</div>
          <select
            value={pane.style.fontFamily}
            onChange={(e) =>
              updatePane(paneId, {
                style: { ...pane.style, fontFamily: (e.target as HTMLSelectElement).value },
              })
            }
            class="w-full bg-mush-input border border-mush-panel rounded px-2 py-1 text-xs text-mush-text outline-none focus:border-mush-accent mt-1"
          >
            <optgroup label="Fixed-width (Recommended)">
              {FONT_CHOICES.filter((f) => f.mono).map((f) => (
                <option key={f.value} value={f.value} style={{ fontFamily: f.value }}>
                  {f.label}
                </option>
              ))}
            </optgroup>
            <optgroup label="Variable-width">
              {FONT_CHOICES.filter((f) => !f.mono).map((f) => (
                <option key={f.value} value={f.value} style={{ fontFamily: f.value }}>
                  {f.label}
                </option>
              ))}
            </optgroup>
          </select>
        </div>

        {/* Font size */}
        <div class="mb-2">
          <label class="text-xs text-mush-dim uppercase tracking-wider">
            Font Size: {pane.style.fontSize}px
          </label>
          <input
            type="range"
            min="10"
            max="24"
            value={pane.style.fontSize}
            onInput={(e) =>
              updatePane(paneId, {
                style: { ...pane.style, fontSize: parseInt((e.target as HTMLInputElement).value, 10) },
              })
            }
            class="w-full accent-mush-accent mt-1"
          />
        </div>

        {/* Background color */}
        <div class="mb-2">
          <label class="text-xs text-mush-dim uppercase tracking-wider">Background</label>
          <div class="flex items-center gap-2 mt-1">
            <input
              type="color"
              value={pane.style.bgColor}
              onInput={(e) =>
                updatePane(paneId, {
                  style: { ...pane.style, bgColor: (e.target as HTMLInputElement).value },
                })
              }
              class="w-6 h-6 border border-mush-panel rounded cursor-pointer"
            />
            <span class="text-xs text-mush-dim">{pane.style.bgColor}</span>
          </div>
        </div>

        {/* ANSI toggle */}
        <div class="mb-1">
          <label class="flex items-center gap-2 text-xs text-mush-text cursor-pointer">
            <input
              type="checkbox"
              checked={pane.style.ansiEnabled}
              onChange={() =>
                updatePane(paneId, {
                  style: { ...pane.style, ansiEnabled: !pane.style.ansiEnabled },
                })
              }
              class="accent-mush-accent"
            />
            ANSI Colors
          </label>
        </div>
      </div>
    </div>
  );
}
