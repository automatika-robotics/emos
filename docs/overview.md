# EMOS — The Embodied Operating System

**The unified orchestration layer for Physical AI.**

**EMOS** is the software layer that transforms quadrupeds, humanoids, and mobile robots into **Physical AI Agents**. Just as Android standardized the smartphone hardware market, EMOS provides a bundled, hardware-agnostic runtime that allows robots to **see**, **think**, **move**, and **adapt** in the real world. It transitions robotics from "custom R&D projects" to a world of standardized, deployable assets.

## The Platform

EMOS decouples the robot's "Body" from its "Mind", creating a standard interface for intelligence:

- **The Runtime:** A bundled stack combining GPU-accelerated navigation with cognitive reasoning and manipulation. It turns a pile of motors and sensors into an intelligent agent out-of-the-box.

- **The Ecosystem:** A development framework that allows engineers to write **Recipes** (Apps) once using a simple Python API, and deploy them across any robot — from wheeled AMRs to humanoids — without rewriting code.

## What is a Physical AI Agent?

A Physical AI Agent is distinct from a disembodied digital agent (like a coding assistant). It combines intelligence with real-time physical adaptivity. EMOS provides the primitives to:

1. **See & Understand:** Interpret the world with multi-modal ML models.
2. **Think & Remember:** Use spatio-temporal semantic memory and contextual reasoning.
3. **Move & Manipulate:** Execute GPU-powered navigation and VLA-based manipulation.
4. **Adapt in Real Time:** Reconfigure logic at runtime based on environmental events.

## What's Inside EMOS?

EMOS is built on open-source, publicly developed core components that work in tandem.

| Component | Layer | Function |
| :--- | :--- | :--- |
| **[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents)** | **The Intelligence Layer** | Orchestration framework for building agentic graphs of ML models, with hierarchical spatio-temporal memory, information routing, and event-driven adaptive reconfiguration. |
| **[Kompass](https://github.com/automatika-robotics/kompass)** | **The Navigation Layer** | Event-driven navigation stack responsible for GPU-powered planning and control for real-world mobility on any hardware. |
| **[Sugarcoat](https://github.com/automatika-robotics/sugarcoat)** | **The Architecture Layer** | Meta-framework providing event-driven system design primitives and a beautifully imperative system specification and launch API. |

## Key Capabilities

### From Custom Code to Universal Recipes

EMOS replaces brittle, task-specific projects with **Recipes**: reusable, hardware-agnostic application packages.

- **Write Once, Run Anywhere:** A "Security Patrol" recipe written for a wheeled robot runs identically on a quadruped. EMOS handles the kinematics and action commands.
- **Multi-Tasking:** The same robot can run multiple recipes, switching between them as needed.

### Real-World Event-Driven Autonomy

Robots must adapt to chaos. EMOS enables dynamic behavior switching based on context.

- **Imperative Fallback Logic:** Treat failure as a control flow state. If a navigation controller gets stuck, or a cloud model disconnects, the Recipe triggers specific recovery functions rather than crashing.

### Auto-Generated Interaction UIs

In EMOS, the automation logic *is* the interface.

- **Schema-Driven Dashboards:** EMOS automatically renders a fully functional web-based UI directly from your Recipe definition.
- **Composable Widgets:** Real-time telemetry, video feeds, and configuration controls are generated as standard web components.

### Hybrid Compute & GPU Acceleration

- **Hybrid AI Inference:** Seamlessly switch between edge NPUs for low-latency perception and Cloud APIs for high-level reasoning.
- **GPU Navigation:** The industry's only navigation engine that moves heavy geometric planning to the GPU (up to **3,106x speedups**), freeing the CPU for application logic.

---

::::{grid} 1 2 3 3
:gutter: 3

:::{grid-item-card} Getting Started
:link: getting-started/installation
:link-type: doc

Install EMOS and run your first Recipe in minutes.
:::

:::{grid-item-card} Core Concepts
:link: concepts/architecture
:link-type: doc

Understand the architecture, components, events, and fallbacks.
:::

:::{grid-item-card} Recipes & Tutorials
:link: recipes/overview
:link-type: doc

Build intelligent robot behaviors with step-by-step guides.
:::

::::
