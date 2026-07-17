import {
  getVapidPublicKey,
  isPushEnabled,
  registerServiceWorker,
  subscribeToPush,
  unsubscribeFromPush,
  getPushSubscription,
  ensurePushSubscription,
} from "@/lib/push";
import { describe, it, expect, vi, beforeEach } from "vitest";

function setNotificationPermission(permission: NotificationPermission) {
  global.Notification = {
    permission,
    requestPermission: vi.fn().mockResolvedValue(permission),
  } as unknown as typeof Notification;
}

describe("push utilities", () => {
  const mockPushManager = {
    getSubscription: vi.fn(),
    subscribe: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = "dGVzdA";

    const mockSw = {
      register: vi.fn().mockResolvedValue({ pushManager: mockPushManager }),
      ready: Promise.resolve({ pushManager: mockPushManager }),
    };
    Object.defineProperty(navigator, "serviceWorker", {
      value: mockSw,
      writable: true,
      configurable: true,
    });
    global.fetch = vi.fn().mockResolvedValue({ ok: true, status: 201 });
    setNotificationPermission("granted");
    mockPushManager.getSubscription.mockResolvedValue(null);
  });

  describe("getVapidPublicKey", () => {
    it("returns a string", () => {
      const result = getVapidPublicKey();
      expect(typeof result).toBe("string");
    });

    it("returns empty string when env var is not set", () => {
      const original = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      delete process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      const result = getVapidPublicKey();
      expect(result).toBe("");
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = original;
    });
  });

  describe("isPushEnabled", () => {
    it("returns true when serviceWorker exists and key is provided", () => {
      expect(isPushEnabled("testKey123")).toBe(true);
    });

    it("returns false when key is empty", () => {
      expect(isPushEnabled("")).toBe(false);
    });

    it("returns false when serviceWorker is not available", () => {
      delete (navigator as any).serviceWorker;
      expect(isPushEnabled("testKey123")).toBe(false);
    });
  });

  describe("registerServiceWorker", () => {
    it("registers sw.js with updateViaCache disabled so fixes propagate", async () => {
      const mockSw = navigator.serviceWorker as any;
      const reg = { pushManager: mockPushManager };
      mockSw.register.mockResolvedValueOnce(reg);
      const result = await registerServiceWorker();
      expect(mockSw.register).toHaveBeenCalledWith("/sw.js", {
        updateViaCache: "none",
      });
      expect(result).toBe(reg);
    });

    it("returns null when serviceWorker is not available", async () => {
      delete (navigator as any).serviceWorker;
      const result = await registerServiceWorker();
      expect(result).toBeNull();
    });
  });

  describe("subscribeToPush", () => {
    it("subscribes and posts to API, returning subscription data", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(mockSub);

      const result = await subscribeToPush();
      expect(result).toEqual({
        endpoint: "https://push.example/sub1",
        p256dhKey: "abc",
        authKey: "xyz",
      });
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("returns null when API rejects but keeps the browser subscription", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        unsubscribe: vi.fn().mockResolvedValue(true),
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: false, status: 500 });

      const result = await subscribeToPush();
      expect(result).toBeNull();
      // The browser subscription survives so the app-open sync can retry the
      // server registration later.
      expect(mockSub.unsubscribe).not.toHaveBeenCalled();
    });

    it("returns null when no service worker registration", async () => {
      delete (navigator as any).serviceWorker;
      const result = await subscribeToPush();
      expect(result).toBeNull();
    });

    it("returns null when notification permission is denied", async () => {
      (
        global.Notification.requestPermission as ReturnType<typeof vi.fn>
      ).mockResolvedValueOnce("denied");
      const result = await subscribeToPush();
      expect(result).toBeNull();
      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
    });

    it("reuses an existing subscription without unsubscribing it", async () => {
      const existingSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(existingSub);
      // subscribe() returns the existing subscription when the key matches.
      mockPushManager.subscribe.mockResolvedValueOnce(existingSub);

      const result = await subscribeToPush();
      expect(existingSub.unsubscribe).not.toHaveBeenCalled();
      expect(result).toEqual({
        endpoint: "https://push.example/sub1",
        p256dhKey: "abc",
        authKey: "xyz",
      });
    });

    it("returns null when subscribing fails and there is no existing subscription", async () => {
      mockPushManager.getSubscription.mockResolvedValueOnce(null);
      mockPushManager.subscribe.mockRejectedValueOnce(
        new Error("subscription failed"),
      );

      const result = await subscribeToPush();
      expect(result).toBeNull();
      expect(mockPushManager.subscribe).toHaveBeenCalledTimes(1);
    });

    it("returns null when rotation fails after the browser rejects the old key", async () => {
      const existingSub = {
        endpoint: "https://push.example/old",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockRejectedValue(new Error("unsubscribe failed")),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(existingSub);
      mockPushManager.subscribe.mockRejectedValueOnce(
        new Error("InvalidStateError"),
      );

      const result = await subscribeToPush();
      expect(result).toBeNull();
    });

    it("rotates the subscription when the browser rejects subscribing over the old key", async () => {
      const existingSub = {
        endpoint: "https://push.example/old",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(existingSub);

      const newSub = {
        endpoint: "https://push.example/new",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe
        .mockRejectedValueOnce(new Error("InvalidStateError"))
        .mockResolvedValueOnce(newSub);

      const result = await subscribeToPush();
      expect(existingSub.unsubscribe).toHaveBeenCalled();
      expect(result).toEqual({
        endpoint: "https://push.example/new",
        p256dhKey: "def",
        authKey: "uvw",
      });
      // Old endpoint is cleaned up server-side.
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({
          method: "DELETE",
          body: JSON.stringify({ endpoint: "https://push.example/old" }),
        }),
      );
    });
  });

  describe("unsubscribeFromPush", () => {
    it("deletes server-side and unsubscribes browser-side", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 204 });

      const result = await unsubscribeFromPush();
      expect(result).toBe(true);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({ method: "DELETE" }),
      );
      expect(mockSub.unsubscribe).toHaveBeenCalled();
    });

    it("returns false when no existing subscription", async () => {
      mockPushManager.getSubscription.mockResolvedValueOnce(null);
      const result = await unsubscribeFromPush();
      expect(result).toBe(false);
    });

    it("still unsubscribes browser-side when the API is unreachable", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockRejectedValueOnce(new Error("offline"));

      const result = await unsubscribeFromPush();
      // The user's intent is to stop notifications; the browser unsubscribe
      // guarantees that even when the server delete fails.
      expect(mockSub.unsubscribe).toHaveBeenCalled();
      expect(result).toBe(true);
    });

    it("returns false when no service worker registration", async () => {
      delete (navigator as any).serviceWorker;
      const result = await unsubscribeFromPush();
      expect(result).toBe(false);
    });
  });

  describe("getPushSubscription", () => {
    it("returns subscription data when subscribed", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      const result = await getPushSubscription();
      expect(result).toEqual({
        endpoint: "https://push.example/sub1",
        p256dhKey: "abc",
        authKey: "xyz",
      });
    });

    it("returns null when not subscribed", async () => {
      mockPushManager.getSubscription.mockResolvedValueOnce(null);
      const result = await getPushSubscription();
      expect(result).toBeNull();
    });

    it("returns null when no service worker registration", async () => {
      delete (navigator as any).serviceWorker;
      const result = await getPushSubscription();
      expect(result).toBeNull();
    });
  });

  describe("ensurePushSubscription", () => {
    it("re-syncs a healthy never-expiring subscription to the server without touching it", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await ensurePushSubscription();

      // expirationTime === null means the subscription never expires; it must
      // never be unsubscribed or rotated.
      expect(mockSub.unsubscribe).not.toHaveBeenCalled();
      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
      // But it is re-upserted so a server that lost the record heals.
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            endpoint: "https://push.example/sub1",
            p256dhKey: "abc",
            authKey: "xyz",
          }),
        }),
      );
    });

    it("re-syncs a subscription with a far-future expiry without rotating it", async () => {
      const futureTime = Date.now() + 48 * 60 * 60 * 1000;
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: futureTime,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await ensurePushSubscription();

      expect(mockSub.unsubscribe).not.toHaveBeenCalled();
      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("silently resubscribes when permission is granted but no subscription exists", async () => {
      mockPushManager.getSubscription.mockResolvedValueOnce(null);
      const newSub = {
        endpoint: "https://push.example/new",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(newSub);

      await ensurePushSubscription();

      expect(mockPushManager.subscribe).toHaveBeenCalled();
      // Never prompts: recovery must not depend on a permission dialog.
      expect(global.Notification.requestPermission).not.toHaveBeenCalled();
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("renews a genuinely expiring subscription", async () => {
      const expiringSub = {
        endpoint: "https://push.example/old",
        expirationTime: Date.now() + 60 * 1000,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(expiringSub);

      const newSub = {
        endpoint: "https://push.example/new",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(newSub);

      await ensurePushSubscription();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            endpoint: "https://push.example/new",
            p256dhKey: "def",
            authKey: "uvw",
          }),
        }),
      );
      // Old endpoint removed server-side after the new one is registered.
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({
          method: "DELETE",
          body: JSON.stringify({ endpoint: "https://push.example/old" }),
        }),
      );
    });

    it("force-rotates when the browser returns the same still-expiring subscription", async () => {
      const soon = Date.now() + 60 * 1000;
      const expiringSub = {
        endpoint: "https://push.example/old",
        expirationTime: soon,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(expiringSub);

      const rotatedSub = {
        endpoint: "https://push.example/rotated",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      // subscribe() with a matching key returns the existing (still expiring)
      // subscription; only an explicit unsubscribe forces a fresh one.
      mockPushManager.subscribe
        .mockResolvedValueOnce(expiringSub)
        .mockResolvedValueOnce(rotatedSub);

      await ensurePushSubscription();

      expect(expiringSub.unsubscribe).toHaveBeenCalled();
      expect(mockPushManager.subscribe).toHaveBeenCalledTimes(2);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            endpoint: "https://push.example/rotated",
            p256dhKey: "def",
            authKey: "uvw",
          }),
        }),
      );
    });

    it("does nothing when notification permission is not granted", async () => {
      setNotificationPermission("default");
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await ensurePushSubscription();

      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
      expect(global.fetch).not.toHaveBeenCalled();
      expect(global.Notification.requestPermission).not.toHaveBeenCalled();
    });

    it("does nothing when push is not enabled", async () => {
      const originalKey = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = "";

      await ensurePushSubscription();

      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = originalKey;
    });

    it("keeps the browser subscription when the server sync fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 });
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await expect(ensurePushSubscription()).resolves.toBeUndefined();
      expect(mockSub.unsubscribe).not.toHaveBeenCalled();
    });

    it("silently catches errors", async () => {
      global.fetch = vi.fn().mockRejectedValue(new Error("offline"));
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await expect(ensurePushSubscription()).resolves.toBeUndefined();
      expect(mockSub.unsubscribe).not.toHaveBeenCalled();
    });
  });
});
