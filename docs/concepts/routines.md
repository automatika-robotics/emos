# Routines

A routine is a procedure with a name: an ordered sequence of steps, each with its own test for success and its own retry policy, that reports where it has got to while it runs. Where a chain of events leaves "detect, then approach, then grasp, then lift" implicit in the wiring, a routine says it in one place. It can be started by an event, from the dashboard, or by [Cortex](../intelligence/cortex.md) as a tool, and it can be paused, resumed and aborted from any of them.

```python
from ros_sugar.core import Action, Routine

pick = Routine(
    "pick_object",
    steps=[
        Action(perception.detect_object, success=perception_out.msg.object_found.is_true(), timeout=5.0),
        Action(arm.move_to_pregrasp, success=arm_state.msg.at_pregrasp.is_true(), timeout=10.0, cancel_method=arm.stop),
        Action(gripper.close, name="grasp", success=gripper_state.msg.closed.is_true(), timeout=3.0,
               max_retries=2, on_fail="fallback", fallback=gripper.reopen),
        Action(arm.lift, success=arm_state.msg.at_lift.is_true()),
    ],
    on_complete=logger_component.log_pick_done,
    on_abort=safety.open_gripper_and_home,
    on_pause=arm.stop,
    description="Detect the object in front of the robot, grasp it and lift it",
)

launcher.on(pick_requested, pick)
```

A routine is registered on an event exactly like an action, with `launcher.on` or in the `events_actions` of `add_pkg`. It does not need an event at all: one passed to `enable_ui(routines=[...])` or to Cortex is hosted just the same, so the dashboard or the agent can be the only thing that ever starts it. The `description` is for whoever has to choose among the routines available, an operator reading a card or a model planning with tools, so write it in plain words.

---

## Steps

A step is an ordinary `Action`. The parameters that make an action monitored, described in [Events and Actions](events-and-actions.md), decide whether the step worked: `success`, a condition on a topic or, when omitted, the method's own return value; `timeout` and `on_timeout`; `max_retries` and `retry_delay`, one budget per step that a reported failure and a timeout spend alike; and `cancel_method`, which is how the step is told to stop when the routine is paused or aborted.

When a step's budget is gone, `on_fail` decides what the sequence around it does:

| `on_fail`            | Effect                                                                                        |
| :------------------- | :-------------------------------------------------------------------------------------------- |
| `"abort"` (default)  | The routine ends as failed and `on_abort` runs.                                               |
| `"skip"`             | The failure is logged and the routine carries on with the next step.                          |
| `"fallback"`         | The action's `fallback` runs. The routine carries on if it succeeds and aborts if it does not. |

`name` is what the step is called in the routine's progress report. It defaults to the method's name, and it has to be unique within the routine.

Three other kinds of step are useful:

- **A wait.** `wait(duration=30.0)` from `ros_sugar.actions` is the one step that runs for as long as it was asked to. A routine that waits more than once names each wait, `wait(duration=5.0, name="settle")`. Pausing or aborting ends a wait at once, and resuming starts it over.
- **A goal to an action server.** An `ActionServerGoal` step sends a goal to a component's action server, or to any server by name, and takes the server's outcome as its verdict. It is the one kind of step that can be stopped for real: pausing or aborting cancels the goal on the server.
- **A bare function.** A callable given as a step is wrapped in an unmonitored action whose return value is its verdict.

```{tip}
Give every step that moves the robot a `timeout` and a `cancel_method`. A success condition without a timeout waits forever if the condition never comes, and a step without a cancel method cannot be interrupted once it is running, only abandoned.
```

---

## Started, not finished

Triggering a routine returns as soon as its first step has been dispatched:

```python
success, message = pick()   # (True, "Routine 'pick_object' started")
```

The outcome arrives later, through `on_complete` and `on_abort` and through the published progress. A recipe that has to react to a routine finishing keys on those, never on the result of whatever started it. Triggering a routine that is already running does nothing, so a repeating event cannot restart one in the middle of a procedure.

---

## Where it runs

