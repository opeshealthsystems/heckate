import { test } from "node:test";
import assert from "node:assert/strict";
import { classify, flushDrafts, type Draft, type SendResult } from "./drafts-core.ts";

const d = (key: string): Draft => ({ key, createdAt: 0, body: { subject: key, description: "", severity: "P3", channel: "app" } });

test("classify", () => {
  assert.equal(classify(null), "retry");
  assert.equal(classify(503), "retry");
  assert.equal(classify(429), "retry");
  assert.equal(classify(422), "drop");
  assert.equal(classify(401), "drop");
  assert.equal(classify(201), "sent");
});

test("all sent leaves nothing", async () => {
  assert.deepEqual(await flushDrafts([d("a"), d("b")], async () => "sent"), []);
});

test("permanent failures are dropped, not retried forever", async () => {
  assert.deepEqual(await flushDrafts([d("a"), d("b")], async () => "drop"), []);
});

test("stops at first retryable failure and keeps order", async () => {
  const seen: string[] = [];
  const plan: Record<string, SendResult> = { a: "sent", b: "retry", c: "sent" };
  const left = await flushDrafts([d("a"), d("b"), d("c")], async (x) => (seen.push(x.key), plan[x.key]));
  assert.deepEqual(seen, ["a", "b"]);
  assert.deepEqual(left.map((x) => x.key), ["b", "c"]);
});
