<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/_static/Emos_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/_static/Emos_light.png">
    <img alt="EMOS" src="docs/_static/Emos_light.png" width="50%">
  </picture>
<h2>The Embodied Operating System</h2>

<p><strong>The open-source runtime for Physical AI.</strong><br>
<sub><strong>意默</strong> — 隐入代码，御机有神</sub></p>

<p>
<a href="https://emos.automatikarobotics.com/"><img src="https://img.shields.io/badge/docs-online-blue?style=flat-square" alt="Documentation"></a>
<a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square" alt="License: MIT"></a>
<a href="https://emos.automatikarobotics.com/getting-started/installation.html"><img src="https://img.shields.io/badge/ROS%202-Humble%20|%20Jazzy%20|%20Kilted%20|%20Lyrical-orange?style=flat-square&logo=ros" alt="ROS 2"></a>
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
    <source media="(prefers-color-scheme: dark)" srcset="docs/_static/images/diagrams/emos_any_robot_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/_static/images/diagrams/emos_any_robot_light.png">
    <img alt="Write it once. Run it on any robot." src="docs/_static/images/diagrams/emos_any_robot_light.png" width="100%">
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

That's a complete robot agent. It sees through a camera, reasons with a vision-language model, and responds, all running as a managed ROS2 lifecycle node. A bigger Recipe is the same thing with more components wired together, and with the scenarios you want the robot to handle written next to them:

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/_static/images/diagrams/emos_recipe_graph_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/_static/images/diagrams/emos_recipe_graph_light.png">
    <img alt="A Recipe is one Python file: a graph and its behaviors" src="docs/_static/images/diagrams/emos_recipe_graph_light.png" width="100%">
  </picture>
  <br>
  <sub>Solid arrows show which component feeds which. Dashed red flags show what the robot should do when something happens.</sub>
</p>

## Architecture

EMOS is built on three open-source components that work in tandem:

