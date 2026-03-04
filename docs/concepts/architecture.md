# Architecture

**The unified orchestration layer for Physical AI.**

EMOS (The Embodied Operating System) is the software layer that transforms quadrupeds, humanoids, and mobile robots into **Physical AI Agents**. Just as Android standardized the smartphone hardware market, EMOS provides a bundled, hardware-agnostic runtime that allows robots to see, think, move, and adapt in the real world.

## The Body/Mind Split

At its core, EMOS decouples the robot's **Body** from its **Mind**, creating a standard interface for intelligence.

- **The Body** encompasses the physical hardware: motors, sensors, actuators, and the low-level drivers that control them. EMOS abstracts over the specifics of any particular robot platform, whether it is a wheeled AMR, a quadruped, or a humanoid.

- **The Mind** is the software intelligence that perceives the world, reasons about it, and decides how to act. EMOS provides the cognitive and navigational primitives that turn raw sensor data into purposeful behavior.

This separation means that the same application logic --- a "Recipe" --- can be written once and deployed across entirely different robot bodies without rewriting code. EMOS handles the translation between intent and hardware.

## The Three Layers

EMOS is built on three open-source, publicly developed core components that work in tandem. Each layer addresses a distinct concern of the robotic software stack.

### Intelligence Layer: EmbodiedAgents

[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) is the orchestration framework for building agentic graphs of ML models. It provides:

- **Multi-modal perception** using vision-language models, object detectors, and speech processing.
- **Hierarchical spatio-temporal memory** for contextual reasoning about the robot's environment over time.
- **Semantic routing** that directs user commands to the correct capability (navigation, vision, conversation) based on intent.
- **Adaptive reconfiguration** that allows the robot to switch between cloud APIs and local models at runtime based on connectivity and latency requirements.

### Navigation Layer: Kompass

[Kompass](https://github.com/automatika-robotics/kompass) is the event-driven navigation stack responsible for real-world mobility. It provides:

- **GPGPU-accelerated planning** that moves heavy geometric computation to the GPU, achieving up to 3,106x speedups over CPU-based approaches and freeing the CPU for application logic.
- **Hardware-agnostic control** that works across wheeled, legged, and tracked platforms.
- **Event-driven architecture** where planners and controllers react to environmental changes (obstacles, terrain shifts, emergency stops) rather than running in fixed polling loops.

### Architecture Layer: Sugarcoat

[Sugarcoat](https://github.com/automatika-robotics/sugarcoat) is the meta-framework that provides the foundational system design primitives on which both EmbodiedAgents and Kompass are built. It provides:

- **Lifecycle-managed Components** that replace standard ROS2 nodes with self-healing, health-aware execution units.
- **An Event-Driven system** that enables dynamic behavior switching based on real-time environmental context.
- **A Launcher and Monitor** that orchestrate multi-process or multi-threaded deployments with automatic lifecycle management.
- **A beautifully imperative Python API** for specifying system configurations as "Recipes" rather than XML launch files.

## How the Layers Work Together

The three layers form a vertical stack where each layer builds on the one below it:

1. **Sugarcoat (Architecture)** provides the execution primitives: Components, Topics, Events, Actions, Fallbacks, and the Launcher. Every node in the system --- whether it handles perception, planning, or control --- is a Sugarcoat Component with lifecycle management, health reporting, and self-healing capabilities.

2. **Kompass (Navigation)** builds on Sugarcoat's Component model to implement specialized navigation nodes: path planners, motion controllers, and drivers. These nodes communicate through Sugarcoat Topics, react to Sugarcoat Events, and recover from failures using Sugarcoat Fallbacks.

3. **EmbodiedAgents (Intelligence)** builds on the same Component model to implement cognitive nodes: vision-language models, semantic routers, and memory systems. These nodes can trigger navigation behaviors in Kompass, respond to navigation events, and share data through the common Topic infrastructure.

At runtime, all three layers are unified by the **Launcher**, which brings the complete system to life in a single Python script --- the Recipe. The Recipe declares which components to run, how they are wired together, what events to monitor, and what actions to take when conditions change. The result is a robot that can see, think, move, and adapt, all orchestrated from one coherent system.

## Recipes: The Developer Interface

A Recipe is a standard Python script that uses the EMOS API to declare an entire robotic application. Recipes are not just scripts; they are complete agentic workflows that combine intelligence, navigation, and system orchestration into a single, readable specification.

```python
from ros_sugar import Launcher
from ros_sugar.core import Event, Action
from ros_sugar.io import Topic

# Define components from any EMOS layer
# ... intelligence components from EmbodiedAgents
# ... navigation components from Kompass
# ... custom components built on Sugarcoat

# Wire them together with Topics, Events, and Actions
# Launch everything with a single call
launcher = Launcher(multi_processing=True)
launcher.add_pkg(components=[...], events_actions={...})
launcher.bringup()
```

This imperative, Pythonic approach replaces the traditional ROS2 workflow of XML launch files and YAML configurations with a single source of truth that is easy to read, version, and share.
