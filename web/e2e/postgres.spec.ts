import { test, expect, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import type { Session } from "../lib/api";
import { verificationToken } from "./mailbox";

const API =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080/api/v1";
const password = "synthetic-full-stack-password";

async function withAuthBudget<T extends { status(): number; headers(): Record<string, string> }>(operation: () => Promise<T>): Promise<T> {
  for (let attempt = 0; attempt < 5; attempt++) {
    const response = await operation();
    if (response.status() !== 429) return response;
    const seconds = Number(response.headers()["retry-after"]);
    expect(seconds).toBeGreaterThan(0);
    expect(seconds).toBeLessThanOrEqual(10);
    await new Promise(resolve => setTimeout(resolve, seconds * 1000));
  }
  throw new Error("Fixture authentication budget did not recover within bounded retries");
}

// Multiple real tests share the process-wide authentication budget. Retry only
// an explicit 429, honor Retry-After and keep every other status a hard failure.
async function signInRespectingThrottle(page: Page, email: string) {
  for (let attempt = 0; attempt < 5; attempt++) {
    await page.getByLabel("Email", { exact: true }).fill(email);
    await page.getByLabel("Password", { exact: true }).fill(password);
    const pending = page.waitForResponse(response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/v1/auth/login");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    const response = await pending;
    if (response.status() === 200) {
      await expect(page.getByText(email, { exact: true })).toBeVisible();
      return;
    }
    expect(response.status()).toBe(429);
    const seconds = Number(response.headers()["retry-after"]);
    expect(seconds).toBeGreaterThan(0);
    expect(seconds).toBeLessThanOrEqual(10);
    await page.waitForTimeout(seconds * 1000);
  }
  throw new Error("Authentication budget did not recover within bounded retries");
}

test("real PostgreSQL auth, tracking, remote session control and account isolation", async ({
  page,
  request,
}) => {
  const suffix = randomUUID();
  const emailOne = `browser-one-${suffix}@example.test`;
  const emailTwo = `browser-two-${suffix}@example.test`;
  const headers = (token: string) => ({ Authorization: `Bearer ${token}` });
  async function create(email: string): Promise<Session> {
    const response = await request.post(API + "/auth/register", {
      data: { email, password },
    });
    expect(response.status()).toBe(201);
    const session: Session = await response.json();
    expect(
      (
        await request.post(API + "/auth/email-verification/request", {
          headers: headers(session.token),
        })
      ).status(),
    ).toBe(202);
    const token = await verificationToken(request, email);
    expect(
      (
        await request.post(API + "/auth/email-verification/confirm", {
          data: { token },
        })
      ).status(),
    ).toBe(204);
    return session;
  }
  async function login(email: string): Promise<Session> {
    const response = await request.post(API + "/auth/login", {
      data: { email, password },
    });
    expect(response.status()).toBe(200);
    return response.json();
  }
  async function signIn(email: string) {
    await page.getByLabel("Email", { exact: true }).fill(email);
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page.getByText(email, { exact: true })).toBeVisible();
  }
  const one = await create(emailOne),
    two = await create(emailTwo);
  await page.goto("/");
  await signIn(emailOne);
  await expect(page.getByRole("article")).toHaveCount(0);
  await page.getByPlaceholder("Search starter catalog…").fill("Solo Leveling");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const entry = page.getByRole("article");
  await expect(entry).toContainText("Solo Leveling");
  await entry.getByRole("button", { name: "+1 episode", exact: true }).click();
  await expect(entry).toContainText("1/12 episodes watched");
  const ratingResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname.startsWith("/api/v1/ratings/"),
  );
  await entry.getByLabel("Rating Solo Leveling").selectOption("4");
  expect((await ratingResponse).status()).toBe(200);
  await expect(entry.getByLabel("Rating Solo Leveling")).toHaveValue("4");
  await expect(
    page.getByRole("region", { name: "Active sessions" }),
  ).toHaveCount(1);
  await expect(
    page.getByRole("region", { name: "Password settings" }),
  ).toHaveCount(1);
  // Revoke an independently issued token using the actual dashboard/API.
  await page
    .getByRole("button", { name: "Manage sessions", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Active sessions" });
  await expect(
    panel.getByRole("button", { name: /^Revoke session/ }),
  ).toHaveCount(1);
  await panel.getByRole("button", { name: /^Revoke session/ }).click();
  await expect(panel.getByRole("status")).toHaveText("Session signed out.");
  expect(
    (
      await request.get(API + "/auth/me", { headers: headers(one.token) })
    ).status(),
  ).toBe(401);
  const remote = await login(emailOne);
  await panel
    .getByRole("button", { name: "Refresh sessions", exact: true })
    .click();
  await expect(
    panel.getByRole("button", { name: /^Revoke session/ }),
  ).toHaveCount(1);
  await panel
    .getByRole("button", { name: "Sign out other sessions", exact: true })
    .click();
  await expect(panel.getByRole("status")).toHaveText(
    "Signed out 1 other session.",
  );
  expect(
    (
      await request.get(API + "/auth/me", { headers: headers(remote.token) })
    ).status(),
  ).toBe(401);
  // Another valid session can revoke this browser, which must discard its data.
  const controller = await login(emailOne);
  const listResponse = await request.get(API + "/auth/sessions", {
    headers: headers(controller.token),
  });
  expect(listResponse.status()).toBe(200);
  const list = await listResponse.json();
  const browser = list.items.find(
    (item: { current: boolean }) => !item.current,
  );
  expect(browser).toBeTruthy();
  expect(
    (
      await request.delete(API + "/auth/sessions/" + browser.id, {
        headers: headers(controller.token),
      })
    ).status(),
  ).toBe(204);
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(panel).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Password settings" }),
  ).toHaveCount(0);
  // A second account sees no first-account history, despite the shared catalog.
  await signIn(emailTwo);
  await expect(page.getByRole("article")).toHaveCount(0);
  await page.getByPlaceholder("Search starter catalog…").fill("Solo Leveling");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("article")).toContainText(
    "0/12 episodes watched",
  );
  const savedResponse = await request.get(API + "/tracker", {
    headers: headers(controller.token),
  });
  expect(savedResponse.status()).toBe(200);
  const saved = await savedResponse.json();
  expect(saved.items).toHaveLength(1);
  expect(saved.items[0].episodesWatched).toBe(1);
  expect(saved.items[0].rating).toBe(4);
  expect(
    (
      await request.patch(API + "/tracker/" + saved.items[0].id, {
        headers: headers(two.token),
        data: { episodesWatched: 99 },
      })
    ).status(),
  ).toBe(404);
  // Remove only the second account's entry through the real UI/API, then
  // re-add with clean progress/rating while the first account remains intact.
  await page.getByRole("article").getByRole("button", { name: "Remove Solo Leveling", exact: true }).click();
  await page.getByRole("article").getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(1);
  await page.getByRole("article").getByRole("button", { name: "Remove Solo Leveling", exact: true }).click();
  const removed = page.waitForResponse(response => response.request().method() === "DELETE" && new URL(response.url()).pathname.startsWith("/api/v1/tracker/"));
  await page.getByRole("button", { name: "Confirm removal", exact: true }).click();
  expect((await removed).status()).toBe(204);
  await expect(page.getByRole("article")).toHaveCount(0);
  const untouched = await request.get(API + "/tracker", { headers: headers(controller.token) });
  expect((await untouched.json()).items[0].rating).toBe(4);
  const cleared = await request.get(API + "/tracker", { headers: headers(two.token) });
  expect((await cleared.json()).items).toHaveLength(0);
  await page.getByPlaceholder("Search starter catalog…").fill("Solo Leveling");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("article")).toContainText("0/12 episodes watched");
  await expect(page.getByLabel("Rating Solo Leveling")).toHaveValue("");
  const exported = await request.get(API + "/auth/export", {
    headers: headers(controller.token),
  });
  expect(exported.status()).toBe(200);
  const snapshot = await exported.json();
  expect(snapshot.user.id).toBe(one.user.id);
  expect(snapshot.data.tracker).toHaveLength(1);
  expect(JSON.stringify(snapshot)).not.toContain("password_hash");
  expect(
    (
      await request.delete(API + "/auth/account", {
        headers: headers(controller.token),
        data: { password },
      })
    ).status(),
  ).toBe(204);
  expect(
    (
      await request.get(API + "/auth/me", {
        headers: headers(controller.token),
      })
    ).status(),
  ).toBe(401);
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(page.getByText(emailTwo, { exact: true })).toBeVisible();
  await expect(page.getByRole("article")).toContainText(
    "0/12 episodes watched",
  );
  // Change the second account's password through the real UI; every old
  // session, including the API-issued token, must become invalid atomically.
  const replacement = "synthetic-replacement-password";
  await page
    .getByRole("button", { name: "Change password", exact: true })
    .click();
  const settings = page.getByRole("region", { name: "Password settings" });
  await settings.getByLabel("Current password", { exact: true }).fill(password);
  await settings.getByLabel("New password", { exact: true }).fill(replacement);
  await settings
    .getByRole("button", { name: "Update password", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  expect(
    (
      await request.get(API + "/auth/me", { headers: headers(two.token) })
    ).status(),
  ).toBe(401);
  await page.getByLabel("Email", { exact: true }).fill(emailTwo);
  await page.getByLabel("Password", { exact: true }).fill(replacement);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(emailTwo, { exact: true })).toBeVisible();
  await expect(page.getByRole("article")).toContainText(
    "0/12 episodes watched",
  );
  await page.screenshot({
    path: "test-results/postgres-browser-isolation.png",
    fullPage: true,
  });
});


