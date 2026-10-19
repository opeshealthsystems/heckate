export type Lang = "en" | "fr";

export interface Service {
  slug: string;
  name: string;
  summary: string;
  faq: { q: string; a: string }[];
}

export interface Dict {
  lang: Lang;
  locale: string;
  nav: { home: string; services: string; about: string; contact: string; portal: string; verify: string };
  home: {
    title: string;
    description: string;
    h1: string;
    lead: string;
    cta: string;
    ctaSupport: string;
    servicesH: string;
    platformH: string;
    platform: string[];
    trustH: string;
    trust: string[];
    supportH: string;
    support: string;
  };
  services: { title: string; description: string; h1: string; items: Service[] };
  about: { title: string; description: string; h1: string; body: string[] };
  contact: {
    title: string; description: string; h1: string; intro: string; emergencyH: string; emergency: string;
    name: string; email: string; phone: string; company: string; message: string; send: string; sending: string;
    thanks: string; error: string; note: string;
  };
  footer: { verify: string; rights: string };
}
