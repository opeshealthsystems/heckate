import { Pressable, StyleSheet, Text, View } from "react-native";
import type { BottomTabBarProps } from "expo-router/build/layouts/Tabs";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { haptic } from "../lib/haptics";
import { useSession } from "../lib/session";
import { font, useTheme } from "../lib/theme";
import { Icon, type IconName } from "./Icon";

export const TAB_ICONS: Record<string, IconName> = {
  home: "radar", tickets: "tickets", report: "alert", field: "wrench", documents: "docCheck", account: "fingerprint",
};

/** Custom tab bar: deep surface, gold indicator on the active tab, count badge for work waiting to sync. */
export function TabBar({ state, descriptors, emitter, navigateToTab }: BottomTabBarProps) {
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  const { pendingDrafts, pendingActions } = useSession();
  const queued = pendingDrafts + pendingActions;
  return (
    <View style={{ backgroundColor: c.void, borderTopColor: c.line, borderTopWidth: StyleSheet.hairlineWidth * 2, paddingBottom: Math.max(insets.bottom, 6), paddingTop: 2, flexDirection: "row" }}>
      {state.routes.map((route, i) => {
        const { options } = descriptors[route.key];
        if ((options as { hidden?: boolean }).hidden) return null;
        const focused = state.index === i;
        const label = String(options.title ?? route.name);
        const color = focused ? c.goldText : c.muted;
        const badge = route.name === "account" && queued > 0 ? queued : 0;
        return (
          <Pressable
            key={route.key}
            accessibilityRole="tab"
            accessibilityLabel={badge ? `${label}, ${badge} waiting to sync` : label}
            accessibilityState={{ selected: focused }}
            onPress={() => {
              const e = emitter.emit({ type: "tabPress", target: route.key, canPreventDefault: true });
              if (!focused && !e.defaultPrevented) { haptic.select(); navigateToTab(route.key); }
            }}
            onLongPress={() => emitter.emit({ type: "tabLongPress", target: route.key })}
            style={{ flex: 1, minHeight: 52, alignItems: "center", justifyContent: "center", gap: 3 }}
          >
            <View style={{ position: "absolute", top: 0, width: focused ? 28 : 0, height: 2, backgroundColor: c.gold, borderRadius: 2 }} />
            <View>
              <Icon name={TAB_ICONS[route.name] ?? "home"} size={23} color={color} stroke={focused ? 2 : 1.6} />
              {badge > 0 && (
                <View style={{ position: "absolute", top: -4, right: -10, minWidth: 16, height: 16, borderRadius: 8, paddingHorizontal: 4, backgroundColor: c.warn, alignItems: "center", justifyContent: "center" }}>
                  <Text style={{ color: c.onGold, fontSize: 10, fontFamily: font.monoBold }}>{badge}</Text>
                </View>
              )}
            </View>
            <Text numberOfLines={1} style={{ color, fontSize: 10.5, letterSpacing: 0.4, fontFamily: focused ? font.semibold : font.medium, textTransform: "uppercase" }}>{label}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}
