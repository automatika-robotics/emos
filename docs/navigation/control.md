# Controller

**Motion control and dynamic obstacle avoidance.**

The Controller is the real-time "pilot" of your robot. While the [Planner](planning.md) looks ahead to find a global route, the Controller deals with the immediate reality — calculating velocity commands to follow the global path (path following) or a global target point (object following) while reacting to dynamic obstacles and adhering to kinematic constraints.

It supports modular **Plugins** allowing you to switch between different control strategies (e.g., *Pure Pursuit* vs *DWA* vs *Visual Servoing*) via configuration.

## Run Types

The Controller typically runs at a high frequency (10Hz-50Hz) to ensure smooth motion.

| Mode | Description |
| :--- | :--- |
| **Timed** | Periodic control loop. Computes a new velocity command periodically if all necessary inputs are available. |
| **Action Server** | Goal tracking. Offers a ROS2 Action, continuously computing control commands until the goal is reached or preempted. |

## Interface

### Inputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **plan** | `Path` | `/plan` |
| **location** | `Odometry`, `PoseStamped`, `Pose` | `/odom` |
| **sensor_data** | `LaserScan`, `PointCloud2` | `/scan` |
| **local_map** | `OccupancyGrid` | `/local_map/occupancy_layer` |
| **vision_detections** | `Trackings`, `Detections2D` | None (provide to enable vision tracking) |

:::{tip}
Provide a `vision_detections` input topic to activate the vision-based target following action server. See the [Vision Tracking tutorial](../recipes/navigation/vision-tracking.md) for details.
:::

### Outputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **command** | `Twist`, `TwistStamped` | `/control` |
| **multi_command** | `TwistArray` | `/control_list` |
| **interpolation** | `Path` | `/interpolated_path` |
| **local_plan** | `Path` | `/local_path` |
| **tracked_point** | `PoseStamped` | `/tracked_point` |

## Algorithms

EMOS includes several production-ready control plugins suited for different environments:

| Algorithm | Type | Description |
| :--- | :--- | :--- |
| **Pure Pursuit** | Path Tracking | Geometric path tracking. Calculates the curvature to move from current position to a look-ahead point on the path. |
| **Stanley** | Front-Wheel Feedback | Geometric path tracking using the front axle as reference. Best for Ackermann steering. |
| **DWA** | Dynamic Window Approach | Sample-based collision avoidance with GPU support. Considers kinematics to find optimal velocity. |
| **DVZ** | Deformable Virtual Zone | Reactive collision avoidance based on risk zones. Extremely fast for crowded dynamic environments. |
| **VisionFollowerRGB** | Visual Servoing | Steers the robot to keep a visual target centered in the camera frame. |
| **VisionFollowerRGBD** | Depth-Aware Servoing | Maintains specific distance and orientation relative to a target using depth data. Perfect for "Follow Me" behaviors. |

See the [Algorithms Reference](../advanced/algorithms.md) for detailed descriptions of each algorithm.

## Usage Example

```python
from kompass.components import Controller, ControllerConfig
from kompass.ros import Topic

# 1. Configuration
my_config = ControllerConfig(loop_rate=20.0)

# 2. Instantiate
my_controller = Controller(component_name="controller", config=my_config)

# 3. Setup
my_controller.inputs(plan=Topic(name='/global_path', msg_type='Path'))
my_controller.run_type = "ActionServer"
my_controller.algorithm = 'DWA'
```
