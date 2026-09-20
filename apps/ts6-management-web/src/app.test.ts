import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { mountManagementApp } from "./app";
import type { ServiceStatus } from "./api";

const stopped: ServiceStatus = {
  state: "stopped",
  startedAt: null,
  uptimeSeconds: null,
};
const running: ServiceStatus = {
  state: "running",
  startedAt: "2026-09-19T18:30:00Z",
  uptimeSeconds: 3_661,
};

let root: HTMLElement;
let unmount: (() => void) | undefined;

beforeEach(() => {
  root = document.createElement("main");
  document.body.replaceChildren(root);
  setDocumentHidden(false);
});

afterEach(() => {
  unmount?.();
  unmount = undefined;
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("TeamSpeak management application", () => {
  it("validates the API key without persisting or logging it", async () => {
    const fetchRequest = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(stopped));
    const localStorageWrite = vi.spyOn(Storage.prototype, "setItem");
    const consoleLog = vi.spyOn(console, "log").mockImplementation(() => undefined);
    const initialURL = window.location.href;
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });

    await authenticate("private-key");

    expect(fetchRequest).toHaveBeenCalledTimes(1);
    expect(fetchRequest).toHaveBeenCalledWith(
      "https://api.example/prod/status",
      expect.objectContaining({
        method: "GET",
        credentials: "omit",
        headers: { "X-Api-Key": "private-key" },
      }),
    );
    expect(byTestID<HTMLInputElement>("api-key").value).toBe("");
    expect(byTestID("access-panel").hidden).toBe(true);
    expect(byTestID("console-panel").hidden).toBe(false);
    expect(document.activeElement).toBe(byTestID("state-label"));
    expect(localStorageWrite).not.toHaveBeenCalled();
    expect(consoleLog).not.toHaveBeenCalled();
    expect(window.location.href).toBe(initialURL);
  });

  it("renders running status and state-dependent controls", async () => {
    const fetchRequest = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(running));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });

    await authenticate("key");

    expect(byTestID("state-label").textContent).toBe("running");
    expect(byTestID("state-detail").textContent).toContain("online");
    expect(byTestID("started-at").textContent).toContain("15:30:00");
    expect(byTestID("started-at").textContent).toContain("Brasília time");
    expect(byTestID("uptime").textContent).toBe("1h 1m");
    expect(byTestID<HTMLButtonElement>("start-button").disabled).toBe(true);
    expect(byTestID<HTMLButtonElement>("stop-button").disabled).toBe(false);
    expect(root.querySelector('[data-service-state="running"]')?.getAttribute("aria-current")).toBe(
      "step",
    );
  });

  it.each([
    {
      name: "start",
      initial: stopped,
      testID: "start-button",
      next: { ...stopped, state: "starting" } satisfies ServiceStatus,
    },
    {
      name: "stop",
      initial: running,
      testID: "stop-button",
      next: { ...stopped, state: "stopping" } satisfies ServiceStatus,
    },
  ])("sends an empty POST /$name request", async ({ name, initial, testID, next }) => {
    const fetchRequest = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(initial))
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(jsonResponse(next));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });
    await authenticate("key");

    byTestID<HTMLButtonElement>(testID).click();

    await vi.waitFor(() => expect(fetchRequest).toHaveBeenCalledTimes(3));
    expect(fetchRequest).toHaveBeenNthCalledWith(
      2,
      `https://api.example/prod/${name}`,
      expect.not.objectContaining({ body: expect.anything() }),
    );
    expect(fetchRequest.mock.calls[1]?.[1]).toMatchObject({
      method: "POST",
      headers: { "X-Api-Key": "key" },
    });
    expect(byTestID("state-label").textContent).toBe(next.state);
  });

  it("disables lifecycle controls while a command is pending", async () => {
    const command = deferred<Response>();
    const fetchRequest = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(stopped))
      .mockReturnValueOnce(command.promise)
      .mockResolvedValueOnce(jsonResponse({ ...stopped, state: "starting" }));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });
    await authenticate("key");

    byTestID<HTMLButtonElement>("start-button").click();

    expect(byTestID<HTMLButtonElement>("start-button").disabled).toBe(true);
    expect(byTestID<HTMLButtonElement>("stop-button").disabled).toBe(true);
    command.resolve(new Response(null, { status: 202 }));
    await vi.waitFor(() => expect(fetchRequest).toHaveBeenCalledTimes(3));
  });

  it("polls every two seconds without overlapping requests", async () => {
    vi.useFakeTimers();
    const poll = deferred<Response>();
    const fetchRequest = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(stopped))
      .mockReturnValueOnce(poll.promise)
      .mockResolvedValue(jsonResponse(running));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });
    await authenticate("key");

    await vi.advanceTimersByTimeAsync(6_000);
    expect(fetchRequest).toHaveBeenCalledTimes(2);

    poll.resolve(jsonResponse(running));
    await vi.runAllTicks();
    await vi.advanceTimersByTimeAsync(2_000);
    expect(fetchRequest).toHaveBeenCalledTimes(3);
  });

  it("pauses polling while hidden and refreshes immediately when visible", async () => {
    vi.useFakeTimers();
    const fetchRequest = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(stopped));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });
    await authenticate("key");

    setDocumentHidden(true);
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(4_000);
    expect(fetchRequest).toHaveBeenCalledTimes(1);

    setDocumentHidden(false);
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.waitFor(() => expect(fetchRequest).toHaveBeenCalledTimes(2));
  });

  it("keeps authentication controls visible after an invalid key", async () => {
    const fetchRequest = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response('{"message":"Forbidden"}', { status: 403 }));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });

    await authenticate("wrong-key", false);

    expect(byTestID("access-panel").hidden).toBe(false);
    expect(byTestID("console-panel").hidden).toBe(true);
    expect(byTestID("feedback").textContent).toBe("The API key was not accepted.");
    expect(byTestID<HTMLInputElement>("api-key").value).toBe("wrong-key");
    expect(byTestID<HTMLInputElement>("api-key").getAttribute("aria-invalid")).toBe("true");
  });

  it("reports polling network errors and disables commands", async () => {
    vi.useFakeTimers();
    const fetchRequest = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(running))
      .mockRejectedValueOnce(new TypeError("network unavailable"));
    unmount = mountManagementApp(root, "https://api.example/prod", { fetchRequest });
    await authenticate("key");

    await vi.advanceTimersByTimeAsync(2_000);

    expect(byTestID("state-label").textContent).toBe("unavailable");
    expect(byTestID("feedback").textContent).toBe("The management API could not be reached.");
    expect(byTestID<HTMLButtonElement>("start-button").disabled).toBe(true);
    expect(byTestID<HTMLButtonElement>("stop-button").disabled).toBe(true);
  });
});

async function authenticate(apiKey: string, expectSuccess = true): Promise<void> {
  const input = byTestID<HTMLInputElement>("api-key");
  input.value = apiKey;
  byTestID<HTMLFormElement>("access-form").dispatchEvent(
    new SubmitEvent("submit", { bubbles: true, cancelable: true }),
  );
  await vi.waitFor(() => {
    if (expectSuccess) {
      expect(byTestID("console-panel").hidden).toBe(false);
    } else {
      expect(byTestID("feedback").hidden).toBe(false);
    }
  });
}

function byTestID<T extends HTMLElement = HTMLElement>(testID: string): T {
  const found = root.querySelector<T>(`[data-testid="${testID}"]`);
  if (!found) {
    throw new Error(`Missing test element: ${testID}`);
  }
  return found;
}

function jsonResponse(status: ServiceStatus): Response {
  return new Response(JSON.stringify(status), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function deferred<T>(): {
  promise: Promise<T>;
  resolve: (value: T) => void;
} {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function setDocumentHidden(hidden: boolean): void {
  Object.defineProperty(document, "hidden", {
    configurable: true,
    value: hidden,
  });
}
