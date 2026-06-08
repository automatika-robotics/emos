# Robot Plugins

EMOS is built to be **robot-agnostic** -- recipes are written against standard, hardware-independent interfaces, so the same behaviour runs on any robot. In practice, though, robot manufacturers often expose their hardware through **custom interfaces**: bespoke ROS 2 messages and services, or entirely non-ROS protocols like UDP, HTTP, or a vendor SDK.

{material-regular}`extension;1.2em;sd-text-primary` **Robot Plugins exist to bridge that gap.** A plugin adapts a manufacturer's custom interface to EMOS's standard one, making it trivial to bring a new robot online without touching your recipes. A plugin can also ship the robot's **preconfigured actions** -- a gait change on a quadruped, a docking routine, an arm stow -- ready for recipes (and [Cortex](../intelligence/cortex.md)) to call by name.

---

## What Are Robot Plugins?

A Robot Plugin is the **translation layer** between an EMOS recipe and a specific robot. It maps the robot's real interfaces onto the standard component I/O that recipes use (`Twist`, `Odometry`, `Imu`, …), so your recipe code never changes when the hardware does -- and it surfaces the robot's high-level **actions** (`stand_up`, `dock`, gait change) and **events** (`low_battery`) for recipes and Cortex to consume directly.

```{note}
This page describes the current plugin framework (Sugarcoat 0.7+): a plugin is a Python **class** you pass to the `Launcher`. It supersedes the earlier dictionary-based `robot_feedback` / `robot_action` API.
```

---

## Why Robot Plugins?

- {material-regular}`swap_horiz;1.2em;sd-text-primary` **Portability** -- Write the recipe once against standard types; switch robots by swapping a single plugin object.

- {material-regular}`auto_fix_high;1.2em;sd-text-primary` **Simplicity** -- The plugin hides all the type conversions, sockets, and service calls behind the scenes.

- {material-regular}`widgets;1.2em;sd-text-primary` **Modularity** -- Keep hardware-specific logic isolated in its own package.

---

## How a Plugin Works

A plugin is a subclass of `ros_sugar.robot.RobotPlugin`. Its `__init__` is **declarative** -- it only *describes* the robot (endpoints, transports, decoders) and does no I/O, so it can be serialized and rebuilt inside component subprocesses. The `Launcher` then runs it in one of two roles automatically:

- {material-regular}`dns;1.2em;sd-text-primary` **HOST** -- lives in the launcher process. Owns the real transports (binds sockets, opens sessions, runs heartbeats), decodes telemetry once, and publishes it on a feedback bus.
- {material-regular}`memory;1.2em;sd-text-primary` **CLIENT** -- lives in each component subprocess. Opens no sockets; it consumes decoded feedback from the bus and sends commands.

During activation, every component's standard input/output topics are matched against the plugin: a ROS-topic match re-points the native subscriber/publisher, and any other transport is bridged through the feedback bus. Components stay unaware their data isn't plain ROS.

---

## Anatomy of a Plugin

| Building block | Role |
| :------------- | :--- |
| `RobotPlugin` | The plugin itself -- subclass this. |
| `Transport` | Where data comes from / goes to: `UdpTransport`, `HttpTransport`, `SdkCallbackTransport`, `RosTopicTransport`, `RosServiceTransport`. |
| `Feedback` | One telemetry stream -- a standard type, a transport, and a decoder (raw payload → ROS message). |
| `RobotCommand` | One command surface -- a standard type, a transport, and an encoder (component output → wire payload). |
| `ActionRegistry` / `EventRegistry` | Named factories that produce the robot's `Action` / `Event` objects. |
| `create_supported_type` | Wraps a robot's custom ROS message as a standard `SupportedType`. |

`Feedback` and `RobotCommand` are keyed by the **standard message-type name** they stand in for (`Twist`, `Odometry`, …) -- that is how the framework matches them to component I/O.

---

## Using a Plugin in a Recipe

Hand a plugin instance to the `Launcher` -- that is the only recipe change:

```python
from ros_sugar.launch import Launcher
from myrobot_plugin import MyRobotPlugin

plugin = MyRobotPlugin()                       # declarative, zero-arg
launcher = Launcher(robot_plugin=plugin)
launcher.add_pkg(components=[planner, controller], multiprocessing=True)

# Plugin-provided events and actions wire up like anything else
launcher.on(plugin.events.low_battery(0.15), plugin.actions.dock())

launcher.bringup()
```

EMOS hosts the plugin, propagates it to every component, and translates all topic I/O through it -- every subscription and publication in your recipe is routed via the plugin automatically.

---

## Installing a Plugin

You don't have to build a plugin to use one -- the Automatika catalog ships ready-made plugins you install with one command (or one click in the dashboard):

```bash
emos plugin install emos-plugin-example
```

See [Robot Plugins (install & manage)](../getting-started/plugins.md) for the full flow.

---

## Writing Your Own

The reference plugin, [`emos-plugin-example`](https://github.com/automatika-robotics/emos-plugin-example), implements one robot across **all** the transport families (UDP telemetry + velocity command, a ROS-topic battery feedback, and a ROS-service docking action), with a mock robot and an end-to-end test suite. Copy it and adapt.

The full step-by-step authoring guide -- wrapping custom message types, defining transports/feedbacks/commands, contributing actions and events, and introspection -- lives in the Sugarcoat docs: [**Creating a Robot Plugin**](https://github.com/automatika-robotics/sugarcoat/blob/main/docs/development/custom_robot_plugin.md).

You can introspect any plugin's exposed surface from the command line:

```bash
python -m ros_sugar.robot inspect myrobot_plugin:MyRobotPlugin
```

(`emos plugin inspect` prints the same tree for the active plugin.)

---

```{seealso}
- [Robot Plugins (install & manage)](../getting-started/plugins.md) -- install a plugin from the catalog with `emos plugin`.
- [Creating a Robot Plugin](https://github.com/automatika-robotics/sugarcoat/blob/main/docs/development/custom_robot_plugin.md) -- the full authoring guide.
- [Extending EMOS](../advanced/extending.md) -- custom components and deploying them as system services.
```
