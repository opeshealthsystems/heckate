import { useCallback, useEffect, useState } from "react";
import { useLocalSearchParams } from "expo-router";
import { KeyboardAvoidingView, Platform, ScrollView, View } from "react-native";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { Banner } from "../components/Banner";
import { Button } from "../components/Button";
import { Section } from "../components/List";
import { Seal, Wordmark } from "../components/Seal";
import { Text } from "../components/Text";
import { Field } from "../components/TextField";
import { DEV_LOGIN, OIDC_CLIENT, OIDC_ISSUER } from "../lib/auth";
import { useSession } from "../lib/session";
import { sp, useTheme } from "../lib/theme";

WebBrowser.maybeCompleteAuthSession();

export default function Login() {
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  const { signIn: signInTo } = useSession();
  const { next } = useLocalSearchParams<{ next?: string }>();
  // After sign-in the person returns to the route they asked for (a deep link), not always the home tab.
  const signIn = useCallback((token: string) => signInTo(token, next), [signInTo, next]);
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
    <KeyboardAvoidingView style={{ flex: 1, backgroundColor: c.bg }} behavior={Platform.OS === "ios" ? "padding" : undefined}>
      <ScrollView contentContainerStyle={{ flexGrow: 1, paddingTop: insets.top + sp.xl, paddingBottom: insets.bottom + sp.lg, paddingHorizontal: sp.lg }} keyboardShouldPersistTaps="handled">
        <View style={{ width: "100%", maxWidth: 420, alignSelf: "center", flexGrow: 1 }}>
          <View style={{ flex: 1, alignItems: "center", justifyContent: "center", gap: sp.md, paddingVertical: sp.xl }}>
            <Seal size={96} />
            <Wordmark />
            <Text tone="label2" style={{ textAlign: "center", marginTop: sp.xs }}>Sign in with your HEKATE account.</Text>
          </View>

          <View style={{ gap: sp.lg }}>
            {!OIDC_ISSUER && !DEV_LOGIN && <Banner icon="warning" tone="red">Sign-in is not configured in this build.</Banner>}
            {!!err && <Banner icon="warning" tone="red">{err}</Banner>}
            {DEV_LOGIN && (
              <Section header="Development token">
                <Field placeholder="Paste a token" autoCapitalize="none" autoCorrect={false} autoComplete="off" textContentType="none" returnKeyType="go" value={token} onChangeText={setTok} onSubmitEditing={() => token.trim() && signIn(token).catch((e) => setErr(e.message))} mono />
                <Button title="Continue with token" disabled={!token.trim()} onPress={() => signIn(token).catch((e) => setErr(e.message))} style={{ alignSelf: "flex-start" }} />
              </Section>
            )}
            {!!OIDC_ISSUER && (
              <View style={{ gap: sp.sm }}>
                <Button kind="primary" title="Sign in" disabled={!request} loading={busy} onPress={() => { setBusy(true); void promptAsync().finally(() => setBusy(false)); }} />
                <Text v="footnote" tone="label2" style={{ textAlign: "center" }}>You will sign in with your organisation's account in the system browser.</Text>
              </View>
            )}
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
