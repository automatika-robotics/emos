# Why EMOS

The robotics industry is undergoing a structural shift. Robots are moving from **single-purpose tools**, hard-coded for fixed tasks, to **general-purpose platforms** that must perform different jobs in different environments. While the AI industry races to build foundation models, a critical vacuum remains in the infrastructure required to ground those models on robots that work in the field.

EMOS fills that vacuum. It is <span class="text-red-strong">the runtime for Physical AI</span>: the layer that turns capable hardware and capable models into a robot that does a job.

---

## The Problem

Modern robot hardware ships with stable locomotion controllers and basic SDKs, but little else. Getting a robot to actually *do something useful*, navigate a cluttered warehouse, respond to voice commands, recover from failures, means stitching together a fragile patchwork of ROS packages, custom launch files and one-off scripts. Every new deployment becomes a bespoke R&D project.

This approach has three fatal flaws:

1. **It doesn't scale.** Every new robot, environment or task requires months of custom engineering.
2. **It doesn't adapt.** Rigid state machines and declarative graphs cannot handle the chaos of the real world: sensor failures, dynamic obstacles, network drops.
3. **It doesn't transfer.** Software written for one robot rarely works on another, even when the task is identical.

---

## What EMOS Changes

### <span class="sd-text-primary">From Custom Projects to Universal Recipes</span>

EMOS replaces brittle, robot-specific software projects with **Recipes**: reusable, hardware-agnostic application packages written in pure Python. A Recipe is a complete agentic workflow, perception, reasoning, navigation, manipulation, memory and interaction, defined in a single script and launched with one command.

- {material-regular}`smart_toy;1.2em;sd-text-primary` **One Robot, Many Tasks:** The same robot runs different Recipes for different jobs: inspection in the morning, delivery at noon, security patrol at night.
- {material-regular}`devices;1.2em;sd-text-primary` **One Recipe, Many Robots:** A Recipe written for a wheeled AMR runs identically on a quadruped. The robot's plugin handles the translation beneath the surface.

### <span class="sd-text-primary">From Rigid Graphs to Adaptive Agents</span>

Legacy stacks treat failure as a system crash. EMOS treats it as a **control flow state**. Its event-driven architecture lets robots reconfigure themselves at runtime:

- {material-regular}`sync;1.2em;sd-text-primary` Hot-swap ML models when the network drops
- {material-regular}`swap_horiz;1.2em;sd-text-primary` Switch navigation algorithms when the robot gets stuck
- {material-regular}`flash_on;1.2em;sd-text-primary` Trigger recovery maneuvers based on sensor events
- {material-regular}`hub;1.2em;sd-text-primary` Compose complex behaviors with logic gates (AND, OR, NOT) across multiple data streams

This isn't bolted-on error handling. Adaptivity is a **first-class primitive** in the system design.

### <span class="sd-text-primary">From Stateless Tools to Embodied Agents</span>

Current robots have logs, not memory. They record data for post-facto analysis but cannot recall it at runtime. EMOS introduces **embodiment primitives** that give robots a sense of self and history:

- {material-regular}`map;1.2em;sd-text-primary` **Spatio-Temporal Semantic Memory:** A queryable world state backed by vector databases that persists across tasks.
- {material-regular}`self_improvement;1.2em;sd-text-primary` **Self-Referential State:** Components can inspect and modify each other's configuration, enabling system-level awareness rather than isolated self-repair.

### <span class="sd-text-primary">From Porting to Plugins</span>

Every robot has its own interface. Some publish vendor-specific ROS messages, others have no ROS at all and talk over UDP, an HTTP API or an SDK. A **plugin** bridges that gap once, so no Recipe ever has to:

- {material-regular}`extension;1.2em;sd-text-primary` **One plugin per robot.** It presents the robot's odometry, cameras, LiDAR, velocity command, actions and events through the standard interfaces Recipes are written against, and carries the geometry and speed limits navigation needs.
- {material-regular}`settings_input_component;1.2em;sd-text-primary` **Drivers included.** When a Recipe asks for the LiDAR, the plugin starts the driver. When nothing needs it, nothing runs. There is no separate driver setup.
- {material-regular}`sensors;1.2em;sd-text-primary` **Sensors too.** A sensor plugin adds a camera bolted onto the robot, or one watching a room, the same way.
- {material-regular}`map;1.2em;sd-text-primary` **Maps built by the robot.** Where a robot ships no mapping software, `emos map new` maps the site with the robot's own LiDAR, and `emos map use` hands the result to every Recipe.

Plugins come from a catalog and install in one command.

