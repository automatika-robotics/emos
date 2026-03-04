# Semantic Routing

The SemanticRouter component in EMOS allows you to route text queries to specific [components](../intelligence/ai-components.md) based on the user's intent or the output of a preceding component.

The router operates in two distinct modes:

1. **Vector Mode (Default):** This mode uses a Vector DB to calculate the mathematical similarity (distance) between the incoming query and the samples defined in your routes. It is extremely fast and lightweight.

2. **LLM Mode (Agentic):** This mode uses an LLM to intelligently analyze the intent of the query and triggers routes accordingly. This is more computationally expensive but can handle complex nuances, context, and negation (e.g., "Don't go to the kitchen" might be routed differently by an agent than a simple vector similarity search).

In this recipe, we will route queries between two components: a General Purpose LLM (for chatting) and a Go-to-X Component (for navigation commands) that we built in the previous [recipe](goto-navigation.md). Lets start by setting up our components.

## Setting up the components

In the following code snippet we will setup our two components.

```python
from typing import Optional
import json
import numpy as np
from agents.components import LLM, SemanticRouter
from agents.models import OllamaModel
from agents.vectordbs import ChromaDB
from agents.config import LLMConfig, SemanticRouterConfig
from agents.clients import ChromaClient, OllamaClient
from agents.ros import Launcher, Topic, Route

# Start a Llama3.2 based llm component using ollama client
llama = OllamaModel(name="llama", checkpoint="llama3.2:3b")
llama_client = OllamaClient(llama)

# Initialize a vector DB that will store our routes
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)


# Make a generic LLM component using the Llama3_2 model
llm_in = Topic(name="text_in_llm", msg_type="String")
llm_out = Topic(name="text_out_llm", msg_type="String")

llm = LLM(
    inputs=[llm_in],
    outputs=[llm_out],
    model_client=llama_client,
    trigger=llm_in,
    component_name="generic_llm",
)

# Make a Go-to-X component using the same Llama3_2 model
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

config = LLMConfig(enable_rag=True,
                   collection_name="map",
                   distance_func="l2",
                   n_results=1,
                   add_metadata=True)

goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=llama_client,
    db_client=chroma_client,
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
    json_string = output[output.find("{"):output.find("}") + 1]

    # load the string as a json and extract position coordinates
    # if there is an error, return None, i.e. no output would be published to goal_point
    try:
        json_dict = json.loads(json_string)
        return np.array(json_dict['position'])
    except Exception:
        return

# add the pre-processing function to the goal_point output topic
goto.add_publisher_preprocessor(goal_point, llm_answer_to_goal_point)
```

```{note}
Note that we have reused the same model and its client for both components.
```

```{note}
For a detailed explanation of the code for setting up the Go-to-X component, check the previous [recipe](goto-navigation.md).
```

```{caution}
In the code block above we are using the same DB client that was setup in the [Semantic Map](semantic-map.md) recipe.
```

## Creating the SemanticRouter

The SemanticRouter takes an input _String_ topic and sends whatever is published on that topic to a _Route_. A _Route_ is a thin wrapper around _Topic_ and takes in the name of a topic to publish on and example queries, that would match a potential query that should be published to a particular topic. For example, if we ask our robot a general question, like "Whats the capital of France?", we do not want that question to be routed to a Go-to-X component, but to a generic LLM. Thus in its route, we would provide examples of general questions. Lets start by creating our routes for the input topics of the two components above.

```python
from agents.ros import Route

# Create the input topic for the router
query_topic = Topic(name="question", msg_type="String")

# Define a route to a topic that processes go-to-x commands
goto_route = Route(routes_to=goto_in,
    samples=["Go to the door", "Go to the kitchen",
        "Get me a glass", "Fetch a ball", "Go to hallway"])

# Define a route to a topic that is input to an LLM component
llm_route = Route(routes_to=llm_in,
    samples=["What is the capital of France?", "Is there life on Mars?",
        "How many tablespoons in a cup?", "How are you today?", "Whats up?"])
```

```{note}
The `routes_to` parameter of a `Route` can be a `Topic` or an `Action`. `Actions` can be system level functions (e.g. to restart a component), functions exposed by components (e.g. to start the VLA component for manipulation, or the 'say' method in TextToSpeech component) or arbitrary functions written in the recipe. `Actions` are a powerful concept in EMOS, because their arguments can come from any topic in the system. To learn more, check out [Events & Actions](../concepts/events-and-actions.md).
```

## Option 1: Vector Mode (Similarity)

