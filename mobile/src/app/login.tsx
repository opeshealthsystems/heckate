import { useEffect, useState } from "react";
import { Image, ScrollView, Text, View } from "react-native";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { Icon, type IconName } from "../components/Icon";
import { Button, Card, Emblem, Input, Notice, Screen } from "../components/ui";
import { DEV_LOGIN, OIDC_CLIENT, OIDC_ISSUER } from "../lib/auth";
import { useSession } from "../lib/session";
import { sp, useTheme } from "../lib/theme";

WebBrowser.maybeCompleteAuthSession();

const FACTS: [IconName, string, string][] = [
  ["key", "Browser sign-in", "OIDC with PKCE in the system browser. This app never sees your password."],
  ["lock", "Token in the keychain", "Your session token is kept in the device's secure storage, not in app files."],
  ["fingerprint", "App lock", "Biometric or device passcode at launch and after 60 s in the background."],
];

export default function Login() {
  const { s, c } = useTheme();
  const { signIn } = useSession();
  const [token, setTok] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const discovery = AuthSession.useAutoDiscovery(OIDC_ISSUER || "https://invalid.invalid");
  const redirectUri = AuthSession.makeRedirectUri({ scheme: "hekate", path: "auth" });
  const [request, response, promptAsync] = AuthSession.useAuthRequest(
    { clientId: OIDC_CLIENT, scopes: ["openid", "profile", "email"], redirectUri, usePKCE: true },
    discovery,
  );

  useEffect(() => {
    if (response?.type !== "success" || !discovery || !request?.codeVerifier) return;
    AuthSession.exchangeCodeAsync(
      { clientId: OIDC_CLIENT, code: response.params.code, redirectUri, extraParams: { code_verifier: request.codeVerifier } },
      discovery,
    )
      .then((t) => signIn(t.accessToken))
      .catch((e) => setErr(String(e.message ?? e)));
  }, [response, discovery, request, redirectUri, signIn]);

  return (
    <Screen noStrip>
      <ScrollView contentContainerStyle={{ padding: sp.lg, paddingBottom: sp.xl, gap: sp.lg, flexGrow: 1 }} keyboardShouldPersistTaps="handled">
        <View style={{ alignItems: "center", gap: sp.md, paddingTop: sp.lg }}>
          <Emblem size={124} />
          <Image source={require("../../assets/logo-white.png")} accessibilityLabel="HEKATE SYSTEMS" style={{ width: 150, height: 60 }} resizeMode="contain" />
        </View>
        <View style={{ gap: 6, alignItems: "center" }}>
          <Text style={[s.overline, { color: c.goldText }]}>Secure access</Text>
          <Text style={[s.h1, { textAlign: "center" }]} accessibilityRole="header">Sign in to your account</Text>
          <Text style={[s.muted, { textAlign: "center", maxWidth: 320 }]}>For HEKATE clients, staff and suppliers. Tickets, field visits and issued documents.</Text>
        </View>

        {!!OIDC_ISSUER && <Button primary icon="key" title="Continue to secure sign-in" disabled={!request} loading={busy} onPress={() => { setBusy(true); void promptAsync().finally(() => setBusy(false)); }} />}
        {!OIDC_ISSUER && !DEV_LOGIN && <Notice tone="bad" icon="alert">Sign-in is not configured in this build.</Notice>}
        {!!err && <Notice tone="bad" icon="alert">{err}</Notice>}

        {DEV_LOGIN && (
          <Card style={{ borderStyle: "dashed", borderColor: c.warn, gap: sp.sm }}>
            <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
              <Icon name="wrench" size={14} color={c.warn} />
              <Text style={[s.overline, { color: c.warn }]}>Developer access</Text>
            </View>
            <Input label="Development token" multiline mono autoCapitalize="none" autoCorrect={false} value={token} onChangeText={setTok} />
            <Button title="Continue with token" disabled={!token.trim()} onPress={() => signIn(token).catch((e) => setErr(e.message))} />
          </Card>
        )}

        <View style={{ gap: sp.sm, marginTop: sp.sm }}>
          {FACTS.map(([icon, title, text]) => (
            <View key={title} style={{ flexDirection: "row", gap: sp.md, alignItems: "flex-start" }}>
              <View style={{ width: 34, height: 34, borderRadius: 8, borderWidth: 1, borderColor: c.line, backgroundColor: c.surface, alignItems: "center", justifyContent: "center" }}>
                <Icon name={icon} size={18} color={c.goldText} />
              </View>
              <View style={{ flex: 1, gap: 1 }}>
                <Text style={[s.text, s.semibold, { fontSize: 14 }]}>{title}</Text>
                <Text style={[s.muted, { fontSize: 13 }]}>{text}</Text>
              </View>
            </View>
          ))}
        </View>
      </ScrollView>
    </Screen>
  );
}
