import test from "node:test";
import assert from "node:assert/strict";
import { contrast, palettes } from "./palette.ts";

const textKeys = ["fg", "text2", "muted", "goldText", "ok", "warn", "bad", "info", "p1", "p2", "p3", "p4"] as const;
const surfaces = ["bg", "surface", "raised"] as const;

for (const mode of ["dark", "light"] as const) {
  test(`${mode}: every text and severity colour is >= 4.5:1 on every surface`, () => {
    const p = palettes[mode];
    const fails: string[] = [];
    for (const k of textKeys) for (const s of surfaces) {
      const r = contrast(p[k], p[s]);
      if (r < 4.5) fails.push(`${k} on ${s} ${r.toFixed(2)}`);
    }
    assert.deepEqual(fails, []);
  });
  test(`${mode}: status text is readable on its tinted fill`, () => {
    const p = palettes[mode];
    const fails: string[] = [];
    for (const [t, b] of [["ok", "okBg"], ["warn", "warnBg"], ["bad", "badBg"], ["info", "infoBg"]] as const) {
      const r = contrast(p[t], p[b]);
      if (r < 4.5) fails.push(`${t} on ${b} ${r.toFixed(2)}`);
    }
    assert.deepEqual(fails, []);
  });
  test(`${mode}: gold button text and input edges are readable`, () => {
    const p = palettes[mode];
    assert.ok(contrast(p.onGold, p.goldFill) >= 4.5);
    assert.ok(contrast(p.inputBorder, p.bg) >= 3, "WCAG 1.4.11 asks 3:1 for control edges");
  });
}
