# Drive Manager

**Safety enforcement and command smoothing.**

The Drive Manager is the final gatekeeper before commands reach your robot's low-level interfaces. Its primary job is to ensure that every command falls within the robot's physical limits, satisfies smoothness constraints, and does not lead to a collision.

It acts as a safety shield, intercepting velocity commands from the Controller and applying **Emergency Stops** or **Slowdowns** based on immediate sensor data.

## Safety Layers

The Drive Manager implements a multi-stage safety pipeline:

| Layer | Description |
| :--- | :--- |
| **Emergency Stop** | Critical Zone. Checks proximity sensors directly. If an obstacle enters the configured safety distance and angle, the robot stops immediately. |
| **Dynamic Slowdown** | Warning Zone. If an obstacle enters the slowdown zone, the robot's velocity is proportionally reduced. |
| **Control Limiting** | Kinematic Constraints. Clamps incoming velocity and acceleration commands to the robot's physical limits. |
| **Smoothing** | Jerk Control. Applies smoothing filters to incoming commands to prevent jerky movements and wheel slip. |

```{figure} /_static/images/diagrams/drive_manager_light.png
:class: light-only
:alt: Emergency Zone & Slowdown Zone
:align: center
:width: 70%

Emergency Zone & Slowdown Zone
```

```{figure} /_static/images/diagrams/drive_manager_dark.png
:class: dark-only
:alt: Emergency Zone & Slowdown Zone
:align: center
:width: 70%
```

:::{tip}
Critical and Slowdown Zone checking is implemented in C++ via kompass-core. The implementation supports both **GPU** and **CPU** acceleration (defaults to GPU if available) for minimal latency.
:::

## Built-in Actions

The Drive Manager provides built-in behaviors for direct control and recovery. These can be triggered via [Events](../concepts/events-and-actions.md):

- **move_forward** — Moves the robot forward for `max_distance` meters, if the path is clear.
- **move_backward** — Moves the robot backward for `max_distance` meters, if the path is clear.
- **rotate_in_place** — Rotates the robot for `max_rotation` radians, checking the safety margin.
- **move_to_unblock** — Recovery behavior. Automatically attempts to move forward, backward, or rotate to free the robot from a collision state or blockage.

:::{warning}
All movement actions require active 360° sensor data (`LaserScan` or `PointCloud2`) to verify that the movement direction is collision-free.
:::

## Interface

### Inputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **command** | `Twist`, `TwistStamped` | `/control` |
| **multi_command** | `TwistArray` | `/control_list` |
| **sensor_data** | `LaserScan`, `Float64`, `Float32`, `PointCloud2` | `/scan` (up to 10) |
| **location** | `Odometry`, `PoseStamped`, `Pose` | `/odom` |

### Outputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **robot_command** | `Twist`, `TwistStamped` | `/cmd_vel` |
| **emergency_stop** | `Bool` | `/emergency_stop` |

## Usage Example

```python
from kompass.components import DriveManager, DriveManagerConfig
from kompass.ros import Topic

# 1. Configuration
my_config = DriveManagerConfig(
    closed_loop=True,
    critical_zone_distance=0.1,    # Stop if obstacle < 10cm
    slowdown_zone_distance=0.3,    # Slow down if obstacle < 30cm
    critical_zone_angle=90.0       # Check 90 degrees cone in front
)

# 2. Instantiate
driver = DriveManager(component_name="driver", config=my_config)

# 3. Remap Outputs
driver.outputs(robot_command=Topic(name='/my_robot_cmd', msg_type='TwistStamped'))
```
