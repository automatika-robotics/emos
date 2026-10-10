# MoveIt Manipulation

Not every manipulation task wants a learned policy. Moving an arm to a pose, following a straight line, picking a box the camera found and putting it down somewhere else: these are what motion planning does well, with collision checking and predictable motion. The **MoveIt** component brings [MoveIt 2](https://moveit.ai) into a recipe as one component with an action server, gripper actions and a planning scene that the robot's cameras can fill, and everything it can do is a tool for [Cortex](../../intelligence/cortex.md).

In this tutorial we bring up an arm with its MoveIt configuration, send it goals of every kind, drive the gripper, pick and place an object, and finish by handing the whole thing to Cortex.

## Prerequisites

The component drives a running `move_group`, the node that MoveIt's configuration package for a robot provides. You need that package for your arm, which the arm's vendor usually ships, and the MoveIt message packages, which EMOS installs in every mode.

```{admonition} Simulation setup
:class: note

A simulated arm to follow this tutorial with, in the same way the [VLA tutorial](vla-manipulation.md) has one, is coming soon. Until then, the Panda demo configuration that ships with MoveIt runs the arm on fake controllers and is enough for everything on this page:

    sudo apt install ros-$ROS_DISTRO-moveit-resources-panda-moveit-config
```

<!-- TODO screenshot: the simulated arm once the simulation setup is published -->

## Bringing up the arm

The configuration names the planning groups from the robot's SRDF: the arm, and the gripper if there is one. The recipe includes the MoveIt launch file so that `move_group` comes up with the component, and the launcher restarts it if it dies.

```python
from agents.components import MoveIt
from agents.config import MoveItConfig
from agents.ros import Launcher

config = MoveItConfig(
    arm_group_name="panda_arm",
    gripper_group_name="hand",
    max_velocity_scaling=0.2,
)
manipulator = MoveIt(config=config, component_name="manipulator")

launcher = Launcher()
launcher.include_launch_file(package="moveit_resources_panda_moveit_config", launch_file="demo.launch.py")
launcher.add_pkg(components=[manipulator])
launcher.bringup()
```

Velocity and acceleration scaling default to a tenth of the robot's limits, which is where a new setup should start. Pass `launch_args={"use_rviz": "false"}` to the launch file on a robot without a display.

## Sending goals

The component serves the `<component_name>/manipulate_with_moveit` action. A goal names what to reach in its `mode`, or leaves it empty and lets the component infer it from the fields that are filled in.

| Mode        | Reaches                                                                                                                                     |
| :---------- | :------------------------------------------------------------------------------------------------------------------------------------------ |
| `named`     | A named target from the SRDF, such as `ready` or `home`. The names are read from the running `move_group`.                                  |
| `pose`      | An end-effector pose. An identity orientation means no preference.                                                                          |
| `joints`    | Explicit joint positions.                                                                                                                   |
| `cartesian` | A straight-line path through waypoints, executed only if enough of it is achievable. Waypoints without an orientation keep the current one. |
| `pick`      | A grasp sequence on an object in the planning scene: approach, open, descend, close, attach, lift.                                          |
| `place`     | The reverse, for the object currently attached: approach the target, descend, open, detach, retreat.                                        |

From a terminal, once the recipe reports its components started:

```bash
ros2 action send_goal /manipulator/manipulate_with_moveit \
    automatika_embodied_agents/action/MoveManipulator "{mode: named, named_target: ready}"

ros2 action send_goal /manipulator/manipulate_with_moveit \
    automatika_embodied_agents/action/MoveManipulator \
    "{mode: pose, target_pose: {header: {frame_id: panda_link0}, pose: {position: {x: 0.4, y: 0.1, z: 0.4}, orientation: {w: 1.0}}}}"
```

A goal also takes `plan_only` to plan without moving, and `velocity_scaling` and `acceleration_scaling` to override the configured limits for that motion. The result carries `success`, MoveIt's error code, a message, and for Cartesian goals the fraction of the path that was achieved. While it runs, the feedback reports the state: planning, monitoring or executing.

## The gripper

Gripper control is a set of component actions rather than goals: `open_gripper`, `close_gripper` and, with a gripper controller, `set_gripper(position)`. From a terminal they are called through the component's method service:

```bash
ros2 service call /manipulator/execute_method automatika_ros_sugar/srv/ExecuteMethod \
    "{name: open_gripper, kwargs_json: '{}'}"
```

`gripper_mode` decides how they act. The default, `move_group`, plans the gripper's planning group to its `open` and `close` named targets. `gripper_command` sends a `GripperCommand` action to a controller instead, at `gripper_command_action`, with the open and close positions and the effort from the config. `stop_motion` halts whatever the arm is doing.

## Picking and placing

A pick needs something to pick, as an object in the planning scene. Objects get there in one of two ways. The camera puts them there, which the next section covers, or the recipe adds them, as boxes with a centre and a size in a frame:

```bash
ros2 service call /manipulator/execute_method automatika_ros_sugar/srv/ExecuteMethod \
    "{name: add_collision_object, kwargs_json: '{\"object_id\": \"cube\", \"center\": [0.5, 0.0, 0.05], \"size\": [0.05, 0.05, 0.05], \"frame_id\": \"panda_link0\"}'}"
```

Then the pick names the object, and the place names where it goes:

```bash
ros2 action send_goal /manipulator/manipulate_with_moveit \
    automatika_embodied_agents/action/MoveManipulator "{mode: pick, target_object: cube}"

ros2 action send_goal /manipulator/manipulate_with_moveit \
    automatika_embodied_agents/action/MoveManipulator \
    "{mode: place, target_pose: {header: {frame_id: panda_link0}, pose: {position: {x: 0.3, y: 0.3, z: 0.05}, orientation: {w: 1.0}}}}"
```

The pick approaches from above by default, from `approach_clearance` metres over the object, or from the side with `approach_mode="side"`. A picked object is attached to the end-effector in the scene, so the planner keeps it clear of obstacles while it is carried, and a place leaves it in the scene where it was released. The sequences assume a planning frame whose z axis points up.

The scene actions round this out: `remove_collision_object`, `clear_collision_objects`, `list_collision_objects`, `attach_object` and `detach_object` for objects, and `clear_octomap` for the occupancy map.

## Seeing the objects

Give the component a `Detections3D` input and what the cameras see becomes the planning scene. Each object arrives as a box in metres, and the component adds it as a collision object named `det__<label>_<rank>`, so `target_object: mug` picks the mug the camera found and every motion plans around the rest.

The boxes come from a Vision component, which lifts its 2D detections into 3D itself when it is given a `Detections3D` output and a source of depth. Three sources work:

- **An RGBD input.** A camera that publishes colour and depth in one message, as a RealSense does with `enable_rgbd`, calibration included. The simplest route.
- **A depth image plus camera info.** The aligned or `depth_registered` stream of a stereo camera, passed as `depth`, with the colour camera's `camera_info`.
- **A point cloud plus camera info.** A cloud from the camera or from a LiDAR, passed as `depth`, with the camera's `camera_info`. TF has to relate the cloud's frame to the camera's.

```python
from agents.components import Vision
from agents.config import VisionConfig
from agents.ros import Topic

rgbd = Topic(name="/camera/rgbd", msg_type="RGBD")
detections_3d = Topic(name="/detections_3d", msg_type="Detections3D")

vision = Vision(
    inputs=[rgbd],
    outputs=[detections_3d],
    config=VisionConfig(enable_local_classifier=True, detections_frame="panda_link0"),
    trigger=rgbd,
    component_name="vision",
)
```

For each box the component takes the median depth inside it, projects the box into space with the camera's intrinsics, and transforms it into `detections_frame`, which for the arm is its planning frame. A box carries a `depth_validity`, the share of it that had usable depth, and boxes below `min_depth_validity` are dropped; lower that for sparse LiDAR clouds. `min_depth` and `max_depth` bound the range, `max_depth_age` how far apart the colour and depth stamps may be, and `static_camera_tf` is set false for a camera on a moving joint. With a camera that publishes depth separately, pass `depth=` and `camera_info=` to the component; they are arguments rather than inputs, since inputs are the pictures the detector runs on.

A VLM does the same for a description. With a planning model and `task="grounding"`, and `detections_frame` in its config, "the red mug" becomes a box labelled with the query, and its `run_task` action does that on demand, which is how Cortex asks where the mug is before it sends the pick. [Multimodal Planning](planning-models.md) covers the planning models. The same boxes feed [Memory](../../intelligence/memory.md), which stores each object at its own position.

On the arm's side, the input and two settings say how the scene follows the camera:

```python
config = MoveItConfig(
    arm_group_name="panda_arm",
    gripper_group_name="hand",
    scene_update_mode="on_goal",
    scene_detection_labels=["mug", "bottle", "box"],
)
manipulator = MoveIt(config=config, inputs=[detections_3d], component_name="manipulator")
```

`scene_update_mode` is `manual`, updating only when `update_planning_scene` is called, `on_goal`, just before each goal, or `continuous`, at `scene_update_rate`. `scene_detection_labels` keeps only the labels you name, objects older than `scene_object_ttl` seconds are dropped, and the scene freezes while an object is held.

## With Cortex

Nothing more is needed for Cortex to use the arm. The action server becomes the `send_goal_to_manipulator_manipulate_with_moveit` tool, with the goal's fields as its parameters, and every action above becomes a `manipulator-<action>` tool, `manipulator-open_gripper` for instance. `get_named_targets` and `list_collision_objects` are offered to the planner as well as the executor, so it can look before it plans.

```python
from agents.components import Cortex, MoveIt

manipulator = MoveIt(config=config, inputs=[detections_3d], component_name="manipulator")
cortex = Cortex(model_client=planner_client, output=cortex_output, component_name="cortex")

launcher.add_pkg(components=[manipulator, vision, cortex])
```

Told "pick up the mug and put it on the tray", the planner lists the scene, sends a pick goal for the mug, waits for it, and sends a place goal at the tray's position. Told "open the gripper and go home", it calls the gripper action and a named goal. [Cortex: The Agentic Harness](cortex-agent.md) explains the planning loop this runs through.

## Complete code

```{code-block} python
:caption: Arm with MoveIt, planning scene from the camera
:linenos:

from agents.components import MoveIt, Vision
from agents.config import MoveItConfig, VisionConfig
from agents.ros import Launcher, Topic

# --- The camera's objects, as metric boxes (see the 3D Perception recipe) ---
rgbd = Topic(name="/camera/rgbd", msg_type="RGBD")
detections_3d = Topic(name="/detections_3d", msg_type="Detections3D")

vision = Vision(
    inputs=[rgbd],
    outputs=[detections_3d],
    config=VisionConfig(enable_local_classifier=True, detections_frame="panda_link0"),
    trigger=rgbd,
    component_name="vision",
)

# --- The arm ---
config = MoveItConfig(
    arm_group_name="panda_arm",
    gripper_group_name="hand",
    max_velocity_scaling=0.2,
    scene_update_mode="on_goal",
)
manipulator = MoveIt(config=config, inputs=[detections_3d], component_name="manipulator")

# --- Launch, with move_group from the arm's MoveIt configuration ---
launcher = Launcher()
launcher.include_launch_file(package="moveit_resources_panda_moveit_config", launch_file="demo.launch.py")
launcher.add_pkg(components=[vision, manipulator])
launcher.bringup()
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
