import type { ReactNode } from "react";
import { KeyboardAvoidingView, Platform, RefreshControl, ScrollView, View, type StyleProp, type ViewStyle } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { sp, useTheme } from "../lib/theme";
import { Text } from "./Text";

/**
 * The screen scaffold: grouped background, 16 pt margins, 24 pt between sections, a column that stops at 640 pt on tablets.
 * On iOS the native header's large title collapses with this scroll view (contentInsetAdjustmentBehavior).
 * `webTitle` draws the large title inline ONLY in the web preview, where the native header is hidden.
 */
export function Screen({ children, refreshing, onRefresh, webTitle, style, keyboard }: { children: ReactNode; refreshing?: boolean; onRefresh?: () => void; webTitle?: string; style?: StyleProp<ViewStyle>; keyboard?: boolean }) {
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  const scroll = (
    <ScrollView
      style={{ flex: 1, backgroundColor: c.bg }}
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode={Platform.OS === "ios" ? "interactive" : "on-drag"}
      refreshControl={onRefresh ? <RefreshControl refreshing={!!refreshing} onRefresh={onRefresh} /> : undefined}
      contentContainerStyle={{ paddingBottom: Platform.OS === "ios" ? sp.xl : insets.bottom + sp.xl, paddingTop: Platform.OS === "web" ? insets.top : 0 }}
    >
      <View style={[{ width: "100%", maxWidth: 640, alignSelf: "center", paddingHorizontal: sp.lg, paddingTop: sp.sm, gap: sp.xl }, style]}>
        {Platform.OS === "web" && !!webTitle && <Text v="largeTitle" accessibilityRole="header" style={{ paddingTop: sp.sm, paddingHorizontal: sp.xs }}>{webTitle}</Text>}
        {children}
      </View>
    </ScrollView>
  );
  if (!keyboard) return scroll;
  return <KeyboardAvoidingView style={{ flex: 1, backgroundColor: c.bg }} behavior={Platform.OS === "ios" ? "padding" : undefined}>{scroll}</KeyboardAvoidingView>;
}
