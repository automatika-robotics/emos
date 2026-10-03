# Navigating a Real Robot

The navigation recipes so far ran in simulation, on plain ROS topics that the simulator happened to publish. A real robot is not that tidy. Its odometry arrives in a vendor protocol, its LiDAR needs a driver started, its velocity commands have to be encoded for its motion controller, and it has moods of its own, a battery, a fall, an obstacle in front of its nose, that the navigation stack should know about. A [plugin](../../concepts/robot-plugins.md) adapts all of that to the interfaces the stack uses, and this tutorial runs the Kompass stack from [Point Navigation](point-navigation.md) on a real quadruped, the DeepRobotics Lite3, through its plugin.

Nothing in the stack changes. What changes is where its inputs come from and where its output goes.

## Installing the plugin

```bash
emos plugin install emos-plugin-lite3
emos plugin inspect emos-plugin-lite3
```

The install builds the plugin and resolves the drivers its manifest declares, the Livox and RealSense drivers for the Lite3. `inspect` prints what the plugin offers: its feedback streams and their keys, its commands, its actions and events, where its sensors sit, and how it maps. That listing is the reference for every binding below. [Plugins](../../getting-started/plugins.md) covers installing and updating.

## Attaching it

```python
from ros_sugar import Launcher
from lite3_plugin import Lite3Plugin

robot = Lite3Plugin()
launcher = Launcher()
launcher.add_plugin(robot)
```

The plugin carries the robot's description: a differential-drive model, a box footprint of the Lite3's size, its velocity and acceleration limits, and its base frame, `body`. The launcher hands them to every component at bringup, so the recipe sets no `launcher.robot` and no frames. A recipe that does set them wins over the plugin, and a unit that differs from the defaults is a subclass:

```python
class MyLite3(Lite3Plugin):
    MOTION_HOST_IP = "10.0.0.42"    # the robot on another subnet
    ROBOT_VX_MAX = 0.6              # a lower speed cap for this unit
```

## Localization from the plugin

The stack needs to know where the robot is, in the map frame. The Lite3 has no localizer of its own, so the plugin provides one: it fuses the leg odometry with the body IMU in a `robot_localization` filter and publishes the transforms from `map` through `odom` to `body`. The recipe binds that feed as the location of every component, and the plugin starts the filter because something asked for it:

```python
from ros_sugar.io import Topic

launcher.inputs(location=Topic(name="odometry_filtered", msg_type="Odometry", use_plugin=True))
```

`use_plugin=True` means the robot plugin serves the topic, and the name is the feedback key. The Lite3 has two odometry streams, the raw leg odometry under `Odometry` and the fused one under `odometry_filtered`, which is why the key matters here. A topic type the plugin has only one of, such as its `Twist` command, binds by type whatever it is called.

## Sensor data

The local mapper builds its grid from the Mid-360's point cloud. Binding the cloud is what starts the LiDAR driver: the launcher starts a driver only for feeds a recipe binds, so a recipe that never reads the cloud never runs it.

```python
from kompass.components import LocalMapper, LocalMapperConfig
from kompass.control import MapConfig

local_mapper = LocalMapper(
    component_name="mapper",
    config=LocalMapperConfig(loop_rate=5.0, map_params=MapConfig(width=4.0, height=4.0, resolution=0.1)),
)
local_mapper.inputs(sensor_data=Topic(name="lidar", msg_type="PointCloud2", use_plugin=True))
```

The cloud arrives on a native ROS topic, `/livox/lidar`, so the binding just points the subscriber at it. The LiDAR's frame is placed on the body by the plugin's mounts, published as a static transform, so the mapper knows where the sensor sits without a URDF. The robot's other streams bind the same way when a recipe wants them: the RealSense image under `camera`, the two ultrasound sensors under `ultrasound_front` and `ultrasound_back`, the battery, the joint states.

## Commands

The drive manager is the last link before the robot. Its output is a `Twist`, and bound to the plugin it goes out as the Lite3's own velocity packets over UDP, with the drive manager's safety zones still applied before anything is sent:

```python
from kompass.components import DriveManager, DriveManagerConfig

driver = DriveManager(
    component_name="drive_manager",
    config=DriveManagerConfig(critical_zone_distance=0.05, critical_zone_angle=90.0, slowdown_zone_distance=0.3),
)
driver.outputs(robot_command=Topic(name="Twist", msg_type="Twist", use_plugin=True))
```

While the recipe runs, the plugin keeps the robot under external control with a heartbeat, and when the recipe ends, however it ends, it hands the robot back to its handset.

## The map

Kompass's map server needs the site's map, and the plugin knows which one is active. The map is built with `emos map new`, which the plugin declares how to do for this robot, and made active with `emos map use`; see [Mapping](../../getting-started/mapping.md). The recipe then asks the plugin for it:

```python
from kompass.components import MapServer, MapServerConfig

map_server = MapServer(
    component_name="global_map_server",
    config=MapServerConfig(loop_rate=1.0, map_file_path=robot.MAPPING.active_grid_path()),
)
```

## The rest of the stack

The planner and the controller are set up exactly as in Point Navigation, and a goal arrives as a clicked point or from the recipe's web interface:

```python
from kompass.components import Controller, Planner, PlannerConfig
from kompass.control import ControllersID

planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
planner.run_type = "ActionServer"
goal = Topic(name="/clicked_point", msg_type="PointStamped")
planner.inputs(goal_point=goal)

controller = Controller(component_name="controller")
controller.algorithm = ControllersID.DWA
controller.direct_sensor = False   # local perception from the mapper's grid
```

