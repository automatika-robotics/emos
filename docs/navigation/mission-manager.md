# Mission Manager

**Multi-waypoint missions as one action.**

A patrol, a delivery round, an inspection route: most real jobs are not one goal but a sequence of them, with something to do at each stop and a decision to make when a stop fails or takes too long. The Mission Manager runs such a sequence as a single ROS 2 action. It sends each waypoint to the [Planner](planning.md), stops the robot when a waypoint is reached and something has to happen there, dwells or waits for a signal, and reports where the mission has got to, all through one goal you can follow, pause, resume or cancel.

Underneath, a mission is a Sugarcoat [routine](../concepts/routines.md). The component translates the goal into a routine, hands it to the Monitor, and reports the routine's progress back as feedback. Retries, timeouts and preemption are the routine engine's; the component owns the translation, the frames, the stopping and the arrival tolerance.

## Wiring

The Mission Manager is given the planner, the controller and the drive manager it works with. The planner has to run as an action server, since that is how waypoints are sent to it.

```python
from kompass.components import Controller, DriveManager, MissionManager, MissionManagerConfig, Planner

planner = Planner(component_name="planner")
planner.run_type = "ActionServer"
controller = Controller(component_name="controller")
driver = DriveManager(component_name="drive_manager")

mission = MissionManager(
    component_name="mission",
    planner=planner,
    controller=controller,
    drive_manager=driver,
    config=MissionManagerConfig(waypoint_timeout=180.0),
)
```

Only the components' names are kept, so the mission survives being launched in its own process. A configuration file sets the same three things directly, as `planner_action` (`planner/navigate_to_goal`), `controller_name` and `drive_manager_name`.

| Property | Value                                                                                 |
| :------- | :------------------------------------------------------------------------------------ |
| Run type | Action server only. A mission is one long goal.                                       |
| Action   | `run_mission`, of type `kompass_interfaces/action/MultiGoalPlanPath`                 |
| Input    | `location`: `Odometry`, `PoseStamped` or `Pose`. Used for the feedback, the end displacement and returning to start. |
| Output   | `mission_status`: `kompass_interfaces/msg/MissionStatus`, latched                     |

## The goal

| Field                   | Meaning                                                                                                              |
| :---------------------- | :------------------------------------------------------------------------------------------------------------------- |
| `goals`                 | The waypoints, as poses, visited in order.                                                                            |
| `frame_id`              | The frame the waypoints are in. Empty means the world frame. Another frame is transformed before the mission starts. |
| `algorithm_name`        | A planning algorithm, or empty for the planner's default.                                                           |
| `end_tolerance`         | Arrival tolerance, applied at every waypoint.                                                                        |
| `pause_duration`        | Seconds to dwell at a reached waypoint: empty for none, one value for all, or one per waypoint.                     |
| `pause_condition_topic` | A `Bool` topic to wait on at each reached waypoint before going on.                                                  |
| `condition_timeout`     | Seconds to wait for that signal. Zero or less waits indefinitely.                                                    |
| `on_timeout`            | What a wait running out means: `ON_TIMEOUT_CONTINUE`, `ON_TIMEOUT_RETURN_TO_START` or `ON_TIMEOUT_ABORT`.            |

From a terminal:

```bash
ros2 action send_goal /mission/run_mission kompass_interfaces/action/MultiGoalPlanPath \
  "{goals: [{position: {x: 1.0, y: 0.0}, orientation: {w: 1.0}},
            {position: {x: 2.0, y: 1.0}, orientation: {w: 1.0}}],
    end_tolerance: {orientation_error: 0.2, lateral_distance_error: 0.15},
    pause_duration: [5.0]}" --feedback
```

A goal that describes no mission is refused before anything starts: no waypoints, a `pause_duration` list of the wrong length, an unknown timeout policy, or a return-to-start policy while the robot's start pose is unknown.

### What the mission does at each waypoint

The robot drives to the waypoint through the planner. If there is nothing to do there, it goes straight on to the next one, which simply replaces the plan being driven. If there is a dwell or a signal to wait for, the mission first stops path tracking and stops the robot, then waits. A wait at the last waypoint is skipped when it is a signal, since there is nothing to continue to; a dwell there still runs. Under the continue policy a wait that runs out is fine and the mission goes on; under the other two it ends the mission.

```{admonition} Returning to start is not followed to the end yet
:class: caution

With `ON_TIMEOUT_RETURN_TO_START`, the drive back to the start is begun but not followed to completion in this version: the mission reports its outcome and the routine is removed while the return is still under way.
```

## Following a mission

The same progress is available three ways: as feedback on the `run_mission` action, on the `mission_status` topic for anything that did not send the goal, and as the routine's own state on `routine/navigation_mission/state`. The feedback carries the current waypoint index, the robot's pose and the time spent in the current pause, and its state says what the mission is doing: navigating, dwelling, waiting for a condition, returning to start, or paused. The status topic adds the states only true once the goal is over: idle, completed, canceled or aborted, published once and kept for late subscribers.

The result says how far the mission got: an outcome, one flag per waypoint saying whether it was reached, the index of the last one reached, and the distance and heading error between the robot and it.

## Pausing, resuming and ending

`pause_mission` and `resume_mission` are component actions, so an event can call them, a routine can, and so can the web interface or a terminal:

```bash
ros2 service call /mission/execute_method automatika_ros_sugar/srv/ExecuteMethod "{name: 'pause_mission'}"
```

Pausing stops the robot where it is. Resuming starts the current step over: a waypoint being driven to is driven to again, a dwell restarts, a wait waits for a new signal. Cancelling the goal, or the `cancel_main_action` service, ends the mission; the planner goal in flight is cancelled, which publishes an empty plan and stops the controller. Deactivating the component ends an ongoing mission first.

## Configuration

| Parameter              | Default                | Purpose                                                                                     |
| :--------------------- | :--------------------- | :------------------------------------------------------------------------------------------ |
| `waypoint_timeout`     | 300 s                  | Time allowed for one waypoint before the mission gives up on it.                            |
| `cursor_poll_rate`     | 5 Hz                   | How often progress is read, which sets how promptly feedback is published.                  |
| `retries`              | 2                      | Extra attempts at stopping the robot and at reading progress before the mission ends.       |
| `end_mission_timeout`  | 10 s                   | How long a deactivation waits for the ongoing mission to end.                               |
| `ui_waypoints_topic`   | `/mission_waypoints`   | Where the web interface publishes waypoints picked on the map.                              |
| `routine_name`         | `navigation_mission`   | The name of the routine every mission runs as. Missions run one at a time.                  |

## From the web interface

The component gathers what its browser card needs, so the recipe passes them through:

```python
launcher.enable_ui(inputs=[*mission.ui_inputs], outputs=[*mission.ui_outputs])
```

Kompass registers its own card for the mission action: waypoints are picked on the map, the goal is sent, the journey is shown as a checklist as it progresses, and the mission can be paused, resumed and cancelled from the card. [Multi-Waypoint Missions](../recipes/navigation/multi-waypoint-mission.md) walks through a recipe.
