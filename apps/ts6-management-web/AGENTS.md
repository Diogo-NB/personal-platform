# TeamSpeak management web guidance

## Sources of truth

Read `README.md` and `DESIGN.md` completely before reviewing, changing,
deploying, or operating this application. Also follow `../ts6/AGENTS.md` for
TeamSpeak architecture, deployment safety, data protection, and AWS
restrictions.

## Fixed decisions

- Keep this application independent from the management API module and Docker
  image.
- Use vanilla TypeScript, HTML, CSS, Vite, and Vitest without a UI framework or
  client-side router.
- Read the deployed API base URL from deployment-generated runtime
  configuration. Local Vite development may use `VITE_API_BASE_URL` from
  `.env`; deployment configuration takes precedence.
- Keep the API key in memory only. Never write it to storage, URLs, logs,
  source files, `.env`, build output, or runtime configuration.
- Call API Gateway directly with `X-Api-Key`; do not proxy API requests through
  CloudFront and do not send browser credentials.
- Preserve the polling, visibility, request-overlap, and disabled-control
  behavior documented in `README.md`.
- Treat `DESIGN.md` as the visual source of truth and maintain WCAG 2.2 AA.

## AWS safety

Agents may run local tests, builds, and offline CDK synthesis. Only the user
runs commands that contact AWS, including CDK diff, deploy, destroy, and
resource queries.

## Required verification

Run from this directory:

```bash
npm ci
npm test
npm run build
```
