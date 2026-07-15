import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {
  POISON_ZSCALERCTL,
  poisonBannerForWidth,
  poisonBeamSegments
} from "../src/brand.ts";
import {Welcome} from "../src/components/Welcome.tsx";
import {paletteFor} from "../src/theme.ts";
import {MotionProvider} from "../src/useMotion.ts";

const renderers: Array<{destroy(): void}> = [];

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

describe("Poison welcome identity", () => {
  test("pins the trusted FIGlet render and exact terminal-cell bounds", () => {
    const rendered = POISON_ZSCALERCTL.lines.join("\n");
    const digest = new Bun.CryptoHasher("sha256").update(rendered).digest("hex");
    expect(digest).toBe("378583226322ee0c7923ca6f5280679189036aba65969c4f5bf45b453b934072");
    expect(POISON_ZSCALERCTL.lines).toHaveLength(10);
    expect(Math.max(...POISON_ZSCALERCTL.lines.map(line => Bun.stringWidth(line)))).toBe(97);
    expect(POISON_ZSCALERCTL.lines.every(line => Bun.stringWidth(line) <= POISON_ZSCALERCTL.width)).toBeTrue();
    expect(POISON_ZSCALERCTL.lines.every(line => /^[\x20-\x7e]+$/u.test(line))).toBeTrue();
  });

  test("uses the literal full title only when all 97 cells fit", () => {
    expect(poisonBannerForWidth(96)).toBeUndefined();
    expect(poisonBannerForWidth(97)).toBe(POISON_ZSCALERCTL);
    expect(poisonBannerForWidth(Number.NaN)).toBeUndefined();
  });

  test("moves a color beam without changing banner text or geometry", () => {
    const line = POISON_ZSCALERCTL.lines[0]!;
    const frames = [0, 1, 5, 24].map(frame => poisonBeamSegments(line, frame, POISON_ZSCALERCTL.width, true));
    for (const segments of frames) {
      expect(`${segments.before}${segments.beam}${segments.after}`).toBe(line);
      expect(Bun.stringWidth(`${segments.before}${segments.beam}${segments.after}`)).toBe(Bun.stringWidth(line));
    }
    expect(new Set(frames.map(frame => frame.before.length)).size).toBeGreaterThan(1);
    const sweep = Array.from({length: 14}, (_, frame) => poisonBeamSegments(
      POISON_ZSCALERCTL.lines.at(-1)!,
      frame,
      POISON_ZSCALERCTL.width,
      true
    ));
    expect(Math.max(...sweep.map(segments => segments.before.length + segments.beam.length))).toBe(
      POISON_ZSCALERCTL.width
    );
    expect(poisonBeamSegments(line, 4, POISON_ZSCALERCTL.width, false)).toEqual({
      before: line,
      beam: "",
      after: ""
    });
  });

  test("renders Poison when roomy and retains the readable literal fallback", async () => {
    const colors = paletteFor("tokyonight", "dark");
    const art = await testRender(createElement(
      MotionProvider,
      {spinner: "hangul", mode: "off", active: false},
      createElement(Welcome, {
        colors,
        compact: false,
        artwork: true,
        availableWidth: 97,
        workspaceLabel: "fixture"
      })
    ), {width: 104, height: 18});
    renderers.push(art.renderer);
    await art.flush();
    const artFrame = art.captureCharFrame();
    for (const line of POISON_ZSCALERCTL.lines) expect(artFrame).toContain(line);
    expect(artFrame).toContain("OpenTUI lab");
    await act(async () => art.renderer.destroy());
    renderers.pop();

    const fallback = await testRender(createElement(Welcome, {
      colors,
      compact: false,
      artwork: true,
      availableWidth: 96,
      workspaceLabel: "fixture"
    }), {width: 104, height: 8});
    renderers.push(fallback.renderer);
    await fallback.flush();
    const fallbackFrame = fallback.captureCharFrame();
    expect(fallbackFrame).toContain("◆ zscalerctl OpenTUI lab");
    for (const line of POISON_ZSCALERCTL.lines) expect(fallbackFrame).not.toContain(line);
    expect(fallbackFrame).not.toContain("ZCTL");
  });
});
