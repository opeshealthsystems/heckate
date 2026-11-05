import test from "node:test";
import assert from "node:assert/strict";
import { contrast, palettes } from "./palette.ts";

const grounds = ["canvas", "bg", "surface"] as const;

for (const mode of ["light", "dark"] as const) {
  const p = palettes[mode];
  test(`${mode}: text colours are >= 4.5:1 on the screen and the grouped surface`, () => {
    const fails: string[] = [];
    for (const k of ["label", "label2", "tint", "red", "orange", "green"] as const) {
      for (const g of grounds) {
        const r = contrast(p[k], p[g]);
        if (r < 4.5) fails.push(`${k} on ${g} ${r.toFixed(2)}`);
      }
    }
    assert.deepEqual(fails, []);
  });
  test(`${mode}: the primary button label is >= 4.5:1 on its fill, and the fill is >= 3:1 against both grounds`, () => {
    assert.ok(contrast(p.onTint, p.tintFill) >= 4.5, "label on fill");
    for (const g of grounds) assert.ok(contrast(p.tintFill, p[g]) >= 3, `fill on ${g}`);
  });
  test(`${mode}: placeholders, chevrons and icons (label3) are >= 3:1 on both grounds`, () => {
    for (const g of grounds) assert.ok(contrast(p.label3, p[g]) >= 3, `label3 on ${g}`);
  });
  test(`${mode}: the hairline separator is visible but quiet`, () => {
    for (const g of grounds) {
      const r = contrast(p.separator, p[g]);
      assert.ok(r >= 1.1 && r <= 2.5, `separator on ${g} ${r.toFixed(2)}`);
    }
  });
}

test("one accent: tint is a blue (b > r and b > g) in both modes, never gold", () => {
  for (const p of Object.values(palettes)) {
    const n = parseInt(p.tint.slice(1), 16);
    const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255];
    assert.ok(b > r && b > g);
  }
});
