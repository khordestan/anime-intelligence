import { test, expect, type Page } from "@playwright/test";

async function mockAPI(
  page: Page,
  authenticated = true,
  identity = false,
  verified = true,
) {
  const entries = new Map<string, object[]>();
  let issued = 0;
  const requests: { path: string; token: string }[] = [];
  await page.route("http://localhost:8080/api/v1**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace(
      "Bearer ",
      "",
    );
    requests.push({ path, token });
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
    };
    const respond = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        headers,
        contentType: "application/json",
        body: JSON.stringify(body),
      });
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    if (path === "")
      return respond({
        authenticationRequired: authenticated,
        identityDeliveryAvailable: identity,
      });
    if (path === "/anime")
      return respond({
        items: [
          {
            id: "10000000-0000-4000-8000-000000000001",
            title: "Synthetic Catalog Title",
            mediaType: "TV",
          },
        ],
      });
    if (path === "/auth/register" || path === "/auth/login") {
      const email = request.postDataJSON().email as string;
      const token = String(++issued).padStart(43, "a");
      entries.set(token, []);
      return respond(
        {
          token,
          user: {
            id: String(issued),
            email,
            displayName: "",
            emailVerified: verified,
          },
          expiresAt: new Date(Date.now() + 86_400_000).toISOString(),
        },
        path.endsWith("register") ? 201 : 200,
      );
    }
    if (authenticated && !entries.has(token))
      return respond({ error: "unauthorized" }, 401);
    if (path === "/auth/logout") {
      entries.delete(token);
      return route.fulfill({ status: 204, headers });
    }
    if (path.startsWith("/tracker/") && request.method() === "DELETE") {
      const items = entries.get(token) ?? [];
      entries.set(token, items.filter((item) => (item as {id: string}).id !== path.split("/").at(-1)));
      return route.fulfill({ status: 204, headers });
    }
    if (path === "/tracker") {
      if (request.method() === "POST") {
        const entry = {
          id: "20000000-0000-4000-8000-000000000001",
          animeId: request.postDataJSON().animeId,
          title: "Synthetic Catalog Title",
          watchStatus: "PLANNING",
          episodesWatched: 0,
        };
        entries.set(token, [entry]);
        return respond(entry, 201);
      }
      const items = entries.get(token) ?? [];
      return respond({ items, count: items.length });
    }
    return respond({ error: "not_found" }, 404);
  });
  return requests;
}

