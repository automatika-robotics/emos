# EMOS CLI

The `emos` CLI manages installation, recipes, the dashboard daemon, and device configuration on a robot. Every long-form action it performs is also exposed over the dashboard's REST API (see [`internal/server/openapi.yaml`](https://github.com/automatika-robotics/emos/blob/main/stack/emos-cli/internal/server/openapi.yaml)), so anything you can do on the terminal you can also drive from a browser or an agentic skill.

## Quick Reference

| Command            | Description                                                 |
| :----------------- | :---------------------------------------------------------- |
| `emos install`     | Install EMOS (interactive mode selection)                   |
| `emos uninstall`   | Remove EMOS (mode-aware cleanup)                            |
| `emos update`      | Update EMOS to the latest version                           |
| `emos status`      | Show installation status                                    |
| `emos serve`       | Run the dashboard daemon (REST API + web UI)                |
| `emos config`      | Inspect or modify device configuration, pairing tokens, TLS |
| `emos recipes`     | List recipes available for download                         |
| `emos pull <name>` | Download a recipe                                           |
| `emos ls`          | List locally installed recipes                              |
| `emos run <name>`  | Run a recipe (foreground, blocking)                         |
| `emos info <name>` | Show sensor/topic requirements for a recipe                 |
| `emos map <cmd>`   | Mapping tools (record, edit)                                |
| `emos plugin <cmd>`| Install and manage the robot plugin                          |
| `emos version`     | Show CLI version                                            |

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

`emos run <name>` adapts to the install mode -- starting the container in container mode, sourcing ROS in native, activating the pixi env in pixi -- before exec-ing the recipe and streaming logs to `~/emos/logs/<recipe>_<timestamp>.log`. Logs are also visible from the dashboard's [Run console](dashboard.md#run-console).

For the full guide to writing, dropping in, and launching custom recipes (including the install-mode pitfalls of running them directly via `python`), see [Running Recipes](running-recipes.md).

## Recipe Layout

A recipe is a directory under `~/emos/recipes/` with the following structure:

