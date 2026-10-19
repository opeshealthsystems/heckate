import { company } from "@/content/company";
import { dicts, path } from "@/content";
import { faqLd, organizationLd, serviceLd } from "@/content/seo";
import type { Lang } from "@/content/types";
import { ContactForm } from "./ContactForm";
import { JsonLd } from "./seo";
import { Shell } from "./Shell";

export function HomePage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  const h = d.home;
  return (
    <Shell lang={lang} here="/">
      <JsonLd data={organizationLd(company, d)} />
      <section className="hero">
        <h1>{h.h1}</h1>
        <p className="lead">{h.lead}</p>
        <p className="actions">
          <a className="btn" href={path(lang, "/contact/")}>{h.cta}</a>
          <a className="btn ghost" href={`${company.portalUrl}/login`}>{h.ctaSupport}</a>
        </p>
      </section>
      <section>
        <h2>{h.servicesH}</h2>
        <div className="grid">
          {d.services.items.map((s) => (
            <article key={s.slug} className="card">
              <h3><a href={`${path(lang, "/services/")}#${s.slug}`}>{s.name}</a></h3>
              <p>{s.summary}</p>
            </article>
          ))}
        </div>
      </section>
      <section>
        <h2>{h.platformH}</h2>
        <ul>{h.platform.map((x) => <li key={x}>{x}</li>)}</ul>
      </section>
      <section>
        <h2>{h.trustH}</h2>
        <ul>{h.trust.map((x) => <li key={x}>{x}</li>)}</ul>
      </section>
      <section className="card">
        <h2>{h.supportH}</h2>
        <p>{h.support}</p>
        <p><a href={`mailto:${company.incidentEmail}`}>{company.incidentEmail}</a></p>
      </section>
    </Shell>
  );
}

export function ServicesPage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  return (
    <Shell lang={lang} here="/services/">
      <JsonLd data={faqLd(d.services.items)} />
      {d.services.items.map((s) => <JsonLd key={s.slug} data={serviceLd(company, s)} />)}
      <h1>{d.services.h1}</h1>
      {d.services.items.map((s) => (
        <section key={s.slug} id={s.slug} className="card">
          <h2>{s.name}</h2>
          <p>{s.summary}</p>
          <dl>
            {s.faq.map((f) => (
              <div key={f.q}>
                <dt>{f.q}</dt>
                <dd>{f.a}</dd>
              </div>
            ))}
          </dl>
        </section>
      ))}
    </Shell>
  );
}

export function AboutPage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  return (
    <Shell lang={lang} here="/about/">
      <JsonLd data={organizationLd(company, d)} />
      <h1>{d.about.h1}</h1>
      {d.about.body.map((p) => <p key={p}>{p}</p>)}
      <dl className="facts">
        <dt>Legal name</dt><dd>{company.legalName}</dd>
        <dt>Location</dt><dd>{company.streetAddress ? `${company.streetAddress}, ` : ""}{company.city}, Cameroon</dd>
        {company.rccm && (<><dt>RCCM</dt><dd>{company.rccm}</dd></>)}
        {company.founders.length > 0 && (<><dt>Founders</dt><dd>{company.founders.join(", ")}</dd></>)}
      </dl>
    </Shell>
  );
}

export function ContactPage({ lang }: { lang: Lang }) {
  const d = dicts[lang];
  const c = d.contact;
  return (
    <Shell lang={lang} here="/contact/">
      <h1>{c.h1}</h1>
      <p>{c.intro}</p>
      <ContactForm t={c} />
      <section className="card">
        <h2>{c.emergencyH}</h2>
        <p>{c.emergency} <a href={`mailto:${company.incidentEmail}`}>{company.incidentEmail}</a>{company.emergencyPhone && <> · <a href={`tel:${company.emergencyPhone}`}>{company.emergencyPhone}</a></>}</p>
      </section>
    </Shell>
  );
}
