import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
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
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Email verification" }),
  ).toContainText("Email verified");
  await expect(page.getByRole("article")).toContainText("Solo Leveling");
  await page.screenshot({
    path: "test-results/identity-recovered.png",
    fullPage: true,
  });
});
