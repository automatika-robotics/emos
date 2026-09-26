# Status & Fallbacks

Robots fail, and a robot that keeps working is one that notices and recovers. Every EMOS component carries a health status that says not just whether it is alive but how it is doing, and a set of fallbacks that run on their own when that status turns bad. The recipe decides what recovery looks like, from retrying a call to restarting the component, and the component carries it out without anyone watching.

---

## Health status

A plain ROS node is either running or gone. An EMOS component distinguishes three ways of failing, because each calls for a different response:

- {material-regular}`check_circle;1.3em;sd-text-success` **Healthy.** The main loop ran and produced a valid result.
- {material-regular}`warning;1.3em;sd-text-warning` **Algorithm failure.** The component ran but could not solve its problem: the planner found no path, the detector found nothing, the solver did not converge. Nothing is broken, and a retry or a different setting may do.
- {material-regular}`error;1.3em;sd-text-danger` **Component failure.** Something inside the component is broken: an exception in a callback, a driver that disconnected. A restart is the usual answer.
- {material-regular}`link_off;1.3em;sd-text-primary` **System failure.** The component is fine but what it depends on is not: an input topic is silent or stale, the network is down. Waiting or restarting the data source is the answer, not restarting the component.

### Reporting it

The component sets its own status from inside its execution step or its callbacks, through `self.health_status`:

```python
self.health_status.set_healthy()
self.health_status.set_fail_algorithm(algorithm_names=["a_star"])
self.health_status.set_fail_component(component_names=["camera_driver"])
self.health_status.set_fail_system(topic_names=["/camera/rgb", "/odom"])
```

Mark the component healthy at the end of every successful step. That is what resets the fallback machinery after a recovery. The names are optional and only make the log more useful.

The status is broadcast on the component's status topic at each execution step, so the rest of the system sees a heartbeat even when the algorithm inside is slow. To push it out at once from a callback or another thread, call `self.broadcast_status()`.

### A pattern for the execution step

```python
def _execution_step(self):
    try:
        if self.input_image is None:
            # The input is missing: a system-level problem
            self.health_status.set_fail_system(topic_names=["/camera/rgb"])
            return

        result = self.model.detect(self.input_image)
        if result is None:
            # The code ran and found nothing: an algorithm-level problem
            self.health_status.set_fail_algorithm(algorithm_names=["detector"])
            return

        self.publish_result(result)
        self.health_status.set_healthy()

    except ConnectionError:
        # The hardware is gone: a component-level problem
        self.health_status.set_fail_component(component_names=["camera"])
```

---

## Fallbacks

A fallback is the set of [actions](events-and-actions.md#actions) a component runs when its status reports a failure. Rather than freezing or crashing, the component tries to recover: switch to a simpler algorithm, reconnect a driver, restart itself.

```{figure} /_static/images/diagrams/fallbacks_dark.png
:class: dark-only
:alt: fig-fallbacks
:align: center
```

```{figure} /_static/images/diagrams/fallbacks_light.png
:class: light-only
:alt: fig-fallbacks
:align: center

The self-healing loop
```

### Which fallback runs

A component has one fallback per failure level, a catch-all, and a last resort:

| Hook                 | Runs when                                                                                   |
| :------------------- | :------------------------------------------------------------------------------------------ |
| `on_system_fail`     | The status reports a system failure.                                                        |
| `on_component_fail`  | The status reports a component failure.                                                     |
| `on_algorithm_fail`  | The status reports an algorithm failure.                                                    |
| `on_fail`            | Any failure for which no level-specific fallback was set.                                   |
| `on_giveup`          | Every action of the fallback that applied has been tried and the failure is still there.   |

The level-specific fallback wins when it exists. Otherwise the catch-all runs. A failure that has neither is reported in the log and left in the broadcast status, and nothing is retried.

### What a fallback is made of

A fallback is one action or a list of them, with a retry count:

```python
from ros_sugar.actions import restart

# Restart the driver, up to three times
driver.on_component_fail(action=restart(component=driver), max_retries=3)
```

A single action is run again while it reports failure, until it succeeds or the retries are spent. A list is an escalation ladder: the first action gets its retries, then the next, then the next, each more drastic than the one before:

```python
planner.on_algorithm_fail(
    action=[
        Action(planner.clear_costmaps),       # cheap and fast
        Action(planner.switch_to_fallback),   # medium
        restart(component=planner),           # last
    ],
    max_retries=1,                            # each once before escalating
)
```

An action that returns `(True, message)` ends the recovery and resets the status to healthy. One that reports failure leaves the status as it is, so the ladder moves on. When the whole list is exhausted the component gives up and runs `on_giveup`, which is where you park the robot safely or alert a human. `max_retries=None`, the default, retries without limit.

Fallbacks run inside the component, not in the Monitor. A timer at the component's `fallback_rate`, a hundred times a second unless configured otherwise, looks at its status and walks the table above, which is why a component whose process is still alive can recover even when nothing else in the recipe is.

```{note}
A fallback has to be a plain action. A monitored action, one with `success`, `timeout`, `max_retries` or `cancel_method`, is refused where the fallback is declared, since a failed component has nothing left to watch a success condition with. A recovery that needs those belongs in a [routine](routines.md), whose steps can be monitored and carry fallbacks of their own.
```

### Setting fallbacks in the recipe

Fallbacks are usually attached from the recipe, which keeps the component reusable and the recovery policy in one place:

```python
from ros_sugar.actions import log, restart

lidar = LidarDriver(component_name="lidar_driver")

lidar.on_component_fail(action=restart(component=lidar))
lidar.on_system_fail(action=log(msg="Waiting for LiDAR data"))
lidar.on_giveup(action=log(msg="LiDAR is down, stopping"))
```

### Setting fallbacks in the component

Recovery that is tied to the component's own internals, such as a serial handshake, lives in the class. The method is marked with `@component_fallback`, which only lets it run while the component is in a state that can handle it, and it follows the same return contract as any action:

```python
from ros_sugar.core import Action, BaseComponent
from ros_sugar.utils import ActionReturnType, component_fallback

class SerialDriver(BaseComponent):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.on_system_fail(action=Action(self.reconnect), max_retries=3)

    def _execution_step(self):
        try:
            self.hw.read()
            self.health_status.set_healthy()
        except ConnectionError:
            self.health_status.set_fail_system()

    @component_fallback
    def reconnect(self) -> ActionReturnType:
        if self.hw.connect():
            return True, "reconnected"
        return False, "handshake failed"
```

The decorator checks the annotation at import, so a fallback that is declared to return anything but the pair fails before the recipe starts. Like `@component_action`, it takes a `description` that Cortex reads when the method is offered as a tool.

---

## Process-level recovery

Everything above runs inside a live component. When the whole process dies, from a segmentation fault in a native library, an out-of-memory kill or a hard exit, there is nothing left in it to run a fallback. That case belongs to the launcher:

```python
launcher.add_pkg(components=[...], package_name="my_pkg", multiprocessing=True)
launcher.on_process_fail(max_retries=3)
launcher.bringup()
```

A component launched with `multiprocessing=True` whose process exits with a non-zero code, outside shutdown and not by your own signal, is started again, up to the given number of times per component. After that it is left down and an error is logged. Stopping the recipe yourself never counts as a failure.

```{seealso}
- [Events and Actions](events-and-actions.md) for the actions a fallback is made of.
- [Routines](routines.md) for recoveries that need monitoring and retries of their own.
- [Launcher](launcher.md) for `on_process_fail` and the multi-process mode.
- [Fallback recipes](../recipes/events-and-resilience/fallback-recipes.md) for worked examples.
```
