# WOL-Secure-lightSwitch (SecureSwitch)

SecureSwitch is a modern, lightweight, and secure web application to manage Wake-on-LAN (WOL) and remote shutdown for devices on your network. Built with a Go backend and a React frontend, it provides a user-friendly interface to track device status, wake them up, and shut them down securely.

## Features

- **Wake-on-LAN (WOL):** Easily wake up machines on your local network using Magic Packets.
- **Remote Shutdown:** Securely shut down devices via SSH commands (supports both password and SSH key authentication).
- **Ping Monitoring:** Online/offline status shown in the dashboard, refreshed by frontend polling every 10 seconds.
- **Role-Based Access Control:** 
  - **Admins:** Can manage users, assign devices, and control any device.
  - **Standard Users:** Can only view and control the specific devices assigned to them by an admin.
- **First-time Setup:** The system automatically prompts you to create the initial administrative account.
- **Dockerized:** Simple and clean deployment using Docker Compose.

## 📸 Screenshots

<div align="center">
  <table>
    <tr>
      <td align="center">
        <b>Admin View</b><br>
        <img src="assets/readMe/mainPanelAdmin.png" width="600"><br>
        <i>Main dashboard for administrators, showing all devices</i>
      </td>
    </tr>
    <tr>
      <td align="center">
        <b>User View</b><br>
        <img src="assets/readMe/mainPanelUser.png" width="600"><br>
        <i>Restricted dashboard for standard users, showing only the selected devices</i>
      </td>
    </tr>
    <tr>
      <td align="center" colspan="2">
        <b>Admin Panel</b><br>
        <img src="assets/readMe/adminPanel.png" width="600"><br>
        <i>User management interface for admins</i>
      </td>
    </tr>
  </table>
</div>

---

## 🚀 Getting Started (Using Docker)

The easiest way to run WOL Secure LightSwitch is via Docker. 

### 1. Prerequisites
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed on your server.

