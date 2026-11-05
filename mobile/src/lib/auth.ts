import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

// The token lives in the device keychain/keystore. The web build is only a preview, where SecureStore does not exist,
// so it keeps the token for the browser session instead.
const store = Platform.OS === "web"
  ? {
      get: async (k: string) => globalThis.sessionStorage?.getItem(k) ?? null,
      set: async (k: string, v: string) => globalThis.sessionStorage?.setItem(k, v),
      del: async (k: string) => globalThis.sessionStorage?.removeItem(k),
    }
  : { get: (k: string) => SecureStore.getItemAsync(k), set: (k: string, v: string) => SecureStore.setItemAsync(k, v), del: (k: string) => SecureStore.deleteItemAsync(k) };

const KEY = "hekate.token";
let cached: string | null = null;

export const OIDC_ISSUER = process.env.EXPO_PUBLIC_OIDC_ISSUER ?? "";
export const OIDC_CLIENT = process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ?? "hekate-mobile";
export const DEV_LOGIN = process.env.EXPO_PUBLIC_DEV_LOGIN === "1";

/**
 * Signs in inside the app with the HEKATE account's email (or username) and password, using the identity provider's
 * token endpoint directly (OAuth password grant). Requires "Direct access grants" on the mobile client in Keycloak.
 * Returns the access token; throws an Error with a message fit to show the person.
 */
export async function passwordSignIn(username: string, password: string): Promise<string> {
  if (!OIDC_ISSUER) throw new Error("Sign-in is not configured in this build.");
  const body = new URLSearchParams({ grant_type: "password", client_id: OIDC_CLIENT, username: username.trim(), password, scope: "openid profile email" });
  let res: Response;
  try {
    res = await fetch(`${OIDC_ISSUER.replace(/\/$/, "")}/protocol/openid-connect/token`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    });
  } catch {
    throw new Error("Can't reach HEKATE. Check your connection and try again.");
  }
  const json = (await res.json().catch(() => ({}))) as { access_token?: string; error?: string; error_description?: string };
  if (res.ok && json.access_token) return json.access_token;
  if (json.error === "invalid_grant") {
    if (/not fully set up|required action/i.test(json.error_description ?? "")) throw new Error("Your account needs to finish setup. Use organisation sign-in once to complete it.");
    if (/disabled/i.test(json.error_description ?? "")) throw new Error("This account is disabled. Contact your administrator.");
    throw new Error("Email or password is incorrect.");
  }
  if (json.error === "unauthorized_client") throw new Error("In-app sign-in isn't enabled for this server yet. Use organisation sign-in.");
  throw new Error("Sign-in failed. Try again in a moment.");
}

export async function loadToken(): Promise<string | null> {
  cached = await store.get(KEY);
  return cached;
}
export const getToken = () => cached;
export async function setToken(t: string) {
  cached = t.trim();
  await store.set(KEY, cached);
}
export async function clearToken() {
  cached = null;
  await store.del(KEY);
}
