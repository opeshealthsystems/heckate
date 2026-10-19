// Single source of truth for company facts used on every page and in JSON-LD.
// Leave a value empty until it is confirmed: empty fields are omitted from the pages and from structured
// data instead of publishing guesses or placeholders. See docs/BLOCKERS.md.
export const company = {
  legalName: "HEKATE SYSTEMS SARL",
  brand: "HEKATE SYSTEMS",
  url: process.env.NEXT_PUBLIC_SITE_URL ?? "https://hekatesystems.com",
  apiUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  portalUrl: process.env.NEXT_PUBLIC_PORTAL_URL ?? "http://localhost:3000",
  incidentEmail: "incident@hekatesystems.com",
  emails: {
    contact: "contact@hekatesystems.com",
    sales: "sales@hekatesystems.com",
    support: "support@hekatesystems.com",
    security: "security@hekatesystems.com",
    careers: "careers@hekatesystems.com",
  },
  addressLines: ["Bonamoussadi, Petit Terrain", "Douala V, Wouri, Littoral, Cameroon"],
  streetAddress: "Bonamoussadi, Petit Terrain",
  district: "Douala V",
  region: "Littoral",
  city: "Douala",
  country: "CM",
  latitude: 4.08508,
  longitude: 9.73863,
  founders: ["Jude Nshome", "Tardzefe Boris Bandin"],
  // To be supplied by the company; shown only once filled in:
  rccm: "",
  niu: "",
  phone: "",
  poBox: "",
  emergencyPhone: "",
};

export type Company = typeof company;
