# External Reflexes

In [Event-Driven Cognition](event-driven-cognition.md), we used a lightweight detector to wake a heavy VLM -- intelligence reacting to the world. In this recipe, we apply the same pattern to the **navigation layer**: the robot transitions from idle patrol to active person-following the moment a human appears in the camera feed.

This is an **External Reflex** -- an event triggered by the environment (not an internal failure) that reconfigures the robot's behavior at runtime.

---

## The Strategy

1. **Reflex (Vision Component):** A lightweight detector runs on every frame, scanning for "person".
2. **Event (The Trigger):** Fires when "person" first appears in the detection labels.
3. **Response (Controller Reconfiguration):** Two actions execute in sequence:
   - Switch the Controller's algorithm to `VisionRGBDFollower`
   - Send a goal to the Controller's ActionServer to begin tracking

---

## Step 1: The Vision Detector

We use the `Vision` component from the intelligence layer with a small embedded classifier -- fast enough to process every frame.

```python
from agents.components import Vision
from agents.config import VisionConfig
from agents.ros import Topic

image0 = Topic(name="/camera/rgbd", msg_type="RGBD")
detections_topic = Topic(name="detections", msg_type="Detections")

detection_config = VisionConfig(threshold=0.5, enable_local_classifier=True)
vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=detection_config,
    component_name="detection_component",
)
```

## Step 2: Define the Event

We use `on_change=True` so the event fires only when "person" *first* appears in the detection labels -- not continuously while a person remains in frame.

```python
from kompass.ros import Event

event_person_detected = Event(
    event_condition=detections_topic.msg.labels.contains("person"),
    on_change=True
)
```

## Step 3: Define the Actions

When the event fires, two actions execute **in sequence**:

1. **Switch algorithm** -- reconfigure the Controller from its current mode to `VisionRGBDFollower`
2. **Trigger the ActionServer** -- send a goal specifying "person" as the tracking target

```python
from kompass.actions import update_parameter, send_component_action_server_goal
from kompass_interfaces.action import TrackVisionTarget

# Action 1: Switch the controller algorithm
switch_algorithm_action = update_parameter(
    component=controller,
    param_name="algorithm",
    new_value="VisionRGBDFollower"
)

# Action 2: Send a tracking goal to the controller's action server
action_request_msg = TrackVisionTarget.Goal()
action_request_msg.label = "person"
action_start_person_following = send_component_action_server_goal(
    component=controller,
    request_msg=action_request_msg,
)
```

```{tip}
Linking an Event to a **list** of Actions executes them in sequence. This lets you chain reconfiguration steps -- switch algorithm first, then send the goal.
```

## Step 4: Wire and Launch

```python
events_action = {
    event_person_detected: [switch_algorithm_action, action_start_person_following]
}
```

---

## Complete Recipe

```{code-block} python
:caption: external_reflexes.py
:linenos:

import numpy as np
from agents.components import Vision
from agents.config import VisionConfig
from agents.ros import Topic
from kompass.components import Controller, ControllerConfig, DriveManager, LocalMapper
from kompass.robot import (
    AngularCtrlLimits, LinearCtrlLimits, RobotGeometry, RobotType, RobotConfig,
)
from kompass.ros import Launcher, Event
from kompass.actions import update_parameter, send_component_action_server_goal
from kompass_interfaces.action import TrackVisionTarget

# --- Vision Detector ---
image0 = Topic(name="/camera/rgbd", msg_type="RGBD")
detections_topic = Topic(name="detections", msg_type="Detections")

detection_config = VisionConfig(threshold=0.5, enable_local_classifier=True)
vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=detection_config,
    component_name="detection_component",
)

# --- Robot Configuration ---
my_robot = RobotConfig(
    model_type=RobotType.ACKERMANN,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=1.0, max_acc=3.0, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=4.0, max_acc=6.0, max_decel=10.0, max_steer=np.pi / 3
    ),
)

# --- Navigation Components ---
depth_cam_info_topic = Topic(
    name="/camera/aligned_depth_to_color/camera_info", msg_type="CameraInfo"
)

config = ControllerConfig(ctrl_publish_type="Parallel")
controller = Controller(component_name="controller", config=config)
controller.inputs(
    vision_detections=detections_topic,
    depth_camera_info=depth_cam_info_topic,
)
controller.algorithm = "VisionRGBDFollower"
controller.direct_sensor = False

driver = DriveManager(component_name="driver")
mapper = LocalMapper(component_name="local_mapper")

# --- Event: Person Detected ---
event_person_detected = Event(
    event_condition=detections_topic.msg.labels.contains("person"),
    on_change=True,
)

# --- Actions: Switch Algorithm + Start Following ---
switch_algorithm_action = update_parameter(
    component=controller,
    param_name="algorithm",
    new_value="VisionRGBDFollower",
)

action_request_msg = TrackVisionTarget.Goal()
action_request_msg.label = "person"
action_start_person_following = send_component_action_server_goal(
    component=controller,
    request_msg=action_request_msg,
)

events_action = {
    event_person_detected: [switch_algorithm_action, action_start_person_following],
}

# --- Launch ---
launcher = Launcher()

launcher.add_pkg(
    components=[vision],
    ros_log_level="warn",
    package_name="automatika_embodied_agents",
    executable_entry_point="executable",
    multiprocessing=True,
)

launcher.kompass(
    components=[controller, mapper, driver],
    events_actions=events_action,
)

launcher.robot = my_robot
launcher.bringup()
```
