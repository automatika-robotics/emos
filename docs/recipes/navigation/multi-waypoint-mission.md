# Multi-Waypoint Missions

[Point Navigation](point-navigation.md) sends the robot to one place. A patrol, a delivery round or an inspection route is a sequence of places, with something to do at each stop: wait a while, wait for someone to press a button, and decide what to do when a stop cannot be reached. The [Mission Manager](../../navigation/mission-manager.md) runs such a sequence as one action, on top of the same stack, and the browser gets a card to build the mission on the map and follow it.

This recipe adds a mission to the point navigation stack in simulation.

## The stack

Everything from Point Navigation stays as it was, with one requirement: the planner runs as an action server, because that is how the mission sends it each waypoint.

```python
from kompass.components import Controller, DriveManager, DriveManagerConfig, LocalMapper, LocalMapperConfig, MapServer, MapServerConfig, Planner, PlannerConfig
from kompass.control import ControllersID, MapConfig

planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
planner.run_type = "ActionServer"

controller = Controller(component_name="controller")
controller.algorithm = ControllersID.DWA
controller.direct_sensor = False

driver = DriveManager(
    component_name="drive_manager",
    config=DriveManagerConfig(critical_zone_distance=0.05, critical_zone_angle=90.0, slowdown_zone_distance=0.3),
)
```

## The mission manager

The mission manager is given the three components it works through. It keeps only their names, so it can run in a process of its own like the rest:

```python
from kompass.components import MissionManager, MissionManagerConfig

mission = MissionManager(
    component_name="mission",
    planner=planner,
    controller=controller,
    drive_manager=driver,
    config=MissionManagerConfig(waypoint_timeout=180.0),
)
```

`waypoint_timeout` is how long one waypoint may take before the mission gives up on it. The defaults of the rest suit a first run: progress is read five times a second, a stop that fails is retried twice, and the routine every mission runs as is called `navigation_mission`.

## Reacting to the mission

A mission publishes its progress on `mission_status`, and pausing and resuming are component actions, so the mission can be wired into events like anything else. Here a `Bool` topic, published by a button in the browser or by another component, pauses and resumes it:

```python
from kompass.ros import Event, Topic
from ros_sugar.actions import log

hold = Topic(name="/hold_mission", msg_type="Bool")
pause_mission = Event(hold.msg.data.is_true(), on_change=True)
resume_mission = Event(hold.msg.data.is_false(), on_change=True)

events_actions = {
    pause_mission: [log(msg="Mission held"), Action(method=mission.pause_mission)],
    resume_mission: Action(method=mission.resume_mission),
}
```

## Launching

The mission's browser card needs its action, its method service and the waypoints topic as inputs, and the status and waypoints as outputs. The component collects them:

```python
from kompass.ros import Launcher

launcher = Launcher()
launcher.add_pkg(
    components=[map_server, planner, controller, local_mapper, driver, mission],
    events_actions=events_actions,
    package_name="kompass",
    multiprocessing=True,
)
launcher.inputs(location=Topic(name="/odometry/filtered", msg_type="Odometry"))
launcher.robot = my_robot
launcher.enable_ui(
    inputs=[hold, *mission.ui_inputs],
    outputs=[*mission.ui_outputs, map_server.get_out_topic(TopicsKeys.GLOBAL_MAP), planner.get_out_topic(TopicsKeys.GLOBAL_PLAN)],
)
launcher.bringup()
```

## Running a mission

Start the simulation as in [Simulation Quick Starts](simulation-quickstarts.md), run the recipe, and open `https://localhost:5001`, accepting the certificate once. The mission card lets you pick waypoints on the map, set the arrival tolerance and a dwell at each stop, and send the mission. As it runs, the card checks the waypoints off, shows which one the robot is heading for, and offers pause, resume and cancel.

<!-- TODO screenshot: the mission card in the recipe's web UI with waypoints on the map and the journey checklist -->

The same mission can be sent from a terminal, with a five second dwell at every stop:

