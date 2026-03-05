# GoTo Navigation

In the previous [recipe](semantic-map.md) we created a semantic map using the MapEncoding component. Intuitively one can imagine that using the map data would require some form of RAG. Let us suppose that we want to create a Go-to-X component, which, when given a command like 'Go to the yellow door', would retrieve the coordinates of the _yellow door_ from the map and publish them to a goal point topic of type _PoseStamped_ to be handled by our robot's [navigation system](../../navigation/overview.md). We will create our Go-to-X component using the LLM [component](../../intelligence/ai-components.md) provided by EMOS. We will start by initializing the component, and configuring it to use RAG.

## Initialize the component

```python
from agents.components import LLM
from agents.models import OllamaModel
from agents.config import LLMConfig
from agents.clients import OllamaClient
from agents.ros import Launcher, Topic

# Start a Llama3.2 based llm component using ollama client
llama = OllamaModel(name="llama", checkpoint="llama3.2:3b")
llama_client = OllamaClient(llama)

# Define LLM input and output topics including goal_point topic of type PoseStamped
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")
```

In order to configure the component to use RAG, we will set the following options in its config.

```python
config = LLMConfig(enable_rag=True,
                   collection_name="map",
                   distance_func="l2",
                   n_results=1,
                   add_metadata=True)
```

Note that the _collection_name_ parameter is the same as the map name we set in the previous [recipe](semantic-map.md). We have also set _add_metadata_ parameter to true to make sure that our metadata is included in the RAG result, as the spatial coordinates we want to get are part of the metadata. Let us have a quick look at the metadata stored in the map by the MapEncoding component.

```
{
    "coordinates": [1.1, 2.2, 0.0],
    "layer_name": "Topic_Name",  # same as topic name that the layer is subscribed to
    "timestamp": 1234567,
    "temporal_change": True
}
```

With this information, we will first initialize our component.

```{caution}
In the following code block we are using the same DB client that was setup in the [Semantic Map](semantic-map.md) recipe.
```

```python
# initialize the component
goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=llama_client,
    db_client=chroma_client,  # check the previous example where we setup this database client
    trigger=goto_in,
    config=config,
    component_name='go_to_x'
)
```

## Pre-process the model output before publishing

Knowing that the output of retrieval will be appended to the beginning of our query as context, we will setup a component level prompt for our LLM.

```python
# set a component prompt
goto.set_component_prompt(
    template="""From the given metadata, extract coordinates and provide
    the coordinates in the following json format:\n {"position": coordinates}"""
)
```

```{note}
One might notice that we have not used an input topic name in our prompt. This is because we only need the input topic to fetch data from the vector DB during the RAG step. The query to the LLM in this case would only be composed of data fetched from the DB and our prompt.
```

As the LLM output will contain text other than the _json_ string that we have asked for, we need to add a pre-processing function to the output topic that extracts the required part of the text and returns the output in a format that can be published to a _PoseStamped_ topic, i.e. a numpy array of floats.

```python
from typing import Optional
import json
import numpy as np

# pre-process the output before publishing to a topic of msg_type PoseStamped
def llm_answer_to_goal_point(output: str) -> Optional[np.ndarray]:
    # extract the json part of the output string (including brackets)
    # one can use sophisticated regex parsing here but we'll keep it simple
    json_string = output[output.find("{") : output.rfind("}") + 1]
    # load the string as a json and extract position coordinates
    # if there is an error, return None, i.e. no output would be published to goal_point
    try:
        json_dict = json.loads(json_string)
        coordinates = np.fromstring(json_dict["position"], sep=',', dtype=np.float64)
        print('Coordinates Extracted:', coordinates)
        if coordinates.shape[0] < 2 or coordinates.shape[0] > 3:
            return
        elif coordinates.shape[0] == 2:  # sometimes LLMs avoid adding the zeros of z-dimension
            coordinates = np.append(coordinates, 0)
        return coordinates
    except Exception:
        return

# add the pre-processing function to the goal_point output topic
goto.add_publisher_preprocessor(goal_point, llm_answer_to_goal_point)
```

## Launching the Components

And we will launch our Go-to-X component.

```python
from agents.ros import Launcher

# Launch the component
launcher = Launcher()
launcher.add_pkg(
    components=[goto]
    )
launcher.bringup()
```

And that is all. Our Go-to-X component is ready. The complete code for this recipe is given below:

```{code-block} python
:caption: Go-to-X Component
:linenos:
from typing import Optional
import json
import numpy as np
from agents.components import LLM
from agents.models import OllamaModel
from agents.vectordbs import ChromaDB
from agents.config import LLMConfig
from agents.clients import ChromaClient, OllamaClient
from agents.ros import Launcher, Topic

# Start a Llama3.2 based llm component using ollama client
llama = OllamaModel(name="llama", checkpoint="llama3.2:3b")
llama_client = OllamaClient(llama)

# Initialize a vector DB that will store our routes
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)

# Define LLM input and output topics including goal_point topic of type PoseStamped
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

config = LLMConfig(enable_rag=True,
                   collection_name="map",
                   distance_func="l2",
                   n_results=1,
                   add_metadata=True)

# initialize the component
goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=llama_client,
    db_client=chroma_client,  # check the previous example where we setup this database client
    trigger=goto_in,
    config=config,
    component_name='go_to_x'
)

# set a component prompt
goto.set_component_prompt(
    template="""From the given metadata, extract coordinates and provide
    the coordinates in the following json format:\n {"position": coordinates}"""
)


# pre-process the output before publishing to a topic of msg_type PoseStamped
def llm_answer_to_goal_point(output: str) -> Optional[np.ndarray]:
    # extract the json part of the output string (including brackets)
    # one can use sophisticated regex parsing here but we'll keep it simple
    json_string = output[output.find("{") : output.rfind("}") + 1]
    # load the string as a json and extract position coordinates
    # if there is an error, return None, i.e. no output would be published to goal_point
    try:
        json_dict = json.loads(json_string)
        coordinates = np.fromstring(json_dict["position"], sep=',', dtype=np.float64)
        print('Coordinates Extracted:', coordinates)
        if coordinates.shape[0] < 2 or coordinates.shape[0] > 3:
            return
        elif coordinates.shape[0] == 2:  # sometimes LLMs avoid adding the zeros of z-dimension
            coordinates = np.append(coordinates, 0)
        return coordinates
    except Exception:
        return


# add the pre-processing function to the goal_point output topic
goto.add_publisher_preprocessor(goal_point, llm_answer_to_goal_point)

# Launch the component
launcher = Launcher()
launcher.add_pkg(
    components=[goto]
    )
launcher.bringup()
```
