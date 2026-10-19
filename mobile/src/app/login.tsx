import { useEffect, useState } from "react";
import { Text, TextInput, View } from "react-native";
import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { SafeAreaView } from "react-native-safe-area-context";
import { Button } from "../components/ui";
import { DEV_LOGIN, OIDC_CLIENT, OIDC_ISSUER } from "../lib/auth";
import { useSession } from "../lib/session";
import { useTheme } from "../lib/theme";

WebBrowser.maybeCompleteAuthSession();

export default function Login() {
  const { s } = useTheme();
  const { signIn } = useSession();
  const [token, setTok] = useState("");
  const [err, setErr] = useState("");
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
    <SafeAreaView style={s.screen}>
      <View style={s.pad}>
        <Text style={s.h1}>HEKATE</Text>
        <Text style={s.muted}>Sign in to report incidents and follow your support tickets.</Text>
        {!!OIDC_ISSUER && <Button primary title="Sign in" disabled={!request} onPress={() => void promptAsync()} />}
        {DEV_LOGIN && (
          <>
            <Text style={s.label}>Development token</Text>
            <TextInput style={[s.input, { minHeight: 96 }]} multiline autoCapitalize="none" autoCorrect={false} value={token} onChangeText={setTok} />
            <Button title="Continue" disabled={!token.trim()} onPress={() => signIn(token).catch((e) => setErr(e.message))} />
          </>
        )}
        {!OIDC_ISSUER && !DEV_LOGIN && <Text style={s.error}>Sign-in is not configured.</Text>}
        {!!err && <Text style={s.error}>{err}</Text>}
      </View>
    </SafeAreaView>
  );
}
