# Cross-Component Healing

In the [Self-Healing with Fallbacks](fallback-recipes.md) recipe, we learned how a component can heal *itself* (e.g., restarting or switching algorithms). But sophisticated autonomy requires more than self-repair -- it requires **system-level awareness**, where components monitor *each other* and take corrective action.

In this recipe, we use **Events** to implement cross-component healing: one component detects a failure, and a *different* component executes the recovery.

---

## Scenario A: The "Unstuck" Reflex

The `Controller` gets stuck in a local minimum (e.g., the robot is facing a corner). It reports an `ALGORITHM_FAILURE` because it cannot find a valid velocity command. We detect this status and ask the `DriveManager` to execute a blind "Unblock" maneuver -- rotate in place or back up.

```{tip}
All component health status topics are accessible via `component.status_topic`.
```

### Define the Event and Action

```python
from kompass.ros import Event, Action, Topic
from automatika_ros_sugar.msg import ComponentStatus

# Event: Controller reports algorithm failure
# keep_event_delay prevents re-triggering while recovery is in progress
event_controller_fail = Event(
    controller.status_topic.msg.status
    == ComponentStatus.STATUS_FAILURE_ALGORITHM_LEVEL,
    keep_event_delay=60.0
)

# Action: DriveManager executes a recovery maneuver
unblock_action = Action(method=driver.move_to_unblock)
```

The `keep_event_delay=60.0` ensures the unblock action fires at most once per minute, giving the controller time to recover before trying again.

---

## Scenario B: The "Blind Mode" Reflex

The `LocalMapper` crashes, failing to provide the high-fidelity local map that the `Controller` depends on. Instead of halting, the `Controller` reconfigures itself to use raw sensor data directly (reactive mode).

```python
from kompass.actions import update_parameter

# Event: Mapper is NOT healthy
# handle_once=True means this fires only ONCE during the system's lifetime
event_mapper_fault = Event(
    mapper.status_topic.msg.status != ComponentStatus.STATUS_HEALTHY,
    handle_once=True
)

# Action: Reconfigure Controller to bypass the mapper
activate_direct_sensor_mode = update_parameter(
    component=controller,
    param_name="use_direct_sensor",
    new_value=True
)
```

---

## Scenario C: Goal Handling via Events

In a production system, goals often arrive from external interfaces like RViz rather than being hardcoded. Events bridge the gap: we listen for clicked points and forward them to the Planner's ActionServer.

### Define the Goal Event

```python
clicked_point = Topic(name="/clicked_point", msg_type="PointStamped")

# Fire on every PointStamped that arrives on /clicked_point
event_clicked_point = Event(clicked_point)
```

### Define the Goal Action

The planner's `trigger_main_action_server` is a component action that sends a goal to its own action server. Its arguments are read from the clicked point at the moment the event fires, so no parser is needed:

```python
from kompass.actions import log

send_goal = Action(
    method=planner.trigger_main_action_server,
    args=(
        clicked_point.msg.point.x,
        clicked_point.msg.point.y,
        0.05,  # goal distance tolerance
        0.2,   # goal angle tolerance, in radians
    ),
)
```

```{tip}
`send_action_goal` and `send_srv_request` from `kompass.actions` reach **any** ROS 2 action server or service from an event, with a goal or request you build yourself. A component's own server is easier to reach through an action like the one above.
```

---

## Wiring Events to Actions

With all events and actions defined, we assemble the event-action dictionary. Each event maps to one or more actions:

```python
events_actions = {
    # RViz click -> log + send goal to planner
    event_clicked_point: [log(msg="Got new goal point"), send_goal],
    # Controller stuck -> unblock maneuver
    event_controller_fail: unblock_action,
    # Mapper down -> switch controller to direct sensor mode
    event_mapper_fault: activate_direct_sensor_mode,
}
```

---

## Complete Recipe

```{code-block} python
:caption: cross_component_healing.py
:linenos:

import numpy as np
import os

from automatika_ros_sugar.msg import ComponentStatus
from kompass.actions import Action, log, update_parameter
from kompass.components import (
    Controller, DriveManager, Planner, PlannerConfig, LocalMapper,
)
from kompass.config import RobotConfig
from kompass.robot import (
    AngularCtrlLimits, LinearCtrlLimits, RobotGeometryType, RobotType,
)
from kompass.ros import Topic, Launcher, Event

# --- Robot Configuration ---
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometryType.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.2, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_omega=0.4, max_acc=2.0, max_decel=2.0, max_ang=np.pi / 3
    ),
)

# --- Components ---
planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
planner.run_type = "ActionServer"

controller = Controller(component_name="controller")
controller.direct_sensor = False

mapper = LocalMapper(component_name="mapper")
driver = DriveManager(component_name="drive_manager")

if os.environ.get("ROS_DISTRO") in ["rolling", "jazzy", "kilted"]:
    cmd_msg_type = "TwistStamped"
else:
    cmd_msg_type = "Twist"

driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_msg_type))

# --- Cross-Component Events ---
# 1. Controller stuck -> DriveManager unblocks
event_controller_fail = Event(
    controller.status_topic.msg.status
    == ComponentStatus.STATUS_FAILURE_ALGORITHM_LEVEL,
    keep_event_delay=60.0
)
unblock_action = Action(method=driver.move_to_unblock)

# 2. Mapper down -> Controller switches to direct sensor mode
event_mapper_fault = Event(
    mapper.status_topic.msg.status != ComponentStatus.STATUS_HEALTHY,
    handle_once=True
)
activate_direct_sensor_mode = update_parameter(
    component=controller, param_name="use_direct_sensor", new_value=True
)

# 3. RViz click -> Planner goal
clicked_point = Topic(name="/clicked_point", msg_type="PointStamped")
event_clicked_point = Event(clicked_point)

send_goal = Action(
    method=planner.trigger_main_action_server,
    args=(clicked_point.msg.point.x, clicked_point.msg.point.y, 0.05, 0.2),
)

# --- Wire Events -> Actions ---
events_actions = {
    event_clicked_point: [log(msg="Got new goal point"), send_goal],
    event_controller_fail: unblock_action,
    event_mapper_fault: activate_direct_sensor_mode,
}

# --- Launch ---
odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")

launcher = Launcher()
launcher.add_pkg(
    components=[planner, controller, mapper, driver],
    package_name="kompass",
    events_actions=events_actions,
    activate_all_components_on_start=True,
    multiprocessing=True,
)
launcher.inputs(location=odom_topic)
launcher.robot = my_robot
launcher.bringup()
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```

