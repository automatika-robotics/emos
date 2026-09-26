# Plugins

EMOS recipes are written against standard interfaces, so the same recipe runs on any robot. Real robots, though, speak their own languages: vendor-specific ROS messages, or no ROS at all but UDP packets, an HTTP API or an SDK. A **plugin** is what bridges that gap. It is an EMOS package that knows one piece of hardware and presents it to EMOS the standard way, so your recipes never have to know the difference.

There are two kinds:

- A **robot plugin** represents the robot itself: its odometry, its cameras and LiDAR, its velocity command, and whatever actions and events the robot offers, like standing up or reporting a low battery. It also carries the robot's geometry and speed limits, so navigation components are configured for the right robot without you writing that configuration.
- A **sensor plugin** represents one extra sensor that is not part of the robot, an inspection camera bolted onto it, say, or a fixed camera watching a room. A robot runs one robot plugin and as many sensor plugins as you like.

Plugins do one more thing for you: they bring their own drivers. When a recipe asks for the robot's LiDAR, the plugin starts the LiDAR driver; when no recipe needs it, nothing runs. That is why there is no separate driver setup in EMOS.

This page is about installing and using plugins. [Robot Plugins](../concepts/robot-plugins.md) explains how they work inside and how to write one.

## Browse the catalog

```bash
emos plugin list
```

```text
PLUGIN                 NAME                         VENDOR         ROLE
emos-plugin-example    Example Robot                Automatika     robot    ● installed
emos-plugin-lite3      DeepRobotics Lite3           Deep Robotics  robot
emos-plugin-m20        PUMA M20                     Deep Robotics  robot
emos-plugin-hikvision  HIKMICRO bispectrum camera   HIKMICRO       sensor
```

The same catalog is on the dashboard's **Plugins** page.

## Install a plugin

```bash
emos plugin install emos-plugin-example
```

The CLI fetches the plugin, builds it for your install mode, and records it. Installing a robot plugin when another one is installed replaces it, after asking you; installing a sensor plugin adds it alongside whatever is there. Expect a few minutes, as the plugin and any driver packages it builds from source are compiled.

When it is done, the CLI prints the one line a recipe needs to use the plugin:

```text
✓ Plugin 'Example Robot' installed.
  In a recipe: from myrobot_plugin import MyRobotPlugin; Launcher(robot_plugin=MyRobotPlugin())
```

Plugins live in a workspace of their own at `~/emos/workspace`, which `emos run` sources for every recipe. Only one plugin operation runs at a time, whether started from the terminal or the dashboard, and a recipe will not start while one is in progress.

### Dependencies

A plugin declares the driver packages it needs, and the CLI installs them in the way that fits your install mode. In pixi mode the packages are added to the EMOS environment from RoboStack and conda-forge. In native mode they are installed with rosdep and apt. Drivers that exist only as source are cloned next to the plugin and built with it, in every mode.

```{note}
Container mode is the exception. The CLI builds the plugin, but it cannot install driver packages into the image, so it prints what the image would need instead. A plugin whose drivers are not in the image still builds and works for everything except those drivers.
```

## See what a plugin provides

```bash
emos plugin inspect                    # the robot plugin
emos plugin inspect emos-plugin-hikvision
```

This prints the plugin's interface: the feeds it publishes, the commands it accepts, and its actions and events, by name. The dashboard's **System** page shows the same information for the robot and each sensor.

To see what a particular recipe needs from the plugin, use `emos info <recipe>`. It lists each topic the recipe uses and marks the ones that come from the robot plugin or from a named sensor plugin, and it tells you when a plugin the recipe expects is not installed.

## Keep plugins up to date

There is no separate update command. `emos update` pulls and rebuilds every installed plugin after it has updated EMOS itself. On the dashboard, **Reinstall** does the same for one plugin.

## Remove a plugin

```bash
emos plugin remove emos-plugin-hikvision   # one plugin
emos plugin remove                         # every plugin, after a confirmation
```

Removing the robot plugin leaves the sensor plugins installed.

## Use a plugin in a recipe

### The robot

Hand the robot plugin to the launcher, and every component in the recipe talks to the robot through it:

```python
from ros_sugar import Launcher
from myrobot_plugin import MyRobotPlugin

robot = MyRobotPlugin()
launcher = Launcher(robot_plugin=robot)
launcher.add_pkg(components=[planner, controller], package_name="kompass", multiprocessing=True)
launcher.on(robot.events.low_battery(20.0), robot.actions.dock())
launcher.bringup()
```

A topic that should come from the plugin says so with `use_plugin=True`, naming the feed the plugin provides. Where the plugin has just one feed of a type, the type is enough; where it has several, use the feed's name:

```python
odom = Topic(name="Odometry", msg_type="Odometry", use_plugin=True)
```

The robot's actions and events are available as `robot.actions.<name>(...)` and `robot.events.<name>(...)`, with the names `emos plugin inspect` shows, and they wire into the launcher like any other action or event. The plugin also supplies the robot's geometry, speed limits and base frame, so you do not set a `RobotConfig` yourself.

### A sensor

A sensor plugin is attached with `add_plugin`, together with a `Mount` that says where the sensor sits, relative to the robot or to a fixed frame. Because a recipe can attach several sensors, each gets an id, and topics refer to the sensor by that id:

```python
from ros_sugar import Launcher
from ros_sugar.robot import Mount
from hikmicro_plugin import HikmicroBispectrum

camera = HikmicroBispectrum(id="inspection_cam")
launcher = Launcher(robot_plugin=robot)
launcher.add_plugin(camera, mount=Mount(parent=robot, xyz=(0.15, 0.0, 0.35)))

image = Topic(name="visible_image", msg_type="Image", use_plugin=camera.id)
```

With no robot at all, mount the sensor on a fixed frame instead: `Mount(parent="world", xyz=(0.0, 0.0, 1.0))`.

```{tip}
Give each sensor an id that says what it is for, `inspection_cam` rather than `cam1`. The id is how recipes, `emos info` and the dashboard refer to that sensor, and it becomes the name of the sensor's frame.
```

## From the dashboard

Everything above is available from the dashboard's [Plugins page](dashboard.md#plugins): the catalog, **Install**, **Add**, **Replace robot**, **Reinstall** and **Remove**, with progress shown on the card while a plugin builds. The [System page](dashboard.md#system) then shows the robot and its sensors as the plugins describe them.

## Writing your own

If your robot is not in the catalog, you can write a plugin for it. The step-by-step guide is in the Sugarcoat documentation, [Creating a Plugin](https://automatika-robotics.github.io/sugarcoat/development/custom_robot_plugin.html), and [`emos-plugin-example`](https://github.com/automatika-robotics/emos-plugin-example) is a complete plugin to copy from. What EMOS adds on top, the entry point and the dependency manifest that `emos plugin install` reads, is described in [Robot Plugins](../concepts/robot-plugins.md).

```{seealso}
- [Robot Plugins](../concepts/robot-plugins.md) for how plugins work and what they can declare.
- [Mapping](mapping.md) for building maps, which a robot plugin also declares.
- [EMOS CLI](cli.md#emos-plugin) for the command reference.
```
