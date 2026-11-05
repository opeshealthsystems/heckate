import * as Notifications from "expo-notifications";
import Constants from "expo-constants";
import { Platform } from "react-native";
import { api } from "./api";

Notifications.setNotificationHandler({
  handleNotification: async () => ({ shouldShowBanner: true, shouldShowList: true, shouldPlaySound: true, shouldSetBadge: false }),
});

/** Registers this device for push. Silently does nothing without permission or an EAS project id. */
export async function registerForPush(): Promise<void> {
  try {
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
