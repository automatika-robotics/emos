# Installation

## EMOS CLI

The fastest way to get started with EMOS is through the CLI. Download the latest release:

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

Or build from source (requires Go 1.23+):

```bash
git clone https://github.com/automatika-robotics/emos.git
cd emos/stack/emos-cli
make build
sudo make install
```

## Deployment Modes

EMOS supports three deployment modes. Run `emos install` without arguments for an interactive menu, or use the `--mode` flag directly.

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

After installation, EMOS packages are available whenever you source ROS2. You can run recipes directly:

```bash
source /opt/ros/jazzy/setup.bash
python3 ~/emos/recipes/my_recipe/recipe.py
```

**Requirements:** A working ROS2 installation (Humble, Jazzy, or Kilted).

:::

:::{tab-item} pixi (Experimental)

```{note}
Currently available only with **ROS2 Jazzy**.
```

Installs ROS2 and all EMOS dependencies into an isolated userspace environment using [pixi](https://pixi.sh). No root privileges, no Docker, no pre-installed ROS2 required. Works on any Linux distribution.

```bash
# Install pixi
curl -fsSL https://pixi.sh/install.sh | bash

# Clone EMOS and install
git clone --recurse-submodules https://github.com/automatika-robotics/emos.git
cd emos
pixi install
pixi run setup
```

This pulls ROS2 Jazzy and all dependencies as pre-built packages from [RoboStack](https://robostack.github.io/) and conda-forge, installs kompass-core with GPU acceleration, then builds the EMOS packages with colcon.

To enter the environment and run recipes:

```bash
pixi shell
source install/setup.sh
python3 ~/emos/recipes/my_recipe/recipe.py
```

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
ros2 run usb_cam usb_cam_node_exe
```

If you place a launch file at `~/emos/robot/launch/bringup_robot.py`, the CLI will start it automatically when you run `emos run`.

:::

:::{tab-item} pixi

Pixi mode assumes you have **no system ROS2 installation**, so sensor drivers are installed into a pixi environment as well — either your EMOS environment or a separate one dedicated to the driver. Packages are pulled from [RoboStack](https://robostack.github.io/) and conda-forge.

**Option A: install the driver into your EMOS pixi environment**

From the same `emos` directory you cloned during installation:

```bash
pixi add ros-jazzy-usb-cam
pixi shell
ros2 run usb_cam usb_cam_node_exe
```

**Option B: run the driver in a separate pixi environment**

Useful if you want to keep the driver isolated or run it on a different machine:

```bash
mkdir ~/sensors && cd ~/sensors
pixi init
pixi project channel add https://prefix.dev/robostack-jazzy
pixi add ros-jazzy-ros-base ros-jazzy-usb-cam ros-jazzy-rmw-zenoh-cpp
pixi shell
export RMW_IMPLEMENTATION=rmw_zenoh_cpp
ros2 run usb_cam usb_cam_node_exe
```

Either way, because both EMOS and the driver use Zenoh as the default RMW, the driver's topics are visible to recipes running in the EMOS pixi environment automatically; no host-side ROS2 installation required.

```{tip}
If a specific driver package isn't available on RoboStack, you can still install it from source into the pixi environment using `colcon`, or fall back to Native mode for that driver only.
```

:::

::::

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
- {material-regular}`smart_toy;1.2em;sd-text-primary` **[RoboML](https://github.com/automatika-robotics/robo-ml)** Automatika's own model serving layer.
- {material-regular}`api;1.2em;sd-text-primary` **OpenAI API-compatible servers** e.g. [llama.cpp](https://github.com/ggml-org/llama.cpp), [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang).
- {material-regular}`precision_manufacturing;1.2em;sd-text-primary` **[LeRobot](https://github.com/huggingface/lerobot)** For Vision-Language-Action (VLA) models.
- {material-regular}`cloud;1.2em;sd-text-primary` **Cloud endpoints** HuggingFace Inference Endpoints, OpenAI, etc.

```{tip}
For larger models, run the serving platform on a GPU-equipped machine on your local network rather than directly on the robot.
```

## Updating

Update your installation to the latest version:

```bash
emos update
```

The CLI detects your installation mode and updates accordingly:

- **Container mode:** pulls the latest image and recreates the container.
- **Native mode:** pulls the latest source, rebuilds, and re-installs packages into `/opt/ros/{distro}/`.

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
