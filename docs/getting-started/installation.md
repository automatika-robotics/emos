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

:::{tab-item} Container (Recommended)

No ROS 2 installation required. Runs EMOS inside a Docker container using the public image.

```bash
emos install --mode container
```

You will be prompted to select a ROS 2 distribution (Jazzy, Humble, or Kilted). The CLI pulls the image, creates the container, and sets up the `~/emos/` directory structure.

**Requirements:** Docker installed and running.

**Sensors:** Must be running externally (on the host or in separate containers). The EMOS container uses `--network host` so it can see ROS 2 topics from the host network.

:::

:::{tab-item} Native

Installs EMOS packages directly into a local ROS 2 workspace. No container needed.

```bash
emos install --mode native
```

The CLI will:

1. Detect your ROS 2 installation
2. Clone the EMOS source into `~/emos/ros_ws/src/`
3. Fetch localization dependencies (angles, geographic_info, robot_localization)
4. Install system packages (portaudio, GeographicLib, rmw-zenoh)
5. Install Python dependencies
6. Install kompass-core with GPU acceleration support
7. Run `rosdep install` and `colcon build`

**Requirements:** A working ROS 2 installation (Humble, Jazzy, or Kilted).

**Sensors:** Must be running on the host before launching recipes.

:::

::::

See the [CLI Reference](cli.md) for the full list of commands.

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
- **Native mode:** pulls the latest source, updates dependencies, and rebuilds the workspace.
- **Licensed mode:** re-validates the license, redeploys configuration files, and pulls the latest image.

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
