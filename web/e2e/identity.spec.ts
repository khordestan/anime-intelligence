import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import type { Session } from "../lib/api";

const API =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080/api/v1";

test("real TLS email proof verifies and recovers an account without leaking URL credentials", async ({
  page,
  request,
}) => {
  const email = `identity-browser-${randomUUID()}@example.test`,
    password = "synthetic-original-password",
    replacement = "synthetic-recovery-password";
  const response = await request.post(API + "/auth/register", {
    data: { email, password },
  });
  expect(response.status()).toBe(201);
  const original: Session = await response.json();
  expect(original.user.emailVerified).toBe(false);
  const auth = { Authorization: `Bearer ${original.token}` };
  async function mailLink(action: string) {
    let messages: { body: string }[] = [];
    await expect
      .poll(
        async () => {
          const mailbox = await request.get("http://127.0.0.1:8026/messages", {
            params: { recipient: email },
            headers: {
              "X-Test-Mailbox-Key": process.env.MAIL_TEST_ACCESS_KEY!,
            },
          });
          expect(mailbox.status()).toBe(200);
          messages = await mailbox.json();
          return messages.some((message) =>
            message.body.includes(`action=${action}`),
          );
        },
        { timeout: 10_000 },
      )
      .toBe(true);
    const message = messages.find((message) =>
      message.body.includes(`action=${action}`),
    )!;
    const link = message.body.match(/http:\/\/127\.0\.0\.1:3000\/#\S+/)?.[0];
    expect(!!link).toBe(true);
    return new URL(link!);
  }
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await page.getByPlaceholder("Search starter catalog…").fill("Solo Leveling");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("article")).toContainText("Solo Leveling");
  await page
    .getByRole("button", { name: "Send verification email", exact: true })
    .click();
  await expect(
    page
      .getByRole("region", { name: "Email verification" })
      .getByRole("status"),
  ).toContainText("Verification requested");
  const verifyLink = await mailLink("verify-email");
  let confirmations = 0;
  page.on("request", (req) => {
    if (
      new URL(req.url()).pathname.endsWith("/auth/email-verification/confirm")
    )
      confirmations++;
  });
  await page.goto(verifyLink.toString());
  await expect(
    page.getByRole("heading", { name: "Confirm your email" }),
  ).toBeVisible();
  expect(await page.evaluate(() => location.hash)).toBe("");
  expect(confirmations).toBe(0);
  const before = await request.get(API + "/auth/me", { headers: auth });
  expect((await before.json()).emailVerified).toBe(false);
  await page.getByRole("button", { name: "Verify email", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Account recovery" }).getByRole("status"),
  ).toHaveText("Email verified. You can sign in normally.");
  expect(confirmations).toBe(1);
  const verified = await request.get(API + "/auth/me", { headers: auth });
  expect((await verified.json()).emailVerified).toBe(true);
  const verifyToken = new URLSearchParams(verifyLink.hash.slice(1)).get(
    "token",
  );
  expect(
    (
      await request.post(API + "/auth/email-verification/confirm", {
        data: { token: verifyToken },
      })
    ).status(),
  ).toBe(400);
  await expect(
    page.getByRole("region", { name: "Email verification" }),
  ).toContainText("Email verified");
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  const unknown = await request.post(API + "/auth/password-reset/request", {
    data: { email: `unknown-${randomUUID()}@example.test` },
  });
  expect(unknown.status()).toBe(202);
  await page
    .getByRole("button", { name: "Forgot password?", exact: true })
    .click();
  await page.getByLabel("Recovery email", { exact: true }).fill(email);
  const accepted = page.waitForResponse((res) =>
    new URL(res.url()).pathname.endsWith("/auth/password-reset/request"),
  );
  await page
    .getByRole("button", { name: "Request recovery email", exact: true })
    .click();
  expect(await (await accepted).json()).toEqual(await unknown.json());
  await expect(page.getByLabel("Recovery email", { exact: true })).toHaveValue(
    "",
  );
  const resetLink = await mailLink("reset-password"),
    resetToken = new URLSearchParams(resetLink.hash.slice(1)).get("token");
  await page.goto(resetLink.toString());
  await expect(
    page.getByRole("heading", { name: "Reset your password" }),
  ).toBeVisible();
  expect(await page.evaluate(() => location.hash)).toBe("");
  expect(
    (await request.get(API + "/auth/me", { headers: auth })).status(),
  ).toBe(200);
  expect(
    await page.evaluate(() => ({
      local: Object.keys(localStorage),
      session: Object.keys(sessionStorage),
    })),
  ).toEqual({ local: [], session: [] });
  await page
    .getByLabel("Recovery new password", { exact: true })
    .fill(replacement);
  await page
    .getByRole("button", { name: "Reset password", exact: true })
    .click();
  await expect(page.getByRole("status")).toHaveText(
    "Password reset. Sign in with your new password.",
  );
  expect(
    (await request.get(API + "/auth/me", { headers: auth })).status(),
  ).toBe(401);
  expect(
    (
      await request.post(API + "/auth/password-reset/confirm", {
        data: { token: resetToken, newPassword: "another-synthetic-password" },
      })
    ).status(),
  ).toBe(400);
  expect(
    (
      await request.post(API + "/auth/login", { data: { email, password } })
    ).status(),
  ).toBe(401);
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill(replacement);
  const recoveredLogin = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith("/auth/login"),
  );
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Email verification" }),
  ).toContainText("Email verified");
  await expect(page.getByRole("article")).toContainText("Solo Leveling");
  const recovered: Session = await (await recoveredLogin).json();
  const account = page.getByRole("region", { name: "Account data" });
  const downloadEvent = page.waitForEvent("download");
  await account
    .getByRole("button", { name: "Download account export", exact: true })
    .click();
  const download = await downloadEvent;
  expect(download.suggestedFilename()).toBe("anime-intelligence-account.json");
  const exportPath = await download.path();
  const text = await readFile(exportPath!, "utf8"),
    snapshot = JSON.parse(text);
  expect(snapshot.user.id).toBe(original.user.id);
  expect(snapshot.user.email).toBe(email);
  expect(snapshot.data.tracker).toHaveLength(1);
  for (const credential of [
    original.token,
    recovered.token,
    password,
    replacement,
    "password_hash",
    "token_hash",
  ])
    expect(text).not.toContain(credential);
  await page.screenshot({
    path: "test-results/identity-recovered.png",
    fullPage: true,
  });
  await account
    .getByRole("button", { name: "Delete account", exact: true })
    .click();
  const remove = account.getByRole("button", {
    name: "Permanently delete my account",
    exact: true,
  });
  await expect(remove).toBeDisabled();
  await account
    .getByLabel("Deletion password", { exact: true })
    .fill("incorrect-synthetic-password");
  await account.getByRole("checkbox").check();
  await remove.click();
  await expect(account.getByRole("alert")).toHaveText(
    "The current password is incorrect.",
  );
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await expect(
    account.getByLabel("Deletion password", { exact: true }),
  ).toHaveValue("");
  await expect(account.getByRole("checkbox")).not.toBeChecked();
  await account
    .getByLabel("Deletion password", { exact: true })
    .fill(replacement);
  await account.getByRole("checkbox").check();
  await remove.click();
  await expect(
    page.getByRole("heading", { name: "Sign in to your library" }),
  ).toBeVisible();
  await expect(page.getByRole("article")).toHaveCount(0);
  await expect(account).toHaveCount(0);
  await expect(page.getByRole("status")).toHaveText(
    "Account deleted. All sessions have been signed out.",
  );
  expect(
    (
      await request.get(API + "/auth/me", {
        headers: { Authorization: `Bearer ${recovered.token}` },
      })
    ).status(),
  ).toBe(401);
});
