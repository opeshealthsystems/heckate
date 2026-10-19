"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Client } from "@/lib/types";

export default function Clients() {
  const [rows, setRows] = useState<Client[]>([]);
  const [f, setF] = useState({ legal_name: "", rccm: "", email: "", phone: "" });
  const [err, setErr] = useState("");
  const load = () => api.clients().then((r) => setRows(r.data)).catch((e) => setErr(e.message));
  useEffect(() => { void load(); }, []);
  return (
    <>
      <h1>Clients</h1>
      {err && <p className="error">{err}</p>}
      <div className="scroll">
        <table>
          <thead><tr><th>Name</th><th>RCCM</th><th>Email</th><th>KYC</th></tr></thead>
          <tbody>
            {rows.map((c) => (
              <tr key={c.tenant_id}>
                <td>{c.legal_name}</td><td>{c.rccm ?? "–"}</td><td>{c.email ?? "–"}</td>
                <td><span className={`chip ${c.kyc_status === "verified" ? "ok" : c.kyc_status === "rejected" ? "bad" : ""}`}>{c.kyc_status}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <h2>Add client</h2>
      <form className="card form" onSubmit={(e) => { e.preventDefault(); setErr(""); api.createClient(f).then(() => { setF({ legal_name: "", rccm: "", email: "", phone: "" }); return load(); }).catch((x) => setErr(x.message)); }}>
        <label htmlFor="n">Legal name</label>
        <input id="n" required value={f.legal_name} onChange={(e) => setF({ ...f, legal_name: e.target.value })} />
        <label htmlFor="r">RCCM</label>
        <input id="r" value={f.rccm} onChange={(e) => setF({ ...f, rccm: e.target.value })} />
        <label htmlFor="e">Email</label>
        <input id="e" type="email" value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} />
        <label htmlFor="p">Phone</label>
        <input id="p" value={f.phone} onChange={(e) => setF({ ...f, phone: e.target.value })} />
        <button className="btn primary">Create</button>
      </form>
    </>
  );
}
