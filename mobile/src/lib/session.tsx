import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { router } from "expo-router";
import NetInfo from "@react-native-community/netinfo";
import { api, ApiError, setUnauthorizedHandler } from "./api";
import { clearToken, loadToken, setToken } from "./auth";
import { classify, flushDrafts, loadDrafts, saveDrafts } from "./drafts";
import { registerForPush } from "./push";
import type { Me } from "./types";

interface Session {
  me: Me | null;
  ready: boolean;
  pendingDrafts: number;
  signIn: (token: string) => Promise<void>;
  signOut: () => Promise<void>;
  syncDrafts: () => Promise<void>;
  refreshDrafts: () => Promise<void>;
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

  const refreshDrafts = useCallback(async () => setPending((await loadDrafts()).length), []);

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
    router.replace("/(tabs)/tickets");
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
      setReady(true);
    })();
  }, [signOut, refreshDrafts]);

  useEffect(() => {
    if (!me) return;
    const unsub = NetInfo.addEventListener((s) => {
      if (s.isConnected) void syncDrafts();
    });
    return unsub;
  }, [me, syncDrafts]);

  return <Ctx.Provider value={{ me, ready, pendingDrafts, signIn, signOut, syncDrafts, refreshDrafts }}>{children}</Ctx.Provider>;
}
