import { expect, type APIRequestContext } from "@playwright/test";

// This endpoint belongs only to cmd/testmail, which binds numeric loopback and
// requires a disposable test key. Production application routes never expose it.
export async function verificationToken(
  request: APIRequestContext,
  recipient: string,
): Promise<string> {
  let token = "";
  await expect
    .poll(
      async () => {
        const response = await request.get("http://127.0.0.1:8026/messages", {
          params: { recipient },
          headers: { "X-Test-Mailbox-Key": process.env.MAIL_TEST_ACCESS_KEY! },
        });
        expect(response.status()).toBe(200);
        const messages: { body: string }[] = await response.json();
        const body = messages.find((message) =>
          message.body.includes("action=verify-email"),
        )?.body;
        const link = body?.match(/http:\/\/127\.0\.0\.1:3000\/#\S+/)?.[0];
        token = link
          ? (new URLSearchParams(new URL(link).hash.slice(1)).get("token") ??
            "")
          : "";
        return token.length === 43;
      },
      { timeout: 10_000 },
    )
    .toBe(true);
  return token;
}
