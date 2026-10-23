import { Platform, Pressable } from "react-native";
import { Redirect, usePathname } from "expo-router";
import type { ReactElement } from "react";
import { useSession } from "../lib/session";
import { useTheme } from "../lib/theme";
import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";

/** A nav-bar button: text ("Cancel") or an icon ("+"), tinted, with a 44 pt hit area. */
export function HeaderButton({ title, icon, label, onPress }: { title?: string; icon?: IconName; label: string; onPress: () => void }) {
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={label} onPress={onPress} hitSlop={12} style={({ pressed }) => ({ minWidth: 44, minHeight: 44, alignItems: "center", justifyContent: "center", opacity: pressed && Platform.OS !== "android" ? 0.5 : 1 })} android_ripple={{ color: "#00000014", borderless: true, radius: 22 }}>
      {icon ? <Icon name={icon} size={24} tone="tint" /> : <Text tone="tint">{title}</Text>}
    </Pressable>
  );
}

/** Screens outside the tab navigator: send a signed-out visitor to sign-in and bring them back (?next=). */
export function useGate(): ReactElement | null {
  const { me, ready } = useSession();
  const pathname = usePathname();
  if (ready && !me) return <Redirect href={{ pathname: "/login", params: { next: pathname } }} />;
  return null;
}

/** Common navigation options: tinted back button, grouped background. */
export function useStackOptions() {
  const { c } = useTheme();
  return {
    headerTintColor: c.tint,
    headerShadowVisible: false,
    headerStyle: Platform.OS === "android" ? { backgroundColor: c.bg } : undefined,
    contentStyle: { backgroundColor: c.bg },
  } as const;
}
