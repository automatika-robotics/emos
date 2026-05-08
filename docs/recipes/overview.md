# Recipes & Tutorials

## What Are EMOS Recipes?

An EMOS **Recipe** is a reusable, hardware-agnostic application package that defines a robot behavior. Recipes replace the brittle, task-specific ROS projects of the past with composable, declarative Python scripts that combine EMOS components -- intelligence, navigation, perception, and memory -- into a single agentic workflow.

A Recipe is not a "script" in the traditional sense. It is a complete application: a graph of [Components](../intelligence/ai-components.md) wired together through [Topics](../concepts/topics.md), enriched with [Events & Actions](../concepts/events-and-actions.md) for runtime adaptivity, and launched with a single call to `Launcher.bringup()`.

### Write Once, Run Anywhere

The core promise of EMOS Recipes is hardware independence. A "Security Patrol" recipe written for a wheeled AMR runs identically on a quadruped -- EMOS handles the kinematic translation and action commands beneath the surface. This decoupling of the robot's **Mind** from its **Body** means that:

- The same Recipe can be deployed across an entire heterogeneous fleet.
- Recipes can be shared, versioned, and composed just like software packages.
- The same robot can run multiple Recipes, switching between them as conditions demand.

## Tutorial Structure

The tutorials in this section follow a graduated structure, building from simple single-component examples to a full agentic system. They are organized into four subsections:

- {material-regular}`psychology;1.2em;sd-text-primary` **[Cognition Recipes](foundation/index.md)** -- Build intelligent agents from the ground up using [EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents): conversational agents, prompt engineering, graph-backed spatio-temporal memory, memory-aware navigation, tool calling, semantic routing, and a complete end-to-end agent.

- {material-regular}`precision_manufacturing;1.2em;sd-text-primary` **[Multimodal Planning & Manipulation](planning-and-manipulation/index.md)** -- The [Cortex](../intelligence/cortex.md) agentic harness (drop one component into your recipe and it turns the rest into a self-directing agent), Cortex paired with Memory, Cortex driving the full stack on a mobile robot, plus VLM-based planning and VLA-based end-to-end manipulation.

- {material-regular}`route;1.2em;sd-text-primary` **[Navigation](navigation/index.md)** -- Set up and use [Kompass](https://github.com/automatika-robotics/kompass), the EMOS navigation engine: simulation quick starts, point navigation, path recording and replay, automated motion testing, and vision-based target tracking with RGB and depth cameras.

- {material-regular}`healing;1.2em;sd-text-primary` **[Adaptivity & Resilience](events-and-resilience/index.md)** -- Make your agents robust and adaptive using multiprocessing with process-level recovery, runtime fallbacks, event-driven cognition, the System Graph view for visualising the running graph, cross-component healing, composed logic gates, and context-aware dynamic actions.

## Recipe Examples

Below are five real-world examples that illustrate what EMOS Recipes look like in practice. Each is a tight Python snippet showing the *shape* of the recipe -- not the full file -- with a link to the full tutorial it lives in.

### 1. The Warehouse Auditor

*A mobile robot that walks the aisles overnight, checks the shelves, and writes the variance report.*

You don't tell it which aisles in what order. You give it the mission -- *"audit aisle 4 and flag anything misshelved or out of stock"* -- and a [Cortex](../intelligence/cortex.md) component on top of Vision, a VLM, Memory, and the Kompass navigation stack inspects every component as a callable tool, plans the steps, dispatches navigation goals, calls the VLM at each shelf, and replans on failure. Zero orchestration glue from you.

```python
# That's the agent. It auto-discovers every other component in the
# launcher and exposes their actions as skills and tools.
cortex = Cortex(
    output=cortex_output,
    model_client=planner_client,
    config=CortexConfig(max_planning_steps=5, max_execution_steps=15),
    component_name="cortex",
)
```

See: [Cortex Driving the Full Stack](planning-and-manipulation/cortex-navigation.md).

### 2. The Home Care Companion

*A household robot that learns the layout of the home it's deployed in -- and remembers it after a reboot.*

[Memory](../intelligence/memory.md) folds every detection, scene caption, and interoception reading into a graph indexed simultaneously by **meaning**, **location**, and **time**. Episodes consolidate into long-term gists; the same fridge gets recognised across different sessions; the whole memory state is in a SQLite file. Power-cycle the robot, point it at the same `db_path`, and the robot never forgets a thing.

```python
memory = Memory(
    layers=[
        MemLayer(subscribes_to=detections),
        MemLayer(subscribes_to=scene_caption),
        MemLayer(subscribes_to=battery, is_internal_state=True),
    ],
    position=position,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/var/lib/robot/memory.db"),
    component_name="memory",
)
```

See: [Spatio-Temporal Memory](foundation/semantic-map.md), [Memory and Cortex](planning-and-manipulation/cortex-memory.md).

### 3. The Last-Mile Delivery Worker

*A delivery worker robot that pulls itself over to a charging pad before its battery dies on someone's lawn.*

Halfway through a multi-stop route, it notices its charge is getting thin and it's still a long way from base. It diverts to the nearest pad, tops up, and continues the run from where it left off -- no operator in the loop, no aborted delivery.

```python
def needs_to_recharge() -> bool:
    return battery_pct() < 15.0 and distance_to_depot_m() > 200.0

events_actions = {
    Event(needs_to_recharge, check_rate=1.0): return_to_nearest_pad,
}
```

See: [Internal-State Events](events-and-resilience/internal-state-events.md).

### 4. The 24/7 Security Patrol

*A patrol robot that stays on its rounds through model-server outages, segfaults, and OS hiccups -- without anyone in the loop.*

Two layers of recovery applied to the whole graph in a few lines. The first reacts to unhealthy components in-process; the second relaunches a component if its process actually exits.

```python
for component in all_components:
    component.on_fail(action=Action(component.restart), max_retries=2)

launcher.on_process_fail(max_retries=3)   # OS-level safety net
```

See: [Multiprocessing & Fault Tolerance](events-and-resilience/multiprocessing.md), [Status & Fallbacks](../concepts/status-and-fallbacks.md).

### 5. The "Off-Grid" Field Mule

*A porter robot that follows a worker around a construction site, no GPS or SLAM required.*

Depth fusion + visual detection lock on to a human guide and the controller follows them. The whole behaviour is one controller-config switch.

```python
from kompass.control import ControllersID

controller.algorithm = ControllersID.VISION_DEPTH
controller.inputs(
    vision_detections=detections_topic,
    depth_camera_info=depth_cam_info_topic,
)
```

See: [Vision Tracking with Depth](navigation/vision-tracking-depth.md).
