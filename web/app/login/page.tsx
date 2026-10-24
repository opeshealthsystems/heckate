"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { DEV_LOGIN, OIDC_ISSUER, setToken, startLogin } from "@/lib/auth";

export default function Login() {
  const router = useRouter();
  const [token, setTok] = useState("");
  const [err, setErr] = useState("");
  return (
    <main className="signin">
      <section className="signin-brand">
        <img className="mark" src="/brand/HEKATE_Seal_Watermark.png" alt="" aria-hidden="true" />
        <img className="logo" src="/brand/HEKATE_Logo_White.png" alt="HEKATE SYSTEMS" width={104} height={42} />
        <div style={{ position: "relative" }}>
          <h1>One secure account for clients, staff and suppliers.</h1>
          <p>Tickets, incidents, quotations, invoices, reports and certificates — on web and the HEKATE app.</p>
        </div>
        <p className="foot">Protected by two-factor sign-in · Data hosted in Cameroon</p>
      </section>
      <section className="signin-form">
        <div>
          <h2>Sign in</h2>
          <p className="muted">Use your HEKATE account.</p>
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
      </section>
    </main>
  );
}
