export type Challenge = {
  action: "reset-password" | "verify-email";
  token: string;
};

// Accept only the two fields emitted by the trusted mail renderer. Query
// credentials, repeated fields, ambiguous actions, and malformed proof fail.
export function parseChallengeFragment(hash: string): Challenge | null {
  if (!hash.startsWith("#")) return null;
  const params = new URLSearchParams(hash.slice(1));
  if (
    params.size !== 2 ||
    params.getAll("action").length !== 1 ||
    params.getAll("token").length !== 1
  )
    return null;
  const action = params.get("action"),
    token = params.get("token");
  if (
    (action !== "reset-password" && action !== "verify-email") ||
    !token ||
    !/^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/.test(token)
  )
    return null;
  return { action, token };
}
