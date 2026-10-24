import { test } from "node:test";
import assert from "node:assert/strict";
import { flushQueue, newKey, outcome, type QueuedAction } from "./queue-core.ts";

const a = (path: string): QueuedAction => ({ key: newKey("t"), method: "POST", path, body: {}, createdAt: 0, label: path });

test("keys are unique", () => {
  assert.notEqual(newKey("x"), newKey("x"));
});

test("a check-out never overtakes a check-in that has to wait", async () => {
  const sent: string[] = [];
  const { left } = await flushQueue([a("/check-in"), a("/check-out")], async (x) => {
    sent.push(x.path);
    return "retry";
  });
  assert.deepEqual(sent, ["/check-in"]);
  assert.deepEqual(left.map((x) => x.path), ["/check-in", "/check-out"]);
});

test("sent actions leave the queue; permanent failures are dropped and reported", async () => {
  const plan: Record<string, "sent" | "drop"> = { "/a": "sent", "/b": "drop", "/c": "sent" };
  const { left, dropped } = await flushQueue([a("/a"), a("/b"), a("/c")], async (x) => plan[x.path]);
  assert.equal(left.length, 0);
  assert.deepEqual(dropped.map((x) => x.path), ["/b"]);
});

test("a retryable failure in the middle keeps the rest in order", async () => {
  const plan: Record<string, "sent" | "retry"> = { "/a": "sent", "/b": "retry", "/c": "sent" };
  const { left } = await flushQueue([a("/a"), a("/b"), a("/c")], async (x) => plan[x.path]);
  assert.deepEqual(left.map((x) => x.path), ["/b", "/c"]);
});

test("status mapping", () => {
  assert.equal(outcome(null), "retry");
  assert.equal(outcome(500), "retry");
  assert.equal(outcome(409), "drop");
  assert.equal(outcome(403), "drop");
  assert.equal(outcome(200), "sent");
});
