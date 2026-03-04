# README.md

<div align="center">
<h1>EMOS</h1>
<h3>The Embodied Operating System</h3>

<p>
<strong>The unified orchestration layer for Physical AI.</strong>
</p>

<p>
<a href="#the-platform">The Platform</a> •
<a href="#architecture">Architecture</a> •
<a href="#features">Features</a> •
<a href="#developer-experience">Developer Experience</a>
</p>
</div>

---

<div align="center">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="/_static/diagrams/robot_stack_dark.png">
<source media="(prefers-color-scheme: light)" srcset="/_static/diagrams/robot_stack_light.png">
<img alt="EMOS in the robot software stack" src="/_static/diagrams/robot_stack_light.png" width="80%">
</picture>
</div>

**EMOS** is the software layer that transforms quadrupeds, humanoids, and mobile robots into **Physical AI Agents**.

Just as Android standardized the smartphone hardware market, EMOS provides a bundled, hardware-agnostic runtime that allows robots to **see**, **think**, **move**, and **adapt** in the real world. It transitions robotics from "custom R&D projects" to a world of standardized, deployable assets.

## The Platform

EMOS decouples the robot's "Body" from its "Mind", creating a standard interface for intelligence:

* **The Runtime:** A bundled stack combining **Kompass** (GPGPU-accelerated navigation) with **EmbodiedAgents** (cognitive reasoning). It turns a pile of motors and sensors into an intelligent agent out-of-the-box.
* **The Ecosystem:** A development framework that allows engineers to write **"Recipes"** (Apps) once using a simple Python API, and deploy them across any robot, from wheeled AMRs to humanoids, without rewriting code.

## What is a Physical AI Agent?

A Physical AI Agent is distinct from a disembodied digital agent (like a coding assistant). It combines intelligence with real-time physical adaptivity. EMOS provides the primitives to:

1. **See & Understand:** Interpret the world with multi-modal ML models.
2. **Think & Remember:** Use spatio-temporal semantic memory and contextual reasoning.
3. **Move & Manipulate:** Execute GPU-powered navigation and VLA-based manipulation.
4. **Adapt in Real Time:** Reconfigure logic at runtime based on environmental events.

## <a id="architecture"></a>Architecture

EMOS is built on open-source, publicly developed core components that work in tandem.

<div align="center">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="/_static/diagrams/emos_diagram_dark.png">
<source media="(prefers-color-scheme: light)" srcset="/_static/diagrams/emos_diagram_light.png">
<img alt="The Embodied Operating System Architecture" src="/_static/diagrams/emos_diagram_light.png" width="60%">
</picture>
</div>

| Component | Layer | Function |
| --- | --- | --- |
| **[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents)** | **The Intelligence Layer** | The orchestration framework for building agentic graphs of ML models, hierarchical spatio-temporal memory, and adaptive reconfiguration. |
| **[Kompass](https://github.com/automatika-robotics/kompass)** | **The Navigation Layer** | The event-driven navigation stack responsible for GPU-powered planning and control for real-world mobility on any hardware. |
| **[Sugarcoat](https://github.com/automatika-robotics/sugarcoat)** | **The Architecture Layer** | The meta-framework providing event-driven system design primitives and a beautifully imperative system specification API. |

## <a id="features"></a>Key Capabilities

EMOS unlocks the full potential of robotic hardware by shifting the paradigm from "Scripts" to "Apps".

### 1. From Custom Code to Universal "Recipes"

EMOS replaces brittle, task-specific ROS projects with **Recipes**: reusable, hardware-agnostic application packages.

* **Write Once, Run Anywhere:** A "Security Patrol" recipe written for a wheeled robot runs identically on a quadruped. EMOS handles the kinematics and action commands.
* **Multi-Tasking:** The same robot can run multiple recipes, switching between them as needed.

### 2. Real-World Event-Driven Autonomy

Robots must adapt to chaos. EMOS enables dynamic behavior switching based on context.

* **Imperative Fallback Logic:** Developers can treat failure as a control flow state. If a navigation controller gets stuck, or a cloud model disconnects, the Recipe can trigger specific recovery functions (like switching to a local LLM) rather than crashing.

### 3. Auto-Generated Interaction UIs

In EMOS, the automation logic *is* the interface.

* **Schema-Driven Dashboards:** EMOS automatically renders a fully functional web-based UI directly from your Recipe definition.
* **Composable Widgets:** Real-time telemetry, video feeds, and configuration controls are generated as standard web components, ready to be embedded in fleet management portals.

### 4. Hybrid Compute & GPU Acceleration

* **Hybrid AI Inference:** Seamlessly switch between edge NPUs for low-latency perception and Cloud APIs for high-level reasoning.
* **GPU Navigation:** The industry's only navigation engine that moves heavy geometric planning to the GPU (up to **3,106x speedups**), freeing the CPU for application logic.

---

## <a id="developer-experience"></a>Developer Experience

EMOS is designed to commoditize robotics software. Whether you are an **End-User** scripting a quick task, an **Integrator** building complex enterprise workflows, or an **OEM** enabling your hardware, EMOS provides the right abstraction.

### Recipe Examples

Recipes are not just scripts; they are complete agentic workflows. Below are real-world examples of EMOS in action.

#### 1. The General Purpose Assistant

*A robot that intelligently routes verbal commands to the correct capability.*

This recipe uses a **Semantic Router** to analyze user intent. "Go to the kitchen" routes to Kompass (Navigation), while "What tool is this?" routes to a VLM (Vision).

```python
# Define routing logic based on semantic meaning, not just keywords
llm_route = Route(samples=["What is the torque for M6?", "Convert inches to mm"])
mllm_route = Route(samples=["What tool is this?", "Is the safety light on?"])
goto_route = Route(samples=["Go to the CNC machine", "Move to storage"])

# The Semantic Router directs traffic based on intent
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route, mllm_route], # Routes to Chat, Nav, or Vision
    default_route=llm_route,
    config=router_config
)

```

#### 2. The Resilient "Always-On" Agent

*Ensuring uptime by falling back to local compute when the internet fails.*

This demonstrates **Runtime Robustness**. We bind an `on_algorithm_fail` event to the intelligence component.

```python
# If the cloud API fails (runtime), instantly switch to the local backup model
llm_component.on_algorithm_fail(
    action=switch_to_backup,
    max_retries=3
)

```

#### 3. The Self-Recovering Warehouse Robot

*A robot that unjams itself without human intervention.*

Instead of a "Red Light" error, the robot uses an event/action pair to trigger a specific maneuvering routine when the planner gets stuck.

```python
events_actions = {
    # If the emergency stop triggers, restart the planner and back away
    event_emergency_stop: [
        ComponentActions.restart(component=planner),
        unblock_action,
    ],
    # If the controller algorithm fails, attempt unblocking maneuver
    event_controller_fail: unblock_action,
}

```

#### 4. The "Off-Grid" Field Mule

*Follow-me functionality in unmapped environments.*

Using the `VisionRGBDFollower`, the robot fuses depth data with visual detection to "lock on" to a human guide, ignoring the need for GPS or SLAM.

```python
# Setup controller to use Depth + Vision Fusion
controller.inputs(
    vision_detections=detections_topic,
    depth_camera_info=depth_cam_info_topic
)
controller.algorithm = "VisionRGBDFollower"

```

---

<div align="center">
<p>
Copyright © 2026 <a href="[https://automatikarobotics.com/](https://automatikarobotics.com/)">Automatika Robotics</a>.
</p>
</div>
