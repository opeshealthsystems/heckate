import test from "node:test";
import assert from "node:assert/strict";
import { slaLine, slaRemaining, slaRing, span } from "./sla.ts";

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

test("span formatting: hours and minutes, days beyond 48 h, never negative", () => {
  assert.equal(span(3 * 3_600_000 + 59 * 60_000), "3 h 59 m");
  assert.equal(span(-(72 * 60_000)), "1 h 12 m");
  assert.equal(span(12 * 60_000), "12 m");
  assert.equal(span(0), "0 m");
  assert.equal(span(76 * 3_600_000), "3 d 4 h");
});

test("sla line: neutral, orange when at risk, red and 'Overdue by' once breached", () => {
  const at = (h: number) => Date.parse("2026-01-01T00:00:00Z") + h * 3_600_000;
  assert.deepEqual(slaLine(T, at(1)), { text: "7 h left", tone: "label2" });
  assert.deepEqual(slaLine(T, at(4) + 60_000), { text: "3 h 59 m left", tone: "orange" });
  assert.deepEqual(slaLine(T, at(9.2)), { text: "Overdue by 1 h 12 m", tone: "red" });
  assert.deepEqual(slaLine({ ...T, sla_breached: true }, at(1)), { text: "Deadline breached", tone: "red" });
  assert.equal(slaLine({ ...T, sla_due_at: null }, at(1)), null);
});

test("sla line: short form for trailing values", () => {
  const at = (h: number) => Date.parse("2026-01-01T00:00:00Z") + h * 3_600_000;
  assert.deepEqual(slaLine(T, at(9.2), true), { text: "Overdue 1 h 12 m", tone: "red" });
  assert.deepEqual(slaLine(T, at(1), true), { text: "7 h", tone: "label2" });
});

test("sla line: resolved tickets say whether the deadline was met", () => {
  assert.deepEqual(slaLine({ ...T, resolved_at: "2026-01-01T04:00:00Z" }, 0), { text: "Resolved within its deadline", tone: "label2" });
  assert.deepEqual(slaLine({ ...T, resolved_at: "2026-01-01T10:00:00Z", sla_breached: true }, 0), { text: "Resolved after its deadline", tone: "red" });
});
