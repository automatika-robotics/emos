# Extending EMOS

EMOS is designed to be extended. This guide covers how to create custom components, deploy them as system services, use built-in services for live reconfiguration, and write robot plugins for hardware portability.

## Creating Custom Components

:::{tip}
To see detailed examples of packages built with EMOS, check out [Kompass](https://automatika-robotics.github.io/kompass/) (navigation) and [EmbodiedAgents](https://automatika-robotics.github.io/embodied-agents/) (intelligence).
:::

:::{note}
Before building your own package, review the core [design concepts](../concepts/components.md).
:::

### Step 1 -- Create a ROS 2 Package

Start by creating a standard ROS 2 Python package:

```bash
ros2 pkg create --build-type ament_python --license Apache-2.0 my-awesome-pkg
```

### Step 2 -- Define Your Component

Create your first functional unit (component) in a new file:

```bash
cd my-awesome-pkg/my_awesome_pkg
touch awesome_component.py
```

### Step 3 -- Setup Component Configuration

Extend `BaseComponentConfig` based on the [attrs](https://www.attrs.org/en/stable/) package:

```python
from attrs import field, define
from ros_sugar.config import BaseComponentConfig, base_validators

@define(kw_only=True)
class AwesomeConfig(BaseComponentConfig):
    """
    Component configuration parameters
    """

    extra_float: float = field(
        default=10.0, validator=base_validators.in_range(min_value=1e-9, max_value=1e9)
    )

    extra_flag: bool = field(default=True)
```

### Step 4 -- Implement the Component

Initialize your component by inheriting from `BaseComponent`. Code the desired functionality in your component:

```python
from ros_sugar.core import ComponentFallbacks, BaseComponent
from ros_sugar.io import Topic

class AwesomeComponent(BaseComponent):
    def __init__(
        self,
        *,
        component_name: str,
        inputs: Optional[Sequence[Topic]] = None,
        outputs: Optional[Sequence[Topic]] = None,
        config_file: Optional[str] = None,
        config: Optional[AwesomeConfig] = None,
        **kwargs,
    ) -> None:
        # Set default config if config is not provided
        self.config: AwesomeConfig = config or AwesomeConfig()

        super().__init__(
            component_name=component_name,
            inputs=inputs,
            outputs=outputs,
            config=self.config,
            config_file=config_file,
            **kwargs,
        )

    def _execution_step(self):
        """
        The execution step is the main (timed) functional unit in the component.
        Gets called automatically at every loop step (with a frequency of
        'self.config.loop_rate').
        """
        super()._execution_step()
        # Add your main execution step here
```

Follow this pattern to create any number of functional units in your package.

### Step 5 -- Create an Entry Point (Multi-Process)

To use your components with the EMOS Launcher in multi-process execution, create an entry point:

```python
#!/usr/bin/env python3
from ros_sugar import executable_main
from my_awesome_pkg.awesome_component import AwesomeComponent, AwesomeConfig

# Create lists of available components/config classes
_components_list = [AwesomeComponent]
_configs_list = [AwesomeConfig]

# Create entry point main
def main(args=None):
    executable_main(list_of_components=_components_list, list_of_configs=_configs_list)
```

Add the entry point to the ROS 2 package `setup.py`:

```python
from setuptools import find_packages, setup

package_name = "my_awesome_pkg"

console_scripts = [
    "executable = my_awesome_pkg.executable:main",
]

setup(
    name=package_name,
    version="1",
    packages=find_packages(),
    install_requires=["setuptools"],
    zip_safe=True,
    entry_points={
        "console_scripts": console_scripts,
    },
)
```

Build your ROS 2 package with colcon, then use the Launcher to bring up your system.

### Step 6 -- Launch with EMOS

Use the EMOS Launcher to bring up your package:

```{code-block} python
:caption: Using the EMOS Launcher with your package
:linenos:

from my_awesome_pkg.awesome_component import AwesomeComponent, AwesomeConfig
from ros_sugar.actions import LogInfo
from ros_sugar.events import OnLess
from ros_sugar import Launcher
from ros_sugar.io import Topic

# Define a set of topics
map_topic = Topic(name="map", msg_type="OccupancyGrid")
audio_topic = Topic(name="voice", msg_type="Audio")
image_topic = Topic(name="camera/rgb", msg_type="Image")

# Init your components
my_component = AwesomeComponent(
    component_name='awesome_component',
    inputs=[map_topic, image_topic],
    outputs=[audio_topic]
)

# Create your events
low_battery = Event(battery_level_topic.msg.data < 15.0)

# Events/Actions
my_events_actions: Dict[event.Event, Action] = {
    low_battery: LogInfo(msg="Battery is Low!")
}

# Create your launcher
launcher = Launcher()

# Add your package components
launcher.add_pkg(
    components=[my_component],
    package_name='my_awesome_pkg',
    executable_entry_point='executable',
    events_actions=my_events_actions,
    activate_all_components_on_start=True,
    multiprocessing=True,
)

# If any component fails -> restart it with unlimited retries
launcher.on_component_fail(action_name="restart")

# Bring up the system
launcher.bringup()
```

---

## Deploying as systemd Services

EMOS recipes can be easily deployed as `systemd` services for production environments or embedded systems where automatic startup and restart behavior is critical.

Once you have a Python script for your EMOS-based package (e.g., `my_awesome_system.py`), install it as a systemd service:

```bash
ros2 run automatika_ros_sugar create_service <path-to-python-script> <service-name>
```

### Arguments

- `<path-to-python-script>`: The full path to your EMOS Python script (e.g., `/path/to/my_awesome_system.py`).
- `<service-name>`: The name of the systemd service (do **not** include the `.service` extension).

### Example

```bash
ros2 run automatika_ros_sugar create_service ~/ros2_ws/my_awesome_system.py my_awesome_service
```

This installs and optionally enables a `systemd` service named `my_awesome_service.service`.

### Full Command Usage

```text
usage: create_service [-h] [--service-description SERVICE_DESCRIPTION]
                      [--install-path INSTALL_PATH]
                      [--source-workspace-path SOURCE_WORKSPACE_PATH]
                      [--no-enable] [--restart-time RESTART_TIME]
                      service_file_path service_name
```

**Positional Arguments:**

- **`service_file_path`**: Path to the Python script to install as a service.
- **`service_name`**: Name of the systemd service (without `.service` extension).

**Optional Arguments:**

- `-h, --help`: Show the help message and exit.
- `--service-description SERVICE_DESCRIPTION`: Human-readable description of the service. Defaults to `"EMOS Service"`.
- `--install-path INSTALL_PATH`: Directory to install the systemd service file. Defaults to `/etc/systemd/system`.
- `--source-workspace-path SOURCE_WORKSPACE_PATH`: Path to the ROS workspace `setup` script. If omitted, it auto-detects the active ROS distribution.
- `--no-enable`: Skip enabling the service after installation.
- `--restart-time RESTART_TIME`: Time to wait before restarting the service if it fails (e.g., `3s`). Default is `3s`.

### What This Does

This command:

1. Creates a `.service` file for `systemd`.
2. Installs it in the specified or default location.
3. Sources the appropriate ROS environment.
4. Optionally enables and starts the service immediately.

Once installed, manage the service with standard `systemd` commands:

```bash
sudo systemctl start my_awesome_service
sudo systemctl status my_awesome_service
sudo systemctl stop my_awesome_service
sudo systemctl enable my_awesome_service
```

---

## Built-in Services for Live Reconfiguration

In addition to the standard [ROS 2 Lifecycle Node](https://github.com/ros2/demos/blob/rolling/lifecycle/README.rst) services, EMOS components provide a powerful set of built-in services for live reconfiguration. These services allow you to dynamically adjust inputs, outputs, and parameters on-the-fly, making it easier to respond to changing runtime conditions or trigger intelligent behavior in response to events. Like any ROS 2 services, they can be called from other Nodes or with the ROS 2 CLI, and can also be called programmatically as part of an action sequence or event-driven workflow in the launch script.

### Replacing an Input or Output with a Different Topic

You can swap an existing topic connection (input or output) with a different topic online without restarting your script. The service will stop the running lifecycle node, replace the connection, and restart it.

- **Service Name:** `/{component_name}/change_topic`
- **Service Type:** `automatika_ros_sugar/srv/ReplaceTopic`

**Example:**

```shell
ros2 service call /awesome_component/change_topic automatika_ros_sugar/srv/ReplaceTopic \
  "{direction: 1, old_name: '/voice', new_name: '/audio_device_0', new_msg_type: 'Audio'}"
```

### Updating a Configuration Parameter Value

The `ChangeParameter` service allows updating a single configuration parameter at runtime. You can choose whether the component remains active during the change, or temporarily deactivates for a safe update.

- **Service Name:** `/{component_name}/update_config_parameter`
- **Service Type:** `automatika_ros_sugar/srv/ChangeParameter`

**Example:**

```shell
ros2 service call /awesome_component/update_config_parameter automatika_ros_sugar/srv/ChangeParameter \
  "{name: 'loop_rate', value: '1', keep_alive: false}"
```

### Updating Multiple Configuration Parameters

The `ChangeParameters` service allows updating multiple parameters at once, ideal for switching modes or reconfiguring components in batches.

- **Service Name:** `/{component_name}/update_config_parameters`
- **Service Type:** `automatika_ros_sugar/srv/ChangeParameters`

**Example:**

```shell
ros2 service call /awesome_component/update_config_parameters automatika_ros_sugar/srv/ChangeParameters \
  "{names: ['loop_rate', 'fallback_rate'], values: ['1', '10'], keep_alive: false}"
```

### Reconfiguring from a File

The `ConfigureFromFile` service lets you reconfigure an entire component from a YAML, JSON, or TOML configuration file while the node is online. This is useful for applying scenario-specific settings or restoring saved configurations in a single operation.

- **Service Name:** `/{component_name}/configure_from_file`
- **Service Type:** `automatika_ros_sugar/srv/ConfigureFromFile`

**Example YAML configuration file:**

```yaml
/**:
  fallback_rate: 10.0

awesome_component:
  loop_rate: 100.0
```

### Executing a Component Method

The `ExecuteMethod` service enables runtime invocation of any class method in the component. This is useful for triggering specific behaviors, tools, or diagnostics during runtime without writing additional interfaces.

- **Service Name:** `/{component_name}/execute_method`
- **Service Type:** `automatika_ros_sugar/srv/ExecuteMethod`

---

## Universal Robot Plugins

EMOS introduces Robot Plugins to seamlessly bridge your automation recipes with diverse robot hardware.

By abstracting manufacturer-specific ROS 2 interfaces, plugins allow you to write generic, portable automation logic that runs on any robot without code changes.

### What Are Robot Plugins?

Different robot manufacturers often use custom messages or services in their ROS 2 interfaces to handle basic operations like sending robot actions or getting diverse low-level feedback such as odometry, battery info, etc. With traditional ROS 2 packages, you need code changes to handle each new message/service type. This creates a "lock-in" where your code becomes tightly coupled to a specific robot.

EMOS Robot Plugins act as a translation layer between your EMOS application and the robot's hardware with all its custom types.

### Why Robot Plugins?

- **Portability:** Write your automation recipe once using standard types. Switch robots by simply changing the plugin configuration.
- **Simplicity:** The plugin handles all the complex type conversions and service calls behind the scenes.
- **Modularity:** Keep hardware-specific logic isolated in a separate package.

### How to Create a Robot Plugin

To create your own custom plugin, you can create a ROS 2 package based on the example [myrobot_plugin_interface](https://github.com/automatika-robotics/robot-plugin-example).

Any plugin package must export a Python module containing two specific dictionaries in its `__init__.py`:

1. `robot_feedback`: Maps standard types to the robot's specific feedback topics (e.g., getting `IMU` or `Odometry` like information).
2. `robot_action`: Maps standard types to the robot's specific action topics or service clients (e.g., sending `Twist` like commands).

The main steps:

**0. (Optional) Define Custom ROS Interfaces**

If your robot's manufacturer-specific messages or services are not available to import from another package, define them in `msg/` and `srv/` folders.

**1. Implement Type Converters (`types.py`)**

Create a `types.py` module to handle data translation:

- **For Each Feedback:** Define a callback function that transforms the custom ROS 2 message into a standard Python type (like a NumPy array). Register it using `create_supported_type`.
- **For Each Action:** Define a converter function that transforms standard Python inputs into the custom ROS 2 message.

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

### How to Use a Plugin in Your Recipe

Using a robot plugin in your EMOS automation recipe is straightforward. After building and installing your plugin package, specify the plugin package name when initializing the `Launcher`:

```python
from ros_sugar import Launcher

# ... Define your components/events/actions/fallbacks here ...

# Initialize the launcher with your specific robot plugin
launcher = Launcher(robot_plugin="myrobot_plugin")

# ... Add it all to the launcher and bringup the system ...
```

### Example Robot Plugin

The [`myrobot_plugin`](https://github.com/automatika-robotics/robot-plugin-example) example bridges standard robot commands (`Twist`) and standard feedback (`Odometry`) to two custom interfaces.

**Custom Interfaces:**

- **`CustomOdom.msg`**: A feedback message containing position (x, y, z) and orientation (pitch, roll, yaw).
- **`CustomTwist.msg`**: A command message for 2D velocity (vx, vy) and angular velocity (vyaw).
- **`RobotActionCall.srv`**: A service definition used to trigger actions on the robot, returning a success boolean.

**Supported Types (`types.py`):**

- *Feedback (Callbacks):* Functions that convert incoming ROS messages into standard types. Example: `_odom_callback` converts `CustomOdom` into a NumPy array `[x, y, yaw]`.
- *Actions (Converters):* Functions that convert standard commands into custom ROS 2 messages. Example: `_ctr_converter` converts velocity inputs into a `CustomTwist` message.

**Service Clients (`clients.py`):**

For robots that handle actions via ROS services, define custom client wrappers inheriting from `RobotPluginServiceClient`. Example: `CustomTwistClient` wraps the `RobotActionCall` service.

**Plugin Entry Point (`__init__.py`):**

Exposes the plugin capabilities using two dictionaries: `robot_feedback` and `robot_action`.

```{raw} html
<div style="text-align: center;">
<iframe width="600" height="338" src="https://www.youtube.com/embed/oZN6pcJKgfY" frameborder="0" allowfullscreen></iframe>
</div>
```
