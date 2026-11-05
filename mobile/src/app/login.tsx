import { useCallback, useEffect, useRef, useState, type ComponentRef } from "react";
import { Link, useLocalSearchParams } from "expo-router";
import { TextInput, View } from "react-native";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { AuthField, AuthShell } from "../components/AuthShell";
import { Button } from "../components/Button";
import { Text } from "../components/Text";
import { DEV_LOGIN, OIDC_CLIENT, OIDC_ISSUER, passwordSignIn } from "../lib/auth";
import { useSession } from "../lib/session";
import { sp } from "../lib/theme";

WebBrowser.maybeCompleteAuthSession();

export default function Login() {
  const { signIn: signInTo } = useSession();
  const { next } = useLocalSearchParams<{ next?: string }>();
  // After sign-in the person returns to the route they asked for (a deep link), not always the home tab.
  const signIn = useCallback((token: string) => signInTo(token, next), [signInTo, next]);

  const [user, setUser] = useState("");
  const [pass, setPass] = useState("");
  const [token, setTok] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const passRef = useRef<ComponentRef<typeof TextInput>>(null);

  // Organisation single sign-on in the system browser stays available as the second option (e.g. for MFA set-up).
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
      .then((r) => signIn(r.accessToken))
      .catch((e) => setErr(String(e.message ?? e)));
  }, [response, discovery, request, redirectUri, signIn]);

  const submit = async () => {
    if (!user.trim() || !pass) {
      setErr("Enter your email and password.");
      return;
    }
    setErr("");
    setBusy(true);
    try {
      await signIn(await passwordSignIn(user, pass));
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  };

  // Keycloak's own reset page: it emails a link, so the new password never passes through the app.
  const forgot = () => void WebBrowser.openBrowserAsync(`${OIDC_ISSUER}/login-actions/reset-credentials?client_id=${encodeURIComponent(OIDC_CLIENT)}`);

  return (
    <AuthShell
      title="Sign in to HEKATE"
      subtitle="Operations workspace"
      footer={
        <View style={{ gap: sp.md, alignItems: "center" }}>
          {!!OIDC_ISSUER && (
            <Text v="subhead" tone="label2" style={{ textAlign: "center" }}>
              New to HEKATE?{" "}
              <Link href="/request-access" accessibilityRole="link"><Text v="subhead" tone="tint" weight="semibold">Request access</Text></Link>
            </Text>
          )}
          <Text v="caption" tone="label2" style={{ textAlign: "center" }}>Encrypted connection · Session kept in secure device storage</Text>
        </View>
      }
    >
      {!OIDC_ISSUER && !DEV_LOGIN && <Text tone="red">Sign-in is not configured in this build.</Text>}

      {!!OIDC_ISSUER && (
        <>
          <AuthField
            label="Work email"
            value={user}
            onChangeText={(v) => { setUser(v); setErr(""); }}
            placeholder="name@company.com"
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="username"
            textContentType="username"
            keyboardType="email-address"
            returnKeyType="next"
            onSubmitEditing={() => passRef.current?.focus()}
          />
          <AuthField
            label="Password"
            inputRef={passRef}
            secure
            value={pass}
            onChangeText={(v) => { setPass(v); setErr(""); }}
            autoComplete="current-password"
            textContentType="password"
            returnKeyType="go"
            onSubmitEditing={() => void submit()}
          />
          <View style={{ alignItems: "flex-end" }}>
            <Button title="Forgot password?" onPress={forgot} style={{ paddingHorizontal: 0 }} />
          </View>
          {!!err && <Text v="footnote" tone="red" accessibilityLiveRegion="polite">{err}</Text>}
          <Button kind="primary" title="Sign in" loading={busy} onPress={() => void submit()} />
          <Button title="Use organisation sign-in (SSO)" disabled={!request} onPress={() => { setErr(""); void promptAsync(); }} />
        </>
      )}

      {DEV_LOGIN && (
        <>
          <AuthField label="Development token" value={token} onChangeText={setTok} placeholder="Paste a token" autoCapitalize="none" autoCorrect={false} />
          <Button title="Continue with token" onPress={() => { if (token.trim()) signIn(token).catch((e) => setErr(e.message)); }} />
          {!OIDC_ISSUER && !!err && <Text v="footnote" tone="red">{err}</Text>}
        </>
      )}
    </AuthShell>
  );
}
