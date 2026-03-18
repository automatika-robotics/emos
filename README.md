<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/_static/Emos_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/_static/Emos_light.png">
    <img alt="EMOS" src="docs/_static/Emos_light.png" width="50%">
  </picture>
<h2>The Embodied Operating System</h2>

<p><strong>The open-source unified orchestration layer for Physical AI.</strong><br>
<sub>面向具身智能的开源统一编排层</sub></p>

<p>
<a href="https://emos.automatikarobotics.com/"><img src="https://img.shields.io/badge/docs-online-blue?style=flat-square" alt="Documentation"></a>
<a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square" alt="License: MIT"></a>
<a href="https://emos.automatikarobotics.com/getting-started/installation.html"><img src="https://img.shields.io/badge/ROS%202-Humble%20|%20Iron%20|%20Jazzy%20|%20Kilted%20|%20Rolling-orange?style=flat-square&logo=ros" alt="ROS 2"></a>
</p>

<p>
<a href="https://emos.automatikarobotics.com/">Documentation</a> &middot;
<a href="https://emos.automatikarobotics.com/getting-started/installation.html">Installation</a> &middot;
<a href="https://emos.automatikarobotics.com/getting-started/quickstart.html">Quick Start</a> &middot;
<a href="https://emos.automatikarobotics.com/recipes/overview.html">Recipes</a> &middot;
<a href="https://discord.gg/B9ZU6qjzND">Discord</a>
</p>

</div>

---

EMOS is the open-source software layer that transforms quadrupeds, humanoids, and mobile robots into **Physical AI Agents**. It decouples the robot's body from its mind, providing a hardware-agnostic runtime that lets robots see, think, move, and adapt, orchestrated entirely from simple Python scripts called **Recipes**.

Write a Recipe once. Deploy it on any robot. No code changes.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/_static/images/diagrams/emos_robot_stack_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/_static/images/diagrams/emos_robot_stack_light.png">
    <img alt="EMOS Robot Stack" src="docs/_static/images/diagrams/emos_robot_stack_light.png" width="70%">
  </picture>
</p>

```python
from agents.clients.ollama import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel
from agents.ros import Topic, Launcher

text_in  = Topic(name="text0", msg_type="String")
image_in = Topic(name="image_raw", msg_type="Image")
text_out = Topic(name="text1", msg_type="String")

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

That's a complete robot agent. It sees through a camera, reasons with a vision-language model, and responds, all running as a managed ROS2 lifecycle node.

## Architecture

EMOS is built on three open-source components that work in tandem:

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/_static/images/diagrams/emos_diagram_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/_static/images/diagrams/emos_diagram_light.png">
    <img alt="EMOS Architecture" src="docs/_static/images/diagrams/emos_diagram_light.png" width="45%">
  </picture>
</p>

| Component | Layer | What It Does |
| :--- | :--- | :--- |
| [**EmbodiedAgents**](https://github.com/automatika-robotics/embodied-agents) | Intelligence & Manipulation | Agentic graphs of ML models with semantic memory, multi-modal perception, manipulation, and event-driven adaptive reconfiguration. |
| [**Kompass**](https://github.com/automatika-robotics/kompass) | Navigation | GPU-accelerated planning and control for real-world mobility. Cross-vendor SYCL support, runs on Nvidia, AMD, Intel, and others. |
| [**Sugarcoat**](https://github.com/automatika-robotics/sugarcoat) | System Primitives | Lifecycle-managed components, event-driven system design, and a Pythonic launch API that replaces XML configuration. |

The three layers form a vertical stack. Sugarcoat provides the execution primitives. Kompass builds navigation nodes on top of them. EmbodiedAgents builds cognitive nodes on the same foundation. At runtime, the **Launcher** brings everything to life from a single Python script, the Recipe.

## Why EMOS

**Hardware-agnostic Recipes.** A security patrol Recipe written for a wheeled AMR runs identically on a quadruped. EMOS handles kinematic translation and action commands beneath the surface.

**Event-driven autonomy.** Robots must adapt to chaos. EMOS enables behavior switching at runtime based on environmental context, not just internal state. If a cloud API disconnects, the Recipe triggers a fallback to a local model. If the navigation controller gets stuck, an event fires a recovery maneuver. Failure is a control flow state, not a crash.

```python
# Cloud API fails? Switch to local backup automatically.
llm.on_algorithm_fail(action=switch_to_backup, max_retries=3)

