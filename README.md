# WOL-Secure-lightSwitch (SecureSwitch)

SecureSwitch is a lightweight web application for managing devices on a local network. It combines Wake-on-LAN (WOL), availability monitoring and optional remote shutdown in one browser interface.

The backend is written in Go and the frontend in React. The application is designed for a Docker deployment behind an HTTPS reverse proxy, with role-based access control so administrators can decide which devices each user may control.

## Features

- **Wake-on-LAN:** wake configured devices by sending Magic Packets through the application host's network interfaces.
- **Availability monitoring:** show whether a device responds to ping checks.
- **Remote shutdown:** optionally execute a restricted shutdown command over SSH, using a private key or a protected password file.
- **Role-based access control:** administrators manage users and devices; standard users can control only their assigned devices.
- **First-run setup:** the first account created is granted administrator privileges.
- **Docker deployment:** the application, database and configuration are kept in a persistent Docker volume.

## Screenshots

### Administrator dashboard

The administrator dashboard shows the configured devices and their current availability.

![Administrator dashboard](assets/readMe/mainPanelAdmin.png)

### User dashboard

Standard users see and control only the devices assigned to them.

![User dashboard](assets/readMe/mainPanelUser.png)

### Administration panel

Administrators can create users and manage device assignments.

![Administration panel](assets/readMe/adminPanel.png)

---

<div style="page-break-after: always;"></div>

# Installation and configuration

## Prerequisites

The recommended deployment requires:

- a Linux host with Docker and Docker Compose;
- an HTTPS reverse proxy, such as Nginx, Caddy or Traefik;
- network access from the Docker host to the devices that will receive WOL packets or SSH commands.

The default Compose configuration uses host networking so WOL broadcasts can use the host's network interfaces. It listens on `127.0.0.1:7500`; users must access SecureSwitch through the HTTPS address of the reverse proxy.

## 1. Start SecureSwitch

Copy [docker-compose.yml](docker-compose.yml) to the server. Review the image reference and environment values before starting it. The repository Compose file currently uses the `latest` tag; for a production installation, prefer an immutable image digest corresponding to the version being deployed.

Validate and start the service:

```bash
docker compose config --quiet
docker compose up -d
```

The Compose file creates the named volume `wol_secure_lightswitch_data` and mounts it at `/app/data`. On the first start, `INITIALIZE_DATA=true` allows the application to create:

- a random JWT signing secret;
- an empty `hosts.yaml` configuration;
- the SQLite database.

No host `data/` directory or manually generated JWT secret is required for this Compose configuration. Do not remove the volume during normal updates: it contains the users, database, device configuration, signing secret and any optional SSH files.

Check the service after it starts:

```bash
docker ps
docker inspect --format '{{.State.Health.Status}}' wol_secure_lightswitch
docker logs --tail 50 wol_secure_lightswitch
```

The healthcheck calls `/healthz` every five minutes. An `unhealthy` status is a diagnostic signal; Docker does not restart a container solely because its health status is unhealthy.

## 2. Configure the HTTPS reverse proxy

SecureSwitch serves plain HTTP internally, but its authentication cookie is marked `Secure`. Users must therefore open the HTTPS URL exposed by the reverse proxy. Direct browser access to `http://SERVER_IP:7500` is not supported for normal use.

### Reverse proxy on the same host

This is the recommended layout. Keep the default binding:

```yaml
environment:
  - PORT=7500
  - BIND_ADDRESS=127.0.0.1
```

Configure the proxy upstream as:

```text
http://127.0.0.1:7500
```

If the proxy forwards client information, configure `TRUSTED_PROXIES` with only the local proxy addresses, for example:

```yaml
environment:
  - TRUSTED_PROXIES=127.0.0.1,::1
```

Do not add broad values such as `0.0.0.0/0` or `::/0`. The backend should not be directly reachable from the LAN.

### Reverse proxy on another host

If the proxy is on another server, bind the application to the network and restrict the port with a firewall. For example, if the SecureSwitch host is `192.168.1.20` and the proxy is `192.168.1.10`:

```yaml
environment:
  - PORT=7500
  - BIND_ADDRESS=0.0.0.0
  - TRUSTED_PROXIES=192.168.1.10
```

Allow port `7500` only from the proxy host. With UFW, the rules must be ordered so the specific allow rule comes before the general deny rule:

```bash
sudo ufw allow in proto tcp from 192.168.1.10 to any port 7500
sudo ufw deny in proto tcp to any port 7500
sudo ufw status numbered
```

Configure the remote proxy upstream as `http://192.168.1.20:7500`. The proxy-to-application connection is still plain HTTP, so use a trusted private network, VPN or encrypted tunnel. Apply the Compose change only after the firewall is ready:

```bash
docker compose config --quiet
docker compose up -d
```

Verify connectivity from the proxy host:

```bash
curl --connect-timeout 5 http://192.168.1.20:7500/healthz
```

## 3. Create the first administrator

Open the HTTPS URL configured on the reverse proxy. On a new installation, SecureSwitch detects that no users exist and presents the initial account setup. The first account is automatically granted administrator privileges.

