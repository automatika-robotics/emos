# EMOS -- The Embodied Operating System

**The unified orchestration layer for Physical AI.**

EMOS transforms robots into **Physical AI Agents**. It provides a hardware-agnostic runtime that lets robots **see**, **think**, **move**, and **adapt** -- all orchestrated from pure Python scripts called **Recipes**.

Write a Recipe once, deploy it on any robot -- from wheeled AMRs to humanoids -- without rewriting code.

::::{tab-set}

:::{tab-item} Build Agents

Wire together vision, language, navigation, and manipulation components using a simple Python API. EMOS handles the ROS2 lifecycle, inter-process communication, and hardware abstraction.

```python
vlm = VLM(inputs=[image, text], outputs=[response], model_client=client)
launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

[Get started](getting-started/quickstart)
:::

:::{tab-item} Navigate Autonomously

GPU-accelerated planning and control for real-world mobility. Swap between DWA, Pure Pursuit, and vision-based tracking algorithms -- or bring your own.

```python
controller = ComponentConfig.controller_node(robot)
controller.algorithms = {"DWA": DWAConfig()}
```

[Explore navigation](navigation/overview)
:::

:::{tab-item} Adapt at Runtime

Event-driven architecture lets agents reconfigure themselves. Hot-swap models, trigger fallbacks on failure, or switch behaviors based on sensor events -- all without restarting.

```python
llm.on_algorithm_fail(action=switch_to_backup, max_retries=3)
```

[Learn about events](concepts/events-and-actions)
:::

:::{tab-item} Deploy Anywhere

The `emos` CLI packages and runs Recipes inside a managed Docker container. Pull official recipes or write your own -- one command to launch.

```bash
emos pull vision_follower
emos run vision_follower
```

[CLI guide](getting-started/cli)
:::

::::

## What's Inside

EMOS is built on three open-source components:

| Component | Role |
| :--- | :--- |
| **[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents)** | Intelligence layer -- agentic graphs of ML models with semantic memory and event-driven reconfiguration |
| **[Kompass](https://github.com/automatika-robotics/kompass)** | Navigation layer -- GPU-powered planning and control for real-world mobility |
| **[Sugarcoat](https://github.com/automatika-robotics/sugarcoat)** | Architecture layer -- event-driven system primitives and imperative launch API |

---

::::{grid} 1 2 3 3
:gutter: 3

:::{grid-item-card} {material-regular}`rocket_launch;1.2em;sd-text-primary` Getting Started
:link: getting-started/installation
:link-type: doc

Install EMOS and run your first Recipe in minutes.
:::

:::{grid-item-card} {material-regular}`menu_book;1.2em;sd-text-primary` Recipes & Tutorials
:link: recipes/overview
:link-type: doc

Build intelligent robot behaviors with step-by-step guides.
:::

:::{grid-item-card} {material-regular}`architecture;1.2em;sd-text-primary` Core Concepts
:link: concepts/architecture
:link-type: doc

Understand the architecture, components, events, and fallbacks.
:::

:::{grid-item-card} {material-regular}`psychology;1.2em;sd-text-primary` Intelligence Layer
:link: intelligence/overview
:link-type: doc

EmbodiedAgents -- agentic graphs of ML models with semantic memory and adaptive reconfiguration.
:::

:::{grid-item-card} {material-regular}`route;1.2em;sd-text-primary` Navigation Layer
:link: navigation/overview
:link-type: doc

Kompass -- GPU-accelerated planning and control for real-world mobility.
:::

:::{grid-item-card} {material-regular}`smart_toy;1.2em;sd-text-primary` AI-Assisted Coding
:link: llms.txt

Get the `llms.txt` for your coding agent and let it write recipes for you.
:::

::::
