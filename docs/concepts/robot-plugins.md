# Robot Plugins

EMOS is built to be robot-agnostic. Recipes are written against standard, hardware-independent interfaces, so the same behaviour runs on any robot. In practice, though, robot manufacturers expose their hardware in their own ways: bespoke ROS 2 messages and services, or no ROS at all but UDP packets, an HTTP API or a vendor SDK.

{material-regular}`extension;1.2em;sd-text-primary` **Plugins exist to bridge that gap.** A plugin adapts one piece of hardware to the standard interface that recipes use, so a new robot comes online without touching a single recipe. It also brings the robot's own vocabulary along: its **actions**, a gait change on a quadruped, a docking routine, an arm stow, and its **events**, such as a low battery, ready for recipes and for [Cortex](../intelligence/cortex.md) to use by name.

This page explains how plugins work and what they can declare. For installing and using one, see [Plugins](../getting-started/plugins.md).

---

## What a plugin is

A plugin is a Python class that describes a robot or a sensor: where its data comes from, how its commands are sent, what it can do and what it can report. The class is declarative. Its constructor only describes the hardware and does no I/O, which lets Sugarcoat serialize the description and rebuild the plugin inside every component process. Sugarcoat then does the actual work of opening connections, decoding telemetry and routing it to the components that asked for it.

There are two kinds, and a recipe can carry one of the first and any number of the second:

| Kind             | Represents                                                                 | Declares in addition                                                              |
| :--------------- | :------------------------------------------------------------------------- | :-------------------------------------------------------------------------------- |
| `RobotPlugin`    | The robot itself: its odometry, cameras, LiDAR, velocity command, actions. | The robot's `RobotConfig`, its base frame, where its built-in sensors sit, and how it maps. |
| `SensorPlugin`   | One extra sensor that is not part of the robot.                            | An `id` recipes refer to it by, and a frame the sensor's data is in.               |

---

## How a plugin works

When a recipe starts, the launcher runs the plugin in two roles. The **host** lives in the launcher process, owns the real transports, decodes each piece of telemetry once, and publishes it on an internal feedback bus. A **client** lives in every component process; it opens no connections of its own but reads decoded feedback from the bus and sends commands back through the host.

During activation, every topic a component declares with `use_plugin` is matched against what the plugin provides. A feed that is already a ROS topic just gets the subscriber re-pointed; anything else is bridged through the bus, and large messages like images and point clouds travel through shared memory. Components never know that their data is not plain ROS. Topics without `use_plugin` are left alone, so a recipe can mix plugin feeds with ordinary ROS topics freely.

---

## Anatomy of a plugin

| Building block                       | Role                                                                                                                       |
| :----------------------------------- | :------------------------------------------------------------------------------------------------------------------------- |
| `RobotPlugin` / `SensorPlugin`       | The plugin itself: subclass one of these.                                                                                  |
| `Transport`                          | Where data comes from or goes to: `UdpTransport`, `HttpTransport`, `SdkCallbackTransport`, `RosTopicTransport`, `RosServiceTransport`. |
| `Feedback`                           | One telemetry stream: a standard message type, a transport, and a decoder from the raw payload to the message.             |
| `RobotCommand`                       | One command surface: a standard message type, a transport, and an encoder from the component's output to the wire.         |
| `ActionRegistry` / `EventRegistry`   | Named factories that produce the robot's `Action` and `Event` objects.                                                     |
| `ProcessSpec`                        | A driver the plugin starts for a feed, such as a LiDAR or camera driver node.                                              |
| `Mount`                              | Where a sensor sits, published as a static transform so nothing else has to.                                               |
| `VendorMapping` / `NativeMapping`    | How the robot is mapped, read by `emos map`.                                                                               |
| `create_supported_type`              | Wraps a robot's custom ROS message as a standard type that components understand.                                          |

Feeds and commands are keyed by name. A recipe asks for a feed either by its standard type, when the plugin has just one of that type, or by its key when there are several: a quadruped with two ultrasound sensors has feeds `ultrasound_front` and `ultrasound_back`, both of type `Range`.

---

## Using a plugin in a recipe

Hand the robot plugin to the launcher and every component in the recipe talks to the robot through it. Topics that should come from the plugin say so with `use_plugin`:

```python
from ros_sugar import Launcher
from ros_sugar.io import Topic
from myrobot_plugin import MyRobotPlugin

robot = MyRobotPlugin()
launcher = Launcher(robot_plugin=robot)

odom = Topic(name="Odometry", msg_type="Odometry", use_plugin=True)   # by type
front = Topic(name="ultrasound_front", msg_type="Range", use_plugin=True)  # by key

launcher.add_pkg(components=[planner, controller], package_name="kompass", multiprocessing=True)
launcher.on(robot.events.low_battery(20.0), robot.actions.dock())
launcher.bringup()
```

The plugin's actions and events come from `robot.actions.<name>(...)` and `robot.events.<name>(...)`, and they wire into the launcher, or into any component's fallbacks, like actions and events of your own. Because the launcher binds the plugin's identity when it is attached, build events and actions after the `Launcher` has the plugin, as above.

