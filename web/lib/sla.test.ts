import { test } from "node:test";
import assert from "node:assert/strict";
import { slaRemaining } from "./sla.ts";

const now = Date.parse("2026-10-19T10:00:00Z");

test("time left", () => {
  assert.deepEqual(slaRemaining("2026-10-19T12:30:00Z", now), { label: "2h 30m left", overdue: false });
});
test("overdue", () => {
  assert.deepEqual(slaRemaining("2026-10-19T09:15:00Z", now), { label: "45m overdue", overdue: true });
});
test("days for long targets", () => {
  assert.equal(slaRemaining("2026-10-22T10:00:00Z", now)?.label, "3d left");
});
test("no due date", () => {
  assert.equal(slaRemaining(null, now), null);
});
