import test from "node:test";
import assert from "node:assert/strict";
import { arcDash, countdown, postureScore, slaRemaining, slaRing } from "./sla.ts";

test("formats time left and overdue", () => {
  const now = Date.parse("2026-01-01T12:00:00Z");
  assert.deepEqual(slaRemaining("2026-01-01T14:30:00Z", now), { label: "2h 30m left", overdue: false });
  assert.deepEqual(slaRemaining("2026-01-01T11:15:00Z", now), { label: "45m overdue", overdue: true });
  assert.deepEqual(slaRemaining("2026-01-04T12:00:00Z", now), { label: "3d left", overdue: false });
  assert.equal(slaRemaining(null, now), null);
});

const T = { created_at: "2026-01-01T00:00:00Z", sla_due_at: "2026-01-01T08:00:00Z", resolved_at: null, sla_breached: false };

test("ring: share used follows the clock and the state steps ok, warn, bad", () => {
  const at = (h: number) => Date.parse("2026-01-01T00:00:00Z") + h * 3_600_000;
  assert.deepEqual(slaRing(T, at(0)), { used: 0, remainingMs: 8 * 3_600_000, state: "ok" });
  assert.equal(slaRing(T, at(2)).used, 0.25);
  assert.equal(slaRing(T, at(2)).state, "ok");
  assert.equal(slaRing(T, at(5)).state, "warn"); // 62% used but 3 h left (< 4 h)
  assert.equal(slaRing(T, at(7)).state, "warn");
  assert.equal(slaRing(T, at(9)).state, "bad");
  assert.equal(slaRing(T, at(9)).used, 1);
});

test("ring: a long window warns at 75% used even with more than 4 h left", () => {
  const long = { ...T, sla_due_at: "2026-01-05T00:00:00Z" }; // 96 h
  assert.equal(slaRing(long, Date.parse("2026-01-03T00:00:00Z")).state, "ok");
  assert.equal(slaRing(long, Date.parse("2026-01-04T01:00:00Z")).state, "warn");
});

test("ring: resolved tickets are met or missed and the arc stops at the resolve time", () => {
  const met = slaRing({ ...T, resolved_at: "2026-01-01T04:00:00Z" }, Date.parse("2026-02-01T00:00:00Z"));
  assert.deepEqual(met, { used: 0.5, remainingMs: 4 * 3_600_000, state: "met" });
  const missed = slaRing({ ...T, resolved_at: "2026-01-01T10:00:00Z", sla_breached: true }, 0);
  assert.equal(missed.state, "missed");
  assert.equal(missed.used, 1);
});

test("ring: no due date, a flagged breach and a degenerate window", () => {
  assert.equal(slaRing({ ...T, sla_due_at: null }, 0).state, "none");
  assert.equal(slaRing({ ...T, sla_breached: true }, Date.parse("2026-01-01T01:00:00Z")).state, "bad");
  const z = slaRing({ ...T, sla_due_at: T.created_at }, Date.parse(T.created_at) + 5);
  assert.equal(z.used, 1);
});

test("countdown formatting", () => {
  assert.equal(countdown(4 * 3_600_000 + 12 * 60_000 + 9_000), "04:12:09");
  assert.equal(countdown(-61_000), "00:01:01");
  assert.equal(countdown(0), "00:00:00");
  assert.equal(countdown(76 * 3_600_000), "76:00:00");
  assert.equal(countdown(130 * 3_600_000), "5d 10h");
});

test("arc dash splits the circumference and clamps", () => {
  const a = arcDash(0.25, 10);
  assert.ok(Math.abs(a.circumference - 62.8319) < 1e-3);
  assert.ok(Math.abs(a.dash - 15.708) < 1e-3);
  assert.ok(Math.abs(a.dash + a.gap - a.circumference) < 1e-9);
  assert.equal(arcDash(2, 10).gap, 0);
  assert.equal(arcDash(-1, 10).dash, 0);
});

test("posture score", () => {
  assert.equal(postureScore(0, 0), null);
  assert.equal(postureScore(4, 1), 75);
  assert.equal(postureScore(3, 9), 0);
  assert.equal(postureScore(5, 0), 100);
});
