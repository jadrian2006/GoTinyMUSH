/**
 * Full xterm-256 + true color ANSI SGR parser.
 * Outputs inline style objects (no CSS classes) so Tailwind purging is irrelevant.
 */

// Standard 16-color palette (indices 0-15)
const COLORS_16: string[] = [
  "#000000", // 0 black
  "#aa0000", // 1 red
  "#00aa00", // 2 green
  "#aa5500", // 3 yellow/brown
  "#0000aa", // 4 blue
  "#aa00aa", // 5 magenta
  "#00aaaa", // 6 cyan
  "#aaaaaa", // 7 white (light gray)
  "#555555", // 8 bright black (dark gray)
  "#ff5555", // 9 bright red
  "#55ff55", // 10 bright green
  "#ffff55", // 11 bright yellow
  "#5555ff", // 12 bright blue
  "#ff55ff", // 13 bright magenta
  "#55ffff", // 14 bright cyan
  "#ffffff", // 15 bright white
];

// Build the full 256-color lookup table
const COLORS_256: string[] = buildXterm256Table();

function buildXterm256Table(): string[] {
  const table: string[] = [...COLORS_16];

  // 16-231: 6x6x6 color cube
  const levels = [0, 95, 135, 175, 215, 255];
  for (let r = 0; r < 6; r++) {
    for (let g = 0; g < 6; g++) {
      for (let b = 0; b < 6; b++) {
        table.push(
          `#${hex2(levels[r])}${hex2(levels[g])}${hex2(levels[b])}`,
        );
      }
    }
  }

  // 232-255: grayscale ramp
  for (let i = 0; i < 24; i++) {
    const v = 8 + i * 10;
    table.push(`#${hex2(v)}${hex2(v)}${hex2(v)}`);
  }

  return table;
}

function hex2(n: number): string {
  return n.toString(16).padStart(2, "0");
}

// Theme defaults for inverse mode when no explicit color is set
const DEFAULT_FG = "#eaeaea";
const DEFAULT_BG = "#1a1a2e";

interface AnsiState {
  fg: string | null;
  bg: string | null;
  bold: boolean;
  underline: boolean;
  blink: boolean;
  inverse: boolean;
  // Track the raw fg color index (0-7) so bold promotion works
  fgIndex: number | null;
}

function freshState(): AnsiState {
  return {
    fg: null,
    bg: null,
    bold: false,
    underline: false,
    blink: false,
    inverse: false,
    fgIndex: null,
  };
}

export interface AnsiSpan {
  text: string;
  style: Record<string, string>;
}

