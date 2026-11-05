import * as Notifications from "expo-notifications";
import Constants from "expo-constants";
import { Platform } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { api } from "./api";

const PREF = "hekate.push.enabled";

/** Whether the person wants push notifications on this phone (default on). */
export async function pushEnabled(): Promise<boolean> {
  try { return (await AsyncStorage.getItem(PREF)) !== "0"; } catch { return true; }
}

Notifications.setNotificationHandler({
  handleNotification: async () => ({ shouldShowBanner: true, shouldShowList: true, shouldPlaySound: true, shouldSetBadge: false }),
});

/** Registers this device for push. Silently does nothing without permission or an EAS project id. */
export async function registerForPush(): Promise<void> {
  try {
    if (!(await pushEnabled())) return;
    const perm = await Notifications.requestPermissionsAsync();
    if (!perm.granted) return;
    const projectId = Constants.expoConfig?.extra?.eas?.projectId as string | undefined;
    if (!projectId) return; // see docs/BLOCKERS.md: needs an EAS project and FCM/APNs credentials
    const t = await Notifications.getExpoPushTokenAsync({ projectId });
    await api.registerDevice(t.data, "expo");
    if (Platform.OS === "android") {
      await Notifications.setNotificationChannelAsync("default", { name: "Support updates", importance: Notifications.AndroidImportance.HIGH });
    }
  } catch {
    /* push is best effort */
  }
}

/** Turns push on or off for this phone. Off removes the device from the server, so nothing is sent to it. */
export async function setPushEnabled(on: boolean): Promise<string | null> {
  await AsyncStorage.setItem(PREF, on ? "1" : "0").catch(() => {});
  try {
    if (on) {
      const perm = await Notifications.requestPermissionsAsync();
      if (!perm.granted) return "Notifications are blocked for HEKATE. Allow them in the phone's Settings.";
      await registerForPush();
      return null;
    }
    const projectId = Constants.expoConfig?.extra?.eas?.projectId as string | undefined;
    if (projectId) {
      const t = await Notifications.getExpoPushTokenAsync({ projectId });
      await api.unregisterDevice(t.data);
    }
    return null;
  } catch {
    return on ? "Could not turn on notifications. Try again when you are online." : "Turned off on this phone; the server will stop once you are online.";
  }
}
