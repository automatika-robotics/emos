# Installation

## EMOS CLI

Everything starts with the `emos` command line tool. It installs EMOS on the robot, keeps it up to date, and runs recipes. Get the latest release with:

```bash
curl -fsSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

This only places the `emos` binary in `/usr/local/bin`. The actual installation happens when you run `emos install`, which we get to next.

If you would rather build the CLI yourself, you need Go 1.25 or newer, plus Node.js and npm, since the dashboard's web app is built and embedded into the binary:

```bash
git clone https://github.com/automatika-robotics/emos.git
cd emos/stack/emos-cli
make build
make install
```

## Deployment Modes

There are three ways to install EMOS, and the right one depends mostly on what is already on the machine. If you are not sure, pick pixi. Running `emos install` without any flags gives you a menu with the same three choices.

| Mode          | Pick it when                                                                | ROS 2 comes from                        |
| :------------ | :-------------------------------------------------------------------------- | :-------------------------------------- |
| **pixi**      | Most robots and development machines. No system ROS 2 and no Docker needed. | An isolated environment under your home |
| **native**    | ROS 2 is already installed and you want EMOS inside it.                     | Your `/opt/ros/<distro>`                |
| **container** | A quick evaluation on a machine with Docker.                                | The public EMOS image                   |

::::{tab-set}

:::{tab-item} pixi

Pixi mode gives you a complete ROS 2 Jazzy environment under your home directory, with nothing installed system-wide. You need [pixi](https://pixi.sh) itself before you begin, and a new shell after installing it so that it is on your path:

```bash
curl -fsSL https://pixi.sh/install.sh | bash

