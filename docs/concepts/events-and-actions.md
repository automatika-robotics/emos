# Events & Actions

A recipe reacts to the world through events and actions. An event is a condition on live data: a battery level under a threshold, a person in the camera image, a robot that has fallen. An action is what happens when the condition holds: a component method, a lifecycle change, a message on a topic, a whole [routine](routines.md). You declare both in the recipe, pair them, and the Monitor that every launcher starts does the watching and the dispatching. Nothing in a component's main loop blocks for it.

---

## Events

### Conditions on topics

A condition is written against a topic's message with ordinary Python. `topic.msg` gives access to every field of the message type, nested as deep as the type goes, and comparing or testing a field produces the condition:

```python
from ros_sugar.core import Event
from ros_sugar.io import Topic

battery = Topic(name="/battery_level", msg_type="Float32")
odom = Topic(name="/odom", msg_type="Odometry")

low_battery = Event(battery.msg.data < 20.0)
far_out = Event(odom.msg.pose.pose.position.x > 5.0)
```

Field paths are checked against the message type when the condition is built, so a typo fails at import rather than at run time. The condition is evaluated each time a message arrives on the topic.

| Operator or method                           | Meaning                                       | Example                                                   |
| :------------------------------------------- | :-------------------------------------------- | :-------------------------------------------------------- |
| `==`, `!=`                                   | Equality.                                     | `topic.msg.status == "IDLE"`                              |
| `>`, `>=`, `<`, `<=`                         | Numeric comparison.                           | `topic.msg.temperature > 75.0`                            |
| `.is_true()`, `.is_false()`                  | Boolean test.                                 | `topic.msg.is_ready.is_true()`                            |
| `.is_in(list)`, `.not_in(list)`              | Membership of the value in a list.            | `topic.msg.mode.is_in(["AUTO", "TELEOP"])`                |
| `.contains(value)`, `.not_contains(value)`   | A string or list holds the value.             | `topic.msg.description.contains("error")`                 |
| `.contains_any(list)`, `.contains_all(list)` | A list holds at least one, or all, of the values. | `topic.msg.labels.contains_all(["window", "desk"])`   |
| `.not_contains_any(list)`, `.not_contains_all(list)` | The negations of the two above.       | `topic.msg.active_ids.not_contains_any([99, 100])`        |

Conditions combine with `&`, `|` and `~`, across topics if you like:

```python
person = Topic(name="/person_detected", msg_type="Bool", data_timeout=0.5)
mode = Topic(name="/robot_mode", msg_type="String", data_timeout=60.0)

emergency_stop = Event(person.msg.data.is_true() & (mode.msg.data == "AUTO"))
```

A compound event keeps the latest message of every topic it involves and evaluates the whole expression whenever any of them updates. `data_timeout` on a topic says how long a message stays valid for that purpose, so a condition never fires on a reading that stopped arriving a minute ago.

Passing the topic itself, `Event(topic)`, makes an event that fires on every message whatever it contains, which is the usual way to trigger an action that takes its arguments from the message.

### Conditions on plugin feeds

The robot's own telemetry is a source like any other. A topic declared with `use_plugin` builds conditions the same way, and a plugin also offers ready-made events with the robot's vocabulary, so the recipe does not need to know how the feed is encoded:

```python
launcher.on(robot.events.low_battery(0.2), robot.actions.sit())
```

Feeds that never touch a ROS topic still work, since the plugin host feeds each decoded message to the Monitor directly. Build plugin events after the plugin is attached with `add_plugin`. [Robot Plugins](robot-plugins.md) covers what a plugin offers.

### Conditions on internal state

Sometimes the trigger is not on a topic at all: a hardware monitor, a derived value that needs real calculation, a state you would rather keep inside the recipe. An event takes any function that returns `bool`, and polls it at `check_rate`:

```python
def is_overheating() -> bool:           # the annotation is required
    return read_temperature() > 75.0

overheat = Event(is_overheating, check_rate=2.0)
launcher.on(overheat, log(msg="Overheating, backing off"))
```

