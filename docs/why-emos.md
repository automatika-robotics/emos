# Why EMOS

The robotics industry is undergoing a structural shift. Robots are transitioning from **single-purpose tools** -- hard-coded for fixed tasks -- to **general-purpose platforms** that must perform different jobs in different environments. While the AI industry races to build foundation models, a critical vacuum remains in the infrastructure required to actually ground these models on robots usable in the field.

EMOS fills that vacuum. It is the **missing orchestration layer** between capable hardware and capable AI.

---

## The Problem

Modern robot hardware ships with stable locomotion controllers and basic SDKs, but little else. Getting a robot to actually *do something useful* -- navigate a cluttered warehouse, respond to voice commands, recover from failures -- requires stitching together a fragile patchwork of ROS packages, custom launch files, and one-off scripts. Every new deployment becomes a bespoke R&D project.

This approach has three fatal flaws:

1. **It doesn't scale.** Every new robot, environment, or task requires months of custom engineering.
2. **It doesn't adapt.** Rigid state machines and declarative graphs cannot handle the chaos of the real world -- sensor failures, dynamic obstacles, network drops.
3. **It doesn't transfer.** Software written for one robot rarely works on another, even if the task is identical.

---

## What EMOS Changes

### From Custom Projects to Universal Recipes

EMOS replaces brittle, robot-specific software projects with **Recipes**: reusable, hardware-agnostic application packages written in pure Python. A Recipe is a complete agentic workflow -- perception, reasoning, navigation, memory, and interaction -- defined in a single script and launched with one command.

- {material-regular}`smart_toy;1.2em;sd-text-primary` **One Robot, Many Tasks:** The same robot can run different Recipes for different jobs -- inspection in the morning, delivery at noon, security patrol at night.
- {material-regular}`devices;1.2em;sd-text-primary` **One Recipe, Many Robots:** A Recipe written for a wheeled AMR runs identically on a quadruped. EMOS handles the kinematic translation beneath the surface.

### From Rigid Graphs to Adaptive Agents

Legacy stacks treat failure as a system crash. EMOS treats it as a **control flow state**. Its event-driven architecture lets robots reconfigure themselves at runtime:

- {material-regular}`sync;1.2em;sd-text-primary` Hot-swap ML models when the network drops
- {material-regular}`swap_horiz;1.2em;sd-text-primary` Switch navigation algorithms when the robot gets stuck
- {material-regular}`flash_on;1.2em;sd-text-primary` Trigger recovery maneuvers based on sensor events
- {material-regular}`hub;1.2em;sd-text-primary` Compose complex behaviors with logic gates (AND, OR, NOT) across multiple data streams

This isn't bolted-on error handling -- adaptivity is a **first-class primitive** in the system design.

### From Stateless Tools to Embodied Agents

Current robots have logs, not memory. They record data for post-facto analysis but cannot recall it at runtime. EMOS introduces **embodiment primitives** that give robots a sense of self and history:

- {material-regular}`map;1.2em;sd-text-primary` **Spatio-Temporal Semantic Memory:** A queryable world-state backed by vector databases that persists across tasks.
- {material-regular}`self_improvement;1.2em;sd-text-primary` **Self-Referential State:** Components can inspect and modify each other's configuration, enabling system-level awareness rather than isolated self-repair.

### From CPU Bottlenecks to GPU-Accelerated Navigation

While other stacks use GPUs only for vision, EMOS moves the entire navigation control stack to the GPU. Kompass, the EMOS navigation engine, provides **GPGPU-accelerated kernels** for motion planning and control:

- {material-regular}`speed;1.2em;sd-text-primary` **Up to 3,106x speedup** over CPU-bound stacks for trajectory evaluation
- {material-regular}`grid_on;1.2em;sd-text-primary` **1,850x speedup** for dense occupancy grid mapping
- {material-regular}`memory;1.2em;sd-text-primary` **Vendor-neutral** -- works on NVIDIA, AMD, Intel, and integrated GPUs via SYCL
- {material-regular}`developer_board;1.2em;sd-text-primary` Falls back to optimized process-level parallelism on CPU-only platforms

This enables reactive autonomy in dynamic, unstructured environments where traditional CPU-bound stacks like Nav2 simply cannot keep up.

### From Separate Backends to Auto-Generated Interaction

In traditional robotics, the automation logic is "backend" and the user interface is a separate custom project. EMOS treats the **Recipe as the single source of truth** -- defining the logic automatically generates a bespoke Web UI for real-time monitoring, configuration, and control. No separate frontend development required.

---

## The Architecture

EMOS is built on three open-source components that work in tandem:

:::{image} _static/images/diagrams/emos_diagram_light.png
:align: center
:width: 500px
:class: light-only
:::

:::{image} _static/images/diagrams/emos_diagram_dark.png
:align: center
:width: 500px
:class: dark-only
:::

| Component | Layer | What It Does |
|:---|:---|:---|
| [**EmbodiedAgents**](https://github.com/automatika-robotics/embodied-agents) | Intelligence | Agentic graphs of ML models with semantic memory, information routing, and adaptive reconfiguration |
| [**Kompass**](https://github.com/automatika-robotics/kompass) | Navigation | GPU-powered planning and control for real-world mobility across all motion models |
| [**Sugarcoat**](https://github.com/automatika-robotics/sugarcoat) | Architecture | Event-driven system primitives, lifecycle management, and the imperative launch API that underpins both layers |

Together, they provide a complete runtime: from raw sensor data to intelligent action, with adaptivity and resilience built in at every level.

---

## Who Is EMOS For

### Robot Managers & End-Users

Use pre-built Recipes or write your own with the high-level Python API. Focus on your business logic -- EMOS handles the robotics complexity.

### Integrators & Solution Providers

EMOS is your SDK for the physical world. Connect robot events to ERPs, building management systems, or fleet software using the event-action architecture. Spend your time on enterprise integration, not low-level robotics plumbing.

### OEM Teams

Write a single Hardware Abstraction Layer plugin and instantly unlock the entire EMOS ecosystem for your chassis. Every Recipe written by any developer runs on your hardware without custom code.

---

## Built for the Real World

EMOS is not a research prototype. It is shaped by the demands of production deployments -- autonomous inspection patrols, security operations, and field robotics on quadruped and wheeled platforms. Every feature in the stack exists because a real-world deployment needed it.

---

## Get Started

::::{grid} 1 2 2 2
:gutter: 3

:::{grid-item-card} {material-regular}`rocket_launch;1.2em;sd-text-primary` Install EMOS
:link: getting-started/installation
:link-type: doc

Get up and running in minutes.
:::

:::{grid-item-card} {material-regular}`menu_book;1.2em;sd-text-primary` Browse Recipes
:link: recipes/overview
:link-type: doc

Step-by-step tutorials from simple to production-grade.
:::

::::
