/* global self */
self.addEventListener("install", event => {
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("push", event => {
  if (event.data?.text() !== "release") return;
  event.waitUntil(self.registration.showNotification("Anime Intelligence", {
    body: "Open your dashboard to review a release update.",
    tag: "anime-release",
  }));
});

self.addEventListener("notificationclick", event => {
  if (event.notification.tag !== "anime-release") return;
  event.notification.close();
  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    const dashboard = windows.find(client => new URL(client.url).pathname === "/");
    if (dashboard) await dashboard.focus();
    else await self.clients.openWindow("/");
  })());
});
