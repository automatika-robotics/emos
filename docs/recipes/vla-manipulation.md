# VLA Manipulation

The frontier of Embodied AI is moving away from modular pipelines (perception -> planning -> control) toward end-to-end learning. **Vision-Language-Action (VLA)** models take visual observations and natural language instructions as input and output direct robot joint commands.

In this tutorial, we will build an agent capable of performing physical manipulation tasks using the **VLA** component. We will utilize the [LeRobot](https://github.com/huggingface/lerobot) ecosystem to load a pretrained "SmolVLA" policy and connect it to a robot arm.

````{important}
In order to run this tutorial you will need to install LeRobot as a model serving platform. You can see the installation instructions [here](https://huggingface.co/docs/lerobot/installation). After installation run the LeRobot async inference server as follows.
```shell
python -m lerobot.async_inference.policy_server --host=<HOST_ADDRESS> --port=<PORT>
````

## Simulation Setup

**WILL BE ADDED SOON**

## Setting up our VLA based Agent

We will start by importing the relevant components.

```python
from agents.components import VLA
from agents.clients import LeRobotClient
from agents.models import LeRobotPolicy
```

## Defining the Senses and Actuators

Unlike purely digital agents, a VLA agent needs to be firmly grounded in its physical body. We need to define the ROS topics that represent the robot's state (proprioception), its vision (eyes), and its actions (motor commands).

In this example, we are working with a so101 arm setup requiring two camera angles, so we define two camera inputs alongside the robot's joint states.

```python
from agents.ros import Topic

# 1. Proprioception: The current angle of the robot's joints
state = Topic(name="/isaac_joint_states", msg_type="JointState")

# 2. Vision: The agent's eyes
camera1 = Topic(name="/front_camera/image_raw", msg_type="Image")
camera2 = Topic(name="/wrist_camera/image_raw", msg_type="Image")

# 3. Action: Where the VLA will publish command outputs
joints_action = Topic(name="/isaac_joint_command", msg_type="JointState")
```

## Setting up the Policy

To drive our VLA component, we need a robot policy. EMOS provides the `LeRobotPolicy` class, which interfaces seamlessly with models trained with LeRobot and hosted on the HuggingFace Hub.

We will use a finetuned **SmolVLA** model, a lightweight VLA policy trained by the LeRobot team and finetuned on our simulation scenario setup above. We also need to provide a `dataset_info_file`. This is useful because the VLA needs to know the statistical distribution of the training data (normalization stats) to correctly interpret the robot's raw inputs. This file is part of the standard LeRobot Dataset format. We will use the info file from the dataset on which our SmolVLA policy was finetuned on.

````{important}
In order to use the LeRobotClient you will need extra dependencies that can be installed as follows:
```shell
pip install grpcio protobuf
pip install torch --index-url https://download.pytorch.org/whl/cpu # And a lightweight CPU version (recommended) of torch
````

```python
# Specify the LeRobot Policy to use
policy = LeRobotPolicy(
    name="my_policy",
    policy_type="smolvla",
    checkpoint="aleph-ra/smolvla_finetune_pick_orange_20000",
    dataset_info_file="https://huggingface.co/datasets/LightwheelAI/leisaac-pick-orange/resolve/main/meta/info.json",
)

# Create the client
client = LeRobotClient(model=policy)
```

```{note}
The **policy_type** parameter supports various architectures including `diffusion`, `act`, `pi0`, and `smolvla`. Ensure this matches the architecture of your checkpoint.
```

## VLA Configuration

This is the most critical step. Pre-trained VLA models expect inputs to be named exactly as they were in the training dataset (e.g., "shoulder_pan.pos"). However, your robot's URDF likely uses different names (e.g., "Rotation" or "joint_1").

We use the `VLAConfig` to create a mapping layer that translates your robot's specific hardware signals into the language the model understands.

1. **Joint Mapping:** Map dataset keys to your ROS joint names.
2. **Camera Mapping:** Map dataset camera names to your ROS image topics.
3. **Safety Limits:** Provide the URDF file so the component knows the physical joint limits and can cap actions safely.

```python
from agents.config import VLAConfig

# Map dataset names (keys) -> Robot URDF names (values)
joints_map = {
    "shoulder_pan.pos": "Rotation",
    "shoulder_lift.pos": "Pitch",
    "elbow_flex.pos": "Elbow",
    "wrist_flex.pos": "Wrist_Pitch",
    "wrist_roll.pos": "Wrist_Roll",
    "gripper.pos": "Jaw",
}

# Map dataset camera names (keys) -> ROS Topics (values)
camera_map = {"front": camera1, "wrist": camera2}

config = VLAConfig(
    observation_sending_rate=3, # Hz: How often we infer
    action_sending_rate=3,      # Hz: How often we publish commands
    joint_names_map=joints_map,
    camera_inputs_map=camera_map,
    # URDF is required for safety capping and joint limit verification
    robot_urdf_file="./so101_new_calib.urdf"
)
```

```{warning}
If the `joint_names_map` is incomplete, the component will raise an error during initialization.
```

## The VLA Component

Now we assemble the component. The `VLA` component acts as a ROS2 Action Server. It creates a feedback loop: it ingests the state and images, processes them through the `LeRobotClient`, and publishes the resulting actions to the `joints_action` topic.

We also define a termination trigger. Since VLA tasks (like picking up an object) are finite, we can tell the component to stop after a specific number of timesteps.

```{note}
The termination trigger can be `timesteps`, `keyboard` and `event`. The event can be based on a topic published by another component observing the scene, for example a VLM component that is asking a periodic question to itself with a `FixedInput`. Check out the [Event-Driven VLA](event-driven-vla.md) recipe.
```

```python
from agents.components import VLA

vla = VLA(
    inputs=[state, camera1, camera2],
    outputs=[joints_action],
    model_client=client,
    config=config,
    component_name="vla_with_smolvla",
)

# Attach the stop trigger
vla.set_termination_trigger("timesteps", max_timesteps=50)
```

## Launching the Component

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.add_pkg(components=[vla])
launcher.bringup()
```

Now we can send our pick and place command to the component. Since the VLA component acts as a **ROS2 Action Server**, we can trigger it directly from the terminal using the standard `ros2 action` CLI.

Open a new terminal, source your workspace and send the goal (the natural language instruction) to the component. The action server endpoint defaults to `component_name/action_name`.

```bash
ros2 action send_goal /vla_with_smolvla/vision_language_action automatika_embodied_agents/action/VisionLanguageAction "{task: 'pick up the oranges and place them in the bowl'}"
```

```{note}
The `task` string is the natural language instruction that the VLA model conditions its actions on. Ensure this instruction matches the distribution of prompts used during the training of the model (e.g. "pick orange", "put orange in bin" etc).
```

And there you have it! You have successfully configured an end-to-end VLA agent. The complete code is available below.

```{code-block} python
:caption: Vision Language Action Agent
:linenos:

from agents.components import VLA
from agents.config import VLAConfig
from agents.clients import LeRobotClient
from agents.models import LeRobotPolicy
from agents.ros import Topic, Launcher

# --- Define Topics ---
state = Topic(name="/isaac_joint_states", msg_type="JointState")
camera1 = Topic(name="/front_camera/image_raw", msg_type="Image")
camera2 = Topic(name="/wrist_camera/image_raw", msg_type="Image")
joints_action = Topic(name="/isaac_joint_command", msg_type="JointState")

# --- Setup Policy (The Brain) ---
policy = LeRobotPolicy(
    name="my_policy",
    policy_type="smolvla",
    checkpoint="aleph-ra/smolvla_finetune_pick_orange_20000",
    dataset_info_file="https://huggingface.co/datasets/LightwheelAI/leisaac-pick-orange/resolve/main/meta/info.json",
)

client = LeRobotClient(model=policy)

# --- Configure Mapping (The Nervous System) ---
# Map dataset names -> robot URDF names
joints_map = {
    "shoulder_pan.pos": "Rotation",
    "shoulder_lift.pos": "Pitch",
    "elbow_flex.pos": "Elbow",
    "wrist_flex.pos": "Wrist_Pitch",
    "wrist_roll.pos": "Wrist_Roll",
    "gripper.pos": "Jaw",
}

# Map dataset cameras -> ROS topics
camera_map = {"front": camera1, "wrist": camera2}

config = VLAConfig(
    observation_sending_rate=3,
    action_sending_rate=3,
    joint_names_map=joints_map,
    camera_inputs_map=camera_map,
    # Ensure you provide a valid path to your robot's URDF
    robot_urdf_file="./so101_new_calib.urdf"
)

# --- Initialize Component ---
vla = VLA(
    inputs=[state, camera1, camera2],
    outputs=[joints_action],
    model_client=client,
    config=config,
    component_name="vla_with_smolvla",
)

# Set the component to stop after a certain number of timesteps
vla.set_termination_trigger('timesteps', max_timesteps=50)

# --- Launch ---
launcher = Launcher()
launcher.add_pkg(components=[vla])
launcher.bringup()
```
