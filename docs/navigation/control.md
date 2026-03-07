# Controller

**Motion control and dynamic obstacle avoidance.**

The Controller is the real-time "pilot" of your robot. While the [Planner](planning.md) looks ahead to find a global route, the Controller deals with the immediate reality — calculating velocity commands to follow the global path (path following) or a global target point (object following) while reacting to dynamic obstacles and adhering to kinematic constraints.

It supports modular **Plugins** allowing you to switch between different control strategies (e.g., *Pure Pursuit* vs *DWA* vs *Visual Servoing*) via configuration.

## Available Run Types

The Controller typically runs at a high frequency (10Hz-50Hz) to ensure smooth motion.

```{list-table}
:widths: 20 80
* - **{material-regular}`schedule;1.2em;sd-text-primary` Timed**
  - **Periodic Control Loop.** Computes a new velocity command periodically if all necessary inputs are available.

* - **{material-regular}`hourglass_top;1.2em;sd-text-primary` Action Server**
  - **Goal Tracking.** Offers a [`ControlPath`](https://github.com/automatika-robotics/kompass/blob/main/kompass_interfaces/action/ControlPath.action) ROS2 Action. Continuously computes control commands until the goal is reached or the action is preempted.

```

## Inputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - plan
  - [`nav_msgs.msg.Path`](http://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Path.html)
  - 1
  - `Topic(name="/plan", msg_type="Path")`

* - location
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.Pose`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/Pose.html)
  - 1
  - `Topic(name="/odom", msg_type="Odometry")`

* - sensor_data
  - [`sensor_msgs.msg.LaserScan`](https://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/LaserScan.html), [`sensor_msgs.msg.PointCloud2`](http://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/PointCloud2.html)
  - 1
  - `Topic(name="/scan", msg_type="LaserScan")`

* - local_map
  - [`nav_msgs.msg.OccupancyGrid`](http://docs.ros.org/en/noetic/api/nav_msgs/html/msg/OccupancyGrid.html)
  - 1
  - `Topic(name="/local_map/occupancy_layer", msg_type="OccupancyGrid")`

* - vision_tracking
  - [`automatika_embodied_agents.msg.Trackings`](https://github.com/automatika-robotics/ros-agents/tree/main/agents_interfaces/msg), [`automatika_embodied_agents.msg.Detections2D`](https://github.com/automatika-robotics/ros-agents/tree/main/agents_interfaces/msg)
  - 1
  - None, Should be provided to use the vision target tracking
```

```{tip}
Provide a `vision_tracking` input topic to the controller to activate the creation of a vision-based target following action server. See the [Vision Tracking tutorial](../recipes/navigation/vision-tracking-rgb.md) for more details.
```

## Outputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - command
  - [`geometry_msgs.msg.Twist`](http://docs.ros.org/en/noetic/api/geometry_msgs/html/msg/Twist.html)
  - 1
  - `Topic(name="/control", msg_type="Twist")`

* - multi_command
  - [`kompass_interfaces.msg.TwistArray`](https://github.com/automatika-robotics/kompass/tree/main/kompass_interfaces/msg)
  - 1
  - `Topic(name="/control_list", msg_type="TwistArray")`

* - interpolation
  - [`nav_msgs.msg.Path`](http://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Path.html)
  - 1
  - `Topic(name="/interpolated_path", msg_type="Path")`

* - local_plan
  - [`nav_msgs.msg.Path`](http://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Path.html)
  - 1
  - `Topic(name="/local_path", msg_type="Path")`

* - tracked_point
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.Pose`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/Pose.html), [`automatika_embodied_agents.msg.Detection2D`](https://github.com/automatika-robotics/ros-agents/tree/main/agents_interfaces/msg)
  - 1
  - `Topic(name="/tracked_point", msg_type="PoseStamped")`
```

## Algorithms

EMOS includes several production-ready control plugins suited for different environments:

- {material-regular}`route;1.2em;sd-text-primary` **[Stanley](../advanced/algorithms.md)** — Geometric path tracking using the front axle as reference. Best for Ackermann steering.

- {material-regular}`shield;1.2em;sd-text-primary` **[DVZ](../advanced/algorithms.md)** — Deformable Virtual Zone. Reactive collision avoidance based on risk zones. Extremely fast for crowded dynamic environments.

- {material-regular}`speed;1.2em;sd-text-primary` **[DWA](../advanced/algorithms.md)** — Dynamic Window Approach. Sample-based collision avoidance with GPU support. Considers kinematics to find optimal velocity.

- {material-regular}`visibility;1.2em;sd-text-primary` **[VisionFollower](../advanced/algorithms.md)** — Vision target following controller. Steers the robot to keep a visual target centered using RGB or depth data.

See the [Algorithms Reference](../advanced/algorithms.md) for detailed descriptions of each algorithm.

## Usage Example

```python
from kompass.components import Controller, ControllerConfig
from kompass.ros import Topic

# Setup custom configuration
my_config = ControllerConfig(loop_rate=10.0)

# Init a controller object
my_controller = Controller(component_name="controller", config=my_config)

# Change an input
my_controller.inputs(plan=Topic(name='/global_path', msg_type='Path'))

# Change run type (default "Timed")
my_controller.run_type = "ActionServer"

# Change plugin
my_controller.plugin = 'DWA'
```
