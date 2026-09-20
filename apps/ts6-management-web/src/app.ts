import { APIError, ManagementAPI, serviceStates } from "./api";
import type { LifecycleAction, ServiceState, ServiceStatus } from "./api";

const pollIntervalMilliseconds = 2_000;
const brasiliaTimeZone = "America/Sao_Paulo";

export interface AppOptions {
  fetchRequest?: typeof fetch;
  pollInterval?: number;
}

export function mountManagementApp(
  root: HTMLElement,
  apiBaseURL: string,
  options: AppOptions = {},
): () => void {
  root.innerHTML = applicationMarkup();
  const controller = new AppController(
    root,
    new ManagementAPI(apiBaseURL, options.fetchRequest ?? window.fetch.bind(window)),
    options.pollInterval ?? pollIntervalMilliseconds,
  );
  controller.start();

  return () => controller.destroy();
}

class AppController {
  private apiKey: string | null = null;
  private status: ServiceStatus | null = null;
  private pollPromise: Promise<void> | null = null;
  private pollTimer: number | null = null;
  private actionPending = false;
  private destroyed = false;

  private readonly accessPanel: HTMLElement;
  private readonly accessForm: HTMLFormElement;
  private readonly apiKeyInput: HTMLInputElement;
  private readonly accessButton: HTMLButtonElement;
  private readonly consolePanel: HTMLElement;
  private readonly stateLabel: HTMLElement;
  private readonly stateDetail: HTMLElement;
  private readonly startedAt: HTMLElement;
  private readonly uptime: HTMLElement;
  private readonly startButton: HTMLButtonElement;
  private readonly stopButton: HTMLButtonElement;
  private readonly forgetButton: HTMLButtonElement;
  private readonly feedback: HTMLElement;
  private readonly visibilityHandler = () => this.handleVisibilityChange();

  constructor(
    private readonly root: HTMLElement,
    private readonly api: ManagementAPI,
    private readonly pollInterval: number,
  ) {
    this.accessPanel = element(root, "access-panel");
    this.accessForm = element(root, "access-form");
    this.apiKeyInput = element(root, "api-key");
    this.accessButton = element(root, "access-button");
    this.consolePanel = element(root, "console-panel");
    this.stateLabel = element(root, "state-label");
    this.stateDetail = element(root, "state-detail");
    this.startedAt = element(root, "started-at");
    this.uptime = element(root, "uptime");
    this.startButton = element(root, "start-button");
    this.stopButton = element(root, "stop-button");
    this.forgetButton = element(root, "forget-button");
    this.feedback = element(root, "feedback");
  }

  start(): void {
    this.accessForm.addEventListener("submit", this.authenticate);
    this.startButton.addEventListener("click", this.startService);
    this.stopButton.addEventListener("click", this.stopService);
    this.forgetButton.addEventListener("click", this.forgetKey);
    document.addEventListener("visibilitychange", this.visibilityHandler);
  }

  destroy(): void {
    this.destroyed = true;
    this.stopPolling();
    this.apiKey = null;
    this.accessForm.removeEventListener("submit", this.authenticate);
    this.startButton.removeEventListener("click", this.startService);
    this.stopButton.removeEventListener("click", this.stopService);
    this.forgetButton.removeEventListener("click", this.forgetKey);
    document.removeEventListener("visibilitychange", this.visibilityHandler);
    this.root.replaceChildren();
  }

  private readonly authenticate = (event: SubmitEvent): void => {
    event.preventDefault();
    const candidate = this.apiKeyInput.value.trim();
    if (!candidate) {
      this.apiKeyInput.setAttribute("aria-invalid", "true");
      this.showFeedback("Enter an API key to continue.", true);
      this.apiKeyInput.focus();
      return;
    }

    this.setAuthenticationPending(true);
    this.apiKeyInput.setAttribute("aria-invalid", "false");
    this.clearFeedback();
    void this.api
      .status(candidate)
      .then((status) => {
        if (this.destroyed) {
          return;
        }
        this.apiKey = candidate;
        this.apiKeyInput.value = "";
        this.status = status;
        this.accessPanel.hidden = true;
        this.consolePanel.hidden = false;
        this.renderStatus();
        this.startPolling();
        this.stateLabel.focus();
      })
      .catch((error: unknown) => {
        this.apiKeyInput.setAttribute("aria-invalid", "true");
        this.showFeedback(errorMessage(error), true);
        this.apiKeyInput.focus();
      })
      .finally(() => this.setAuthenticationPending(false));
  };

