import { company } from "@/content/company";
import { dicts, path } from "@/content";
import { faqLd, organizationLd, pageLd, serviceLd } from "@/content/seo";
import type { Block, Lang } from "@/content/types";
import { ContactForm } from "./ContactForm";
import { JsonLd } from "./seo";
import { Shell } from "./Shell";

function Label({ children }: { children: string }) {
  return <div className="label">{children}</div>;
}

export function HomePage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  const h = d.home;
  return (
    <Shell lang={lang} here="/">
      <JsonLd data={organizationLd(company, d)} />
      <section className="hero navy">
        <img className="watermark" src="/brand/HEKATE_Seal_Watermark.png" alt="" aria-hidden="true" />
        <div className="wrap hero-in">
          <div className="stack">
            <div className="label on-navy">{h.eyebrow}</div>
            <h1>{h.h1}</h1>
            <p className="lead">{h.lead}</p>
            <p className="actions">
              <a className="btn gold big" href={path(lang, "/contact/")}>{h.ctaPrimary}</a>
              <a className="btn outline big" href={path(lang, "/services/")}>{h.ctaSecondary}</a>
            </p>
          </div>
          <div className="card portal">
            <h2>{h.portal.h}</h2>
            <p className="small">{h.portal.note}</p>
            <a className="btn navy-solid big block" href={`${company.portalUrl}/login`}>{h.portal.cta}</a>
            <div className="portal-links">
              <a href={`${company.portalUrl}/verify`}>{h.portal.verify}</a>
            </div>
          </div>
        </div>
      </section>

      <section className="band" aria-label="Sectors">
        <ul className="wrap sectors">{h.sectors.map((s) => <li key={s}>{s}</li>)}</ul>
      </section>

      <section id="services" className="wrap section">
        <div className="head"><Label>{h.servicesLabel}</Label><h2>{h.servicesH}</h2></div>
        <div className="grid">
          {d.services.practices.map((s) => (
            <article key={s.slug} className="tile">
              <span className="num">{s.n}</span>
              <h3><a href={`${path(lang, "/services/")}#${s.slug}`}>{s.name}</a></h3>
              <p>{s.summary}</p>
            </article>
          ))}
        </div>
      </section>

      <section id="platform" className="band">
        <div className="wrap section">
          <div className="stack">
            <Label>{h.platformLabel}</Label>
            <h2>{h.platformH}</h2>
            <p>{h.platformLead}</p>
            <ul className="ticks">{h.platformPoints.map((x) => <li key={x}>{x}</li>)}</ul>
          </div>
        </div>
      </section>

      <section id="identity" className="navy">
        <div className="wrap section stack">
          <div className="head"><div className="label on-navy">{h.identityLabel}</div><h2>{h.identityH}</h2><p className="lead">{h.identityLead}</p></div>
          <div className="grid">
            {h.identityItems.map((i) => (<div key={i.h} className="tile raised"><h3>{i.h}</h3><p>{i.t}</p></div>))}
          </div>
        </div>
      </section>

      <section id="guarantees" className="wrap section stack">
        <div className="head"><Label>{h.guaranteesLabel}</Label><h2>{h.guaranteesH}</h2></div>
        <div className="grid">
          {h.guarantees.map((g) => (<div key={g.h} className="tile"><h3>{g.h}</h3><p>{g.t}</p></div>))}
        </div>
        <p><a href={path(lang, "/guarantees/")}>{h.guaranteesLink} →</a></p>
        <form className="card verify" method="get" action={`${company.portalUrl}/verify`}>
          <h3>{h.verifyH}</h3>
          <p>{h.verifyBody}</p>
          <div className="row">
            <label className="sr" htmlFor="ref">{h.verifyField}</label>
            <input id="ref" name="ref" placeholder="HS-INV-2026-000001" autoComplete="off" required pattern="HS-[A-Z0-9-]+-[0-9]{4}-[0-9]{6}(-[0-9A-Z])?" />
            <button className="btn navy-solid">{h.verifyCta}</button>
          </div>
        </form>
      </section>

      <section id="support" className="band">
        <div className="wrap section stack">
          <div className="head"><Label>{h.supportLabel}</Label><h2>{h.supportH}</h2></div>
          <div className="grid">
            {h.steps.map((s, i) => (<div key={s.h} className="tile"><span className="num">{String(i + 1).padStart(2, "0")}</span><h3>{s.h}</h3><p>{s.t}</p></div>))}
          </div>
        </div>
      </section>
    </Shell>
  );
}