```bash
ros2 action send_goal /mission/run_mission kompass_interfaces/action/MultiGoalPlanPath \
  "{goals: [{position: {x: 1.0, y: 0.0}, orientation: {w: 1.0}},
            {position: {x: 2.0, y: 1.0}, orientation: {w: 1.0}}],
    end_tolerance: {orientation_error: 0.2, lateral_distance_error: 0.15},
    pause_duration: [5.0]}" --feedback
```

Two things to try. Publish `true` on `/hold_mission` while the robot drives: it stops where it is, the card shows the mission paused, and `false` sends it on to the same waypoint again. And add a `pause_condition_topic` to the goal with a `Bool` topic of your own: the robot then waits at each stop until something publishes `true` there, for as long as `condition_timeout` allows, and `on_timeout` decides what happens if nothing does.

## Complete code

```{code-block} python
:caption: A mission on the point navigation stack
:linenos:

import os

import numpy as np
from ament_index_python.packages import get_package_share_directory
from kompass.components import (
    Controller,
    DriveManager,
    DriveManagerConfig,
    LocalMapper,
    LocalMapperConfig,
    MapServer,
    MapServerConfig,
    MissionManager,
    MissionManagerConfig,
    Planner,
    PlannerConfig,
    TopicsKeys,
)
from kompass.control import ControllersID, MapConfig
from kompass.robot import AngularCtrlLimits, LinearCtrlLimits, RobotConfig, RobotGeometryType, RobotType
from kompass.ros import Action, Event, Launcher, Topic
from ros_sugar.actions import log

kompass_sim_dir = get_package_share_directory(package_name="kompass_sim")

# --- The robot ---
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometryType.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(max_omega=0.4, max_acc=2.0, max_decel=2.0, max_ang=np.pi / 3),
)

# --- The stack ---
planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
planner.run_type = "ActionServer"

controller = Controller(component_name="controller")
controller.algorithm = ControllersID.DWA
controller.direct_sensor = False

driver = DriveManager(
    component_name="drive_manager",
    config=DriveManagerConfig(critical_zone_distance=0.05, critical_zone_angle=90.0, slowdown_zone_distance=0.3),
)
cmd_msg_type = "TwistStamped" if os.environ.get("ROS_DISTRO") in ["rolling", "jazzy", "kilted"] else "Twist"
driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_msg_type))

local_mapper = LocalMapper(
    component_name="mapper",
    config=LocalMapperConfig(map_params=MapConfig(width=3.0, height=3.0, resolution=0.05)),
)
local_mapper.inputs(sensor_data=Topic(name="/scan", msg_type="LaserScan"))

map_server = MapServer(
    component_name="global_map_server",
    config=MapServerConfig(
        map_file_path=os.path.join(kompass_sim_dir, "maps", "turtlebot3_webots.yaml"),
        grid_resolution=0.5,
    ),
)

# --- The mission manager ---
mission = MissionManager(
    component_name="mission",
    planner=planner,
    controller=controller,
    drive_manager=driver,
    config=MissionManagerConfig(waypoint_timeout=180.0),
)

# --- Hold and release the mission from a topic ---
hold = Topic(name="/hold_mission", msg_type="Bool")
events_actions = {
    Event(hold.msg.data.is_true(), on_change=True): [log(msg="Mission held"), Action(method=mission.pause_mission)],
    Event(hold.msg.data.is_false(), on_change=True): Action(method=mission.resume_mission),
}

# --- Launch ---
launcher = Launcher()
launcher.add_pkg(
    components=[map_server, planner, controller, local_mapper, driver, mission],
    events_actions=events_actions,
    package_name="kompass",
    multiprocessing=True,
)
launcher.inputs(location=Topic(name="/odometry/filtered", msg_type="Odometry"))
launcher.robot = my_robot
launcher.enable_ui(
    inputs=[hold, *mission.ui_inputs],
    outputs=[*mission.ui_outputs, map_server.get_out_topic(TopicsKeys.GLOBAL_MAP), planner.get_out_topic(TopicsKeys.GLOBAL_PLAN)],
)
launcher.bringup()
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
