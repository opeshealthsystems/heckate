import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { AppState } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import * as LocalAuthentication from "expo-local-authentication";
import { canEnforce, capability, shouldLockOnResume, type Capability } from "./lock-core";
import { useSession } from "./session";

// The session token stays in SecureStore (auth.ts). The lock only decides whether the UI may be shown:
// at cold start and after 60 s in the background, the user must pass the device's biometric or passcode prompt.

const KEY = "hekate.lock.enabled";

interface LockState {
  /** the user's setting (default on) */
  enabled: boolean;
  /** what this device can do: biometric, passcode only, or nothing (then the lock cannot be enforced) */
  cap: Capability;
  locked: boolean;
  /** true while the app is inactive (app switcher): the UI is hidden from the snapshot */
  covered: boolean;
  error: string;
  unlock: () => Promise<boolean>;
  /** Turns the lock on or off; both need a successful prompt. Returns a message when it did not change. */
  setEnabled: (on: boolean) => Promise<string | null>;
}
const Ctx = createContext<LockState | null>(null);
export const useLock = () => {
  const v = useContext(Ctx);
  if (!v) throw new Error("useLock outside provider");
  return v;
};

async function probe(): Promise<Capability> {
  try {
    const [hw, enrolled, level] = await Promise.all([
      LocalAuthentication.hasHardwareAsync(),
      LocalAuthentication.isEnrolledAsync(),
      LocalAuthentication.getEnrolledLevelAsync(),
    ]);
    return capability(level, hw && enrolled);
  } catch {
    return "none";
  }
}

export function LockProvider({ children }: { children: ReactNode }) {
  const { me, ready } = useSession();
  const [enabled, setEnabledState] = useState(true);
  const [cap, setCap] = useState<Capability>("none");
  const [locked, setLocked] = useState(false);
  const [covered, setCovered] = useState(false);
  const [error, setError] = useState("");
  const backgroundedAt = useRef<number | null>(null);
  const prompting = useRef(false);
  const enabledRef = useRef(true);
  const capRef = useRef<Capability>("none");
  const started = useRef(false);

  const prompt = useCallback(async (message: string): Promise<boolean> => {
    if (prompting.current) return false;
    prompting.current = true;
    try {
      const r = await LocalAuthentication.authenticateAsync({ promptMessage: message, cancelLabel: "Cancel", fallbackLabel: "Use passcode", disableDeviceFallback: false });
      if (!r.success) setError(r.error === "user_cancel" || r.error === "system_cancel" || r.error === "app_cancel" ? "" : "That did not match. Try again.");
      else setError("");
      return r.success;
    } catch (e) {
      setError((e as Error).message);
      return false;
    } finally {
      prompting.current = false;
    }
  }, []);

  const unlock = useCallback(async () => {
    const ok = await prompt("Unlock HEKATE");
    if (ok) setLocked(false);
    return ok;
  }, [prompt]);

  // Cold start: once the stored session is known, lock before the first screen is readable.
  useEffect(() => {
    if (!ready || started.current) return;
    started.current = true;
    (async () => {
      const [stored, c] = await Promise.all([AsyncStorage.getItem(KEY).catch(() => null), probe()]);
      const on = stored !== "0";
      enabledRef.current = on;
      capRef.current = c;
      setEnabledState(on);
      setCap(c);
      if (me && on && canEnforce(c)) {
        setLocked(true);
        void unlock();
      }
    })();
  }, [ready, me, unlock]);

  useEffect(() => {
    const sub = AppState.addEventListener("change", (st) => {
      if (st === "active") {
        setCovered(false);
        const at = backgroundedAt.current;
        backgroundedAt.current = null;
        if (!prompting.current && canEnforce(capRef.current) && shouldLockOnResume(at, Date.now(), enabledRef.current)) {
          setLocked(true);
          void probe().then((c) => { capRef.current = c; setCap(c); });
          void unlock();
        }
      } else {
        setCovered(true);
        if (st === "background" && backgroundedAt.current === null) backgroundedAt.current = Date.now();
      }
    });
    return () => sub.remove();
  }, [unlock]);

  const setEnabled = useCallback(async (on: boolean): Promise<string | null> => {
    const c = await probe();
    capRef.current = c;
    setCap(c);
    if (!canEnforce(c)) return "This device has no screen lock. Set a passcode in the phone's settings first.";
    const ok = await prompt(on ? "Turn on the HEKATE app lock" : "Turn off the HEKATE app lock");
    if (!ok) return "Not changed: the device did not confirm it was you.";
    enabledRef.current = on;
    setEnabledState(on);
    AsyncStorage.setItem(KEY, on ? "1" : "0").catch(() => {});
    return null;
  }, [prompt]);

  const value = useMemo(() => ({ enabled, cap, locked: locked && !!me, covered, error, unlock, setEnabled }), [enabled, cap, locked, me, covered, error, unlock, setEnabled]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
