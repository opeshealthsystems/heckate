import test from "node:test";
import assert from "node:assert/strict";
import { diffLines, versionText } from "./diff.ts";

test("identical texts have no changes", () => {
  assert.deepEqual(diffLines("a\nb", "a\nb").map((l) => l.kind), ["same", "same"]);
});
test("an edited line shows as one removal and one addition", () => {
  assert.deepEqual(diffLines("a\nold\nc", "a\nnew\nc"), [
    { kind: "same", text: "a" }, { kind: "del", text: "old" }, { kind: "add", text: "new" }, { kind: "same", text: "c" },
  ]);
});
test("added and removed lines", () => {
  assert.deepEqual(diffLines("a", "a\nb").map((l) => l.kind), ["same", "add"]);
  assert.deepEqual(diffLines("a\nb", "b").map((l) => l.kind), ["del", "same"]);
});
test("version text includes title, signer and headed sections", () => {
  assert.equal(versionText({ title: "T", signed_by: "S", sections: [{ heading: "H", body: "B" }] }), "Title: T\nSigned for HEKATE by: S\n\n## H\nB");
});
