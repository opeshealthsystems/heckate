import type { FormEntry } from "./forms-library";
import type { PreviewSource } from "@/components/DocumentPreview";

export interface Narrative { id: string; kind: string; form_code: string | null; tenant_id: string; title: string; ref: string | null; created_at: string; revision: number; supersedes: string | null; superseded_by: string | null }
export interface Section { heading: string; body: string }
export interface Version { version: number; event: "created" | "edited" | "issued"; title: string; sections: Section[]; signed_by: string | null; change_note: string; changed_by: string; changed_at: string; content_sha256: string }

/** Library entries the platform can fill in as a text document, and how it files them. */
export const FILLABLE: Record<string, { kind: string; form?: string }> = {
  "HS-PR": { kind: "proposal" }, "HS-OC": { kind: "order_confirmation" }, "HS-SOW": { kind: "sow" }, "HS-RPT": { kind: "pentest_report" },
  ...Object.fromEntries(["06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "23", "24", "26", "31", "32", "33", "37", "39", "41"].map((n) => [`HS-FRM-${n}`, { kind: "form", form: `FRM-${n}` }])),
};

/** The approved blank template, exactly as signed (its SHA-256 is in the register), for looking at before filling in. */
export function formPreview(f: FormEntry): PreviewSource {
  return {
    title: `${f.code} · ${f.title}`,
    filename: `${f.code}.pdf`,
    note: `Blank approved template, version ${f.version} · ${f.cls} · SHA-256 ${f.sha256.slice(0, 16)}…`,
    load: async () => {
      const r = await fetch(f.file);
      if (!r.ok) throw new Error("Template not available");
      return r.blob();
    },
  };
}

/** The folders of the approved pack, in its own order (A to F). */
export const FOLDER_ORDER = ["Stationery", "Client Intake", "Commercial", "Project Delivery", "HR", "Suppliers"];
