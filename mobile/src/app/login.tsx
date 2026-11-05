import { useCallback, useEffect, useRef, useState, type ComponentRef } from "react";
import { Link, useLocalSearchParams } from "expo-router";
import { TextInput, View } from "react-native";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { AuthField, AuthShell } from "../components/AuthShell";
import { Button } from "../components/Button";
import { Text } from "../components/Text";
import { API_URL } from "../lib/api";
import { DEV_LOGIN, OIDC_CLIENT, OIDC_ISSUER, passwordSignIn } from "../lib/auth";
import { useSession } from "../lib/session";
import { sp, useTheme } from "../lib/theme";

WebBrowser.maybeCompleteAuthSession();

export default function Login() {
  const { signIn: signInTo } = useSession();
  const { c } = useTheme();
  const { next } = useLocalSearchParams<{ next?: string }>();
  // After sign-in the person returns to the route they asked for (a deep link), not always the home tab.
  const signIn = useCallback((token: string) => signInTo(token, next), [signInTo, next]);

  const [user, setUser] = useState("");
  const [pass, setPass] = useState("");
  const [token, setTok] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const passRef = useRef<ComponentRef<typeof TextInput>>(null);
  // One-tap demo accounts. The list comes from the server (HEKATE_DEMO_LOGINS), so it can be withdrawn without an app update.
  const [demos, setDemos] = useState<{ label: string; email: string; password: string }[]>([]);
  const [demoBusy, setDemoBusy] = useState("");
  useEffect(() => {
    fetch(`${API_URL}/v1/public/demo-logins`).then((r) => (r.ok ? r.json() : { data: [] })).then((b) => setDemos(Array.isArray(b?.data) ? b.data : [])).catch(() => {});
  }, []);
  const demo = async (d: { label: string; email: string; password: string }) => {
    setErr("");
    setDemoBusy(d.label);
    try {
      await signIn(await passwordSignIn(d.email, d.password));
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setDemoBusy("");
    }
  };

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
          {demos.length > 0 && (
            <View style={{ gap: sp.sm, marginTop: sp.sm }}>
              <Text v="footnote" tone="label2" style={{ textAlign: "center" }}>Try the app as</Text>
              <View style={{ flexDirection: "row", flexWrap: "wrap", gap: sp.sm, justifyContent: "center" }}>
                {demos.map((d) => (
                  <Button
                    key={d.label}
                    title={d.label}
                    loading={demoBusy === d.label}
                    disabled={!!demoBusy}
                    onPress={() => void demo(d)}
                    accessibilityHint={`Signs in with the ${d.label} demo account`}
                    style={{ borderWidth: 1, borderColor: c.separator, paddingHorizontal: sp.md, minWidth: 120 }}
                  />
                ))}
              </View>
            </View>
          )}
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
