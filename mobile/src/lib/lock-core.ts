// Pure timing rules of the app lock (the native prompt itself lives in lock.tsx).

export const LOCK_AFTER_MS = 60_000;

/** True when returning to the foreground at `now` must show the lock screen. */
export function shouldLockOnResume(backgroundedAt: number | null, now: number, enabled: boolean, afterMs = LOCK_AFTER_MS): boolean {
  if (!enabled || backgroundedAt === null) return false;
  return now - backgroundedAt >= afterMs;
}

export type Capability = "biometric" | "passcode" | "none";

/**
 * What the device can offer. Security level 0 means no screen lock at all.
 * `enrolledBiometric` is true when a fingerprint or face is enrolled; otherwise a device passcode is the fallback.
 */
export function capability(level: number, enrolledBiometric: boolean): Capability {
  if (enrolledBiometric) return "biometric";
  return level > 0 ? "passcode" : "none";
}

/** The lock can only be enforced when the device itself has a screen lock. */
export const canEnforce = (c: Capability) => c !== "none";
