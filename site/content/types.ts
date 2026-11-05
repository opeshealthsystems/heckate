export type Lang = "en" | "fr";

/** One of the six practices shown on the home page and the services page. */
export interface Service {
  slug: string;
  n: string;
  name: string;
  summary: string;
}

export interface Faq {
  q: string;
  a: string;
}

export interface Group {
  h: string;
  note?: string;
  items: { name?: string; text: string }[];
}

/** Blocks of a long document such as the guarantee policies. */
export type Block =
  | { t: "p"; text: string }
  | { t: "ul"; items: string[] }
  | { t: "ol"; items: string[] }
  | { t: "table"; head: string[]; rows: string[][] };

export interface Dict {
  lang: Lang;
  locale: string;
  nav: { services: string; platform: string; identity: string; about: string; contact: string; portal: string; report: string; verify: string };
  home: {
    title: string;
    description: string;
    eyebrow: string;
    h1: string;
    lead: string;
    ctaPrimary: string;
    ctaSecondary: string;
    portal: { h: string; emailLabel: string; cta: string; note: string; verify: string };
    sectors: string[];
    servicesLabel: string;
    servicesH: string;
    platformLabel: string;
    platformH: string;
    platformLead: string;
    platformPoints: string[];
    identityLabel: string;
    identityH: string;
    identityLead: string;
    identityItems: { h: string; t: string }[];
    guaranteesLabel: string;
    guaranteesH: string;
    guarantees: { h: string; t: string }[];
    guaranteesLink: string;
    verifyH: string;
    verifyBody: string;
    verifyField: string;
    verifyCta: string;
    supportLabel: string;
    supportH: string;
    steps: { h: string; t: string }[];
  };
  services: {
    title: string;
    description: string;
    h1: string;
    intro: string;
    practices: Service[];
    groups: Group[];
    faqH: string;
    faq: Faq[];
  };
  about: {
    title: string;
    description: string;
    label: string;
    h1: string;
    lead: string;
    missionH: string;
    mission: string;
    howH: string;
    how: string;
    method: string;
    whereH: string;
    where: string;
    leadershipH: string;
    leaders: { initials: string; name: string; role: string; bio: string }[];
    platformsH: string;
    platforms: { name: string; text: string }[];
    sectorsH: string;
    detailsH: string;
    detailLabels: { legalName: string; legalForm: string; office: string; coordinates: string; website: string; rccm: string; niu: string; phone: string };
    legalForm: string;
    policiesH: string;
  };
  contact: {
    title: string;
    description: string;
    h1: string;
    intro: string;
    emergency: string;
    emergencyCta: string;
    fullName: string; organisation: string; email: string; phone: string; service: string; servicePick: string; message: string;
    consent: string; send: string; sending: string; thanks: string; error: string;
    officeH: string; departmentsH: string; departments: { address: string; purpose: string }[]; phonePostH: string; hoursH: string; hours: string;
  };
  guarantees: {
    title: string;
    description: string;
    h1: string;
    meta: string;
    notice?: string;
    intro: string;
    sections: { h: string; blocks: Block[] }[];
  };
  footer: {
    companyH: string; clientsH: string; legalH: string;
    about: string; contact: string; clientPortal: string; verify: string; guarantees: string;
    rights: string; otherLang: string; app: string;
  };
}
