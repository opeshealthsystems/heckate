import * as Haptics from "expo-haptics";
import { Platform } from "react-native";

// Haptics are a nicety: never let them throw (web, simulators, devices without a taptic engine).
const run = (f: () => Promise<void>) => {
  if (Platform.OS === "web") return;
  f().catch(() => {});
};

export const haptic = {
  /** a light tap for ordinary presses and tab changes */
  tap: () => run(() => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light)),
  /** a firmer tap for a committing action (check in, send, sign) */
  press: () => run(() => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium)),
  select: () => run(() => Haptics.selectionAsync()),
  success: () => run(() => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success)),
  warning: () => run(() => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning)),
  error: () => run(() => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error)),
};
