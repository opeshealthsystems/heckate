import { Children, isValidElement, type ReactNode } from "react";
import { Platform, Pressable, StyleSheet, Switch, View, type StyleProp, type ViewStyle } from "react-native";
import { haptic } from "../lib/haptics";
import { radius, ROW_MIN, sp, useTheme, type Colors } from "../lib/theme";
import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";

const ICON_COL = 28;
const GAP = sp.md;

/**
 * Inset grouped list: a rounded surface holding rows with hairline separators inset to the text edge.
 * `header` is the one allowed uppercase text in the app (small, secondaryLabel, 13 pt); `footer` is a quiet footnote.
 */
export function Section({ header, footer, footerTone = "label2", children, style }: { header?: string; footer?: ReactNode; footerTone?: keyof Colors; children?: ReactNode; style?: StyleProp<ViewStyle> }) {
  const { c } = useTheme();
  const rows = Children.toArray(children).filter(Boolean) as ReactNode[];
  if (!rows.length && !header) return null;
  const hasIcon = rows.some((r) => isValidElement(r) && !!(r.props as { icon?: IconName }).icon);
  const inset = sp.lg + (hasIcon ? ICON_COL + GAP : 0);
  return (
    <View style={[{ gap: sp.sm }, style]}>
      {!!header && (
        <Text v="footnote" tone="label2" accessibilityRole="header" style={{ textTransform: "uppercase", paddingHorizontal: sp.lg }}>{header}</Text>
      )}
      {rows.length > 0 && (
        <View style={{ backgroundColor: c.surface, borderRadius: radius.group, overflow: "hidden" }}>
          {rows.map((r, i) => (
            <View key={isValidElement(r) && r.key != null ? r.key : i}>
              {r}
              {i < rows.length - 1 && <View style={{ height: StyleSheet.hairlineWidth, backgroundColor: c.separator, marginLeft: inset }} />}
            </View>
          ))}
        </View>
      )}
      {typeof footer === "string" ? <Text v="footnote" tone={footerTone} style={{ paddingHorizontal: sp.lg }}>{footer}</Text> : footer}
    </View>
  );
}

export interface RowProps {
  title: string;
  subtitle?: ReactNode;
  /** trailing text (secondaryLabel) */
  value?: string;
  /** leading icon, 22 pt */
  icon?: IconName;
  iconTone?: keyof Colors;
  onPress?: () => void;
  /** show the disclosure chevron; default true when the row is pressable and has no trailing control */
  chevron?: boolean;
  /** a trailing control (Switch, check) */
  right?: ReactNode;
  /** title in the tint colour: an action row such as "Report an incident" */
  action?: boolean;
  /** title in red: a destructive row */
  destructive?: boolean;
  /** monospace title: references, ids and codes only */
  mono?: boolean;
  /** how many lines the title may take (default 2) */
  titleLines?: number;
  disabled?: boolean;
  accessibilityLabel?: string;
  accessibilityHint?: string;
}

/** One list row, at least 44 pt tall. Pressed = system highlight on iOS, ripple on Android. No bouncing. */
export function Row({ title, subtitle, value, icon, iconTone = "tint", onPress, chevron, right, action, destructive, mono, titleLines = 2, disabled, accessibilityLabel, accessibilityHint }: RowProps) {
  const { c } = useTheme();
  const showChevron = chevron ?? (!!onPress && !right && !action && !destructive);
  const titleTone: keyof Colors = destructive ? "red" : action ? "tint" : "label";
  const body = (
    <View style={{ minHeight: ROW_MIN, flexDirection: "row", alignItems: "center", gap: GAP, paddingLeft: sp.lg, paddingRight: sp.lg, paddingVertical: subtitle ? 10 : 11 }}>
      {!!icon && <View style={{ width: ICON_COL, alignItems: "center" }}><Icon name={icon} tone={destructive ? "red" : iconTone} /></View>}
      <View style={{ flex: 1, gap: 2 }}>
        <Text tone={titleTone} mono={mono} numberOfLines={titleLines} weight={action ? "regular" : undefined}>{title}</Text>
        {typeof subtitle === "string" ? <Text v="subhead" tone="label2" numberOfLines={2}>{subtitle}</Text> : subtitle}
      </View>
      {!!value && <Text tone="label2" numberOfLines={1} style={{ flexShrink: 1, maxWidth: "50%", textAlign: "right" }}>{value}</Text>}
      {right}
      {showChevron && <Icon name="chevron" size={16} tone="label3" />}
    </View>
  );
  if (!onPress) return <View accessible accessibilityLabel={accessibilityLabel} style={{ opacity: disabled ? 0.5 : 1 }}>{body}</View>;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      accessibilityHint={accessibilityHint}
      accessibilityState={{ disabled: !!disabled }}
      disabled={disabled}
      onPress={onPress}
      android_ripple={{ color: Platform.OS === "android" ? "#00000014" : undefined }}
      style={({ pressed }) => ({ backgroundColor: pressed && Platform.OS !== "android" ? c.pressed : undefined, opacity: disabled ? 0.5 : 1 })}
    >
      {body}
    </Pressable>
  );
}

/** A row with a native Switch. The switch changes state with a selection haptic. */
export function SwitchRow({ title, subtitle, value, onValueChange, icon }: { title: string; subtitle?: ReactNode; value: boolean; onValueChange: (v: boolean) => void; icon?: IconName }) {
  const { c } = useTheme();
  return (
    <Row
      title={title}
      subtitle={subtitle}
      icon={icon}
      right={<Switch accessibilityLabel={title} value={value} onValueChange={(v) => { haptic.select(); onValueChange(v); }} trackColor={{ true: c.tintFill, false: undefined }} ios_backgroundColor={undefined} />}
    />
  );
}

/** Small coloured status dot: only for P1 (red) and P2 (orange). */
export function Dot({ tone }: { tone: keyof Colors }) {
  const { c } = useTheme();
  return <View style={{ width: 8, height: 8, borderRadius: 4, backgroundColor: c[tone] }} />;
}
