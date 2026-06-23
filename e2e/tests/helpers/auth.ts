import { request, APIRequestContext } from "@playwright/test";

const API_BASE_URL = process.env.E2E_API_URL ?? "http://api:8080";
const TEST_EMAIL = process.env.E2E_TEST_EMAIL ?? "test@example.com";
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD ?? "password123";

export interface LoginResponse {
  accessToken: string;
}

export interface Vehicle {
  id: string;
  name: string;
  modelId: string;
  currentPercent: number;
  targetPercent: number;
  batteryCapacityKwh: number;
}

export interface Plug {
  id: string;
  userId: string;
  name: string;
  namespace: string;
  mqttTopic: string;
  type: string;
  powerOn: boolean;
  online: boolean;
  initialized: boolean;
  vehicleId?: string | null;
  createdAt: string;
}

export interface ChargeSession {
  id: string;
  vehicleId: string;
  status: string;
  energyAddedKwh: number | null;
  startPercent: number;
  currentPercent?: number | null;
  targetPercent?: number | null;
  endPercent: number | null;
  startedAt: string;
  stoppedAt: string | null;
}

export class ApiHelper {
  private apiContext: APIRequestContext;
  private accessToken: string;

  public constructor(apiContext: APIRequestContext, accessToken: string) {
    this.apiContext = apiContext;
    this.accessToken = accessToken;
  }

  private headers(): Record<string, string> {
    return {
      Authorization: `Bearer ${this.accessToken}`,
      "Content-Type": "application/json",
    };
  }

  public async get(path: string) {
    return this.apiContext.get(`${API_BASE_URL}${path}`, {
      headers: this.headers(),
    });
  }

  public async getJson<T>(path: string): Promise<T> {
    const response = await this.apiContext.get(`${API_BASE_URL}${path}`, {
      headers: this.headers(),
    });
    if (!response.ok()) {
      throw new Error(
        `API error: ${String(response.status())} ${response.statusText()} for ${path}`,
      );
    }
    const body = await response.text();
    if (!body) {
      throw new Error(`Empty response body for ${path}`);
    }
    return JSON.parse(body) as T;
  }

  // Returns null if session doesn't exist (204 No Content)
  public async getSession(path: string): Promise<ChargeSession | null> {
    const response = await this.apiContext.get(`${API_BASE_URL}${path}`, {
      headers: this.headers(),
    });
    if (response.status() === 204) {
      return null;
    }
    if (!response.ok()) {
      throw new Error(
        `API error: ${String(response.status())} ${response.statusText()} for ${path}`,
      );
    }
    const body = await response.text();
    if (!body) {
      return null;
    }
    return JSON.parse(body) as ChargeSession;
  }

  public async post(path: string, data?: unknown) {
    return this.apiContext.post(`${API_BASE_URL}${path}`, {
      headers: this.headers(),
      data,
    });
  }

  public async patch(path: string, data?: unknown) {
    return this.apiContext.patch(`${API_BASE_URL}${path}`, {
      headers: this.headers(),
      data,
    });
  }

  public async del(path: string) {
    return this.apiContext.delete(`${API_BASE_URL}${path}`, {
      headers: this.headers(),
    });
  }
}

export async function createApiHelper(): Promise<ApiHelper> {
  const apiContext = await request.newContext();
  const loginResponse = await apiContext.post(
    `${API_BASE_URL}/api/auth/login`,
    {
      data: {
        email: TEST_EMAIL,
        password: TEST_PASSWORD,
      },
    },
  );

  if (!loginResponse.ok()) {
    const status = loginResponse.status();
    throw new Error(`ApiHelper: login failed (${String(status)})`);
  }

  const loginData = (await loginResponse.json()) as LoginResponse;
  return new ApiHelper(apiContext, loginData.accessToken);
}

/**
 * Reset database and mock-tasmota to seed state.
 * Uses unauthenticated POST to dev-only /api/reset endpoint (guarded by cfg.IsDev()).
 * The reset endpoint blocks until all plugs are online via MQTT LWT,
 * so no additional polling is needed on the client side.
 */
export async function resetAllState(): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/api/reset`, { method: "POST" });
  if (response.status !== 200) {
    const body = await response.text().catch(() => "");
    throw new Error(
      `resetAllState: expected 200, got ${String(response.status)} ${body}`,
    );
  }
}
