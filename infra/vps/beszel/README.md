# Beszel monitoring

Prepared for the VPS; deployment and verification are manual in Dokploy.
The hub stores the dashboard, accounts, and metric history in its own SQLite
database. It does not need the shared PostgreSQL instance or Keycloak.

Run one Dokploy **Compose** service named `beszel` in `platform` / `production`,
reading `infra/vps/beszel/compose.yaml` from this repository. It contains the
hub and agent. The hub starts by default; the agent is behind the `agent`
Compose profile so the hub can generate its credentials first.

| Container service | Initial memory limit |
|---|---|
| `platform-beszel` | 256 MiB |
| `platform-beszel-agent` | 64 MiB |

These limits are starting budgets, not measured consumption. Images are pinned
to 0.21.1; no images are built on the VPS.

## Deploy the hub

1. Create the `beszel` Compose service with this repository as its Git source
   and `infra/vps/beszel/compose.yaml` as its Compose path.
2. Use Compose deployment mode, not Swarm Stack. Leave Domains empty and
   disable any automatic domain or Traefik route. Do not enable isolated
   deployments that rewrite the loopback port binding.
3. Leave `COMPOSE_PROFILES` unset in Dokploy's environment settings for the
   first deployment. Deploy: only the hub starts, with no agent credentials
   needed. It creates its data volume and the socket volume.
4. On your workstation, open an SSH tunnel:

   ```sh
   ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:8090:127.0.0.1:8090 vps
   ```

5. Open `http://localhost:8090` and create the initial administrator account.
   Keep the tunnel running while using the dashboard.

No DNS record, certificate, or Hostinger firewall change is needed. The hub
uses its Compose project's default bridge network and does not need
`dokploy-network`.

## Register the VPS and deploy the agent

1. In Beszel, open **Add System**. Name it `Hostinger VPS` and use
   `/beszel_socket/beszel.sock` as **Host / IP**. If a port field is shown,
   leave its default; the socket path selects the local connection.
2. Copy the public key and token from the generated agent configuration.
   Finish saving the system.
3. In the existing `beszel` service's Dokploy environment settings, set:

   ```dotenv
   COMPOSE_PROFILES=agent
   BESZEL_AGENT_KEY='ssh-ed25519 <public key from Beszel>'
   BESZEL_AGENT_TOKEN='<token from Beszel>'
   ```

   Paste the complete generated key, including its `ssh-ed25519` prefix and
   any trailing comment. Keep the token in Dokploy, out of Git and logs.
4. Redeploy the same service. Both containers now start. Keep
   `COMPOSE_PROFILES=agent` for subsequent deployments. Keep Domains empty.
   Disable automatic network injection for the agent if Dokploy offers it:
   `network_mode: host` cannot
   be combined with `networks`.

The agent uses host networking for the VPS's network-interface statistics.
Its listener is a Unix socket in the volume shared with the hub, so it does
not listen on TCP 45876. `HUB_URL` also lets the agent initiate a WebSocket
connection through the hub's loopback port.

Only the agent mounts Docker's socket for container metrics. The `:ro` mount
does not restrict Docker API operations; the agent is a trusted host component.
Do not expose the Docker socket over TCP or mount it in the hub.

## Verify and configure alerts

- Check that both containers become healthy in Dokploy. The agent healthcheck
  confirms its process is running, not that it has connected to the hub.
- Check that `Hostinger VPS` becomes connected and that host CPU, memory,
  disk, and network charts populate. Compare the reported disk size with the
  VPS's approximately 96 GB disk; see the upstream disk guide if detection
  needs a `FILESYSTEM` override.
- Confirm the container list includes Keycloak, the shared PostgreSQL,
  InvestFlow, and Dokploy's containers that are currently running.
- Redeploy the hub and confirm the administrator account and metric history
  remain and the agent reconnects.
- Confirm the published hub binding remains `127.0.0.1:8090`, and there is no
  public domain route or TCP listener for the agent.
- Configure a notification destination in Beszel and send its test message.
  Set initial alerts for host memory above 85%, disk above 80%, and CPU above
  90% for five minutes where the alert supports a duration. Tune thresholds
  after observing normal load.

The hub runs on the monitored VPS, so it cannot notify you while that VPS is
down. Use Beszel's outbound heartbeat with an external monitoring service if
you need whole-host outage alerts. This setup does not promise a dedicated
CPU steal-time chart; verify available metrics before relying on one.

## Persistence, recovery, and upgrades

The hub's `hub-data` volume holds accounts, history, settings, and its private
key. The agent's `agent-data` volume preserves its identity. Dokploy prefixes
all three volume names with the Compose project's name; preserve the project
identity when redeploying. The socket volume is shared by both containers
within the same Compose project.

PostgreSQL backups do not cover Beszel. Configure backups of the hub's data
before relying on its history. For a manual filesystem backup, stop the hub
before copying its data volume and restart it afterward so the SQLite files
are consistent. Keep copies off the VPS and protect them as secrets. Beszel
history backups are not configured by these Compose files.

For recovery, restore the hub data volume before starting the hub, then start
the agent with its existing key and token. If hub data is lost, create a new
administrator and system and replace the agent's key and token. Do not delete
data volumes or use `docker compose down -v` during routine redeployments.

For upgrades, review upstream release notes, back up hub data, and change both
image pins together. Recheck health, connectivity, charts, and notifications.
An older image may not support a migrated database; restore the pre-upgrade
backup if rollback requires it.

## References

- [Getting started and local Unix socket setup](https://beszel.dev/guide/getting-started)
- [Compose profiles](https://docs.docker.com/compose/how-tos/profiles/)
- [Agent configuration](https://beszel.dev/guide/agent-installation)
- [Healthchecks](https://beszel.dev/guide/healthchecks)
- [Root disk detection](https://beszel.dev/guide/additional-disks)
- [Environment variables and outbound heartbeat](https://beszel.dev/guide/environment-variables)
- [Release notes](https://github.com/henrygd/beszel/releases)
