# Event-Driven Cognition

Robots process a massive amount of sensory data. Running a large Vision Language Model (VLM) on every single video frame to ask "What is happening?", while possible with smaller models, is in fact computationally expensive and redundant.

In this tutorial, we will use the **Event-Driven** nature of EMOS to create a smart "Reflex-Cognition" loop. We will use a lightweight detector to monitor the scene efficiently (the Reflex), and only when a specific object (a person) is found, we will trigger a larger VLM to describe them (the Cognition). One can imagine that this description can be used for logging robot's observations or parsed for triggering further actions downstream.

## The Strategy: Reflex and Cognition

1. **Reflex (Vision Component):** A fast, lightweight object detector runs on every frame. It acts as a gatekeeper.
2. **Event (The Trigger):** We define a smart event that fires only when the detector finds a "person" (and hasn't seen one recently).
3. **Cognition (VLM Component):** A more powerful VLM wakes up only when triggered by the event to describe the scene.

### 1. The Reflex: Vision Component

First, we set up the `Vision` component. This component is designed to be lightweight. By enabling the local classifier, we can run a small optimized model contained within the component, directly on the edge.

```python
from agents.components import Vision
from agents.config import VisionConfig
from agents.ros import Topic

# Define Topics
camera_image = Topic(name="/image_raw", msg_type="Image")
detections = Topic(name="/detections", msg_type="Detections") # Output of Vision

# Setup the Vision Component (The Trigger)
# We use a lower threshold to ensure we catch people easily and we use a small embedded model
vision_config = VisionConfig(threshold=0.6, enable_local_classifier=True)

vision_detector = Vision(
    inputs=[camera_image],
    outputs=[detections],
    trigger=camera_image, # Runs on every frame
    config=vision_config,
    component_name="eye_detector",
)
```

The `trigger=camera_image` argument tells this component to process every single message that arrives on the `/image_raw` topic.

### 2. The Trigger: Smart Events

Now, we need to bridge the gap between detection and description. We don't want the VLM to fire 30 times a second just because a person is standing in the frame.

We use `Event` with `on_change` mode. This event type is perfect for state changes. It monitors a list inside a message (in this case, the `labels` list of the detections).

```python
from agents.ros import Event

# Define the Event
# This event listens to the 'detections' topic.
# It triggers ONLY if the "labels" list inside the message contains "person"
# after not containing a person (within a 5 second interval).
event_person_detected = Event(
    detections.msg.labels.contains_any(["person"]),
    on_change=True,  # Trigger only when a change has occurred to stop repeat triggering
    keep_event_delay=5,  # A delay in seconds
)
```

```{note}
**`keep_event_delay=5`**: This is a debouncing mechanism. It ensures that once the event triggers, it won't trigger again for at least 5 seconds, even if the person remains in the frame. This prevents our VLM from being flooded with requests and can be quite useful to prevent jittery detections, which are common especially for mobile robots.
```

```{seealso}
Events can be used to create arbitrarily complex agent graphs. Check out all the events available in the [Events](../../concepts/events-and-actions.md) documentation.
```

### 3. The Cognition: VLM Component

Finally, we set up the heavy lifter. We will use a `VLM` component powered by **Qwen-VL** running on Ollama.

Crucially, this component does **not** have a topic trigger like the vision detector. Instead, it is triggered by `event_person_detected`.

We also need to tell the VLM _what_ to do when it wakes up. Since there is no user typing a question, we inject a `FixedInput`, a static prompt that acts as a standing order.

```python
from agents.components import VLM
from agents.clients import OllamaClient
from agents.models import OllamaModel
from agents.ros import FixedInput

description_output = Topic(name="/description", msg_type="String") # Output of VLM

# Setup a model client for the component
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:7b")
ollama_client = OllamaClient(model=qwen_vl)

# We define a fixed prompt that is injected whenever the component runs.
fixed_prompt = FixedInput(
    name="prompt",
    msg_type="String",
    fixed="A person has been detected. Describe their appearance briefly.",
)

visual_describer = VLM(
    inputs=[fixed_prompt, camera_image], # Takes the fixed prompt + current image
    outputs=[description_output],
    model_client=ollama_client,
    trigger=event_person_detected, # CRITICAL: Only runs when the event fires
    component_name="visual_describer",
)
```

## Launching the Application

We combine everything into a launcher.

```python
from agents.ros import Launcher

# Launch
launcher = Launcher()
launcher.add_pkg(
    components=[vision_detector, visual_describer],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

## See the results in the UI

We can see this recipe in action if we enable the UI. We can do so by simply adding the following line in the launcher.

```python
launcher.enable_ui(outputs=[camera_image, detections, description_output])
```

````{note}
In order to run the client you will need to install [FastHTML](https://www.fastht.ml/) and [MonsterUI](https://github.com/AnswerDotAI/MonsterUI) with
```shell
pip install python-fasthtml monsterui
````

The client displays a web UI on **http://localhost:5001** if you have run it on your machine. Or you can access it at **http://<IP_ADDRESS_OF_THE_ROBOT>:5001** if you have run it on the robot.

### Complete Code

Here is the complete recipe for the Event-Driven Cognition agent:

```{code-block} python
:caption: Event-Driven Cognition
:linenos:
from agents.components import Vision, VLM
from agents.config import VisionConfig
from agents.clients import OllamaClient
from agents.models import OllamaModel
from agents.ros import Launcher, Topic, FixedInput, Event

# Define Topics
camera_image = Topic(name="/image_raw", msg_type="Image")
detections = Topic(name="/detections", msg_type="Detections") # Output of Vision
description_output = Topic(name="/description", msg_type="String") # Output of VLM

# Setup the Vision Component (The Trigger)
# We use a lower threshold to ensure we catch people easily and we use a small local model
vision_config = VisionConfig(threshold=0.6, enable_local_classifier=True)

vision_detector = Vision(
    inputs=[camera_image],
    outputs=[detections],
    trigger=camera_image, # Runs on every frame
    config=vision_config,
    component_name="eye_detector",
)

# Define the Event
# This event listens to the 'detections' topic.
# It triggers ONLY if the "labels" list inside the message contains "person"
# after not containing a person (within a 5 second interval).
event_person_detected = Event(
    detections.msg.labels.contains_any(["person"]),
    on_change=True,  # Trigger only when a change has occurred to stop repeat triggering
    keep_event_delay=5,  # A delay in seconds
)

# Setup the VLM Component (The Responder)
# This component does NOT run continuously. It waits for the event.

# Setup a model client for the component
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:7b")
ollama_client = OllamaClient(model=qwen_vl)


# We define a fixed prompt that is injected whenever the component runs.
fixed_prompt = FixedInput(
    name="prompt",
    msg_type="String",
    fixed="A person has been detected. Describe their appearance briefly.",
)

visual_describer = VLM(
    inputs=[fixed_prompt, camera_image], # Takes the fixed prompt + current image
    outputs=[description_output],
    model_client=ollama_client,
    trigger=event_person_detected, # CRITICAL: Only runs when the event fires
    component_name="visual_describer",
)

# Launch
launcher = Launcher()
launcher.enable_ui(outputs=[camera_image, detections, description_output])
launcher.add_pkg(
    components=[vision_detector, visual_describer],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```
