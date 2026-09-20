export interface RuntimeConfig {
  apiBaseUrl: string;
}

interface RuntimeEnvironment {
  VITE_API_BASE_URL?: string;
}

declare global {
  interface Window {
    __TS6_RUNTIME_CONFIG__?: RuntimeConfig;
  }
}

export function readRuntimeConfig(
  target: Window = window,
  environment: RuntimeEnvironment = import.meta.env,
): RuntimeConfig {
  const runtimeApiBaseUrl = target.__TS6_RUNTIME_CONFIG__?.apiBaseUrl?.trim();
  const apiBaseUrl = runtimeApiBaseUrl || environment.VITE_API_BASE_URL?.trim();
  if (!apiBaseUrl) {
    throw new Error("The API endpoint is not configured.");
  }

  return { apiBaseUrl: apiBaseUrl.replace(/\/+$/, "") };
}
