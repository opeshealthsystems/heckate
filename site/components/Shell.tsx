import type { ReactNode } from "react";
import { company } from "@/content/company";
import { dicts, path } from "@/content";
import type { Lang } from "@/content/types";

export function Shell({ lang, here, children }: { lang: Lang; here: string; children: ReactNode }) {
  const d = dicts[lang];
  const other: Lang = lang === "en" ? "fr" : "en";
  const home = path(lang, "/");
  return (
    <>
      <a className="skip" href="#main">{lang === "en" ? "Skip to content" : "Aller au contenu"}</a>
      <header className="top">
        <div className="wrap top-in">
          <a className="logo" href={home}><img src="/brand/HEKATE_Logo_White.png" alt="HEKATE SYSTEMS" width={104} height={42} /></a>
          <input id="nav-toggle" type="checkbox" className="nav-toggle" aria-label="Menu" />
          <label htmlFor="nav-toggle" className="burger" aria-hidden="true">☰</label>
          <nav aria-label="Main">
            <a href={`${path(lang, "/services/")}`}>{d.nav.services}</a>
            <a href={`${home}#platform`}>{d.nav.platform}</a>
            <a href={`${home}#identity`}>{d.nav.identity}</a>
            <a href={path(lang, "/about/")}>{d.nav.about}</a>
            <a href={path(lang, "/contact/")}>{d.nav.contact}</a>
          </nav>
          <div className="top-cta">
            <a className="btn outline" href={`${company.portalUrl}/login`}>{d.nav.report}</a>
            <a className="btn gold" href={`${company.portalUrl}/login`}>{d.nav.portal}</a>
          </div>
        </div>
      </header>
      <main id="main">{children}</main>
      <footer className="foot">
        <div className="wrap foot-in">
          <div>
            <img src="/brand/HEKATE_Logo_White.png" alt="HEKATE SYSTEMS" width={104} height={42} />
            <p>{company.addressLines.map((l) => <span key={l}>{l}<br /></span>)}</p>
          </div>
          <div>
            <h2>{d.footer.companyH}</h2>
            <a href={path(lang, "/about/")}>{d.footer.about}</a>
            <a href={path(lang, "/contact/")}>{d.footer.contact}</a>
          </div>
          <div>
            <h2>{d.footer.clientsH}</h2>
            <a href={`${company.portalUrl}/login`}>{d.footer.clientPortal}</a>
            <a href={`${company.portalUrl}/verify`}>{d.footer.verify}</a>
          </div>
          <div>
            <h2>{d.footer.legalH}</h2>
            <a href={path(lang, "/guarantees/")}>{d.footer.guarantees}</a>
            <a href={path(other, here)} hrefLang={other} lang={other}>{d.footer.otherLang}</a>
          </div>
        </div>
        <p className="wrap foot-legal">
          © {new Date().getFullYear()} {company.legalName}
          {company.rccm && ` · RCCM ${company.rccm}`}
          {company.niu && ` · NIU ${company.niu}`}
          {" · "}{d.footer.rights}
        </p>
      </footer>
    </>
  );
}
