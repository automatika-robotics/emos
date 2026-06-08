# Installation

## EMOS CLI

The fastest way to get started with EMOS is through the CLI. Download the latest release:

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

Or build from source (requires Go 1.25+):

```bash
git clone https://github.com/automatika-robotics/emos.git
cd emos/stack/emos-cli
make build
sudo make install
```

```{tip}
The CLI is a single static binary with no runtime dependencies, copy `/usr/local/bin/emos` to another machine on the same architecture and it just works.
```

## Deployment Modes

EMOS supports four deployment modes. Run `emos install` without arguments for an interactive menu, or use the `--mode` flag directly.

::::{tab-set}

:::{tab-item} Container

No ROS2 installation required. Runs EMOS inside a Docker container using the public image.

```bash
emos install --mode container
```

You will be prompted to select a ROS2 distribution (Jazzy, Humble, or Kilted). The CLI pulls the image, creates the container, and sets up the `~/emos/` directory structure.

**Requirements:** Docker installed and running.

:::

:::{tab-item} Native

Builds EMOS packages from source and installs them directly into your ROS2 installation at `/opt/ros/{distro}/`. No container needed.

```bash
emos install --mode native
```

The CLI will:

1. Detect your ROS2 installation
2. Clone the EMOS source and dependencies into a build workspace (`~/emos/ros_ws/`)
3. Install system packages (portaudio, GeographicLib, rmw-zenoh)
4. Install Python dependencies
5. Install kompass-core with GPU acceleration support
6. Build all packages with colcon and install them into `/opt/ros/{distro}/`

After installation, EMOS packages are available whenever you source `/opt/ros/<distro>/setup.bash`. See [Running Recipes](running-recipes.md) for how to launch a recipe -- directly with `python` or via the `emos run` flow.

**Requirements:** A working ROS2 installation (Humble, Jazzy, or Kilted).

:::

:::{tab-item} pixi

```{note}
Currently pinned to **ROS 2 Jazzy**.
```

