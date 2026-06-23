// Push notification utilities for PWA.
// All functions are client-side only - use behind 'isPushEnabled' check.

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
  return navigator.serviceWorker.register("/sw.js");
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

export async function subscribeToPush(): Promise<PushSubscriptionData | null> {
  const reg = await getRegistration();
  if (!reg) {
    return null;
  }

  // Explicitly request notification permission (required on Android Chrome)
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    return null;
  }

  const existingSub = await reg.pushManager.getSubscription();
  if (existingSub) {
    await existingSub.unsubscribe();
  }

  const applicationServerKey = urlBase64ToBufferKey(getVapidPublicKey());

  const sub = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey,
  });

  const data = subscriptionToData(sub);

  const res = await fetch("/api/push-subscriptions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });

  if (!res.ok) {
    await sub.unsubscribe();
    return null;
  }

  return data;
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

  const res = await fetch("/api/push-subscriptions", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ endpoint: sub.endpoint }),
  });

  // Unsubscribe browser-side to prevent resource leaks
  if (res.ok) {
    await sub.unsubscribe();
  }

  return res.ok;
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
// at which the subscription is considered expiring and should be refreshed.
const pushSubscriptionExpiryThresholdMs = 24 * 60 * 60 * 1000;

// pushResubscribeCooldownMs is the minimum interval between resubscriptions.
const pushResubscribeCooldownMs = 24 * 60 * 60 * 1000;

// pushLastResubscribeKey is the localStorage key for tracking last resubscribe time.
const pushLastResubscribeKey = "evcc_lastPushResubscribe";

function getLastResubscribeTime(): number {
  if (typeof localStorage === "undefined") return 0;
  try {
    const val = localStorage.getItem(pushLastResubscribeKey);
    return val ? Number(val) : 0;
  } catch {
    return 0;
  }
}

function setLastResubscribeTime() {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(pushLastResubscribeKey, String(Date.now()));
  } catch {
    // localStorage may be unavailable
  }
}

function canResubscribe(): boolean {
  const last = getLastResubscribeTime();
  return Date.now() - last >= pushResubscribeCooldownMs;
}

function isSubscriptionExpiring(sub: PushSubscription): boolean {
  if (sub.expirationTime === null) {
    return true;
  }
  const expiresInMs = sub.expirationTime - Date.now();
  return expiresInMs < pushSubscriptionExpiryThresholdMs;
}

/**
 * ensurePushSubscription checks the current push subscription on page load
 * and resubscribes if it's expiring. Uses localStorage cooldown to prevent
 * spamming resubscriptions. Safe to call from useEffect on every page load.
 */
export async function ensurePushSubscription(): Promise<void> {
  if (!isPushEnabled()) return;

  const reg = await getRegistration();
  if (!reg) return;

  const sub = await reg.pushManager.getSubscription();
  if (!sub || !isSubscriptionExpiring(sub)) return;
  if (!canResubscribe()) return;

  try {
    await sub.unsubscribe();

    const permission = await Notification.requestPermission();
    if (permission !== "granted") return;

    const applicationServerKey = urlBase64ToBufferKey(getVapidPublicKey());
    const newSub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey,
    });

    const data = subscriptionToData(newSub);

    const res = await fetch("/api/push-subscriptions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });

    if (res.ok) {
      setLastResubscribeTime();
    }
  } catch {
    // Silently fail - subscription will be retried on next page load
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
