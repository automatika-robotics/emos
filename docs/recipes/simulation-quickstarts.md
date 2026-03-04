# Simulation Quick Starts

Ready to see EMOS in action? This page walks you through launching a full autonomous navigation stack in simulation using either **Webots** or **Gazebo**. Each section is self-contained -- pick the simulator you prefer and follow along.

---

## Webots Simulator

**Launch a full autonomous navigation stack in under 5 minutes.**

In this tutorial, we use a single Python script -- a "Recipe" -- to build a complete point-to-point navigation system. We will use the [Webots](https://github.com/cyberbotics/webots_ros2) simulator and a [Turtlebot3](https://emanual.robotis.com/docs/en/platform/turtlebot3/overview/#notices) to demonstrate how EMOS components link together.

### 1. Prepare the Environment

To make things easy, we created **kompass_sim**, a package with ready-to-launch simulation environments.

1.  **Build the Simulation:**
    Clone and build the simulator support package in your ROS2 workspace:
    ```bash
    git clone https://github.com/automatika-robotics/kompass-sim.git
    cd .. && rosdep install --from-paths src --ignore-src -r -y
    colcon build --packages-select kompass_sim
    source install/setup.bash
    ```

2.  **Launch Webots:**
    Start the Turtlebot3 simulation world. This will bring up Webots, RViz, and the robot localization nodes:
    ```bash
    ros2 launch kompass_sim webots_turtlebot3.launch.py
    ```

### 2. The Navigation Recipe

The power of EMOS lies in its Python API. Instead of complex XML/YAML launch files, you define your navigation logic in a clean script.

**Create a file named `quick_start.py` and paste the following code:**

```python
import numpy as np
import os
from ament_index_python.packages import (
    get_package_share_directory,
)

# IMPORT ROBOT CONFIG PRIMITIVES
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotGeometry,
    RobotType,
    RobotConfig,
    RobotFrames,
)

# IMPORT EMOS NAVIGATION COMPONENTS
from kompass.components import (
    Controller,
    DriveManager,
    DriveManagerConfig,
    Planner,
    PlannerConfig,
    LocalMapper,
    LocalMapperConfig,
    MapServer,
    MapServerConfig,
    TopicsKeys,
)

# IMPORT ALGORITHMS CONFIG
from kompass.control import ControllersID, MapConfig

# IMPORT ROS PRIMITIVES
from kompass.ros import Topic, Launcher, Event, Action, actions


kompass_sim_dir = get_package_share_directory(package_name="kompass_sim")

# Setup your robot configuration
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=0.4, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)

# Configure the Global Planner
planner_config = PlannerConfig(loop_rate=1.0)
planner = Planner(component_name="planner", config=planner_config)
planner.run_type = "Timed"

# Configure the motion controller
controller = Controller(component_name="controller")
controller.algorithm = ControllersID.PURE_PURSUIT
controller.direct_sensor = (
    False  # Get local perception from a "map" instead (from the local mapper)
)

# Configure the Drive Manager (Direct commands sending to robot)
driver_config = DriveManagerConfig(
    critical_zone_distance=0.05,
    critical_zone_angle=90.0,
    slowdown_zone_distance=0.3,
)
driver = DriveManager(component_name="drive_manager", config=driver_config)
# Publish Twist or TwistStamped from the DriveManager based on the distribution
if "ROS_DISTRO" in os.environ and (
    os.environ["ROS_DISTRO"] in ["rolling", "jazzy", "kilted"]
):
    cmd_msg_type: str = "TwistStamped"
else:
    cmd_msg_type = "Twist"

driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_msg_type))

# Configure a Local Mapper
local_mapper_config = LocalMapperConfig(
    map_params=MapConfig(width=3.0, height=3.0, resolution=0.05)
)
local_mapper = LocalMapper(component_name="mapper", config=local_mapper_config)

# Configure the global Map Server
map_file = os.path.join(kompass_sim_dir, "maps", "turtlebot3_webots.yaml")
map_server_config = MapServerConfig(
    loop_rate=1.0,
    map_file_path=map_file,  # Path to a 2D map yaml file or a point cloud file
    grid_resolution=0.5,
    pc_publish_row=False,
)
map_server = MapServer(component_name="global_map_server", config=map_server_config)

# Setup the launcher
launcher = Launcher()

# Add navigation components
launcher.kompass(
    components=[map_server, controller, planner, driver, local_mapper],
    multiprocessing=True,
)

# Get odom from localizer filtered odom for all components
odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")
launcher.inputs(location=odom_topic)

# Set the robot config for all components
launcher.robot = my_robot
launcher.frames = RobotFrames(world="map", odom="map", scan="LDS-01")

# Enable the UI
# Outputs: Static Map, Global Plan, Robot Odometry
launcher.enable_ui(
    outputs=[
        map_server.get_out_topic(TopicsKeys.GLOBAL_MAP),
        odom_topic,
        planner.get_out_topic(TopicsKeys.GLOBAL_PLAN),
    ],
)

# Run the Recipe
launcher.bringup()

```

### 3. Run and Navigate

Open a new terminal and run your recipe:

```bash
python3 quick_start.py

```

You will see the components starting up in the terminal. Once ready, you have two ways to control the robot.

#### Option A: The EMOS Web UI

The recipe includes `launcher.enable_ui(...)`, which automatically spins up a lightweight web interface for monitoring and control.

1. **Check Terminal:** Look for a log message indicating the UI URL: `http://0.0.0.0:5001`.
2. **Open Browser:** Navigate to that URL.
3. **Send Goal:** You will see the map and the robot's live position. Simply click the publish point button and **click anywhere on the map** to trigger the Planner and send the robot to that location.

#### Option B: RViz

If you prefer the standard ROS tools:

1. Go to the **RViz** window launched in Step 1.
2. Select the **Publish Point** tool (sometimes called `Clicked Point`) from the top toolbar.
3. Click anywhere on the map grid.
4. The robot will plan a path (Blue Line) and immediately start driving.

### What just happened?

* **Components**: You configured your robot and the navigation components directly in your Python recipe.
* **Launcher**: Automatically managed the lifecycle of 5 ROS2 nodes in multi-processing.
* **Web UI**: Visualized the map, plan, and odometry topics instantly without installing extra frontend tools.

---

## Gazebo Simulator

**Launch a full autonomous navigation stack in under 5 minutes.**

In this tutorial, we use a single Python script -- a "Recipe" -- to build a complete point-to-point navigation system. We will use the [Gazebo](https://gazebosim.org/docs/latest/getstarted/) simulator and a [Turtlebot3 Waffle Pi](https://emanual.robotis.com/docs/en/platform/turtlebot3/overview/#notices) to demonstrate how EMOS components link together.

### 1. Install Gazebo

If you haven't already, install the default Gazebo version for your ROS distribution (replace `${ROS_DISTRO}` with `humble`, `jazzy`, or `rolling`):

```bash
sudo apt-get install ros-${ROS_DISTRO}-ros-gz

```

### 2. Prepare the Environment

To make things easy, we created **kompass_sim**, a package with ready-to-launch simulation environments.

1. **Build the Simulation:**
Clone and build the simulator support package in your ROS2 workspace:
```bash
git clone https://github.com/automatika-robotics/kompass-sim.git
cd .. && rosdep install --from-paths src --ignore-src -r -y
colcon build --packages-select kompass_sim
source install/setup.bash

```

2. **Set the Model:**
Tell the simulation to use the "Waffle Pi" model:
```bash
export TURTLEBOT3_MODEL=waffle_pi

```

3. **Launch Gazebo:**
Start the Turtlebot3 house simulation. This will bring up Gazebo, RViz, and the localization nodes:
```bash
ros2 launch kompass_sim gazebo_turtlebot3_house.launch.py

```

### 3. The Navigation Recipe

The power of EMOS lies in its Python API. Instead of complex XML/YAML launch files, you define your navigation logic in a clean script.

**Create a file named `quick_start_gz.py` and paste the following code:**

```python
import numpy as np
import os
from ament_index_python.packages import get_package_share_directory

# IMPORT ROBOT CONFIG PRIMITIVES
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotGeometry,
    RobotType,
    RobotConfig,
    RobotFrames,
)

# IMPORT EMOS NAVIGATION COMPONENTS
from kompass.components import (
    Controller,
    DriveManager,
    DriveManagerConfig,
    Planner,
    PlannerConfig,
    LocalMapper,
    LocalMapperConfig,
    MapServer,
    MapServerConfig,
    TopicsKeys,
)

# IMPORT ALGORITHMS CONFIG
from kompass.control import ControllersID, MapConfig

# IMPORT ROS PRIMITIVES
from kompass.ros import Topic, Launcher, Event, Action, actions


kompass_sim_dir = get_package_share_directory(package_name="kompass_sim")

# Setup your robot configuration (Turtlebot3 Waffle Pi)
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.BOX, # Waffle Pi is rectangular
    geometry_params=np.array([0.3, 0.3, 0.2]), # Length, Width, Height
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.26, max_acc=1.0, max_decel=1.0),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=1.8, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)

# Configure the Global Planner
planner_config = PlannerConfig(loop_rate=1.0)
planner = Planner(component_name="planner", config=planner_config)
planner.run_type = "Timed"

# Configure the motion controller
controller = Controller(component_name="controller")
controller.algorithm = ControllersID.PURE_PURSUIT
controller.direct_sensor = (
    False  # Get local perception from a "map" instead (from the local mapper)
)

# Configure the Drive Manager (Direct commands sending to robot)
driver_config = DriveManagerConfig(
    critical_zone_distance=0.05,
    critical_zone_angle=90.0,
    slowdown_zone_distance=0.3,
)
driver = DriveManager(component_name="drive_manager", config=driver_config)

# Handle Twist/TwistStamped compatibility
if "ROS_DISTRO" in os.environ and (
    os.environ["ROS_DISTRO"] in ["rolling", "jazzy", "kilted"]
):
    cmd_msg_type: str = "TwistStamped"
else:
    cmd_msg_type = "Twist"

driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type=cmd_msg_type))

# Configure a Local Mapper
local_mapper_config = LocalMapperConfig(
    map_params=MapConfig(width=3.0, height=3.0, resolution=0.05)
)
local_mapper = LocalMapper(component_name="mapper", config=local_mapper_config)

# Configure the global Map Server
# Note: We use the 'house' map to match the Gazebo world
map_file = os.path.join(kompass_sim_dir, "maps", "turtlebot3_gazebo_house.yaml")
map_server_config = MapServerConfig(
    loop_rate=1.0,
    map_file_path=map_file,
    grid_resolution=0.5,
    pc_publish_row=False,
)
map_server = MapServer(component_name="global_map_server", config=map_server_config)

# Setup the launcher
launcher = Launcher()

# Add navigation components
launcher.kompass(
    components=[map_server, controller, planner, driver, local_mapper],
    multiprocessing=True,
)

# Get odom from localizer filtered odom for all components
odom_topic = Topic(name="/odometry/filtered", msg_type="Odometry")
launcher.inputs(location=odom_topic)

# Set the robot config and frames
launcher.robot = my_robot
# Standard Gazebo TB3 frames: world=map, odom=odom, scan=base_scan
launcher.frames = RobotFrames(world="map", odom="odom", scan="base_scan")

# Enable the UI
# Outputs: Static Map, Global Plan, Robot Odometry
launcher.enable_ui(
    outputs=[
        map_server.get_out_topic(TopicsKeys.GLOBAL_MAP),
        odom_topic,
        planner.get_out_topic(TopicsKeys.GLOBAL_PLAN),
    ],
)

# Run the Recipe
launcher.bringup()

```

### 4. Run and Navigate

Open a new terminal and run your recipe:

```bash
python3 quick_start_gz.py

```

You will see the components starting up in the terminal. Once ready, you have two ways to control the robot.

#### Option A: The EMOS Web UI

The recipe includes `launcher.enable_ui(...)`, which automatically spins up a lightweight web interface for monitoring and control.

1. **Check Terminal:** Look for a log message indicating the UI URL: `http://0.0.0.0:5001`.
2. **Open Browser:** Navigate to that URL.
3. **Send Goal:** You will see the map and the robot's live position. Simply click the publish point button and **click anywhere on the map** to trigger the Planner and send the robot to that location.

#### Option B: RViz

If you prefer the standard ROS tools:

1. Go to the **RViz** window launched in Step 1.
2. Select the **Publish Point** tool (sometimes called `Clicked Point`) from the top toolbar.
3. Click anywhere on the map grid.
4. The robot will plan a path (Blue Line) and immediately start driving.

### What just happened?

* **Customization**: We adapted the robot configuration (`RobotConfig`) to match the Waffle Pi's rectangular geometry and adjusted the `RobotFrames` to match Gazebo's standard output (`base_scan`).
* **Launcher**: Managed the lifecycle of the entire stack.
* **Perception**: The Local Mapper is processing the Gazebo laser scan to provide obstacle avoidance data to the Controller.

---

## Next Steps

:::{tip}
Check the [Point Navigation](point-navigation.md) recipe for a deep dive into these recipes.
:::