| Component                                                                    | Layer                         | What It Does                                                                       |
| :--------------------------------------------------------------------------- | :---------------------------- | :--------------------------------------------------------------------------------- |
| [**EmbodiedAgents**](https://github.com/automatika-robotics/embodied-agents) | Intelligence and manipulation | Graphs of ML models with planning, memory, and classical and learned manipulation. |
| [**Kompass**](https://github.com/automatika-robotics/kompass)                | Navigation                    | Cross-vendor GPU-accelerated planning and control.                                 |
| [**Sugarcoat**](https://github.com/automatika-robotics/sugarcoat)            | System primitives             | Lifecycle-managed components and event-driven system design for ROS 2.             |

The three layers form a vertical stack. Sugarcoat provides the execution primitives. Kompass builds navigation nodes on top of them. EmbodiedAgents builds cognitive nodes on the same foundation. At runtime, the **Launcher** brings everything to life from a single Python script, the Recipe.

## Why EMOS

**Hardware-agnostic Recipes.** A security patrol Recipe written for a wheeled AMR runs identically on a quadruped. EMOS handles kinematic translation and action commands beneath the surface.

**Event-driven autonomy.** Robots must adapt to chaos. EMOS enables behavior switching at runtime based on environmental context, not just internal state. If a cloud API disconnects, the Recipe triggers a fallback to a local model. If the navigation controller gets stuck, an event fires a recovery maneuver. Failure is a control flow state, not a crash.

```python
# Cloud API fails? Switch to local backup automatically.
llm.on_algorithm_fail(action=switch_to_backup, max_retries=3)

# Emergency stop? Restart the planner and back away.
launcher.on(event_emergency_stop, [restart(component=planner), unblock_action])
```

**Full agentic operation.** Hand [Cortex](https://emos.automatikarobotics.com/recipes/planning-and-manipulation/cortex-agent.html) a task in plain language and it builds a plan from everything the Recipe can already do, calling on any component along the way, replanning when a step fails and writing the tools it is missing. Each plan runs as a routine you can pause or abort from the web UI.

**GPU-accelerated navigation.** Kompass moves heavy geometric planning to the GPU, [orders of magnitude faster](https://emos.automatikarobotics.com/advanced/benchmarks.html) than CPU-based approaches. It is the first navigation framework with cross-vendor GPU support via SYCL.

**ML models as first-class citizens.** Object detection outputs can trigger controller switches. VLMs can alter planning strategy. Vision components can drive target following. The intelligence and navigation layers are deeply integrated through a shared component model.

**Plugins, drivers and maps included.** A plugin adapts one robot or sensor to the standard interfaces Recipes use, and starts the drivers it needs only when a Recipe asks for them. Ready-made plugins install from the catalog in one command, and where a robot ships no mapping software, `emos map new` builds the site map with the robot's own LiDAR.

**Auto-generated web UIs.** EMOS renders a fully functional web interface directly from your Recipe definition, real-time telemetry, video feeds, and configuration controls with zero frontend code. Underneath it, a JSON and WebSocket API gives scripts, control centers and fleet managers the same access without ROS. Everything is served over HTTPS with the robot's own certificate, browsers are paired once with a code, API clients carry a key, and both can be revoked.

<p align="center">
<img alt="Auto-generated Web UI" src="docs/_static/ui_navigation.gif" width="60%">
</p>

## Installation

Install the EMOS CLI from the latest [release](https://github.com/automatika-robotics/emos/releases):

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

Or build from source (requires Go 1.25+):

```bash
git clone https://github.com/automatika-robotics/emos.git
cd emos/stack/emos-cli
make build && sudo make install
```

Then choose your install mode:

```bash
# pixi mode (recommended): ROS 2 and the whole stack in your home directory, no Docker
emos install --mode pixi

# Native mode: builds on the ROS 2 you already have
emos install --mode native

# Container mode: runs the public image in Docker
emos install --mode container

# Or run without arguments for an interactive menu
emos install
```

**pixi mode** is recommended. It installs ROS 2 Jazzy and every EMOS package into an isolated environment under your home directory, on any Linux distribution, and offers GPU builds where a CUDA toolkit is found.

**Native mode** detects your ROS 2 installation (Humble, Jazzy or Kilted), fetches the EMOS source, installs the dependencies including GPU-accelerated kompass-core, and builds a workspace.

**Container mode** pulls the public EMOS image and runs recipes inside it. Plugins that need sensor drivers, and mapping, want one of the other two modes.

`emos update` keeps any of them current, and a nightly pre-release is available on the dev channel. See the [installation docs](https://emos.automatikarobotics.com/getting-started/installation.html) for details.

### Models

Models can run on the robot itself with no server, through the built-in local models for language, vision, detection, speech-to-text and text-to-speech, or on [Ollama](https://ollama.com), [RoboML](https://github.com/automatika-robotics/robo-ml), [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang), [LeRobot](https://github.com/huggingface/lerobot) and any OpenAI-compatible endpoint.

See the [CLI documentation](stack/emos-cli/README.md) for the full command reference.

## Dashboard

`emos serve` starts the dashboard, a web app on the robot where you browse, install and run recipes, install plugins and manage maps, with live logs. It is served over HTTPS, mDNS publishes the device as `<robot-name>.local` so you can reach it from any laptop or phone on the same network, and pairing is one-time with a six-digit code; access is revocable.

<p align="center">
  <img alt="EMOS Dashboard" src="docs/_static/images/dashboard_overview.png" width="80%">
</p>

See the [Dashboard guide](https://emos.automatikarobotics.com/getting-started/dashboard.html) for a walkthrough.

## Recipes

Recipes are not scripts. They are complete agentic workflows that combine intelligence, navigation, and system orchestration into a single, readable Python file.

**The Agent You Talk To** hands a task in plain language to Cortex, which plans with every capability the Recipe has:

```python
cortex = Cortex(
    output=cortex_output,
    model_client=planner_client,
    config=CortexConfig(max_planning_steps=5, max_execution_steps=10),
)
```

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

The [documentation](https://emos.automatikarobotics.com/recipes/overview.html) includes more than thirty tutorials covering conversational agents, local models, spatio-temporal memory, tool calling, Cortex, VLA and MoveIt manipulation, point navigation and multi-waypoint missions, a real robot through its plugin, vision tracking, multiprocessing, runtime fallbacks, motion detection and event-driven cognition.

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

| Area                             | Repository                                                                           | What belongs there                                       |
| :------------------------------- | :----------------------------------------------------------------------------------- | :------------------------------------------------------- |
| CLI, installation, recipes, docs | [**emos**](https://github.com/automatika-robotics/emos/issues)                       | CLI bugs, recipe issues, deployment, documentation       |
| AI components, model clients     | [**embodied-agents**](https://github.com/automatika-robotics/embodied-agents/issues) | LLM/VLM/VLA components, model clients, speech, vision    |
| Navigation, planning, control    | [**kompass**](https://github.com/automatika-robotics/kompass/issues)                 | Planner, controller, drive manager, mapping, algorithms  |
| Core framework, events, launcher | [**sugarcoat**](https://github.com/automatika-robotics/sugarcoat/issues)             | Components, events/actions, fallbacks, lifecycle, web UI |

If you're unsure, just open it on [emos](https://github.com/automatika-robotics/emos/issues) and we'll route it to the right place.

### Getting started with development

Each package has developer documentation with guides for extending the framework:

- [EmbodiedAgents developer docs](https://agents.automatikarobotics.com) — custom components, clients, models
- [Kompass developer docs](https://kompass.automatikarobotics.com) — algorithms, navigation components, data types
- [Sugarcoat developer docs](https://sugarcoat.automatikarobotics.com) — type system, event internals, testing

## License

MIT. See the [LICENSE](LICENSE) file for details.
