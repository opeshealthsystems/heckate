"use client";
import { useState } from "react";
import { company } from "@/content/company";
import type { Dict } from "@/content/types";

export function ContactForm({ t, services }: { t: Dict["contact"]; services: string[] }) {
  const [state, setState] = useState<"idle" | "sending" | "done" | "error">("idle");
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    setState("sending");
    // The enquiry API takes name, email, phone, company, message and the honeypot; the chosen service
    // travels at the start of the message so the CRM lead keeps it.
    const service = String(f.get("service") ?? "");
    const message = [service && `${t.service}: ${service}`, String(f.get("message") ?? "")].filter(Boolean).join("\n\n");
    try {
      const res = await fetch(`${company.apiUrl}/v1/public/enquiries`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: f.get("name"), email: f.get("email"), phone: f.get("phone"), company: f.get("company"), website: f.get("website"), message,
        }),
      });
      setState(res.ok ? "done" : "error");
    } catch {
      setState("error");
    }
  }
  if (state === "done") return <p className="notice" role="status">{t.thanks}</p>;
  return (
    <form onSubmit={submit} className="card form">
      <label htmlFor="name">{t.fullName}</label>
      <input id="name" name="name" required maxLength={200} autoComplete="name" />
      <label htmlFor="company">{t.organisation}</label>
      <input id="company" name="company" autoComplete="organization" />
      <label htmlFor="email">{t.email}</label>
      <input id="email" name="email" type="email" required autoComplete="email" />
      <label htmlFor="phone">{t.phone}</label>
      <input id="phone" name="phone" autoComplete="tel" />
      <label htmlFor="service">{t.service}</label>
      <select id="service" name="service" defaultValue="">
        <option value="">{t.servicePick}</option>
        {services.map((s) => <option key={s} value={s}>{s}</option>)}
      </select>
      <label htmlFor="message">{t.message}</label>
      <textarea id="message" name="message" rows={5} maxLength={4500} required />
      {/* honeypot: hidden from people, filled by bots */}
      <input name="website" tabIndex={-1} autoComplete="off" aria-hidden="true" className="hp" />
      <label className="check"><input type="checkbox" name="consent" required /> <span>{t.consent}</span></label>
      {state === "error" && <p className="error" role="alert">{t.error}</p>}
      <button className="btn navy-solid big" disabled={state === "sending"}>{state === "sending" ? t.sending : t.send}</button>
    </form>
  );
}
