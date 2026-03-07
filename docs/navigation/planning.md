# Global Planner

**Global path planning and trajectory generation.**

The Planner component is responsible for finding an optimal or suboptimal path from a start to a goal location using complete map information (i.e. the global or reference map).

It leverages the **[Open Motion Planning Library (OMPL)](https://ompl.kavrakilab.org/)** backend to support various sampling-based algorithms (RRT*, PRM, etc.), capable of handling complex kinematic constraints. Collision checking is handled by the **[FCL (Flexible Collision Library)](https://github.com/flexible-collision-library/fcl)** for precise geometric collision detection.

## Available Run Types

Planner can be used with all four available Run Types:

```{list-table}
:widths: 20 80
* - **{material-regular}`schedule;1.2em;sd-text-primary` Timed**
  - **Periodic Re-planning.** Compute a new plan periodically (e.g., at 1Hz) from the robot's current location to the last received goal.

* - **{material-regular}`touch_app;1.2em;sd-text-primary` Event**
  - **Reactive Planning.** Trigger a new plan computation *only* when a new message is received on the `goal_point` topic.

* - **{material-regular}`dns;1.2em;sd-text-primary` Service**
  - **Request/Response.** Offers a standard ROS2 Service (`PlanPath`). Computes a single plan per request and returns it immediately.

* - **{material-regular}`hourglass_top;1.2em;sd-text-primary` Action Server**
  - **Long-Running Goal.** Offers a standard ROS2 Action. continuously computes and updates the plan until the goal is reached or canceled.
```

## Inputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - map
  - [`nav_msgs.msg.OccupancyGrid`](http://docs.ros.org/en/noetic/api/nav_msgs/html/msg/OccupancyGrid.html)
  - 1
  - `Topic(name="/map", msg_type="OccupancyGrid", qos_profile=QoSConfig(durability=TRANSIENT_LOCAL))`

* - goal_point
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.PointStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PointStamped.html)
  - 1
  - `Topic(name="/goal", msg_type="PointStamped")`

* - location
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.Pose`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/Pose.html)
  - 1
  - `Topic(name="/odom", msg_type="Odometry")`
```

:::{note}
`goal_point` input is only used if the Planner is running as TIMED or EVENT Component. In the other two types, the goal point is provided in the service request or the action goal.
:::

## Outputs

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

* - reached_end
  - `std_msgs.msg.Bool`
  - 1
  - `Topic(name="/reached_end", msg_type="Bool")`
```

## OMPL Algorithms

EMOS integrates over 25 OMPL geometric planners. See the [Planning Algorithms (OMPL)](../advanced/algorithms.md#planning-algorithms-ompl) section of the Algorithms Reference for a complete list with benchmarks and per-planner configuration parameters.

## Collision Checking (FCL)

[FCL](https://github.com/flexible-collision-library/fcl) is a generic library for performing proximity and collision queries on geometric models. EMOS leverages FCL to perform precise collision checks between the robot's kinematic model and both static (map) and dynamic (sensor) obstacles during path planning and control.

## Usage Example

```python
from kompass.components import Planner, PlannerConfig
from kompass.config import ComponentRunType
from kompass.ros import Topic
from kompass_core.models import RobotType, RobotConfig, RobotGeometry, LinearCtrlLimits, AngularCtrlLimits
import numpy as np

# Configure your robot
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=1.0, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=1.0, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)

# Setup the planner config
config = PlannerConfig(
    robot=my_robot,
    loop_rate=1.0   # 1Hz
)

planner = Planner(component_name="planner", config=config)

planner.run_type = ComponentRunType.EVENT   # Can also pass a string "Event"

# Add rviz clicked_point as input topic
goal_topic = Topic(name="/clicked_point", msg_type="PoseStamped")
planner.inputs(goal_point=goal_topic)
```
