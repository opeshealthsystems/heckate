export interface Line { description: string; quantity: number; unit_price: number }
export interface Quote {
  id: string; tenant_id: string; ref: string | null; title: string; status: "draft" | "sent" | "approved" | "rejected";
  valid_until: string | null; notes: string; created_at: string; lines?: Line[];
  net: number; tva: number; cac: number; total: number;
}
export interface Invoice {
  id: string; tenant_id: string; quote_id: string | null; ref: string | null; title: string;
  status: "draft" | "pending_approval" | "issued" | "paid"; issue_date: string | null; due_date: string | null;
  net: number; tva: number; cac: number; total: number; paid: number; credited: number; balance: number;
  created_by: string; requested_by: string | null; approved_by: string | null; created_at: string; lines?: Line[];
}