### 2. Configuration
Create a `docker-compose.yml` file on your server (or use the one provided in the repository [docker-compose.yml](https://github.com/LoStome/WOL-Secure-lightSwitch/blob/main/docker-compose.yml)):

Generate a dedicated JWT secret before starting the service. Keep this file private and never commit it:

```bash
mkdir -p data
openssl rand -hex 32 > data/jwt_secret
chmod 600 data/jwt_secret
```

```yaml
services:
  wol-switch:
    image: docker.io/lostome/wol_secure_lightswitch@sha256:ceafb2762ffb45b889089b65df241cddae2a4951aa55f3ed60527a500db616dd
    container_name: wol_secure_lightswitch
    restart: unless-stopped
    network_mode: host # Required for Wake-on-LAN broadcasts
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O /dev/null http://127.0.0.1:$${PORT:-7500}/healthz"]
      interval: 5m
      timeout: 5s
      retries: 3
      start_period: 10s
    environment:
      - PORT=7500
      - BIND_ADDRESS=127.0.0.1
      - TZ=Europe/Rome
      - JWT_SECRET_FILE=/run/secrets/jwt_secret
      - SSH_KNOWN_HOSTS_FILE=/run/secrets/ssh_known_hosts
    secrets:
      - jwt_secret
      - source: ssh_private_key
        target: ssh_private_key
        mode: 0400
      - source: ssh_known_hosts
        target: ssh_known_hosts
        mode: 0444
    volumes:
      - ./data:/app/data # hosts.yaml and SQLite database; keep this directory private

secrets:
  jwt_secret:
    file: ./data/jwt_secret
  ssh_private_key:
    file: /home/user/.ssh/wol_switch_ed25519 # dedicated SSH key for this service
  ssh_known_hosts:
    file: /home/user/.ssh/known_hosts
```
Replace the SSH secret file paths with files on your server. Provision the target SSH host keys into `known_hosts` out of band and use a dedicated key for this service where possible. The container runs as the unprivileged `wol` user; ensure it can read the mounted secret files and read/write the host data directory. Keep `data/` and the SSH files private. For local runs outside Docker, set `JWT_SECRET` to a random value of at least 32 characters; `JWT_SECRET_FILE` takes precedence when both are set. The service refuses to start without a valid secret.

For password authentication, create a protected file containing only the target account's password. Add these entries to the service and top-level `secrets` sections of Compose:

```yaml
# Under services.wol-switch.secrets:
- source: ssh_password
  target: ssh_password

# Top-level secrets:
ssh_password:
  file: /home/user/.config/secureswitch/ssh_password
```

Then set `password_file: "/run/secrets/ssh_password"` and `key_path: ""` for that host in `hosts.yaml`. Do not configure both credential fields for the same host. With Docker Compose `file:` secrets, the source is bind-mounted and the service-level `mode`, `uid`, and `gid` settings are not applied. Protect the host file itself and make it readable by the container's `wol` user (check its numeric UID/GID in the image); otherwise SSH authentication will fail when the application reads `password_file`. This requirement also applies to the JWT and private-key files mounted from the host.

The healthcheck in this Compose service definition runs automatically and reports whether `/healthz` is healthy. Docker's `restart: unless-stopped` restarts a stopped process, but does not restart a container solely because its health status is `unhealthy`. A Dockerfile `HEALTHCHECK` could make a check the image default, but requires rebuilding and publishing the image, and a deployment can still override or disable it. Neither form alone forces recovery from an unhealthy status.

### 2.1 Required HTTPS reverse proxy

SecureSwitch is designed to run behind an HTTPS reverse proxy. Direct use through `http://SERVER_IP:7500` is not supported: the backend serves plain HTTP and its authentication cookie is always marked `Secure`, so browsers only send it over HTTPS. Users must open the HTTPS URL of the reverse proxy, never the backend URL.

`network_mode: host` is retained so the application can send Wake-on-LAN broadcasts through the host interfaces. `BIND_ADDRESS` controls only the HTTP listener and does not restrict outgoing WOL packets.

Choose one of the following reverse-proxy layouts.

#### Reverse proxy on the same host (recommended)

Keep the default Compose setting:

```yaml
environment:
  - PORT=7500
  - BIND_ADDRESS=127.0.0.1
  - TRUSTED_PROXIES=127.0.0.1,::1
```

Configure the reverse proxy upstream as `http://127.0.0.1:7500`. The backend then accepts connections only from the local host and is not reachable directly from the LAN. As optional defense in depth, an active UFW installation can explicitly reject external traffic to the backend port:

```bash
sudo ufw deny in to any port 7500 proto tcp
sudo ufw status numbered
```

#### Reverse proxy on another host

Assume this example network:

- SecureSwitch host: `192.168.1.20`
- Reverse proxy host: `192.168.1.10`

First, allow only the reverse proxy through the firewall. Add the specific allow rule before the general deny rule so the proxy is not blocked:

```bash
sudo ufw allow in proto tcp from 192.168.1.10 to any port 7500
sudo ufw deny in proto tcp to any port 7500
sudo ufw status numbered
```

Check that the allow rule for `192.168.1.10` appears before the general deny rule for port `7500`. These commands assume UFW is already enabled; before enabling a new firewall remotely, preserve the machine's SSH or other administration access according to the operating system documentation.

Then allow the backend to receive the proxy connection and trust forwarded client addresses only from that proxy:

```yaml
environment:
  - PORT=7500
  - BIND_ADDRESS=0.0.0.0
  - TRUSTED_PROXIES=192.168.1.10
```

Validate and apply the Compose configuration only after the firewall rules are in place:

```bash
docker compose config --quiet
docker compose up -d
```

Configure the remote reverse proxy upstream as `http://192.168.1.20:7500`. The connection between the proxy and SecureSwitch is still plain HTTP: use this layout only on a trusted private network or through an encrypted tunnel/VPN. All user-facing traffic must enter through the proxy's HTTPS URL.

Verify from the reverse proxy host that the backend responds:

```bash
curl --connect-timeout 5 http://192.168.1.20:7500/
```

Run the same command from another LAN host and confirm that it is blocked. If a rollback is needed, restore `BIND_ADDRESS=127.0.0.1`, run `docker compose up -d`, inspect the numbered rules with `sudo ufw status numbered`, and remove only the rules added for port `7500` with `sudo ufw delete RULE_NUMBER`.

Global trusted-proxy ranges such as `0.0.0.0/0` or `::/0` are rejected. Never trust a subnet containing untrusted clients.

SSH host keys are checked against the configured `known_hosts` file; unknown or changed host keys are not accepted automatically. For Docker Compose, mount that file as `ssh_known_hosts` as shown above. Outside Docker, set `SSH_KNOWN_HOSTS_FILE` to its path (the default is `data/.ssh/known_hosts`). Configure exactly one SSH credential source per host: `key_path` or `password_file`; keep credential files outside published or shared configuration bundles.

### Back up and restore application data

If you want to make a backup, the persistent `data/` directory contains `hosts.yaml` and `secure-switch.db`. The database contains user records, password hashes, and device assignments, so store backups with restrictive access. Back up only the database and host configuration; do not put JWT or SSH secrets in the backup bundle. Stop the service first so SQLite is closed, then run these commands from the directory containing `docker-compose.yml`:

```bash
docker compose stop
backup_dir="../secureswitch-backup-$(date +%Y%m%d-%H%M%S)"
umask 077
mkdir -m 700 "$backup_dir"
cp data/secure-switch.db data/hosts.yaml "$backup_dir/"
docker compose start
```

To restore, stop the service, copy the chosen backup's `secure-switch.db` and `hosts.yaml` back into `data/`, then start it again. Keep the existing secret files in place separately; restoring the database does not restore or rotate secrets. Verify `/healthz` and sign in after the service starts.

### 3. Define Your Devices
In the same directory as your `docker-compose.yml`, create a `data` folder and inside it, create a `hosts.yaml` file based on this structure (see also [data/hosts.yaml.example](https://github.com/LoStome/WOL-Secure-lightSwitch/blob/main/data/hosts.yaml.example)):

```yaml
# Example hosts.yaml
- id: server-proxmox
  name: "Proxmox Node 1"
  mac: "AA:BB:CC:DD:EE:FF"
  ip: "192.168.1.101" 
  user: "switchbot"
  password_file: "" # set to /run/secrets/ssh_password when using password authentication
  key_path: "/run/secrets/ssh_private_key" # alternatively, use password_file; never configure both
  cmd: "sudo -n /usr/sbin/poweroff"
  ping_interval: 15
  skip_interfaces: ["Tailscale", "vEthernet", "Loopback", "Bluetooth"]

- id: pc-gaming
  name: "PC Gaming Windows"
  mac: "11:22:33:44:55:66"
  ip: "192.168.1.100"
  user: "windows_user"
  password_file: ""
  key_path: "/run/secrets/ssh_private_key"
  cmd: "shutdown /s /t 0"
  ping_interval: 30
  skip_interfaces: ["docker", "veth", "br-"]
```

Host IDs are the device IDs used by the administrator when assigning access. Each ID must be unique, contain 1-64 ASCII characters, start with a letter or number, and then contain only letters, numbers, `.`, `_` or `-`. Assignments must use an ID that exists in `hosts.yaml` and may not repeat an ID.

### Account input rules

When creating an account, use a valid email address of at most 254 bytes. Passwords must contain at least 12 characters and no more than 72 UTF-8 bytes. When editing an account, leave the password field empty to keep the current password. An explicitly empty device list removes all assignments; an omitted device list leaves existing assignments unchanged. JSON request bodies larger than 64 KiB are rejected.

## 🔐 Remote Shutdown Setup (Linux)

To allow SecureSwitch to shut down your Linux machine, you need to configure the target system to allow the `poweroff` command without manual password entry.

- *Notes:*
  - *This is needed only if you want to use the shutdown feature.*
  - *Password and key are used only for logging into your machine, not for the shutdown command.*

### 1. Create a Dedicated User (Recommended)
For better security, it is best to use a dedicated user (e.g., `switchbot`) instead of `root`. Run these commands on your Linux shell:

```bash
# Create the user
sudo adduser switchbot
```

### 2. Enable Passwordless Shutdown
The SSH account does not need membership in the `sudo` group. Grant it passwordless permission for only the command used by the example configuration, `/usr/sbin/poweroff`.

Edit a dedicated sudoers file with `visudo`:

```bash
sudo visudo -f /etc/sudoers.d/switchbot
```

Add the following line (replace `switchbot` if you chose a different SSH username):

```plaintext
switchbot ALL=(root) NOPASSWD: /usr/sbin/poweroff
```

`visudo` checks the file when saving. You can also validate it explicitly:

```bash
sudo visudo -cf /etc/sudoers.d/switchbot
```

### 3. Verify Your hosts.yaml Configuration
Ensure your hosts.yaml matches the setup. Use the `-n` (non-interactive) flag in the command to prevent the application from hanging if permissions are misconfigured:

```yaml
- id: server-proxmox
  name: "Proxmox Node"
  ip: "192.168.1.101"
  user: "switchbot"
  key_path: "/run/secrets/ssh_private_key" # SSH login key mounted by Docker Compose
  cmd: "sudo -n /usr/sbin/poweroff"
```

Provision the matching public key for `switchbot` on the target and the target host key in the configured SSH `known_hosts` file. For local runs outside Docker, configure `key_path` and `SSH_KNOWN_HOSTS_FILE` to point to readable files.


### 4. Run the Service
Start the container in the background:
```bash
docker compose up -d
```

---

## 🖥️ Usage & First-Time Setup

1. **Access the Web Interface:** Open the HTTPS URL configured on your TLS reverse proxy. Do not browse directly to the application's plain-HTTP `PORT`.
2. **Initial Setup:** On the first visit, the system will recognize that no users exist and will prompt you to create the first account. This account will automatically be granted **Admin privileges**.
3. **Admin Dashboard:** 
   - Once logged in as an Admin, you can see all devices defined in your `hosts.yaml`.
   - You can create new users and explicitly assign specific devices to them.
   - For instance, you can give your friend access *only* to their own gaming PC to turn it on remotely.
4. **Controlling Devices:** Click on a device card to wake it up or shut it down. The online indicator will show you if the device responds to pings.

---
