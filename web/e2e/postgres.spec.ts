import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import type { Session } from "../lib/api";

const API =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080/api/v1";
const password = "synthetic-full-stack-password";

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
    return response.json();
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
  await page.screenshot({
    path: "test-results/postgres-browser-isolation.png",
    fullPage: true,
  });
});
