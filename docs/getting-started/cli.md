# EMOS CLI

The `emos` CLI manages installation, recipes, the dashboard daemon, and device configuration on a robot. Every long-form action it performs is also exposed over the dashboard's REST API (see [`internal/server/openapi.yaml`](https://github.com/automatika-robotics/emos/blob/main/stack/emos-cli/internal/server/openapi.yaml)), so anything you can do on the terminal you can also drive from a browser or an agentic skill.

## Quick Reference

| Command            | Description                                                         |
| :----------------- | :------------------------------------------------------------------ |
| `emos install`     | Install EMOS (interactive mode selection)                           |
| `emos update`      | Update EMOS to the latest version                                   |
| `emos status`      | Show installation status                                            |
| `emos serve`       | Run the dashboard daemon (REST API + web UI)                        |
| `emos config`      | Inspect or modify device configuration, pairing tokens, TLS         |
| `emos recipes`     | List recipes available for download                                 |
| `emos pull <name>` | Download a recipe                                                   |
| `emos ls`          | List locally installed recipes                                      |
| `emos run <name>`  | Run a recipe (foreground, blocking)                                 |
| `emos info <name>` | Show sensor/topic requirements for a recipe                         |
| `emos map <cmd>`   | Mapping tools (record, edit)                                        |
| `emos version`     | Show CLI version                                                    |

```{tip}
Every command supports `-h`/`--help`. The CLI is also a single static binary — copy `/usr/local/bin/emos` to another machine and it just works (no Python, no runtime dependencies).
```

## Typical Workflows

### First-time setup

```bash
# 1. Install EMOS (interactive mode menu)
emos install

# 2. Start the dashboard (printed pairing code is shown once)
emos serve

# 3. (optional) Make the dashboard auto-start at boot
sudo emos serve install-service
```

After step 2, point a browser at `http://emos.local:8765` (or scan the QR), enter the pairing code, and you're in. See [Dashboard](dashboard.md).

### CLI-only recipe loop

```bash
emos recipes                  # browse the catalog
emos pull vision_follower     # download a recipe
emos info vision_follower     # check what sensors it needs
emos run vision_follower      # launch it (blocks until exit)
```

## Running Recipes

`emos run <name>` adapts to the install mode:

- **Container mode** — starts the EMOS Docker container, configures the RMW (Zenoh by default), verifies sensor topics, executes the recipe inside the container.
- **Native / pixi mode** — verifies the ROS 2 environment, configures the RMW, verifies sensor topics, executes the recipe directly on the host.
- **Licensed mode** — uses the licensed deployment image with a sensor manifest from the robot package; otherwise identical to container mode.

In native or pixi mode you can also run recipes without the CLI:

```bash
# Native: source the system ROS install
source /opt/ros/jazzy/setup.bash
python3 ~/emos/recipes/my_recipe/recipe.py

# Pixi: enter the project shell first
pixi shell
source install/setup.sh
python3 ~/emos/recipes/my_recipe/recipe.py
```

All output is streamed to your terminal and saved to `~/emos/logs/<recipe>_<timestamp>.log`. Logs are also visible from the dashboard's [Run console](dashboard.md#run-console).

## Writing Custom Recipes

A recipe is a directory under `~/emos/recipes/` with the following structure:

```
~/emos/recipes/
  my_recipe/
    recipe.py          # Main entry point (required)
    manifest.json      # Optional Zenoh / display-name / description
```

### `manifest.json`

```json
{
  "name": "My Recipe",
  "description": "Does the thing.",
  "zenoh_router_config_file": "my_recipe/zenoh_config.json5"
}
```

- {material-regular}`label;1.2em;sd-text-primary` **name** — display name for the dashboard's recipe cards. Falls back to the directory name.
- {material-regular}`description;1.2em;sd-text-primary` **description** — short blurb shown on the recipe detail page.
- {material-regular}`settings;1.2em;sd-text-primary` **zenoh_router_config_file** — path (relative to `~/emos/recipes/`) to a Zenoh router `.json5` config file. Only consulted when the recipe runs under `rmw_zenoh_cpp`.

```{note}
Sensor requirements are auto-extracted from `recipe.py` by parsing `Topic(name=..., msg_type=...)` declarations. You don't need to list them in the manifest. Run `emos info <recipe>` (or open the recipe in the dashboard) to see the inferred requirements.
```

### `recipe.py`

A standard EMOS Python script:

