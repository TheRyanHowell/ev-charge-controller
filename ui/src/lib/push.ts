// Push notification utilities for PWA.
// All functions are client-side only - use behind 'isPushEnabled' check.
//
// Reliability principles:
// - Never unsubscribe a working subscription unless a replacement strategy is
//   in hand; an unsubscribe followed by a failed resubscribe silently kills
//   notifications until the user notices and re-enables them.
// - A subscription with expirationTime === null never expires (Chrome/FCM);
//   only a non-null expirationTime close to now needs renewal.
// - Re-upsert the current subscription to the server on every app open, so a
//   server that lost the record (pruned endpoint, restored database) heals
//   without user action.

interface PushSubscriptionData {
  endpoint: string;
  p256dhKey: string;
  authKey: string;
}

export function getVapidPublicKey(): string {
  return typeof process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY === "string"
    ? process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY
    : "";
}

export function isPushEnabled(key?: string): boolean {
  return "serviceWorker" in navigator && !!(key ?? getVapidPublicKey());
}

export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!("serviceWorker" in navigator)) {
    return null;
  }
  // updateViaCache: "none" makes the browser revalidate sw.js on every
  // registration check, so service worker fixes reach installed PWAs promptly.
  return navigator.serviceWorker.register("/sw.js", { updateViaCache: "none" });
}

async function getRegistration(): Promise<ServiceWorkerRegistration | null> {
  if (!("serviceWorker" in navigator)) {
    return null;
  }
  const reg = await navigator.serviceWorker.ready;
  return reg;
}

export async function getPushSubscription(): Promise<PushSubscriptionData | null> {
  const reg = await getRegistration();
  if (!reg) {
    return null;
  }
  const sub = await reg.pushManager.getSubscription();
  if (!sub) {
    return null;
  }
  return subscriptionToData(sub);
}

async function postSubscription(data: PushSubscriptionData): Promise<boolean> {
  const res = await fetch("/api/push-subscriptions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  return res.ok;
}

async function deleteSubscription(endpoint: string): Promise<boolean> {
  const res = await fetch("/api/push-subscriptions", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ endpoint }),
  });
  return res.ok;
}

/**
 * subscribeAndRegister obtains a push subscription (reusing the existing one
 * where the browser allows it) and registers it with the server.
 *
 * The old subscription is only unsubscribed when the browser refuses to
 * subscribe over it (key change), or when it genuinely needs rotation - never
 * as a precaution. If the server registration fails the browser subscription
 * is kept so the next app-open sync can retry.
 */
async function subscribeAndRegister(
  reg: ServiceWorkerRegistration,
  existingSub: PushSubscription | null,
): Promise<PushSubscriptionData | null> {
  const oldEndpoint = existingSub?.endpoint ?? null;
  const applicationServerKey = urlBase64ToBufferKey(getVapidPublicKey());
  const subscribeOptions = { userVisibleOnly: true, applicationServerKey };

  let sub: PushSubscription;
  try {
    sub = await reg.pushManager.subscribe(subscribeOptions);
  } catch {
    // InvalidStateError: an existing subscription uses a different server key.
    // Only now is dropping the old subscription unavoidable.
    if (!existingSub) {
      return null;
    }
    await existingSub.unsubscribe();
    sub = await reg.pushManager.subscribe(subscribeOptions);
  }

  // subscribe() returns the existing subscription unchanged when the key
  // matches; a genuinely expiring subscription must be rotated explicitly.
  if (sub.endpoint === oldEndpoint && isSubscriptionExpiring(sub)) {
    await sub.unsubscribe();
    sub = await reg.pushManager.subscribe(subscribeOptions);
  }

  const data = subscriptionToData(sub);
  if (!(await postSubscription(data))) {
    return null;
  }

  if (oldEndpoint && oldEndpoint !== sub.endpoint) {
    // Best effort - a dead endpoint is also pruned server-side on 410.
    await deleteSubscription(oldEndpoint).catch(() => false);
  }

  return data;
}

export async function subscribeToPush(): Promise<PushSubscriptionData | null> {
  const reg = await getRegistration();
  if (!reg) {
    return null;
  }

  // Explicitly request notification permission (required on Android Chrome).
  // Safe here: this function is only called from a user gesture.
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    return null;
  }

  const existingSub = await reg.pushManager.getSubscription();
  try {
    return await subscribeAndRegister(reg, existingSub);
  } catch {
    return null;
  }
}

export async function unsubscribeFromPush(): Promise<boolean> {
  const reg = await getRegistration();
  if (!reg) {
    return false;
  }

  const sub = await reg.pushManager.getSubscription();
  if (!sub) {
    return false;
  }

  // Server delete is best effort: the user's intent is to stop notifications,
  // which the browser-side unsubscribe guarantees. A leftover server record
  // gets 410 on the next send and is pruned automatically.
  try {
    await deleteSubscription(sub.endpoint);
  } catch {
    // Ignore - see above.
  }

  return sub.unsubscribe();
}

function subscriptionToData(sub: PushSubscription): PushSubscriptionData {
  const details = sub.toJSON();
  return {
    endpoint: sub.endpoint,
    p256dhKey: details.keys?.p256dh ?? "",
    authKey: details.keys?.auth ?? "",
  };
}

// pushSubscriptionExpiryThresholdMs is the window before expirationTime
// at which the subscription is considered expiring and should be renewed.
const pushSubscriptionExpiryThresholdMs = 24 * 60 * 60 * 1000;

function isSubscriptionExpiring(sub: PushSubscription): boolean {
  // null means the subscription never expires (the common case on Chrome/FCM).
  if (sub.expirationTime === null || sub.expirationTime === undefined) {
    return false;
  }
  const expiresInMs = sub.expirationTime - Date.now();
  return expiresInMs < pushSubscriptionExpiryThresholdMs;
}

/**
 * ensurePushSubscription self-heals the push pipeline on app open:
 * - permission granted + healthy subscription: re-upsert it to the server in
 *   case the server lost the record.
 * - permission granted + no subscription (or an expiring one): silently
 *   (re)subscribe - no prompt is shown because permission is already granted.
 * - permission not granted: do nothing. Never prompts; prompting belongs to
 *   the explicit user-gesture path (subscribeToPush).
 *
 * Safe to call from useEffect on every page load; failures are silent and
 * retried on the next app open.
 */
export async function ensurePushSubscription(): Promise<void> {
  if (!isPushEnabled()) return;
  if (typeof Notification === "undefined") return;
  if (Notification.permission !== "granted") return;

  try {
    const reg = await getRegistration();
    if (!reg) return;

    const sub = await reg.pushManager.getSubscription();
    if (sub && !isSubscriptionExpiring(sub)) {
      await postSubscription(subscriptionToData(sub));
      return;
    }

    await subscribeAndRegister(reg, sub);
  } catch {
    // Silently fail - retried on next page load.
  }
}

function urlBase64ToBufferKey(base64String: string): ArrayBuffer {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = base64String.replace(/-/g, "+").replace(/_/g, "/") + padding;
  const binaryString = atob(base64);
  const bytes = new Uint8Array(binaryString.length);
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes.buffer;
}