emos install --mode pixi
```

The CLI clones the EMOS workspace into `~/.local/share/emos` (or `$XDG_DATA_HOME/emos` if you have that set) and pulls ROS 2 Jazzy and all of its dependencies as prebuilt packages from [RoboStack](https://robostack.github.io/) and conda-forge. It then builds kompass-core from source, so that navigation can use whatever GPU the machine has, and finally builds the EMOS packages with colcon. Expect the first install to take 10 to 20 minutes on a desktop, and noticeably longer on a robot's own board.

**Requirements:** Linux on amd64 or arm64, pixi, and sudo for the kompass-core toolchain.

:::

:::{tab-item} native

Native mode is for machines that already have ROS 2 installed and where you want EMOS to live alongside it, in `/opt/ros/<distro>`:

```bash
emos install --mode native
```

The CLI looks for ROS 2 installations (Humble, Jazzy or Kilted are supported) and asks you to confirm the one it found. It then clones the EMOS source into `~/emos/ros_ws/`, installs the system packages the stack needs with apt (PortAudio, jq, the Zenoh RMW and the MoveIt message packages), builds kompass-core from source with GPU support, installs the Python dependencies with pip, resolves whatever ROS dependencies are still missing with rosdep, and builds the four EMOS packages with colcon. As a last step it merges the build into `/opt/ros/<distro>/`, so from then on EMOS is simply part of your ROS 2 installation: source `/opt/ros/<distro>/setup.bash` and it is there.

**Requirements:** a working ROS 2 installation and sudo.

:::

:::{tab-item} container

Container mode needs nothing but Docker, which makes it the quickest way to try EMOS on a machine that has neither ROS 2 nor a GPU toolchain set up:

```bash
emos install --mode container
```

After you pick a ROS 2 distribution (Jazzy, Humble or Kilted), the CLI pulls `ghcr.io/automatika-robotics/emos:<distro>-latest` and creates a container named `emos`. The container uses the host's network, has access to USB devices, sees your `~/emos` directory as `/emos`, and gets the NVIDIA runtime when Docker has one. Recipes run inside it.

The convenience comes with a few limits that the other two modes do not have. The image does not contain the sensor drivers that a robot plugin may depend on, so when you install such a plugin the CLI tells you what the image would need rather than installing it. Building maps with EMOS itself is not available in a container. And anything you install inside the container by hand is gone the next time `emos update` recreates it.

**Requirements:** Docker, installed and running.

:::

::::

The [CLI Reference](cli.md) lists every command and flag.

## What every install asks

Whichever mode you choose, the last thing `emos install` does is offer to set up a systemd service so that the [dashboard](dashboard.md) starts at every boot. If you say yes, it prints the URLs the dashboard is reachable at, a pairing code that is shown only this once, and a QR code you can scan with a phone. If you decline now, `emos serve install-service` sets the service up at any later time.

```{tip}
Some boards have a power supply that cannot keep up with a full-load compile. If yours resets or powers off during the install, cap the compile jobs with `EMOS_BUILD_JOBS=4 emos install --mode pixi`. See [Troubleshooting](troubleshooting.md#the-board-resets-or-powers-off-during-a-pixi-install).
```

## Reach the Dashboard

If you enabled the service, the dashboard is already running and comes back on its own after every reboot. The installer printed everything you need to get in: the URLs, the six-digit pairing code and the QR code. Open one of the URLs in a browser. The first visit shows a certificate warning, because the robot signs its own certificate. Enter the pairing code, and that browser stays paired for about 90 days.

If you did not catch that output, or need the URLs again later:

```bash
emos serve
```

```{seealso}
[Dashboard](dashboard.md) walks through pairing, the pages, and the security model.
```

## Connecting Your Robot

EMOS talks to a robot through a plugin. The plugin knows the robot's own interfaces, starts its sensor drivers when a recipe needs them, and exposes the robot's actions and events to recipes. For a robot in the catalog, installing the plugin is the only setup needed:

```bash
emos plugin list                        # what is in the catalog
emos plugin install <plugin>            # install the plugin for your robot
```

Extra sensors, whether mounted on the robot or placed somewhere in its environment, are added the same way as sensor plugins. The [Plugins](plugins.md) page covers installing and using them, what to do with a robot that is not in the catalog yet, and how to check what a recipe expects with `emos info`.

## Model Serving Platform

EMOS is agnostic to model serving platforms. You need at least one of the following available on your network:

- {material-regular}`download;1.2em;sd-text-primary` **[Ollama](https://ollama.com)** Recommended for local inference.
- {material-regular}`smart_toy;1.2em;sd-text-primary` **[RoboML](https://github.com/automatika-robotics/robo-ml)** Automatika's own open-source model serving package for quick prototyping.
- {material-regular}`api;1.2em;sd-text-primary` **OpenAI API-compatible fast inference servers** e.g. [llama.cpp](https://github.com/ggml-org/llama.cpp), [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang).
- {material-regular}`precision_manufacturing;1.2em;sd-text-primary` **[LeRobot](https://github.com/huggingface/lerobot)** For Vision-Language-Action (VLA) models, version 0.6.0 or newer.
- {material-regular}`cloud;1.2em;sd-text-primary` **Cloud endpoints** e.g. OpenAI, Claude, HuggingFace Inference etc. The API key is read from an environment variable.

```{tip}
For larger models, run the serving platform on a GPU-equipped machine on your local network, or use a cloud endpoint, rather than running models directly on the robot.
```

## Updating

```bash
emos update
```

The update happens in two rounds. First the CLI updates itself: if there is a newer release, it downloads it, replaces its own binary, restarts the dashboard service if one is running, and then asks you to run `emos update` once more. That second run is the one that updates the installation, in the way that fits its mode.

Whatever the mode, every installed plugin, the robot and any sensors, is pulled and rebuilt at the end.

## Trying the dev channel

If you want to run the latest EMOS before it is released, there is a nightly build. Every night the unreleased branch is built and published as a pre-release named `v<version>-dev.<date>`, together with matching `<distro>-dev` container images. The installer takes a flag for it:

```bash
curl -fsSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash -s -- --dev
```

On a machine that already has EMOS, follow that with `emos update`, and the workspace, or the container image, moves to the same nightly.

## Uninstalling

```bash
emos uninstall
```

Run this as your own user rather than with `sudo`; the command escalates on its own for the few steps that need root.

It also removes the plugin workspace at `~/emos/workspace`, your recipes and logs under `~/emos` unless you pass `--keep-data`, and `~/.config/emos` unless you pass `--keep-config`.

A few things are deliberately left alone: the maps in `~/emos/maps`, exported maps in `~/emos/map-archives`, and the robot's certificate and API keys in `~/emos/.ui-security`. The CLI binary is never removed either, the command prints the one-liner for that.

```{tip}
Uninstall before switching modes, for example from native to pixi. It clears the state that would otherwise carry over and confuse the new install.
```

## Installing from Source (Developer Setup)

If you are contributing to the stack, or want it in a workspace of your own, you can build all four packages by hand: **Sugarcoat** (architecture), **EmbodiedAgents** (intelligence), **Kompass** (navigation) and **emos_mapping** (map building). If all you want is the latest stack, the pixi mode above does exactly this for you in an isolated environment.

### 1. Create a workspace

```shell
mkdir -p emos_ws/src
cd emos_ws/src
```

### 2. Clone the stack

```shell
git clone --recurse-submodules https://github.com/automatika-robotics/emos.git
cp -r emos/stack/sugarcoat .
cp -r emos/stack/embodied-agents .
cp -r emos/stack/kompass .
cp -r emos/stack/emos_mapping .
```

### 3. Install Python dependencies

```shell
PIP_BREAK_SYSTEM_PACKAGES=1 pip install numpy opencv-python-headless 'attrs>=23.2.0' jinja2 httpx setproctitle msgpack msgpack-numpy platformdirs tqdm pyyaml toml websockets ollama 'redis[hiredis]' pyaudio soundfile python-fasthtml monsterui
```

### 4. Install the Kompass core engine

::::{tab-set}

:::{tab-item} GPU Support (Recommended)

Builds kompass-core with GPU acceleration for NVIDIA, AMD, Intel and Arm Mali GPUs:

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/kompass-core/refs/heads/main/build_dependencies/install_gpu.sh | bash
```

:::

:::{tab-item} CPU Only

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
