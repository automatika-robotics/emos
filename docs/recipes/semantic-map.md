# Semantic Map

Autonomous Mobile Robots (AMRs) keep a representation of their environment in the form of occupancy maps. One can layer semantic information on top of these occupancy maps and with the use of multimodal LLMs one can even add answers to arbitrary questions about the environment to this map. In EMOS such maps can be created using vector databases which are specifically designed to store natural language data and retrieve it based on natural language queries. Thus an embodied agent can keep a text based _spatio-temporal memory_, from which it can do retrieval to answer questions or do spatial planning.

Here we will show an example of generating such a map using object detection information and questions answered by an MLLM. This map can of course be made arbitrarily complex and robust by adding checks on the data being stored, however in our example we will keep things simple. Lets start by importing relevant [components](../intelligence/ai-components.md).

```python
from agents.components import MapEncoding, Vision, MLLM
```

Next, we will use a vision component to provide us with object detections, as we did in the [Prompt Engineering](prompt-engineering.md) recipe.

## Setting up a Vision Component

```python
from agents.ros import Topic

# Define the image input topic
image0 = Topic(name="image_raw", msg_type="Image")
# Create a detection topic
detections_topic = Topic(name="detections", msg_type="Detections")
```

Additionally the component requires a model client with an object detection model. We will use the RESP client for RoboML and use the VisionModel, a convenient model class made available in EMOS for initializing all vision models available in the opensource [mmdetection](https://github.com/open-mmlab/mmdetection) library. We will specify the model we want to use by specifying the checkpoint attribute.

```{note}
Learn about setting up RoboML with vision [here](https://www.github.com/automatika-robotics/roboml).
```

```python
from agents.models import VisionModel
from agents.clients.roboml import RoboMLRESPClient
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

The vision component will provide us with semantic information to add to our map. However, object names are only the most basic semantic element of the scene. One can view such basic elements in aggregate to create more abstract semantic associations. This is where multimodal LLMs come in.

## Setting up an MLLM Component

With large scale multimodal LLMs we can ask higher level introspective questions about the sensor information the robot is receiving and record this information on our spatio-temporal map. As an example we will setup an MLLM component that periodically asks itself the same question, about the nature of the space the robot is present in. In order to achieve this we will use two concepts. First is that of a **FixedInput**, a simulated [Topic](../concepts/topics.md) that has a fixed value whenever it is read by a listener. And the second is that of a _timed_ component. In EMOS, components can get triggered by either an input received on a Topic or automatically after a certain period of time. This latter trigger specifies a timed component. Lets see what all of this looks like in code.

```python
from agents.clients import OllamaClient
from agents.models import OllamaModel
from agents.ros import FixedInput

# Define a model client (working with Ollama in this case)
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)

# Define a fixed input for the component
introspection_query = FixedInput(
    name="introspection_query", msg_type="String",
    fixed="What kind of a room is this? Is it an office, a bedroom or a kitchen? Give a one word answer, out of the given choices")
# Define output of the component
introspection_answer = Topic(name="introspection_answer", msg_type="String")

# Start a timed (periodic) component using the mllm model defined earlier
# This component answers the same question after every 15 seconds
introspector = MLLM(
    inputs=[introspection_query, image0],  # we use the image0 topic defined earlier
    outputs=[introspection_answer],
    model_client=qwen_client,
    trigger=15.0,  # we provide the time interval as a float value to the trigger parameter
    component_name="introspector",
)
```

LLM/MLLM model outputs can be unpredictable. Before publishing the answer of our question to the output topic, we want to ensure that the model has indeed provided a one word answer, and this answer is one of the expected choices. EMOS allows us to add arbitrary pre-processor functions to data that is going to be published (conversely, we can also add post-processing functions to data that has been received in a listener's callback, but we will see that in another recipe). We will add a simple pre-processing function to our output topic as follows:

```python
# Define an arbitrary function to validate the output of the introspective component
# before publication.
from typing import Optional

def introspection_validation(output: str) -> Optional[str]:
    for option in ["office", "bedroom", "kitchen"]:
        if option in output.lower():
            return option

introspector.add_publisher_preprocessor(introspection_answer, introspection_validation)
```

This should ensure that our component only publishes the model output to this topic if the validation function returns an output. All that is left to do now is to setup our MapEncoding component.

## Creating a Semantic Map as a Vector DB

The final step is to store the output of our models in a spatio-temporal map. EMOS provides a MapEncoding component that takes input data being published by other components and appropriately stores them in a vector DB. The input to a MapEncoding component is in the form of map layers. A _MapLayer_ is a thin abstraction over _Topic_, with certain additional parameters. We will create our map layers as follows:

```python
from agents.ros import MapLayer

