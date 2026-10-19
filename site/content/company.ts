// Single source of truth for company facts used on every page and in JSON-LD.
// Leave a value empty until it is confirmed: empty fields are omitted from structured data
// instead of publishing guesses. See docs/BLOCKERS.md.
export const company = {
  legalName: "HEKATE SYSTEMS SARL",
  brand: "HEKATE",
  url: process.env.NEXT_PUBLIC_SITE_URL ?? "https://hekatesystems.com",
  apiUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  portalUrl: process.env.NEXT_PUBLIC_PORTAL_URL ?? "http://localhost:3000",
  incidentEmail: "incident@hekatesystems.com",
  city: "Douala",
  country: "CM",
  // To be supplied by the company:
  rccm: "",
  streetAddress: "",
  phone: "",
  emergencyPhone: "",
  founders: [] as string[],
};

export type Company = typeof company;
