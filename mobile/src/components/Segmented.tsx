import type { ComponentType } from "react";
import { Platform, View, type ColorValue, type TextStyle } from "react-native";
import SegmentedControlLib from "@react-native-segmented-control/segmented-control";
import { haptic } from "../lib/haptics";
import { sp, useTheme } from "../lib/theme";

// The package ships class-component typings from an older React Native; this is the subset of props used here.
interface SegProps {
  values: string[]; selectedIndex: number; appearance?: "light" | "dark";
  onChange: (e: { nativeEvent: { selectedSegmentIndex: number } }) => void;
  backgroundColor?: ColorValue; tintColor?: ColorValue; fontStyle?: TextStyle; activeFontStyle?: TextStyle;
}
const SegmentedControl = SegmentedControlLib as unknown as ComponentType<SegProps>;

/**
 * The platform segmented control (UISegmentedControl on iOS). `inset` pads it to sit inside a grouped Section row.
 * Other platforms get the library's own fallback, tinted with system colours.
 */
export function Segmented<T extends string>({ options, labels, value, onChange, inset }: { options: readonly T[]; labels?: Record<T, string>; value: T; onChange: (v: T) => void; inset?: boolean }) {
  const { c, scheme } = useTheme();
  const idx = Math.max(0, options.indexOf(value));
  const fallback = Platform.OS !== "ios"
    ? { backgroundColor: c.bg as string, tintColor: c.surface as string, fontStyle: { color: c.label as string, fontSize: 14 }, activeFontStyle: { color: c.label as string, fontSize: 14, fontWeight: "600" as const } }
    : {};
  return (
    <View style={inset ? { paddingHorizontal: sp.md, paddingVertical: sp.sm } : undefined}>
      <SegmentedControl
        values={options.map((o) => labels?.[o] ?? o)}
        selectedIndex={idx}
        appearance={scheme}
        onChange={(e) => {
          const next = options[e.nativeEvent.selectedSegmentIndex];
          if (next !== undefined && next !== value) { haptic.select(); onChange(next); }
        }}
        {...fallback}
      />
    </View>
  );
}
