import { PACK_HEADINGS } from "./pack-headings.ts";

export interface Sec { heading: string; body: string }

const norm = (s: string) => s.toLowerCase().replace(/^[\s\d.)\-:]+/, "").replace(/[\s:]+$/, "").replace(/\s+/g, " ");

/** The headings of a form's own boxes, English / French as the approved pack prints them. Empty for forms with no pack layout. */
export function packHeadings(code: string): string[] {
  return PACK_HEADINGS[code] ?? [];
}

/**
 * The sections the editor starts from: the sections already written (kept as they are, first) followed by every pack
 * heading not yet present, empty. Text typed under a pack heading is printed in that box of the form; empty sections
 * are not saved. A form with no pack layout starts with one untitled section.
 */
export function withPackHeadings(code: string, existing: Sec[]): Sec[] {
  const heads = packHeadings(code);
  const have = new Set(existing.map((s) => norm(s.heading)));
  const out = [...existing];
  for (const h of heads) {
    if (!have.has(norm(h))) { out.push({ heading: h, body: "" }); have.add(norm(h)); }
  }
  return out.length ? out : [{ heading: "", body: "" }];
}
