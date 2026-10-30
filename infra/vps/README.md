# Shared VPS services

Shared services that run on the VPS and that other projects depend on. Each
directory is one Dokploy **Compose** service in the Dokploy project
`platform`, reading its `compose.yaml` from this repository. Only secrets live
in Dokploy's environment settings.

| Service | Compose path | Public address |
|---|---|---|
| PostgreSQL 17 | `infra/vps/postgres/compose.yaml` | None |
| Keycloak 26 | `infra/vps/keycloak/compose.yaml` | `https://auth.diogo-nb.com.br` (`/realms/`, `/resources/` only) |

DNS records live in [`PersonalPlatformDnsStack`](../aws/README.md#dns).

## Network

Projects reach Postgres over one external Docker network, `platform-internal`.
Only the shared services and the project containers that need Postgres join
it (for InvestFlow: `api` and `migrate`, not `web`). Projects reach Keycloak
through its public hostname, because tokens carry it as `iss`.

Every container on the network resolves every other container's service name,
so shared services use unique names (`platform-postgres`,
`platform-keycloak`). Never give a service on this network a generic name such
as `postgres` or `api`. Projects are isolated from each other by Postgres
roles, not by the network.

The Postgres compose file creates the network on its first deploy; every other
compose file joins it as `external`. Deploy Postgres before anything that uses
the network.

## PostgreSQL

Secrets in Dokploy: `POSTGRES_PASSWORD` (the `postgres` superuser).

Each project gets one login role and one database it owns, both named after
the project. Create them with the superuser; the script prompts for the
role's password and sends it hashed:

```sh
sudo docker exec -it "$(sudo docker ps -qf name=platform-postgres)" \
  psql -U postgres -v project=<name> -f /opt/platform/create-project.sql
```

Run it for `keycloak` before deploying Keycloak, and for each project before
its first deploy. The script removes the default `PUBLIC` access to the new
database, so one project's role cannot connect to another project's database.
Projects then run their own migrations with their own role.

Connection string for a project container on `platform-internal`:

```
postgresql://<name>:<password>@platform-postgres:5432/<name>
```

`max_connections` is the default 100 and is shared by every project. Keycloak
caps its pool at 20. Give each application an explicit pool size (Prisma:
`connection_limit` in the URL, e.g. `?connection_limit=10`) and keep the sum
well below 100.

There are no backups yet; see the backlog in
[`docs/vps-platform.md`](../../docs/vps-platform.md#backlog).

## Keycloak

One instance, one realm per project. Realms, clients and users are created by
hand in the admin console; there are no realm files in production.

Secrets in Dokploy: `KC_DB_PASSWORD` (the `keycloak` role) and
`KC_BOOTSTRAP_ADMIN_PASSWORD`. Keycloak creates the temporary bootstrap admin
only while the master realm has no admin.

Traefik routes only `/realms/` and `/resources/` publicly. The admin console
is published on the VPS loopback only:

```sh
ssh -N -L 8080:localhost:8080 vps
# then open http://localhost:8080/admin
```

The local port must be 8080, because `KC_HOSTNAME_ADMIN` is
`http://localhost:8080`; stop anything else listening on 8080 locally first.
On the VPS, port 8080 is also where Traefik's dashboard would be published:
leave Dokploy's Traefik dashboard option off.

The master realm's login page stays public. The admin console signs in through
`https://auth.diogo-nb.com.br/realms/master/...` even when opened through the
tunnel (verified in a local test with `/admin` blocked), so blocking
`/realms/master` would break admin sign-in. After the first deploy:

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
