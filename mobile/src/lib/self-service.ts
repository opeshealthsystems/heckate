import { useCallback, useState } from "react";
import { api, ApiError, replay } from "./api";
import { classify } from "./drafts-core";
import { haptic } from "./haptics";
import { enqueue } from "./queue";
import { newKey } from "./queue-core";
import { useSession } from "./session";

export interface Outcome { text: string; ok: boolean }

/**
 * Time, leave and expense entry: sends now, or keeps the request on the phone until there is a connection.
 * (The same behaviour as before the redesign; only the screens moved.)
 */
export function useSelfService() {
  const { refreshQueue } = useSession();
  const [out, setOut] = useState<Outcome | null>(null);
  const [lastClaim, setLastClaim] = useState<string | null>(null);

  /** Sends now or keeps it on the phone until there is a connection. */
  const submit = useCallback(async (label: string, path: string, body: Record<string, unknown>, done: () => void) => {
    setOut(null);
    const action = { key: newKey("self"), method: "POST" as const, path, body, createdAt: Date.now(), label };
    try {
      const r = await replay(action);
      if (r === "retry") {
        await enqueue(action);
        await refreshQueue();
        haptic.warning();
        setOut({ text: `${label} saved on this phone and will be sent when you are online.`, ok: true });
        done();
      } else if (r === "drop") {
        haptic.error();
        setOut({ text: `${label} was refused by the server. Check the dates and amounts.`, ok: false });
      } else {
        haptic.success();
        setOut({ text: `${label} sent.`, ok: true });
        done();
      }
    } catch (e) {
      haptic.error();
      setOut({ text: e instanceof ApiError ? e.message : (e as Error).message, ok: false });
    }
  }, [refreshQueue]);

  /** Expense claims keep the new claim's id when sent online, so a receipt photo can be attached to it. */
  const submitClaim = useCallback(async (body: { expense_date: string; category: string; description: string; amount: number }, done: () => void) => {
    setOut(null);
    const key = newKey("self");
    try {
      const c = await api.claimExpense(body, key);
      haptic.success();
      setLastClaim(c.id);
      setOut({ text: "Expense claim sent. You can attach a photo of the receipt below.", ok: true });
      done();
    } catch (e) {
      if (classify(e instanceof ApiError ? e.status : null) === "retry") {
        await enqueue({ key, method: "POST", path: "/v1/expenses", body, createdAt: Date.now(), label: "Expense claim" });
        await refreshQueue();
        haptic.warning();
        setLastClaim(null);
        setOut({ text: "Expense claim saved on this phone and will be sent when you are online.", ok: true });
        done();
      } else {
        haptic.error();
        setOut({ text: e instanceof ApiError ? e.message : (e as Error).message, ok: false });
      }
    }
  }, [refreshQueue]);

  return { out, lastClaim, submit, submitClaim };
}