Installs ROS2 and all EMOS dependencies into an isolated userspace environment using [pixi](https://pixi.sh). No root privileges, no Docker, no pre-installed ROS2 required. Works on any Linux distribution.

```bash
# Install pixi first (emos install --mode pixi tells you if it's missing)
curl -fsSL https://pixi.sh/install.sh | bash

# Install EMOS in pixi mode
emos install --mode pixi
```

The CLI clones the EMOS workspace into `~/.local/share/emos`, pulls ROS 2 Jazzy and all dependencies as pre-built packages from [RoboStack](https://robostack.github.io/) and conda-forge, installs kompass-core with GPU acceleration, then builds the EMOS packages with colcon — independent of any system ROS 2.

See [Running Recipes](running-recipes.md) for how to launch a recipe -- directly from a `pixi shell` or via the `emos run` flow.

**Requirements:** Linux (amd64 or arm64). No root, Docker, or ROS2 needed.

:::

::::

See the [CLI Reference](cli.md) for the full list of commands.

## Which Mode Should I Choose?

| Scenario                                         | Recommended Mode |
| :----------------------------------------------- | :--------------- |
| No ROS2 on host, quick evaluation                | **Container**    |
| ROS2 already installed, system-level integration | **Native**       |
| No root, no Docker, any Linux distro             | **Pixi**         |

## Reach the Dashboard

During installation you were asked whether to enable the EMOS dashboard as a systemd service. Pick the path you chose below.

### If you enabled the systemd service (recommended)

The dashboard is already running and will come up automatically at every boot. The installer printed the access details once -- a six-digit pairing code, the URLs the dashboard is reachable at, and a scannable QR code. Open any of the URLs in a browser, enter the code, and the browser is paired for ~90 days.

If you missed the install output (or you've already paired and just need the URLs again), reprint the access summary at any time:

```bash
emos serve
```

When the dashboard is already running as a service, `emos serve` detects that and just shows the URLs and management commands -- it does not try to bind a second instance. Manage the service directly with:

```bash
systemctl status emos-dashboard.service
systemctl restart emos-dashboard.service
journalctl -u emos-dashboard.service -f
```

If you've lost the original pairing code, issue a fresh one with `emos config rotate-pairing`.

### If you skipped the systemd service

Start the dashboard manually whenever you want to use it:

```bash
emos serve
```

The first launch prints the pairing code, URLs, and QR code. The process runs in the foreground and stops when you `Ctrl-C` it. You can enable the service later with `emos serve install-service`.

```{seealso}
[Dashboard](dashboard.md) — full walkthrough of pairing, recipes, and run console
```

## Preparing Your Hardware

Before running recipes, you need sensor drivers publishing data on ROS2 topics. EMOS recipes declare the topics they expect (e.g. `Image` from a camera, `LaserScan` from a lidar). Run `emos info <recipe>` to see what a recipe needs.

### Installing Sensor Drivers

::::{tab-set}

:::{tab-item} Container

The EMOS container runs with `--privileged` and has access to all USB devices on the host. You can install and run sensor drivers directly **inside the container** — no ROS2 installation on the host is needed.

```bash
# Install a sensor driver inside the container:
docker exec -it emos bash -c "apt-get update && apt-get install -y ros-jazzy-usb-cam"

# Launch the driver inside the container (in a separate terminal):
docker exec -it emos bash -c "source /ros_entrypoint.sh && ros2 run usb_cam usb_cam_node_exe"
```

The driver's topics are immediately visible to recipes running in the same container.

```{tip}
If you have sensor drivers already running on the host with ROS2, they can bridge into the container automatically via Zenoh (the default RMW). Start the host driver with `export RMW_IMPLEMENTATION=rmw_zenoh_cpp`.
```

:::

:::{tab-item} Native

Install the driver package and launch it directly:

```bash
sudo apt install ros-jazzy-usb-cam
source /opt/ros/jazzy/setup.bash
export RMW_IMPLEMENTATION=rmw_zenoh_cpp
ros2 run usb_cam usb_cam_node_exe
```

If you place a launch file at `~/emos/robot/launch/bringup_robot.py`, the CLI will start it automatically when you run `emos run`.

:::

:::{tab-item} pixi

Pixi mode assumes you have **no system ROS2 installation**, so sensor drivers are installed into the pixi environment too. The EMOS workspace already has the [RoboStack](https://robostack.github.io/) `robostack-jazzy` channel configured, so adding a driver is a **single command** — install it straight into the EMOS environment:

```bash
cd ~/.local/share/emos
pixi add ros-jazzy-usb-cam
RMW_IMPLEMENTATION=rmw_zenoh_cpp pixi run ros2 run usb_cam usb_cam_node_exe
```

The driver lives in the same environment as your recipes, and because both use Zenoh as the default RMW, its topics are visible to running recipes automatically — no system ROS2, no separate project, no extra channel setup.

```{note}
`emos update` **preserves** drivers you add this way: it stashes your local `pixi.toml` / `pixi.lock` changes around the update and reapplies them. (In the rare case a release changes `pixi.toml` itself, you get a clear conflict to resolve rather than a silent overwrite.)
```

```{tip}
If a driver package isn't on RoboStack, install it from source into the EMOS environment with `colcon`, or fall back to Native mode for that driver only.
```

:::

::::

```{important}
Match the driver's RMW implementation to the one your recipe uses, or the driver's topics won't be visible to it. EMOS recipes default to **Zenoh** -- set `export RMW_IMPLEMENTATION=rmw_zenoh_cpp` in the shell where you launch the driver. If the recipe overrides this (e.g. `emos run <recipe> --rmw rmw_cyclonedds_cpp`), export the same value instead.
```

### Verifying Sensors

Before running a recipe, confirm your sensors are publishing:

```bash
# 1. See what the recipe needs
emos info vision_follower

# 2. Check topics exist
ros2 topic list

# 3. Confirm data is flowing
ros2 topic hz /image_raw
```

If `ros2 topic hz` shows a non-zero rate, the sensor is ready.

```{seealso}
If sensor verification fails during `emos run`, see [Troubleshooting](troubleshooting.md).
```

## Model Serving Platform

EMOS is agnostic to model serving platforms. You need at least one of the following available on your network:

- {material-regular}`download;1.2em;sd-text-primary` **[Ollama](https://ollama.com)** Recommended for local inference.
- {material-regular}`smart_toy;1.2em;sd-text-primary` **[RoboML](https://github.com/automatika-robotics/robo-ml)** Automatika's own open-source model serving package for quick prototyping.
- {material-regular}`api;1.2em;sd-text-primary` **OpenAI API-compatible fast inference servers** e.g. [llama.cpp](https://github.com/ggml-org/llama.cpp), [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang).
- {material-regular}`precision_manufacturing;1.2em;sd-text-primary` **[LeRobot](https://github.com/huggingface/lerobot)** For Vision-Language-Action (VLA) models.
- {material-regular}`cloud;1.2em;sd-text-primary` **Cloud endpoints** e.g. OpenAI, Claude, HuggingFace Inference etc. using an API key.

```{tip}
For larger models, run the serving platform on a GPU-equipped machine on your local network, or use a cloud endpoint, rather than running models directly on the robot.
```

## Updating

Update your installation to the latest version:

```bash
emos update
```

The CLI detects your installation mode and updates accordingly:

- **Container mode:** pulls the latest image and recreates the container.
- **Native mode:** pulls the latest source, rebuilds, and re-installs packages into `/opt/ros/{distro}/`.
- **Pixi mode:** runs `git pull` and `git submodule update` in the EMOS workspace at `~/.local/share/emos`, refreshes the pixi environment (`pixi install`), and rebuilds the EMOS packages (`pixi run setup`). Any installed robot plugin is pulled and rebuilt too.

## Uninstalling

To remove EMOS from a device:

```bash
sudo emos uninstall
```

After confirming, the CLI:

- Runs mode-specific cleanup:
  - **Container mode:** removes the Docker container. The image is preserved unless `--remove-image` is passed.
  - **Native mode:** The CLI prints the manual `rm` commands so you can clean them ROS packages yourself if you want.
  - **Pixi mode:** removes `.pixi/`, `build/`, `install/`, `log/` under your EMOS clone. The cloned repo itself is preserved.
- Removes `~/emos/recipes`, `~/emos/logs`, and `~/.config/emos` (installed recipes, run logs, and dashboard auth state).

Pass `--keep-data` to preserve `~/emos/recipes` and `~/emos/logs`. Pass `--keep-config` to preserve `~/.config/emos` (so previously paired browsers remain valid). Pass `-y` / `--yes` to skip the confirmation prompt.

The CLI binary at `/usr/local/bin/emos` is never removed automatically. The command prints the one-liner you can run after the process exits.

```{tip}
Use `emos uninstall` before switching install modes (e.g. native -> pixi). It clears auth tokens and mode-specific state that would otherwise carry over and confuse the new install.
```

## Installing from Source (Developer Setup)

If you want to build the full EMOS stack from source for contributing or accessing the latest features, follow the steps below. This installs all three stack components: **Sugarcoat** (architecture), **EmbodiedAgents** (intelligence), and **Kompass** (navigation).

### 1. Create a unified workspace

```shell
mkdir -p emos_ws/src
cd emos_ws/src
```

### 2. Clone the stack

```shell
git clone https://github.com/automatika-robotics/emos.git
cp -r emos/stack/sugarcoat .
cp -r emos/stack/embodied-agents .
cp -r emos/stack/kompass .
```

### 3. Install Python dependencies

```shell
PIP_BREAK_SYSTEM_PACKAGES=1 pip install numpy opencv-python-headless 'attrs>=23.2.0' jinja2 httpx setproctitle msgpack msgpack-numpy platformdirs tqdm pyyaml toml websockets
```

### 4. Install the Kompass core engine

The `kompass-core` package provides optimized planning and control algorithms.

::::{tab-set}

:::{tab-item} GPU Support (Recommended)

For production robots or high-performance simulation, install with GPU acceleration:

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/kompass-core/refs/heads/main/build_dependencies/install_gpu.sh | bash
```

:::

:::{tab-item} CPU Only

For quick testing or lightweight environments:

```bash
pip install kompass-core
```

:::

::::

### 5. Install ROS dependencies and build

```shell
cd emos_ws
rosdep update
rosdep install -y --from-paths src --ignore-src
colcon build
source install/setup.bash
```

You now have the complete EMOS stack built and ready to use.
