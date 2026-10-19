"use client";
import { useState } from "react";
import { company } from "@/content/company";
import type { Dict } from "@/content/types";

export function ContactForm({ t }: { t: Dict["contact"] }) {
  const [state, setState] = useState<"idle" | "sending" | "done" | "error">("idle");
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    setState("sending");
    try {
      const res = await fetch(`${company.apiUrl}/v1/public/enquiries`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(Object.fromEntries(f)),
      });
      setState(res.ok ? "done" : "error");
    } catch {
      setState("error");
    }
  }
  if (state === "done") return <p className="notice" role="status">{t.thanks}</p>;
  return (
    <form onSubmit={submit} className="card form">
      <label htmlFor="name">{t.name}</label>
      <input id="name" name="name" required maxLength={200} autoComplete="name" />
      <label htmlFor="email">{t.email}</label>
      <input id="email" name="email" type="email" required autoComplete="email" />
      <label htmlFor="phone">{t.phone}</label>
      <input id="phone" name="phone" autoComplete="tel" />
      <label htmlFor="company">{t.company}</label>
      <input id="company" name="company" autoComplete="organization" />
      <label htmlFor="message">{t.message}</label>
      <textarea id="message" name="message" rows={5} maxLength={5000} />
      {/* honeypot: hidden from people, filled by bots */}
      <input name="website" tabIndex={-1} autoComplete="off" aria-hidden="true" className="hp" />
      {state === "error" && <p className="error" role="alert">{t.error}</p>}
      <button className="btn" disabled={state === "sending"}>{state === "sending" ? t.sending : t.send}</button>
      <p className="muted small">{t.note}</p>
    </form>
  );
}