export function ServicesPage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  const s = d.services;
  return (
    <Shell lang={lang} here="/services/">
      <JsonLd data={faqLd(s.faq)} />
      {s.practices.map((p) => <JsonLd key={p.slug} data={serviceLd(company, p)} />)}
      <section className="wrap section stack">
        <h1 className="page-h">{s.h1}</h1>
        <p className="lead dark">{s.intro}</p>
        <div className="grid">
          {s.practices.map((p) => (
            <article key={p.slug} id={p.slug} className="tile">
              <span className="num">{p.n}</span>
              <h2 className="h3">{p.name}</h2>
              <p>{p.summary}</p>
            </article>
          ))}
        </div>
      </section>
      {s.groups.map((g, i) => (
        <section key={g.h} className={i % 2 === 0 ? "band" : ""}>
          <div className="wrap section stack">
            <h2>{g.h}</h2>
            {g.note && <p className="muted">{g.note}</p>}
            <ul className="detail">
              {g.items.map((it) => (<li key={it.text}>{it.name && <strong>{it.name}. </strong>}{it.text}</li>))}
            </ul>
          </div>
        </section>
      ))}
      <section className="wrap section stack">
        <h2>{s.faqH}</h2>
        <dl className="faq">
          {s.faq.map((f) => (<div key={f.q}><dt>{f.q}</dt><dd>{f.a}</dd></div>))}
        </dl>
      </section>
    </Shell>
  );
}

export function AboutPage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  const a = d.about;
  const L = a.detailLabels;
  return (
    <Shell lang={lang} here="/about/">
      <JsonLd data={organizationLd(company, d)} />
      <section className="wrap section stack">
        <Label>{a.label}</Label>
        <h1 className="page-h">{a.h1}</h1>
        <p className="lead dark">{a.lead}</p>
      </section>
      <section className="band">
        <div className="wrap section grid3">
          <div><h2 className="h3">{a.missionH}</h2><p>{a.mission}</p></div>
          <div><h2 className="h3">{a.howH}</h2><p>{a.how}</p><p className="muted">{a.method}</p></div>
          <div><h2 className="h3">{a.whereH}</h2><p>{a.where}</p></div>
        </div>
      </section>
      <section className="wrap section stack">
        <h2>{a.leadershipH}</h2>
        <div className="grid">
          {a.leaders.map((p) => (
            <div key={p.name} className="tile person">
              <span className="avatar" aria-hidden="true">{p.initials}</span>
              <div><h3>{p.name}</h3><p className="role">{p.role}</p><p>{p.bio}</p></div>
            </div>
          ))}
        </div>
      </section>
      <section className="band">
        <div className="wrap section stack">
          <h2>{a.platformsH}</h2>
          <div className="grid">
            {a.platforms.map((p) => (<div key={p.name} className="tile"><h3>{p.name}</h3><p>{p.text}</p></div>))}
          </div>
          <h2>{a.sectorsH}</h2>
          <ul className="chips">{d.home.sectors.map((s) => <li key={s}>{s}</li>)}</ul>
        </div>
      </section>
      <section className="wrap section stack">
        <h2>{a.detailsH}</h2>
        <dl className="facts">
          <dt>{L.legalName}</dt><dd>{company.legalName}</dd>
          <dt>{L.legalForm}</dt><dd>{a.legalForm}</dd>
          <dt>{L.office}</dt><dd>{company.addressLines.join(", ")}</dd>
          <dt>{L.coordinates}</dt><dd>{company.latitude}° N, {company.longitude}° E</dd>
          <dt>{L.website}</dt><dd>hekatesystems.com</dd>
          {company.rccm && (<><dt>{L.rccm}</dt><dd>{company.rccm}</dd></>)}
          {company.niu && (<><dt>{L.niu}</dt><dd>{company.niu}</dd></>)}
          {company.phone && (<><dt>{L.phone}</dt><dd>{company.phone}</dd></>)}
        </dl>
        <h2 className="h3">{a.policiesH}</h2>
        <p><a href={path(lang, "/guarantees/")}>{d.footer.guarantees}</a></p>
      </section>
    </Shell>
  );
}

