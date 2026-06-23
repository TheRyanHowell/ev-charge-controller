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
    global.fetch = vi.fn();
    global.Notification = {
      requestPermission: vi.fn().mockResolvedValue("granted"),
    } as unknown as typeof Notification;

    // Reset localStorage mock
    Object.defineProperty(window, "localStorage", {
      value: {
        getItem: vi.fn().mockReturnValue(null),
        setItem: vi.fn(),
      },
      writable: true,
      configurable: true,
    });
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
    it("returns the registration on success", async () => {
      const mockSw = navigator.serviceWorker as any;
      const reg = { pushManager: mockPushManager };
      mockSw.register.mockResolvedValueOnce(reg);
      const result = await registerServiceWorker();
      expect(mockSw.register).toHaveBeenCalledWith("/sw.js");
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
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 201 });

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

    it("unsubscribes and returns null when API rejects", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        unsubscribe: vi.fn().mockResolvedValue(true),
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: false, status: 500 });

      const result = await subscribeToPush();
      expect(result).toBeNull();
      expect(mockSub.unsubscribe).toHaveBeenCalled();
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
    });

    it("unsubscribes existing subscription before creating new one", async () => {
      const existingSub = {
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(existingSub);

      const mockSub = {
        endpoint: "https://push.example/sub2",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 201 });

      const result = await subscribeToPush();
      expect(existingSub.unsubscribe).toHaveBeenCalled();
      expect(result).toEqual({
        endpoint: "https://push.example/sub2",
        p256dhKey: "def",
        authKey: "uvw",
      });
    });
  });

  describe("unsubscribeFromPush", () => {
    it("returns true when API accepts", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 200 });

      const result = await unsubscribeFromPush();
      expect(result).toBe(true);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("returns false when no existing subscription", async () => {
      mockPushManager.getSubscription.mockResolvedValueOnce(null);
      const result = await unsubscribeFromPush();
      expect(result).toBe(false);
    });

    it("returns false when API rejects", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: false, status: 404 });

      const result = await unsubscribeFromPush();
      expect(result).toBe(false);
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
    it("resubscribes when subscription has null expirationTime", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      const newMockSub = {
        endpoint: "https://push.example/sub2",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(newMockSub);

      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 201 });

      await ensurePushSubscription();

      expect(mockSub.unsubscribe).toHaveBeenCalled();
      expect(mockPushManager.subscribe).toHaveBeenCalled();
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/push-subscriptions",
        expect.objectContaining({ method: "POST" }),
      );
      expect(localStorage.setItem).toHaveBeenCalledWith(
        "evcc_lastPushResubscribe",
        expect.any(String),
      );
    });

    it("does nothing when subscription is not expiring", async () => {
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
    });

    it("does nothing when within cooldown period", async () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        String(Date.now() - 1000),
      );

      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await ensurePushSubscription();

      expect(mockSub.unsubscribe).not.toHaveBeenCalled();
    });

    it("does nothing when not subscribed", async () => {
      mockPushManager.getSubscription.mockResolvedValueOnce(null);

      await ensurePushSubscription();

      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
    });

    it("does nothing when push is not enabled", async () => {
      const originalKey = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = "";

      await ensurePushSubscription();

      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = originalKey;
    });

    it("silently catches errors during resubscription", async () => {
      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockRejectedValue(new Error("boom")),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      await expect(ensurePushSubscription()).resolves.toBeUndefined();
      expect(mockPushManager.subscribe).not.toHaveBeenCalled();
    });

    it("handles localStorage getItem throwing", async () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockImplementation(
        () => {
          throw new Error("localStorage unavailable");
        },
      );

      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      const newMockSub = {
        endpoint: "https://push.example/sub2",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(newMockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 201 });

      await ensurePushSubscription();
      expect(mockPushManager.subscribe).toHaveBeenCalled();
    });

    it("handles localStorage setItem throwing", async () => {
      (localStorage.setItem as ReturnType<typeof vi.fn>).mockImplementation(
        () => {
          throw new Error("localStorage unavailable");
        },
      );

      const mockSub = {
        endpoint: "https://push.example/sub1",
        expirationTime: null,
        toJSON: () => ({ keys: { p256dh: "abc", auth: "xyz" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.getSubscription.mockResolvedValueOnce(mockSub);

      const newMockSub = {
        endpoint: "https://push.example/sub2",
        toJSON: () => ({ keys: { p256dh: "def", auth: "uvw" } }),
        unsubscribe: vi.fn().mockResolvedValue(true),
      };
      mockPushManager.subscribe.mockResolvedValueOnce(newMockSub);
      global.fetch = vi.fn().mockResolvedValueOnce({ ok: true, status: 201 });

      await ensurePushSubscription();
      expect(mockPushManager.subscribe).toHaveBeenCalled();
    });
  });
});
