import test from "node:test";
import assert from "node:assert/strict";
import { parseChallengeFragment } from "./challenge.ts";
const token = "A".repeat(43);
test("mail proof requires a single known purpose and a canonical fragment token", () => {
  assert.deepEqual(
    parseChallengeFragment(`#action=reset-password&token=${token}`),
    { action: "reset-password", token },
  );
  assert.deepEqual(
    parseChallengeFragment(`#token=${token}&action=verify-email`),
    { action: "verify-email", token },
  );
  for (const value of [
    "",
    `?action=reset-password&token=${token}`,
    `#action=unknown&token=${token}`,
    `#action=verify-email&token=${token}&token=${token}`,
    `#action=verify-email&action=reset-password&token=${token}`,
    `#action=verify-email&token=${token}&userId=spoofed`,
    `#action=verify-email&token=${"B".repeat(43)}`,
    "#action=verify-email&token=%00",
  ]) {
    assert.equal(parseChallengeFragment(value), null);
  }
});