# Emergency stop? Restart the planner and back away.
events_actions = {
    event_emergency_stop: [
        ComponentActions.restart(component=planner),
        unblock_action,
    ],
}
```

**GPU-accelerated navigation.** Kompass moves heavy geometric planning to the GPU, achieving up to **3,106x speedups** over CPU-based approaches. It is the first navigation framework with cross-vendor GPU support via SYCL.

**ML models as first-class citizens.** Object detection outputs can trigger controller switches. VLMs can alter planning strategy. Vision components can drive target following. The intelligence and navigation layers are deeply integrated through a shared component model.

**Auto-generated web UIs.** EMOS renders a fully functional web dashboard directly from your Recipe definition, real-time telemetry, video feeds, and configuration controls with zero frontend code.

<p align="center">
<img alt="Auto-generated Web UI" src="docs/_static/ui_navigation.gif" width="60%">
</p>

## Installation

Install the EMOS CLI from the latest [release](https://github.com/automatika-robotics/emos/releases):

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

Or build from source (requires Go 1.23+):

```bash
git clone https://github.com/automatika-robotics/emos.git
cd emos/stack/emos-cli
make build && sudo make install
```

Then choose your deployment mode:

```bash
# Container mode (no ROS required, runs in Docker)
emos install --mode container

# Native mode (requires existing ROS 2 installation)
emos install --mode native

# Or run without arguments for an interactive menu
emos install
```

**Container mode** pulls the public EMOS image from GHCR and runs it in Docker. Sensor drivers must be running externally.

**Native mode** detects your ROS 2 installation, fetches the EMOS source, installs all dependencies (including GPU-accelerated kompass-core), and builds a workspace at `~/emos/ros_ws`.

You also need a model serving platform. EMOS supports [Ollama](https://ollama.com), [RoboML](https://github.com/automatika-robotics/robo-ml), [vLLM](https://github.com/vllm-project/vllm), [LeRobot](https://github.com/huggingface/lerobot), and any OpenAI-compatible endpoint.

See the [CLI documentation](stack/emos-cli/README.md) for the full command reference.

## Recipes

Recipes are not scripts. They are complete agentic workflows that combine intelligence, navigation, and system orchestration into a single, readable Python file.

**The General-Purpose Assistant** routes verbal commands to the right capability using semantic intent:

```python
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route, vision_route],
    default_route=llm_route,
    config=router_config
)
```

**The Vision-Guided Follower** fuses depth and RGB to track a human guide without GPS or SLAM:

```python
controller.inputs(
    vision_detections=detections_topic,
    depth_camera_info=depth_cam_info_topic
)
controller.algorithm = "VisionRGBDFollower"
```

The [documentation](https://emos.automatikarobotics.com/recipes/overview.html) includes 16 recipes covering conversational agents, prompt engineering, semantic mapping, tool calling, VLA manipulation, point navigation, vision tracking, multiprocessing, runtime fallbacks, and event-driven cognition.

## AI-Assisted Recipe Development

EMOS publishes an [`llms.txt`](https://emos.automatikarobotics.com/llms.txt), a structured context file designed for AI coding agents. Feed it to your preferred LLM and let it write Recipes for you.

## Running Recipes

With the EMOS CLI:

```bash
emos recipes              # Browse available recipes
emos pull vision_follower  # Download one
emos run vision_follower   # Launch it
```

Custom recipes go in `~/emos/recipes/<name>/` with a `recipe.py` and `manifest.json`. See the [CLI guide](https://emos.automatikarobotics.com/getting-started/cli.html) for details.

## Contributing

EMOS has been developed in collaboration between [Automatika Robotics](https://automatikarobotics.com/) and [Inria](https://inria.fr/). Contributions from the community are welcome.

### Where to open issues

EMOS is a monorepo that integrates three independently developed packages. You can open issues here or directly in the relevant repository:

| Area | Repository | What belongs there |
| :--- | :--- | :--- |
| CLI, installation, recipes, docs | [**emos**](https://github.com/automatika-robotics/emos/issues) | CLI bugs, recipe issues, deployment, documentation |
| AI components, model clients | [**embodied-agents**](https://github.com/automatika-robotics/embodied-agents/issues) | LLM/VLM/VLA components, model clients, speech, vision |
| Navigation, planning, control | [**kompass**](https://github.com/automatika-robotics/kompass/issues) | Planner, controller, drive manager, mapping, algorithms |
| Core framework, events, launcher | [**sugarcoat**](https://github.com/automatika-robotics/sugarcoat/issues) | Components, events/actions, fallbacks, lifecycle, web UI |

If you're unsure, just open it on [emos](https://github.com/automatika-robotics/emos/issues) and we'll route it to the right place.

### Getting started with development

Each package has developer documentation with guides for extending the framework:

- [EmbodiedAgents developer docs](https://agents.automatikarobotics.com) — custom components, clients, models
- [Kompass developer docs](https://kompass.automatikarobotics.com) — algorithms, navigation components, data types
- [Sugarcoat developer docs](https://sugarcoat.automatikarobotics.com) — type system, event internals, testing

## License

MIT. See the [LICENSE](LICENSE) file for details.