export function ContactPage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  const c = d.contact;
  return (
    <Shell lang={lang} here="/contact/">
      <JsonLd data={organizationLd(company, d)} />
      <JsonLd data={pageLd(company, "ContactPage", c.title, c.description, `${company.url}${path(lang, "/contact/")}`)} />
      <section className="wrap section stack">
        <h1 className="page-h">{c.h1}</h1>
        <p className="lead dark">{c.intro}</p>
        <div className="alert">
          <p>{c.emergency}</p>
          <a className="btn navy-solid" href={`${company.portalUrl}/login`}>{c.emergencyCta}</a>
        </div>
        <div className="split contact">
          <ContactForm t={c} services={d.services.practices.map((p) => p.name)} />
          <div className="stack">
            <div><h2 className="h3">{c.officeH}</h2><p>{company.addressLines.map((l) => <span key={l}>{l}<br /></span>)}{company.latitude}° N, {company.longitude}° E</p></div>
            <div>
              <h2 className="h3">{c.departmentsH}</h2>
              <ul className="plain">{c.departments.map((x) => (<li key={x.address}><a href={`mailto:${x.address}`}>{x.address}</a><br /><span className="muted small">{x.purpose}</span></li>))}</ul>
            </div>
            {(company.phone || company.poBox) && (
              <div><h2 className="h3">{c.phonePostH}</h2><p>{[company.phone, company.poBox && `P.O. Box ${company.poBox}`].filter(Boolean).join(" · ")}</p></div>
            )}
            <div><h2 className="h3">{c.hoursH}</h2><p>{c.hours}</p></div>
          </div>
        </div>
      </section>
    </Shell>
  );
}

function Blocks({ blocks }: { blocks: Block[] }) {
  return (
    <>
      {blocks.map((b, i) => {
        if (b.t === "p") return <p key={i}>{b.text}</p>;
        if (b.t === "ul") return <ul key={i}>{b.items.map((x) => <li key={x}>{x}</li>)}</ul>;
        if (b.t === "ol") return <ol key={i}>{b.items.map((x) => <li key={x}>{x}</li>)}</ol>;
        return (
          <div key={i} className="table-wrap" tabIndex={0} role="region" aria-label={b.head.join(", ")}>
            <table>
              <thead><tr>{b.head.map((h) => <th key={h} scope="col">{h}</th>)}</tr></thead>
              <tbody>{b.rows.map((r) => (<tr key={r[0]}>{r.map((c, j) => (j === 0 ? <th key={j} scope="row">{c}</th> : <td key={j}>{c}</td>))}</tr>))}</tbody>
            </table>
          </div>
        );
      })}
    </>
  );
}

export function GuaranteesPage({ lang }: { lang: Lang }) {
  const g = dicts[lang].guarantees;
  const en = dicts.en.guarantees;
  const sections = g.sections.length ? g.sections : en.sections;
  return (
    <Shell lang={lang} here="/guarantees/">
      <JsonLd data={pageLd(company, "WebPage", g.title, g.description, `${company.url}${path(lang, "/guarantees/")}`)} />
      <article className="wrap section prose">
        <h1 className="page-h">{g.h1}</h1>
        <p className="mono small">{g.meta}</p>
        {g.notice && <p className="alert">{g.notice}</p>}
        <p>{g.intro || en.intro}</p>
        {sections.map((s) => (<section key={s.h}><h2>{s.h}</h2><Blocks blocks={s.blocks} /></section>))}
      </article>
    </Shell>
  );
}
