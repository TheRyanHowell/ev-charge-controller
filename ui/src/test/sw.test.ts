// Tests for the service worker (public/sw.js).
//
// The service worker is plain JS outside the module graph, so it is loaded
// from disk and evaluated against a mocked `self`/`clients`/`fetch`. This
// guards the two behaviors that historically broke notifications:
// - the push handler must ONLY display the notification (a previous version
//   rotated the subscription on every push and silently killed delivery)
// - pushsubscriptionchange must re-register with camelCase field names (the
//   API's strict decoder rejects anything else with a 400)
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it, expect, vi, beforeEach } from "vitest";

type Listener = (event: unknown) => void;

interface MockSubscription {
  endpoint: string;
  options?: { applicationServerKey: ArrayBuffer };
  toJSON: () => { keys?: { p256dh?: string; auth?: string } };
}

function makeSubscription(
  endpoint: string,
  applicationServerKey?: ArrayBuffer,
): MockSubscription {
  return {
    endpoint,
    ...(applicationServerKey ? { options: { applicationServerKey } } : {}),
    toJSON: () => ({ keys: { p256dh: "p256", auth: "auth" } }),
  };
}

function loadServiceWorker() {
  const code = readFileSync(resolve(process.cwd(), "public/sw.js"), "utf8");

  const listeners = new Map<string, Listener>();
  const showNotification = vi.fn().mockResolvedValue(undefined);
  const subscribe = vi.fn();
  const swSelf = {
    addEventListener: (name: string, fn: Listener) => listeners.set(name, fn),
    skipWaiting: vi.fn(),
    registration: {
      showNotification,
      pushManager: { subscribe },
    },
  };
  const clients = {
    claim: vi.fn().mockResolvedValue(undefined),
    matchAll: vi.fn(),
    openWindow: vi.fn().mockResolvedValue(undefined),
  };
  const fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ publicKey: "dGVzdA" }),
  });

  new Function("self", "clients", "fetch", code)(swSelf, clients, fetchMock);

  const fire = (name: string): Listener => {
    const listener = listeners.get(name);
    if (!listener) {
      throw new Error(`service worker registered no '${name}' listener`);
    }
    return listener;
  };

  return {
    listeners,
    swSelf,
    clients,
    fetchMock,
    showNotification,
    subscribe,
    fire,
  };
}

function makeEvent(fields: Record<string, unknown>) {
  const waits: Promise<unknown>[] = [];
  return {
    event: {
      ...fields,
      waitUntil: (p: Promise<unknown>) => waits.push(p),
    },
    settle: () => Promise.all(waits),
  };
}

interface FetchCall {
  url: string;
  method: string;
  body: string | undefined;
}

function fetchCalls(fetchMock: ReturnType<typeof vi.fn>): FetchCall[] {
  return fetchMock.mock.calls.map(([url, init]) => ({
    url: url as string,
    method: ((init as RequestInit | undefined)?.method ?? "GET") as string,
    body: (init as RequestInit | undefined)?.body as string | undefined,
  }));
}

function mustFindBody(
  fetchMock: ReturnType<typeof vi.fn>,
  method: string,
): unknown {
  const call = fetchCalls(fetchMock).find((c) => c.method === method);
  if (!call || call.body === undefined) {
    throw new Error(`expected a ${method} call with a body`);
  }
  return JSON.parse(call.body);
}

