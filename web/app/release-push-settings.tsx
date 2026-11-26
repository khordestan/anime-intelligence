"use client";

import { useState } from "react";
import { ApiClient } from "../lib/api";
import { browserPushSupported, subscribeToBrowserPush, unsubscribeBrowserPush, withBrowserPush } from "../lib/release-push";

const path = "/releases/browser-push";
type Registration = { id: string; createdAt: string };
type Settings = { publicKey?: string; registration: Registration | null };

export default function ReleasePushSettings({ client }: { client: ApiClient }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  async function load() {
    if (busy) return;
    const current = client.sessionGuard();
    setBusy(true);
    setMessage("");
    try {
      const response = await client.request<Settings>(path);
      if (current()) setSettings(response);
    } catch {
      if (current()) setMessage("Could not load browser alert settings.");
    } finally {
      if (current()) setBusy(false);
    }
  }

  async function enable() {
    if (!settings?.publicKey || busy) return;
    if (!browserPushSupported()) {
      setMessage("This browser cannot receive background alerts here.");
      return;
    }
    const current = client.sessionGuard();
    const key = settings.publicKey;
    setBusy(true);
    setMessage("");
    try {
      const permission = await Notification.requestPermission();
      if (!current()) return;
      if (permission !== "granted") {
        setMessage("Allow notifications in your browser to enable this device.");
        return;
      }
      await withBrowserPush(async () => {
        if (!current()) return;
        const subscription = await subscribeToBrowserPush(key);
        try {
          if (!current()) {
            await subscription.unsubscribe();
            return;
          }
          const { endpoint, keys } = subscription.toJSON();
          const registration = await client.request<Registration>(path, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ endpoint, keys }),
          });
          if (current()) setSettings({ publicKey: key, registration });
        } catch (error) {
          await subscription.unsubscribe();
          throw error;
        }
      });
    } catch {
      if (current()) setMessage("Could not enable browser alerts. Try again.");
    } finally {
      if (current()) setBusy(false);
    }
  }

  async function disable() {
    if (!settings?.registration || busy) return;
    const current = client.sessionGuard();
    setBusy(true);
    setMessage("");
    try {
      await client.request<void>(path + "/" + settings.registration.id, { method: "DELETE" });
      if (!current()) return;
      setSettings({ ...settings, registration: null });
      await unsubscribeBrowserPush();
    } catch {
      if (current()) setMessage("Could not finish disabling browser alerts. Refresh the settings to check.");
    } finally {
      if (current()) setBusy(false);
    }
  }

  return (
    <details className="mt-3 rounded-lg border border-white/10 p-3 text-sm">
      <summary className="cursor-pointer font-medium">Background browser alerts</summary>
      <p className="mt-2 text-slate-300">Receive generic release alerts in one browser, even when this dashboard is closed.</p>
      {settings && (
        <div className="mt-2 space-y-2">
          <p>{settings.registration ? "Your account has a browser registered." : "No browser is registered."}</p>
          {settings.publicKey ? (
            <button type="button" onClick={() => void enable()} disabled={busy} className="mr-3 underline disabled:opacity-50">
              {settings.registration ? "Use this browser instead" : "Enable on this browser"}
            </button>
          ) : <p>Browser alert delivery is currently unavailable.</p>}
          {settings.registration && (
            <button type="button" onClick={() => void disable()} disabled={busy} className="mr-3 underline disabled:opacity-50">
              Disable browser alerts
            </button>
          )}
        </div>
      )}
      <button type="button" onClick={() => void load()} disabled={busy} className="mt-2 underline disabled:opacity-50">
        {settings ? "Refresh browser alert settings" : "Check browser alert settings"}
      </button>
      {message && <p role="status" className="mt-2 text-rose-200">{message}</p>}
    </details>
  );
}
