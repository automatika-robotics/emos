# Components

**Stop writing boilerplate. Start writing core logic.**

In EMOS, a `Component` is the fundamental unit of execution. It replaces the standard ROS2 Node with a robust, **Lifecycle-Managed**, and **Self-Healing** entity designed for production-grade autonomy.

While a standard ROS2 node requires you to manually handle parameter callbacks, error catching, and state transitions, an EMOS Component handles this plumbing automatically, letting you focus entirely on your algorithm.

## Why Build with EMOS Components?

EMOS Components come with "superpowers" out of the box.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`autorenew;1.5em;sd-text-primary` Lifecycle Native</span> - Every component is a **Managed Lifecycle Node**. It supports `Configure`, `Activate`, `Deactivate`, and `Shutdown` states automatically, ensuring deterministic startup and shutdown.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`healing;1.5em;sd-text-primary` Self-Healing</span> - Components have a built-in "Immune System." If an algorithm fails or a driver disconnects, the component can trigger **[Fallbacks](status-and-fallbacks.md#fallback-strategies)** to restart or reconfigure itself without crashing the stack.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`monitor_heart;1.5em;sd-text-primary` Health Aware</span> - Components actively report their **[Health Status](status-and-fallbacks.md#health-status)** (Healthy, Algorithm Failure, etc.) to the system, enabling system-wide reflexes and alerts.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`verified;1.5em;sd-text-primary` Type-Safe Config</span> - Component configurations are validated using `attrs` models, catching type errors before runtime, and allowing easy Pythonic configuration in your recipe.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`hub;1.5em;sd-text-primary` Auto-Wiring</span> - Inputs and Outputs are declarative. Define a `Topic` as an input or output to your component, and EMOS automatically handles the subscription, serialization, and callback plumbing for you.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`bolt;1.5em;sd-text-primary` Event-Driven</span> - Components are reactive by design. They can be configured to execute their main logic only when triggered by an **[Event](events-and-actions.md)** or a Service call, rather than running in a continuous loop.


```{figure} /_static/images/diagrams/component_dark.png
:class: dark-only
:alt: component structure
:align: center
```

```{figure} /_static/images/diagrams/component_light.png
:class: light-only
:alt: component structure
:align: center

Component Architecture
```


## Execution Modes (Run Types)

A Component isn't just a `while(True)` loop. You can configure *how* its main functionality executes using the `run_type` property.

```{list-table}
:widths: 15 50 35
:header-rows: 1

* - Run Type
  - Description
  - Best For...

* - **Timed**
  - Executes the main step in a fixed-frequency loop (e.g., 10Hz).
  - Controllers, Planners, Drivers

* - **Event**
  - Dormant until triggered by a specific Topic or Event.
  - Image Processors, Detectors

* - **Server**
  - Dormant until a ROS2 Service Request is received.
  - Calibration Nodes, Compute Servers

* - **ActionServer**
  - Dormant until a ROS2 Action Goal is received.
  - Long-running tasks (Navigation, Arms)
```

**Configuration Example:**

```python
from ros_sugar.config import ComponentRunType
from ros_sugar.core import BaseComponent

# Can set from Component
comp = BaseComponent(component_name='test')
comp.run_type = "Server"    # or ComponentRunType.SERVER
```

:::{tip} All the functionalities implemented in ROS2 nodes can be found in the Component.
:::

## Declarative Inputs & Outputs

Wiring up data streams shouldn't be tedious. EMOS allows you to define inputs and outputs declaratively.

When the component launches, it automatically creates the necessary publishers, subscribers, and type converters based on your definitions.

```python
from ros_sugar.core import BaseComponent
from ros_sugar.io import Topic

# 1. Define your interface
map_topic   = Topic(name="map", msg_type="OccupancyGrid")
voice_topic = Topic(name="voice_cmd", msg_type="Audio")
image_topic = Topic(name="camera/rgb", msg_type="Image")

# 2. Auto-wire the component
# EMOS handles the QoS, callback groups, and serialization automatically
comp = BaseComponent(
    component_name='audio_processor',
    inputs=[map_topic, image_topic],
    outputs=[voice_topic]
)
```

:::{tip}
EMOS provides built-in "Converters" for common ROS2 types (Images, Pose, etc.), so you can work with native Python objects instead of raw ROS2 messages.
:::


:::{seealso} Check the full configuration options of Topics [here](topics.md)
:::


## The Component Immune System: Health & Fallbacks

A robust robot doesn't just crash when an error occurs; it degrades gracefully.

### Health Status

Instead of printing a log message and dying, a Component reports its **Health Status**. This status is both:

- {material-regular}`settings;1.2em;sd-text-primary` **Internal:** Used immediately by the component to trigger local recovery strategies.

- {material-regular}`cell_tower;1.2em;sd-text-primary` **External:** Broadcasted to alert other parts of the system.

### Fallbacks (Self-Healing)

You can define **reflexes** that trigger automatically when health degrades.

* {material-regular}`restart_alt;1.2em;sd-text-danger` *Is the driver dead?*  **Restart** the node.
* {material-regular}`tune;1.2em;sd-text-warning` *Is the planner stuck?*  **Reconfigure** the tolerance parameters.
* {material-regular}`swap_horiz;1.2em;sd-text-primary` *Is the sensor noisy?*  **Switch** to a different algorithm.

> **Learn More:** [Status & Fallbacks Guide](./status-and-fallbacks.md).


## Pro Tips for Component Devs

:::{admonition} Best Practices
:class: tip

* **Keep `__init__` Light:** Do not open heavy resources (cameras, models) in `__init__`. Use `custom_on_configure` or `custom_on_activate`. This allows your node to be introspected and configured *before* it starts consuming resources.

* **Always Report Status:** Make it a habit to call `self.health_status.set_healthy()` at the end of a successful `_execution_step`. This acts as a heartbeat for the system.

* **Catch, Don't Crash:** Wrap your main logic in `try/except` blocks. Instead of raising an exception, catch it and report `set_fail_algorithm`, for example. This keeps the process alive and allows your [Fallbacks](status-and-fallbacks.md) to kick in and save the day.
:::