A robot plugin also carries the robot's `RobotConfig` and base frame, and the launcher hands them to every component. A recipe that sets `launcher.robot` itself still wins, so a plugin's configuration is a default, not a lock.

### Sensor plugins and mounts

A sensor plugin is attached with `add_plugin`, and the recipe says where the sensor sits with a `Mount`. The mount is published as a static transform, so the sensor's data has a frame in TF without any URDF. The parent can be the robot plugin, another sensor plugin, or a fixed frame by name:

```python
from ros_sugar.robot import Mount
from hikmicro_plugin import HikmicroBispectrum

camera = HikmicroBispectrum(id="inspection_cam")
launcher.add_plugin(camera, mount=Mount(parent=robot, xyz=(0.15, 0.0, 0.35), rpy=(0.0, 0.0, 1.57)))

image = Topic(name="visible_image", msg_type="Image", use_plugin=camera.id)
```

Every plugin has an `id`, derived from its name unless you pass one, and a sensor's frame defaults to `<id>_frame`. Topics refer to a sensor plugin by that id, which is what makes two cameras of the same kind in one recipe unambiguous. Robot plugins list their own built-in sensors as mounts in the same way, so a LiDAR on the robot's back has a frame before any driver starts.

### Drivers the plugin starts

A plugin can declare the driver nodes its feeds need, as a list of `ProcessSpec`s returned by `required_processes()`. The launcher starts them alongside the recipe, restarts them if they die, and stops them at the end. A well-written plugin looks at which feeds the recipe actually requested and starts only those drivers, which is why a recipe that never touches the camera never spins up the camera driver.

The same mechanism is how a plugin provides localization. A robot without its own localizer can run a sensor-fusion node over its odometry and IMU, started only when a recipe binds the fused feed, and publish the transforms navigation needs. The plugin declares the package that node comes from as one of its dependencies, so nothing about localization is part of the EMOS install itself.

### Mapping

A robot plugin declares how the robot is mapped, and `emos map` reads that declaration. `VendorMapping` describes a robot with its own mapping software, as the commands that start and stop a session, apply a map and export or import one, plus where that software keeps its maps. `NativeMapping` describes a robot that EMOS maps itself: which feed carries the LiDAR cloud, which one the IMU, and the height band and resolution of the occupancy grid. Either kind exposes `active_grid_path()`, which is how a recipe finds the map in use. [Mapping](../getting-started/mapping.md) covers the workflow.

---

## Installing a plugin

You do not have to write a plugin to use one. The catalog ships ready-made plugins that install with one command, or one click on the dashboard:

```bash
emos plugin install emos-plugin-example
```

See [Plugins](../getting-started/plugins.md) for installing, updating and removing them.

---

## Writing your own

The reference plugin, [`emos-plugin-example`](https://github.com/automatika-robotics/emos-plugin-example), implements one robot across every transport family, UDP telemetry and velocity commands, a ROS-topic battery feed and a ROS-service docking action, with a mock robot and a test suite. Copy it and adapt it. The full authoring guide, from wrapping custom message types to declaring mounts and mapping, is in the Sugarcoat documentation: [Creating a Plugin](https://automatika-robotics.github.io/sugarcoat/development/custom_robot_plugin.html).

Two things are specific to EMOS.

**The entry point.** The catalog lists each plugin with an entry point of the form `module:ClassName`, for example `myrobot_plugin:MyRobotPlugin`. `emos plugin install` imports the class through it, and `emos plugin inspect` shows what the class describes. You can run the same introspection yourself while developing:

```bash
python -m ros_sugar.robot inspect myrobot_plugin:MyRobotPlugin
```

**The dependency manifest.** Drivers a plugin needs are declared in a file named `emos-plugin.yaml` at the root of the plugin's repository. Every key is optional, and a plugin with no driver dependencies needs no manifest at all:

```yaml
sources:                                  # repositories built from source, next to the plugin
  - git: https://github.com/RoboSense-LiDAR/rslidar_sdk
    ref: v1.5.20                          # tag or branch
    recursive: true                       # clone its submodules too
deps:
  ros: [realsense2_camera]                # ROS packages, installed as ros-<distro>-<name>
  system:
    conda: [libpcap, ffmpeg]              # for pixi installs
    apt: [libpcap0.8-dev, ffmpeg]         # for native installs
```

`sources` are for drivers that have no binary package: they are cloned into the plugin workspace and built together with the plugin, in every install mode. `deps` are installed as binaries, from RoboStack and conda-forge on a pixi install and with rosdep and apt on a native install. Native installs resolve dependencies from the plugin's `package.xml`, so declare the ROS packages there as well. Container installs build the plugin but only print the dependencies, since packages cannot be added to the image from a running robot.

```{seealso}
- [Plugins](../getting-started/plugins.md) for installing and using plugins.
- [Creating a Plugin](https://automatika-robotics.github.io/sugarcoat/development/custom_robot_plugin.html) for the full authoring guide.
- [Extending EMOS](../advanced/extending.md) for custom components and deploying them as services.
```
