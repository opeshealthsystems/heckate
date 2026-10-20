import { test } from "node:test";
import assert from "node:assert/strict";
import { unpricedNote } from "./labour.ts";

test("nothing to report when every hour is priced", () => {
  assert.equal(unpricedNote(0), null);
  assert.equal(unpricedNote(-1), null);
});
test("reports unpriced hours instead of guessing", () => {
  assert.equal(unpricedNote(3), "3 approved hours have no rate, so they are not in the actual cost.");
  assert.equal(unpricedNote(1), "1 approved hour has no rate, so it is not in the actual cost.");
  assert.match(unpricedNote(7.5)!, /^7\.5 approved hours/);
});
