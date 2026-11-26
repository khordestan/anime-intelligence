import { expect, test } from "@playwright/test";

test("browser alerts require explicit enrollment and support withdrawal", async ({ page }) => {
  await page.addInitScript(() => {
    let subscription: { toJSON: () => unknown; unsubscribe: () => Promise<boolean> } | null = null;
    const pushManager = {
      getSubscription: async () => subscription,
      subscribe: async () => {
        subscription = {
          toJSON: () => ({ endpoint: "https://fcm.googleapis.com/fcm/send/synthetic", keys: { auth: "synthetic", p256dh: "synthetic" } }),
          unsubscribe: async () => { subscription = null; return true; },
        };
        return subscription;
      },
    };
    const worker = { active: { state: "activated" }, pushManager };
    Object.defineProperty(window, "PushManager", { configurable: true, value: class {} });
    Object.defineProperty(window, "Notification", { configurable: true, value: { requestPermission: async () => "granted" } });
    Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: {
      register: async (script: string, options: RegistrationOptions) => {
        if (script !== "/release-push.js" || options.scope !== "/release-push/") throw new Error("Unexpected service worker");
        return worker;
      },
      getRegistration: async () => worker,
    } });
  });
  const calls: { method: string; body: unknown }[] = [];
  const registration = { id: "10000000-0000-4000-8000-000000000001", createdAt: "2026-01-01T00:00:00Z" };
  let enabled = false;
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
    };
    const reply = (json: unknown) => route.fulfill({ status: 200, headers, json });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return reply({ authenticationRequired: true });
    if (path === "/auth/login") return reply({
      token: "synthetic-test-token",
      expiresAt: new Date(Date.now() + 60000).toISOString(),
      user: { id: "synthetic-owner", email: "push@example.test", emailVerified: true },
    });
    if (path === "/releases/browser-push" || path === "/releases/browser-push/" + registration.id) {
      calls.push({ method: request.method(), body: request.postData() ? request.postDataJSON() : null });
      if (request.method() === "PUT") {
        enabled = true;
        return reply(registration);
      }
      if (request.method() === "DELETE") {
        enabled = false;
        return route.fulfill({ status: 204, headers });
      }
      return reply({ publicKey: "AQ", registration: enabled ? registration : null });
    }
    return reply({ items: [] });
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("push@example.test");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.getByRole("button", { name: "Manage release alerts", exact: true }).click();
  await page.getByText("Background browser alerts", { exact: true }).click();
  expect(calls).toEqual([]);
  await page.getByRole("button", { name: "Check browser alert settings" }).click();
  await expect(page.getByText("No browser is registered.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Enable on this browser" }).click();
  await expect(page.getByText("Your account has a browser registered.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Disable browser alerts" }).click();
  await expect(page.getByText("No browser is registered.", { exact: true })).toBeVisible();
  expect(calls).toEqual([
    { method: "GET", body: null },
    { method: "PUT", body: { endpoint: "https://fcm.googleapis.com/fcm/send/synthetic", keys: { auth: "synthetic", p256dh: "synthetic" } } },
    { method: "DELETE", body: null },
  ]);
});
