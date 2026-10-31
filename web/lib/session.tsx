"use client";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "./api";
import { clearToken, getToken } from "./auth";
import type { Me } from "./types";

const Ctx = createContext<Me | null>(null);
export const useMe = () => {
  const me = useContext(Ctx);
  if (!me) throw new Error("useMe outside session");
  return me;
};

export function SessionGate({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [err, setErr] = useState("");
  const router = useRouter();
  useEffect(() => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    api.me().then(setMe).catch((e) => {
      // Only a rejected token ends the session. A network error must not sign the user out.
      if (e instanceof ApiError && e.status === 401) {
        clearToken();
        router.replace("/login");
        return;
      }
      setErr(String(e.message ?? e));
    });
  }, [router]);
  if (err) return <p className="error pad" role="alert">{err} <button className="btn" onClick={() => location.reload()}>Retry</button></p>;
  if (!me) return <p className="muted pad">Loading…</p>;
  return <Ctx.Provider value={me}>{children}</Ctx.Provider>;
}
