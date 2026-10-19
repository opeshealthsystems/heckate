import { test } from "node:test";
import assert from "node:assert/strict";
import { company } from "./company.ts";
import { en } from "./en.ts";
import { fr } from "./fr.ts";
import { faqLd, organizationLd, sitemapXml, PAGES } from "./seo.ts";

test("organization JSON-LD omits unconfirmed facts", () => {
  const ld = organizationLd({ ...company, rccm: "", phone: "", founders: [] }, en);
  assert.equal(ld.legalName, "HEKATE SYSTEMS SARL");
  for (const k of ["identifier", "telephone", "founder"]) assert.ok(!(k in ld), `${k} must be absent`);
});

test("organization JSON-LD includes facts once supplied", () => {
  const ld = organizationLd({ ...company, rccm: "RC/DLA/2026/B/1", phone: "+237000", founders: ["A", "B"] }, en) as Record<string, unknown>;
  assert.ok(ld.identifier && ld.telephone && Array.isArray(ld.founder));
});

test("FAQPage covers every question in both languages", () => {
  for (const d of [en, fr]) {
    const n = d.services.items.reduce((a, s) => a + s.faq.length, 0);
    assert.equal((faqLd(d.services.items).mainEntity as unknown[]).length, n);
  }
});

function keyPaths(o: unknown, prefix = ""): string[] {
  if (Array.isArray(o)) return [`${prefix}[${o.length}]`, ...o.flatMap((x, i) => keyPaths(x, `${prefix}[${i}]`))];
  if (o && typeof o === "object") return Object.entries(o).flatMap(([k, v]) => keyPaths(v, `${prefix}.${k}`));
  return [prefix];
}

test("EN and FR have identical keys, list lengths and service slugs", () => {
  const strip = (d: typeof en) => ({ ...d, lang: "", locale: "", services: { ...d.services, items: d.services.items.map((s) => ({ slug: s.slug, n: s.faq.length })) } });
  assert.deepEqual(keyPaths(strip(fr)), keyPaths(strip(en)));
  assert.deepEqual(fr.services.items.map((s) => s.slug), en.services.items.map((s) => s.slug));
  assert.deepEqual(fr.services.items.map((s) => s.faq.length), en.services.items.map((s) => s.faq.length));
});

test("sitemap lists both languages with hreflang", () => {
  const x = sitemapXml("https://example.com", PAGES);
  assert.match(x, /<loc>https:\/\/example.com\/fr\/services\/<\/loc>/);
  assert.match(x, /hreflang="fr"/);
});
