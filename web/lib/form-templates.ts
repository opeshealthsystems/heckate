/** Starting points for online forms. The sender can edit the questions before sending; none of these is a numbered pack form. */
export interface FormField { key: string; label: string; type: "text" | "textarea" | "number" | "date" | "select" | "checkbox"; required?: boolean; options?: string[] }
export interface FormTemplate { id: string; title: string; fields: FormField[] }

export const FORM_TEMPLATES: FormTemplate[] = [
  {
    id: "site-survey", title: "Site survey",
    fields: [
      { key: "site_name", label: "Site name", type: "text", required: true },
      { key: "address", label: "Address and access instructions", type: "textarea", required: true },
      { key: "contact", label: "On-site contact (name and phone)", type: "text", required: true },
      { key: "racks", label: "Number of racks", type: "number" },
      { key: "power", label: "Power supply", type: "select", options: ["Mains only", "Mains and generator", "Solar"], required: true },
      { key: "ups", label: "A UPS is installed", type: "checkbox" },
      { key: "visit_date", label: "Preferred survey date", type: "date" },
      { key: "notes", label: "Anything else we should know", type: "textarea" },
    ],
  },
  {
    id: "contact-details", title: "Company contact and invoicing details",
    fields: [
      { key: "legal_name", label: "Legal name", type: "text", required: true },
      { key: "rccm", label: "RCCM number", type: "text" },
      { key: "niu", label: "Tax number (NIU)", type: "text" },
      { key: "billing_address", label: "Billing address", type: "textarea", required: true },
      { key: "billing_contact", label: "Person who receives invoices (name and email)", type: "text", required: true },
      { key: "po_required", label: "Invoices need a purchase order number", type: "checkbox" },
    ],
  },
  {
    id: "incident", title: "Incident report",
    fields: [
      { key: "when", label: "When did it start", type: "date", required: true },
      { key: "what", label: "What happened", type: "textarea", required: true },
      { key: "impact", label: "Impact", type: "select", options: ["No service affected", "Some users affected", "A whole site is down"], required: true },
      { key: "actions", label: "What has been done so far", type: "textarea" },
    ],
  },
  {
    id: "acceptance", title: "Delivery acceptance",
    fields: [
      { key: "delivered", label: "All items were delivered", type: "checkbox", required: true },
      { key: "tested", label: "The work was tested in front of you", type: "checkbox", required: true },
      { key: "remarks", label: "Remarks or defects", type: "textarea" },
      { key: "accepted_by", label: "Name of the person accepting", type: "text", required: true },
      { key: "accepted_on", label: "Date", type: "date", required: true },
    ],
  },
];

export type Answers = Record<string, string | number | boolean | null | undefined>;

/** Turns what was typed into what the API stores: numbers as numbers, empty text left out, a checkbox as yes or no. */
export function cleanAnswers(fields: FormField[], raw: Answers): Record<string, string | number | boolean> {
  const out: Record<string, string | number | boolean> = {};
  for (const f of fields) {
    const v = raw[f.key];
    if (f.type === "checkbox") { out[f.key] = v === true; continue; }
    if (v === undefined || v === null || v === "") continue;
    out[f.key] = f.type === "number" ? Number(v) : String(v);
  }
  return out;
}
