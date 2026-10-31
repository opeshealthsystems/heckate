import { ActionSheetIOS, Alert, Platform } from "react-native";

export interface Choice { label: string; onPress: () => void }

/** A native action sheet on iOS, a system alert with buttons on Android (3 choices at most including Cancel). */
export function chooseAction(title: string, choices: Choice[]): void {
  if (Platform.OS === "ios") {
    ActionSheetIOS.showActionSheetWithOptions({ title, options: [...choices.map((c) => c.label), "Cancel"], cancelButtonIndex: choices.length }, (i) => choices[i]?.onPress());
  } else if (Platform.OS === "web") {
    choices[0]?.onPress();
  } else {
    Alert.alert(title, undefined, [...choices.map((c) => ({ text: c.label, onPress: c.onPress })), { text: "Cancel", style: "cancel" as const }]);
  }
}

/** Destructive confirmation with the platform's own alert. */
export function confirmDestructive(title: string, message: string, confirmLabel: string, onConfirm: () => void): void {
  if (Platform.OS === "web") {
    if (typeof window !== "undefined" && window.confirm(`${title}\n${message}`)) onConfirm();
    return;
  }
  Alert.alert(title, message, [{ text: "Cancel", style: "cancel" }, { text: confirmLabel, style: "destructive", onPress: onConfirm }]);
}
