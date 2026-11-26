const scope = "/release-push/";

let pending: Promise<unknown> = Promise.resolve();

export function withBrowserPush<T>(task: () => Promise<T>): Promise<T> {
  const result = pending.then(task);
  pending = result.catch(() => {});
  return result;
}

export function browserPushSupported() {
  return typeof window !== "undefined" && window.isSecureContext &&
    "Notification" in window && "serviceWorker" in navigator && "PushManager" in window;
}

async function activeRegistration() {
  const registration = await navigator.serviceWorker.register("/release-push.js", { scope });
  if (registration.active?.state === "activated") return registration;
  const worker = registration.installing ?? registration.waiting ?? registration.active;
  if (!worker) throw new Error("Browser notification setup failed.");
  await new Promise<void>((resolve, reject) => {
    const finish = (error?: Error) => {
      window.clearTimeout(timer);
      worker.removeEventListener("statechange", check);
      if (error) reject(error);
      else resolve();
    };
    const check = () => {
      if (worker.state === "activated") finish();
      else if (worker.state === "redundant") finish(new Error("Browser notification setup failed."));
    };
    const timer = window.setTimeout(() => finish(new Error("Browser notification setup timed out.")), 5000);
    worker.addEventListener("statechange", check);
    check();
  });
  return registration;
}

function decodePublicKey(key: string): Uint8Array<ArrayBuffer> {
  const padding = "=".repeat((4 - key.length % 4) % 4);
  const bytes = atob(key.replaceAll("-", "+").replaceAll("_", "/") + padding);
  return Uint8Array.from(bytes, char => char.charCodeAt(0));
}

export async function subscribeToBrowserPush(key: string) {
  const registration = await activeRegistration();
  const old = await registration.pushManager.getSubscription();
  await old?.unsubscribe();
  return registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: decodePublicKey(key),
  });
}

export function unsubscribeBrowserPush() {
  if (!browserPushSupported()) return Promise.resolve();
  return withBrowserPush(async () => {
    const registration = await navigator.serviceWorker.getRegistration(scope);
    const subscription = await registration?.pushManager.getSubscription();
    await subscription?.unsubscribe();
  });
}
