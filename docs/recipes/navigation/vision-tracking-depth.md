# Vision Tracking with Depth

This tutorial guides you through creating a vision tracking system using a depth camera. We leverage RGBD with the `VisionRGBDFollower` in [Kompass](https://github.com/automatika-robotics/kompass) to detect and follow objects more robustly. With depth information available, this creates a more precise understanding of the environment and leads to more accurate and robust object following compared to [using RGB images alone](vision-tracking-rgb.md).

---

## Before You Start

### Setup Your Depth Camera ROS 2 Node

Your robot needs a depth camera to see in 3D and get the `RGBD` input. For this tutorial, we are using an **Intel RealSense** that is available on many mobile robots and well supported in ROS 2 and in simulation.

To get your RealSense camera running:

```bash
sudo apt install ros-<ros2-distro>-realsense2-camera

# Launch the camera node to start streaming both color and depth images
ros2 launch realsense2_camera rs_camera.launch.py
```

### Start vision detection using an ML model

To implement and run this example we will need a detection model processing the RGBD camera images to provide the Detection information. Similarly to the [RGB tutorial](vision-tracking-rgb.md), we will use [EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents). It provides a Vision Component which will allow us to easily deploy a ROS node in our system that interacts with vision models.

---

## Step 1: Vision Component and Model Client

In this example, we will set `enable_local_classifier` to `True` in the vision component so the model would be deployed directly on the robot. Additionally, we will set the input topic to be the `RGBD` camera topic. This setting will allow the `Vision` component to **publish both the depth and the RGB image data along with the detections**.

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

```{seealso}
See all available VisionModel options in the [Models](../../intelligence/models.md) reference, and all available model clients in the [Clients](../../intelligence/clients.md) reference.
```

---

## Step 2: Robot Configuration

You can set up your robot in the same way we did in the [RGB tutorial](vision-tracking-rgb.md). Here we use an Ackermann model as an example:

```python
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotGeometry,
    RobotType,
    RobotConfig,
)
import numpy as np

# Setup your robot configuration
my_robot = RobotConfig(
    model_type=RobotType.ACKERMANN,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=1.0, max_acc=3.0, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=4.0, max_acc=6.0, max_decel=10.0, max_steer=np.pi / 3
    ),
)
```

---

## Step 3: Controller with VisionRGBDFollower

Now we set up the `Controller` component to use the `VisionRGBDFollower`. Compared to the RGB version, we need two additional inputs:

- The **detections topic** from the vision component
- The **depth camera info topic** for depth-to-3D projection

```python
from kompass.components import Controller, ControllerConfig

depth_cam_info_topic = Topic(name="/camera/aligned_depth_to_color/camera_info", msg_type="CameraInfo")

config = ControllerConfig(ctrl_publish_type="Parallel")
controller = Controller(component_name="controller", config=config)
controller.inputs(vision_detections=detections_topic, depth_camera_info=depth_cam_info_topic)
controller.algorithm = "VisionRGBDFollower"
```

---

## Step 4: Helper Components

To make the system more complete and robust, we add:
- `DriveManager` -- to handle sending direct commands to the robot and ensure safety with its emergency stop
- `LocalMapper` -- to provide the controller with more robust local perception; to do so we also set the controller's `direct_sensor` property to `False`

```python
from kompass.components import DriveManager, LocalMapper

controller.direct_sensor = False
driver = DriveManager(component_name="driver")
mapper = LocalMapper(component_name="local_mapper")
```

---

## Full Recipe Code

```{code-block} python
:caption: vision_depth_follower.py
:linenos:

from agents.components import Vision
from agents.config import VisionConfig
from agents.ros import Topic
from kompass.components import Controller, ControllerConfig, DriveManager, LocalMapper
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotGeometry,
    RobotType,
    RobotConfig,
)
from kompass.ros import Launcher
import numpy as np


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

# Setup your robot configuration
my_robot = RobotConfig(
    model_type=RobotType.ACKERMANN,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=1.0, max_acc=3.0, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=4.0, max_acc=6.0, max_decel=10.0, max_steer=np.pi / 3
    ),
)

depth_cam_info_topic = Topic(name="/camera/aligned_depth_to_color/camera_info", msg_type="CameraInfo")

# Setup the controller
config = ControllerConfig(ctrl_publish_type="Parallel")
controller = Controller(component_name="controller", config=config)
controller.inputs(vision_detections=detections_topic, depth_camera_info=depth_cam_info_topic)
controller.algorithm = "VisionRGBDFollower"
controller.direct_sensor = False

# Add additional helper components
driver = DriveManager(component_name="driver")
mapper = LocalMapper(component_name="local_mapper")

# Bring it up with the launcher
launcher = Launcher()
launcher.add_pkg(components=[vision], ros_log_level="warn",
                 package_name="automatika_embodied_agents",
                 executable_entry_point="executable",
                 multiprocessing=True)
launcher.kompass(components=[controller, mapper, driver])
# Set the robot config for all components
launcher.robot = my_robot
launcher.bringup()
```

```{tip}
You can take your design to the next step and make your system more robust by adding some [events](../events-and-resilience/event-driven-cognition.md) or defining some [fallbacks](../events-and-resilience/fallback-recipes.md).
```
