import { describe, it, expect } from "vitest";
import { parseAnsiLine } from "./ansiParser";

describe("parseAnsiLine", () => {
  it("returns plain text with empty style when no ANSI codes", () => {
    const result = parseAnsiLine("hello world");
    expect(result).toEqual([{ text: "hello world", style: {} }]);
  });

  it("returns single empty-text span for empty string", () => {
    const result = parseAnsiLine("");
    expect(result).toEqual([{ text: "", style: {} }]);
  });

  // --- Standard 16 colors ---

  it("parses standard foreground colors (30-37)", () => {
    const result = parseAnsiLine("\x1b[31mRED\x1b[0m");
    expect(result[0]).toEqual({ text: "RED", style: { color: "#aa0000" } });
    // No trailing span — reset at end of string produces nothing
    expect(result).toHaveLength(1);
  });

  it("parses standard background colors (40-47)", () => {
    const result = parseAnsiLine("\x1b[44mBLUE BG\x1b[0m");
    expect(result[0]).toEqual({
      text: "BLUE BG",
      style: { backgroundColor: "#0000aa" },
    });
  });

  it("parses bright foreground colors (90-97)", () => {
    const result = parseAnsiLine("\x1b[92mBRIGHT GREEN\x1b[0m");
    expect(result[0]).toEqual({
      text: "BRIGHT GREEN",
      style: { color: "#55ff55" },
    });
  });

  it("parses bright background colors (100-107)", () => {
    const result = parseAnsiLine("\x1b[101mBRIGHT RED BG\x1b[0m");
    expect(result[0]).toEqual({
      text: "BRIGHT RED BG",
      style: { backgroundColor: "#ff5555" },
    });
  });

  // --- Attributes ---

  it("parses bold attribute", () => {
    const result = parseAnsiLine("\x1b[1mBOLD\x1b[0m");
    expect(result[0].style.fontWeight).toBe("bold");
  });

  it("parses underline attribute", () => {
    const result = parseAnsiLine("\x1b[4mUNDER\x1b[0m");
    expect(result[0].style.textDecoration).toBe("underline");
  });

  it("parses blink attribute", () => {
    const result = parseAnsiLine("\x1b[5mBLINK\x1b[0m");
    expect(result[0].style.animation).toBe("ansi-blink 1s step-end infinite");
  });

  // --- Bold + color promotion ---

  it("promotes standard fg 0-7 to bright when bold is set first", () => {
    // bold then red: should be bright red
    const result = parseAnsiLine("\x1b[1;31mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#ff5555");
    expect(result[0].style.fontWeight).toBe("bold");
  });

  it("promotes standard fg when bold is applied after color", () => {
    // red then bold
    const result = parseAnsiLine("\x1b[31m\x1b[1mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#ff5555");
  });

  it("does not promote bright colors further", () => {
    // bold + bright green (92) stays bright green
    const result = parseAnsiLine("\x1b[1;92mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#55ff55");
  });

  // --- 256-color mode ---

  it("parses 256-color foreground (standard range 0-15)", () => {
    const result = parseAnsiLine("\x1b[38;5;9mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#ff5555"); // bright red
  });

  it("parses 256-color foreground (cube range 16-231)", () => {
    // Index 16 = rgb(0,0,0) in the cube
    const result = parseAnsiLine("\x1b[38;5;16mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#000000");

    // Index 196 = rgb(255,0,0) — red in 6x6x6 cube
    const result2 = parseAnsiLine("\x1b[38;5;196mTEXT\x1b[0m");
    expect(result2[0].style.color).toBe("#ff0000");
  });

  it("parses 256-color foreground (grayscale range 232-255)", () => {
    // Index 232 = darkest gray (8,8,8)
    const result = parseAnsiLine("\x1b[38;5;232mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#080808");

    // Index 255 = lightest gray (238,238,238)
    const result2 = parseAnsiLine("\x1b[38;5;255mTEXT\x1b[0m");
    expect(result2[0].style.color).toBe("#eeeeee");
  });

  it("parses 256-color background", () => {
    const result = parseAnsiLine("\x1b[48;5;21mTEXT\x1b[0m");
    expect(result[0].style.backgroundColor).toBe("#0000ff");
  });

  // --- True color (24-bit) ---

  it("parses true color foreground", () => {
    const result = parseAnsiLine("\x1b[38;2;128;64;32mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("rgb(128,64,32)");
  });

  it("parses true color background", () => {
    const result = parseAnsiLine("\x1b[48;2;0;255;128mTEXT\x1b[0m");
    expect(result[0].style.backgroundColor).toBe("rgb(0,255,128)");
  });

  it("clamps true color values to 0-255", () => {
    // Values > 255 get clamped to 255
    const result = parseAnsiLine("\x1b[38;2;300;0;128mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("rgb(255,0,128)");
  });

  // --- Inverse ---

  it("swaps fg/bg with inverse", () => {
    const result = parseAnsiLine("\x1b[31;7mTEXT\x1b[0m");
    // fg was red, inverse means bg becomes red, fg becomes default bg
    expect(result[0].style.backgroundColor).toBe("#aa0000");
    expect(result[0].style.color).toBe("#1a1a2e"); // default bg as fg
  });

  it("uses theme defaults for inverse when no colors set", () => {
    const result = parseAnsiLine("\x1b[7mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("#1a1a2e"); // default bg
    expect(result[0].style.backgroundColor).toBe("#eaeaea"); // default fg
  });

  // --- Reset codes ---

  it("resets all attributes with SGR 0", () => {
    const result = parseAnsiLine("\x1b[1;31;44mSTYLED\x1b[0mPLAIN");
    expect(result[0].style.fontWeight).toBe("bold");
    expect(result[0].style.color).toBe("#ff5555");
    expect(result[0].style.backgroundColor).toBe("#0000aa");
    expect(result[1].style).toEqual({});
  });

  it("resets foreground with SGR 39", () => {
    const result = parseAnsiLine("\x1b[31mRED\x1b[39mDEFAULT");
    expect(result[0].style.color).toBe("#aa0000");
    expect(result[1].style.color).toBeUndefined();
  });

  it("resets background with SGR 49", () => {
    const result = parseAnsiLine("\x1b[41mRED BG\x1b[49mDEFAULT");
    expect(result[0].style.backgroundColor).toBe("#aa0000");
    expect(result[1].style.backgroundColor).toBeUndefined();
  });

  it("resets bold with SGR 22", () => {
    const result = parseAnsiLine("\x1b[1mBOLD\x1b[22mNORMAL");
    expect(result[0].style.fontWeight).toBe("bold");
    expect(result[1].style.fontWeight).toBeUndefined();
  });

  // --- Combined / complex sequences ---

  it("handles multiple SGR params in one escape", () => {
    const result = parseAnsiLine("\x1b[1;4;31mTEXT\x1b[0m");
    expect(result[0].style.fontWeight).toBe("bold");
    expect(result[0].style.textDecoration).toBe("underline");
    expect(result[0].style.color).toBe("#ff5555"); // bold promotion
  });

  it("handles text between multiple escapes", () => {
    const result = parseAnsiLine("A\x1b[31mB\x1b[32mC\x1b[0mD");
    expect(result).toHaveLength(4);
    expect(result[0]).toEqual({ text: "A", style: {} });
    expect(result[1].text).toBe("B");
    expect(result[1].style.color).toBe("#aa0000");
    expect(result[2].text).toBe("C");
    expect(result[2].style.color).toBe("#00aa00");
    expect(result[3]).toEqual({ text: "D", style: {} });
  });

  it("handles empty SGR (bare ESC[m) as reset", () => {
    const result = parseAnsiLine("\x1b[31mRED\x1b[mPLAIN");
    expect(result[0].style.color).toBe("#aa0000");
    expect(result[1].style).toEqual({});
  });

  it("handles 256-color fg + bg together", () => {
    const result = parseAnsiLine("\x1b[38;5;82;48;5;234mTEXT\x1b[0m");
    // 82 = cube color, 234 = grayscale
    expect(result[0].style.color).toBeDefined();
    expect(result[0].style.backgroundColor).toBeDefined();
  });

  it("handles true color fg + standard bg in one sequence", () => {
    const result = parseAnsiLine("\x1b[38;2;100;200;50;41mTEXT\x1b[0m");
    expect(result[0].style.color).toBe("rgb(100,200,50)");
    expect(result[0].style.backgroundColor).toBe("#aa0000");
  });
});
