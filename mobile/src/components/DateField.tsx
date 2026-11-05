import { createElement } from "react";
import { Platform, View } from "react-native";
import DateTimePicker, { DateTimePickerAndroid } from "@react-native-community/datetimepicker";
import { haptic } from "../lib/haptics";
import { ROW_MIN, sp, useTheme } from "../lib/theme";
import { Row } from "./List";
import { Text } from "./Text";

/** "2026-10-23" <-> local Date. The API keeps the ISO value; the screen shows the device locale. */
export const isoDay = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
export const fromIso = (s: string) => {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : new Date();
};
const show = (s: string) => fromIso(s).toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short", year: "numeric" });

/**
 * A date row for grouped forms. iOS: the system compact date picker. Android: the system date dialog.
 * (The web preview uses the browser's own date input.) `value` and `onChange` use ISO yyyy-mm-dd.
 */
export function DateField({ label, value, onChange }: { label: string; value: string; onChange: (iso: string) => void }) {
  const { c, scheme } = useTheme();
  const date = fromIso(value);
  if (Platform.OS === "ios") {
    return (
      <View style={{ minHeight: ROW_MIN, flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingHorizontal: sp.lg, paddingVertical: 6 }}>
        <Text>{label}</Text>
        <DateTimePicker value={date} mode="date" display="compact" themeVariant={scheme} accentColor={c.tint as string} onChange={(_e, d) => { if (d) { haptic.select(); onChange(isoDay(d)); } }} />
      </View>
    );
  }
  if (Platform.OS === "android") {
    return <Row title={label} value={show(value)} onPress={() => DateTimePickerAndroid.open({ value: date, mode: "date", onChange: (_e, d) => { if (d) onChange(isoDay(d)); } })} />;
  }
  return (
    <View style={{ minHeight: ROW_MIN, flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingHorizontal: sp.lg, paddingVertical: 6 }}>
      <Text>{label}</Text>
      {createElement("input", { type: "date", value, "aria-label": label, onChange: (e: { target: { value: string } }) => e.target.value && onChange(e.target.value), style: { fontFamily: "system-ui, -apple-system, sans-serif", fontSize: 17, border: 0, background: "transparent", color: c.label as string, colorScheme: scheme } })}
    </View>
  );
}