describe("service worker", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("registers handlers for the full notification lifecycle", () => {
    const { listeners } = loadServiceWorker();
    expect([...listeners.keys()]).toEqual(
      expect.arrayContaining([
        "install",
        "activate",
        "push",
        "pushsubscriptionchange",
        "notificationclick",
      ]),
    );
  });

  it("activates immediately on install", async () => {
    const { swSelf, clients, fire } = loadServiceWorker();

    fire("install")(makeEvent({}).event);
    expect(swSelf.skipWaiting).toHaveBeenCalled();

    const { event, settle } = makeEvent({});
    fire("activate")(event);
    await settle();
    expect(clients.claim).toHaveBeenCalled();
  });

  describe("push", () => {
    it("shows the notification from the payload", async () => {
      const { showNotification, fire } = loadServiceWorker();
      const { event, settle } = makeEvent({
        data: {
          json: () => ({ title: "Charge Complete", body: "RM1 reached 80%" }),
        },
      });

      fire("push")(event);
      await settle();

      expect(showNotification).toHaveBeenCalledWith(
        "Charge Complete",
        expect.objectContaining({
          body: "RM1 reached 80%",
          requireInteraction: true,
        }),
      );
    });

    it("falls back to defaults when the push has no payload", async () => {
      const { showNotification, fire } = loadServiceWorker();
      const { event, settle } = makeEvent({ data: null });

      fire("push")(event);
      await settle();

      expect(showNotification).toHaveBeenCalledWith(
        "EV Charge",
        expect.objectContaining({ body: "" }),
      );
    });

    it("never touches the subscription or the network from the push handler", async () => {
      const { fetchMock, subscribe, fire } = loadServiceWorker();
      const { event, settle } = makeEvent({
        data: { json: () => ({ title: "T", body: "B" }) },
      });

      fire("push")(event);
      await settle();

      // Regression guard: a previous version rotated the subscription on
      // every received push, which destroyed delivery on any failure.
      expect(subscribe).not.toHaveBeenCalled();
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe("pushsubscriptionchange", () => {
    it("resubscribes with the old subscription's key and re-registers camelCase fields", async () => {
      const { fetchMock, subscribe, fire } = loadServiceWorker();
      const oldKey = new ArrayBuffer(8);
      const oldSub = makeSubscription("https://push.example/old", oldKey);
      const newSub = makeSubscription("https://push.example/new");
      subscribe.mockResolvedValueOnce(newSub);

      const { event, settle } = makeEvent({ oldSubscription: oldSub });
      fire("pushsubscriptionchange")(event);
      await settle();

      expect(subscribe).toHaveBeenCalledWith({
        userVisibleOnly: true,
        applicationServerKey: oldKey,
      });

      // The API decodes with DisallowUnknownFields and requires camelCase.
      expect(mustFindBody(fetchMock, "POST")).toEqual({
        endpoint: "https://push.example/new",
        p256dhKey: "p256",
        authKey: "auth",
      });

      expect(mustFindBody(fetchMock, "DELETE")).toEqual({
        endpoint: "https://push.example/old",
      });
    });

    it("does not delete the old endpoint when it is unchanged", async () => {
      const { fetchMock, subscribe, fire } = loadServiceWorker();
      const sub = makeSubscription(
        "https://push.example/same",
        new ArrayBuffer(8),
      );
      subscribe.mockResolvedValueOnce(sub);

      const { event, settle } = makeEvent({ oldSubscription: sub });
      fire("pushsubscriptionchange")(event);
      await settle();

      const methods = fetchCalls(fetchMock).map((c) => c.method);
      expect(methods).toContain("POST");
      expect(methods).not.toContain("DELETE");
    });

    it("fetches the VAPID key when the old subscription is unavailable", async () => {
      const { fetchMock, subscribe, fire } = loadServiceWorker();
      const newSub = makeSubscription("https://push.example/new");
      subscribe.mockResolvedValueOnce(newSub);

      const { event, settle } = makeEvent({});
      fire("pushsubscriptionchange")(event);
      await settle();

      expect(fetchCalls(fetchMock)[0]).toMatchObject({
        url: "/api/push-subscriptions",
        method: "GET",
      });
      expect(subscribe).toHaveBeenCalledWith(
        expect.objectContaining({
          userVisibleOnly: true,
          applicationServerKey: expect.any(ArrayBuffer),
        }),
      );
    });

    it("uses the browser-provided replacement subscription when present", async () => {
      const { fetchMock, subscribe, fire } = loadServiceWorker();
      const oldSub = makeSubscription(
        "https://push.example/old",
        new ArrayBuffer(8),
      );
      const replacement = makeSubscription("https://push.example/replacement");

      const { event, settle } = makeEvent({
        oldSubscription: oldSub,
        newSubscription: replacement,
      });
      fire("pushsubscriptionchange")(event);
      await settle();

      expect(subscribe).not.toHaveBeenCalled();
      expect(mustFindBody(fetchMock, "POST")).toMatchObject({
        endpoint: "https://push.example/replacement",
      });
    });

    it("swallows renewal failures so the event handler never rejects", async () => {
      const consoleError = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});
      const { subscribe, fire } = loadServiceWorker();
      subscribe.mockRejectedValueOnce(new Error("push service down"));

      const { event, settle } = makeEvent({
        oldSubscription: makeSubscription(
          "https://push.example/old",
          new ArrayBuffer(8),
        ),
      });
      fire("pushsubscriptionchange")(event);

      await expect(settle()).resolves.toBeDefined();
      expect(consoleError).toHaveBeenCalledWith(
        "[SW] Failed to renew push subscription:",
        expect.any(Error),
      );
    });
  });

  describe("notificationclick", () => {
    it("focuses an existing window", async () => {
      const { clients, fire } = loadServiceWorker();
      const focus = vi.fn().mockResolvedValue(undefined);
      clients.matchAll.mockResolvedValueOnce([{ focus }]);

      const close = vi.fn();
      const { event, settle } = makeEvent({ notification: { close } });
      fire("notificationclick")(event);
      await settle();

      expect(close).toHaveBeenCalled();
      expect(focus).toHaveBeenCalled();
      expect(clients.openWindow).not.toHaveBeenCalled();
    });

    it("opens a new window when none exists", async () => {
      const { clients, fire } = loadServiceWorker();
      clients.matchAll.mockResolvedValueOnce([]);

      const { event, settle } = makeEvent({ notification: { close: vi.fn() } });
      fire("notificationclick")(event);
      await settle();

      expect(clients.openWindow).toHaveBeenCalledWith("/");
    });
  });
});
