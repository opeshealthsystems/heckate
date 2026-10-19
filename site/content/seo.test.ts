import { test } from "node:test";
import assert from "node:assert/strict";
import { company } from "./company.ts";
import { en } from "./en.ts";
import { fr } from "./fr.ts";
import { faqLd, organizationLd, sitemapXml, PAGES } from "./seo.ts";

test("organization JSON-LD omits unconfirmed facts", () => {
  const ld = organizationLd({ ...company, rccm: "", niu: "", phone: "", poBox: "" }, en);
  assert.equal(ld.legalName, "HEKATE SYSTEMS SARL");
  for (const k of ["identifier", "taxID", "telephone"]) assert.ok(!(k in ld), `${k} must be absent`);
});

test("organization JSON-LD includes facts once supplied", () => {
  const ld = organizationLd({ ...company, rccm: "RC/DLA/2026/B/1", niu: "M0000", phone: "+237000" }, en) as Record<string, unknown>;
  assert.ok(ld.identifier && ld.taxID && ld.telephone && Array.isArray(ld.founder));
});

test("FAQPage covers every question in both languages", () => {
  for (const d of [en, fr]) {
    assert.equal((faqLd(d.services.faq).mainEntity as unknown[]).length, d.services.faq.length);
  }
});

test("no placeholder values reach the public pages", () => {
  const all = JSON.stringify([en, fr]);
  assert.ok(!/\[(NUMBER|PHONE|P\.O\. BOX)\]/.test(all), "placeholders such as [PHONE] must be omitted, not published");
});

function keyPaths(o: unknown, prefix = ""): string[] {
  if (Array.isArray(o)) return [`${prefix}[${o.length}]`, ...o.flatMap((x, i) => keyPaths(x, `${prefix}[${i}]`))];
  if (o && typeof o === "object") return Object.entries(o).flatMap(([k, v]) => keyPaths(v, `${prefix}.${k}`));
  return [prefix];
}

test("EN and FR have identical keys, list lengths and practice slugs", () => {
  const strip = (d: typeof en) => ({ ...d, lang: "", locale: "", guarantees: { ...d.guarantees, notice: "", intro: "", sections: [] } });
  assert.deepEqual(keyPaths(strip(fr)), keyPaths(strip(en)));
  assert.deepEqual(fr.services.practices.map((s) => s.slug), en.services.practices.map((s) => s.slug));
});

test("sitemap lists both languages with hreflang", () => {
  const x = sitemapXml("https://example.com", PAGES);
  assert.match(x, /<loc>https:\/\/example.com\/fr\/services\/<\/loc>/);
  assert.match(x, /hreflang="fr"/);
});