const ANSI_REGEX = /\x1b\[([0-9;]*)m/g;

/**
 * Parse a single line of text containing ANSI escape sequences.
 * Returns an array of spans with inline style objects.
 */
export function parseAnsiLine(line: string): AnsiSpan[] {
  const spans: AnsiSpan[] = [];
  let lastIndex = 0;
  let state = freshState();
  let match: RegExpExecArray | null;

  ANSI_REGEX.lastIndex = 0;
  while ((match = ANSI_REGEX.exec(line)) !== null) {
    // Push text before this escape
    if (match.index > lastIndex) {
      const text = line.slice(lastIndex, match.index);
      if (text) spans.push({ text, style: stateToStyle(state) });
    }
    // Process SGR parameters
    state = processSGR(match[1], state);
    lastIndex = ANSI_REGEX.lastIndex;
  }

  // Remaining text after last escape
  if (lastIndex < line.length) {
    spans.push({ text: line.slice(lastIndex), style: stateToStyle(state) });
  }

  if (spans.length === 0) {
    spans.push({ text: line, style: {} });
  }

  return spans;
}

function processSGR(params: string, prev: AnsiState): AnsiState {
  const state = { ...prev };

  if (!params || params === "0") {
    return freshState();
  }

  const codes = params.split(";").map(Number);
  let i = 0;

  while (i < codes.length) {
    const c = codes[i];

    if (c === 0) {
      // Reset all
      Object.assign(state, freshState());
    } else if (c === 1) {
      state.bold = true;
      // Bold promotion: if we already have a standard fg 0-7, promote it
      if (state.fgIndex !== null && state.fgIndex >= 0 && state.fgIndex <= 7) {
        state.fg = COLORS_16[state.fgIndex + 8];
      }
    } else if (c === 4) {
      state.underline = true;
    } else if (c === 5 || c === 6) {
      state.blink = true;
    } else if (c === 7) {
      state.inverse = true;
    } else if (c === 21 || c === 22) {
      state.bold = false;
      // Undo bold promotion
      if (state.fgIndex !== null && state.fgIndex >= 0 && state.fgIndex <= 7) {
        state.fg = COLORS_16[state.fgIndex];
      }
    } else if (c === 24) {
      state.underline = false;
    } else if (c === 25) {
      state.blink = false;
    } else if (c === 27) {
      state.inverse = false;
    } else if (c >= 30 && c <= 37) {
      // Standard foreground
      const idx = c - 30;
      state.fgIndex = idx;
      state.fg = state.bold ? COLORS_16[idx + 8] : COLORS_16[idx];
    } else if (c === 38) {
      // Extended foreground: 38;5;N or 38;2;R;G;B
      const result = parseExtendedColor(codes, i);
      if (result.color) {
        state.fg = result.color;
        state.fgIndex = null;
      }
      i = result.nextIndex;
      continue;
    } else if (c === 39) {
      // Default foreground
      state.fg = null;
      state.fgIndex = null;
    } else if (c >= 40 && c <= 47) {
      // Standard background
      state.bg = COLORS_16[c - 40];
    } else if (c === 48) {
      // Extended background: 48;5;N or 48;2;R;G;B
      const result = parseExtendedColor(codes, i);
      if (result.color) state.bg = result.color;
      i = result.nextIndex;
      continue;
    } else if (c === 49) {
      // Default background
      state.bg = null;
    } else if (c >= 90 && c <= 97) {
      // Bright foreground
      const idx = c - 90 + 8;
      state.fg = COLORS_16[idx];
      state.fgIndex = null; // Already bright, no promotion
    } else if (c >= 100 && c <= 107) {
      // Bright background
      state.bg = COLORS_16[c - 100 + 8];
    }

    i++;
  }

  return state;
}

function parseExtendedColor(
  codes: number[],
  startIndex: number,
): { color: string | null; nextIndex: number } {
  const mode = codes[startIndex + 1];

  if (mode === 5) {
    // 256-color: 38;5;N or 48;5;N
    const n = codes[startIndex + 2];
    if (n !== undefined && n >= 0 && n <= 255) {
      return { color: COLORS_256[n], nextIndex: startIndex + 3 };
    }
    return { color: null, nextIndex: startIndex + 3 };
  }

  if (mode === 2) {
    // True color: 38;2;R;G;B or 48;2;R;G;B
    const r = codes[startIndex + 2] ?? 0;
    const g = codes[startIndex + 3] ?? 0;
    const b = codes[startIndex + 4] ?? 0;
    return {
      color: `rgb(${clamp(r)},${clamp(g)},${clamp(b)})`,
      nextIndex: startIndex + 5,
    };
  }

  return { color: null, nextIndex: startIndex + 2 };
}

function clamp(n: number): number {
  return Math.max(0, Math.min(255, n));
}

function stateToStyle(state: AnsiState): Record<string, string> {
  const style: Record<string, string> = {};

  let fg = state.fg;
  let bg = state.bg;

  if (state.inverse) {
    const effectiveFg = fg ?? DEFAULT_FG;
    const effectiveBg = bg ?? DEFAULT_BG;
    fg = effectiveBg;
    bg = effectiveFg;
  }

  if (fg) style.color = fg;
  if (bg) style.backgroundColor = bg;
  if (state.bold) style.fontWeight = "bold";
  if (state.underline) style.textDecoration = "underline";
  if (state.blink) style.animation = "ansi-blink 1s step-end infinite";

  return style;
}
