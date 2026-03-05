# Kompass

**The navigation engine of EMOS -- GPU-accelerated, event-driven autonomy for mobile robots.**

[Kompass](https://github.com/automatika-robotics/kompass) lets you create sophisticated navigation stacks within a single Python script -- with blazingly fast, hardware-agnostic performance. It is the only open-source navigation framework with cross-vendor GPU acceleration.

## Why Kompass?

Robotic navigation isn't about perfecting a single component -- it's about architecting a system that survives contact with the real world. While metric navigation has matured, deploying robots extensively in dynamic environments remains an unsolved challenge.

```{epigraph}
_"... while it is worthwhile to extend navigation research in directions orthogonal to metric navigation, the community should also not overlook the problems that still remain in this space, especially when robots are expected to be extensively and reliably deployed in the real world."_

-- Lessons learned from The BARN Challenge at ICRA 2022, *[full article](https://www.researchgate.net/publication/362858861_Autonomous_Ground_Navigation_in_Highly_Constrained_Spaces_Lessons_learned_from_The_BARN_Challenge_at_ICRA_2022)*
```

Kompass was built to fill this gap:

- {material-regular}`bolt;1.2em;sd-text-primary` **Adaptive Event-Driven Core** -- The stack reconfigures itself on the fly based on environmental context. Use *Pure Pursuit* on open roads, switch to *DWA* indoors, fall back to a docking controller near the station -- all triggered by events, not brittle Behavior Trees. Adapt to external world events ("Crowd Detected", "Entering Warehouse"), not just internal robot states.

- {material-regular}`speed;1.2em;sd-text-primary` **GPU-Accelerated, Vendor-Agnostic** -- Core algorithms in C++ with SYCL-based GPU support. Runs natively on **Nvidia, AMD, Intel, and other** GPUs without vendor lock-in -- the first navigation framework to support cross-GPU acceleration. Up to **3,106x speedups** over CPU-based approaches.

- {material-regular}`psychology;1.2em;sd-text-primary` **ML Models as First-Class Citizens** -- Event-driven design means ML model outputs can directly reconfigure the navigation stack. Use object detection to switch controllers, VLMs to answer abstract perception queries, or [EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) vision components for target tracking -- all seamlessly integrated through EMOS's unified architecture.

- {material-regular}`code;1.2em;sd-text-primary` **Pythonic Simplicity** -- Configure a sophisticated, multi-fallback navigation system in a single readable Python script. Core algorithms are decoupled from ROS wrappers, so upgrading ROS distributions won't break your navigation logic. Extend with new planners in Python for prototyping or C++ for production.

## Architecture

Kompass has a modular event-driven architecture, divided into several interacting components each responsible for one navigation subtask.

```{figure} /_static/images/diagrams/system_components_light.png
:class: light-only
:alt: Navigation Components
:align: center

The main components of the Kompass navigation stack.
```

```{figure} /_static/images/diagrams/system_components_dark.png
:class: dark-only
:alt: Navigation Components
:align: center
```

## Navigation Components

::::{grid} 1 2 3 3
:gutter: 3

:::{grid-item-card} {material-regular}`route;1.2em;sd-text-primary` Planner
:link: planning
:link-type: doc

Global path planning using OMPL algorithms (RRT*, PRM, etc.).
:::

:::{grid-item-card} {material-regular}`gamepad;1.2em;sd-text-primary` Controller
:link: control
:link-type: doc

Real-time local control with DWA, Pure Pursuit, Stanley, and DVZ plugins.
:::

:::{grid-item-card} {material-regular}`security;1.2em;sd-text-primary` Drive Manager
:link: drive-manager
:link-type: doc

Safety enforcement, emergency stops, and command smoothing.
:::

:::{grid-item-card} {material-regular}`grid_on;1.2em;sd-text-primary` Local Mapper
:link: mapping
:link-type: doc

Real-time ego-centric occupancy grid from sensor data.
:::

:::{grid-item-card} {material-regular}`public;1.2em;sd-text-primary` Map Server
:link: mapping
:link-type: doc

Static global map management with 3D PCD support.
:::

:::{grid-item-card} {material-regular}`settings;1.2em;sd-text-primary` Robot Config
:link: robot-config
:link-type: doc

Define kinematics, geometry, and control limits for your platform.
:::

::::

## Minimum Sensor Requirements

| Sensor | Purpose | Required? |
| :--- | :--- | :--- |
| Odometry | Robot pose estimation | Yes |
| LiDAR or Depth Camera | Obstacle detection | Yes |
| Pose Source (SLAM/AMCL) | Global localization | Yes (for map-based nav) |
| RGB Camera | Vision-based tracking | Optional |
| 3D LiDAR | 3D mapping | Optional |
