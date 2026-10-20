import test from "node:test";
import assert from "node:assert/strict";
import { packHeadings, withPackHeadings } from "./pack-sections.ts";

test("every pack form the editor fills in starts from the pack's own headings", () => {
  for (const n of ["06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "23", "24", "26", "31", "32", "33", "37", "39", "41"]) {
    assert.ok(packHeadings(`HS-FRM-${n}`).length > 0, `HS-FRM-${n}`);
  }
  for (const c of ["HS-PR", "HS-OC", "HS-SOW"]) assert.ok(packHeadings(c).length > 0, c);
  assert.deepEqual(packHeadings("HS-RPT"), [], "the penetration test report has no pack layout");
});

test("headings carry English and French as the pack prints them", () => {
  assert.ok(packHeadings("HS-FRM-07").includes("Trading name / Nom commercial"));
  assert.ok(packHeadings("HS-FRM-26").includes("Risks, decisions and notes / Risques, décisions, notes"));
  assert.ok(packHeadings("HS-FRM-24").includes("Rates"));
});

test("new documents get every heading empty; drafts keep their text and gain the missing headings", () => {
  const fresh = withPackHeadings("HS-FRM-31", []);
  assert.equal(fresh.length, packHeadings("HS-FRM-31").length);
  assert.ok(fresh.every((s) => s.body === ""));
  const kept = withPackHeadings("HS-FRM-31", [{ heading: "lessons learned / Retour d’expérience", body: "Good." }, { heading: "My own", body: "Extra." }]);
  assert.equal(kept[0].body, "Good.");
  assert.equal(kept[1].heading, "My own");
  assert.equal(kept.filter((s) => /lessons learned/i.test(s.heading)).length, 1, "a heading already there is not added twice");
  assert.ok(kept.length > 2);
});

test("a form without a pack layout starts with one untitled section", () => {
  assert.deepEqual(withPackHeadings("HS-RPT", []), [{ heading: "", body: "" }]);
});
