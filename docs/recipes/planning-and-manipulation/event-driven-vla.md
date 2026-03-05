# Event-Driven VLA

In the previous [VLA Manipulation](vla-manipulation.md) recipe, we saw how VLAs can be used in EMOS to perform physical tasks. However, the real utility of VLAs is unlocked when they are part of a bigger cognitive system. With its event-driven agent graph development, EMOS allows us to do exactly that.

Most VLA policies are "open-loop" regarding task completion -- they run for a fixed number of steps and then stop, regardless of whether they succeeded or failed.

In this tutorial, we will build a **Closed-Loop Agent** while using an open-loop policy. Even if the model correctly outputs its termination condition (i.e. an absorbing state policy), our design can act as a safety valve. We will combine:

- {material-regular}`smart_toy;1.2em;sd-text-primary` **The Player (VLA):** Attempts to pick up an object.
- {material-regular}`visibility;1.2em;sd-text-primary` **The Referee (VLM):** Watches the camera stream and judges if the task is complete.

We will use the **Event System** to trigger a stop command on the VLA the moment the VLM confirms success.

## The Player: Setting up the VLA

First, we setup our VLA component exactly as we did in the previous recipe. We will use the same **SmolVLA** policy trained for picking oranges.

```python
from agents.components import VLA
from agents.config import VLAConfig
from agents.clients import LeRobotClient
from agents.models import LeRobotPolicy
from agents.ros import Topic

# Define Topics
state = Topic(name="/isaac_joint_states", msg_type="JointState")
camera1 = Topic(name="/front_camera/image_raw", msg_type="Image")
camera2 = Topic(name="/wrist_camera/image_raw", msg_type="Image")
joints_action = Topic(name="/isaac_joint_command", msg_type="JointState")

# Setup Policy
policy = LeRobotPolicy(
    name="my_policy",
    policy_type="smolvla",
    checkpoint="aleph-ra/smolvla_finetune_pick_orange_20000",
    dataset_info_file="https://huggingface.co/datasets/LightwheelAI/leisaac-pick-orange/resolve/main/meta/info.json",
)
client = LeRobotClient(model=policy)

# Configure VLA (Mapping omitted for brevity, see previous tutorial)
# ... (assume joints_map and camera_map are defined)
config = VLAConfig(
    observation_sending_rate=5,
    action_sending_rate=5,
    joint_names_map=joints_map,
    camera_inputs_map=camera_map,
    robot_urdf_file="./so101_new_calib.urdf"
)

player = VLA(
    inputs=[state, camera1, camera2],
    outputs=[joints_action],
    model_client=client,
    config=config,
    component_name="vla_player",
)
```

## The Referee: Setting up the VLM

Now we introduce the "Referee". We will use a Vision Language Model (like Qwen-VL) to monitor the scene.

We want this component to periodically look at the `camera1` feed and answer a specific question: _"Are all the oranges in the bowl?"_

We use a `FixedInput` to ensure the VLM is asked the exact same question every time.

```python
from agents.components import VLM
from agents.clients import OllamaClient
from agents.models import OllamaModel
from agents.ros import FixedInput

# Define the topic where the VLM publishes its judgment
referee_verdict = Topic(name="/referee/verdict", msg_type="String")

# Setup the Model
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:7b")
qwen_client = OllamaClient(model=qwen_vl)

# Define the constant question
question = FixedInput(
    name="prompt",
    msg_type="String",
    fixed="Look at the image. Are all the orange in the bowl? Answer only with YES or NO."
)

# Initialize the VLM
# Note: We trigger periodically (regulated by loop_rate)
referee = VLM(
    inputs=[question, camera1],
    outputs=[referee_verdict],
    model_client=qwen_client,
    trigger=10.0,
    component_name="vlm_referee"
)
```

```{note}
To prevent the VLM from consuming too much compute, we have configured a `float` trigger, which means our `VLM` component will be triggered, not by a topic, but periodically with a `loop_rate` of once every 10 seconds.
```

```{tip}
In order to make sure that the VLM output is formatted as per our requirement (YES or NO), checkout how to use pre-processors in the [Semantic Map](../foundation/semantic-map.md) recipe. For now we will assume that if YES is part of the output string, the event should fire.
```