test("account switching and reload never retain another account's library or bearer", async ({
  page,
}) => {
  const requests = await mockAPI(page);
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Create an account", exact: true })
    .click();
  await page
    .getByLabel("Email", { exact: true })
    .fill("browser-one@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-browser-password");
  await page
    .getByRole("button", { name: "Create account", exact: true })
    .click();
  await expect(
    page.getByText("browser-one@example.test", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Active sessions" }),
  ).toHaveCount(1);
  await expect(
    page.getByRole("region", { name: "Password settings" }),
  ).toHaveCount(1);
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(1);
  await expect(page.getByRole("article")).toContainText(
    "Synthetic Catalog Title",
  );
  await page.screenshot({
    path: "test-results/authenticated-dashboard.png",
    fullPage: true,
  });
  const firstToken = requests.find(
    (x) => x.path === "/tracker" && x.token,
  )?.token;
  expect(firstToken).toHaveLength(43);
  await page.getByRole("article").getByRole("button", { name: "Remove Synthetic Catalog Title", exact: true }).click();
  await expect(page.getByRole("button", { name: "Confirm removal", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(0);
  await page
    .getByLabel("Email", { exact: true })
    .fill("browser-two@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByText("browser-two@example.test", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(page.getByLabel("Password", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Confirm removal", exact: true })).toHaveCount(0);
  expect(
    await page.evaluate(() => ({
      local: Object.keys(localStorage),
      session: Object.keys(sessionStorage),
    })),
  ).toEqual({ local: [], session: [] });
  const beforeReload = requests.length;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  await expect(page.getByRole("article")).toHaveCount(0);
  expect(requests.slice(beforeReload).every((x) => !x.token)).toBe(true);
});

test("local development still works without an account", async ({ page }) => {
  await mockAPI(page, false);
  await page.goto("/");
  await expect(
    page.getByText("Local development", { exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Email", { exact: true })).toHaveCount(0);
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(1);
});

test("a rejected session clears private data and returns to sign in", async ({
  page,
}) => {
  await mockAPI(page);
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("expired@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-expired-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByText("expired@example.test", { exact: true }),
  ).toBeVisible();
  await page.route("**/api/v1/tracker", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      headers: { "Access-Control-Allow-Origin": "http://127.0.0.1:3000" },
      body: '{"error":"unauthorized"}',
    }),
  );
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(page.getByLabel("Email", { exact: true })).toHaveValue("");
});

test("session controls revoke one login, then every other login", async ({
  page,
}) => {
  await mockAPI(page);
  const current = "60000000-0000-4000-8000-000000000001";
  const first = "60000000-0000-4000-8000-000000000002";
  const second = "60000000-0000-4000-8000-000000000003";
  let ids = [current, first, second];
  const deleted: string[] = [];
  let bulkCalls = 0;
  await page.route("**/api/v1/auth/sessions**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Methods": "GET, POST, DELETE, OPTIONS",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
    };
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    expect(request.headers().authorization).toMatch(/^Bearer /);
    if (request.method() === "DELETE") {
      const id = path.split("/").pop()!;
      deleted.push(id);
      ids = ids.filter((x) => x !== id);
      return route.fulfill({ status: 204, headers });
    }
    if (path.endsWith("revoke-others")) {
      bulkCalls++;
      const revoked = ids.length - 1;
      ids = [current];
      return route.fulfill({ json: { revoked }, headers });
    }
    const createdAt = new Date().toISOString(),
      expiresAt = new Date(Date.now() + 86400000).toISOString();
    return route.fulfill({
      json: {
        items: ids.map((id) => ({
          id,
          createdAt,
          expiresAt,
          lastUsedAt: createdAt,
          current: id === current,
        })),
        count: ids.length,
        hasMore: false,
      },
      headers,
    });
  });
  await page.goto("/");
  await page
    .getByLabel("Email", { exact: true })
    .fill("browser-sessions@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-session-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page
    .getByRole("button", { name: "Manage sessions", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Active sessions" });
  await expect(
    panel.getByText("Current session", { exact: true }),
  ).toBeVisible();
  await expect(
    panel.getByRole("button", { name: /^Revoke session/ }),
  ).toHaveCount(2);
  await panel
    .getByRole("button", { name: `Revoke session ${first}`, exact: true })
    .click();
  await expect(panel.getByRole("status")).toHaveText("Session signed out.");
  expect(deleted).toEqual([first]);
  await expect(
    panel.getByRole("button", { name: /^Revoke session/ }),
  ).toHaveCount(1);
  await panel
    .getByRole("button", { name: "Sign out other sessions", exact: true })
    .click();
  await expect(panel.getByRole("status")).toHaveText(
    "Signed out 1 other session.",
  );
  expect(bulkCalls).toBe(1);
  await expect(
    panel.getByRole("button", { name: /^Revoke session/ }),
  ).toHaveCount(0);
  await expect(
    panel.getByRole("button", { name: "Sign out other sessions", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByText("browser-sessions@example.test", { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/session-management.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Active sessions" }),
  ).toHaveCount(0);
});

test("password change handles reauthentication failure then clears all private UI", async ({
  page,
}) => {
  await mockAPI(page);
  let attempts = 0;
  await page.route("**/api/v1/auth/password", async (route) => {
    const request = route.request();
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "POST, OPTIONS",
    };
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    expect(request.headers().authorization).toMatch(/^Bearer /);
    attempts++;
    const body = request.postDataJSON();
    if (body.currentPassword !== "synthetic-current-password")
      return route.fulfill({
        status: 401,
        headers,
        json: { error: "invalid_current_password" },
      });
    expect(body.newPassword).toBe("synthetic-replacement-password");
    return route.fulfill({ status: 204, headers });
  });
  await page.goto("/");
  await page
    .getByLabel("Email", { exact: true })
    .fill("browser-password@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-current-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page
    .getByRole("button", { name: "Change password", exact: true })
    .click();
  const panel = page.getByRole("region", { name: "Password settings" });
  await panel
    .getByLabel("Current password", { exact: true })
    .fill("incorrect-password");
  await panel
    .getByLabel("New password", { exact: true })
    .fill("synthetic-replacement-password");
  await panel
    .getByRole("button", { name: "Update password", exact: true })
    .click();
  await expect(panel.getByRole("alert")).toHaveText(
    "The current password is incorrect.",
  );
  await expect(
    page.getByText("browser-password@example.test", { exact: true }),
  ).toBeVisible();
  await expect(
    panel.getByLabel("Current password", { exact: true }),
  ).toHaveValue("");
  await expect(panel.getByLabel("New password", { exact: true })).toHaveValue(
    "",
  );
  await panel
    .getByLabel("Current password", { exact: true })
    .fill("synthetic-current-password");
  await panel
    .getByLabel("New password", { exact: true })
    .fill("synthetic-replacement-password");
  await panel
    .getByRole("button", { name: "Update password", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  await expect(panel).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Active sessions" }),
  ).toHaveCount(0);
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(page.getByRole("status")).toHaveText(
    "Password changed. Sign in with your new password.",
  );
  expect(attempts).toBe(2);
});

test("identity fragments are scrubbed and wait for an explicit confirmation", async ({
  page,
}) => {
  await mockAPI(page, true, true);
  const token = "A".repeat(43);
  let confirmations = 0;
  await page.route(
    "**/api/v1/auth/email-verification/confirm",
    async (route) => {
      confirmations++;
      expect(route.request().postDataJSON()).toEqual({ token });
      return route.fulfill({
        status: 204,
        headers: { "Access-Control-Allow-Origin": "http://127.0.0.1:3000" },
      });
    },
  );
  await page.goto(`/#action=verify-email&token=${token}`);
  await expect(
    page.getByRole("heading", { name: "Confirm your email" }),
  ).toBeVisible();
  expect(await page.evaluate(() => location.hash)).toBe("");
  expect(confirmations).toBe(0);
  await page.getByRole("button", { name: "Verify email", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Account recovery" }).getByRole("status"),
  ).toContainText("Email verified");
  expect(confirmations).toBe(1);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Confirm your email" }),
  ).toHaveCount(0);
  expect(
    await page.evaluate(() =>
      JSON.stringify({ local: localStorage, session: sessionStorage }),
    ),
  ).not.toContain(token);
});

test("unverified accounts stay signed in with library locked when mail is unavailable", async ({
  page,
}) => {
  const requests = await mockAPI(page, true, false, false);
  await page.goto("/");
  await page
    .getByLabel("Email", { exact: true })
    .fill("unverified-browser@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Email verification required" }),
  ).toContainText("Email delivery is currently unavailable");
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic");
  await expect(
    page.getByRole("button", { name: "Add", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Account data" }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Password settings" }),
  ).toBeVisible();
  await expect(
    page.getByText("unverified-browser@example.test", { exact: true }),
  ).toBeVisible();
  expect(
    requests.filter((request) => request.path === "/tracker"),
  ).toHaveLength(0);
});


test("library removal confirms, cancels, preserves failures and permits re-add", async ({ page }) => {
  await mockAPI(page, false);
  await page.goto("/");
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const article = page.getByRole("article");
  await expect(article).toHaveCount(1);
  let deletes = 0;
  let fail = true;
  await page.route("**/api/v1/tracker/*", async route => {
    if (route.request().method() !== "DELETE") return route.fallback();
    deletes++;
    if (fail) return route.fulfill({status: 503, contentType: "application/json", body: JSON.stringify({error:"private-server-error"}), headers:{"Access-Control-Allow-Origin":"http://127.0.0.1:3000"}});
    return route.fallback();
  });
  await article.getByRole("button", { name: "Remove Synthetic Catalog Title", exact: true }).click();
  await expect(article.getByText(/Adding it again starts fresh/)).toBeVisible();
  await article.getByRole("button", { name: "Cancel", exact: true }).click();
  expect(deletes).toBe(0);
  await article.getByRole("button", { name: "Remove Synthetic Catalog Title", exact: true }).click();
  await article.getByRole("button", { name: "Confirm removal", exact: true }).click();
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("Request failed. Please try again.");
  await expect(article).toHaveCount(1);
  await expect(article.getByRole("button", { name: "Confirm removal", exact: true })).toBeEnabled();
  fail = false;
  await article.getByRole("button", { name: "Confirm removal", exact: true }).click();
  await expect(article).toHaveCount(0);
  expect(deletes).toBe(2);
  await expect(page.getByRole("main").getByRole("alert")).toHaveCount(0);
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(article).toHaveCount(1);
  await expect(article.getByRole("group", { name: "Remove Synthetic Catalog Title" })).toHaveCount(0);
});
