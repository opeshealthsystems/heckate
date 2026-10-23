import { TextInput, View, type TextInputProps } from "react-native";
import { monoFamily, ROW_MIN, sp, typeStyle, useTheme } from "../lib/theme";
import { MAX_FONT_SCALE } from "../lib/type-scale";
import { Text } from "./Text";

export interface FieldProps extends Omit<TextInputProps, "style"> {
  /** label at the left of the row (single line fields). Multi-line fields take their label from the Section header. */
  label?: string;
  /** references, ids and codes only */
  mono?: boolean;
}

/**
 * A text field that lives inside a grouped Section: label on the left, input filling the row (single line),
 * or an inset multi-line area. Validation text goes in the Section footer (red, 13 pt).
 */
export function Field({ label, mono, multiline, ...p }: FieldProps) {
  const { c } = useTheme();
  const input = (
    <TextInput
      allowFontScaling
      maxFontSizeMultiplier={MAX_FONT_SCALE}
      placeholderTextColor={c.label3}
      selectionColor={c.tint as string}
      cursorColor={c.tint as string}
      multiline={multiline}
      textAlignVertical={multiline ? "top" : "center"}
      accessibilityLabel={p.accessibilityLabel ?? label ?? p.placeholder}
      style={[
        typeStyle("body"),
        { color: c.label as string, flex: 1, paddingVertical: 0, outlineStyle: "none" } as object,
        mono && { fontFamily: monoFamily },
        multiline && { minHeight: 96, paddingTop: 2 },
        !!label && !multiline && { textAlign: "right" },
      ]}
      {...p}
    />
  );
  return (
    <View style={{ minHeight: multiline ? 120 : ROW_MIN, flexDirection: "row", alignItems: multiline ? "flex-start" : "center", gap: sp.md, paddingHorizontal: sp.lg, paddingVertical: multiline ? sp.md : 6 }}>
      {!!label && !multiline && <Text style={{ minWidth: 96, maxWidth: "40%" }} numberOfLines={1}>{label}</Text>}
      {input}
    </View>
  );
}