A routine spans components, so no single component can host it. The launcher hands every routine to the Monitor, the one node that can reach them all. A step written as a component's method, `Action(arm.move)`, is not called on the object your recipe holds: the Monitor sends it to the component over the component's own service, so the step runs on the component whatever process it lives in, and components launched with `multiprocessing=True` take part like any other.

Arguments travel with the call, so a step can be given a ROS message, a field of one, an array or raw bytes. A value that cannot be sent, an open socket or an object only the recipe's process knows, is rejected at bringup, as is a step that targets a component the launcher does not know. A step that reads its arguments from a topic reads them when the step is entered, not when the routine was started, so it acts on what is true at that moment.

---

## Pausing, resuming and aborting

Control is available as ordinary system actions from `ros_sugar.actions`, so an event can drive it. An emergency stop can abort a routine, and a "clear" button can resume it:

```python
from ros_sugar.actions import abort_routine, pause_routine, resume_routine, start_routine

launcher.on(emergency_stop, abort_routine(routine_name="pick_object", reason="emergency stop"))
launcher.on(operator_pause, pause_routine(routine_name="pick_object"))
launcher.on(operator_go, resume_routine(routine_name="pick_object"))
```

A pause preempts the step in flight, runs `on_pause`, and holds there. Resuming enters that same step again from its start, since a step is the smallest thing a routine can be positioned at. An abort ends the routine at once and runs `on_abort`.

Preempting a step stops what the step itself runs, not what it set in motion. A navigation step that has already handed the robot a goal is stopped while the robot keeps driving. `on_pause` is where the routine undoes that, and it can be one action or several run in order:

```python
Routine(
    "go_to_kitchen",
    steps=[Action(planner.go_to, kwargs={"goal": kitchen})],
    on_pause=[Action(controller.stop_path_tracking), Action(driver.stop_robot)],
)
```

They run after the step has been preempted, while the routine reports itself paused. One that fails is logged and the rest still run, because stopping some of what moves is better than stopping none of it.

Taking a routine down is not the same as aborting it. When the recipe shuts down, whatever is in flight is preempted and `on_abort` does not run.

---

## Following progress

Every routine publishes its progress on `/routine/<name>/state` as JSON in a `std_msgs/String`:

```json
{"name": "pick_object", "status": "running", "index": 2,
 "active_step": "grasp", "steps": ["detect_object", "move_to_pregrasp", "grasp", "lift"],
 "step_message": "At pregrasp pose", "abort_reason": "", "elapsed": 4.31}
```

`status` is one of `idle`, `running`, `paused`, `completed`, `failed` and `aborted`. `step_message` is what the last finished step returned, or, for a failed routine, why the step it failed at did. `abort_reason` is the reason an aborted routine was given. The routine object keeps the same information in `latest_step_messages` and `step_messages()`.

The topic is latched: the state is published when the Monitor takes the routine on, and after that only when it changes. Subscribe with transient-local durability to receive the current state on connect:

```python
from rclpy.qos import DurabilityPolicy
from ros_sugar.config import QoSConfig

node.create_subscription(
    String, "/routine/pick_object/state", on_state,
    QoSConfig(durability=DurabilityPolicy.TRANSIENT_LOCAL, queue_size=1).to_ros(),
)
```

---

## From the dashboard and Cortex

Routines passed to `enable_ui` appear among the recipe's Tasks in its web interface, each with its steps checked off as it goes, a log of the steps it entered, and the controls its state allows: start while it is idle, pause and abort while it runs, resume and abort while it is paused. A routine can be given by name when something else registers it with the Monitor at run time.

```python
launcher.on(pick_requested, pick)
launcher.enable_ui(routines=[pick, patrol, "docking"])
```

The same controls are served to scripts by the recipe's JSON API, under `/api/routines`, and the state above is streamed over a WebSocket. See [Web UI](web-ui.md).

Cortex hosts routines as tools. Each becomes something the agent can start, follow and abort, and its `description` is what the model reads to decide when. See [Cortex](../intelligence/cortex.md).

```{seealso}
- [Events and Actions](events-and-actions.md) for monitored actions, the building blocks of a routine.
- [Launcher](launcher.md) for registering routines and the Monitor that hosts them.
- [Web UI](web-ui.md) for the Tasks cards and the JSON API.
```
