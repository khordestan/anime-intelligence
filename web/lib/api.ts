export type User = { id: string; email: string; displayName: string };
export type Session = { token: string; expiresAt: string; user: User };
export class StaleSessionError extends Error {}
export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

// One client per mounted dashboard. Credentials live only in memory. Switching
// accounts aborts in-flight work and rejects late results, even if a transport
// ignores AbortSignal. Private responses must never cross an account boundary.
export class ApiClient {
  private session: Session | null = null;
  private revision = 0;
  private controller = new AbortController();
  private readonly base: string;
  private readonly transport: typeof fetch;
  constructor(base: string, transport: typeof fetch = fetch) {
    this.base = base;
    this.transport = transport;
  }
  setSession(session: Session | null) {
    this.controller.abort();
    this.controller = new AbortController();
    this.revision++;
    this.session = session;
  }
  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const revision = this.revision;
    const headers = new Headers(init.headers);
    headers.delete("Authorization");
    if (this.session)
      headers.set("Authorization", "Bearer " + this.session.token);
    const response = await this.transport.call(globalThis, this.base + path, {
      ...init,
      headers,
      cache: "no-store",
      credentials: "omit",
      signal: this.controller.signal,
    });
    if (revision !== this.revision) throw new StaleSessionError();
    if (!response.ok) {
      let message = "Request failed. Please try again.";
      if (response.status === 401)
        message = "Your session ended. Sign in again.";
      if (response.status === 409)
        message = "This account is unavailable. Try signing in.";
      if (response.status === 429)
        message = "Too many attempts. Please wait before trying again.";
      if (response.status === 400)
        message =
          "Check your input. Passwords must contain 12–1024 bytes when registering.";
      throw new ApiError(response.status, message);
    }
    const body = response.status === 204 ? undefined : await response.json();
    if (revision !== this.revision) throw new StaleSessionError();
    return body as T;
  }
}
