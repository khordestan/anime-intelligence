import test from "node:test";
import assert from "node:assert/strict";
import { ApiClient, ApiError, StaleSessionError } from "./api.ts";

const session = (token) => ({
  token,
  expiresAt: "2099-01-01T00:00:00Z",
  user: { id: token, email: "synthetic@example.test", displayName: "" },
});

test("private requests bind the current bearer and never send cookies/cache", async () => {
  const calls = [];
  const client = new ApiClient("http://api.test", async (url, init) => {
    calls.push({ url, init });
    return Response.json({ items: [] });
  });
  client.setSession(session("synthetic-one"));
  await client.request("/tracker", {
    credentials: "include",
    cache: "force-cache",
    headers: { Authorization: "Bearer spoofed" },
  });
  assert.equal(
    calls[0].init.headers.get("Authorization"),
    "Bearer synthetic-one",
  );
  assert.equal(calls[0].init.credentials, "omit");
  assert.equal(calls[0].init.cache, "no-store");
  client.setSession(null);
  await client.request("/anime");
  assert.equal(calls[1].init.headers.has("Authorization"), false);
});

test("switching accounts discards late private responses even without transport cancellation", async () => {
  let finish;
  const client = new ApiClient(
    "http://api.test",
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  client.setSession(session("synthetic-one"));
  const pending = client.request("/tracker");
  client.setSession(session("synthetic-two"));
  finish(Response.json({ items: [{ title: "private result" }] }));
  await assert.rejects(pending, StaleSessionError);
});

test("signout during JSON decoding discards the old account response", async () => {
  let decoded;
  const client = new ApiClient("http://api.test", async () => ({
    ok: true,
    status: 200,
    json: () =>
      new Promise((resolve) => {
        decoded = resolve;
      }),
  }));
  client.setSession(session("synthetic-one"));
  const pending = client.request("/tracker");
  await new Promise((resolve) => setImmediate(resolve));
  client.setSession(null);
  decoded({ items: [{ title: "private result" }] });
  await assert.rejects(pending, StaleSessionError);
});

test("server details are hidden and unauthorized errors retain status", async () => {
  const client = new ApiClient("http://api.test", async () =>
    Response.json({ error: "internal database details" }, { status: 401 }),
  );
  await assert.rejects(
    client.request("/tracker"),
    (err) =>
      err instanceof ApiError &&
      err.status === 401 &&
      !err.message.includes("database"),
  );
});
