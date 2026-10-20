import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { router } from "expo-router";
import NetInfo from "@react-native-community/netinfo";
import { api, ApiError, replay, setUnauthorizedHandler } from "./api";
import { clearToken, loadToken, setToken } from "./auth";
import { classify, flushDrafts, loadDrafts, saveDrafts } from "./drafts";
import { flushQueue } from "./queue-core";
import { loadQueue, saveQueue } from "./queue";
import { registerForPush } from "./push";
import type { Me } from "./types";

interface Session {
  me: Me | null;
  ready: boolean;
  pendingDrafts: number;
  pendingActions: number;
  signIn: (token: string) => Promise<void>;
  signOut: () => Promise<void>;
  syncDrafts: () => Promise<void>;
  refreshDrafts: () => Promise<void>;
  refreshQueue: () => Promise<void>;
}
const Ctx = createContext<Session | null>(null);
export const useSession = () => {
  const s = useContext(Ctx);
  if (!s) throw new Error("useSession outside provider");
  return s;
};

export function SessionProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [ready, setReady] = useState(false);
  const [pendingDrafts, setPending] = useState(0);
  const [pendingActions, setPendingActions] = useState(0);

  const refreshDrafts = useCallback(async () => setPending((await loadDrafts()).length), []);
  const refreshQueue = useCallback(async () => setPendingActions((await loadQueue()).length), []);

  const syncDrafts = useCallback(async () => {
    const drafts = await loadDrafts();
    if (!drafts.length) return;
    const left = await flushDrafts(drafts, async (d) => {
      try {
        await api.createTicket(d.body, d.key);
        return "sent";
      } catch (e) {
        return classify(e instanceof ApiError ? e.status : null);
      }
    });
    await saveDrafts(left);
    setPending(left.length);
    // Field actions (check-in, check-out, time, leave) go after tickets, each in the order it happened.
    const q = await loadQueue();
    if (q.length) {
      const res = await flushQueue(q, replay);
      await saveQueue(res.left);
      setPendingActions(res.left.length);
    }
  }, []);

  const signOut = useCallback(async () => {
    await clearToken();
    setMe(null);
    router.replace("/login");
  }, []);

  const signIn = useCallback(async (token: string) => {
    await setToken(token);
    try {
      setMe(await api.me());
    } catch (e) {
      await clearToken();
      throw e;
    }
    void registerForPush();
    router.replace("/(tabs)/home");
  }, []);

  useEffect(() => {
    setUnauthorizedHandler(() => void signOut());
    (async () => {
      const t = await loadToken();
      if (t) {
        try {
          setMe(await api.me());
          void registerForPush();
        } catch (e) {
          // offline at launch keeps the session; a rejected token does not
          if (e instanceof ApiError && e.status !== null) await clearToken();
        }
      }
      await refreshDrafts();
      await refreshQueue();
      setReady(true);
    })();
  }, [signOut, refreshDrafts, refreshQueue]);

  useEffect(() => {
    if (!me) return;
    const unsub = NetInfo.addEventListener((s) => {
      if (s.isConnected) void syncDrafts();
    });
    return unsub;
  }, [me, syncDrafts]);

  return <Ctx.Provider value={{ me, ready, pendingDrafts, pendingActions, signIn, signOut, syncDrafts, refreshDrafts, refreshQueue }}>{children}</Ctx.Provider>;
}