```
~/emos/recipes/
  my_recipe/
    recipe.py          # Main entry point (required)
    manifest.json      # Optional: Zenoh config / display name / description
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

For the full walkthrough -- writing the recipe, dropping it in, verifying discovery, and launching it via `emos run` or the dashboard -- see [Running Recipes](running-recipes.md).

## Command Reference

### `emos install`

```bash
emos install                           # interactive mode menu
emos install --mode container          # OSS container (no ROS required on host)
emos install --mode native             # native (uses host's ROS 2)
emos install --mode pixi               # self-contained ROS via pixi (no system ROS)
emos install --mode licensed <key>     # licensed deployment (requires license key)
emos install --distro jazzy            # pin a ROS distro for container/native mode
```

| Flag       | Default    | Description                                          |
| :--------- | :--------- | :--------------------------------------------------- |
| `--mode`   | _(prompt)_ | One of: `container`, `native`, `pixi`, `licensed`.   |
| `--distro` | _(prompt)_ | ROS 2 distribution: `jazzy`, `humble`, `kilted`.    |

The installer offers, at the end, to:

- Install a systemd unit so the dashboard auto-starts at boot (see [Make the dashboard start automatically](dashboard.md#make-the-dashboard-start-automatically)).
- Persist the chosen device name and a fresh pairing code to `~/.config/emos/config.json`.

```{note}
Pixi mode requires [pixi](https://pixi.sh) on the host (`emos install --mode pixi` errors with install instructions if it's missing). It clones the EMOS workspace and builds it under `~/.local/share/emos`, independent of any system ROS. Currently pinned to ROS 2 Jazzy. See [Installation](installation.md#deployment-modes).
```

### `emos uninstall`

```bash
sudo emos uninstall                       # interactive
sudo emos uninstall --yes                 # non-interactive
sudo emos uninstall --keep-data           # preserve recipes + logs
sudo emos uninstall --keep-config         # preserve dashboard auth state
sudo emos uninstall --remove-image        # also docker rmi (container / licensed)
```

Stops the dashboard service and runs mode-specific cleanup. By default also removes `~/emos/recipes`, `~/emos/logs`, and `~/.config/emos`.

| Flag             | Default | Description                                                                                  |
| :--------------- | :------ | :------------------------------------------------------------------------------------------- |
| `--keep-data`    | `false` | Preserve `~/emos/recipes` and `~/emos/logs`.                                                 |
| `--keep-config`  | `false` | Preserve `~/.config/emos` (keeps device name + dashboard pairing across reinstall).          |
| `--remove-image` | `false` | Also `docker rmi` the EMOS image (container / licensed modes only; preserved by default).    |
| `-y`, `--yes`    | `false` | Skip the confirmation prompt.                                                                |

Mode-specific behavior:

- **Container / licensed:** `docker stop` + `docker rm` the EMOS container; licensed also removes the container auto-restart unit and `~/emos/robot/`.
- **Native:** removes the build workspace and `pip uninstall`s `kompass-core`. EMOS package files in `/opt/ros/<distro>/` are co-mingled with ROS by colcon and **cannot** be cleanly removed -- the command prints the manual `rm` commands rather than running them, so you can review and apply if you want.
- **Pixi:** removes the EMOS-owned pixi workspace at `~/.local/share/emos` (cloned repo + env + build) wholesale. A workspace you cloned yourself elsewhere is preserved — only its build artifacts are stripped.

The CLI binary at `/usr/local/bin/emos` is never removed automatically -- a running process can't reliably unlink itself. The command prints the `sudo rm` one-liner for you.

```{tip}
Run `emos uninstall` before switching install modes (e.g. native -> pixi). It clears auth tokens and mode-specific state that would otherwise carry over and confuse the new install.
```

### `emos update`

```bash
emos update
```

Detects the install mode and updates accordingly. Container mode pulls the latest image and recreates the container; native mode pulls the latest source, rebuilds, and re-installs into `/opt/ros/{distro}/`; pixi mode runs `git pull`, refreshes the pixi env (`pixi install`), and rebuilds the EMOS packages (`pixi run setup`). If a robot plugin is installed, it is also pulled to its latest commit and rebuilt.

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

| Flag              | Default   | Description                                                                                      |
| :---------------- | :-------- | :----------------------------------------------------------------------------------------------- |
| `--addr`          | _(empty)_ | `host:port` to bind. Empty falls back to the configured port (`emos config set port`) or `8765`. |
| `--no-mdns`       | `false`   | Skip mDNS announcement. The dashboard is then only reachable by IP / explicit hostname.          |
| `--no-auth`       | `false`   | **Dev only.** Accept all requests without a bearer token.                                        |
| `--tls`           | `false`   | Serve over HTTPS using a self-signed cert under `~/.config/emos/`.                               |
| `--qr`            | `false`   | Print a QR code with the dashboard URL and exit (no daemon).                                     |
| `-v`, `--verbose` | `false`   | Log every HTTP request (including reads) at DEBUG.                                               |

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
emos config reset                 # reset device state (pairing/name/port); keeps install info
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

| Flag                  | Default         | Description                                                        |
| :-------------------- | :-------------- | :----------------------------------------------------------------- |
| `--rmw`               | `rmw_zenoh_cpp` | One of: `rmw_zenoh_cpp`, `rmw_fastrtps_cpp`, `rmw_cyclonedds_cpp`. |
| `--skip-sensor-check` | `false`         | Skip the 10-second sensor-topic verification.                      |

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

### `emos plugin`

Install and manage the robot plugin — a ROS package that adapts a specific robot to the EMOS stack. A robot runs **one plugin at a time**. See [Robot Plugins](plugins.md) for the full guide.

```bash
emos plugin list                       # browse the catalog (active plugin marked)
emos plugin install <name>             # install + activate (replaces any active plugin)
emos plugin inspect                    # show the active plugin's feedbacks/commands/actions/events
emos plugin remove                     # remove the active plugin
```

| Subcommand          | Description                                                                                  |
| :------------------ | :------------------------------------------------------------------------------------------- |
| `list`              | List plugins available in the Automatika catalog; flags the currently active one.            |
| `install <name>`    | Clone, build (for your install mode), and activate a plugin. Prompts before replacing one.    |
| `inspect`           | Pretty-print the active plugin's introspection tree (same data the dashboard System page shows). |
| `remove`            | Remove the active plugin from the robot.                                                      |

Installing a plugin makes it importable; a recipe opts in with `Launcher(robot_plugin=MyRobotPlugin())`. The dashboard's [Plugins page](dashboard.md#plugins) drives the same flow from a browser.

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

| Path                     | Mode   | Purpose                                                 |
| :----------------------- | :----- | :------------------------------------------------------ |
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
   - **Firefox** keeps its own trust store: _Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import_ `~/.config/emos/tls.crt`, then tick _"Trust this CA to identify websites."_
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
* - `~/.local/share/emos/`
  - Pixi-mode install: cloned EMOS workspace, pixi env, and colcon overlay (`emos install --mode pixi`).
* - `~/emos/workspace/`
  - Robot-plugin source and build overlay (`emos plugin install`).
* - `/etc/systemd/system/emos-dashboard.service`
  - Dashboard auto-start unit (created by `emos serve install-service`).
* - `/etc/systemd/system/emos.service`
  - Container auto-restart unit (created by licensed install).
```

```{tip}
`~/.config/emos/config.json` is **the** persistent state. `emos config reset` clears the dashboard's device-side state (paired browsers, custom name, custom port) but **preserves install info** (mode, ROS distro, license key) so the dashboard keeps recognising the device as installed afterwards.
```
