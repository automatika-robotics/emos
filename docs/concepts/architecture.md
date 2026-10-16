# Architecture

EMOS, the Embodied Operating System, is the software layer that turns a robot into a physical AI agent. It gives quadrupeds, humanoids and mobile robots one runtime in which they see, think, move and adapt, and it does so without caring which robot it is on. The application, a recipe, is written once against standard interfaces, and a plugin adapts it to the hardware.

## The body and the mind

EMOS separates the robot's **body** from its **mind**. The body is the hardware: motors, sensors, actuators and the drivers that speak to them. The mind is the software that perceives, reasons and decides. Between the two sits a standard interface, and everything EMOS adds to a robot lives on the mind's side of it. That is what lets the same recipe run on a wheeled base, a quadruped or a humanoid.

## The stack

Three open-source packages make up the stack, each building on the one below it.

:::{image} ../_static/images/diagrams/emos_diagram_light.png
:align: center
:width: 500px
:class: light-only
:::

:::{image} ../_static/images/diagrams/emos_diagram_dark.png
:align: center
:width: 500px
:class: dark-only
:::

### Sugarcoat, the architecture layer

[Sugarcoat](https://github.com/automatika-robotics/sugarcoat) is the framework the other two are built on. It provides the primitives every recipe is made of:

- {material-regular}`autorenew;1.2em;sd-text-primary` [Components](components.md), lifecycle-managed nodes that report their health and heal themselves.
- {material-regular}`flash_on;1.2em;sd-text-primary` [Events and actions](events-and-actions.md) and [routines](routines.md), the reactive layer that switches behaviour on live data.
- {material-regular}`rocket_launch;1.2em;sd-text-primary` The [launcher](launcher.md) and its Monitor, which bring a recipe up in threads or processes and orchestrate it while it runs.
- {material-regular}`extension;1.2em;sd-text-primary` The [plugin framework](robot-plugins.md), through which a robot and its sensors are adapted to the standard interfaces.
- {material-regular}`web;1.2em;sd-text-primary` The [web interface](web-ui.md) a recipe can serve for itself.

### Kompass, the navigation layer

[Kompass](https://github.com/automatika-robotics/kompass) is the event-driven navigation stack: global and local mapping, planning, control and a motion server, built as Sugarcoat components. Its heavy geometry runs on the GPU where there is one, on any vendor's, and it drives wheeled, legged and tracked platforms alike. See [Navigation](../navigation/overview.md).

### EmbodiedAgents, the intelligence layer

[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) is the framework for agentic graphs of models: vision-language models, speech in and out, object detection in two and three dimensions, a spatio-temporal memory, semantic routing of commands, motion policies, and Cortex, the agent that plans with the recipe's actions and routines as its tools. Its components run models locally or through cloud APIs and can switch between them at run time. See [Intelligence](../intelligence/overview.md).

## Around the stack

Three more things ship with EMOS and sit around the stack rather than inside it.

**Plugins** adapt one robot or one sensor to the interfaces above. A robot plugin carries the robot's odometry, sensors and commands, its own actions and events, its localization and how it is mapped. A sensor plugin adds one device that is not part of the robot. They are installed from a catalog, and the [plugins](../getting-started/plugins.md) page covers using them.

**The CLI**, `emos`, installs the stack in one of three modes, keeps it updated, manages plugins, builds and manages [maps](../getting-started/mapping.md), and runs recipes with their logs kept. It is the operator's tool on the robot itself. See the [CLI reference](../getting-started/cli.md).

**The dashboard** is the robot's web page: recipes to pull, run and follow, plugins to install, maps, and the system's state, served securely to a paired browser on the same network. It is separate from the web interface a recipe serves for itself. See the [dashboard](../getting-started/dashboard.md).

## Recipes

A recipe is a Python script that declares a whole application: which components run, how they are wired, which plugin adapts the robot, which events matter and what happens when they fire. There are no launch files and no scattered configuration.

```python
from ros_sugar import Launcher
from ros_sugar.core import Event
from ros_sugar.io import Topic
from myrobot_plugin import MyRobotPlugin

# Components from any layer: perception from EmbodiedAgents,
# planning and control from Kompass, or your own on Sugarcoat

robot = MyRobotPlugin()
launcher = Launcher()
launcher.add_plugin(robot)
launcher.add_pkg(components=[planner, controller], package_name="kompass", multiprocessing=True)
launcher.add_pkg(components=[vision, agent], package_name="agents", multiprocessing=True)
launcher.on(robot.events.low_battery(20.0), robot.actions.dock())
launcher.bringup()
```

The same script runs from a terminal while it is being written and through `emos run` or the dashboard once it is done. [Running Recipes](../getting-started/running-recipes.md) covers both.
