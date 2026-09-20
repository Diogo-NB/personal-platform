import { describe, expect, it } from "vitest";

import { readRuntimeConfig } from "./runtime-config";

describe("readRuntimeConfig", () => {
  it("normalizes the deployment-provided API URL", () => {
    const target = {
      __TS6_RUNTIME_CONFIG__: { apiBaseUrl: " https://api.example/prod/// " },
    } as Window;

    expect(readRuntimeConfig(target, { VITE_API_BASE_URL: "http://localhost:8080" })).toEqual({
      apiBaseUrl: "https://api.example/prod",
    });
  });

  it("uses the Vite environment URL when deployment configuration is absent", () => {
    expect(
      readRuntimeConfig({} as Window, {
        VITE_API_BASE_URL: " http://localhost:8080/ ",
      }),
    ).toEqual({ apiBaseUrl: "http://localhost:8080" });
  });

  it("rejects missing configuration", () => {
    expect(() => readRuntimeConfig({} as Window, {})).toThrow("API endpoint is not configured");
  });

  it("contains no API key field", () => {
    const config = readRuntimeConfig(
      {
        __TS6_RUNTIME_CONFIG__: { apiBaseUrl: "https://api.example/prod" },
      } as Window,
      {},
    );

    expect(Object.keys(config)).toEqual(["apiBaseUrl"]);
  });
});