Two rules apply. The function's return annotation has to be `bool`, and it cannot be a component action, which has a different contract. The polling runs on the Monitor, and `check_rate` in hertz defaults to the Monitor's own loop rate. [Internal-State Events](../recipes/events-and-resilience/internal-state-events.md) is a worked recipe.

### When an event fires

| Parameter                 | Effect                                                                                                                                                 |
| :------------------------ | :----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `on_change=True`          | Fire only when the condition turns from false to true, not on every message while it stays true. The right choice for "goal reached" or "entered zone". |
| `handle_once=True`        | Fire exactly once for the life of the recipe. Useful for initialisation.                                                                                |
| `keep_event_delay=2.0`    | After firing, ignore further triggers for this many seconds. A debounce for noisy signals.                                                             |
| `check_rate=10.0`         | For a function condition, how often it is polled, in hertz.                                                                                            |

### An example

A drone stops if something is close ahead or its bumper is hit, and heads home when the battery is low while it is far up:

```python
from ros_sugar.core import Event
from ros_sugar.io import Topic

proximity = Topic(name="/radar_front", msg_type="Float32", data_timeout=0.2)
bumper = Topic(name="/bumper", msg_type="Bool", data_timeout=0.1)
battery = Topic(name="/battery", msg_type="Float32")
location = Topic(name="/pose", msg_type="Pose")

danger = (proximity.msg.data < 0.2) | bumper.msg.data.is_true()
needs_return = (battery.msg.data < 20.0) & (location.msg.position.z > 100.0)

safety_event = Event(danger)
return_event = Event(needs_return, on_change=True)
```

---

## Actions

An action wraps something callable together with its arguments, ready to run when an event fires or a fallback needs it. Where it runs depends on what it wraps:

| Action                                                          | Runs in                                                                                     |
| :-------------------------------------------------------------- | :------------------------------------------------------------------------------------------ |
| A component method marked with `@component_action`              | The component, whichever process it lives in.                                               |
| A system action from `ros_sugar.actions`                        | The Monitor.                                                                                |
| A plain function defined in the recipe                          | The launcher process.                                                                       |
| A plugin action, from `robot.actions`                           | The plugin, which sends the command to the robot.                                           |
| A standard ROS launch action, such as `TimerAction`             | ROS launch itself.                                                                          |

### The `Action` class

```python
from ros_sugar.core import Action

stop = Action(navigator.stop)
set_rate = Action(navigator.update_parameter, kwargs={"param_name": "fallback_rate", "new_value": 1000})
notify = Action(send_slack_message, args=("Battery low",))
```