```python
from agents.clients.ollama import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel
from agents.ros import Topic, Launcher

text_in  = Topic(name="text0",     msg_type="String")
image_in = Topic(name="image_raw", msg_type="Image")
text_out = Topic(name="text1",     msg_type="String")

model  = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
client = OllamaClient(model)

vlm = VLM(
    inputs=[text_in, image_in],
    outputs=[text_out],
    model_client=client,
    trigger=text_in,
)

launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

### Verifying your custom recipe

```bash
emos ls               # confirm it appears
emos info my_recipe   # check sensor requirements
emos run my_recipe    # launch it
```

It will also show up in the dashboard's **Recipes → Installed** tab automatically.

## Command Reference

### `emos install`

```bash
emos install                           # interactive mode menu
emos install --mode container          # OSS container (no ROS required on host)
emos install --mode native             # native (uses host's ROS 2)
emos install --mode licensed <key>     # licensed deployment (requires license key)
emos install --distro jazzy            # pin a ROS distro for container/native mode
```

| Flag        | Default       | Description                                               |
| :---------- | :------------ | :-------------------------------------------------------- |
| `--mode`    | _(prompt)_    | One of: `container`, `native`, `licensed`.                |
| `--distro`  | _(prompt)_    | ROS 2 distribution: `jazzy`, `humble`, `kilted`.          |

The installer offers, at the end, to:

- Install a systemd unit so the dashboard auto-starts at boot (see [Make the dashboard start automatically](dashboard.md#make-the-dashboard-start-automatically)).
- Persist the chosen device name and a fresh pairing code to `~/.config/emos/config.json`.

```{note}
The pixi install path is currently driven from the EMOS repo (`pixi run setup`) rather than `emos install --mode pixi`. See [Installation](installation.md#deployment-modes).
```

### `emos update`

```bash
emos update
```

Detects the install mode and updates accordingly. Container mode pulls the latest image and recreates the container; native mode pulls the latest source, rebuilds, and re-installs into `/opt/ros/{distro}/`.

### `emos status`

```bash
emos status
```

Shows install mode, ROS distro, container/service state, and dashboard service health. Subset of what `emos config show` reports.

### `emos serve`

Run the dashboard daemon. See the dedicated [Dashboard](dashboard.md) page for the UX.

```bash
emos serve                             # foreground; HTTP on the configured port
emos serve --tls                       # opt into HTTPS (self-signed cert)
emos serve --addr :9000                # bind to a custom port for one run
emos serve --qr                        # print a QR for the dashboard URL and exit
```

| Flag         | Default | Description                                                                            |
| :----------- | :------ | :------------------------------------------------------------------------------------- |
| `--addr`     | _(empty)_ | `host:port` to bind. Empty falls back to the configured port (`emos config set port`) or `8765`. |
| `--no-mdns`  | `false` | Skip mDNS announcement. The dashboard is then only reachable by IP / explicit hostname. |
| `--no-auth`  | `false` | **Dev only.** Accept all requests without a bearer token.                              |
| `--tls`      | `false` | Serve over HTTPS using a self-signed cert under `~/.config/emos/`.                     |
| `--qr`       | `false` | Print a QR code with the dashboard URL and exit (no daemon).                           |
| `-v`, `--verbose` | `false` | Log every HTTP request (including reads) at DEBUG.                                |

#### `emos serve install-service`

```bash
sudo emos serve install-service
```

Writes `/etc/systemd/system/emos-dashboard.service`, enables it, and starts it. The unit's `ExecStart` points at the binary you ran the command with, so it follows the active install (`/usr/local/bin/emos` if installed via the script).

#### `emos serve uninstall-service`

```bash
sudo emos serve uninstall-service
```

Stops, disables, and removes the unit. Does not remove the cert, config, or recipes.

### `emos config`

Inspect and modify everything in `~/.config/emos/config.json` — install info, device name, port, paired-device tokens, and TLS material.

```bash
emos config show                  # human-readable device state
emos config get [key]             # print one value or the whole config as JSON
emos config set <key> <value>     # writable keys: name, port
emos config path                  # print the config file path
emos config tokens                # list paired browsers / agents
emos config revoke-token <id|label>  # revoke a single paired device
emos config rotate-pairing        # issue a fresh pairing code (existing tokens stay valid)
emos config tls-fingerprint       # print the dashboard TLS cert SHA-256 fingerprint
emos config tls-regenerate        # re-mint the self-signed TLS cert (use after IP change)
emos config reset                 # WIPE config.json (keeps recipes, logs)
```

#### `emos config show`

Prints a single-pane summary:

```text
EMOS DEVICE STATE

  Identity:              epic-otter
  Mode:                  native
  ROS distro:            jazzy
  Dashboard port:        8765
  Recipes:               /home/you/emos/recipes
  Logs:                  /home/you/emos/logs
  Config file:           /home/you/.config/emos/config.json

  Pairing configured:    yes
  Active tokens:         2
  Dashboard service:     active (emos-dashboard.service)
```

#### `emos config get [key]`

With no argument, prints the full config as JSON. With a key (`name`, `mode`, `ros_distro`, `port`), prints a single value — useful in shell scripts.

#### `emos config set <key> <value>`

Writable keys: `name` (mDNS hostname segment, validated against `[a-z0-9-]`), `port` (1–65535). Other fields are managed by the installer / serve daemon.

```bash
emos config set name happy-robot
emos config set port 9000
sudo systemctl restart emos-dashboard.service   # if running as a service
```

#### `emos config tokens` and `revoke-token`

Lists paired browsers without leaking the underlying token hash:

```text
ID        LABEL        ISSUED            EXPIRES
4d0e9c01  phone        2026-04-12 10:32  2026-07-11 10:32
71a3f82b  laptop       2026-04-12 11:07  2026-07-11 11:07
```

Revoke one device:

```bash
emos config revoke-token 4d0e9c01     # by short id (prefix of hash)
emos config revoke-token phone        # by exact label
```

#### `emos config rotate-pairing`

Issues a new six-digit pairing code, **without** revoking already-paired tokens. Useful when a code may have been seen by someone who shouldn't get further access.

```bash
emos config rotate-pairing
# ✓ New pairing code (shown once): 829471
```

#### `emos config tls-fingerprint` / `tls-regenerate`

See [HTTPS (Optional)](#https-optional) below.

### `emos pull`

```bash
emos pull <recipe_short_name>
```

Downloads a recipe from the Automatika catalog and extracts it to `~/emos/recipes/<name>/`. Overwrites the existing version if present. Requires internet.

### `emos ls`

```bash
emos ls
```

Lists everything under `~/emos/recipes/`. The dashboard's **Recipes → Installed** tab shows the same set.

### `emos info`

```bash
emos info <recipe_name_or_path>
```

Inspects a recipe's Python source via AST and prints its sensor and topic requirements. Accepts either a recipe name (looked up in `~/emos/recipes/`) or a path to a `.py` file:

```bash
emos info vision_follower      # ~/emos/recipes/vision_follower/recipe.py
emos info ./my_recipe.py       # explicit path
```

The output groups topics into:

- **Required Sensors** — `Image`, `LaserScan`, `Imu`, `Audio`, `Odometry`, `RGBD`, `PointCloud2`, `CompressedImage`. Hardware label and suggested apt packages tailored to your distro.
- **Other Topics** — non-sensor topics declared by the recipe.

### `emos run`

```bash
emos run <recipe_short_name>
emos run <recipe> --rmw rmw_cyclonedds_cpp
emos run <recipe> --skip-sensor-check
```

| Flag                   | Default            | Description                                              |
| :--------------------- | :----------------- | :------------------------------------------------------- |
| `--rmw`                | `rmw_zenoh_cpp`    | One of: `rmw_zenoh_cpp`, `rmw_fastrtps_cpp`, `rmw_cyclonedds_cpp`. |
| `--skip-sensor-check`  | `false`            | Skip the 10-second sensor-topic verification.            |

#### What happens during `emos run`

1. Reads `recipe.py` and extracts `Topic(...)` declarations.
2. Identifies sensor topics.
3. Starts the Zenoh router (only when using `rmw_zenoh_cpp`).
4. Launches `~/emos/robot/launch/bringup_robot.py` if it exists (native / pixi only; container mode uses an in-container bringup).
5. Verifies each sensor topic is publishing (polls `ros2 topic list` for up to 10 s).
6. Executes the recipe — output streams to the terminal and is saved to `~/emos/logs/`.

#### When to use `--skip-sensor-check`

- Sensors that publish on-demand (service-triggered cameras).
- Replaying a rosbag whose topic names differ from the recipe.
- Pure AI recipes (LLM chat, TTS) that don't require sensor data.

```{warning}
If you skip the check and a sensor topic never arrives, the recipe may hang silently waiting for data. Use `ros2 topic hz /topic_name` to diagnose.
```

```{seealso}
[Troubleshooting](troubleshooting.md) for common errors during recipe execution.
```

### `emos map`

Mapping subcommands for creating and editing environment maps:

```bash
emos map record               # record mapping data on the robot
emos map install-editor       # install the map editor container (one-time)
emos map edit <file.tar.gz>   # process a ROS bag into a PCD map
```

### `emos version`

```bash
emos version
# emos vX.Y.Z
```

Prints the CLI version. The dashboard's `/api/v1/info` returns the same value.

## HTTPS (Optional)

The dashboard serves plain HTTP by default. That's the right default on a trusted LAN — bearer-token auth gates every write, so eavesdropping yields nothing useful unless someone can also intercept the pairing handshake.

You can opt in to HTTPS with a self-signed certificate:

```bash
emos serve --tls
```

### What the flag does

On first launch under `--tls`, the daemon mints a 2-year ECDSA P-256 certificate and persists it under `~/.config/emos/`:

| Path | Mode | Purpose |
| :--- | :--- | :--- |
| `~/.config/emos/tls.crt` | `0644` | PEM-encoded leaf certificate (also acts as its own CA). |
| `~/.config/emos/tls.key` | `0600` | PEM-encoded private key. Never copy this off the robot. |

The certificate's SubjectAltName covers:

- `localhost`, `<device-name>.local`, `emos.local`
- `127.0.0.1`, `::1`
- Every LAN IPv4 the device sees at mint time (excluding loopback / docker / veth / tailscale-style virtual interfaces).

It re-uses the persisted cert on subsequent launches and auto-rotates inside the 30-day-before-expiry window. Network-level changes (new IP, new mDNS name) don't trigger automatic rotation — see `tls-regenerate` below.

### Trusting the cert

A self-signed cert produces a "Not Secure" warning the first time a browser hits it. The warning is encryption-preserving — TLS still negotiates a session — but the trust chain is empty, so browsers refuse to call the connection authenticated. Two paths from there:

1. **Click through, every time.** Fine for a workshop / one-off install. The encryption protects against passive sniffing on the LAN, the bearer token still protects against unauthenticated access, and the warning is mostly cosmetic. Verify the cert before clicking through:

    ```bash
    emos config tls-fingerprint
    ```

    Compare the printed SHA-256 fingerprint with the one the browser shows under "View Certificate" before trusting it.

2. **Add the cert to a trust store** so the warning goes away permanently for that device:

    - **Firefox** keeps its own trust store: *Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import* `~/.config/emos/tls.crt`, then tick *"Trust this CA to identify websites."*
    - **Chrome / Edge** follow the OS trust store. On Linux: `sudo cp tls.crt /usr/local/share/ca-certificates/emos-<name>.crt && sudo update-ca-certificates`. On macOS, drop the cert into Keychain Access and mark it Always Trust. On Windows, `certmgr.msc → Trusted Root Certification Authorities → Import`.

### When to regenerate

The SAN list is fixed at mint time. If the robot's primary IP changes (new network, new DHCP lease) or you renamed it (`emos config set name`), the cert no longer matches the new address and the browser will show a different error (`NET::ERR_CERT_COMMON_NAME_INVALID`). Regenerate:

```bash
emos config tls-regenerate
sudo systemctl restart emos-dashboard.service   # if running as a service
```

Inspect the active fingerprint at any time:

```bash
emos config tls-fingerprint
# TLS CERTIFICATE
#   Fingerprint (SHA-256):
#     8B:F2:0C:...:E7
#   Expires: 2028-04-28
#   Certificate: /home/you/.config/emos/tls.crt
#   Private key: /home/you/.config/emos/tls.key
```

### When you actually need HTTPS

Most EMOS deployments are happy on HTTP. Reach for `--tls` when:

- You're building dashboard features that need a **secure context** — `getUserMedia` (microphone / camera), Web Audio capture, Service Workers, Web Bluetooth. Browsers won't expose these APIs over plain HTTP.
- You're running on a network you don't fully control (a venue Wi-Fi, a colocated factory) where token-only auth feels too thin.
- An organisational policy requires HTTPS end-to-end.

```{tip}
Per-recipe Sugarcoat web UIs that need a secure context handle their own TLS independently of the dashboard. See the [Dynamic Web UI](../concepts/web-ui.md) page.
```

### Running as a service with TLS

The systemd unit installed by `emos serve install-service` runs plain HTTP. To switch the service to HTTPS, edit its `ExecStart`:

```bash
sudo systemctl edit --full emos-dashboard.service
# add --tls to the ExecStart line, e.g.
# ExecStart=/usr/local/bin/emos serve --addr :8765 --tls
sudo systemctl daemon-reload
sudo systemctl restart emos-dashboard.service
```

## Files & Paths

```{list-table}
:header-rows: 1
:widths: 35 65

* - Path
  - Purpose
* - `~/.config/emos/config.json`
  - Single source of truth: install info, device name, port, paired-device tokens (hashed). Mode `0600`.
* - `~/.config/emos/tls.crt` / `tls.key`
  - Self-signed TLS material (created by `emos serve --tls` or `emos config tls-regenerate`).
* - `~/emos/recipes/`
  - Installed recipes. Each subdirectory is one recipe.
* - `~/emos/logs/`
  - Per-run log files: `<recipe>_<timestamp>.log`. Streamed by `emos run` and the dashboard.
* - `~/emos/ros_ws/`
  - Native-mode build workspace.
* - `/etc/systemd/system/emos-dashboard.service`
  - Dashboard auto-start unit (created by `emos serve install-service`).
* - `/etc/systemd/system/emos.service`
  - Container auto-restart unit (created by licensed install).
```

```{tip}
`~/.config/emos/config.json` is **the** persistent state. Wiping it (`emos config reset`) returns the device to "first boot" without touching recipes or logs.
```
