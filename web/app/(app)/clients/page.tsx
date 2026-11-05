"use client";
import { useEffect, useState } from "react";
import { api, fetchFile } from "@/lib/api";
import { useMe } from "@/lib/session";
import { DocumentPreview, type PreviewSource } from "@/components/DocumentPreview";
import { ExportButton } from "@/components/ExportButton";
import { Icon } from "@/components/Icon";
import type { Client } from "@/lib/types";

export default function Clients() {
  const me = useMe();
  const statements = me.roles.some((r) => ["ceo", "manager", "finance", "sales"].includes(r));
  const [preview, setPreview] = useState<PreviewSource | null>(null);
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
          <thead><tr><th scope="col">Name</th><th scope="col">RCCM</th><th scope="col">Email</th><th scope="col">KYC</th>{statements && <th scope="col"><span className="sr">Statement</span></th>}</tr></thead>
          <tbody>
            {rows.map((c) => (
              <tr key={c.tenant_id}>
                <td>{c.legal_name}</td><td>{c.rccm ?? "–"}</td><td>{c.email ?? "–"}</td>
                <td><span className={`chip ${c.kyc_status === "verified" ? "ok" : c.kyc_status === "rejected" ? "bad" : ""}`}>{c.kyc_status}</span></td>
                {statements && (
                  <td className="nowrap">
                    <button className="btn" onClick={() => setPreview({ title: `Statement of account · ${c.legal_name}`, filename: `statement-${c.legal_name}.pdf`, load: () => fetchFile(`/v1/clients/${c.tenant_id}/statement/pdf`) })}><Icon name="invoices" size={16} /> Statement</button>{" "}
                    <ExportButton path={`/v1/exports/client-statement/${c.tenant_id}`} filename={`statement-${c.legal_name.replace(/[^A-Za-z0-9]+/g, "-")}`} label="CSV" />
                  </td>
                )}
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
      {preview && <DocumentPreview source={preview} onClose={() => setPreview(null)} />}
    </>
  );
}