  private readonly startService = (): void => {
    void this.runCommand("start");
  };

  private readonly stopService = (): void => {
    void this.runCommand("stop");
  };

  private readonly forgetKey = (): void => {
    this.stopPolling();
    this.apiKey = null;
    this.status = null;
    this.actionPending = false;
    this.consolePanel.hidden = true;
    this.accessPanel.hidden = false;
    this.clearFeedback();
    this.apiKeyInput.focus();
  };

  private async runCommand(action: LifecycleAction): Promise<void> {
    if (this.actionPending || !this.apiKey) {
      return;
    }

    this.actionPending = true;
    this.clearFeedback();
    this.renderControls();
    try {
      await this.api.command(action, this.apiKey);
      await this.refreshStatus();
      this.showFeedback(
        action === "start" ? "Start request accepted." : "Stop request accepted.",
        false,
      );
    } catch (error: unknown) {
      this.showFeedback(errorMessage(error), true);
    } finally {
      this.actionPending = false;
      this.renderControls();
    }
  }

  private startPolling(): void {
    if (this.pollTimer !== null || document.hidden || !this.apiKey) {
      return;
    }
    this.pollTimer = window.setInterval(() => void this.refreshStatus(), this.pollInterval);
  }

  private stopPolling(): void {
    if (this.pollTimer !== null) {
      window.clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  private handleVisibilityChange(): void {
    if (document.hidden) {
      this.stopPolling();
      return;
    }
    if (this.apiKey) {
      void this.refreshStatus();
      this.startPolling();
    }
  }

  private refreshStatus(): Promise<void> {
    if (!this.apiKey) {
      return Promise.resolve();
    }
    if (this.pollPromise) {
      return this.pollPromise;
    }

    const apiKey = this.apiKey;
    this.pollPromise = this.api
      .status(apiKey)
      .then((status) => {
        if (this.destroyed || this.apiKey !== apiKey) {
          return;
        }
        this.status = status;
        this.renderStatus();
        if (!this.actionPending) {
          this.clearFeedback();
        }
      })
      .catch((error: unknown) => {
        if (this.apiKey === apiKey) {
          this.status = null;
          this.renderStatus();
          this.showFeedback(errorMessage(error), true);
        }
      })
      .finally(() => {
        this.pollPromise = null;
      });

    return this.pollPromise;
  }

  private renderStatus(): void {
    const state = this.status?.state ?? null;
    this.stateLabel.textContent = state ?? "unavailable";
    this.stateLabel.dataset.state = state ?? "unavailable";
    this.stateDetail.textContent = stateDescription(state);
    this.startedAt.textContent = formatStartedAt(this.status?.startedAt ?? null);
    this.uptime.textContent = formatUptime(this.status?.uptimeSeconds ?? null);

    for (const stage of this.root.querySelectorAll<HTMLElement>("[data-service-state]")) {
      const isCurrent = stage.dataset.serviceState === state;
      stage.classList.toggle("is-current", isCurrent);
      if (isCurrent) {
        stage.setAttribute("aria-current", "step");
      } else {
        stage.removeAttribute("aria-current");
      }
    }
    this.renderControls();
  }

  private renderControls(): void {
    const state = this.status?.state;
    this.startButton.disabled = this.actionPending || state !== "stopped";
    this.stopButton.disabled = this.actionPending || (state !== "running" && state !== "starting");
    this.startButton.setAttribute("aria-busy", String(this.actionPending));
    this.stopButton.setAttribute("aria-busy", String(this.actionPending));
  }

  private setAuthenticationPending(pending: boolean): void {
    this.apiKeyInput.disabled = pending;
    this.accessButton.disabled = pending;
    this.accessButton.textContent = pending ? "Checking…" : "Open console";
  }

  private showFeedback(message: string, isError: boolean): void {
    this.feedback.hidden = false;
    this.feedback.textContent = message;
    this.feedback.dataset.kind = isError ? "error" : "success";
  }

  private clearFeedback(): void {
    this.feedback.hidden = true;
    this.feedback.textContent = "";
    delete this.feedback.dataset.kind;
  }
}

function applicationMarkup(): string {
  const stages = serviceStates
    .map(
      (state) => `
        <li data-service-state="${state}">
          <span class="stage-mark" aria-hidden="true"></span>
          <span>${state}</span>
        </li>`,
    )
    .join("");

  return `
    <div class="app-shell">
      <header class="masthead">
        <a class="brand" href="/" aria-label="TeamSpeak control home">
          <span class="brand-mark" aria-hidden="true">TS</span>
          <span>TeamSpeak <strong>Control</strong></span>
        </a>
        <span class="environment">production / sa-east-1</span>
      </header>

      <section class="access-panel" data-testid="access-panel">
        <div class="access-intro">
          <p class="section-label">Private control plane</p>
          <h1>Identify to operate.</h1>
          <p>The key is validated once, kept only in this tab's memory, and cleared when the page closes.</p>
        </div>
        <form class="access-form" data-testid="access-form" novalidate>
          <label for="api-key">API key</label>
          <input
            id="api-key"
            data-testid="api-key"
            name="api-key"
            type="password"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            aria-describedby="api-key-note feedback"
            required
          />
          <p class="field-note" id="api-key-note">Sent directly to API Gateway over HTTPS. Never stored.</p>
          <button class="button button-primary" data-testid="access-button" type="submit">Open console</button>
        </form>
      </section>

      <section class="console-panel" data-testid="console-panel" hidden>
        <div class="status-main">
          <p class="section-label">Service condition</p>
          <div class="state-line">
            <h1 data-testid="state-label" data-state="unknown" tabindex="-1">unknown</h1>
            <span class="live-label">live telemetry</span>
          </div>
          <p class="state-detail" data-testid="state-detail">Waiting for status.</p>

          <dl class="telemetry">
            <div>
              <dt>Started at</dt>
              <dd data-testid="started-at">—</dd>
            </div>
            <div>
              <dt>Uptime</dt>
              <dd data-testid="uptime">—</dd>
            </div>
            <div>
              <dt>Refresh</dt>
              <dd>2 seconds</dd>
            </div>
          </dl>
        </div>

        <aside class="state-track" aria-label="Lifecycle states">
          <p class="section-label">Lifecycle</p>
          <ol>${stages}</ol>
        </aside>

        <div class="command-deck">
          <div>
            <p class="section-label">Commands</p>
            <p>Requests reconcile the singleton service. Only one command can run at a time.</p>
          </div>
          <div class="command-buttons">
            <button class="button button-primary" data-testid="start-button" type="button" disabled>Start server</button>
            <button class="button button-secondary" data-testid="stop-button" type="button" disabled>Stop server</button>
          </div>
        </div>

        <button class="forget-button" data-testid="forget-button" type="button">Forget API key</button>
      </section>

      <p class="feedback" data-testid="feedback" id="feedback" role="status" aria-live="polite" hidden></p>

      <footer>
        <span>TS6 / singleton node</span>
        <span>browser → API Gateway</span>
      </footer>
    </div>`;
}

function element<T extends HTMLElement>(root: HTMLElement, testID: string): T {
  const found = root.querySelector<T>(`[data-testid="${testID}"]`);
  if (!found) {
    throw new Error(`Missing application element: ${testID}`);
  }
  return found;
}

function stateDescription(state: ServiceState | null): string {
  switch (state) {
    case "stopped":
      return "The TeamSpeak task is offline and not consuming Fargate capacity.";
    case "starting":
      return "AWS accepted the start request. The task is moving toward ready.";
    case "running":
      return "The TeamSpeak task is online and reporting as running.";
    case "stopping":
      return "AWS accepted the stop request. The task is draining.";
    default:
      return "Current service status is unavailable.";
  }
}

function formatStartedAt(value: string | null): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) {
    return "—";
  }
  const formatted = new Intl.DateTimeFormat("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
    timeZone: brasiliaTimeZone,
  }).format(date);
  return `${formatted} (Brasília time)`;
}

function formatUptime(value: number | null): string {
  if (value === null) {
    return "—";
  }
  const seconds = Math.max(0, Math.floor(value));
  const hours = Math.floor(seconds / 3_600);
  const minutes = Math.floor((seconds % 3_600) / 60);
  return `${hours}h ${minutes}m`;
}

function errorMessage(error: unknown): string {
  if (error instanceof APIError) {
    return error.message;
  }
  return "The request could not be completed.";
}
