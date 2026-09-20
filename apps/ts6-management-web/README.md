# TeamSpeak 6 management web app

This framework-free TypeScript SPA is the browser control panel for the
TeamSpeak management API. It is an independent application from the Go service
in [`../ts6-management-api`](../ts6-management-api/README.md).

## Behavior and security

The app validates a password-style API-key entry with `GET /status`, then polls
the status every two seconds while the tab is visible. Polling pauses while
hidden, refreshes immediately when visible again, and never overlaps. Start and
stop send empty `POST` requests and remain disabled when the lifecycle state or
an in-flight command makes them invalid. Running status timestamps are rendered
in the `America/Sao_Paulo` time zone and explicitly labeled as Brasília time.

The browser sends the key through `X-Api-Key` without cookies or browser
credentials. The key exists only in JavaScript memory: the input is cleared
after validation, and the value is never written to storage, URLs, logs, build
output, or runtime configuration. Closing the page or choosing **Forget API
key** clears the in-memory reference.

## Runtime configuration and hosting

Local Vite development reads `VITE_API_BASE_URL` from `.env`. Copy the provided
example before starting the dev server:

```bash
cp .env.example .env
```

The `.env` file is ignored by Git. Never add the API key to a `VITE_` variable:
Vite embeds those variables in browser code. CDK replaces `runtime-config.js`
during deployment with a file containing only the API Gateway base URL, and
that deployment configuration takes precedence over the build-time value.

CDK deploys `dist` to a private, encrypted, public-access-blocked S3 bucket.
CloudFront reads the bucket through Origin Access Control, redirects HTTP to
HTTPS, and serves only static files. The browser calls API Gateway directly.

## Local development

```bash
npm ci
npm test
npm run build
npm run dev
```

Build this application before synthesizing or deploying the TeamSpeak stack.
Deployment and API-key retrieval are private user operations documented in
[`../ts6/README.md`](../ts6/README.md).