## The robot's own events and actions

The plugin brings the robot's vocabulary with it. The Lite3 reports a low battery, an obstacle ahead of its front ultrasound, a disturbed balance and a fall, and its actions are its motion-host behaviours: sit or stand, the three gaits, a stop, and a few tricks. They register like any other event and action, and they sit naturally next to the stack's own events:

```python
from ros_sugar.actions import stop

launcher.on(robot.events.low_battery(15.0), robot.actions.sit_stand())
launcher.on(robot.events.fallen(), stop(component=controller))
```

Build them after the plugin is attached, since attaching binds the identity the events resolve against.

The Lite3 never acknowledges a command, so an action's result only says the command was sent. Its telemetry says whether anything happened, and keyword arguments given to an action factory go to the `Action` it builds, so an action can wait for the robot to confirm:

```python
status = robot.feedbacks["robot_status"].as_topic()
stand_up = robot.actions.sit_stand(success=status.msg.data == "standing", timeout=10.0)
```

This is a monitored action, described in [Events and Actions](../../concepts/events-and-actions.md#monitored-actions), and it works the same as a step of a routine.

## A sensor on top

A sensor that is not part of the robot is a sensor plugin, attached with a `Mount` that says where it sits, so its frame exists in TF. A thermal PTZ camera on the Lite3's back:

```python
from ros_sugar.robot import Mount
from hikmicro_plugin import HikmicroBispectrum

camera = HikmicroBispectrum(id="inspection_cam")
launcher.add_plugin(camera, mount=Mount(parent=robot, xyz=(0.15, 0.0, 0.35)))

thermal = Topic(name="thermal_image", msg_type="Image", use_plugin=camera.id)
launcher.on(camera.events.over_temperature(60.0), camera.actions.goto_preset(1))
```

A sensor plugin is named by its id in `use_plugin`, and it comes with events and actions of its own, here the camera's thermometry and its pan, tilt and presets.

## Running it

`emos info` shows, for every topic in the recipe, what provides it: the robot plugin, a sensor plugin by its id, or a plain ROS topic. Then run it, and the plugin starts the drivers the recipe bound:

```bash
emos info lite3_navigation
emos run lite3_navigation
```

## Complete code

```{code-block} python
:caption: The Kompass stack on a Lite3
:linenos:

from automatika_ros_sugar.msg import ComponentStatus
from kompass.components import (
    Controller,
    DriveManager,
    DriveManagerConfig,
    LocalMapper,
    LocalMapperConfig,
    MapServer,
    MapServerConfig,
    Planner,
    PlannerConfig,
    TopicsKeys,
)
from kompass.control import ControllersID, MapConfig
from kompass.ros import Action, Event, Launcher, Topic
from ros_sugar.actions import stop
from lite3_plugin import Lite3Plugin

# --- The robot ---
robot = Lite3Plugin()
launcher = Launcher()
launcher.add_plugin(robot)

# --- The map the robot was mapped with ---
map_server = MapServer(
    component_name="global_map_server",
    config=MapServerConfig(loop_rate=1.0, map_file_path=robot.MAPPING.active_grid_path()),
)

# --- Planning and control, as in Point Navigation ---
planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
planner.run_type = "ActionServer"
goal = Topic(name="/clicked_point", msg_type="PointStamped")
planner.inputs(goal_point=goal)

controller = Controller(component_name="controller")
controller.algorithm = ControllersID.DWA
controller.direct_sensor = False

# --- Local perception from the robot's LiDAR ---
local_mapper = LocalMapper(
    component_name="mapper",
    config=LocalMapperConfig(loop_rate=5.0, map_params=MapConfig(width=4.0, height=4.0, resolution=0.1)),
)
local_mapper.inputs(sensor_data=Topic(name="lidar", msg_type="PointCloud2", use_plugin=True))

# --- Commands out through the plugin ---
driver = DriveManager(
    component_name="drive_manager",
    config=DriveManagerConfig(critical_zone_distance=0.05, critical_zone_angle=90.0, slowdown_zone_distance=0.3),
)
driver.outputs(robot_command=Topic(name="Twist", msg_type="Twist", use_plugin=True))

# --- Events: the stack's and the robot's ---
emergency_stop = driver.get_out_topic(TopicsKeys.EMERGENCY)
robot_blocked = Event(
    emergency_stop.msg.data.is_true()
    | (controller.status_topic.msg.status == ComponentStatus.STATUS_FAILURE_ALGORITHM_LEVEL)
)
send_goal = Action(method=planner.trigger_main_action_server, args=(goal.msg.point.x, goal.msg.point.y, 0.05, 0.2))

launcher.add_pkg(
    components=[map_server, planner, controller, local_mapper, driver],
    events_actions={
        Event(goal): send_goal,
        robot_blocked: Action(method=driver.move_to_unblock),
    },
    package_name="kompass",
    multiprocessing=True,
)
launcher.on(robot.events.low_battery(15.0), robot.actions.sit_stand())
launcher.on(robot.events.fallen(), stop(component=controller))

# --- Localization from the plugin, for every component ---
launcher.inputs(location=Topic(name="odometry_filtered", msg_type="Odometry", use_plugin=True))

launcher.enable_ui(inputs=[planner.ui_main_action_input], outputs=[goal])
launcher.bringup()
```

```{seealso}
- [Robot Plugins](../../concepts/robot-plugins.md) for how a plugin works and what it declares.
- [Plugins](../../getting-started/plugins.md) for the catalog, installing, updating and removing.
- [Mapping](../../getting-started/mapping.md) for building the map this recipe navigates on.
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
