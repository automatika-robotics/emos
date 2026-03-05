# Robot Plugins

EMOS introduces Robot Plugins to **seamlessly bridge your automation recipes with diverse robot hardware**.

By abstracting manufacturer-specific ROS 2 interfaces, {material-regular}`extension;1.2em;sd-text-primary` **plugins allow you to write generic, portable automation logic that runs on any robot without code changes.**

---

## What Are Robot Plugins?

Different robot manufacturers often use custom messages or services in their ROS 2 interfaces to handle basic operations like sending robot actions or getting diverse low-level feedback such as odometry, battery info, etc. With traditional ROS 2 packages, you need code changes to handle each new message/service type. This creates a "lock-in" where your code becomes tightly coupled to a specific robot.

EMOS Robot Plugins act as a **translation layer**. They sit between your EMOS application and the robot's hardware with all its custom types.

---

## Why Robot Plugins?

- {material-regular}`swap_horiz;1.2em;sd-text-primary` **Portability** -- Write your automation recipe once using standard types. Switch robots by simply changing the plugin configuration.

- {material-regular}`auto_fix_high;1.2em;sd-text-primary` **Simplicity** -- The plugin handles all the complex type conversions and service calls behind the scenes.

- {material-regular}`widgets;1.2em;sd-text-primary` **Modularity** -- Keep hardware-specific logic isolated in a separate package.

---

## How to Create a Robot Plugin

To create your own custom plugin, you can create a ROS 2 package based on the example [myrobot_plugin_interface](https://github.com/automatika-robotics/robot-plugin-example).

Any plugin package must export a Python module containing two specific dictionaries in its `__init__.py`:

1. {material-regular}`sensors;1.2em;sd-text-success` `robot_feedback` -- Maps standard types to the robot's specific feedback topics (e.g., getting `IMU` or `Odometry` like information).
2. {material-regular}`send;1.2em;sd-text-warning` `robot_action` -- Maps standard types to the robot's specific action topics or service clients (e.g., sending `Twist` like commands).

### The Steps

**0. (Optional) Define Custom ROS Interfaces**

If your robot's manufacturer-specific messages or services are not available to import from another package, define them in `msg/` and `srv/` folders.

**1. Implement Type Converters (`types.py`)**

Create a `types.py` module to handle data translation:

- {material-regular}`download;1.2em;sd-text-success` **For Each Feedback:** Define a callback function that transforms the custom ROS 2 message into a standard Python type (like a NumPy array). Register it using `create_supported_type`.
- {material-regular}`upload;1.2em;sd-text-warning` **For Each Action:** Define a converter function that transforms standard Python inputs into the custom ROS 2 message.

```python
from ros_sugar.robot_plugin import create_supported_type

# Example: Creating a supported type for feedback
RobotOdometry = create_supported_type(CustomOdom, callback=_odom_callback)
```

**2. Handle Service Clients (`clients.py`)**

If your robot actions require calling a ROS 2 service, create a class inheriting from `RobotPluginServiceClient` in `clients.py`. Implement the `_publish` method to construct and send the service request.

**3. Register the Plugin (`__init__.py`)**

Expose your new capabilities in `__init__.py` by defining two dictionaries:

```python
from . import types, clients

robot_feedback = {
    "Odometry": Topic(name="myrobot_odom", msg_type=types.RobotOdometry),
}

robot_action = {
    "Twist": clients.CustomTwistClient
}
```

**4. Configure the Build**

Use the same `CMakeLists.txt` and `package.xml` for your new plugin package. Make sure to add any additional dependencies.

---

## How to Use a Plugin in Your Recipe

Using a robot plugin in your EMOS automation recipe is straightforward. After building and installing your plugin package, specify the plugin package name when initializing the `Launcher`:

```python
from ros_sugar import Launcher

# ... Define your components/events/actions/fallbacks here ...

# Initialize the launcher with your specific robot plugin
launcher = Launcher(robot_plugin="myrobot_plugin")

# ... Add it all to the launcher and bringup the system ...
```

That's it. EMOS handles the rest -- every topic subscription and publication in your recipe is automatically translated through the plugin.

---

## Example: A Complete Robot Plugin

The [`myrobot_plugin`](https://github.com/automatika-robotics/robot-plugin-example) example bridges standard robot commands (`Twist`) and standard feedback (`Odometry`) to two custom interfaces.

### Custom Interfaces

- {material-regular}`description;1.2em;sd-text-secondary` **`CustomOdom.msg`** -- A feedback message containing position (x, y, z) and orientation (pitch, roll, yaw).
- {material-regular}`description;1.2em;sd-text-secondary` **`CustomTwist.msg`** -- A command message for 2D velocity (vx, vy) and angular velocity (vyaw).
- {material-regular}`description;1.2em;sd-text-secondary` **`RobotActionCall.srv`** -- A service definition used to trigger actions on the robot, returning a success boolean.

### Supported Types (`types.py`)

- {material-regular}`download;1.2em;sd-text-success` **Feedback (Callbacks):** Functions that convert incoming ROS messages into standard types. Example: `_odom_callback` converts `CustomOdom` into a NumPy array `[x, y, yaw]`.
- {material-regular}`upload;1.2em;sd-text-warning` **Actions (Converters):** Functions that convert standard commands into custom ROS 2 messages. Example: `_ctr_converter` converts velocity inputs into a `CustomTwist` message.

### Service Clients (`clients.py`)

For robots that handle actions via ROS services, define custom client wrappers inheriting from `RobotPluginServiceClient`. Example: `CustomTwistClient` wraps the `RobotActionCall` service.

### Plugin Entry Point (`__init__.py`)

Exposes the plugin capabilities using two dictionaries: `robot_feedback` and `robot_action`.

### Testing

A `server_node.py` is provided to simulate the robot's ROS 2 server. It spins a minimal node that listens to `robot_control_service` requests and logs the received velocity commands, allowing you to test the `CustomTwistClient` functionality.

---

## See it in Action

Here the example plugin is tested with [**Kompass**](https://github.com/automatika-robotics/kompass), the EMOS navigation engine built on top of Sugarcoat.

Start by running the [`turtlebot3_test`](https://github.com/automatika-robotics/kompass/blob/main/kompass/recipes/turtlebot3.py) **without** the plugin and observe the subscribed and published topics. You will see the components subscribed to `/odometry/filtered` of type `Odometry`, and the `DriveManager` publishing `Twist` on `/cmd_vel`.

To enable the plugin, just edit one line:

```python
launcher = Launcher(robot_plugin="myrobot_plugin")
```

Re-run the recipe and the components now expect the plugin odometry topic of type `CustomOdometry`. The `DriveManager` no longer publishes `/cmd_vel` -- instead, it has created a service client in accordance with the custom plugin.

```{raw} html
<div style="text-align: center;">
<iframe width="600" height="338" src="https://www.youtube.com/embed/oZN6pcJKgfY" frameborder="0" allowfullscreen></iframe>
</div>
```

```{seealso}
To learn about creating custom EMOS components and deploying them as system services, see [Extending EMOS](../advanced/extending.md).
```
