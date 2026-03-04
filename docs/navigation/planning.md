# Path Planning

**Global path planning and trajectory generation.**

The Planner component is responsible for finding an optimal or suboptimal path from a start to a goal location using complete map information (i.e. the global or reference map).

It leverages the **[Open Motion Planning Library (OMPL)](https://ompl.kavrakilab.org/)** backend to support various sampling-based algorithms (RRT*, PRM, etc.), capable of handling complex kinematic constraints. Collision checking is handled by the **[FCL (Flexible Collision Library)](https://github.com/flexible-collision-library/fcl)** for precise geometric collision detection.

## Run Types

The Planner is flexible and can be executed in four different modes:

| Mode | Description |
| :--- | :--- |
| **Timed** | Periodic re-planning. Compute a new plan periodically (e.g., at 1Hz) from the robot's current location to the last received goal. |
| **Event** | Reactive planning. Trigger a new plan computation only when a new message is received on the `goal_point` topic. |
| **Service** | Request/Response. Offers a standard ROS2 Service. Computes a single plan per request and returns it immediately. |
| **Action Server** | Long-running goal. Offers a standard ROS2 Action, continuously computing and updating the plan until the goal is reached or canceled. |

## Interface

### Inputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **map** | `OccupancyGrid` | `/map` |
| **goal_point** | `Odometry`, `PoseStamped`, `PointStamped` | `/goal` (`PointStamped`) |
| **location** | `Odometry`, `PoseStamped`, `Pose` | `/odom` (`Odometry`) |

:::{note}
`goal_point` input is only used if the Planner is running as TIMED or EVENT. In other modes, the goal point is provided in the service request or action goal.
:::

### Outputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **plan** | `Path` | `/plan` |
| **reached_end** | `Bool` | `/reached_end` |

## Usage Example

```python
import numpy as np
from kompass.components import Planner, PlannerConfig
from kompass.config import ComponentRunType
from kompass.ros import Topic
from kompass.control import RobotType, RobotConfig, RobotGeometry, LinearCtrlLimits, AngularCtrlLimits

# 1. Configure your Robot Constraints
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=1.0, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(max_vel=1.0, max_acc=2.0)
)

# 2. Configure the Planner
config = PlannerConfig(robot=my_robot, loop_rate=1.0)

# 3. Instantiate
planner = Planner(component_name="planner", config=config)

# 4. Set Run Type & Remap Topics
planner.run_type = ComponentRunType.EVENT
planner.inputs(goal_point=Topic(name="/clicked_point", msg_type="PoseStamped"))
```

## OMPL Algorithms

EMOS integrates over 25 OMPL geometric planners. See the [Algorithms Reference](../advanced/algorithms.md) for a complete list with benchmarks and per-planner configuration parameters.

## Collision Checking (FCL)

[FCL](https://github.com/flexible-collision-library/fcl) is a generic library for performing proximity and collision queries on geometric models. EMOS leverages FCL to perform precise collision checks between the robot's kinematic model and both static (map) and dynamic (sensor) obstacles during path planning and control.
