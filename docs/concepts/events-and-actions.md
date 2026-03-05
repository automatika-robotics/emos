# Events & Actions

**Dynamic behavior switching based on real-time environmental context.**

EMOS's Event-Driven architecture enables dynamic behavior switching based on real-time environmental context. This allows robots to react instantly to changes in their internal state or external environment without complex, brittle if/else chains.

## Events

An Event in EMOS monitors a specific **ROS2 Topic**, and defines a triggering condition based on the incoming topic data. You can write natural Python expressions (e.g., `topic.msg.data > 5`) to define exactly when an event should trigger the associated Action(s).

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`hub;1.5em;sd-text-primary` Compose Logic - </span> Combine triggers using simple Pythonic syntax (`(lidar_clear) & (goal_seen)`).

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`sync;1.5em;sd-text-primary` Fuse Data - </span> Monitor multiple topics simultaneously via a synchronized **Blackboard** that ensures data freshness.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`speed;1.5em;sd-text-primary` Stay Fast - </span> All evaluation happens asynchronously in a dedicated worker pool. Your main component loop **never blocks**.


:::{admonition} Think in Behaviors
:class: tip
Events are designed to be read like a sentence:
*"If the battery is low AND we are far from home, THEN navigate to the charging dock."*
:::

:::{tip} Events can be paired with EMOS [`Action`](#actions)(s) or with any standard [ROS2 Launch Action](https://docs.ros.org/en/kilted/Tutorials/Intermediate/Launch/Using-Event-Handlers.html)
:::

### Defining Events

The Event API uses a fluent, expressive syntax that allows you to access ROS2 message attributes directly via `topic.msg`.

#### Basic Single-Topic Event

```python
from ros_sugar.core import Event
from ros_sugar.io import Topic

# 1. Define the Source
# `data_timeout` parameter is optional. It ensures data is considered "stale" after 0.5s
battery = Topic(name="/battery_level", msg_type="Float32", data_timeout=0.5)

# 2. Define the Event
# Triggers when percentage drops below 20%
low_batt_event = Event(battery.msg.data < 20.0)
```

#### Composed Conditions (Logic & Multi-Topic)

You can combine multiple conditions using standard Python bitwise operators (`&`, `|`, `~`) to create complex behavioral triggers. Events can also span multiple different topics. EMOS automatically manages a "Blackboard" of the latest messages from all involved topics, ensuring synchronization and data "freshness".

- **Example**: Trigger a "Stop" event only if an obstacle is detected AND the robot is currently in "Auto" mode.

```python
from ros_sugar.core import Event
from ros_sugar.io import Topic

lidar_topic = Topic(name="/person_detected", msg_type="Bool", data_timeout=0.5)
status_topic = Topic(name="/robot_mode", msg_type="String", data_timeout=60.0)

# Complex Multi-Topic Condition
emergency_stop_event = Event((lidar_topic.msg.data.is_true()) & (status_topic.msg.data == "AUTO"))
```

:::{admonition} Handling Stale Data
:class: warning
When combining multiple topics, data synchronization is critical. Use the `data_timeout` parameter on your `Topic` definition to ensure you never act on old sensor data.
:::

### Event Configuration

Refine *when* and *how* the event triggers using these parameters:

* <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`change_circle` On Change (`on_change=True`)</span> - Triggers **only** when the condition transitions from `False` to `True` (Edge Trigger). Useful for state transitions (e.g., "Goal Reached") rather than continuous firing.
* <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`all_inclusive` On Any (`Topic`)</span> - If you pass the `Topic` object itself as the condition, the event triggers on **every received message**, regardless of content.
* <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`looks_one` Handle Once (`handle_once=True`)</span> - The event will fire exactly one time during the lifecycle of the system. Useful for initialization sequences.
* <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`timer` Event Delay (`keep_event_delay=2.0`)</span> - Prevents rapid firing (debouncing). Ignores subsequent triggers for the specified duration (in seconds).


### Supported Conditional Operators

You can use standard Python operators or specific helper methods on any topic attribute to define the event triggering condition.

| Operator / Method | Description | Example |
| :--- | :--- | :--- |
| **`==`**, **`!=`** | Equality checks. | `topic.msg.status == "IDLE"` |
| **`>`**, **`>=`**, **`<`**, **`<=`** | Numeric comparisons. | `topic.msg.temperature > 75.0` |
| **`.is_true()`** | Boolean True check. | `topic.msg.is_ready.is_true()` |
| **`.is_false()`**, **`~`** | Boolean False check. | `topic.msg.is_ready.is_false()` or `~topic.msg.is_ready` |
| **`.is_in(list)`** | Value exists in a list. | `topic.msg.mode.is_in(["AUTO", "TELEOP"])` |
| **`.not_in(list)`** | Value is not in a list. | `topic.msg.id.not_in([0, 1])` |
| **`.contains(val)`** | String/List contains a value. | `topic.msg.description.contains("error")` |
| **`.contains_any(list)`** | List contains *at least one* of the values. | `topic.msg.error_codes.contains_any([404, 500])` |
| **`.contains_all(list)`** | List contains *all* of the values. | `topic.msg.detections.labels.contains_all(["window", "desk"])` |
| **`.not_contains_any(list)`** | List contains *none* of the values. | `topic.msg.active_ids.not_contains_any([99, 100])` |


### Event Usage Examples

#### Automatic Adaptation (Terrain Switching)

Scenario: A perception or ML node publishes a string to `/terrain_type`. We want to change the robot's gait when the terrain changes.

```{code-block} python
:caption: quadruped_controller.py
:linenos:

from typing import Literal
from ros_sugar.component import BaseComponent

class QuadrupedController(BaseComponent):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        # Some logic

    def switch_gait_controller(self, controller_type: Literal['stairs', 'sand', 'snow', 'gravel']):
        self.get_logger().info("New terrain detected! Switching gait.")
        # Logic to change controller parameters...
```

```{code-block} python
:caption: quadruped_controller_recipe.py
:linenos:

from my_pkg.components import QuadrupedController
from ros_sugar.core import Event, Action
from ros_sugar.io import Topic
from ros_sugar import Launcher

quad_controller = QuadrupedController(component_name="quadruped_controller")

# Define the Event Topic
terrain_topic = Topic(name="/terrain_type", msg_type="String")

# Define the Event
# Logic: Trigger when the detected terrain changes
# on_change=True ensures we only trigger the switch the FIRST time stairs are seen.
# Add an optional delay to prevent rapid event triggering
event_terrain_changed = Event(terrain_topic, on_change=True, keep_event_delay=60.0)

# Define the Action
# Call self.switch_gait_controller() when triggered and pass the detected terrain to the method
change_gait_action = Action(method=self.activate_stairs_controller, args=(terrain_topic.msg.data))

# Register
my_launcher = Launcher()
my_launcher.add_pkg(
            components=[quad_controller],
            events_actions={stairs_event: change_gait_action},
        )
```


#### Autonomous Drone Safety

Scenario: An autonomous drone **stops** if an obstacle is close OR the bumper is hit. It also sends a warning if the battery is low AND we are far from the land.

```python
from ros_sugar.core import Event, Action
from ros_sugar.io import Topic

# --- Topics ---
proximity_sensor   = Topic(name="/radar_front", msg_type="Float32", data_timeout=0.2)
bumper  = Topic(name="/bumper", msg_type="Bool", data_timeout=0.1)
battery = Topic(name="/battery", msg_type="Float32")
location = Topic(name="/pose", msg_type="Pose")

# --- Conditions ---
# 1. Safety Condition (Composite OR)
# Stop if proximity_sensor < 0.2m OR Bumper is Hit
is_danger = (proximity_sensor.msg.data < 0.2) | (bumper.msg.data.is_true())

# 2. Return Home Condition (Composite AND)
# Return if Battery < 20% AND Distance > 100m
needs_return = (battery.msg.data < 20.0) & (location.position.z > 100.0)

# --- Events ---
safety_event = Event(is_danger)

return_event = Event(needs_return, on_change=True)
```

---

## Actions

**Executable context-aware behaviors for your robotic system.**

Actions are not just static function calls; they are **dynamic, context-aware routines** that can adapt their parameters in real-time based on live system data.

They can represent:

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`smart_toy;1.2em;sd-text-primary` Component Behaviors — </span> Routines defined within your components. *e.g., Stopping the robot, executing a motion pattern, or saying a sentence.*

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`settings;1.2em;sd-text-primary` System Behaviors — </span> Lifecycle management, configuration and plumbing. *e.g., Reconfiguring a node, restarting a driver, or re-routing input streams.*

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`extension;1.2em;sd-text-primary` User Custom Behaviors — </span> Arbitrary Python functions. *e.g., Calling an external REST API, logging to a file, or sending a slack notification.*


### Trigger Mechanisms

Actions sit dormant until activated by one of two mechanisms:

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`flash_on;1.2em;sd-text-primary` Event-Driven (Reflexive) - </span> Triggered instantly when a specific **Event** condition is met.
    **Example:** "Obstacle Detected" $\rightarrow$ `stop_robot()`

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`healing;1.2em;sd-text-primary` Fallback-Driven (Restorative) - </span> Triggered automatically by a Component when its internal **Health Status** degrades.
    **Example:** "Camera Driver Failed" $\rightarrow$ `restart_driver()`


### The `Action` Class

At its core, the `Action` class is a wrapper around any Python callable. It packages a function along with its arguments, preparing them for execution at runtime.

But unlike standard Python functions, EMOS Actions possess a superpower: [Dynamic Data Injection](#dynamic-data-injection). You can bind their arguments directly to live ROS2 Topics, allowing the Action to fetch the latest topic message or a specific message argument the moment it triggers.

```python
class Action:
    def __init__(self, method: Callable, args: tuple = (), kwargs: Optional[Dict] = None):
```

- `method`: The function or routine to execute.
- `args`: Positional arguments (can be static values OR dynamic Topic values).
- `kwargs`: Keyword arguments (can be static values OR dynamic Topic values).

### Basic Usage

```python
from ros_sugar.component import BaseComponent
from ros_sugar.core import Action
import logging

def custom_routine():
    logging.info("I am executing an action!")

my_component = BaseComponent(node_name='test_component')

# 1. Component Method
action1 = Action(method=my_component.start)

# 2. Method with keyword arguments
action2 = Action(method=my_component.update_parameter, kwargs={"param_name": "fallback_rate", "new_value": 1000})

# 3. External Function
action3 = Action(method=custom_routine)
```

### Dynamic Data Injection

**This is EMOS's superpower.**

You can create complex, context-aware behaviors without writing any "glue code" or custom parsers.

When you bind an Action argument to a `Topic`, the system automatically resolves the binding at runtime, fetching the current value from the topic attributes and injecting it into your function.

#### Example: Cross-Topic Data Access

**Scenario**: An event occurs on Topic 1. You want to log a message that includes the current status from Topic 2 and a sensor reading from Topic 3.

```python
from ros_sugar.core import Event, Action
from ros_sugar.io import Topic

# 1. Define Topics
topic_1 = Topic(name="system_alarm", msg_type="Bool")
topic_2 = Topic(name="robot_mode", msg_type="String")
topic_3 = Topic(name="battery_voltage", msg_type="Float32")

# 2. Define the Event
# Trigger when Topic 1 becomes True
event_on_first_topic = Event(topic_1.msg.data.is_true())

# 3. Define the Target Function
def log_context_message(mode, voltage):
    print(f"System Alarm! Current Mode: {mode}, Voltage: {voltage}V")

# 4. Define the Dynamic Action
# We bind the function arguments directly to the data fields of Topic 2 and Topic 3
my_action = Action(
    method=log_context_message,
    # At runtime, these are replaced by the actual values from the topics
    args=(topic_2.msg.data, topic_3.msg.data)
)
```

### Pre-defined Actions

EMOS provides a suite of pre-defined, thread-safe actions for managing components and system resources via the `ros_sugar.actions` module.

:::{admonition} Import Note
:class: tip
All pre-defined actions are **keyword-only** arguments. They can be imported directly:
`from ros_sugar.actions import start, stop, reconfigure`
:::

#### Component-Level Actions

These actions directly manipulate the state or configuration of a specific `BaseComponent` derived object.

| Action Method                           | Arguments                                                     | Description                                                                                                                                 |
| :-------------------------------------- | :------------------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------ |
| **`start`**                             | `component`                                                   | Triggers the component's Lifecycle transition to **Active**.                                                                                |
| **`stop`**                              | `component`                                                   | Triggers the component's Lifecycle transition to **Inactive**.                                                                              |
| **`restart`**                           | `component`<br>`wait_time` (opt)                              | Stops the component, waits `wait_time` seconds (default 0), and Starts it again.                                                            |
| **`reconfigure`**                       | `component`<br>`new_config`<br>`keep_alive`                   | Reloads the component with a new configuration object or file path. <br>`keep_alive=True` (default) keeps the node running during update.   |
| **`update_parameter`**                  | `component`<br>`param_name`<br>`new_value`<br>`keep_alive`    | Updates a **single** configuration parameter.                                                                                               |
| **`update_parameters`**                 | `component`<br>`params_names`<br>`new_values`<br>`keep_alive` | Updates **multiple** configuration parameters simultaneously.                                                                               |
| **`send_component_service_request`**    | `component`<br>`srv_request_msg`                              | Sends a request to the component's main service with a specific message.                                                                    |
| **`trigger_component_service`**         | `component`                                                   | Triggers the component's main service. <br>Creates the request message dynamically during runtime from the incoming Event topic data.       |
| **`send_component_action_server_goal`** | `component`<br>`request_msg`                                  | Sends a goal to the component's main action server with a specific message.                                                                 |
| **`trigger_component_action_server`**   | `component`                                                   | Triggers the component's main action server. <br>Creates the request message dynamically during runtime from the incoming Event topic data. |

#### System-Level Actions

These actions interact with the broader ROS2 system and are executed by the central `Monitor`.

| Action Method               | Arguments                                       | Description                                                              |
| :-------------------------- | :---------------------------------------------- | :----------------------------------------------------------------------- |
| **`log`**                   | `msg`<br>`logger_name` (opt)                    | Logs a message to the ROS console.                                       |
| **`publish_message`**       | `topic`<br>`msg`<br>`publish_rate`/`period`     | Publishes a specific message to a topic. Can be single-shot or periodic. |
| **`send_srv_request`**      | `srv_name`<br>`srv_type`<br>`srv_request_msg`   | Sends a request to a ROS 2 Service with a specific message.              |
| **`trigger_service`**       | `srv_name`<br>`srv_type`                        | Triggers the a given ROS2 service.                                       |
| **`send_action_goal`**      | `server_name`<br>`server_type`<br>`request_msg` | Sends a specific goal to a ROS 2 Action Server.                          |
| **`trigger_action_server`** | `server_name`<br>`server_type`                  | Triggers a given ROS2 action server.                                     |


:::{admonition} Automatic Data Conversion
:class: note
When using **`trigger_*`** actions paired with an Event, EMOS attempts to create the required service/action request from the incoming Event topic data automatically via **duck typing**.

If automatic conversion is not possible, or if the action is not paired with an Event, it sends a default (empty) request.
:::
