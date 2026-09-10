// Web Push subscription helper: registers the service worker, requests
// permission, subscribes with the backend's VAPID public key, and posts the
// subscription to the API.

import { api } from "./api";

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const output = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; ++i) output[i] = raw.charCodeAt(i);
  return output;
}

export function pushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window
  );
}

export async function registerServiceWorker(): Promise<ServiceWorkerRegistration> {
  return navigator.serviceWorker.register("/sw.js");
}

// enablePush wires up the full subscription flow and returns true on success.
export async function enablePush(): Promise<boolean> {
  if (!pushSupported()) throw new Error("Push notifications are not supported in this browser");

  const registration = await registerServiceWorker();
  await navigator.serviceWorker.ready;

  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error("Notification permission was denied");
  }

  const { public_key } = await api<{ public_key: string }>(
    "/api/v1/push/vapid-public-key",
    { auth: false }
  );
  if (!public_key) throw new Error("Server has no VAPID key configured");

  const existing = await registration.pushManager.getSubscription();
  const subscription =
    existing ||
    (await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(public_key) as BufferSource,
    }));

  await api("/api/v1/push/subscribe", {
    method: "POST",
    body: JSON.stringify(subscription.toJSON()),
  });
  return true;
}

export async function sendTestPush() {
  await api("/api/v1/push/test", { method: "POST", body: JSON.stringify({}) });
}

// disablePush unsubscribes this browser and tells the backend to drop the
// subscription, turning off the daily reminder.
export async function disablePush(): Promise<void> {
  let endpoint = "";
  if (pushSupported()) {
    try {
      const registration = await navigator.serviceWorker.ready;
      const sub = await registration.pushManager.getSubscription();
      if (sub) {
        endpoint = sub.endpoint;
        await sub.unsubscribe();
      }
    } catch {
      /* ignore — still tell the backend to drop our subscriptions */
    }
  }
  await api("/api/v1/push/unsubscribe", {
    method: "POST",
    body: JSON.stringify({ endpoint }),
  });
}
