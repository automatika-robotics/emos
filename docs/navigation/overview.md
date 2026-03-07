# Kompass

**The navigation engine of EMOS --** <span class="text-red-strong">GPU-accelerated, event-driven autonomy for mobile robots</span>

[Kompass](https://github.com/automatika-robotics/kompass) lets you create sophisticated navigation stacks with blazingly fast, hardware-agnostic performance. It is the only open-source navigation framework with cross-vendor GPU acceleration.

## Why Kompass?

Robotic navigation isn't about perfecting a single component; it is about architecting a system that survives contact with the real world.

While metric navigation has matured, deploying robots extensively in dynamic environments remains an unsolved challenge. As highlighted by the **ICRA BARN Challenges**, static pipelines fail when faced with the unpredictability of the physical world:

> _"A single stand-alone approach that is able to address all variety of obstacle configurations all together is still out of our reach."_
> — **Lessons from The 3rd BARN Challenge (ICRA 2024)**

**Kompass was built to fill this gap.** Unlike existing solutions that rely on rigid behavior trees, Kompass is an event-driven, GPU-native stack designed for maximum adaptability and hardware efficiency.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`bolt;1.2em;sd-text-primary` Adaptive Event-Driven Core</span> -- The stack reconfigures itself on the fly based on environmental context. Use *Pure Pursuit* on open roads, switch to *DWA* indoors, fall back to a docking controller near the station -- all triggered by events, not brittle Behavior Trees. Adapt to external world events ("Crowd Detected", "Entering Warehouse"), not just internal robot states.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`speed;1.2em;sd-text-primary` GPU-Accelerated, Vendor-Agnostic</span> -- Core algorithms in C++ with SYCL-based GPU support. Runs natively on **Nvidia, AMD, Intel, and other** GPUs without vendor lock-in -- the first navigation framework to support cross-GPU acceleration. Up to **3,106x speedups** over CPU-based approaches.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`psychology;1.2em;sd-text-primary` ML Models as First-Class Citizens</span> -- Event-driven design means ML model outputs can directly reconfigure the navigation stack. Use object detection to switch controllers, VLMs to answer abstract perception queries, or [EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) vision components for target tracking -- all seamlessly integrated through EMOS's unified architecture.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`code;1.2em;sd-text-primary` Pythonic Simplicity</span> -- Configure a sophisticated, multi-fallback navigation system in a single readable Python script. Core algorithms are decoupled from ROS wrappers, so upgrading ROS distributions won't break your navigation logic. Extend with new planners in Python for prototyping or C++ for production.

---

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

Each component runs as a ROS2 lifecycle node and communicates with the other components using ROS2 topics, services or action servers:

```{figure} /_static/images/diagrams/system_graph_light.png
:class: light-only
:alt: Kompass Full System
:align: center

System Diagram for Point Navigation
```

```{figure} /_static/images/diagrams/system_graph_dark.png
:class: dark-only
:alt: Kompass Full System
:align: center
```

---

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

Real-time local control with DWA, Stanley, DVZ, and Vision Follower plugins.
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

---

## Minimum Sensor Requirements

Kompass is designed to be flexible in terms of sensor configurations. However, at least the following sensors are required for basic autonomous navigation:

- {material-regular}`speed;1.2em;sd-text-primary` **Odometry Source** (e.g., wheel encoders, IMU or visual odometry)
- {material-regular}`radar;1.2em;sd-text-primary` **Obstacle Detection Sensor** (e.g., 2D LiDAR **or** Depth Camera)
- {material-regular}`my_location;1.2em;sd-text-primary` **Robot Pose Source** (e.g., localization system such as AMCL or visual SLAM)

These provide the minimal data necessary for localization, mapping, and safe path execution.

## Optional Sensors for Enhanced Features

Additional sensors can enhance navigation capabilities and unlock advanced features:

- {material-regular}`camera;1.2em;sd-text-secondary` **RGB Camera(s)** — Enables vision-based navigation, object tracking, and semantic navigation.
- {material-regular}`view_in_ar;1.2em;sd-text-secondary` **Depth Camera** — Improves obstacle avoidance in 3D environments and enables more accurate object tracking.
- {material-regular}`sensors;1.2em;sd-text-secondary` **3D LiDAR** — Enhances perception in complex environments with full 3D obstacle detection.
- {material-regular}`satellite_alt;1.2em;sd-text-secondary` **GPS** — Enables outdoor navigation and geofenced planning.
- {material-regular}`cell_tower;1.2em;sd-text-secondary` **UWB / BLE Beacons** — Improves localization in GPS-denied environments.

---

Kompass supports dynamic configuration, allowing it to operate with minimal sensors and scale up for complex applications when additional sensing is available.
