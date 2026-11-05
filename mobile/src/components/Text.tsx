import { Text as RNText, type TextProps } from "react-native";
import type { Variant } from "../lib/type-scale";
import { MAX_FONT_SCALE } from "../lib/type-scale";
import { monoFamily, typeStyle, useTheme, type Colors } from "../lib/theme";

export interface Props extends TextProps {
  /** platform text style, default body */
  v?: Variant;
  /** colour token, default label */
  tone?: keyof Colors;
  /** semibold (600) or bold (700) instead of the style's own weight */
  weight?: "regular" | "semibold" | "bold";
  /** monospace: only for references, ids and codes, at body size */
  mono?: boolean;
}

/** The only text component. System font, platform type scale, Dynamic Type honoured up to 1.4x. */
export function Text({ v = "body", tone = "label", weight, mono, style, maxFontSizeMultiplier = MAX_FONT_SCALE, allowFontScaling = true, ...p }: Props) {
  const { c } = useTheme();
  const base = typeStyle(v);
  return (
    <RNText
      allowFontScaling={allowFontScaling}
      maxFontSizeMultiplier={v === "largeTitle" ? 1.2 : maxFontSizeMultiplier}
      style={[base, { color: c[tone] }, weight && { fontWeight: weight === "regular" ? "400" : weight === "semibold" ? "600" : "700" }, mono && { fontFamily: monoFamily }, style]}
      {...p}
    />
  );
}