After signing in, the administrator can:

1. view all devices defined in `hosts.yaml`;
2. create additional users;
3. assign device IDs to each standard user;
4. wake or shut down devices for which the account has permission.

## 4. Configure devices

Create a local file named `hosts.yaml` and use [data/hosts.yaml.example](data/hosts.yaml.example) as the complete reference. For a minimal WOL-only device, the required fields are `id`, `name` and `mac`:

```yaml
- id: pc-gaming
  name: Gaming PC
  mac: "11:22:33:44:55:66"
  ip: "192.168.1.100"
  ping_interval: 30
  skip_interfaces: ["docker", "veth", "br-"]
```

The device ID must be unique, 1–64 ASCII characters long, start with a letter or digit, and contain only letters, digits, `.`, `_` or `-`. User assignments refer to this ID exactly.

For WOL network selection:

- `wol_interface` selects an exact network interface name and has highest priority;
- otherwise, `ip` or a resolvable hostname helps select the compatible subnet;
- if neither is configured, exactly one eligible IPv4 network must remain;
- `skip_interfaces` is always applied and uses case-insensitive substring matching.

To upload the configuration into the persistent volume:

```bash
docker cp ./hosts.yaml wol_secure_lightswitch:/app/data/hosts.yaml
docker exec -u 0 wol_secure_lightswitch chown wol:wol /app/data/hosts.yaml
docker exec -u 0 wol_secure_lightswitch chmod 600 /app/data/hosts.yaml
```

SecureSwitch reloads `hosts.yaml` automatically. Check the health status and logs after uploading:

```bash
docker inspect --format '{{.State.Health.Status}}' wol_secure_lightswitch
docker logs --tail 50 wol_secure_lightswitch
```

If the new YAML is invalid, the healthcheck reports a configuration failure while the last valid device list remains available. Correct the file and upload it again.

## 5. Optional SSH shutdown

Wake-on-LAN does not require SSH. Configure SSH only for devices that need remote shutdown.

### SSH files in the persistent volume

For key-based authentication, copy a dedicated private key and a verified `known_hosts` file into `/app/data/.ssh/`:

```bash
docker exec -u 0 wol_secure_lightswitch mkdir -p /app/data/.ssh
docker cp ./wol_switch_ed25519 wol_secure_lightswitch:/app/data/.ssh/wol_switch_ed25519
docker cp ./known_hosts wol_secure_lightswitch:/app/data/.ssh/known_hosts
docker exec -u 0 wol_secure_lightswitch chown -R wol:wol /app/data/.ssh
docker exec -u 0 wol_secure_lightswitch chmod 700 /app/data/.ssh
docker exec -u 0 wol_secure_lightswitch chmod 600 /app/data/.ssh/wol_switch_ed25519
docker exec -u 0 wol_secure_lightswitch chmod 600 /app/data/.ssh/known_hosts
```

Verify the target host fingerprint independently before adding it to `known_hosts`. Unknown or changed host keys are rejected. The default Docker path is `/app/data/.ssh/known_hosts`; `SSH_KNOWN_HOSTS_FILE` can override it.

Add the SSH settings to the device:

```yaml
- id: server-proxmox
  name: Proxmox Node
  mac: "AA:BB:CC:DD:EE:FF"
  ip: "192.168.1.101"
  user: switchbot
  key_path: /app/data/.ssh/wol_switch_ed25519
  cmd: sudo -n /usr/sbin/poweroff
```

Use exactly one SSH credential source for each device: `key_path` or `password_file`. Never put a plaintext password or private key directly in `hosts.yaml`.

Password authentication is supported when `password_file` points to a protected file in the persistent volume. The file must contain only the target account's password and must be readable by the unprivileged `wol` user.

### Restrict the shutdown command on a Linux target

Create a dedicated account on the target host:

```bash
sudo adduser switchbot
sudo visudo -f /etc/sudoers.d/switchbot
```

Add only the required command:

```text
switchbot ALL=(root) NOPASSWD: /usr/bin/poweroff
```

Use the actual path reported by the target system if it differs. Validate the rule:

```bash
sudo visudo -cf /etc/sudoers.d/switchbot
```

Install the matching public key on the target. The SSH account does not need membership in the `sudo` group.

## Account input rules

- Email addresses must be valid.
- Passwords must contain at least 12 characters.
- When editing an account, leave the password field empty to retain the current password.
- An explicitly empty device list removes all assignments; an omitted list leaves existing assignments unchanged.

## Local development

Outside Docker, configure a JWT secret with at least 32 characters using `JWT_SECRET`, or provide `JWT_SECRET_FILE`. A configured secret file takes precedence. The application refuses to start when the required secret is missing or invalid. Set `INITIALIZE_DATA=true` only when first-run data creation is intended.

Run backend checks from `backend/`:

```bash
go test ./...
go vet ./...
```

Build and lint the frontend from `frontend/`:

```bash
npm run build
npm run lint
```

For a complete deployment configuration check:

```bash
docker compose config --quiet
```
