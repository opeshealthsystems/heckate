import * as SecureStore from "expo-secure-store";

const KEY = "hekate.token";
let cached: string | null = null;

export const OIDC_ISSUER = process.env.EXPO_PUBLIC_OIDC_ISSUER ?? "";
export const OIDC_CLIENT = process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ?? "hekate-mobile";
export const DEV_LOGIN = process.env.EXPO_PUBLIC_DEV_LOGIN === "1";

export async function loadToken(): Promise<string | null> {
  cached = await SecureStore.getItemAsync(KEY);
  return cached;
}
export const getToken = () => cached;
export async function setToken(t: string) {
  cached = t.trim();
  await SecureStore.setItemAsync(KEY, cached);
}
export async function clearToken() {
  cached = null;
  await SecureStore.deleteItemAsync(KEY);
}
