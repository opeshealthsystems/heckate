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
  const links = [
    ["/dashboard", "Dashboard"],
    ["/tickets", staff ? "Ticket queue" : "Support"],
    ...(staff ? [["/clients", "Clients"], ["/leads", "Leads"], ["/documents", "Documents"]] : []),
  ];
  return (
    <div className="shell">
      <header className="top">
        <strong>HEKATE</strong>
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
