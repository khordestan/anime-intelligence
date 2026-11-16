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

test("native-style fetch receives the global receiver", async () => {
  const client = new ApiClient("http://api.test", function () {
    assert.equal(this, globalThis);
    return Promise.resolve(Response.json({ ok: true }));
  });
  assert.deepEqual(await client.request("/metadata"), { ok: true });
});

test("password reauthentication errors expose only allowlisted codes", async () => {
  const client = new ApiClient("http://api.test", async () =>
    Response.json(
      { error: "invalid_current_password", message: "private server detail" },
      { status: 401 },
    ),
  );
  await assert.rejects(
    client.request("/auth/password"),
    (error) =>
      error instanceof ApiError &&
      error.code === "invalid_current_password" &&
      !error.message.includes("private"),
  );
});

test("late unauthorized JSON cannot sign out a different account", async () => {
  let decoded;
  const client = new ApiClient("http://api.test", async () => ({
    ok: false,
    status: 401,
    json: () =>
      new Promise((resolve) => {
        decoded = resolve;
      }),
  }));
  client.setSession(session("synthetic-one"));
  const pending = client.request("/auth/password");
  await new Promise((resolve) => setImmediate(resolve));
  client.setSession(session("synthetic-two"));
  decoded({ error: "unauthorized" });
  await assert.rejects(pending, StaleSessionError);
});

test("verification denial preserves a typed policy error and hides server details", async () => {
  const client = new ApiClient("http://api.test", async () =>
    Response.json(
      {
        error: "email_verification_required",
        message: "private server internals",
      },
      { status: 403 },
    ),
  );
  client.setSession(session("synthetic-unverified"));
  await assert.rejects(
    client.request("/tracker"),
    (error) =>
      error instanceof ApiError &&
      error.status === 403 &&
      error.code === "email_verification_required" &&
      error.message === "Verify your email before using your library.",
  );
});


test("late DELETE completion cannot cross an account boundary and never decodes 204", async () => {
  let complete;
  const client = new ApiClient("http://api.test", () => new Promise(resolve => { complete = resolve; }));
  client.setSession(session("synthetic-one"));
  const pending = client.request("/tracker/synthetic-entry", {method: "DELETE"});
  client.setSession(session("synthetic-two"));
  complete({ok: true, status: 204, json: () => { throw new Error("204 body must not be decoded"); }});
  await assert.rejects(pending, StaleSessionError);
});


test("catalog filter validation has a safe specific message", async () => {
  const client = new ApiClient("http://api.test", async () =>
    Response.json({ error: "invalid_catalog_filter", message: "synthetic hidden SQL details" }, { status: 400 }),
  );
  await assert.rejects(client.request("/anime?genre=invalid"), error =>
    error instanceof ApiError && error.code === "invalid_catalog_filter" &&
    error.message === "Check the catalog filters and try again.",
  );
});
