// Token handling. Production uses Keycloak (Authorization Code + PKCE); dev can paste a token.
// The token lives in sessionStorage only: it dies with the tab and is never written to localStorage.
const KEY = "hekate.token";
const PKCE = "hekate.pkce";

export const OIDC_ISSUER = process.env.NEXT_PUBLIC_OIDC_ISSUER ?? "";
export const OIDC_CLIENT = process.env.NEXT_PUBLIC_OIDC_CLIENT_ID ?? "hekate-web";
export const DEV_LOGIN = process.env.NEXT_PUBLIC_DEV_LOGIN === "1";

export function getToken(): string | null {
  try {
    return sessionStorage.getItem(KEY);
  } catch {
    return null;
  }
}

export function setToken(t: string) {
  try {
    sessionStorage.setItem(KEY, t.trim());
  } catch {
    /* storage unavailable: session lasts until reload */
  }
}

export function clearToken() {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
}

function b64url(buf: ArrayBuffer | Uint8Array): string {
  const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export async function pkceChallenge(verifier: string): Promise<string> {
  return b64url(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier)));
}

async function discovery() {
  const res = await fetch(`${OIDC_ISSUER}/.well-known/openid-configuration`);
  if (!res.ok) throw new Error("Cannot reach the sign-in service");
  return res.json() as Promise<{ authorization_endpoint: string; token_endpoint: string }>;
}

export async function startLogin() {
  const verifier = b64url(crypto.getRandomValues(new Uint8Array(32)));
  const state = b64url(crypto.getRandomValues(new Uint8Array(16)));
  sessionStorage.setItem(PKCE, JSON.stringify({ verifier, state }));
  const { authorization_endpoint } = await discovery();
  const q = new URLSearchParams({
    client_id: OIDC_CLIENT,
    response_type: "code",
    scope: "openid profile email",
    redirect_uri: `${location.origin}/auth/callback`,
    code_challenge: await pkceChallenge(verifier),
    code_challenge_method: "S256",
    state,
  });
  location.assign(`${authorization_endpoint}?${q}`);
}

export async function finishLogin(params: URLSearchParams) {
  const saved = JSON.parse(sessionStorage.getItem(PKCE) ?? "null");
  sessionStorage.removeItem(PKCE);
  const code = params.get("code");
  if (!saved || !code || params.get("state") !== saved.state) throw new Error("Sign-in was not completed");
  const { token_endpoint } = await discovery();
  const res = await fetch(token_endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      client_id: OIDC_CLIENT,
      code,
      redirect_uri: `${location.origin}/auth/callback`,
      code_verifier: saved.verifier,
    }),
  });
  if (!res.ok) throw new Error("Sign-in failed");
  const { access_token } = await res.json();
  setToken(access_token);
}