### <span class="sd-text-primary">From CPU Bottlenecks to GPU-Accelerated Navigation</span>

In robotics, GPUs are usually reserved for perception. EMOS moves the whole navigation control stack onto the GPU as well. Kompass, the EMOS navigation engine, provides **GPGPU-accelerated kernels** for motion planning and control:

- {material-regular}`speed;1.2em;sd-text-primary` **Orders of magnitude faster** than CPU-bound stacks for trajectory evaluation and dense occupancy mapping. See the [benchmarks](advanced/benchmarks.md).
- {material-regular}`memory;1.2em;sd-text-primary` **Vendor-neutral:** works on NVIDIA, AMD, Intel and integrated GPUs via SYCL
- {material-regular}`developer_board;1.2em;sd-text-primary` Falls back to optimized process-level parallelism on CPU-only platforms

This enables reactive autonomy in dynamic, unstructured environments where traditional CPU-bound stacks cannot keep up.

### <span class="sd-text-primary">From Separate Backends to Auto-Generated Interaction</span>

In traditional robotics, the automation logic is "backend" and the user interface is a separate custom project. EMOS treats the **Recipe as the single source of truth**, and the interfaces follow from it:

- {material-regular}`web;1.2em;sd-text-primary` **Web UI generated from the Recipe:** defining the logic produces a bespoke interface for real-time monitoring, configuration and control. No frontend development required.
- {material-regular}`dashboard;1.2em;sd-text-primary` **A dashboard on the robot:** reachable from any phone or laptop on its network. Install Recipes from the catalog, start and stop them, watch their output, manage plugins and maps, all without a terminal.
- {material-regular}`lock;1.2em;sd-text-primary` **Encrypted and access-controlled:** the dashboard, every generated UI and the API are served over HTTPS with the robot's own certificate. Browsers are paired once with a code, API clients carry a key, and both can be revoked at any time.

---

## The Architecture

EMOS is built on three open-source components that work in tandem:

| Component | Layer | What It Does |
|:---|:---|:---|
| [**EmbodiedAgents**](https://github.com/automatika-robotics/embodied-agents) | <span class="text-red-strong">Intelligence</span> | Agentic graphs of ML models with Cortex, semantic memory, manipulation, information routing and adaptive reconfiguration |
| [**Kompass**](https://github.com/automatika-robotics/kompass) | <span class="text-red-strong">Navigation</span> | GPU-powered planning and control for real-world mobility across all motion models |
| [**Sugarcoat**](https://github.com/automatika-robotics/sugarcoat) | <span class="text-red-strong">Architecture</span> | Event-driven system primitives, lifecycle management, the plugin framework and the imperative launch API that underpins both layers |

Together they provide a complete runtime: from raw sensor data to intelligent action, with adaptivity and resilience built in at every level. The `emos` CLI installs, updates and runs the lot, in a container, on an existing ROS 2, or in a self-contained environment under your home directory.

---

## Who Is EMOS For

### 1. Robot Managers & End-Users

Run pre-built Recipes from the dashboard, or write your own with the high-level Python API. Focus on your business logic; EMOS handles the robotics complexity.

### 2. Integrators & Solution Providers

EMOS is your SDK for the physical world. Connect robot events to ERPs, building management systems or fleet software using the event-action architecture. Spend your time on enterprise integration, not low-level robotics plumbing.

### 3. OEM Teams

Write one plugin for your chassis and publish it to the catalog. From then on every Recipe anyone writes runs on your hardware, and your customers install it with one command.

---

## <span class="sd-text-primary">EMOS is Built for the Real World</span>

EMOS is not a research prototype. It is shaped by production deployments: autonomous inspection patrols, security operations and field robotics on quadruped and wheeled platforms. Most robots available on the market are supported through plugins, along with a range of external sensors, and every feature in the stack exists because a deployment needed it.

---

## Get Started

::::{grid} 1 3 3 3
:gutter: 3

:::{grid-item-card} {material-regular}`rocket_launch;1.2em;sd-text-primary` Install EMOS
:link: getting-started/installation
:link-type: doc

Get up and running in minutes.
:::

:::{grid-item-card} {material-regular}`extension;1.2em;sd-text-primary` Put It on Your Robot
:link: getting-started/plugins
:link-type: doc

Install the plugin for your robot from the catalog.
:::

:::{grid-item-card} {material-regular}`menu_book;1.2em;sd-text-primary` Browse Recipes
:link: recipes/overview
:link-type: doc

Step-by-step tutorials from simple to production-grade.
:::

::::
