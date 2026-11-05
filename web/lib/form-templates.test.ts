import test from "node:test";
import assert from "node:assert/strict";
import { cleanAnswers, FORM_TEMPLATES } from "./form-templates.ts";

test("every template has unique question keys and select questions have options", () => {
  for (const t of FORM_TEMPLATES) {
    const keys = t.fields.map((f) => f.key);
    assert.equal(new Set(keys).size, keys.length, t.title);
    for (const f of t.fields) {
      assert.match(f.key, /^[a-z][a-z0-9_]{0,39}$/, f.key);
      if (f.type === "select") assert.ok((f.options ?? []).length > 0, f.key);
    }
  }
});

test("answers are sent as the API expects", () => {
  const fields = FORM_TEMPLATES[0].fields;
  const out = cleanAnswers(fields, { site_name: "Douala DC", racks: "12", ups: true, notes: "", power: "Solar" });
  assert.equal(out.racks, 12);
  assert.equal(out.site_name, "Douala DC");
  assert.equal(out.ups, true);
  assert.equal("notes" in out, false, "empty text is left out");
  assert.equal(cleanAnswers(fields, {}).ups, false, "an unticked box is still an answer");
});
