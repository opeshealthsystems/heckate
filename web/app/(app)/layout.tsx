"use client";
import { Fragment, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { SessionGate, useMe } from "@/lib/session";
import { clearToken } from "@/lib/auth";
import { Icon, type IconName } from "@/components/Icon";

const WORK = new Set(["/dashboard", "/tickets", "/projects", "/interventions", "/timesheets"]);
const groupOf = (href: string) => (WORK.has(href) ? "Work" : "Business");

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
  const ordered = [...links].sort((a, b) => Number(groupOf(a[0]) !== "Work") - Number(groupOf(b[0]) !== "Work"));
  // The menu is the list of what a role may open; a page typed into the address bar follows the same rule.
  const allowed = ordered.some(([href]) => path === href || path.startsWith(href + "/"));
  const icons: Record<string, IconName> = {"/dashboard": "dashboard", "/tickets": "tickets", "/clients": "clients", "/leads": "leads", "/quotes": "quotes", "/invoices": "invoices", "/projects": "projects", "/interventions": "field", "/timesheets": "time", "/inventory": "inventory", "/accounting": "accounting", "/procurement": "purchasing", "/hr": "people", "/payroll": "payroll", "/documents": "documents", "/developer": "developer"};
  return (
    <div className="shell">
      <header className="side">
        <div className="side-head">
          <Link href="/dashboard" className="logo"><img src="/brand/HEKATE_Seal_White.png" alt="HEKATE SYSTEMS" width={72} height={72} /></Link>
          <button className="btn menu" aria-expanded={open} aria-label="Menu" onClick={() => setOpen(!open)}><Icon name={open ? "close" : "menu"} size={20} /></button>
        </div>
        <nav className={open ? "open" : ""} aria-label="Main">
          {ordered.map(([href, label], i) => (
            <Fragment key={href}>
            {(i === 0 || groupOf(href) !== groupOf(ordered[i - 1][0])) && <span className="side-group">{groupOf(href)}</span>}
            <Link href={href} className={path.startsWith(href) ? "active" : ""} aria-current={path.startsWith(href) ? "page" : undefined} onClick={() => setOpen(false)}>
              <Icon name={icons[href] ?? "dashboard"} size={18} />
              {label}
            </Link>
            </Fragment>
          ))}
          <div className="side-foot">
            <span className="who">{me.roles.map((r) => (r === "ceo" ? "CEO" : r.replace(/_/g, " "))).join(", ")}</span>
            <button
              className="btn ghost"
              onClick={() => {
                clearToken();
                router.push("/login");
              }}
            >
              <Icon name="signout" size={16} /> Sign out
            </button>
          </div>
        </nav>
      </header>
      <main className="content">
        <div className="page">
          {allowed ? children : (
            <>
              <h1>Not available</h1>
              <p className="notice" role="alert">Your role does not have access to this page. <Link href="/dashboard">Back to the dashboard</Link></p>
            </>
          )}
        </div>
      </main>
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
