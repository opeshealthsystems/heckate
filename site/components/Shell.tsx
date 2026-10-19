import type { ReactNode } from "react";
import { company } from "@/content/company";
import { dicts, path } from "@/content";
import type { Lang } from "@/content/types";

export function Shell({ lang, here, children }: { lang: Lang; here: string; children: ReactNode }) {
  const d = dicts[lang];
  const other: Lang = lang === "en" ? "fr" : "en";
  return (
    <>
      <a className="skip" href="#main">Skip to content</a>
      <header className="top">
        <a className="brand" href={path(lang, "/")}>HEKATE</a>
        <input id="nav-toggle" type="checkbox" className="nav-toggle" aria-label="Menu" />
        <label htmlFor="nav-toggle" className="burger" aria-hidden="true">☰</label>
        <nav>
          <a href={path(lang, "/")}>{d.nav.home}</a>
          <a href={path(lang, "/services/")}>{d.nav.services}</a>
          <a href={path(lang, "/about/")}>{d.nav.about}</a>
          <a href={path(lang, "/contact/")}>{d.nav.contact}</a>
          <a href={`${company.portalUrl}/login`} className="btn small">{d.nav.portal}</a>
          <a href={path(other, here)} hrefLang={other} lang={other}>{other.toUpperCase()}</a>
        </nav>
      </header>
      <main id="main">{children}</main>
      <footer className="foot">
        <p>© {new Date().getFullYear()} {company.legalName}. {d.footer.rights}</p>
        <p>{company.city}, Cameroon · <a href={`mailto:${company.incidentEmail}`}>{company.incidentEmail}</a></p>
        <p><a href={`${company.portalUrl}/verify`}>{d.footer.verify}</a></p>
      </footer>
    </>
  );
}
