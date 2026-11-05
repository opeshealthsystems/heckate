import { test } from "node:test";
import assert from "node:assert/strict";
import { xaf } from "./format.ts";

test("XAF has no decimals and groups thousands", () => {
  assert.match(xaf(1192500), /^1.192.500 XAF$/);
  assert.equal(xaf(null), "–");
  assert.match(xaf(0), /^0 XAF$/);
});
