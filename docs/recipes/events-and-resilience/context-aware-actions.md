# Context-Aware Actions

In previous recipes, our actions used **static** arguments -- pre-defined at configuration time. For example, in [Self-Healing with Fallbacks](fallback-recipes.md), we defined `Action(method=controller.set_algorithm, args=(ControllersID.PURE_PURSUIT,))` where the target algorithm is hardcoded.

But what if the action depends on **what** the robot is seeing, or **where** it was told to go? Real-world autonomy requires **dynamic data injection** -- action arguments fetched from the system at the time of execution.

---

## The Concept: Static vs Dynamic

| Type | Argument Set At | Example |
|---|---|---|
| **Static** | Configuration time | `args=(ControllersID.PURE_PURSUIT,)` |
| **Dynamic** | Event firing time | `args=(command_topic.msg.data,)` |

With dynamic injection, you pass a **topic message field** as an argument. EMOS resolves the actual value when the event fires, not when the recipe is written.

---

## Navigation Example: Semantic Navigation

We build a system where you publish a location name (like "kitchen") to a topic, and the robot automatically looks up the coordinates and navigates there.

### 1. Define the Command Source

```python
from kompass.ros import Topic

# Simulates a voice command or fleet management instruction
# Examples: "kitchen", "reception", "station_a"
command_topic = Topic(name="/user_command", msg_type="String")
```

### 2. Write the Lookup Function

```python
import subprocess

# A simple map of the environment
# In a real app, this could come from a database or semantic memory
WAYPOINTS = {
    "kitchen":   {"x": 2.0, "y": 0.5},
    "reception": {"x": 0.0, "y": 0.0},
    "station_a": {"x": -1.5, "y": 2.0},
}

def navigate_to_location(location_name: str):
    """Looks up coordinates and publishes a goal to the planner."""
    key = location_name.strip().lower()
    if key not in WAYPOINTS:
        print(f"Unknown location: {key}")
        return

    coords = WAYPOINTS[key]
    topic_cmd = (
        f"ros2 topic pub --once /clicked_point geometry_msgs/msg/PointStamped "
        f"'{{header: {{frame_id: \"map\"}}, point: {{x: {coords['x']}, y: {coords['y']}, z: 0.0}}}}'"
    )
    subprocess.run(topic_cmd, shell=True)
```

### 3. Define the Event and Action

```python
from kompass.ros import Event, Action
from sugar.msg import ComponentStatus

# Trigger on any new command, but only if the mapper is healthy
event_command_received = Event(
    event_condition=(
        command_topic
        & (mapper.status_topic.msg.status == ComponentStatus.STATUS_HEALTHY)
    ),
)

# DYNAMIC INJECTION: command_topic.msg.data is resolved at event-fire time
action_process_command = Action(
    method=navigate_to_location,
    args=(command_topic.msg.data,)
)
```

When someone publishes `"kitchen"` to `/user_command`, the Event fires and the Action calls `navigate_to_location("kitchen")` -- the string is fetched live from the topic.

---

## Intelligence Example: Dynamic Prompt Injection

The same pattern works for the intelligence layer. Consider a Vision component that detects objects, and a VLM that should describe *whatever* was detected -- not just "person":

```python
from agents.ros import Topic, Event, Action, FixedInput
from agents.components import Vision, VLM

# Vision outputs
detections = Topic(name="/detections", msg_type="Detections")
camera_image = Topic(name="/image_raw", msg_type="Image")

# Event: any object detected
event_object_detected = Event(
    detections.msg.labels.length() > 0,
    on_change=True,
    keep_event_delay=5
)

# Dynamic prompt: inject the detected label into the VLM query
def describe_detected_object(label: str):
    """Called with the actual detected label at event time."""
    return f"A {label} has been detected. Describe what you see."

action_describe = Action(
    method=describe_detected_object,
    args=(detections.msg.labels[0],)  # First detected label, resolved dynamically
)
```

---

## Complete Navigation Recipe

Launch this script, then publish a string to `/user_command` (e.g., `ros2 topic pub /user_command std_msgs/String "data: kitchen" --once`) to see the robot navigate.

```{code-block} python
:caption: semantic_navigation.py
:linenos:

import os
import subprocess
import numpy as np

from sugar.msg import ComponentStatus
from kompass.components import (
    Controller, DriveManager, Planner, PlannerConfig, LocalMapper,
)
from kompass.config import RobotConfig
from kompass.control import ControllersID
from kompass.robot import (
    AngularCtrlLimits, LinearCtrlLimits, RobotGeometry, RobotType,
)
from kompass.ros import Topic, Launcher, Action, Event

# --- Waypoint Database ---
WAYPOINTS = {
    "kitchen":   {"x": 2.0, "y": 0.5},
    "reception": {"x": 0.0, "y": 0.0},
    "station_a": {"x": -1.5, "y": 2.0},
}

def navigate_to_location(location_name: str):
    key = location_name.strip().lower()
    if key not in WAYPOINTS:
        print(f"Unknown location: {key}")
        return
    coords = WAYPOINTS[key]
    topic_cmd = (
        f"ros2 topic pub --once /clicked_point geometry_msgs/msg/PointStamped "
        f"'{{header: {{frame_id: \"map\"}}, point: {{x: {coords['x']}, y: {coords['y']}, z: 0.0}}}}'"
    )
    subprocess.run(topic_cmd, shell=True)

# --- Command Topic ---
command_topic = Topic(name="/user_command", msg_type="String")

# --- Robot Configuration ---
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.2, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=0.4, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)

# --- Components ---
planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
goal_topic = Topic(name="/clicked_point", msg_type="PointStamped")
planner.inputs(goal_point=goal_topic)

controller = Controller(component_name="controller")
controller.direct_sensor = False
controller.algorithm = ControllersID.DWA

mapper = LocalMapper(component_name="mapper")
driver = DriveManager(component_name="drive_manager")

if os.environ.get("ROS_DISTRO") in ["rolling", "jazzy", "kilted"]:
    cmd_msg_type = "TwistStamped"
else:
    cmd_msg_type = "Twist"
driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_msg_type))

# --- Context-Aware Event & Action ---
event_command_received = Event(
    event_condition=(
        command_topic
        & (mapper.status_topic.msg.status == ComponentStatus.STATUS_HEALTHY)
    ),
)

action_process_command = Action(
    method=navigate_to_location,
    args=(command_topic.msg.data,)  # Dynamic injection
)

events_actions = {
    event_command_received: action_process_command,
}

# --- Launch ---
launcher = Launcher()
launcher.kompass(
    components=[planner, controller, driver, mapper],
    activate_all_components_on_start=True,
    multi_processing=True,
    events_actions=events_actions,
)

odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")
launcher.inputs(location=odom_topic)
launcher.robot = my_robot
launcher.bringup()
```
