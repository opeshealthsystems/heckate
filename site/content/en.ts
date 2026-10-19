import type { Dict } from "./types";

export const en: Dict = {
  lang: "en",
  locale: "en_CM",
  nav: { home: "Home", services: "Services", about: "About", contact: "Contact", portal: "Client portal", verify: "Verify a document" },
  home: {
    title: "HEKATE Systems — cybersecurity, digital identity and ICT services in Cameroon",
    description: "HEKATE Systems provides penetration testing, security assessments, digital identity verification and incident response from Douala, Cameroon.",
    h1: "Cybersecurity and digital identity services, built in Cameroon",
    lead: "We test your defences, verify identities and respond when something goes wrong, with a client portal that shows every ticket, document and certificate in one place.",
    cta: "Talk to us",
    ctaSupport: "Report an incident",
    servicesH: "What we do",
    platformH: "One platform for your support and your documents",
    platform: [
      "Report an incident from the portal or the mobile app and follow it live.",
      "Every ticket carries a service-level timer, so you can see when we owe you a response.",
      "Quotes, invoices, contracts and reports are available in your client space.",
      "Every document we issue has a reference and a public verification page.",
    ],
    trustH: "Trust and security",
    trust: [
      "Two-factor sign-in for all staff and role-based access to client data.",
      "Each client sees only their own records, enforced in the database.",
      "Encrypted storage and backups, with an audit trail of every change.",
      "Our platform is penetration tested before it goes live.",
    ],
    supportH: "Need help now?",
    support: "Clients can open a ticket in the portal or the app. For an emergency, write to the incident address below and we will open a P1 ticket.",
  },
  services: {
    title: "Services — HEKATE Systems",
    description: "Penetration testing, security assessment, hecID digital identity verification, background screening and incident response.",
    h1: "Services",
    items: [
      {
        slug: "penetration-testing",
        name: "Penetration testing",
        summary: "Authorised, scoped attacks on your applications, networks and infrastructure, with a written report and a re-test.",
        faq: [
          { q: "What does a penetration test cost in Cameroon?", a: "The price depends on scope: number of applications, networks and environments. We scope each engagement first and send a fixed quotation before any work starts. Use the contact form to request one." },
          { q: "What do we receive at the end?", a: "A signed, restricted report with findings ranked by severity and fixes to apply. A re-test of fixed findings is tracked against the engagement." },
          { q: "Do you need our written permission?", a: "Yes. Every test starts with a signed authorisation and rules of engagement." },
        ],
      },
      {
        slug: "security-assessment",
        name: "Security assessment",
        summary: "A structured review of your controls, policies and configuration against recognised good practice.",
        faq: [
          { q: "How is an assessment different from a penetration test?", a: "An assessment reviews how you are set up: policies, configuration and processes. A penetration test attacks the systems to prove what can be exploited. Many clients do both." },
          { q: "Can you issue a certificate?", a: "Where an assessment meets the criteria we agree up front, we issue a certificate with a reference that anyone can check on our verification page." },
        ],
      },
      {
        slug: "digital-identity",
        name: "Digital identity (hecID) and e-KYC",
        summary: "Identity verification and e-KYC integration so you know who your customers are.",
        faq: [
          { q: "What is hecID?", a: "hecID is HEKATE's digital identity service. It lets organisations verify a person's identity electronically as part of onboarding." },
          { q: "How do we integrate it?", a: "Submit an e-KYC integration request through the contact form and we will agree the scope and technical steps with your team." },
        ],
      },
      {
        slug: "background-screening",
        name: "Background screening",
        summary: "Consent-based background checks for hiring and partner due diligence, handled as restricted data.",
        faq: [
          { q: "Do you need the person's consent?", a: "Yes. Screening only starts with a signed consent, and results are handled as restricted information." },
        ],
      },
      {
        slug: "incident-response",
        name: "Incident response",
        summary: "Emergency help when you are under attack or have found a compromise, with every action logged.",
        faq: [
          { q: "How do I report an incident?", a: "Clients open a ticket in the portal or app and mark it P1 if the service is down. You can also email the incident address; every channel becomes one ticket." },
          { q: "Who responds to a critical incident?", a: "A P1 pages the on-call engineer and a manager at the same time, and the service-level clock starts immediately." },
        ],
      },
    ],
  },
  about: {
    title: "About — HEKATE Systems",
    description: "HEKATE SYSTEMS SARL is a cybersecurity and digital identity company based in Douala, Cameroon.",
    h1: "About HEKATE",
    body: [
      "HEKATE SYSTEMS SARL is a cybersecurity and digital identity company based in Douala, Cameroon.",
      "We help organisations test their defences, verify identities and respond to incidents, and we run our own client platform so that support, documents and certificates are tracked in one place.",
    ],
  },
  contact: {
    title: "Contact — HEKATE Systems",
    description: "Contact HEKATE Systems in Douala, Cameroon, or report an incident.",
    h1: "Contact us",
    intro: "Tell us what you need and we will reply by email.",
    emergencyH: "Incident or emergency",
    emergency: "Clients: open a P1 ticket in the portal or the app. Otherwise write to",
    name: "Your name", email: "Email", phone: "Phone (optional)", company: "Company (optional)", message: "How can we help?",
    send: "Send", sending: "Sending…",
    thanks: "Thank you. We have your message and will reply by email.",
    error: "Sorry, we could not send your message. Please email us instead.",
    note: "We use your details only to reply to you.",
  },
  footer: { verify: "Verify a HEKATE document", rights: "All rights reserved." },
};
