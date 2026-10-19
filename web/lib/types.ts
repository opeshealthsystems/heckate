// Hand-written to match api/openapi/openapi.yaml until SDK generation is wired into CI.
export type Severity = "P1" | "P2" | "P3" | "P4";
export type Status = "open" | "in_progress" | "resolved" | "closed";
export type Channel = "app" | "portal" | "email" | "phone";

export interface Ticket {
  id: string;
  tenant_id: string;
  subject: string;
  description: string;
  severity: Severity;
  channel: Channel;
  status: Status;
  reporter: string;
  assignee: string | null;
  respond_due_at: string | null;
  sla_due_at: string | null;
  first_response_at: string | null;
  resolved_at: string | null;
  sla_breached: boolean;
  rating: number | null;
  rating_comment: string | null;
  version: number;
  created_at: string;
}

export interface Comment {
  id: string;
  ticket_id: string;
  author: string;
  body: string;
  internal: boolean;
  created_at: string;
}

export interface Client {
  tenant_id: string;
  legal_name: string;
  rccm: string | null;
  email: string | null;
  phone: string | null;
  kyc_status: "pending" | "verified" | "rejected";
  created_at: string;
}

export interface Lead {
  id: string;
  name: string;
  email: string;
  phone: string | null;
  company: string | null;
  message: string;
  source: string;
  stage: string;
  owner: string | null;
  created_at: string;
}

export interface Me {
  sub: string;
  email: string;
  tenant_id: string;
  roles: string[];
  scope: "staff" | "client" | "other";
}

export interface DocumentEntry {
  ref: string;
  code: string;
  title: string;
  tenant_id: string | null;
  status: "valid" | "revoked";
  issued_by: string;
  issued_at: string;
}

export interface VerificationSignature { checked: boolean; valid: boolean; scheme?: string; reason?: string }
export interface Verification {
  valid: boolean;
  result?: "authentic" | "replaced" | "revoked" | "not_found" | "malformed" | "signature_invalid";
  type?: string;
  version?: string;
  classification?: string;
  issued_by?: string;
  verified_count?: number;
  last_verified?: string | null;
  signature?: VerificationSignature;
  reason?: string;
  ref?: string;
  code?: string;
  title?: string;
  status?: string;
  issued_at?: string;
  sha256?: string;
  superseded_by?: string;
}

export interface SupportReport {
  open: number;
  sla_breached: number;
  p1_open: number;
  rated: number;
  avg_rating: number | null;
}
