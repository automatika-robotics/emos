# Point Navigation

In the [Simulation Quick Starts](simulation-quickstarts.md), you ran a script that launched a full navigation stack. Now, let's break that script down step-by-step to understand how to configure EMOS navigation for your specific needs.

## Step 1: Robot Configuration

The first step is to tell EMOS *what* it is driving. The `RobotConfig` object defines the physical constraints and kinematics of your platform. This is crucial because the **Controller** uses these limits to generate feasible velocity commands, and the **Planner** uses the geometry to check for collisions.

```python
# 1. Define the Robot
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,  # Motion Model (e.g., Turtlebot3)
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),     # Radius=0.1m, Height=0.3m

    # 2. Define Control Limits
    ctrl_vx_limits=LinearCtrlLimits(
        max_vel=0.4,    # Max speed (m/s)
        max_acc=1.5,    # Max acceleration (m/s^2)
        max_decel=2.5   # Max deceleration (braking)
    ),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=0.4,
        max_acc=2.0,
        max_decel=2.0,
        max_steer=np.pi / 3
    ),
)

```

:::{tip}
EMOS supports **Ackermann** (Car-like), **Differential Drive**, and **Omni-directional** models. Changing the `model_type` here automatically reconfigures the underlying control math.
:::

---

## Step 2: Core Components

Next, we initialize the "Brains" of the operation.

### The Planner & Controller

We use the **Pure Pursuit** algorithm for path tracking. Note the `direct_sensor=False` flag -- this tells the controller *not* to subscribe to raw sensor data directly, but to rely on the processed **Local Map** instead.

```python
# Global Planner (runs at 1Hz)
planner = Planner(
    component_name="planner",
    config=PlannerConfig(loop_rate=1.0)
)
planner.run_type = "Timed"

# Local Controller
controller = Controller(component_name="controller")
controller.algorithm = ControllersID.PURE_PURSUIT
controller.direct_sensor = False  # Use Local Mapper for perception

```

### The Drive Manager

This component sits between the controller and the motors. We configure **Safety Zones** here: if an obstacle breaches the `critical_zone_distance` (0.05m), the Drive Manager triggers a hardware-level stop, overriding the controller.

```python
driver_config = DriveManagerConfig(
    critical_zone_distance=0.05,  # Emergency Stop threshold
    critical_zone_angle=90.0,     # Frontal cone angle
    slowdown_zone_distance=0.3,   # Slow down threshold
)
driver = DriveManager(component_name="drive_manager", config=driver_config)

```

### Dynamic Message Types

Different ROS2 versions use different message types for velocity (`Twist` vs `TwistStamped`). This snippet makes your recipe portable across **Humble**, **Jazzy**, and **Rolling**.

```python
# Auto-detect ROS distribution
if "ROS_DISTRO" in os.environ and (
    os.environ["ROS_DISTRO"] in ["rolling", "jazzy", "kilted"]
):
    cmd_msg_type = "TwistStamped"
else:
    cmd_msg_type = "Twist"

# Bind the output topic
driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_msg_type))

```

---

## Step 3: Mapping & Perception

Navigation requires two types of maps: a **Static Global Map** for long-term planning, and a **Dynamic Local Map** for immediate obstacle avoidance.

```python
# 1. Local Mapper: Builds a 3x3m sliding window around the robot
local_mapper = LocalMapper(
    component_name="mapper",
    config=LocalMapperConfig(
        map_params=MapConfig(width=3.0, height=3.0, resolution=0.05)
    )
)

# 2. Map Server: Loads the static house map from a file
map_server = MapServer(
    component_name="global_map_server",
    config=MapServerConfig(
        map_file_path=os.path.join(kompass_sim_dir, "maps", "turtlebot3_webots.yaml"),
        grid_resolution=0.5
    )
)

```

---

## Step 4: The Launcher & UI

Finally, the `Launcher` ties everything together. It manages the lifecycle of all nodes and sets up the **Web UI**.

We use `enable_ui` to pipe data directly to the browser:

* **Outputs:** We stream the Global Map, the Planned Path, and the Robot's Odometry to the browser for visualization.

