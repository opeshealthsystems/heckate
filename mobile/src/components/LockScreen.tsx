import { StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useLock } from "../lib/lock";
import { useSession } from "../lib/session";
import { sp, useTheme } from "../lib/theme";
import { Icon } from "./Icon";
import { Backdrop, Button, Emblem } from "./ui";

/** Covers the whole app while locked, and hides it in the app switcher snapshot. */
export function LockGate() {
  const { locked, covered, cap, error, unlock } = useLock();
  const { signOut } = useSession();
  const { c, s } = useTheme();
  const insets = useSafeAreaInsets();
  if (!locked && !covered) return null;
  return (
    <View style={[StyleSheet.absoluteFill, { backgroundColor: c.void, zIndex: 100, paddingTop: insets.top, paddingBottom: insets.bottom + sp.lg }]} accessibilityViewIsModal>
      <Backdrop />
      <View style={{ flex: 1, alignItems: "center", justifyContent: "center", gap: sp.lg, paddingHorizontal: sp.xl }}>
        <Emblem size={132} />
        <View style={{ alignItems: "center", gap: 6 }}>
          <Text style={[s.h1, { letterSpacing: 4 }]}>HEKATE</Text>
          <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
            <Icon name="lock" size={14} color={c.goldText} />
            <Text style={[s.overline, { color: c.goldText }]}>Session locked</Text>
          </View>
        </View>
        {locked && (
          <Text style={[s.muted, { textAlign: "center", maxWidth: 300 }]}>
            {cap === "biometric" ? "Confirm it is you with your fingerprint or face. You can use the device passcode instead." : "Confirm it is you with the device passcode."}
          </Text>
        )}
        {!!error && locked && <Text style={[s.error, { textAlign: "center" }]} accessibilityLiveRegion="polite">{error}</Text>}
      </View>
      {locked && (
        <View style={{ gap: sp.sm, paddingHorizontal: sp.lg }}>
          <Button primary icon={cap === "biometric" ? "fingerprint" : "key"} title={cap === "biometric" ? "Unlock" : "Unlock with passcode"} onPress={() => void unlock()} />
          <Button variant="ghost" title="Sign out instead" onPress={() => void signOut()} />
          <Text style={[s.mono, { textAlign: "center", color: c.muted, fontSize: 11 }]}>Locks after 60 s in the background</Text>
        </View>
      )}
    </View>
  );
}
