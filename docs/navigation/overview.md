# Navigation Layer

**GPU-accelerated, event-driven navigation for autonomous mobile robots.**

The EMOS Navigation Layer provides robust, customizable, and hardware-agnostic navigation capabilities. It enables you to create sophisticated navigation stacks within a single Python script with blazingly fast performance.

## Why EMOS Navigation?

Robotic navigation isn't about perfecting a single component — it is about architecting a system that survives contact with the real world. While metric navigation has matured, deploying robots extensively in dynamic environments remains an unsolved challenge.

EMOS Navigation was built to fill this gap with four key differentiators:

### True Adaptive Navigation (Event-Driven Core)

The navigation stack can **reconfigure itself on the fly** based on environmental context:

- **Dynamic Response:** Adapt not just to internal robot states (like Battery Low), but to external world events (e.g., "Crowd Detected" or "Entering Warehouse").
- **Context-Aware Control:** Configure the system to use *Pure Pursuit* on open roads, switch to *DWA* indoors, and fallback to a precise docking controller — all triggered by environmental context.
- **Simplified Logic:** Unlike complex Behavior Trees that can become unmanageable, EMOS allows you to define clean, event-based transitions and fallback behaviors for every component.

### High-Performance & Vendor-Agnostic GPU Acceleration

The navigation layer is engineered for speed, utilizing C++ for core algorithms and multi-threading for CPU tasks.

- **SYCL-Based Architecture:** The first navigation framework to support cross-GPU acceleration.
- **Hardware Freedom:** Runs natively on **Nvidia, AMD, Intel and other integrated** GPUs/APUs without vendor lock-in.
- **Parallel Power:** Compute-heavy tasks like path planning, control, and map updates are offloaded to the GPU, freeing up your CPU for high-level logic.

### ML Models as First-Class Citizens

Because EMOS is event-driven, it can directly utilize ML model outputs to drive navigation behavior:

- **Neuro-Symbolic Control:** Use an object detection model to trigger a controller switch (e.g., switching to Human-Aware navigation when people are detected).
- **VLM Integration:** Use Vision Language Models to answer abstract queries and alter the robot's planning strategy.
- **Deep Integration:** The Intelligence Layer and Navigation Layer work together seamlessly through EMOS's unified component architecture.

### Pythonic Simplicity

- **One-Script Configuration:** Configure a sophisticated, multi-fallback navigation system in a single, readable Python script.
- **Clean Architecture:** Core algorithms are decoupled from ROS wrappers, ensuring upgrading ROS distributions doesn't break your navigation logic.
- **Extensible:** Plug in new planners or controllers in Python for prototyping or C++ for production.

## Architecture

The Navigation Layer has a modular event-driven architecture, divided into several interacting components each responsible for one navigation subtask.

```{figure} /_static/images/diagrams/system_components_light.png
:class: light-only
:alt: Navigation Components
:align: center

The main components of the EMOS navigation stack.
```

```{figure} /_static/images/diagrams/system_components_dark.png
:class: dark-only
:alt: Navigation Components
:align: center
```

## Navigation Components

| Component | Function |
| :--- | :--- |
| [**Robot Config**](robot-config.md) | Define your robot's physical constraints (kinematics, geometry, control limits). |
| [**Planner**](planning.md) | Global path planning using OMPL algorithms (RRT*, PRM, etc.). |
| [**Controller**](control.md) | Real-time local control with multiple algorithm plugins (DWA, Pure Pursuit, Stanley, DVZ). |
| [**Drive Manager**](drive-manager.md) | Safety enforcement, emergency stops, and command smoothing. |
| [**Local Mapper**](mapping.md) | Real-time ego-centric occupancy grid generation from sensor data. |
| [**Map Server**](mapping.md) | Static global map management with 3D PCD support. |
| [**Motion Server**](motion-server.md) | System validation, calibration, and motion data recording. |

## Minimum Sensor Requirements

| Sensor | Purpose | Required? |
| :--- | :--- | :--- |
| Odometry | Robot pose estimation | Yes |
| LiDAR or Depth Camera | Obstacle detection | Yes |
| Pose Source (SLAM/AMCL) | Global localization | Yes (for map-based nav) |
| RGB Camera | Vision-based tracking | Optional |
| 3D LiDAR | 3D mapping | Optional |