This is the standard approach. In Vector mode, the SemanticRouter component works by storing these examples in a vector DB. Distance is calculated between an incoming query's embedding and the embeddings of example queries to determine which _Route_(_Topic_) the query should be sent on. For the database client we will use the ChromaDB client setup in the [Semantic Map](semantic-map.md) recipe. We will specify a router name in our router config, which will act as a _collection_name_ in the database.

```python
from agents.components import SemanticRouter
from agents.config import SemanticRouterConfig

router_config = SemanticRouterConfig(router_name="go-to-router", distance_func="l2")
# Initialize the router component
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    default_route=llm_route,  # If none of the routes fall within a distance threshold
    config=router_config,
    db_client=chroma_client,  # Providing db_client enables Vector Mode
    component_name="router"
)
```

## Option 2: LLM Mode (Agentic)

Alternatively, we can use an LLM to make routing decisions. This is useful if your routes require "understanding" rather than just similarity. We simply provide a `model_client` instead of a `db_client`.

```{note}
We can even use the same LLM (`model_client`) as we are using for our other Q&A components.
```

```python
# No SemanticRouterConfig needed, we can use LLMConfig or let it be default
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    model_client=llama_client, # Providing model_client enables LLM Mode
    component_name="smart_router"
)

```

And that is it. Whenever something is published on the input topic **question**, it will be routed, either to a Go-to-X component or an LLM component. We can now expose this topic to our command interface. The complete code for setting up the router is given below:

```{code-block} python
:caption: Semantic Routing
:linenos:
from typing import Optional
import json
import numpy as np
from agents.components import LLM, SemanticRouter
from agents.models import OllamaModel
from agents.vectordbs import ChromaDB
from agents.config import LLMConfig, SemanticRouterConfig
from agents.clients import ChromaClient, OllamaClient
from agents.ros import Launcher, Topic, Route

# Start a Llama3.2 based llm component using ollama client
llama = OllamaModel(name="llama", checkpoint="llama3.2:3b")
llama_client = OllamaClient(llama)

# Initialize a vector DB that will store our routes
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)


# Make a generic LLM component using the Llama3_2 model
llm_in = Topic(name="text_in_llm", msg_type="String")
llm_out = Topic(name="text_out_llm", msg_type="String")

llm = LLM(
    inputs=[llm_in],
    outputs=[llm_out],
    model_client=llama_client,
    trigger=llm_in,
    component_name="generic_llm",
)


# Define LLM input and output topics including goal_point topic of type PoseStamped
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

config = LLMConfig(
    enable_rag=True,
    collection_name="map",
    distance_func="l2",
    n_results=1,
    add_metadata=True,
)

# initialize the component
goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=llama_client,
    db_client=chroma_client,  # check the previous example where we setup this database client
    trigger=goto_in,
    config=config,
    component_name="go_to_x",
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
    json_string = output[output.find("{") : output.find("}") + 1]

    # load the string as a json and extract position coordinates
    # if there is an error, return None, i.e. no output would be published to goal_point
    try:
        json_dict = json.loads(json_string)
        return np.array(json_dict["position"])
    except Exception:
        return


# add the pre-processing function to the goal_point output topic
goto.add_publisher_preprocessor(goal_point, llm_answer_to_goal_point)

# Create the input topic for the router
query_topic = Topic(name="question", msg_type="String")

# Define a route to a topic that processes go-to-x commands
goto_route = Route(
    routes_to=goto_in,
    samples=[
        "Go to the door",
        "Go to the kitchen",
        "Get me a glass",
        "Fetch a ball",
        "Go to hallway",
    ],
)

# Define a route to a topic that is input to an LLM component
llm_route = Route(
    routes_to=llm_in,
    samples=[
        "What is the capital of France?",
        "Is there life on Mars?",
        "How many tablespoons in a cup?",
        "How are you today?",
        "Whats up?",
    ],
)

# --- MODE 1: VECTOR ROUTING (Active) ---
router_config = SemanticRouterConfig(router_name="go-to-router", distance_func="l2")

router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    default_route=llm_route,
    config=router_config,
    db_client=chroma_client, # Vector mode requires db_client
    component_name="router",
)

# --- MODE 2: LLM ROUTING (Commented Out) ---
# To use LLM routing (Agentic), comment out the block above and uncomment this:
#
# router = SemanticRouter(
#     inputs=[query_topic],
#     routes=[llm_route, goto_route],
#     default_route=llm_route,
#     model_client=llama_client, # LLM mode requires model_client
#     component_name="router",
# )

# Launch the components
launcher = Launcher()
launcher.add_pkg(components=[llm, goto, router])
launcher.bringup()
```
