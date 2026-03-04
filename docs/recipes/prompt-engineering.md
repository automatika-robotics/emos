# Prompt Engineering

In this recipe we will use the output of an object detection component to enrich the prompt of a VLM (MLLM) component. Let us start by importing the components.

```python
from agents.components import Vision, MLLM
```

## Setting up the Object Detection Component

For object detection and tracking, EMOS provides a unified Vision [component](../intelligence/ai-components.md). This component takes as input an image topic published by a camera device onboard our robot. The output of this component can be a _detections_ topic in case of object detection or a _trackings_ topic in case of object tracking. In this example we will use a _detections_ topic.

```python
from agents.ros import Topic

# Define the image input topic
image0 = Topic(name="image_raw", msg_type="Image")
# Create a detection topic
detections_topic = Topic(name="detections", msg_type="Detections")
```

Additionally the component requires a model client with an object detection model. We will use the RESP client for RoboML and use the VisionModel, a convenient model class made available in EMOS for initializing all vision models available in the opensource [mmdetection](https://github.com/open-mmlab/mmdetection) library. We will specify the model we want to use by specifying the checkpoint attribute.

```{note}
Learn about setting up RoboML with vision [here](https://github.com/automatika-robotics/roboml/blob/main/README.md#for-vision-models-support).
```

```{seealso}
Checkout all available mmdetection models and their benchmarking results in the [mmdetection model zoo](https://github.com/open-mmlab/mmdetection?tab=readme-ov-file#overview-of-benchmark-and-model-zoo).
```

```python
from agents.models import VisionModel
from agents.clients import RoboMLRESPClient, RoboMLHTTPClient
from agents.config import VisionConfig

# Add an object detection model
object_detection = VisionModel(name="object_detection",
                               checkpoint="dino-4scale_r50_8xb2-12e_coco")
roboml_detection = RoboMLRESPClient(object_detection)

# Initialize the Vision component
detection_config = VisionConfig(threshold=0.5)
vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=detection_config,
    model_client=roboml_detection,
    component_name="detection_component",
)
```

```{tip}
Notice that we passed in an optional config to the component. Component configs can be used to setup various parameters in the component. If the component calls an ML model then inference parameters for the model can be set in the component config.
```

## Setting up the MLLM Component

For the MLLM component, we will provide an additional text input topic, which will listen to our queries. The output of the component will be another text topic. We will use the RoboML HTTP client with the multimodal LLM Idefics2 by the good folks at HuggingFace for this example.

```python
from agents.models import TransformersMLLM

# Define MLLM input and output text topics
text_query = Topic(name="text0", msg_type="String")
text_answer = Topic(name="text1", msg_type="String")

# Define a model client (working with roboml in this case)
idefics = TransformersMLLM(name="idefics_model", checkpoint="HuggingFaceM4/idefics2-8b")
idefics_client = RoboMLHTTPClient(idefics)

# Define an MLLM component
# We can pass in the detections topic which we defined previously directly as an optional input
# to the MLLM component in addition to its other required inputs
mllm = MLLM(
    inputs=[text_query, image0, detections_topic],
    outputs=[text_answer],
    model_client=idefics_client,
    trigger=text_query,
    component_name="mllm_component"
)
```

Next we will setup a component level prompt to ensure that our text query and the output of the detections topic are sent to the model as we intend. We will do this by passing a jinja2 template to the **set_component_prompt** function.

```python
mllm.set_component_prompt(
    template="""Imagine you are a robot.
    This image has following items: {{ detections }}.
    Answer the following about this image: {{ text0 }}"""
)
```

```{caution}
The names of the topics used in the jinja2 template are the same as the name parameters set when creating the Topic objects.
```

## Launching the Components

Finally we will launch our components as we did in the previous example.

```python
from agents.ros import Launcher

# Launch the components
launcher = Launcher()
launcher.add_pkg(
    components=[vision, mllm]
    )
launcher.bringup()
```

And there we have it. Complete code of this example is provided below.

```{code-block} python
:caption: Prompt Engineering with Object Detection
:linenos:
from agents.components import Vision, MLLM
from agents.models import VisionModel, TransformersMLLM
from agents.clients import RoboMLRESPClient, RoboMLHTTPClient
from agents.ros import Topic, Launcher
from agents.config import VisionConfig

image0 = Topic(name="image_raw", msg_type="Image")
detections_topic = Topic(name="detections", msg_type="Detections")

object_detection = VisionModel(
    name="object_detection", checkpoint="dino-4scale_r50_8xb2-12e_coco"
)
roboml_detection = RoboMLRESPClient(object_detection)

detection_config = VisionConfig(threshold=0.5)
vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=detection_config,
    model_client=roboml_detection,
    component_name="detection_component",
)

text_query = Topic(name="text0", msg_type="String")
text_answer = Topic(name="text1", msg_type="String")

idefics = TransformersMLLM(name="idefics_model", checkpoint="HuggingFaceM4/idefics2-8b")
idefics_client = RoboMLHTTPClient(idefics)

mllm = MLLM(
    inputs=[text_query, image0, detections_topic],
    outputs=[text_answer],
    model_client=idefics_client,
    trigger=text_query,
    component_name="mllm_component"
)

mllm.set_component_prompt(
    template="""Imagine you are a robot.
    This image has following items: {{ detections }}.
    Answer the following about this image: {{ text0 }}"""
)
launcher = Launcher()
launcher.add_pkg(
    components=[vision, mllm]
    )
launcher.bringup()
```
