import { useState, type ComponentRef, type ReactNode, type Ref } from "react";
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, TextInput, View, type TextInputProps } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { MIN_TARGET, radius, sp, typeStyle, useTheme } from "../lib/theme";
import { MAX_FONT_SCALE } from "../lib/type-scale";
import { Icon } from "./Icon";
import { Seal } from "./Seal";
import { Text } from "./Text";

/**
 * The shared frame for the signed-out screens (sign in, request access): the platform seal, a title and a card
 * of fields, centred and scrollable above the keyboard. Colours, type and spacing all come from the theme.
 */
export function AuthShell({ title, subtitle, children, footer }: { title: string; subtitle?: string; children: ReactNode; footer?: ReactNode }) {
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  return (
    <KeyboardAvoidingView style={{ flex: 1, backgroundColor: c.canvas }} behavior={Platform.OS === "ios" ? "padding" : "height"}>
      <ScrollView
        contentContainerStyle={{ flexGrow: 1, justifyContent: "center", paddingTop: insets.top + sp.xl, paddingBottom: insets.bottom + sp.xl, paddingHorizontal: sp.lg }}
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode={Platform.OS === "ios" ? "interactive" : "on-drag"}
      >
        <View style={{ width: "100%", maxWidth: 420, alignSelf: "center", gap: sp.lg }}>
          <View style={{ alignItems: "center", gap: sp.sm }}>
            <Seal size={96} />
            <Text v="title" accessibilityRole="header" style={{ textAlign: "center", marginTop: sp.sm }}>{title}</Text>
            {!!subtitle && <Text v="subhead" tone="label2" style={{ textAlign: "center" }}>{subtitle}</Text>}
          </View>
          <View style={{ gap: sp.md }}>{children}</View>
          {footer}
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

/** A labelled, outlined input. `secure` adds a show/hide toggle for passwords. */
export function AuthField({ label, secure, inputRef, ...p }: TextInputProps & { label: string; secure?: boolean; inputRef?: Ref<ComponentRef<typeof TextInput>> }) {
  const { c } = useTheme();
  const [focus, setFocus] = useState(false);
  const [shown, setShown] = useState(false);
  return (
    <View style={{ gap: sp.xs }}>
      <Text v="footnote" tone="label2">{label}</Text>
      <View style={{ flexDirection: "row", alignItems: "center", minHeight: MIN_TARGET + 4, borderWidth: 1, borderRadius: radius.field, borderColor: focus ? c.tint : c.separator, backgroundColor: c.surface }}>
        <TextInput
          ref={inputRef}
          allowFontScaling
          maxFontSizeMultiplier={MAX_FONT_SCALE}
          placeholderTextColor={c.label3}
          selectionColor={c.tint as string}
          cursorColor={c.tint as string}
          secureTextEntry={secure && !shown}
          accessibilityLabel={label}
          {...p}
          onFocus={(e) => { setFocus(true); p.onFocus?.(e); }}
          onBlur={(e) => { setFocus(false); p.onBlur?.(e); }}
          style={[typeStyle("body"), { flex: 1, minWidth: 0, color: c.label as string, paddingHorizontal: sp.md, paddingVertical: sp.sm, outlineStyle: "none" } as object]}
        />
        {secure && (
          <Pressable onPress={() => setShown((v) => !v)} hitSlop={8} accessibilityRole="button" accessibilityLabel={shown ? "Hide password" : "Show password"} style={{ width: MIN_TARGET, height: MIN_TARGET, alignItems: "center", justifyContent: "center" }}>
            <Icon name={shown ? "eyeOff" : "eye"} size={22} tone="label2" />
          </Pressable>
        )}
      </View>
    </View>
  );
}
