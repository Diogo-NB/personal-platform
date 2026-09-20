export const serviceStates = ["stopped", "starting", "running", "stopping"] as const;

export type ServiceState = (typeof serviceStates)[number];

export interface ServiceStatus {
  state: ServiceState;
  startedAt: string | null;
  uptimeSeconds: number | null;
}

export type LifecycleAction = "start" | "stop";

export class APIError extends Error {
  constructor(
    readonly status: number | null,
    message: string,
  ) {
    super(message);
    this.name = "APIError";
  }
}

export class ManagementAPI {
  constructor(
    private readonly baseURL: string,
    private readonly fetchRequest: typeof fetch,
  ) {}

  async status(apiKey: string): Promise<ServiceStatus> {
    const response = await this.request("/status", apiKey, { method: "GET" });
    const payload: unknown = await response.json();
    if (!isServiceStatus(payload)) {
      throw new APIError(response.status, "The API returned an invalid status response.");
    }

    return payload;
  }

  async command(action: LifecycleAction, apiKey: string): Promise<void> {
    await this.request(`/${action}`, apiKey, { method: "POST" });
  }

  private async request(path: string, apiKey: string, init: RequestInit): Promise<Response> {
    let response: Response;
    try {
      response = await this.fetchRequest(`${this.baseURL}${path}`, {
        ...init,
        cache: "no-store",
        credentials: "omit",
        headers: { "X-Api-Key": apiKey },
        referrerPolicy: "no-referrer",
      });
    } catch {
      throw new APIError(null, "The management API could not be reached.");
    }

    if (!response.ok) {
      throw new APIError(response.status, responseMessage(response.status));
    }

    return response;
  }
}

function isServiceStatus(value: unknown): value is ServiceStatus {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const status = value as Record<string, unknown>;
  return (
    serviceStates.includes(status.state as ServiceState) &&
    (status.startedAt === null || typeof status.startedAt === "string") &&
    (status.uptimeSeconds === null ||
      (typeof status.uptimeSeconds === "number" && Number.isFinite(status.uptimeSeconds)))
  );
}

function responseMessage(status: number): string {
  if (status === 401 || status === 403) {
    return "The API key was not accepted.";
  }
  if (status === 429) {
    return "The API is receiving too many requests. Try again shortly.";
  }
  if (status >= 500) {
    return "The management service is temporarily unavailable.";
  }

  return "The management request was rejected.";
}
