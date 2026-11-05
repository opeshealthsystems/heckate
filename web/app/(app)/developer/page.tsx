"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { when } from "@/lib/format";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Plan { code: string; name: string; rate_per_min: number; monthly_quota: number; price_xaf: number; active: boolean }
interface Key { id: string; name: string; prefix: string; scopes: string[]; plan: string; created_at: string; expires_at: string | null; revoked_at: string | null; last_used_at: string | null }
interface Usage { month: string; calls: number }

const RESOURCES = ["tickets", "invoices", "quotes", "projects", "interventions", "assets"];

export default function Developer() {
  const me = useMe();
  const admin = me.roles.includes("client_admin");
  const [plans, setPlans] = useState<Plan[]>([]);
  const [keys, setKeys] = useState<Key[]>([]);
  const [usage, setUsage] = useState<Record<string, Usage[]>>({});
  const [secret, setSecret] = useState("");
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState<string[]>(["read:tickets"]);
  const load = useCallback(async () => {
    const [p, k] = await Promise.all([raw.get<{ data: Plan[] }>("/v1/api-plans"), raw.get<{ data: Key[] }>("/v1/api-keys")]);
    setPlans(p.data);
    setKeys(k.data);
    const u: Record<string, Usage[]> = {};
    await Promise.all(k.data.map(async (x) => { u[x.id] = (await raw.get<{ data: Usage[] }>(`/v1/api-keys/${x.id}/usage`)).data; }));
    setUsage(u);
  }, []);
  useEffect(() => { void load().catch(() => {}); }, [load]);
  const act = useAction(load);
  const plan = (code: string) => plans.find((p) => p.code === code);
  const toggle = (s: string) => setScopes(scopes.includes(s) ? scopes.filter((x) => x !== s) : [...scopes, s]);
  return (
    <>
      <h1>Developer</h1>
      <p className="muted">API keys give your own systems read-only access to your company&apos;s data. Send the key as <code>Authorization: Bearer hk_…</code>. The full contract is in the API reference.</p>
      <Err msg={act.err} />
      {secret && (
        <div className="notice" role="status">
          <strong>Copy this key now. It will not be shown again.</strong>
          <pre className="wrap" data-testid="new-key">{secret}</pre>
          <button className="btn" onClick={() => setSecret("")}>I have saved it</button>
        </div>
      )}
      <h2>Keys</h2>
      <Table head={["Name", "Key", "Scopes", "Plan", "This month", "Last used", "Status", ""]} empty={keys.length ? undefined : "No API keys yet."}>
        {keys.map((k) => {
          const p = plan(k.plan);
          const calls = usage[k.id]?.[0]?.calls ?? 0;
          return (
            <tr key={k.id}>
              <td>{k.name}</td><td><code>{k.prefix}…</code></td><td className="wrap">{k.scopes.join(", ")}</td>
              <td>{p?.name ?? k.plan}</td>
              <td className="num">{calls.toLocaleString("en")}{p ? ` / ${p.monthly_quota.toLocaleString("en")}` : ""}</td>
              <td>{k.last_used_at ? when(k.last_used_at) : "never"}</td>
              <td><Pill text={k.revoked_at ? "revoked" : "active"} tone={k.revoked_at ? "bad" : "ok"} /></td>
              <td>{!k.revoked_at && <button className="btn" onClick={() => { if (confirm(`Revoke ${k.name}? Systems using it stop working immediately.`)) void act.run(() => raw.del(`/v1/api-keys/${k.id}`)); }}>Revoke</button>}</td>
            </tr>
          );
        })}
      </Table>
      {admin ? (
        <>
          <h2>New key</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { const r = await raw.post<{ secret: string }>("/v1/api-keys", { name, scopes }); setSecret(r.secret); setName(""); }); }}>
            <Field id="key-name" label="Name (what will use it)"><input id="key-name" required maxLength={80} value={name} onChange={(e) => setName(e.target.value)} /></Field>
            <fieldset>
              <legend>Can read</legend>
              {RESOURCES.map((r) => (
                <label key={r} className="check"><input type="checkbox" checked={scopes.includes(`read:${r}`)} onChange={() => toggle(`read:${r}`)} /> {r}</label>
              ))}
            </fieldset>
            <button className="btn primary" disabled={act.busy || !scopes.length}>Create key</button>
          </form>
        </>
      ) : (
        <p className="muted">Only your company&apos;s administrator can create keys.</p>
      )}
      <h2>Plans</h2>
      <Table head={["Plan", "Requests per minute", "Requests per month", "Price"]}>
        {plans.filter((p) => p.active).map((p) => (
          <tr key={p.code}><td>{p.name}</td><td className="num">{p.rate_per_min}</td><td className="num">{p.monthly_quota.toLocaleString("en")}</td><td className="num">{p.price_xaf ? `${p.price_xaf.toLocaleString("en")} XAF / month` : "Free"}</td></tr>
        ))}
      </Table>
    </>
  );
}
