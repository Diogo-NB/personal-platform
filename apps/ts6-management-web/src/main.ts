import { mountManagementApp } from "./app";
import { readRuntimeConfig } from "./runtime-config";

const root = document.querySelector<HTMLElement>("#app");
if (!root) {
  throw new Error("The application root is missing.");
}

try {
  const config = readRuntimeConfig();
  mountManagementApp(root, config.apiBaseUrl);
} catch (error: unknown) {
  root.innerHTML = `
    <section class="configuration-error" role="alert">
      <p class="section-label">Configuration fault</p>
      <h1>Console unavailable.</h1>
      <p>${error instanceof Error ? error.message : "The runtime configuration could not be read."}</p>
    </section>`;
}
