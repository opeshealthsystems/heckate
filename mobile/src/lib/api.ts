import { clearToken, getToken } from "./auth";
import type { NewTicket } from "./drafts";
import type { SendResult } from "./drafts-core";
import { classify } from "./drafts-core";
import type { QueuedAction } from "./queue-core";
import type { Comment, EmployeeRecord, Intervention, Me, Status, StockItem, Ticket, Timesheet, Warehouse } from "./types";

export const API_URL = process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(public status: number | null, public title: string, public detail: string) {
    super(detail ? `${title}: ${detail}` : title);
  }
}

let onUnauthorized: () => void = () => {};
export const setUnauthorizedHandler = (f: () => void) => (onUnauthorized = f);

async function call<T>(method: string, path: string, body?: unknown, extra: Record<string, string> = {}): Promise<T> {
  const headers: Record<string, string> = { ...extra };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  } catch {
    throw new ApiError(null, "Offline", "Cannot reach HEKATE");
  }
  if (res.status === 401 && token) {
    await clearToken();
    onUnauthorized();
  }
  if (!res.ok) {
    const p = await res.json().catch(() => ({}));
    throw new ApiError(res.status, p.title ?? res.statusText, p.detail ?? "");
  }
  return res.status === 204 ? (undefined as T) : res.json();
}

/** Replays one queued action. The original Idempotency-Key makes a retry after a lost response safe. */
export async function replay(a: QueuedAction): Promise<SendResult> {
  try {
    await call(a.method, a.path, a.body, { "Idempotency-Key": a.key });
    return "sent";
  } catch (e) {
    return classify(e instanceof ApiError ? e.status : null);
  }
}

export const api = {
  me: () => call<Me>("GET", "/v1/me"),
  tickets: () => call<{ data: Ticket[]; next_cursor: string | null }>("GET", "/v1/tickets?limit=50"),
  ticket: (id: string) => call<Ticket>("GET", `/v1/tickets/${id}`),
  createTicket: (b: NewTicket & { tenant_id?: string }, key: string) => call<Ticket>("POST", "/v1/tickets", b, { "Idempotency-Key": key }),
  updateTicket: (id: string, b: { status?: Status; assignee?: string }, version: number) =>
    call<Ticket>("PATCH", `/v1/tickets/${id}`, b, { "If-Match": `W/"${version}"` }),
  comments: (id: string) => call<{ data: Comment[] }>("GET", `/v1/tickets/${id}/comments`),
  addComment: (id: string, body: string, internal: boolean, key: string) =>
    call<Comment>("POST", `/v1/tickets/${id}/comments`, { body, internal }, { "Idempotency-Key": key }),
  rate: (id: string, rating: number) => call<Ticket>("POST", `/v1/tickets/${id}/rating`, { rating, comment: "" }),
  interventions: () => call<{ data: Intervention[] }>("GET", "/v1/interventions"),
  intervention: (id: string) => call<Intervention>("GET", `/v1/interventions/${id}`),
  items: () => call<{ data: StockItem[] }>("GET", "/v1/items"),
  warehouses: () => call<{ data: Warehouse[] }>("GET", "/v1/warehouses"),
  timesheets: () => call<{ data: Timesheet[] }>("GET", "/v1/timesheets"),
  logTime: (b: { work_date: string; hours: number; note: string }, key: string) => call<Timesheet>("POST", "/v1/timesheets", b, { "Idempotency-Key": key }),
  submitTimesheet: (id: string, key: string) => call<Timesheet>("POST", `/v1/timesheets/${id}/submit`, {}, { "Idempotency-Key": key }),
  myRecord: () => call<EmployeeRecord>("GET", "/v1/employees/me"),
  requestLeave: (b: { kind: string; start_date: string; end_date: string; reason: string }, key: string) => call<unknown>("POST", "/v1/leave", b, { "Idempotency-Key": key }),
  claimExpense: (b: { expense_date: string; category: string; description: string; amount: number }, key: string) => call<unknown>("POST", "/v1/expenses", b, { "Idempotency-Key": key }),
  registerDevice: (token: string, platform: "ios" | "android" | "expo") => call<void>("PUT", "/v1/devices", { token, platform }),
};
