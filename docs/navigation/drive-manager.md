# Drive Manager

**Safety enforcement and command smoothing.**

The Drive Manager is the final gatekeeper before commands reach your robot's low-level interfaces. Its primary job is to ensure that every command falls within the robot's physical limits, satisfies smoothness constraints, and does not lead to a collision.

It acts as a safety shield, intercepting velocity commands from the Controller and applying **Emergency Stops** or **Slowdowns** based on immediate sensor data.

## Safety Layers

The Drive Manager implements a multi-stage safety pipeline:

- {material-regular}`block;1.2em;sd-text-danger` **Emergency Stop** — Critical Zone. Checks proximity sensors directly. If an obstacle enters the configured safety distance and angle, the robot stops immediately.

- {material-regular}`slow_motion_video;1.2em;sd-text-warning` **Dynamic Slowdown** — Warning Zone. If an obstacle enters the slowdown zone, the robot's velocity is proportionally reduced.

- {material-regular}`tune;1.2em;sd-text-primary` **Control Limiting** — Kinematic Constraints. Clamps incoming velocity and acceleration commands to the robot's physical limits.

- {material-regular}`filter_alt;1.2em;sd-text-primary` **Control Smoothing** — Jerk Control. Applies smoothing filters to incoming commands to prevent jerky movements and wheel slip.

- {material-regular}`lock_open;1.2em;sd-text-primary` **Robot Unblocking** — Moves the robot forward, backwards or rotates in place if the space is free to move the robot away from a blocking point. This action can be configured to be triggered with an external event.

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

The safety check runs over every sensor given as `sensor_data` and takes the most cautious result: a LiDAR scan, any number of point clouds, from front and rear 3D LiDARs or depth cameras, fused with their mount transforms taken from TF, and single-beam `Range` sensors such as ultrasound, each classified into the forward or backward zone by where it is mounted. A sensor whose data is older than `sensor_data_timeout` is stale; by default that stops the robot, and `stale_sensor_policy="skip"` runs the check on the remaining sensors instead.

```{note}
Critical and Slowdown Zone checking is implemented in C++ in [kompass-core](https://github.com/automatika-robotics/kompass-core) for fast emergency behaviors. The core implementation supports both **GPU** and **CPU** (**defaults to GPU if available**).
```

## Built-in Actions

The Drive Manager provides built-in behaviors for direct control and recovery. These can be triggered via [Events](../concepts/events-and-actions.md):

```{list-table}
:widths: 20 70
:header-rows: 1

* - Action
  - Function

* - **move_forward**
  - Moves the robot forward for `max_distance` meters, if the forward direction is clear of obstacles.

* - **move_backward**
  - Moves the robot backwards for `max_distance` meters, if the backward direction is clear of obstacles.

* - **rotate_in_place**
  - Rotates the robot in place for `max_rotation` radians, if the given safety margin around the robot is clear of obstacles.

* - **move_to_unblock**
  - Recovery behavior. Automatically attempts to move forward, backward, or rotate to free the robot from a collision state or blockage. Takes optional `max_distance_forward`, `max_distance_backwards` and `max_rotation` limits.

* - **stop_robot**
  - Clears the command queue and publishes zero velocity until the robot is at rest, or `stop_timeout` seconds have passed. What a [mission](mission-manager.md) calls at every waypoint.
```

Every action returns `(success, message)`, and the movement actions consult the safety sensors, a LiDAR scan, point clouds or range sensors, to check that the direction is clear before moving.

## Available Run Types

```{list-table}
:widths: 10 80

* - **Timed**
  - Sends incoming command periodically to the robot.
```

## Inputs

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

* - sensor_data
  - [`sensor_msgs.msg.LaserScan`](https://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/LaserScan.html), [`sensor_msgs.msg.PointCloud2`](http://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/PointCloud2.html), [`sensor_msgs.msg.Range`](https://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/Range.html)
  - 1 + (10 optional)
  - `Topic(name="/scan", msg_type="LaserScan")`

* - location
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.Pose`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/Pose.html)
  - 1
  - `Topic(name="/odom", msg_type="Odometry")`
```

## Outputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - robot_command
  - [`geometry_msgs.msg.Twist`](http://docs.ros.org/en/noetic/api/geometry_msgs/html/msg/Twist.html), [`geometry_msgs.msg.TwistStamped`](http://docs.ros.org/en/noetic/api/geometry_msgs/html/msg/TwistStamped.html)
  - 1
  - `Topic(name="/cmd_vel", msg_type="Twist")`

* - emergency_stop
  - `std_msgs.msg.Bool`
  - 1
  - `Topic(name="/emergency_stop", msg_type="Bool")`
```

## Configuration

| Field                      | Default   | Effect                                                                                        |
| :------------------------- | :-------- | :-------------------------------------------------------------------------------------------- |
| `critical_zone_distance`   | 0.05 m    | An obstacle closer than this, inside the critical cone, stops the robot.                      |
| `critical_zone_angle`      | 45°       | The width of the critical cone in front of, and behind, the robot.                            |
| `slowdown_zone_distance`   | 0.2 m     | An obstacle closer than this slows the robot down in proportion.                              |
| `disable_safety_stop`      | false     | Turns the emergency stop off, for a simulator that does not need it.                          |
| `use_without_scan_sensor`  | false     | Runs with no safety sensor at all, which disables the movement actions' checks.               |
| `sensor_data_timeout`      | 0.2 s     | Sensor data older than this is stale.                                                         |
| `stale_sensor_policy`      | `stop`    | What a stale sensor means: `stop` the robot, or `skip` that sensor.                           |
| `pc_min_height`            | 0.0 m     | Points below this height are ignored in point clouds.                                         |
| `closed_loop`, `closed_loop_span`, `cmd_tolerance` | false, 3, 0.05 | Send commands in closed loop against the robot's measured velocity. |
| `smooth_commands`          | false     | Filters incoming commands against jerk.                                                       |
| `use_gpu`                  | true      | Run the zone check on the GPU when there is one.                                              |

## Usage Example

```python
from kompass.components import DriveManager, DriveManagerConfig
from kompass.ros import Topic

# Setup custom configuration
# closed_loop: send commands to the robot in closed loop (checks feedback from robot state)
# critical_zone_distance: for emergency stop (m)
my_config = DriveManagerConfig(
    closed_loop=True,
    critical_zone_distance=0.1,    # Stop if obstacle < 10cm
    slowdown_zone_distance=0.3,    # Slow down if obstacle < 30cm
    critical_zone_angle=90.0       # Check 90 degrees cone in front
)

# Instantiate
driver = DriveManager(component_name="driver", config=my_config)

# Remap Outputs
driver.outputs(robot_command=Topic(name='/my_robot_cmd', msg_type='Twist'))
```
