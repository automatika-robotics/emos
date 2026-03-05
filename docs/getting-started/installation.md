# Installation

## EMOS CLI

The fastest way to get started with EMOS is through the CLI. Open your terminal and run:

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos-cli/main/install.sh | sudo bash
```

This downloads the official installer, which checks for dependencies (`gum`, `curl`, `jq`), fetches the latest `emos` executable, and places it in `/usr/local/bin` so it is available system-wide.

Once installed, set up your environment:

```bash
emos install YOUR-LICENSE-KEY-HERE
```

See the [CLI Reference](cli.md) for the full list of commands.

## Prerequisites

:::{admonition} ROS 2 Required
:class: note
EMOS requires a working **ROS 2** installation, **Iron or later** (up to Rolling).
Please ensure you have [ROS 2 installed](https://docs.ros.org/) before proceeding.
:::

## Model Serving Platform

EMOS is agnostic to model serving platforms. You need at least one of the following available on your network:

- {material-regular}`download;1.2em;sd-text-primary` **[Ollama](https://ollama.com)** -- Recommended for local inference.
- {material-regular}`smart_toy;1.2em;sd-text-primary` **[RoboML](https://github.com/automatika-robotics/robo-ml)** -- Automatika's own model serving layer.
- {material-regular}`api;1.2em;sd-text-primary` **OpenAI API-compatible servers** -- e.g. [llama.cpp](https://github.com/ggml-org/llama.cpp), [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang).
- {material-regular}`precision_manufacturing;1.2em;sd-text-primary` **[LeRobot](https://github.com/huggingface/lerobot)** -- For Vision-Language-Action (VLA) models.
- {material-regular}`cloud;1.2em;sd-text-primary` **Cloud endpoints** -- HuggingFace Inference Endpoints, OpenAI, etc.

```{tip}
For larger models, run the serving platform on a GPU-equipped machine on your local network rather than directly on the robot.
```

## Installing from Source (Developer Setup)

If you want to build the full EMOS stack from source -- for contributing or accessing the latest features -- follow the steps below. This installs all three stack components: **Sugarcoat** (architecture), **EmbodiedAgents** (intelligence), and **Kompass** (navigation).

### 1. Create a unified workspace

```shell
mkdir -p emos_ws/src
cd emos_ws/src
```

### 2. Install Python dependencies

```shell
pip install numpy opencv-python-headless 'attrs>=23.2.0' jinja2 httpx setproctitle msgpack msgpack-numpy platformdirs tqdm pyyaml toml websockets
```

### 3. Clone the stack

```shell
# Sugarcoat -- the component architecture layer
git clone https://github.com/automatika-robotics/sugarcoat

# EmbodiedAgents -- intelligence components
git clone https://github.com/automatika-robotics/embodied-agents

# Kompass -- navigation engine
git clone https://github.com/automatika-robotics/kompass
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