## The Bridge: Semantic Event Trigger

Now comes the "Self-Referential" magic. We simply define an **Event** that fires when the `/referee/verdict` topic contains the word "YES".

```python
from agents.ros import Event

# Define the Success Event
event_task_success = Event(
    referee_verdict.msg.data.contains("YES")  # the topic, attribute and value to check in it
)
```

Finally, we attach this event to the VLA using the `set_termination_trigger` method. We set the mode to `event`.

```python
# Tell the VLA to stop immediately when the event fires
player.set_termination_trigger(
    mode="event",
    stop_event=event_task_success,
    max_timesteps=500 # Fallback: stop if 500 steps pass without success
)
```

```{seealso}
Events are a very powerful concept in EMOS. You can get infinitely creative with them. For example, imagine setting off the VLA component with a voice command. This can be done by combining the output of a SpeechToText component and an Event that generates an action command. To learn more about them check out the recipes for [Events & Actions](../events-and-resilience/event-driven-cognition.md).
```

## Launching the System

When we launch this graph:

- The **VLA** starts moving the robot to pick the orange.
- The **VLM** simultaneously watches the feed.
- Once the oranges are in the bowl, the VLM outputs "YES".
- The **Event** system catches this, interrupts the VLA, and signals that the task is complete.

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.add_pkg(components=[player, referee])
launcher.bringup()
```

You can send the action command to the VLA as defined in the previous [VLA Manipulation](vla-manipulation.md) recipe.

## Complete Code

```{code-block} python
:caption: Closed-Loop VLA with VLM Verifier
:linenos:

from agents.components import VLA, VLM
from agents.config import VLAConfig
from agents.clients import LeRobotClient, OllamaClient
from agents.models import LeRobotPolicy, OllamaModel
from agents.ros import Topic, Launcher, FixedInput
from agents.ros import Event

# --- Define Topics ---
state = Topic(name="/isaac_joint_states", msg_type="JointState")
camera1 = Topic(name="/front_camera/image_raw", msg_type="Image")
camera2 = Topic(name="/wrist_camera/image_raw", msg_type="Image")
joints_action = Topic(name="/isaac_joint_command", msg_type="JointState")
referee_verdict = Topic(name="/referee/verdict", msg_type="String")

# --- Setup The Player (VLA) ---
policy = LeRobotPolicy(
    name="my_policy",
    policy_type="smolvla",
    checkpoint="aleph-ra/smolvla_finetune_pick_orange_20000",
    dataset_info_file="https://huggingface.co/datasets/LightwheelAI/leisaac-pick-orange/resolve/main/meta/info.json",
)
vla_client = LeRobotClient(model=policy)

# VLA Config (Mappings assumed defined as per previous tutorial)
# joints_map = { ... }
# camera_map = { ... }

config = VLAConfig(
    observation_sending_rate=5,
    action_sending_rate=5,
    joint_names_map=joints_map,
    camera_inputs_map=camera_map,
    robot_urdf_file="./so101_new_calib.urdf"
)

player = VLA(
    inputs=[state, camera1, camera2],
    outputs=[joints_action],
    model_client=vla_client,
    config=config,
    component_name="vla_player",
)

# --- Setup The Referee (VLM) ---
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:7b")
qwen_client = OllamaClient(model=qwen_vl)

# A static prompt for the VLM
question = FixedInput(
    name="prompt",
    msg_type="String",
    fixed="Look at the image. Are all the orange in the bowl? Answer only with YES or NO."
)

referee = VLM(
    inputs=[question, camera1],
    outputs=[referee_verdict],
    model_client=qwen_client,
    trigger=camera1,
    component_name="vlm_referee"
)

# --- Define the Logic (Event) ---
# Create an event that looks for "YES" in the VLM's output
event_task_success = Event(
    referee_verdict.msg.data.contains("YES")  # the topic, attribute and value to check in it
)

# Link the event to the VLA's stop mechanism
player.set_termination_trigger(
    mode="event",
    stop_event=event_success,
    max_timesteps=400 # Failsafe
)

# --- Launch ---
launcher = Launcher()
launcher.add_pkg(components=[player, referee])
launcher.bringup()
```
