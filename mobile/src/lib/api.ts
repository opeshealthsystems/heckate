import { clearToken, getToken } from "./auth";
import type { NewTicket } from "./drafts";
import type { Comment, Me, Status, Ticket } from "./types";

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
  registerDevice: (token: string, platform: "ios" | "android" | "expo") => call<void>("PUT", "/v1/devices", { token, platform }),
};
