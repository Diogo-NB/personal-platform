# TeamSpeak 6 on the VPS

The VPS server is the permanent TeamSpeak instance. The operator confirmed the
migration and live operation succeeded. S3 backups and restore testing still
require verification.

The AWS server and management applications are retired, and the operator
confirmed the old CloudFormation stack was deleted. Cleanup of retained EFS
storage and old DNS names still requires verification.

## Deployment

Create a Dokploy **Compose** service named `teamspeak6` in
`personal` / `production`, using this repository as its Git source and
`infra/vps/teamspeak/compose.yaml` as its Compose path.

The service pulls the official `6.0.0-beta13.1` image for `linux/amd64`.
Configuration lives in Compose environment variables; there is no Dockerfile,
image build, GHCR publishing, or CI deployment workflow for this service.
Keep automatic deployments disabled so pushes cannot interrupt an active session.
Deploy manually after reviewing the selected Git revision.

1. In Hostinger's panel firewall, allow inbound `9987/UDP` for voice and
   `30033/TCP` for file transfer. Check these host ports are free first.
   Do not open query ports, Dokploy's internal port, or Swarm ports.
2. Select Compose mode, not Swarm Stack. Leave Domains empty and disable
   automatic domain/Traefik routes. Preserve the published ports; do not enable
   isolated deployments that rewrite them. This server does not require
   `dokploy-network`, PostgreSQL, or Keycloak.
3. No Dokploy environment variables are required. License acceptance is explicit
   in Compose; read the license bundled with the pinned image before deployment.
4. Deploy. Open the TeamSpeak container logs privately, retrieve the initial
   administrator privilege key, and claim it using your client. Store it in a
   password manager; do not paste startup logs or keys into Git or issues.
5. Connect to `teamspeak.diogo-nb.com.br` on voice port `9987` after the DNS
   deployment below. Set a server password
   and review channel, group, upload, and download permissions in TeamSpeak.

Traffic goes directly to the published ports. There is no HTTP route or
Let's Encrypt certificate for this service. Keep Dokploy's Domains tab empty
even when using the custom hostname.

## DNS

`PersonalPlatformDnsStack` declares `teamspeak.diogo-nb.com.br` as an IPv4 alias
to `vps.diogo-nb.com.br`. After confirming the AWS account, `sa-east-1`, cost,
and blast radius, the operator reviews and deploys only the DNS stack:

```sh
cd infra/aws
cdk diff PersonalPlatformDnsStack
cdk deploy PersonalPlatformDnsStack
dig A teamspeak.diogo-nb.com.br +short
```

The VPS alias has been deployed. Keep it and the shared hosted zone in place.
Future diffs must preserve existing service records unless a change is intended.
The old `ts.diogo-nb.com.br` record belongs to the retired AWS service and is
removed separately. If troubleshooting this alias, connect directly to
`vps.diogo-nb.com.br` on the same voice port.

## Storage and resources

Embedded SQLite and local files use the named volume `teamspeak6-data`, mounted
at `/var/tsserver`. Logs and crash dumps also live there. S3 file transfer,
ServerQuery SSH/HTTP/HTTPS, and Prometheus are disabled.

Dokploy prefixes the volume with its Compose project name. Record the actual
volume name after deployment and preserve that project identity on redeploys.
Never run two containers against the same SQLite volume or delete it during a
routine update. Do not use `docker compose down -v`.

The initial memory limit is 256 MiB; verify it with connected users and file
transfers. Monitor memory, OOM restarts, disk use, and latency in Dokploy or
Beszel. A 120-second stop grace period allows orderly shutdown. Docker stdout
logs are limited to three 10 MiB files; application logs and crash dumps inside
the data volume need separate disk-use monitoring.

## Backups

Backups require setup in Dokploy and S3; the Compose file alone does not create
them. Use Dokploy's **Volume Backups** for the entire named volume, rather than
the database backup feature or a raw copy of a running SQLite database.

Configure an S3 destination backed by a private encrypted bucket, using
credentials scoped to the TeamSpeak backup location. Keep credentials in
Dokploy's destination settings, not the TeamSpeak container or repository.
Review bucket permissions, retention, and ongoing storage charges before creation.
Do not change the existing manually managed `skycrate-storage` bucket's policies
or lifecycle rules as part of this deployment.

Create the volume backup with:

| Setting | Value |
|---|---|
| Service | `personal-teamspeak6` |
| Volume | Select the actual Dokploy-prefixed `teamspeak6-data` volume |
| Destination | The configured private S3 destination |
| Prefix | `teamspeak6/` |
| Schedule | Daily during unused hours; verify the scheduler timezone in Dokploy |
| Turn off Container | Enabled |
| Enabled | Yes |

The server is unavailable during backup. Confirm it stops gracefully and
restarts afterward. Retain daily archives for 30 days using an S3 lifecycle
rule limited to this backup prefix. If bucket versioning is enabled, configure
noncurrent-version expiry too. Store pre-upgrade archives separately if they
must outlive that retention window.

Run a manual backup and verify it reached S3. Test a restore before depending on
the schedule. Daily backups allow up to one day of data loss; they preserve only
what TeamSpeak actually writes to the volume, so verify chat retention explicitly.

## Verification and recovery

After the first deployment:

- Test voice with the expected users and upload/download with permitted groups.
- Create a channel, redeploy, and confirm the channel and permissions survive.
- Confirm query ports are not published and the memory limit is applied.
- Confirm a scheduled backup completes and the server returns afterward.

For a restore test, stop the service, record its image version and actual volume
name, and retain the current volume. Restore an archive through Dokploy into a
new, unused volume. Point the Compose service at that restored volume using an
explicit external volume mapping, review Dokploy's rendered Compose, and then
start it with the matching image version. Do not restore over the running volume.
Verify channels, permissions, files, and the administrator identity. Record the
tested restore mapping and backup time privately for incident recovery.

Keep Compose configuration and its Git revision alongside the backup record;
the image version and environment are not part of the data volume. Restoring an
older backup discards changes made after that backup.

## Updates

1. Read the proposed image's release notes, configuration changes, and license.
2. Inform connected users, run a manual backup with the container stopped, and
   confirm the archive reached S3. Record the current image and Git revision.
3. Change only the version in this directory's `compose.yaml`; use an explicit
   release tag, never `latest`. Validate locally:

   ```sh
   docker compose -f infra/vps/teamspeak/compose.yaml config --quiet
   ```

4. Commit and push through the normal review workflow. In the existing Dokploy
   service, deploy the reviewed revision. Dokploy pulls the new image and
   replaces the container while keeping its named volume. Expect downtime.
5. Verify voice, files, permissions, persistence, logs, and memory usage.

An older image may not understand an upgraded database. If rollback requires
restoring data, use the pre-upgrade archive with the previous image version.
Changing the image tag alone does not undo database migrations. Never create a
second deployment sharing this volume to attempt a rolling update.

## References

- [Official server repository and license](https://github.com/teamspeak/teamspeak6-server)
- [Configuration reference](https://github.com/teamspeak/teamspeak6-server/blob/main/CONFIG.md)
- [Dokploy Compose](https://docs.dokploy.com/docs/core/docker-compose)
- [Dokploy volume backups and restores](https://docs.dokploy.com/docs/core/volume-backups)
- [SQLite backup guidance](https://www.sqlite.org/backup.html)
