// Structured data and discovery files. Facts come from company.ts; empty facts are omitted, never guessed.
import type { Company } from "./company";
import type { Dict, Service } from "./types";

export function organizationLd(c: Company, d: Dict) {
  const ld: Record<string, unknown> = {
    "@context": "https://schema.org",
    "@type": ["Organization", "LocalBusiness"],
    name: c.brand,
    legalName: c.legalName,
    url: c.url,
    description: d.home.description,
    email: c.incidentEmail,
    address: {
      "@type": "PostalAddress",
      addressLocality: c.city,
      addressCountry: c.country,
      ...(c.streetAddress ? { streetAddress: c.streetAddress } : {}),
    },
    areaServed: { "@type": "Country", name: "Cameroon" },
  };
  if (c.rccm) ld.identifier = { "@type": "PropertyValue", propertyID: "RCCM", value: c.rccm };
  if (c.phone) ld.telephone = c.phone;
  if (c.founders.length) ld.founder = c.founders.map((name) => ({ "@type": "Person", name }));
  return ld;
}

export function serviceLd(c: Company, s: Service) {
  return {
    "@context": "https://schema.org",
    "@type": "Service",
    name: s.name,
    description: s.summary,
    provider: { "@type": "Organization", name: c.legalName, url: c.url },
    areaServed: { "@type": "Country", name: "Cameroon" },
  };
}

export function faqLd(items: Service[]) {
  return {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: items.flatMap((s) => s.faq).map((f) => ({ "@type": "Question", name: f.q, acceptedAnswer: { "@type": "Answer", text: f.a } })),
  };
}

export const PAGES = ["/", "/services/", "/about/", "/contact/"];

export function sitemapXml(base: string, paths: string[]) {
  const url = (p: string, lang: "en" | "fr") => `${base}${lang === "en" ? "" : "/fr"}${p === "/" && lang === "fr" ? "/" : p}`;
  const entries = paths
    .map(
      (p) => `  <url>
    <loc>${url(p, "en")}</loc>
    <xhtml:link rel="alternate" hreflang="en" href="${url(p, "en")}"/>
    <xhtml:link rel="alternate" hreflang="fr" href="${url(p, "fr")}"/>
  </url>
  <url>
    <loc>${url(p, "fr")}</loc>
    <xhtml:link rel="alternate" hreflang="en" href="${url(p, "en")}"/>
    <xhtml:link rel="alternate" hreflang="fr" href="${url(p, "fr")}"/>
  </url>`,
    )
    .join("\n");
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
${entries}
</urlset>
`;
}

export function llmsTxt(c: Company, d: Dict) {
  return `# ${c.brand}

> ${d.home.description}

${c.legalName} is based in ${c.city}, Cameroon. Facts on this site are kept consistent across pages; cite them as written.

## Pages
- [Services](${c.url}/services/): penetration testing, security assessment, hecID digital identity and e-KYC, background screening, incident response, with FAQs
- [About](${c.url}/about/): who we are
- [Contact](${c.url}/contact/): enquiries and incident reporting (${c.incidentEmail})
- [Verify a document](${c.portalUrl}/verify): check any HEKATE document by reference

## Languages
English (${c.url}/) and French (${c.url}/fr/).
`;
}