```python
launcher = Launcher()

# 1. Register Components
launcher.kompass(
    components=[map_server, controller, planner, driver, local_mapper],
    multiprocessing=True,
)

# 2. Bind Odometry (Input for all components)
odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")
launcher.inputs(location=odom_topic)

# 3. Apply Robot Config & Frames
launcher.robot = my_robot
launcher.frames = RobotFrames(world="map", odom="map", scan="LDS-01")

# 4. Enable the Web Interface
launcher.enable_ui(
    outputs=[
        map_server.get_out_topic(TopicsKeys.GLOBAL_MAP),
        odom_topic,
        planner.get_out_topic(TopicsKeys.GLOBAL_PLAN),
    ],
)

launcher.bringup()

```

---

## Full Recipe Code

Here is the complete script. You can save this as `nav_recipe.py` and run it in any workspace where EMOS is installed.

```python
import numpy as np
import os
from ament_index_python.packages import get_package_share_directory

from kompass.robot import (
    AngularCtrlLimits, LinearCtrlLimits, RobotGeometry, RobotType, RobotConfig, RobotFrames
)
from kompass.components import (
    Controller, DriveManager, DriveManagerConfig, Planner, PlannerConfig,
    LocalMapper, LocalMapperConfig, MapServer, MapServerConfig, TopicsKeys
)
from kompass.control import ControllersID, MapConfig
from kompass.ros import Topic, Launcher

def kompass_bringup():
    kompass_sim_dir = get_package_share_directory(package_name="kompass_sim")

    # 1. Robot Configuration
    my_robot = RobotConfig(
        model_type=RobotType.DIFFERENTIAL_DRIVE,
        geometry_type=RobotGeometry.Type.CYLINDER,
        geometry_params=np.array([0.1, 0.3]),
        ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
        ctrl_omega_limits=AngularCtrlLimits(max_vel=0.4, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3),
    )

    # 2. Components
    planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
    planner.run_type = "Timed"

    controller = Controller(component_name="controller")
    controller.algorithm = ControllersID.PURE_PURSUIT
    controller.direct_sensor = False

    driver = DriveManager(
        component_name="drive_manager",
        config=DriveManagerConfig(critical_zone_distance=0.05, slowdown_zone_distance=0.3)
    )

    # 3. Dynamic Command Type
    cmd_type = "TwistStamped" if os.environ.get("ROS_DISTRO") in ["rolling", "jazzy"] else "Twist"
    driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_type))

    # 4. Mapping
    local_mapper = LocalMapper(
        component_name="mapper",
        config=LocalMapperConfig(map_params=MapConfig(width=3.0, height=3.0, resolution=0.05))
    )

    map_server = MapServer(
        component_name="global_map_server",
        config=MapServerConfig(
            map_file_path=os.path.join(kompass_sim_dir, "maps", "turtlebot3_webots.yaml"),
            grid_resolution=0.5
        )
    )

    # 5. Launch
    launcher = Launcher()
    launcher.kompass(
        components=[map_server, controller, planner, driver, local_mapper],
        multiprocessing=True,
    )

    odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")
    launcher.inputs(location=odom_topic)

    launcher.robot = my_robot
    launcher.frames = RobotFrames(world="map", odom="map", scan="LDS-01")

    # 6. UI
    launcher.enable_ui(
        inputs=[planner.ui_main_action_input],
        outputs=[
            map_server.get_out_topic(TopicsKeys.GLOBAL_MAP),
            odom_topic,
            planner.get_out_topic(TopicsKeys.GLOBAL_PLAN),
        ],
    )

    launcher.bringup()

if __name__ == "__main__":
    kompass_bringup()

```

---

## Next Steps

Congratulations! You have created a full production-grade navigation recipe.

* **[Vision Tracking](vision-tracking.md)**: Replace the Pure Pursuit controller with a Vision Follower to chase targets.
* **[Runtime Model Fallback](../events-and-resilience/fallback-recipes.md)**: Learn how to make your recipe robust by automatically restarting components if they crash.
* **[Event-Driven Cognition](../events-and-resilience/event-driven-cognition.md)**: Make your system reactive (e.g., "If battery is low, go to dock") by adding Events and Actions.
