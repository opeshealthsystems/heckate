"use client";
import { useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { SessionGate, useMe } from "@/lib/session";
import { clearToken } from "@/lib/auth";

function Shell({ children }: { children: ReactNode }) {
  const me = useMe();
  const path = usePathname();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const staff = me.scope === "staff";
  const has = (...r: string[]) => me.roles.some((x) => r.includes(x));
  const links: [string, string][] = [
    ["/dashboard", "Dashboard"],
  ];
  // Auditors and suppliers have no ticket queue; every other signed-in role does.
  if (has("ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics", "procurement", "client_admin", "client_user")) links.push(["/tickets", staff ? "Ticket queue" : "Support"]);
  if (staff && has("ceo", "manager", "sales")) links.push(["/clients", "Clients"], ["/leads", "Leads"]);
  else if (staff && has("support", "finance", "engineer")) links.push(["/clients", "Clients"]);
  // Money is for management, finance and sales, and for the client themselves; other staff do not see it.
  if (has("ceo", "manager", "finance", "sales", "client_user", "client_admin")) links.push(["/quotes", "Quotes"], ["/invoices", "Invoices"]);
  if (staff || has("client_user", "client_admin")) links.push(["/projects", "Projects"], ["/interventions", has("client_user", "client_admin") ? "Site visits" : "Field"]);
  if (has("ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee")) links.push(["/timesheets", "Time"]);
  if (has("ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics")) links.push(["/inventory", "Inventory"]);
  if (has("client_user", "client_admin")) links.push(["/inventory", "Equipment"]);
  if (has("ceo", "manager", "finance", "auditor")) links.push(["/accounting", "Accounting"]);
  if (has("ceo", "manager", "procurement", "finance", "supplier")) links.push(["/procurement", has("supplier") ? "Supplier portal" : "Purchasing"]);
  if (staff) links.push(["/hr", "People"]);
  if (has("hr", "finance", "manager", "ceo", "auditor")) links.push(["/payroll", "Payroll"]);
  if (staff && has("ceo", "manager", "finance", "sales", "support", "engineer", "hr", "logistics", "procurement")) links.push(["/documents", "Documents"]);
  if (has("client_admin", "client_user")) links.push(["/developer", "Developer"]);
  return (
    <div className="shell">
      <header className="top">
        <Link href="/dashboard" className="logo"><img src="/brand/HEKATE_Logo_White.png" alt="HEKATE SYSTEMS" width={104} height={42} /></Link>
        <button className="btn menu" aria-expanded={open} aria-label="Menu" onClick={() => setOpen(!open)}>☰</button>
        <nav className={open ? "open" : ""}>
          {links.map(([href, label]) => (
            <Link key={href} href={href} className={path.startsWith(href) ? "active" : ""} onClick={() => setOpen(false)}>
              {label}
            </Link>
          ))}
          <button
            className="btn ghost"
            onClick={() => {
              clearToken();
              router.push("/login");
            }}
          >
            Sign out
          </button>
        </nav>
      </header>
      <main className="content">{children}</main>
    </div>
  );
}

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <SessionGate>
      <Shell>{children}</Shell>
    </SessionGate>
  );
}
