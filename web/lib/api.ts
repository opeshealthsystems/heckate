import { clearToken, getToken } from "./auth";
import type {
  Client, Comment, DocumentEntry, Lead, Me, Severity, Channel, Status, SupportReport, Ticket, Verification,
} from "./types";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(public status: number, public title: string, public detail: string) {
    super(detail ? `${title}: ${detail}` : title);
  }
}

async function call<T>(method: string, path: string, body?: unknown, extra: Record<string, string> = {}): Promise<T> {
  const headers: Record<string, string> = { ...extra };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method === "POST") headers["Idempotency-Key"] = crypto.randomUUID();
  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  } catch {
    // Offline, DNS failure or a navigation aborting the request: not an authentication problem.
    throw new ApiError(0, "Network error", "Cannot reach the server. Check your connection.");
  }
  if (res.status === 401 && token) {
    clearToken();
    if (typeof location !== "undefined") location.assign("/login");
  }
  if (!res.ok) {
    const p = await res.json().catch(() => ({}));
    throw new ApiError(res.status, p.title ?? res.statusText, p.detail ?? "");
  }
  return res.status === 204 ? (undefined as T) : res.json();
}

export interface Attachment {
  id: string; entity_type: string; entity_id: string; filename: string; content_type: string; size: number; created_at: string; owner: string;
}

/** Uploads one file; the browser sets the multipart boundary, so no Content-Type header here. */
export async function uploadAttachment(entityType: string, entityId: string, file: File): Promise<Attachment> {
  const form = new FormData();
  form.set("entity_type", entityType);
  form.set("entity_id", entityId);
  form.set("file", file);
  const token = getToken();
  let res: Response;
  try {
    res = await fetch(`${API_URL}/v1/attachments`, { method: "POST", headers: token ? { Authorization: `Bearer ${token}` } : {}, body: form });
  } catch {
    throw new ApiError(0, "Network error", "Cannot reach the server. Check your connection.");
  }
  if (!res.ok) {
    const p = await res.json().catch(() => ({}));
    throw new ApiError(res.status, p.title ?? res.statusText, p.detail ?? "");
  }
  return res.json();
}

/** Downloads an attachment with the bearer token and hands it to the browser as a file. */
export async function downloadAttachment(a: Pick<Attachment, "id" | "filename">): Promise<void> {
  const token = getToken();
  const res = await fetch(`${API_URL}/v1/attachments/${a.id}`, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
  if (!res.ok) throw new ApiError(res.status, "Download failed", "");
  const url = URL.createObjectURL(await res.blob());
  const link = document.createElement("a");
  link.href = url;
  link.download = a.filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

/** Generic call for endpoints without a hand-written wrapper. */
export const raw = {
  get: <T,>(path: string) => call<T>("GET", path),
  post: <T,>(path: string, body?: unknown) => call<T>("POST", path, body ?? {}),
  put: <T,>(path: string, body?: unknown) => call<T>("PUT", path, body ?? {}),
  patch: <T,>(path: string, body?: unknown) => call<T>("PATCH", path, body ?? {}),
};

export const api = {
  me: () => call<Me>("GET", "/v1/me"),
  tickets: (q: { status?: string; severity?: string; cursor?: string } = {}) => {
    const p = new URLSearchParams();
    Object.entries(q).forEach(([k, v]) => v && p.set(k, v));
    return call<{ data: Ticket[]; next_cursor: string | null }>("GET", `/v1/tickets?${p}`);
  },
  ticket: (id: string) => call<Ticket>("GET", `/v1/tickets/${id}`),
  createTicket: (b: { tenant_id?: string; subject: string; description: string; severity: Severity; channel: Channel }) =>
    call<Ticket>("POST", "/v1/tickets", b),
  updateTicket: (id: string, b: { status?: Status; assignee?: string }, etag?: string) =>
    call<Ticket>("PATCH", `/v1/tickets/${id}`, b, etag ? { "If-Match": etag } : {}),
  comments: (id: string) => call<{ data: Comment[] }>("GET", `/v1/tickets/${id}/comments`),
  addComment: (id: string, body: string, internal = false) => call<Comment>("POST", `/v1/tickets/${id}/comments`, { body, internal }),
  rate: (id: string, rating: number, comment: string) => call<Ticket>("POST", `/v1/tickets/${id}/rating`, { rating, comment }),
  report: () => call<SupportReport>("GET", "/v1/reports/support"),
  clients: () => call<{ data: Client[] }>("GET", "/v1/clients"),
  createClient: (b: { legal_name: string; rccm?: string; email?: string; phone?: string }) => call<Client>("POST", "/v1/clients", b),
  leads: () => call<{ data: Lead[] }>("GET", "/v1/leads"),
  updateLead: (id: string, b: { stage?: string; owner?: string }) => call<Lead>("PATCH", `/v1/leads/${id}`, b),
  issueDocument: (b: { code: string; title: string; tenant_id?: string }) => call<DocumentEntry>("POST", "/v1/documents", b),
  verify: (ref: string) => call<Verification>("GET", `/v1/public/verify/${encodeURIComponent(ref)}`),
};
