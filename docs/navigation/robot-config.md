# Robot Configuration

Before EMOS can drive a robot it needs to know what the robot is: how it moves, how big it is, and how fast it may go. That description is a `RobotConfig`, and the launcher hands it to every navigation component. A [robot plugin](../concepts/robot-plugins.md) carries one for its robot, so a recipe on a supported robot writes none of this. For any other robot, or to override the plugin's, the recipe builds it:

```python
import numpy as np
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotConfig,
    RobotGeometryType,
    RobotType,
)

my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometryType.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(max_omega=0.4, max_acc=2.0, max_decel=2.0, max_ang=np.pi / 3),
)
```

Every field is required on purpose. A robot without limits is not a robot EMOS will move, so `RobotConfig()` with nothing in it is an error rather than a default. The classes live in Sugarcoat and are re-exported by `kompass.robot` and `kompass.config`.

## Motion models

Pick the one that matches the drivetrain, and the control maths follows.

- {material-regular}`directions_car;1.2em;sd-text-primary` **Ackermann.** Car-like vehicles. Non-holonomic, with a limited steering angle, and unable to turn in place.
- {material-regular}`swap_horiz;1.2em;sd-text-primary` **Differential drive.** Two driven wheels, or a quadruped walking like one. Forward and backward motion and turning in place.
- {material-regular}`open_with;1.2em;sd-text-primary` **Omni.** Holonomic platforms: mecanum wheels, or a quadruped that side-steps. Motion in any direction and rotation at once.

## Geometry

The geometry is the robot's collision volume, which planning and obstacle avoidance keep clear. `geometry_params` is a NumPy array whose meaning depends on the shape:

```{list-table}
:widths: 15 25 60
:header-rows: 1

* - Type
  - Parameters
  - Shape

* - **BOX**
  - `[length, width, height]`
  - Axis-aligned box.

* - **CYLINDER**
  - `[radius, length_z]`
  - Vertical cylinder.

* - **SPHERE**
  - `[radius]`
  - Sphere.

* - **ELLIPSOID**
  - `[axis_x, axis_y, axis_z]`
  - Axis-aligned ellipsoid.

* - **CAPSULE**
  - `[radius, length_z]`
  - Cylinder with hemispherical ends.

* - **CONE**
  - `[radius, length_z]`
  - Vertical cone.
```

The type is a `RobotGeometryType`, and the motion model a `RobotType`; both also accept their names as strings, `"CYLINDER"` and `"DIFFERENTIAL_DRIVE"`, which is how a configuration file spells them.

## Control limits

The limits bound what any component may command. Linear limits apply to forward motion, and to sideways motion for an omni robot, and angular limits to rotation:

```python
ctrl_vx = LinearCtrlLimits(max_vel=1.0, max_acc=1.5, max_decel=2.5)
ctrl_vy = LinearCtrlLimits(max_vel=0.5, max_acc=0.7, max_decel=3.5)   # omni robots only
ctrl_omega = AngularCtrlLimits(max_omega=1.0, max_acc=2.0, max_decel=2.0, max_ang=np.pi / 3)

my_robot = RobotConfig(
    model_type=RobotType.OMNI,
    geometry_type=RobotGeometryType.BOX,
    geometry_params=np.array([0.6, 0.4, 0.3]),
    ctrl_vx_limits=ctrl_vx,
    ctrl_vy_limits=ctrl_vy,
    ctrl_omega_limits=ctrl_omega,
)
```

| Limit                     | Linear                | Angular                  |
| :------------------------ | :-------------------- | :----------------------- |
| Maximum velocity          | `max_vel` in m/s      | `max_omega` in rad/s     |
| Maximum acceleration      | `max_acc` in m/s²     | `max_acc` in rad/s²      |
| Maximum deceleration      | `max_decel` in m/s²   | `max_decel` in rad/s²    |
| Minimum velocity          | `min_vel`, 0.05 m/s   | `min_omega`, 0.01 rad/s  |
| Maximum steering angle    |                       | `max_ang` in rad, for Ackermann robots |

Acceleration and deceleration are separate so that a robot can accelerate gently and still brake hard. The minimum velocities are a dead band: commands below them are treated as a stop, which keeps a controller from creeping and lets the drive manager know when the robot has come to rest. `ctrl_vy_limits` is the one optional field, and defaults to no lateral motion.

## Coordinate frames

Only two frames are configured, the world frame that plans and maps are expressed in and the robot's base frame. Every other frame, a LiDAR's, a camera's, is read from the messages the sensor publishes and resolved through TF, so nothing about sensor placement is written here.

```python
from kompass.config import RobotFrames

frames = RobotFrames(world="map", robot_base="base_link")
```

Those are the defaults, so most recipes never set them. A robot plugin sets the base frame to the robot's own, and the launcher applies the frames to every component with `launcher.frames`, or one at a time with `launcher.robot_frame` and `launcher.world_frame`.

```{seealso}
- [Configuration](../advanced/configuration.md) for the same description in a YAML, TOML or JSON file.
- [Robot Plugins](../concepts/robot-plugins.md) for the description a plugin provides.
```
