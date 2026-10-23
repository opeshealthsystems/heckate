import { useCallback, useState } from "react";
import { View } from "react-native";
import Constants from "expo-constants";
import { router, Stack, useFocusEffect } from "expo-router";
import { useNetInfo } from "@react-native-community/netinfo";
import { Banner, SyncBanner } from "../../../components/Banner";
import { Row, Section, SwitchRow } from "../../../components/List";
import { Screen } from "../../../components/Screen";
import { Seal } from "../../../components/Seal";
import { Segmented } from "../../../components/Segmented";
import { Text } from "../../../components/Text";
import { api, API_URL, getLastSync } from "../../../lib/api";
import { confirmDestructive } from "../../../lib/dialogs";
import { useLock } from "../../../lib/lock";
import { useSession } from "../../../lib/session";
import { sp, useTheme, type Preference } from "../../../lib/theme";
import type { EmployeeRecord } from "../../../lib/types";

const roleLabel = (r: string) => (r === "ceo" ? "CEO" : r.charAt(0).toUpperCase() + r.slice(1).replace(/_/g, " "));
const hostOf = (u: string) => u.replace(/^https?:\/\//, "").replace(/\/.*$/, "");
const PREFS: Preference[] = ["system", "light", "dark"];
const PREF_LABEL: Record<Preference, string> = { system: "System", light: "Light", dark: "Dark" };

export default function Account() {
  const { pref, setPref } = useTheme();
  const { me, signOut, pendingActions, pendingDrafts } = useSession();
  const lock = useLock();
  const net = useNetInfo();
  const staff = me?.scope === "staff";
  const [rec, setRec] = useState<EmployeeRecord | null>(null);
  const [recMsg, setRecMsg] = useState("");
  const [lockMsg, setLockMsg] = useState("");
  const [synced, setSynced] = useState<number | null>(null);

  useFocusEffect(useCallback(() => {
    setSynced(getLastSync());
    if (!staff) return;
    api.myRecord().then((r) => { setRec(r); setRecMsg(""); setSynced(getLastSync()); }).catch((e) => setRecMsg((e as Error).message));
  }, [staff]));

  const lockOn = lock.enabled && lock.cap !== "none";
  const lockSub = lock.cap === "none"
    ? "This device has no screen lock, so the app lock cannot be enforced."
    : lock.cap === "biometric" ? "Face ID or fingerprint, with your passcode as a fallback." : "Uses your device passcode.";
  const tls = API_URL.startsWith("https://");
  const version = Constants.expoConfig?.version ?? "1.0.0";
  const build = Constants.nativeBuildVersion ?? Constants.expoConfig?.ios?.buildNumber ?? Constants.expoConfig?.android?.versionCode?.toString() ?? "development";
  const queued = pendingActions + pendingDrafts;
  const leaveText = rec ? `${rec.annual_leave_balance} of ${rec.annual_leave_entitlement} days left` : recMsg ? (/not found/i.test(recMsg) ? "No record yet" : "Unavailable") : "Loading…";

  return (
    <Screen webTitle="Account">
      <Stack.Screen options={{ title: "Account" }} />
      <SyncBanner />

      <Section>
        <Row icon="account" iconTone="label2" title={me?.email || me?.sub || "Signed in"} titleLines={1} subtitle={me?.roles.map(roleLabel).join(", ")} />
      </Section>

      <Section header="Security" footer="Your session token is kept in the device's secure storage. Sign-in uses your organisation's account in the system browser.">
        <SwitchRow icon="lock" title="App lock" subtitle={lockSub} value={lockOn} onValueChange={(on) => { setLockMsg(""); void lock.setEnabled(on).then((m) => setLockMsg(m ?? "")); }} />
        <Row title="Locks" value="After 60 s away" />
      </Section>
      {!!lockMsg && <Banner icon="warning">{lockMsg}</Banner>}

      {staff && (
        <Section header="Time and expenses">
          <Row icon="calendar" iconTone="label2" title="Annual leave" value={leaveText} />
          <Row icon="clock" iconTone="label2" title="Log hours" onPress={() => router.push("/(tabs)/account/hours")} />
          <Row icon="calendar" iconTone="label2" title="Request leave" onPress={() => router.push("/(tabs)/account/leave")} />
          <Row icon="receipt" iconTone="label2" title="Expense claim" onPress={() => router.push("/(tabs)/account/expense")} />
        </Section>
      )}

      <Section header="Appearance">
        <Segmented inset options={PREFS} labels={PREF_LABEL} value={pref} onChange={setPref} />
      </Section>

      <Section header="Connection">
        <Row title="Status" value={net.isConnected === false ? "Offline" : "Online"} />
        <Row title="Encryption" value={tls ? "HTTPS" : "Not encrypted (development)"} />
        <Row title="Server" value={hostOf(API_URL)} />
        <Row title="Last sync" value={synced ? new Date(synced).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" }) : "Not yet"} />
        <Row title="Queued updates" value={String(queued)} />
        <Row title="Version" value={`${version} (${build})`} />
      </Section>

      <Section>
        <Row icon="signOut" destructive title="Sign out" onPress={() => confirmDestructive("Sign out?", "You will need to sign in again to use HEKATE.", "Sign out", () => void signOut())} />
      </Section>

      <View style={{ alignItems: "center", gap: sp.sm, paddingTop: sp.sm }}>
        <Seal size={40} />
        <Text v="footnote" tone="label2">HEKATE Systems</Text>
      </View>
    </Screen>
  );
}
