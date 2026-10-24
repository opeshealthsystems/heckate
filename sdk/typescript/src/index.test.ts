import test from "node:test";
import assert from "node:assert/strict";
import { createHekate } from "./index.ts";

function fake() {
  const seen: Request[] = [];
  const fetch = async (req: Request) => {
    seen.push(req);
    return new Response(JSON.stringify({ data: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
  };
  return { seen, fetch: fetch as unknown as typeof globalThis.fetch };
}

test("sends the bearer token and no idempotency key on reads", async () => {
  const f = fake();
  const api = createHekate({ baseUrl: "https://api.example.com", token: "hk_abc_def", fetch: f.fetch });
  const { data } = await api.GET("/v1/tickets");
  assert.deepEqual(data, { data: [] });
  assert.equal(f.seen[0].headers.get("Authorization"), "Bearer hk_abc_def");
  assert.equal(f.seen[0].headers.get("Idempotency-Key"), null);
  assert.equal(new URL(f.seen[0].url).pathname, "/v1/tickets");
});

test("adds a fresh idempotency key to every POST", async () => {
  const f = fake();
  const api = createHekate({ baseUrl: "https://api.example.com", token: "t", fetch: f.fetch });
  await api.POST("/v1/payroll/runs", { body: { period: "2026-10" } } as never);
  await api.POST("/v1/payroll/runs", { body: { period: "2026-11" } } as never);
  const keys = f.seen.map((r) => r.headers.get("Idempotency-Key"));
  assert.equal(keys.length, 2);
  assert.ok(keys.every(Boolean));
  assert.notEqual(keys[0], keys[1]);
});
