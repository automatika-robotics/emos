# Robot Plugins

EMOS is built to be **robot-agnostic** — recipes run against standard, hardware-independent interfaces. But robot manufacturers often expose their hardware through **custom interfaces** (bespoke ROS messages and services, or non-ROS protocols like UDP, HTTP, or a vendor SDK). A **robot plugin** bridges that gap: it adapts one robot's custom interface to EMOS's standard one, so connecting a new robot is trivial and your recipes never change. A plugin can also bundle the robot's **preconfigured actions** — a gait change on a quadruped, a docking routine, an arm stow — ready to call by name.

This page covers **installing and managing** a plugin on a robot. For what plugins are and how to write one, see [Robot Plugins (concept)](../concepts/robot-plugins.md).

```{important}
A robot runs **one plugin at a time** — it represents *this* robot. Installing a new plugin replaces the active one.
```

## Browse the catalog

Plugins are published in the Automatika catalog. List what's available:

```bash
emos plugin list
```

```text
PLUGIN                NAME                          VENDOR
emos-plugin-example   Example Robot (reference …)   Automatika        ● active
emos-plugin-lite3     DeepRobotics Lite3            Deep Robotics
```

The active plugin (if any) is marked. You can browse the same catalog from the dashboard's **Plugins** page.

## Install a plugin

```bash
emos plugin install emos-plugin-example
```

EMOS fetches the plugin, builds it for your install mode, and activates it. If a different plugin is already active, you'll be asked to confirm the replacement.

When it finishes, the CLI prints how to reference the plugin from a recipe — for example:

```text
✓ Plugin 'Example Robot (reference plugin)' installed and activated.
  In a recipe: from myrobot_plugin import MyRobotPlugin; Launcher(robot_plugin=MyRobotPlugin())
```

```{note}
EMOS must be installed first (`emos install`). The plugin is built against your active install mode (Native / Pixi / Container).
```

## Inspect the active plugin

See exactly what the robot exposes — its feedback streams, commands, actions, and events:

```bash
emos plugin inspect
```

This prints the plugin's introspection tree (the same data the dashboard's **System** page shows for the active robot).

## Remove the active plugin

```bash
emos plugin remove
```

Removes the plugin from the robot. You can install another from the catalog at any time.

## Using a plugin in a recipe

Installing a plugin makes it importable; a recipe opts in by handing an instance to the `Launcher`:

```python
from ros_sugar.launch import Launcher
from myrobot_plugin import MyRobotPlugin

launcher = Launcher(robot_plugin=MyRobotPlugin())
launcher.add_pkg(components=[...])
launcher.bringup()
```

From there, every component's standard I/O is translated through the plugin, and the plugin's actions and events are available to your recipe (and to [Cortex](../intelligence/cortex.md), which registers them as tools automatically). See [Robot Plugins (concept)](../concepts/robot-plugins.md) for the details.

## From the dashboard

Everything above is also available without a terminal — see the dashboard's [Plugins page](dashboard.md#plugins): browse the catalog, install with one click, and watch the active robot (name, vendor, and picture) appear on the [System page](dashboard.md#system).

```{seealso}
- [Robot Plugins (concept)](../concepts/robot-plugins.md) — what plugins are and how to write one.
- [Dashboard](dashboard.md#plugins) — installing and viewing plugins from the browser.
- [EMOS CLI](cli.md#emos-plugin) — the full `emos plugin` command reference.
```