test("real PostgreSQL exact episode editor and franchise preferences stay owner scoped", async ({ page, request }) => {
  const suffix = randomUUID();
  const sessions: Session[] = [];
  const emails = [`episode-owner-${suffix}@example.test`, `episode-other-${suffix}@example.test`];
  const headers = (token: string) => ({ Authorization: `Bearer ${token}` });
  for (const email of emails) {
    const registration = await withAuthBudget(() => request.post(API + "/auth/register", { data: { email, password } }));
    expect(registration.status()).toBe(201);
    const session: Session = await registration.json();
    sessions.push(session);
    expect((await withAuthBudget(() => request.post(API + "/auth/email-verification/request", { headers: headers(session.token) }))).status()).toBe(202);
    const proof = await verificationToken(request, email);
    expect((await withAuthBudget(() => request.post(API + "/auth/email-verification/confirm", { data: { token: proof } }))).status()).toBe(204);
  }
  await page.goto("/");
  await signInRespectingThrottle(page, emails[0]);
  await page.getByPlaceholder("Search starter catalog…").fill("Solo Leveling");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const article = page.getByRole("article").filter({ hasText: "Solo Leveling" });
  await article.getByRole("button", { name: "Edit watched episodes Solo Leveling", exact: true }).click();
  await article.getByLabel("Watched episode numbers Solo Leveling", { exact: true }).fill("3, 1");
  await article.getByRole("button", { name: "Replace episode selection", exact: true }).click();
  await expect(article).toContainText("2/12 episodes watched");
  await expect(article.getByRole("button", { name: "+1 episode", exact: true })).toBeDisabled();
  const library = await request.get(API + "/tracker", { headers: headers(sessions[0].token) });
  const saved = await library.json();
  expect(saved.items).toHaveLength(1);
  expect(saved.items[0].exactProgress).toBe(true);
  const path = API + "/tracker/" + saved.items[0].id + "/episodes";
  const progress = await request.get(path, { headers: headers(sessions[0].token) });
  expect((await progress.json()).numbers).toEqual([1, 3]);
  expect((await request.get(path, { headers: headers(sessions[1].token) })).status()).toBe(404);
  expect((await request.put(path, { headers: headers(sessions[1].token), data: { numbers: [2] } })).status()).toBe(404);
  const panel = page.getByRole("region", { name: "Release alerts", exact: true });
  await panel.getByRole("button", { name: "Manage release alerts", exact: true }).click();
  await panel.getByLabel("Release scope Solo Leveling", { exact: true }).selectOption("FRANCHISE");
  await panel.getByLabel("Release track Solo Leveling", { exact: true }).selectOption("DUB");
  await panel.getByRole("button", { name: "Follow releases Solo Leveling", exact: true }).click();
  await expect(panel.getByRole("button", { name: "Stop alerts Solo Leveling", exact: true })).toBeVisible();
  const follows = await request.get(API + "/follows", { headers: headers(sessions[0].token) });
  const owned = await follows.json();
  expect(owned.items).toHaveLength(1);
  expect(owned.items[0].scope).toBe("FRANCHISE");
  expect(owned.items[0].track).toBe("DUB");
  const foreign = await request.get(API + "/follows", { headers: headers(sessions[1].token) });
  expect((await foreign.json()).items).toEqual([]);
  await panel.getByRole("button", { name: "Stop alerts Solo Leveling", exact: true }).click();
  const taste = page.getByRole("region", { name: "Personal Match", exact: true });
  await taste.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  const preferenceRow = taste.getByRole("list", { name: "Feature preferences", exact: true }).getByRole("listitem").first();
  await preferenceRow.getByRole("spinbutton").fill("0.8");
  await preferenceRow.getByRole("button", { name: /^Save preference / }).click();
  await expect(taste.getByRole("status")).toHaveText("Preference updated.");
  await expect(taste.getByText(/No supported matches yet/)).toBeVisible();
  const ownPreferences = await request.get(API + "/preferences", { headers: headers(sessions[0].token) });
  expect((await ownPreferences.json()).items.some((p: { weight: number | null }) => p.weight === 0.8)).toBe(true);
  const foreignPreferences = await request.get(API + "/preferences", { headers: headers(sessions[1].token) });
  expect((await foreignPreferences.json()).items.every((p: { weight: number | null }) => p.weight === null)).toBe(true);

  await expect(panel.getByRole("button", { name: "Follow releases Solo Leveling", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(panel).toHaveCount(0);
  await signInRespectingThrottle(page, emails[1]);
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(page.getByRole("group", { name: "Episode selection Solo Leveling", exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => ({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) }))).toEqual({ local: [], session: [] });
});
