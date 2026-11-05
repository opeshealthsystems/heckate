// Offline drafts: tickets reported without a connection are queued with a stable idempotency key
// and sent when the network returns. Retrying is safe because the API replays a repeated key.
import AsyncStorage from "@react-native-async-storage/async-storage";
import type { Draft, NewTicket } from "./drafts-core";

export { classify, flushDrafts } from "./drafts-core";
export type { Draft, NewTicket, SendResult } from "./drafts-core";

const KEY = "hekate.drafts";

export async function loadDrafts(): Promise<Draft[]> {
  try {
    return JSON.parse((await AsyncStorage.getItem(KEY)) ?? "[]");
  } catch {
    return [];
  }
}

export async function saveDrafts(d: Draft[]) {
  await AsyncStorage.setItem(KEY, JSON.stringify(d));
}

export async function addDraft(body: NewTicket): Promise<Draft> {
  const draft: Draft = { key: `draft-${Date.now()}-${Math.random().toString(36).slice(2)}`, body, createdAt: Date.now() };
  await saveDrafts([...(await loadDrafts()), draft]);
  return draft;
}
