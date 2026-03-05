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
from sugar.msg import ComponentStatus

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
from kompass import event
from kompass.actions import ComponentActions

# Fire whenever a new PointStamped arrives on /clicked_point
event_clicked_point = event.OnGreater(
    "rviz_goal",
    Topic(name="/clicked_point", msg_type="PointStamped"),
    0,
    ["header", "stamp", "sec"],
)
```

### Define the Goal Action with a Parser

The clicked point message needs to be converted into a `PlanPath.Goal`. We write a parser function and attach it to the action:

```python
from kompass_interfaces.action import PlanPath
from kompass_interfaces.msg import PathTrackingError
from geometry_msgs.msg import Pose, PointStamped
from kompass.actions import LogInfo

# Create the action server goal action
send_goal = ComponentActions.send_action_goal(
    action_name="/planner/plan_path",
    action_type=PlanPath,
    action_request_msg=PlanPath.Goal(),
)

# Parse PointStamped into PlanPath.Goal
def goal_point_parser(*, msg: PointStamped, **_):
    action_request = PlanPath.Goal()
    goal = Pose()
    goal.position.x = msg.point.x
    goal.position.y = msg.point.y
    action_request.goal = goal
    end_tolerance = PathTrackingError()
    end_tolerance.orientation_error = 0.2
    end_tolerance.lateral_distance_error = 0.05
    action_request.end_tolerance = end_tolerance
    return action_request

send_goal.event_parser(goal_point_parser, output_mapping="action_request_msg")
```

```{tip}
`ComponentActions.send_srv_request` and `ComponentActions.send_action_goal` let you call **any** ROS 2 service or action server from an event -- not just EMOS services.
```

---

## Wiring Events to Actions

With all events and actions defined, we assemble the event-action dictionary. Each event maps to one or more actions:

```python
events_actions = {
    # RViz click -> log + send goal to planner
    event_clicked_point: [LogInfo(msg="Got new goal point"), send_goal],
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

from sugar.msg import ComponentStatus
from kompass_interfaces.action import PlanPath
from kompass_interfaces.msg import PathTrackingError
from geometry_msgs.msg import Pose, PointStamped

from kompass import event
from kompass.actions import Action, ComponentActions, LogInfo, update_parameter
from kompass.components import (
    Controller, DriveManager, Planner, PlannerConfig, LocalMapper,
)
from kompass.config import RobotConfig
from kompass.robot import (
    AngularCtrlLimits, LinearCtrlLimits, RobotGeometry, RobotType,
)
from kompass.ros import Topic, Launcher, Event

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
event_clicked_point = event.OnGreater(
    "rviz_goal",
    Topic(name="/clicked_point", msg_type="PointStamped"),
    0, ["header", "stamp", "sec"],
)

send_goal = ComponentActions.send_action_goal(
    action_name="/planner/plan_path",
    action_type=PlanPath,
    action_request_msg=PlanPath.Goal(),
)

def goal_point_parser(*, msg: PointStamped, **_):
    action_request = PlanPath.Goal()
    goal = Pose()
    goal.position.x = msg.point.x
    goal.position.y = msg.point.y
    action_request.goal = goal
    end_tolerance = PathTrackingError()
    end_tolerance.orientation_error = 0.2
    end_tolerance.lateral_distance_error = 0.05
    action_request.end_tolerance = end_tolerance
    return action_request

send_goal.event_parser(goal_point_parser, output_mapping="action_request_msg")

# --- Wire Events -> Actions ---
events_actions = {
    event_clicked_point: [LogInfo(msg="Got new goal point"), send_goal],
    event_controller_fail: unblock_action,
    event_mapper_fault: activate_direct_sensor_mode,
}

# --- Launch ---
odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")

launcher = Launcher()
launcher.kompass(
    components=[planner, controller, mapper, driver],
    events_actions=events_actions,
    activate_all_components_on_start=True,
    multi_processing=True,
)
launcher.inputs(location=odom_topic)
launcher.robot = my_robot
launcher.bringup()
```
