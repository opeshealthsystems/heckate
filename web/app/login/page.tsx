"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { DEV_LOGIN, OIDC_ISSUER, setToken, startLogin } from "@/lib/auth";

export default function Login() {
  const router = useRouter();
  const [token, setTok] = useState("");
  const [err, setErr] = useState("");
  return (
    <main className="center">
      <div className="card narrow">
        <h1>HEKATE</h1>
        <p className="muted">Sign in to the client portal or staff console.</p>
        {OIDC_ISSUER && (
          <button className="btn primary block" onClick={() => startLogin().catch((e) => setErr(e.message))}>
            Sign in
          </button>
        )}
        {DEV_LOGIN && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setToken(token);
              router.push("/dashboard");
            }}
          >
            <label htmlFor="tok">Development token</label>
            <textarea id="tok" rows={4} value={token} onChange={(e) => setTok(e.target.value)} placeholder="Paste a token from `api devtoken`" />
            <button className="btn block" disabled={!token.trim()}>Continue</button>
          </form>
        )}
        {!OIDC_ISSUER && !DEV_LOGIN && <p className="error">Sign-in is not configured.</p>}
        {err && <p className="error">{err}</p>}
        <p className="muted small">
          <a href="/verify">Verify a HEKATE document</a>
        </p>
      </div>
    </main>
  );
}
