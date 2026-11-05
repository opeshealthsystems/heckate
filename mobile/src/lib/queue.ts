import AsyncStorage from "@react-native-async-storage/async-storage";
import type { QueuedAction } from "./queue-core";

const KEY = "hekate.queue";

export async function loadQueue(): Promise<QueuedAction[]> {
  try {
    return JSON.parse((await AsyncStorage.getItem(KEY)) ?? "[]");
  } catch {
    return [];
  }
}

export async function saveQueue(q: QueuedAction[]) {
  await AsyncStorage.setItem(KEY, JSON.stringify(q));
}

export async function enqueue(a: QueuedAction) {
  await saveQueue([...(await loadQueue()), a]);
}
