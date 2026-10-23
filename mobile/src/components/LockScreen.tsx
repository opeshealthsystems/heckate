import { StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useLock } from "../lib/lock";
import { useSession } from "../lib/session";
import { sp, useTheme } from "../lib/theme";
import { Button } from "./Button";
import { Seal, Wordmark } from "./Seal";
import { Text } from "./Text";

/** Covers the whole app while locked, and hides it in the app switcher snapshot. Flat: seal, one sentence, Unlock. */
export function LockGate() {
  const { locked, covered, cap, error, unlock } = useLock();
  const { signOut } = useSession();
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  if (!locked && !covered) return null;
  return (
    <View style={[StyleSheet.absoluteFill, { backgroundColor: c.canvas, zIndex: 100, paddingTop: insets.top, paddingBottom: insets.bottom + sp.lg, paddingHorizontal: sp.lg }]} accessibilityViewIsModal>
      <View style={{ flex: 1, alignItems: "center", justifyContent: "center", gap: sp.md }}>
        <Seal size={96} />
        <Wordmark />
        <Text v="title" style={{ textAlign: "center", marginTop: sp.sm }}>HEKATE is locked</Text>
        {locked && (
          <Text tone="label2" style={{ textAlign: "center", maxWidth: 300 }}>
            {cap === "biometric" ? "Use Face ID, your fingerprint or your passcode to continue." : "Use your device passcode to continue."}
          </Text>
        )}
        {!!error && locked && <Text v="footnote" tone="red" style={{ textAlign: "center" }} accessibilityLiveRegion="polite">{error}</Text>}
      </View>
      {locked && (
        <View style={{ gap: sp.sm, width: "100%", maxWidth: 420, alignSelf: "center" }}>
          <Button kind="primary" title="Unlock" onPress={() => void unlock()} />
          <Button title="Sign out" onPress={() => void signOut()} />
        </View>
      )}
    </View>
  );
}
