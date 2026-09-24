# WOL-Secure-lightSwitch (SecureSwitch)

SecureSwitch is a web app for Wake-on-LAN and optional SSH shutdown. It shows device availability, supports an initial administrator and lets that administrator assign devices to other users.

## Install with Docker Compose

You need a Linux Docker host with Docker Compose and an HTTPS reverse proxy. SecureSwitch serves HTTP on `127.0.0.1:7500` by default; browsers must use the proxy's HTTPS address because authentication uses a Secure cookie. Host networking lets the app send WOL broadcasts.

1. Copy [docker-compose.yml](docker-compose.yml) to your server. Set its `image:` to the immutable digest of an image built from this version of the code. Older images do not create the initial configuration automatically; the `latest` tag may still refer to an older release.
2. If another service uses port `7500`, stop it or choose another `PORT` and update the proxy upstream. Keep `BIND_ADDRESS=127.0.0.1` when the proxy runs on the same host.
3. From the directory containing the Compose file, start the service:

   ```bash
   docker compose config --quiet
   docker compose up -d
   ```

4. Compose creates the named volume `wol_secure_lightswitch_data`. On first start, the app creates a random JWT secret, an empty `hosts.yaml` and its SQLite database there. No host directory or SSH files are needed. An empty device list is healthy.
5. Point the local HTTPS proxy upstream to `http://127.0.0.1:7500`. Open the HTTPS URL and create the first administrator account. The dashboard initially says “No devices configured.” Add devices using the example below; configure SSH only if you need remote shutdown.

Docker selects the appropriate CPU architecture from a multi-platform image; no `platform:` setting is needed. A container marked `unhealthy` is not restarted solely because of its health status.

### Add devices

Create a local `hosts.yaml` containing a YAML list. For WOL, a device needs `id`, `name` and `mac`; set `ip` or `wol_interface` when needed to select the correct network. For example:

```yaml
- id: pc-gaming
  name: Gaming PC
  mac: "11:22:33:44:55:66"
  ip: "192.168.1.100"
  ping_interval: 30
  skip_interfaces: ["docker", "veth", "br-"]
```

The full field example is in [data/hosts.yaml.example](data/hosts.yaml.example). Device IDs must be unique, 1–64 ASCII characters long, begin with a letter or digit, and otherwise contain only letters, digits, `.`, `_` or `-`. User assignments use these IDs.

From the directory containing your local `hosts.yaml`, copy it into the running container's persistent volume:

```bash
docker cp ./hosts.yaml wol_secure_lightswitch:/app/data/hosts.yaml
docker exec -u 0 wol_secure_lightswitch chown wol:wol /app/data/hosts.yaml
docker exec -u 0 wol_secure_lightswitch chmod 600 /app/data/hosts.yaml
```

The app reloads `hosts.yaml` automatically. Check its health and logs after uploading:

```bash
docker inspect --format '{{.State.Health.Status}}' wol_secure_lightswitch
docker logs --tail 50 wol_secure_lightswitch
```

If the YAML is invalid, the healthcheck reports `unhealthy`; the last valid device list remains available. Correct the file and check again.

### Optional SSH shutdown

WOL needs no SSH setup. For shutdown, place a dedicated private key and a verified `known_hosts` file in `/app/data/.ssh/` in the named volume. Verify the target host key fingerprint independently before adding it; unknown or changed host keys are rejected. On the Docker host, with the two files in your current directory:

```bash
docker exec -u 0 wol_secure_lightswitch mkdir -p /app/data/.ssh
docker cp ./wol_switch_ed25519 wol_secure_lightswitch:/app/data/.ssh/wol_switch_ed25519
docker cp ./known_hosts wol_secure_lightswitch:/app/data/.ssh/known_hosts
docker exec -u 0 wol_secure_lightswitch chown -R wol:wol /app/data/.ssh
docker exec -u 0 wol_secure_lightswitch chmod 700 /app/data/.ssh
docker exec -u 0 wol_secure_lightswitch chmod 600 /app/data/.ssh/wol_switch_ed25519
docker exec -u 0 wol_secure_lightswitch chmod 600 /app/data/.ssh/known_hosts
```

Add the SSH settings to that device in `hosts.yaml`:

```yaml
  user: switchbot
  key_path: /app/data/.ssh/wol_switch_ed25519
  cmd: sudo -n /usr/sbin/poweroff
```

The app uses `/app/data/.ssh/known_hosts` by default in Docker. You may instead set `SSH_KNOWN_HOSTS_FILE` to another readable file. Password authentication is supported with `password_file` pointing to a protected file in the volume; use either `password_file` or `key_path`, never both for one device. Do not put plaintext passwords or private keys in `hosts.yaml`.

For a Linux target, create a dedicated SSH account and allow only the required poweroff command:

```bash
sudo adduser switchbot
sudo visudo -f /etc/sudoers.d/switchbot
```

Add `switchbot ALL=(root) NOPASSWD: /usr/sbin/poweroff` to that sudoers file. Install the matching public key on the target and verify the rule with `sudo visudo -cf /etc/sudoers.d/switchbot`. The SSH account does not need membership in the `sudo` group.

### Other proxy layouts

For a proxy on another host, change `BIND_ADDRESS` to `0.0.0.0`, set `TRUSTED_PROXIES` to that proxy's IP, and use a host firewall to permit port `7500` only from the proxy. The proxy-to-app connection is plain HTTP, so use a trusted private network or encrypted tunnel. Do this before opening access to the backend; direct browser access over HTTP does not support Secure authentication cookies.

### Update, back up and restore

To update, change only the image reference in the same Compose file and run `docker compose up -d`. Keep the `wol_secure_lightswitch_data` volume. It contains `secure-switch.db`, `hosts.yaml`, `jwt_secret` and any optional SSH files. Do not remove the volume during updates: removing it deletes the installation's users, configuration and signing secret.

Back up the entire volume, including the JWT secret, to a protected location. Stop the service first so SQLite is closed. On the Docker host:

```bash
docker compose stop wol-switch
umask 077
docker run --rm -v wol_secure_lightswitch_data:/source:ro -v "$PWD":/backup alpine:3.24 tar -C /source -czf /backup/secure-switch-backup.tar.gz .
docker compose start wol-switch
```

Store this archive privately: it contains password hashes, the JWT secret and potentially SSH credentials. To restore, stop the service, preserve a backup of its current volume, then extract the archive into `wol_secure_lightswitch_data`:

```bash
docker compose stop wol-switch
docker run --rm -v wol_secure_lightswitch_data:/target -v "$PWD":/backup alpine:3.24 tar -C /target -xzf /backup/secure-switch-backup.tar.gz
docker compose start wol-switch
```

Restore into an empty volume when possible. If restoring into an existing volume, remove or archive its prior contents first so files absent from the backup do not remain. Verify `/healthz` and login after starting the service. A normal image update does not require backup restoration.

### Local development

Outside Docker, set `JWT_SECRET` to a random value of at least 32 characters, or provide `JWT_SECRET_FILE`. An explicitly configured secret file takes precedence; a missing or invalid one fails startup. `INITIALIZE_DATA=true` opts into first-run file creation. Run `go test ./...` from `backend/` and `npm run build` from `frontend/` before building an image.

## Screenshots

![Admin dashboard](assets/readMe/mainPanelAdmin.png)
![User dashboard](assets/readMe/mainPanelUser.png)
![Admin panel](assets/readMe/adminPanel.png)
