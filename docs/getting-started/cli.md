# EMOS CLI

The `emos` CLI installs and updates EMOS, manages recipes, plugins and maps, and runs the dashboard daemon. Every long-running action is also exposed over the dashboard's REST API (see [`internal/server/openapi.yaml`](https://github.com/automatika-robotics/emos/blob/main/stack/emos-cli/internal/server/openapi.yaml)), so anything you can do on the terminal you can also drive from a browser or an agentic skill.

## Quick Reference

| Command             | Description                                                 |
| :------------------ | :---------------------------------------------------------- |
| `emos install`      | Install EMOS in pixi, native or container mode              |
| `emos uninstall`    | Remove EMOS (mode-aware cleanup)                            |
| `emos update`       | Update the CLI, the installation and every installed plugin |
| `emos status`       | Show the installation's health                              |
| `emos version`      | Show the CLI version                                        |
| `emos serve`        | Run the dashboard daemon (REST API + web UI)                |
| `emos config`       | Device configuration, pairing, TLS, and recipe UI API keys  |
| `emos recipes`      | List the recipes in the catalog                             |
| `emos pull <name>`  | Download a recipe                                           |
| `emos ls`           | List locally installed recipes                              |
| `emos info <name>`  | Show the topics a recipe uses and what provides them        |
| `emos run <name>`   | Run a recipe (foreground, blocking)                         |
| `emos plugin <cmd>` | Install and manage robot and sensor plugins                 |
| `emos map <cmd>`    | Build and manage maps of the robot's environment            |
| `emos completion`   | Shell completion scripts                                    |

```{tip}
Every command supports `-h`/`--help`. The CLI is a single static binary: copy `/usr/local/bin/emos` to another machine of the same architecture and it works.
```

## Typical Workflows

### First-time setup

```bash
emos install                    # interactive mode menu, offers the dashboard at boot
emos serve                      # prints the URLs, the pairing code and a QR (once)
```

Point a browser at `https://emos.local:8765` (or scan the QR), accept the certificate after checking its fingerprint, enter the pairing code, and you are in. See [Dashboard](dashboard.md).

### CLI-only recipe loop

```bash
emos recipes                    # browse the catalog
emos pull vision_follower       # download a recipe
emos info vision_follower       # what it needs, and what provides it
emos run vision_follower        # launch it (blocks until exit)
```

A recipe is a directory under `~/emos/recipes/` with a `recipe.py` and an optional `manifest.json`. See [Running Recipes](running-recipes.md) for the layout, the manifest, and how a recipe of your own gets there.

## Command Reference

### `emos install`

```bash
emos install                           # interactive mode menu
emos install --mode pixi               # self-contained ROS 2 via pixi (no system ROS)
emos install --mode native             # into the host's ROS 2
emos install --mode container          # Docker container, no ROS 2 on the host
emos install --mode native --distro jazzy
```

| Flag       | Default    | Description                                                                                               |
| :--------- | :--------- | :-------------------------------------------------------------------------------------------------------- |
| `--mode`   | _(prompt)_ | One of `pixi`, `native`, `container`.                                                                     |
| `--distro` | _(prompt)_ | ROS 2 distribution for native and container mode: `jazzy`, `humble`, `kilted`. Pixi mode is always Jazzy. |

