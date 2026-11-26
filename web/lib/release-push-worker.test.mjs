import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";

test("the push worker shows only generic release notices and opens the local dashboard", async () => {
  const handlers = new Map();
  const notices = [];
  const opened = [];
  const self = {
    addEventListener: (event, handler) => handlers.set(event, handler),
    registration: {
      showNotification: async (title, options) => { notices.push({ title, ...options }); },
    },
    clients: {
      matchAll: async () => [],
      openWindow: async url => { opened.push(url); },
    },
  };
  const script = await readFile(new URL("../public/release-push.js", import.meta.url), "utf8");
  vm.runInNewContext(script, { self });
  const dispatch = async (name, event) => {
    const promises = [];
    handlers.get(name)({ ...event, waitUntil: promise => promises.push(promise) });
    await Promise.all(promises);
  };
  await dispatch("push", { data: { text: () => "synthetic message with private details" } });
  assert.deepEqual(notices, []);
  await dispatch("push", { data: { text: () => "release" } });
  assert.deepEqual(notices, [{
    title: "Anime Intelligence",
    body: "Open your dashboard to review a release update.",
    tag: "anime-release",
  }]);
  let closed = 0;
  await dispatch("notificationclick", { notification: { tag: "anime-release", close: () => { closed++; } } });
  assert.equal(closed, 1);
  assert.deepEqual(opened, ["/"]);
});
