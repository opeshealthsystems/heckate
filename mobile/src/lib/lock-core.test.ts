import test from "node:test";
import assert from "node:assert/strict";
import { canEnforce, capability, LOCK_AFTER_MS, shouldLockOnResume } from "./lock-core.ts";

test("locks only after 60 s in the background", () => {
  assert.equal(LOCK_AFTER_MS, 60_000);
  assert.equal(shouldLockOnResume(1000, 1000 + 59_999, true), false);
  assert.equal(shouldLockOnResume(1000, 1000 + 60_000, true), true);
  assert.equal(shouldLockOnResume(1000, 1000 + 3_600_000, true), true);
});

test("never locks when disabled or when the app was not backgrounded", () => {
  assert.equal(shouldLockOnResume(0, 10 ** 9, false), false);
  assert.equal(shouldLockOnResume(null, 10 ** 9, true), false);
});

test("capability falls back from biometrics to the device passcode to nothing", () => {
  assert.equal(capability(2, true), "biometric");
  assert.equal(capability(1, false), "passcode");
  assert.equal(capability(0, false), "none");
  assert.equal(canEnforce("none"), false);
  assert.equal(canEnforce("passcode"), true);
});
