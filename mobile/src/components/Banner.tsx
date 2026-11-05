import { View } from "react-native";
import { useNetInfo } from "@react-native-community/netinfo";
import { radius, sp, useTheme, type Colors } from "../lib/theme";
import { useSession } from "../lib/session";
import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";

/** A quiet inline notice on the grouped surface: icon, one or two lines. Errors are red text; everything else is neutral. */
export function Banner({ icon = "info", tone = "label2", children }: { icon?: IconName; tone?: keyof Colors; children: string }) {
  const { c } = useTheme();
  return (
    <View accessibilityRole={tone === "red" ? "alert" : undefined} style={{ backgroundColor: c.surface, borderRadius: radius.group, paddingHorizontal: sp.lg, paddingVertical: sp.md, flexDirection: "row", alignItems: "flex-start", gap: sp.md }}>
      <Icon name={icon} tone={tone} size={22} />
      <Text v="subhead" tone={tone === "red" ? "red" : "label2"} style={{ flex: 1 }}>{children}</Text>
    </View>
  );
}

/** Shown only when the phone is offline or updates are waiting to send. Otherwise nothing. */
export function SyncBanner() {
  const net = useNetInfo();
  const { pendingDrafts, pendingActions } = useSession();
  const waiting = pendingDrafts + pendingActions;
  const offline = net.isConnected === false;
  if (!offline && !waiting) return null;
  const text = offline
    ? `You are offline. Changes are saved on this phone and sent when you are back online.${waiting ? ` ${waiting} waiting.` : ""}`
    : `${waiting} update${waiting === 1 ? "" : "s"} waiting to send. They go out automatically.`;
  return <Banner icon={offline ? "offline" : "sync"}>{text}</Banner>;
}

/** Inline error with a retry action, as a banner. */
export function ErrorBanner({ message }: { message: string }) {
  return <Banner icon="warning" tone="red">{message}</Banner>;
}
