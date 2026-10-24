import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema";

export type { paths, components } from "./schema";

export interface Options {
  /** Base URL of the API, for example https://api.hekatesystems.com */
  baseUrl: string;
  /** An API key (hk_...) or an OIDC access token. */
  token: string;
  /** For tests or non-browser runtimes; defaults to the global fetch. */
  fetch?: typeof fetch;
}

/**
 * A typed client for the HEKATE API. Every path, parameter and response body is checked against the
 * OpenAPI contract at compile time. POST requests get an Idempotency-Key automatically, so a retried
 * request is never applied twice.
 */
export function createHekate({ baseUrl, token, fetch }: Options) {
  const client = createClient<paths>({ baseUrl, fetch });
  const auth: Middleware = {
    onRequest({ request }) {
      request.headers.set("Authorization", `Bearer ${token}`);
      if (request.method === "POST" && !request.headers.has("Idempotency-Key")) {
        request.headers.set("Idempotency-Key", crypto.randomUUID());
      }
      return request;
    },
  };
  client.use(auth);
  return client;
}
