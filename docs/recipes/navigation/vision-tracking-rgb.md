# Vision Tracking with RGB

In this tutorial we will create a vision-based target following navigation system to follow a moving target using an RGB camera input. This recipe demonstrates a core EMOS pattern: combining an [EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) perception component with a [Kompass](https://github.com/automatika-robotics/kompass) navigation controller in a single script.

---

## Before You Start

### Get and start your camera ROS 2 node

Based on the type of the camera used on your robot, you need to install and launch its respective ROS 2 node provided by the manufacturer.

To run and test this example on your development machine, you can use your webcam along with the `usb_cam` package:
```shell
sudo apt install ros-<ros2-distro>-usb-cam
ros2 run usb_cam usb_cam_node_exe
```

### Start vision detection/tracking using an ML model

To implement and run this example we will need a detection model processing the RGB camera images to provide the Detection or Tracking information. The most convenient way to obtain this is to use [EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) and [RoboML](https://github.com/automatika-robotics/roboml) to deploy and serve the model locally. EmbodiedAgents provides a Vision Component, which will allow us to easily deploy a ROS node in our system that interacts with vision models.

Before starting with this tutorial you need to install both packages:

- Install **EMOS**: check the instructions in the [installation guide](../../getting-started/installation.md)
- Install RoboML: `pip install roboml`

After installing both packages, you can start `roboml` to serve the model later either on the robot (or your development machine), or on another machine in the local network or any server in the cloud. To start a RoboML RESP server, simply run:

```shell
roboml-resp
```
```{tip}
Save the IP of the machine running `roboml` as we will use it later in our model client.
```

---

## Step 1: Vision Model Client

First, we need to import the `VisionModel` class that defines the model used later in the component, and a model client to communicate with the model which can be running on the same hardware or in the cloud. Here we will use a `RESPModelClient` from RoboML as we activated the RESP based model server in RoboML.

```python
from agents.models import VisionModel
from agents.clients import RoboMLRESPClient
```

Now let's configure the model we want to use for detections/tracking and the model client:

```python
object_detection = VisionModel(
    name="object_detection",
    checkpoint="rtmdet_tiny_8xb32-300e_coco",
)
roboml_detection = RoboMLRESPClient(object_detection, host='127.0.0.1', logging_level="warn")
  # 127.0.0.1 should be replaced by the IP of the machine running roboml.
```
The model is configured with a name and a checkpoint (any checkpoint from the mmdetection framework can be used, see [available checkpoints](https://github.com/open-mmlab/mmdetection?tab=readme-ov-file#overview-of-benchmark-and-model-zoo)). In this example, we have chosen a model checkpoint trained on the MS COCO dataset which has over 80 [classes](https://github.com/amikelive/coco-labels/blob/master/coco-labels-2014_2017.txt) of commonly found objects.

---

## Step 2: Vision Component

We start by importing the required component along with its configuration class:

```python
from agents.components import Vision
from agents.config import VisionConfig
```

After setting up the model client, we need to select the input/output topics to configure the vision component:

```python
from agents.ros import Topic

# RGB camera input topic is set to the compressed image topic
image0 = Topic(name="/image_raw/compressed", msg_type="CompressedImage")

# Select the output topics: detections and trackings
detections_topic = Topic(name="detections", msg_type="Detections")
trackings_topic = Topic(name="trackings", msg_type="Trackings")

# Select the vision component configuration
detection_config = VisionConfig(
    threshold=0.5, enable_visualization=True
)

# Create the component
vision = Vision(
    inputs=[image0],
    outputs=[detections_topic, trackings_topic],
    trigger=image0,
    config=detection_config,
    model_client=roboml_detection,
    component_name="detection_component",
)
```
The component inputs/outputs are defined to get the images from the camera topic and provide both detections and trackings. The `trigger` of the component is set to the image input topic so the component works in an Event-Based runtype and provides a new detection/tracking on each new image.

In the component configuration, the parameter `enable_visualization` is set to `True` to get a visualization of the output on an additional pop-up window for debugging purposes. The `threshold` parameter (confidence threshold for object detection) is set to `0.5`.

---

## Step 3: Robot Configuration

We can select the robot motion model, control limits and other geometry parameters using the `RobotConfig` class:

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
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=0.2, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)
```

```{seealso}
See more details about the robot configuration in the [Point Navigation](point-navigation.md#step-1-robot-configuration) recipe and in the [Robot Configuration](../../navigation/robot-config.md) reference.
```

---

## Step 4: Navigation Controller

To implement the target following system we will use the `Controller` component to generate the tracking commands and the `DriveManager` to handle the safe communication with the robot driver.

We select the vision follower method parameters by importing the config class `VisionRGBFollowerConfig` (see default parameters in the [Algorithms](../../advanced/algorithms.md) reference), then configure both our components:

```python
from kompass.components import Controller, ControllerConfig, DriveManager
from kompass.control import VisionRGBFollowerConfig

# Set the controller component configuration
config = ControllerConfig(loop_rate=10.0, ctrl_publish_type="Sequence", control_time_step=0.3)

# Init the controller
controller = Controller(
    component_name="my_controller", config=config
)
# Set the vision tracking input to either the detections or trackings topic
controller.inputs(vision_tracking=detections_topic)

# Set the vision follower configuration
vision_follower_config = VisionRGBFollowerConfig(
    control_horizon=3, enable_search=False, target_search_pause=6, tolerance=0.2
)
controller.algorithms_config = vision_follower_config

# Init the drive manager with the default parameters
driver = DriveManager(component_name="my_driver")
```
Here we selected a loop rate for the controller of `10Hz` and a control step for generating the commands of `0.3s`, and we selected to send the commands sequentially as they get computed. The vision follower is configured with a `control_horizon` equal to three future control time steps and a `target_search_pause` equal to 6 control time steps. We also chose to disable the search, meaning that the tracking action would end when the robot loses the target.

```{tip}
`target_search_pause` is implemented so the robot would pause and wait while tracking to avoid losing the target due to quick movement and slow model response. It should be adjusted based on the inference time of the model.
```

---

## Step 5: Launch

All that is left is to add all three components to the launcher and bring up the system.

```python
from kompass.ros import Launcher

launcher = Launcher()

# setup agents as a package in the launcher and add the vision component
launcher.add_pkg(
    components=[vision],
    ros_log_level="warn",
)

# setup the navigation components in the launcher
launcher.kompass(
    components=[controller, driver],
)
# Set the robot config for all components
launcher.robot = my_robot

# Start all the components
launcher.bringup()
```

---

## Full Recipe Code

```{code-block} python
:caption: vision_rgb_follower.py
:linenos:

import numpy as np
from agents.components import Vision
from agents.models import VisionModel
from agents.clients import RoboMLRESPClient
from agents.config import VisionConfig
from agents.ros import Topic

from kompass.components import Controller, ControllerConfig, DriveManager
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotGeometry,
    RobotType,
    RobotConfig,
)
from kompass.control import VisionRGBFollowerConfig
from kompass.ros import Launcher

# RGB camera input topic is set to the compressed image topic
image0 = Topic(name="/image_raw/compressed", msg_type="CompressedImage")

# Select the output topics: detections and trackings
detections_topic = Topic(name="detections", msg_type="Detections")
trackings_topic = Topic(name="trackings", msg_type="Trackings")

object_detection = VisionModel(
    name="object_detection",
    checkpoint="rtmdet_tiny_8xb32-300e_coco",
)
roboml_detection = RoboMLRESPClient(object_detection, host='127.0.0.1', logging_level="warn")

# Select the vision component configuration
detection_config = VisionConfig(threshold=0.5, enable_visualization=True)

# Create the component
vision = Vision(
    inputs=[image0],
    outputs=[detections_topic, trackings_topic],
    trigger=image0,
    config=detection_config,
    model_client=roboml_detection,
    component_name="detection_component",
)

# Setup your robot configuration
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=0.2, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)

# Set the controller component configuration
config = ControllerConfig(
    loop_rate=10.0, ctrl_publish_type="Sequence", control_time_step=0.3
)

# Init the controller
controller = Controller(component_name="my_controller", config=config)
controller.inputs(vision_tracking=detections_topic)

# Set the vision follower configuration
vision_follower_config = VisionRGBFollowerConfig(
    control_horizon=3, enable_search=False, target_search_pause=6, tolerance=0.2
)
controller.algorithms_config = vision_follower_config

# Init the drive manager with the default parameters
driver = DriveManager(component_name="my_driver")

launcher = Launcher()
launcher.add_pkg(
    components=[vision],
    ros_log_level="warn",
)
launcher.kompass(
    components=[controller, driver],
)
launcher.robot = my_robot
launcher.bringup()
```

---

## Trigger the Following Action

After running your complete system you can send a goal to the controller's action server `/my_controller/track_vision_target` of type `kompass_interfaces.action.TrackVisionTarget` to start tracking a selected label (`person` for example):

```shell
ros2 action send_goal /my_controller/track_vision_target kompass_interfaces/action/TrackVisionTarget "{label: 'person'}"
```

You can also re-run the previous script and activate the target search by adding the following config or sending the config along with the action send_goal:

```python
vision_follower_config = VisionRGBFollowerConfig(
    control_horizon=3, enable_search=True, target_search_pause=6, tolerance=0.2
)
```

```shell
ros2 action send_goal /my_controller/track_vision_target kompass_interfaces/action/TrackVisionTarget "{label: 'person', search_radius: 1.0, search_timeout: 30}"
```

---

## Next Steps

- **[Vision Tracking with Depth](vision-tracking-depth.md)** -- Extend this approach with RGBD for more robust and accurate following.
- **[Runtime Model Fallback](../events-and-resilience/fallback-recipes.md)** -- Make your recipe robust by switching models on failure.
