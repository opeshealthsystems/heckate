// Pure offline-action queue logic (no React Native imports) so it can be tested in Node.
import { classify, type SendResult } from "./drafts-core.ts";

export interface QueuedAction {
  key: string; // Idempotency-Key: replaying a sent action is harmless
  method: "POST" | "PUT" | "PATCH";
  path: string;
  body: unknown;
  createdAt: number;
  label: string; // shown to the user while it waits
}

export function newKey(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

/**
 * Replays actions in order. A retryable failure stops the run so later actions never overtake earlier
 * ones (a check-out must not be sent before its check-in). Permanent failures are dropped and reported.
 */
export async function flushQueue(
  items: QueuedAction[],
  send: (a: QueuedAction) => Promise<SendResult>,
): Promise<{ left: QueuedAction[]; dropped: QueuedAction[] }> {
  const left: QueuedAction[] = [];
  const dropped: QueuedAction[] = [];
  for (let i = 0; i < items.length; i++) {
    const r = await send(items[i]);
    if (r === "retry") {
      left.push(...items.slice(i));
      break;
    }
    if (r === "drop") dropped.push(items[i]);
  }
  return { left, dropped };
}

/** Maps an HTTP status (or null for a network failure) to what the queue should do. */
export const outcome = (status: number | null): SendResult => classify(status);
