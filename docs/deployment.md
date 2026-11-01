# Deploying torq to a VPS

Every push to `main` that passes CI is built into an image, published to the GitHub Container Registry
(GHCR) and rolled out on your server automatically:

```
git push ─▶ GitHub Actions: lint, tests, container + e2e checks
              └▶ publish: multi-arch image → ghcr.io/<owner>/torq:sha-<commit> and :latest
                   └▶ deploy: SSH to the VPS → docker compose pull && up -d → wait until healthy
browser ─▶ https://torq.<your-domain> ─▶ Cloudflare ─▶ tunnel (cloudflared on the VPS) ─▶ torq
```

The VPS never builds anything and needs no open web ports. Cloudflare Tunnel connects outward, so only SSH
is reachable from the internet. Images are pinned to the commit they were built from, so rolling back is
one command (see [Rolling back](#rolling-back)).

## 1. Create the server

Any small Linux VPS with Docker works. torq uses about 30 MB of RAM (up to ~200 MB during an online
catalog update) and a few hundred MB of disk.

**Recommended:** Hetzner Cloud **CAX11** (2 ARM vCPU, 4 GB RAM, 40 GB disk, about €4/month), in
Falkenstein or Nuremberg (data stays in the EU). The image is built for both `arm64` and `amd64`, so an
x86 plan such as CX22 works too.

- Image: **Ubuntu 24.04**, with your personal SSH key.
- Enable **Backups** (about 20% of the server price). They are daily snapshots of the whole disk, which
  includes the torq database and uploads.
- Firewall: allow **inbound TCP 22 only**. Ports 80/443 are not needed.

## 2. Prepare the server

As `root` on the VPS:

```sh
apt update && apt upgrade -y
apt install -y unattended-upgrades          # automatic security updates
curl -fsSL https://get.docker.com | sh      # Docker Engine + Compose plugin

# Account that GitHub Actions deploys with. Membership in the docker group is
# root-equivalent, so its key is only stored as a GitHub secret.
adduser --disabled-password --gecos "" deploy
usermod -aG docker deploy
install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
install -d -o deploy -g deploy /opt/torq
```

On your own machine, create a key pair used only for deployments and authorize it:

```sh
ssh-keygen -t ed25519 -N "" -C "torq deploy" -f torq-deploy
ssh root@<server-ip> "cat >> /home/deploy/.ssh/authorized_keys && chown deploy:deploy /home/deploy/.ssh/authorized_keys && chmod 600 /home/deploy/.ssh/authorized_keys" < torq-deploy.pub
```

## 3. Create the Cloudflare Tunnel

Your domain must use Cloudflare DNS (free plan).

1. Cloudflare dashboard → **Zero Trust** → **Networks** → **Tunnels** → **Create a tunnel** →
   *Cloudflared* → name it `torq`.
2. Copy the **token** shown in the install command (the long string after `--token`). You don't need to
   run the install command; the compose file starts cloudflared.
3. **Public hostname**: subdomain `torq`, your domain, service type **HTTP**, URL **`torq:8080`**.

Optional: put **Zero Trust → Access → Applications** in front of the hostname (free for up to 50 users) to
require an e-mail one-time PIN before anyone even reaches the torq login page.

## 4. Configure torq on the server

Copy the template and fill in your URL and the tunnel token:

```sh
scp deploy/env.example deploy@<server-ip>:/opt/torq/torq.env
ssh deploy@<server-ip> "chmod 600 /opt/torq/torq.env && nano /opt/torq/torq.env"
```

`torq.env` holds every setting from the README's *Configuration* table. It stays on the server; deployments
only replace `docker-compose.yml` and `.env` (the pinned image) next to it.

## 5. Connect GitHub

In the repository: **Settings → Secrets and variables → Actions**.

| Kind | Name | Value |
|---|---|---|
| Variable | `DEPLOY_HOST` | Server IP or host name. Deployments stay off until this is set. |
| Variable | `DEPLOY_USER` | `deploy` (the default when unset) |
| Secret | `DEPLOY_SSH_KEY` | Contents of the private key file `torq-deploy` |
| Secret | `DEPLOY_KNOWN_HOSTS` | Output of `ssh-keyscan -t ed25519 <server-ip>` |

Check the `ssh-keyscan` fingerprint against the one in your provider's console (or `ssh-keygen -lf
/etc/ssh/ssh_host_ed25519_key.pub` on the server). Then delete the private key file from your machine.

The deploy job runs in the GitHub environment **production**, which GitHub creates on the first run. Under
**Settings → Environments → production** you can add required reviewers if you want to approve each
rollout.

The VPS pulls the image with the workflow's short-lived token and logs out afterwards, so the image can
stay private and no registry credential is stored on the server.

## 6. First deployment

Push to `main` (or re-run the latest CI run). When the **deploy** job is green, open
`https://torq.<your-domain>` and create the first administrator account. Do this right away: until an
admin exists, anyone who reaches the URL can create it.

## Day to day

```sh
ssh deploy@<server-ip>
cd /opt/torq
docker compose ps                 # status and health
docker compose logs -f torq       # application logs
docker compose restart torq
```

### Rolling back

Every image is tagged with its commit. To run an earlier one:

```sh
cd /opt/torq
echo "TORQ_IMAGE=ghcr.io/<owner>/torq:sha-<commit>" > .env
docker compose up -d
```

Schema migrations only move forward. Before each upgrade, torq copies the database to
`/data/backups/`. If an older version refuses a newer database, restore the copy taken before the upgrade
(or a Hetzner backup).

### Backups

- Hetzner **Backups** keep 7 daily disk snapshots. Restoring the server restores torq.
- For an extra copy, download the data volume now and then:
  `ssh deploy@<server-ip> "docker run --rm -v torq_torq-data:/data:ro alpine tar czf - -C /data ." > torq-backup.tgz`
  (stop torq first, `docker compose stop torq`, for a guaranteed consistent database copy).