At the end the installer offers to enable the dashboard at boot, and on a machine with CUDA a pixi install offers to build sherpa-onnx and llama-cpp-python for the GPU. See [Installation](installation.md#deployment-modes) for what each mode does and where it puts things.

### `emos uninstall`

```bash
emos uninstall                         # interactive
emos uninstall --yes                   # non-interactive
emos uninstall --keep-data             # keep recipes and logs
emos uninstall --keep-config           # keep ~/.config/emos (dashboard pairing, device name)
emos uninstall --remove-image          # container mode: also remove the image
```

| Flag             | Default | Description                                                        |
| :--------------- | :------ | :----------------------------------------------------------------- |
| `--keep-data`    | `false` | Preserve `~/emos/recipes` and `~/emos/logs`.                       |
| `--keep-config`  | `false` | Preserve `~/.config/emos`, so paired browsers survive a reinstall. |
| `--remove-image` | `false` | Also remove the EMOS Docker image (container mode).                |
| `-y`, `--yes`    | `false` | Skip the confirmation prompt.                                      |

Run it as your own user, not with `sudo`. It stops and removes the dashboard service, cleans up according to the install mode, removes the plugin workspace, and then the data and config directories unless told to keep them. Maps, map archives and the robot's certificate are left in place, and so is the binary. [Installation](installation.md#uninstalling) has the full list.

### `emos update`

```bash
emos update
```

Updates the CLI first. When a newer release exists it replaces its own binary, restarts the dashboard service if one is running, and asks you to run `emos update` again. The second run updates the installation for its mode (pixi: fetch, refresh the environment, rebuild; native: fetch, rebuild, merge into `/opt/ros`; container: pull the image and recreate the container), then pulls and rebuilds every installed plugin. On a pixi install with CUDA the GPU build is offered again at the end.

### `emos status`

```bash
emos status
```

Shows the CLI version, an update notice when a newer release exists, the install mode and ROS distro, the container's state or the pixi workspace, and whether each EMOS Python package and ROS package is present. On the dev channel it also prints `Channel: dev` and how to get back to stable.

### `emos version`

```bash
emos version
```

Prints the version card and an update notice when a newer release exists. The dashboard's `/api/v1/info` returns the same version.

### `emos serve`

Runs the dashboard daemon. See [Dashboard](dashboard.md) for the pages and the pairing flow.

```bash
emos serve                             # foreground, HTTPS on the configured port
emos serve --addr :9000                # bind another port for this run
emos serve --qr                        # print a QR code with the dashboard URL and exit
```

| Flag              | Default   | Description                                                                                           |
| :---------------- | :-------- | :---------------------------------------------------------------------------------------------------- |
| `--addr`          | _(empty)_ | `host:port` to bind. Empty uses the configured port (`emos config set port`), which defaults to 8765. |
| `--no-mdns`       | `false`   | Skip the mDNS announcement. The dashboard is then reachable by IP or explicit hostname only.          |
| `--no-auth`       | `false`   | **Dev only.** Accept requests without a bearer token. Refuses to bind anything but loopback.          |
| `--no-tls`        | `false`   | **Dev only.** Serve plain HTTP instead of HTTPS.                                                      |
| `--qr`            | `false`   | Print a QR code with the dashboard URL and exit.                                                      |
| `-v`, `--verbose` | `false`   | Log every HTTP request. By default only mutations and errors are logged.                              |

On start it prints the robot's identity, the URLs it is reachable at, the certificate's fingerprint, and the pairing code on first launch. When the dashboard is already running as a service, `emos serve` only prints that summary.

#### `emos serve install-service`

```bash
emos serve install-service
```

Writes `/etc/systemd/system/emos-dashboard.service`, enables it and starts it. Run it as your user; it escalates with sudo for the parts that need root. The unit runs `emos serve` as your user with the binary you invoked, restarts it on failure, and confines it to `~/emos` and `~/.config/emos` for writes.

#### `emos serve uninstall-service`

```bash
emos serve uninstall-service
```

Stops, disables and removes the unit. Recipes, configuration and the certificate are untouched.

### `emos config`

Inspects and edits `~/.config/emos/config.json`, the paired browsers, the robot's certificate and the API keys of recipe UIs.

```bash
emos config show                        # human-readable device state
emos config get [key]                   # one value, or the whole config as JSON
emos config set <key> <value>           # writable keys: name, port
emos config path                        # print the config file path
emos config tokens                      # list paired browsers and agents
emos config revoke-token <id|label>     # revoke one paired device
emos config rotate-pairing              # issue a fresh pairing code (paired browsers stay paired)
emos config tls-fingerprint             # print the certificate's SHA-256 fingerprint
emos config tls-regenerate              # mint a fresh certificate (after an IP or name change)
emos config reset                       # reset pairing, tokens, name and port; keeps the install
emos config api-keys create --name <who> [--scopes read,command] [--expires YYYY-MM-DD]
emos config api-keys list
emos config api-keys revoke <id>
```

#### `emos config get` and `set`

`get` with no key prints the whole config as JSON; with `name`, `mode`, `ros_distro` or `port` it prints that one value, which is handy in scripts. `set` accepts `name` (the mDNS hostname segment, `[a-z0-9-]`) and `port` (1 to 65535). Restart the dashboard afterwards:

```bash
emos config set name happy-robot
emos config set port 9000
sudo systemctl restart emos-dashboard.service   # if running as a service
```

#### `emos config tokens` and `revoke-token`

```text
ID        LABEL        ISSUED            EXPIRES
4d0e9c01  phone        2026-04-12 10:32  2026-07-11 10:32
71a3f82b  laptop       2026-04-12 11:07  2026-07-11 11:07
```

```bash
emos config revoke-token 4d0e9c01     # by id prefix
emos config revoke-token phone        # by label
```

#### `emos config rotate-pairing`

Issues a new six-digit pairing code without revoking the browsers already paired. A running dashboard picks the new code up immediately.

```bash
emos config rotate-pairing
# ✓ New pairing code (shown once): 829471
```

#### `emos config tls-fingerprint` and `tls-regenerate`

The fingerprint is what you compare with the browser's certificate view before trusting the dashboard the first time. Regenerate the certificate after the robot changes network or name, then restart the dashboard and any running recipe. See [HTTPS](#https).

#### `emos config api-keys`

A recipe that calls `enable_ui` serves its API over HTTPS and needs a key on every request except the health check. The browser needs none. Keys carry scopes: `read` covers data and streams, `command` covers publishing, services, goals and cancels. `create` prints the key once; running recipes pick up created and revoked keys without a restart.

```bash
emos config api-keys create --name laptop --scopes read,command
emos config api-keys list
emos config api-keys revoke <id>
```

### `emos recipes`

```bash
emos recipes
```

Lists the recipes in the Automatika catalog. Requires internet.

### `emos pull`

```bash
emos pull <recipe_name>
```

Downloads a recipe from the catalog into `~/emos/recipes/<name>/`. Asks before overwriting an existing one.

### `emos ls`

```bash
emos ls
```

Lists the recipes under `~/emos/recipes/`. The dashboard's **Recipes → Installed** tab shows the same set.

### `emos info`

```bash
emos info <recipe_name_or_path>
```

Reads the recipe's `Topic(...)` declarations and prints its sensors and other topics, each with the message type and where it comes from:

- **robot plugin**: provided by the installed robot plugin.
- **plugin `<id>`**: provided by the sensor plugin the recipe attaches under that id.
- **ROS topic**: a plain topic that a driver, or a node the recipe starts, has to publish.

The command also says whether the robot or sensor plugin a recipe needs is installed. It accepts a recipe name (looked up in `~/emos/recipes/`) or a path to a `.py` file.

### `emos run`

```bash
emos run <recipe_name>
emos run <recipe_name> --rmw rmw_fastrtps_cpp
```

| Flag    | Default   | Description                                                                                 |
| :------ | :-------- | :------------------------------------------------------------------------------------------ |
| `--rmw` | _(unset)_ | `rmw_fastrtps_cpp` or `rmw_zenoh_cpp`. Unset keeps the environment's RMW, or ROS's default. |

What happens:

1. The recipe directory is checked, and the run is refused while a plugin is being installed, updated or removed.
2. The environment for the install mode is prepared: the container is started, or ROS and the EMOS packages are checked in native and pixi mode. Installed plugins are sourced too.
3. With `--rmw rmw_zenoh_cpp`, a Zenoh router is started, or an already running one is reused.
4. `recipe.py` runs from its own directory. Output goes to the terminal and to `~/emos/logs/<recipe>_<timestamp>.log`.

Press Ctrl+C once to stop the recipe gracefully and twice to kill it. The CLI reports whether the recipe finished, was stopped, or exited with an error, and exits with a non-zero status in the last case.

### `emos plugin`

Plugins are ROS packages that adapt hardware to the EMOS stack. A robot runs one robot plugin plus any number of sensor plugins. See [Plugins](plugins.md) for the full guide.

```bash
emos plugin list                       # the catalog, installed plugins marked
emos plugin install <plugin>           # a robot plugin replaces the current robot; a sensor plugin is added
emos plugin inspect [slug]             # feedbacks, commands, actions and events of an installed plugin
emos plugin remove [slug]              # remove one plugin, or all of them without a slug
```

| Subcommand         | Description                                                                                 |
| :----------------- | :------------------------------------------------------------------------------------------ |
| `list`             | List the catalog with name, vendor and role, and mark the installed ones.                   |
| `install <plugin>` | Clone the plugin and its declared sources, resolve its dependencies, build, and record it.  |
| `inspect [slug]`   | Print the plugin's interface. Without a slug: the robot plugin, or the first sensor plugin. |
| `remove [slug]`    | Remove one plugin and rebuild the rest, or remove every plugin when no slug is given.       |

After an install the CLI prints the line a recipe needs: `Launcher(robot_plugin=...)` for a robot, `launcher.add_plugin(..., mount=Mount(...))` for a sensor. Installed plugins are updated by `emos update`.

### `emos map`

Builds and manages maps of the robot's environment. How a robot maps is declared in its plugin: EMOS drives the mapping software that ships with the robot, or builds the map itself from the robot's LiDAR when the robot ships none.

```bash
emos map setup [--rebuild]             # install the mapping backend EMOS builds maps with (pixi only)
emos map new [name] [--rmw ...]        # build a new map by driving the robot
emos map list                          # the maps on this robot, the active one marked
emos map use <name>                    # make a map the one the robot localizes against
emos map export [name] [-o dir]        # package a map for copying off the robot
emos map import <archive>              # unpack an exported map
emos map rm <name>                     # delete a map
```

`emos map new` needs a terminal: you drive the robot with its own controller and press Enter when you have finished. Native maps live in `~/emos/maps`, exported archives in `~/emos/map-archives`, and the session's output in `~/emos/logs/map-<name>_<timestamp>.log`. Building maps with EMOS itself is not available in container mode.

### `emos completion`

```bash
source <(emos completion bash)         # or zsh, fish, powershell
```

Generates the shell completion script.

## HTTPS

The dashboard serves HTTPS with a certificate minted for the robot on the first `emos serve` or `emos run`, stored at `~/emos/.ui-security/tls.crt` and `tls.key`. Recipe web UIs present the same certificate. Plain HTTP requests to the dashboard port are redirected to HTTPS; `emos serve --no-tls` is for development only.

The certificate is self-signed, so a browser shows a "Not Secure" warning on first contact. Compare the fingerprint that `emos serve` printed, or `emos config tls-fingerprint`, with the one the browser shows, then continue, or import `tls.crt` into the browser's trust store to silence the warning for good. The certificate covers `localhost`, `<name>.local`, `emos.local` and the robot's LAN addresses at mint time; after a network or name change run `emos config tls-regenerate` and restart the dashboard and any running recipe.

```{tip}
`emos config reset` clears the dashboard's device state (paired browsers, custom name, custom port) but keeps the install info, so the dashboard still recognises the device as installed afterwards.
```
