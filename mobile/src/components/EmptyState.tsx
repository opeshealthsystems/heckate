import { ActivityIndicator, View } from "react-native";
import { sp } from "../lib/theme";
import { Button } from "./Button";
import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";

/** Centred icon, a headline, one sentence and at most one action. */
export function EmptyState({ icon, title, text, action }: { icon: IconName; title: string; text?: string; action?: { title: string; onPress: () => void; primary?: boolean } }) {
  return (
    <View style={{ alignItems: "center", gap: sp.sm, paddingVertical: sp.xxl, paddingHorizontal: sp.lg }}>
      <Icon name={icon} size={44} tone="label3" />
      <Text v="title" style={{ textAlign: "center", marginTop: sp.sm }} accessibilityRole="header">{title}</Text>
      {!!text && <Text v="subhead" tone="label2" style={{ textAlign: "center", maxWidth: 320 }}>{text}</Text>}
      {action && <Button title={action.title} kind={action.primary ? "primary" : "secondary"} onPress={action.onPress} style={action.primary ? { alignSelf: "stretch", marginTop: sp.lg } : { marginTop: sp.sm }} />}
    </View>
  );
}

/** Inline loading: a native spinner on the grouped background. */
export function Loading() {
  return <View style={{ paddingVertical: sp.xxl, alignItems: "center" }}><ActivityIndicator /></View>;
}
