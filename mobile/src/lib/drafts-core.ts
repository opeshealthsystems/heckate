// Pure offline-queue logic (no React Native imports) so it can be unit tested in Node.
import type { Channel, Severity } from "./types";

export interface NewTicket {
  subject: string;
  description: string;
  severity: Severity;
  channel: Channel;
}
export interface Draft {
  key: string;
  body: NewTicket;
  createdAt: number;
}
export type SendResult = "sent" | "retry" | "drop";

/** Sends drafts in order. Stops at the first retryable failure so ordering is kept; returns what is left. */
export async function flushDrafts(drafts: Draft[], send: (d: Draft) => Promise<SendResult>): Promise<Draft[]> {
  const left: Draft[] = [];
  for (let i = 0; i < drafts.length; i++) {
    const r = await send(drafts[i]);
    if (r === "retry") {
      left.push(...drafts.slice(i));
      break;
    }
  }
  return left;
}

/** Network errors and 408/429/5xx are worth retrying; other 4xx will never succeed. */
export function classify(status: number | null): SendResult {
  if (status === null || status === 408 || status === 429 || status >= 500) return "retry";
  return status >= 400 ? "drop" : "sent";
}