`Action(method, args=None, kwargs=None)` is the whole of it for a fire-and-forget action. The further parameters, `success`, `timeout`, `on_timeout`, `max_retries`, `retry_delay` and `cancel_method`, make the action monitored, described [below](#monitored-actions). `name` and `description` label it for routines and for tools, and `on_fail` and `fallback` are read only when the action is a step of a routine.

### The return contract

Every action returns a pair, `(success, message)`. The boolean says whether it worked, and the string carries a result on success or the reason on failure. Component actions declare this with `ActionReturnType`, and the decorator checks the annotation at import:

```python
from ros_sugar.core import BaseComponent
from ros_sugar.utils import ActionReturnType, component_action

class Gripper(BaseComponent):
    @component_action
    def close(self) -> ActionReturnType:
        if self._blocked:
            return False, "gripper is obstructed"
        return True, "gripper closed"
```

A raised exception is reported as a failure carrying its message, and a return that does not follow the contract, `None` included, is logged and treated as a failure. That is deliberate: everything that consumes an action reads its outcome, and a malformed value would otherwise pass as success. An action with something structured to report puts it in the string, as JSON for instance. `@component_action(active=True)` restricts the method to the component's active state, and `@component_action(description={...})` attaches a tool description that Cortex reads; without one, the docstring serves.

### Arguments from topics

An action's arguments can be taken from live data instead of fixed values. Pass a `topic.msg.<field>` expression, the same kind used in conditions, and the value is read from the latest message when the action runs:

::::{tab-set}

:::{tab-item} Positional

```python
sensor = Topic(name="/sensor", msg_type="Float32")

def handle_reading(value: float) -> ActionReturnType:
    return True, f"reading {value}"

launcher.on(Event(sensor), Action(handle_reading, args=(sensor.msg.data,)))
```

:::

:::{tab-item} Keyword

```python
odom = Topic(name="/odom", msg_type="Odometry")

launcher.on(Event(odom), Action(navigate, kwargs={"x": odom.msg.pose.pose.position.x, "y": odom.msg.pose.pose.position.y}))
```

:::

:::{tab-item} Mixed

```python
# "WARNING" is fixed, the value comes from the topic at run time
Action(log_alert, args=("WARNING", sensor.msg.data))
```

:::

::::

The topics an action reads from do not have to be the ones its event watches. An alarm on one topic can log the mode from a second and the voltage from a third.

### Monitored actions

By default an action is fire-and-forget: it dispatches the method and nothing afterwards knows whether the method achieved anything. Giving an action any of `success`, `timeout`, `max_retries`, `retry_delay` or `cancel_method` turns monitoring on. The action then dispatches, waits for a verdict, and dispatches again while the verdict is negative and the retry budget allows:

```python
grasp = Action(
    gripper.close,
    success=gripper_state.msg.closed.is_true(),
    timeout=3.0,
    max_retries=3,
    cancel_method=gripper.stop,
)
```

The verdict comes from one of two places. With a `success` condition, live data is authoritative: the action succeeded when the condition becomes true, even if the method reported otherwise. Without one, the method's own `(success, message)` return decides. Success is detected on message arrival, not by polling, and only from data that arrived after the dispatch, so an arm that already happened to be at its target does not report an instant success.

| Parameter        | Effect                                                                                                    |
| :--------------- | :-------------------------------------------------------------------------------------------------------- |
| `timeout`        | Seconds to wait for the verdict on each attempt.                                                          |
| `on_timeout`     | What a timeout means: `"retry"` (default), `"fail"` or `"succeed"`.                                       |
| `max_retries`    | Number of re-dispatches, so the total number of attempts is one more.                                     |
| `retry_delay`    | Seconds between attempts.                                                                                 |
| `cancel_method`  | Called when the action is halted, so the thing it started can be told to stop.                           |

There is one retry budget, spent by a reported failure and by a timeout alike. `halt()` stops the waiting and retrying of an action in flight and calls its `cancel_method`; it cannot interrupt a method that is already executing, which is why any action used where it may be preempted should have one. Monitoring runs wherever the action runs: in the component for a component action, on the Monitor for a system action or a recipe function.

```{warning}
A `success` condition without a `timeout` waits forever if the condition never comes true. Set a timeout on every monitored action, and the recipe logs a warning when you forget.
```

A step that sends a goal to a component's action server has a form of its own, `ActionServerGoal`, which takes the server's outcome as its verdict and can be cancelled for real, on the server, when halted. A component's main action server accepts one goal at a time, and `cancel_main_goal` is the component action that clears it.

### Registering actions

An event and its actions are paired with `launcher.on`, or with the `events_actions` mapping of `add_pkg`. The action side can be one action or a list, run in order, and a routine is registered the same way:

```python
launcher.on(low_battery, [log(msg="Battery low"), robot.actions.dock()])
launcher.on(pick_requested, pick_routine)
launcher.add_pkg(components=[navigator], events_actions={obstacle: Action(navigator.stop)})
```

### System actions

`ros_sugar.actions` provides ready-made actions for managing components and the ROS graph. All of them take keyword arguments only.

The component-level ones act on one component:

| Action                                | Arguments                                              | What it does                                                                                                       |
| :------------------------------------ | :----------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------- |
| `start`                               | `component`                                            | Transitions the component to active.                                                                               |
| `stop`                                | `component`                                            | Transitions it to inactive.                                                                                        |
| `restart`                             | `component`, `wait_time`                               | Stops it, waits, and starts it again.                                                                              |
| `reconfigure`                         | `component`, `new_config`, `keep_alive`                | Reloads the component with a new configuration object or file. `keep_alive=True` keeps it running meanwhile.      |
| `update_parameter`                    | `component`, `param_name`, `new_value`, `keep_alive`   | Changes one configuration parameter.                                                                               |
| `update_parameters`                   | `component`, `params_names`, `new_values`, `keep_alive`| Changes several at once.                                                                                           |
| `send_component_service_request`      | `component`, `srv_request_msg`                         | Calls the component's main service with the given request.                                                         |
| `trigger_component_service`           | `component`                                            | Calls the component's main service, building the request from the event's message.                               |
| `send_component_action_server_goal`   | `component`, `request_msg`                             | Sends the given goal to the component's main action server.                                                        |
| `trigger_component_action_server`     | `component`                                            | Sends a goal to the main action server, built from the event's message.                                           |

The system-level ones act on the graph, and on routines:

| Action                                                                 | Arguments                                              | What it does                                                                        |
| :--------------------------------------------------------------------- | :----------------------------------------------------- | :---------------------------------------------------------------------------------- |
| `log`                                                                  | `msg`, `logger_name`                                   | Writes a line to the ROS log.                                                       |
| `publish_message`                                                      | `topic`, `msg`, `publish_rate`, `publish_period`       | Publishes a message once, or at a rate for a period.                               |
| `send_srv_request`                                                     | `srv_name`, `srv_type`, `srv_request_msg`              | Calls any ROS service with the given request.                                       |
| `trigger_service`                                                      | `srv_name`, `srv_type`                                 | Calls a service with a request built from the event's message.                     |
| `send_action_goal`                                                     | `server_name`, `server_type`, `request_msg`            | Sends a goal to any ROS action server.                                              |
| `trigger_action_server`                                                | `server_name`, `server_type`                           | Sends a goal built from the event's message.                                        |
| `wait`                                                                 | `duration`, `name`                                     | Dwells for the given seconds, as a step of a routine.                               |
| `start_routine`, `pause_routine`, `resume_routine`, `abort_routine`    | `routine_name`, and `reason` for abort                 | Controls a [routine](routines.md) by name.                                          |

The `trigger_*` actions build the request or goal from the event's message by matching field names. When no match is possible, or the action is not paired with an event, an empty request is sent.

---

## Fallbacks

Events are one of two things that trigger actions. The other is a component's own health: when a component reports a failure, it runs the fallback actions it was given, and a component whose process dies is restarted by the launcher. Both are covered in [Status and Fallbacks](status-and-fallbacks.md).

---

## An example

A perception node publishes the terrain it sees, and a quadruped changes gait when it changes:

```{code-block} python
:caption: quadruped_controller.py

from ros_sugar.core import BaseComponent
from ros_sugar.utils import ActionReturnType, component_action

class QuadrupedController(BaseComponent):
    @component_action
    def switch_gait(self, terrain: str) -> ActionReturnType:
        # Change the controller's parameters for the terrain
        return True, f"gait set for {terrain}"
```

```{code-block} python
:caption: recipe.py

from my_pkg.components import QuadrupedController
from ros_sugar import Launcher
from ros_sugar.core import Action, Event
from ros_sugar.io import Topic

controller = QuadrupedController(component_name="quadruped_controller")
terrain = Topic(name="/terrain_type", msg_type="String")

# Every message on the topic, at most once a minute
terrain_seen = Event(terrain, keep_event_delay=60.0)
change_gait = Action(controller.switch_gait, args=(terrain.msg.data,))

launcher = Launcher()
launcher.add_pkg(components=[controller], package_name="my_pkg")
launcher.on(terrain_seen, change_gait)
launcher.bringup()
```

```{seealso}
- [Routines](routines.md) for sequences of monitored actions.
- [Status and Fallbacks](status-and-fallbacks.md) for actions that run on failure.
- [Components](components.md) for writing component actions.
- The [events and resilience recipes](../recipes/events-and-resilience/index.md) for worked examples.
```
