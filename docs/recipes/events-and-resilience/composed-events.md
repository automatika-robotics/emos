# Logic Gates & Composed Events

In the previous recipes, we triggered actions based on single, isolated conditions -- "If Mapper Fails" or "If Person Detected". But real-world autonomy is rarely that simple. A robot shouldn't stop *every* time it sees an obstacle -- maybe only if it's moving fast. It shouldn't return home *just* because the battery is low -- maybe only after finishing its current task.

In this recipe, we use **logic operators** to compose multiple conditions into smarter, more robust event triggers.

---

## Logic Operators

EMOS lets you compose complex triggers using standard Python bitwise operators. This turns your Event definitions into a high-level logic circuit.

| Logic | Operator | Description | Use Case |
|:---|:---|:---|:---|
| **AND** | `&` | All conditions must be True | Speed > 0 **AND** Obstacle Close |
| **OR** | `\|` | At least one condition is True | Lidar Blocked **OR** Bumper Hit |
| **NOT** | `~` | Inverts the condition | Target Seen **AND NOT** Low Battery |

---

## Navigation Example: Smart Emergency Stop

**The problem:** A naive emergency stop triggers whenever an object is within 0.5m. But if the robot is docking or maneuvering in a tight elevator, this stops it unnecessarily.

**The solution:** Trigger ONLY if an obstacle is close **AND** the robot is moving fast.

### 1. Define the Data Sources

```python
from kompass.ros import Topic

# Radar distance reading (0.2s timeout for safety-critical data)
radar = Topic(name="/radar_front", msg_type="Float32", data_timeout=0.2)

# Odometry (0.5s timeout)
odom = Topic(name="/odom", msg_type="Odometry", data_timeout=0.5)
```

### 2. Compose the Event

```python
from kompass.ros import Event, Action

# Condition A: Obstacle within 0.3m
is_obstacle_close = radar.msg.data < 0.3

# Condition B: Robot moving faster than 0.1 m/s
is_robot_moving_fast = odom.msg.twist.twist.linear.x > 0.1

# Composed Event: BOTH must be True
event_smart_stop = Event(
    event_condition=(is_obstacle_close & is_robot_moving_fast),
    on_change=True
)
```

### 3. Wire to Action

```python
from kompass.components import DriveManager

driver = DriveManager(component_name="drive_manager")

# Emergency stop action
stop_action = Action(method=driver.stop)

events_actions = {
    event_smart_stop: stop_action,
}
```

Now the robot stops only when it *should* -- fast approach toward an obstacle -- and ignores close objects during slow precision maneuvers.

---

## Intelligence Example: Conditional Cognition

We can apply the same logic to the intelligence layer. Consider the [Event-Driven Cognition](event-driven-cognition.md) recipe where a Vision detector triggers a VLM. What if we only want the VLM to run when the robot has sufficient battery?

```python
from agents.ros import Topic, Event

# Detection output from the Vision component
detections = Topic(name="/detections", msg_type="Detections")

# Battery level topic
battery = Topic(name="/battery_state", msg_type="Float32")

# Condition A: Person detected (with debounce)
person_detected = detections.msg.labels.contains_any(["person"])

# Condition B: Battery above 20%
battery_ok = battery.msg.data > 20.0

# Composed Event: person detected AND battery sufficient
event_describe_person = Event(
    event_condition=(person_detected & battery_ok),
    on_change=True,
    keep_event_delay=5
)
```

The VLM only wakes up when both conditions are met -- saving compute when the battery is low.

---

## OR Logic: Redundant Sensors

The `|` operator is useful for sensor redundancy. If *either* the front lidar detects a close obstacle or the bumper is pressed, trigger an emergency stop:

```python
from kompass.ros import Topic, Event

lidar = Topic(name="/scan_front", msg_type="Float32")
bumper = Topic(name="/bumper", msg_type="Bool")

is_lidar_blocked = lidar.msg.data < 0.2
is_bumper_pressed = bumper.msg.data == True

event_any_collision = Event(
    event_condition=(is_lidar_blocked | is_bumper_pressed),
    on_change=True
)
```

---

## NOT Logic: Exclusion

The `~` operator inverts a condition. Use it to exclude scenarios:

```python
from kompass.ros import Topic, Event

# Only track the target if the robot is NOT in manual override mode
manual_mode = Topic(name="/manual_override", msg_type="Bool")
target_seen = detections.msg.labels.contains_any(["person"])

event_auto_track = Event(
    event_condition=(target_seen & ~manual_mode.msg.data),
    on_change=True
)
```

---

## Event Configuration Reference

All composed events support these parameters:

| Parameter | Description | Default |
|---|---|---|
| `on_change` | Trigger only when the condition *transitions* to True (edge-triggered) | `False` |
| `handle_once` | Fire only once during the system's lifetime | `False` |
| `keep_event_delay` | Minimum seconds between consecutive triggers (debounce) | `0` |

```{seealso}
For the full list of event types and configuration options, see the [Events & Actions](../../concepts/events-and-actions.md) reference.
```