# Object detection output from vision component
layer1 = MapLayer(subscribes_to=detections_topic, temporal_change=True)
# Introspection output from mllm component
layer2 = MapLayer(subscribes_to=introspection_answer, resolution_multiple=3)
```

_temporal_change_ parameter specifies that for the same spatial position the output coming in from the component needs to be stored along with timestamps, as the output can change over time. By default this option is set to **False**. _resolution_multiple_ specifies that we can coarse grain spatial coordinates by combining map grid cells.

Next we need to provide our component with localization information via an odometry topic and a map data topic (of type OccupancyGrid). The latter is necessary to know the actual resolution of the robots map.

```python
# Initialize mandatory topics defining the robots localization in space
position = Topic(name="odom", msg_type="Odometry")
map_topic = Topic(name="map", msg_type="OccupancyGrid")
```

```{caution}
Be sure to replace the name parameter in topics with the actual topic names being published on your robot.
```

Finally we initialize the MapEncoding component by providing it a database client. For the database client we will use HTTP DB client from RoboML. Much like model clients, the database client is initialized with a vector DB specification. For our example we will use Chroma DB, an open source multimodal vector DB.

```{seealso}
Checkout Chroma DB [here](https://trychroma.com).
```

```python
from agents.vectordbs import ChromaDB
from agents.clients import ChromaClient
from agents.config import MapConfig

# Initialize a vector DB that will store our semantic map
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)

# Create the map component
map_conf = MapConfig(map_name="map")  # We give our map a name
map = MapEncoding(
    layers=[layer1, layer2],
    position=position,
    map_topic=map_topic,
    config=map_conf,
    db_client=chroma_client,
    trigger=15.0,  # map layer data is stored every 15 seconds
    component_name="map_encoding",
)
```

## Launching the Components

And as always we will launch our components as we did in the previous recipes.

```python
from agents.ros import Launcher

# Launch the components
launcher = Launcher()
launcher.add_pkg(
    components=[vision, introspector, map]
    )
launcher.bringup()
```

And that is it. We have created our spatio-temporal semantic map using the outputs of two model components. The complete code for this recipe is below:

```{code-block} python
:caption: Semantic Mapping with MapEncoding
:linenos:
from typing import Optional
from agents.components import MapEncoding, Vision, MLLM
from agents.models import VisionModel, OllamaModel
from agents.clients import RoboMLRESPClient, ChromaClient, OllamaClient
from agents.ros import Topic, MapLayer, Launcher, FixedInput
from agents.vectordbs import ChromaDB
from agents.config import MapConfig, VisionConfig

# Define the image input topic
image0 = Topic(name="image_raw", msg_type="Image")
# Create a detection topic
detections_topic = Topic(name="detections", msg_type="Detections")

# Add an object detection model
object_detection = VisionModel(
    name="object_detection", checkpoint="dino-4scale_r50_8xb2-12e_coco"
)
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


# Define a model client (working with Ollama in this case)
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)

# Define a fixed input for the component
introspection_query = FixedInput(
    name="introspection_query",
    msg_type="String",
    fixed="What kind of a room is this? Is it an office, a bedroom or a kitchen? Give a one word answer, out of the given choices",
)
# Define output of the component
introspection_answer = Topic(name="introspection_answer", msg_type="String")

# Start a timed (periodic) component using the mllm model defined earlier
# This component answers the same question after every 15 seconds
introspector = MLLM(
    inputs=[introspection_query, image0],  # we use the image0 topic defined earlier
    outputs=[introspection_answer],
    model_client=qwen_client,
    trigger=15.0,  # we provide the time interval as a float value to the trigger parameter
    component_name="introspector",
)


# Define an arbitrary function to validate the output of the introspective component
# before publication.
def introspection_validation(output: str) -> Optional[str]:
    for option in ["office", "bedroom", "kitchen"]:
        if option in output.lower():
            return option


introspector.add_publisher_preprocessor(introspection_answer, introspection_validation)

# Object detection output from vision component
layer1 = MapLayer(subscribes_to=detections_topic, temporal_change=True)
# Introspection output from mllm component
layer2 = MapLayer(subscribes_to=introspection_answer, resolution_multiple=3)

# Initialize mandatory topics defining the robots localization in space
position = Topic(name="odom", msg_type="Odometry")
map_topic = Topic(name="map", msg_type="OccupancyGrid")

# Initialize a vector DB that will store our semantic map
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)

# Create the map component
map_conf = MapConfig(map_name="map")  # We give our map a name
map = MapEncoding(
    layers=[layer1, layer2],
    position=position,
    map_topic=map_topic,
    config=map_conf,
    db_client=chroma_client,
    trigger=15.0,
    component_name="map_encoding",
)

# Launch the components
launcher = Launcher()
launcher.add_pkg(
    components=[vision, introspector, map]
    )
launcher.bringup()
```
