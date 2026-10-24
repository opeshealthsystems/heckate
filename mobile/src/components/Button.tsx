import { ActivityIndicator, Platform, Pressable, View, type StyleProp, type ViewStyle } from "react-native";
import { BUTTON_HEIGHT, MIN_TARGET, radius, sp, useTheme } from "../lib/theme";
import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";

export type ButtonKind = "primary" | "secondary" | "destructive";

/**
 * primary: filled with the brand tint, white label, 50 pt, full width. One per screen at most.
 * secondary: plain tinted text. destructive: red text (confirm with an Alert before acting).
 */
export function Button({ title, onPress, kind = "secondary", icon, loading, disabled, style, accessibilityHint }: { title: string; onPress: () => void; kind?: ButtonKind; icon?: IconName; loading?: boolean; disabled?: boolean; style?: StyleProp<ViewStyle>; accessibilityHint?: string }) {
  const { c } = useTheme();
  const primary = kind === "primary";
  const off = disabled || loading;
  const tone = primary ? "onTint" : kind === "destructive" ? "red" : "tint";
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={title}
      accessibilityHint={accessibilityHint}
      accessibilityState={{ disabled: !!off, busy: !!loading }}
      disabled={off}
      onPress={onPress}
      hitSlop={primary ? undefined : 8}
      android_ripple={{ color: primary ? "#FFFFFF33" : "#00000014" }}
      style={({ pressed }) => [
        {
          minHeight: primary ? BUTTON_HEIGHT : MIN_TARGET,
          borderRadius: radius.button,
          paddingHorizontal: sp.lg,
          alignItems: "center",
          justifyContent: "center",
          overflow: "hidden",
          backgroundColor: primary ? c.tintFill : undefined,
          opacity: disabled && !loading ? 0.4 : pressed && Platform.OS !== "android" ? 0.6 : 1,
        },
        style,
      ]}
    >
      <View style={{ flexDirection: "row", alignItems: "center", gap: sp.sm }}>
        {loading ? <ActivityIndicator color={primary ? (c.onTint as string) : undefined} /> : !!icon && <Icon name={icon} size={20} tone={tone} />}
        <Text v="headline" tone={tone} weight="semibold" numberOfLines={1} style={{ textAlign: "center" }}>{title}</Text>
      </View>
    </Pressable>
  );
}
