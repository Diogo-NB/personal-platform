# Shared VPS services

Shared services that run on the VPS and that other projects depend on, all in
the Dokploy project `platform`. Only secrets live in Dokploy's environment
settings.

| Service | Dokploy service type | Public address |
|---|---|---|
| PostgreSQL 17 | Dokploy Database (PostgreSQL) | None |
| Keycloak 26 | Compose, `infra/vps/keycloak/compose.yaml` | `https://auth.diogo-nb.com.br` |

DNS records live in [`PersonalPlatformDnsStack`](../aws/README.md#dns).

## Network

Shared services and projects meet on Dokploy's `dokploy-network`, the network
Traefik routes through. PostgreSQL is reachable on it by its internal host;
project containers that need it join `dokploy-network` as an external network
(most already do, because their domain requires it). Projects reach Keycloak
through its public hostname, because tokens carry it as `iss`.

Every container on the network resolves every other container's service name,
so services use unique names (`platform-keycloak`). Never
give a service a generic name such as `postgres` or `api`. Projects are
isolated from each other by Postgres roles, not by the network.

## PostgreSQL

A Dokploy Database service: image `postgres:17-alpine`, database user
`postgres` (the superuser), no external port, memory limit 1 GiB. Its
configuration lives in Dokploy, not in Git. Dokploy generates its internal
host (currently `platform-postgres-ufprvz`) and it cannot be renamed, so
clients take it from their Dokploy environment (`POSTGRES_HOST`, or the host
in `DATABASE_URL`) rather than from Git. If the service is ever recreated,
update those values.

Each project gets one login role and one database it owns, both named after
the project. Create them with the superuser and
[`postgres/create-project.sql`](postgres/create-project.sql), which prompts
for the role's password and sends it hashed. From this repository:

```sh
ssh vps 'cat > /tmp/create-project.sql' < infra/vps/postgres/create-project.sql
ssh -t vps 'c=$(sudo docker ps -qf name=platform-postgres) &&
  sudo docker cp /tmp/create-project.sql "$c":/tmp/create-project.sql &&
  sudo docker exec -it "$c" psql -U postgres -v project=<name> -f /tmp/create-project.sql'
```

Run it for `keycloak` before deploying Keycloak, and for each project before
its first deploy. The script removes the default `PUBLIC` access to the new
database, so one project's role cannot connect to another project's database.
Projects then run their own migrations with their own role.

Connection string for a project container on `dokploy-network`:

```
postgresql://<name>:<password>@<internal host>:5432/<name>
```

`max_connections` is the default 100 and is shared by every project. Keycloak
caps its pool at 20. Give each application an explicit pool size (Prisma:
`connection_limit` in the URL, e.g. `?connection_limit=10`) and keep the sum
well below 100.

Backups are not configured yet. Dokploy's database backups (scheduled
`pg_dump` to an S3 destination) are the planned approach; see the backlog in
[`docs/vps-platform.md`](../../docs/vps-platform.md#backlog).

## Keycloak

One instance, one realm per project. Realms, clients and users are created by
hand in the admin console; there are no realm files in production.

Environment in Dokploy: `POSTGRES_HOST` (the database's internal host),
`KC_DB_PASSWORD` (the `keycloak` role) and `KC_BOOTSTRAP_ADMIN_PASSWORD`. Keycloak creates the temporary bootstrap admin
only while the master realm has no admin.

The public route is set in Dokploy's Domains tab for the service: host
`auth.diogo-nb.com.br`, path `/`, service `platform-keycloak`, container port
8080, HTTPS with Let's Encrypt. The whole host is public, including the admin
console at `https://auth.diogo-nb.com.br/admin`, so the `master` realm's admin
accounts are protected only by their credentials. After the first deploy:

1. Sign in as the bootstrap admin, create a permanent admin user in `master`
   with OTP required, then delete the bootstrap admin.
2. In `master` > Realm settings > Security defenses, enable brute force
   detection.

### Adding a project realm

Create the realm and its clients in the admin console. The issuer is
`https://auth.diogo-nb.com.br/realms/<realm>`. For InvestFlow's realm and
clients, see [`docs/vps-platform.md`](../../docs/vps-platform.md#investflow).

## Memory

| Container | Limit | Notes |
|---|---|---|
| `platform-postgres` | 1 GiB | ~70 MiB idle locally |
| `platform-keycloak` | 768 MiB | heap capped at 384 MiB; ~490 MiB after startup locally |

## Dokploy notes

- Traefik's Let's Encrypt account email is Dokploy's placeholder
  (`test@localhost.com` in `/etc/dokploy/traefik/traefik.yml`). Set a real
  address in Dokploy before the first certificate is requested.
- The Dokploy panel is public at `dokploy.diogo-nb.com.br` so CI can call its
  deploy webhooks. Restricting it to the webhook path is in the backlog.
