# Semantic Routing

The SemanticRouter component in EMOS allows you to route text queries to specific [components](../../intelligence/ai-components.md) based on the user's intent or the output of a preceding component.

The router has three ways of deciding where a query goes:

1. **LLM Mode (Agentic):** An LLM reads the intent of the query and picks the route. It handles nuance, context and negation (*"Don't go to the kitchen"* is not a request to go there), at the cost of a generation per query.

2. **Decision Mode:** A decision model reads the intent the same way, but answers in one forward pass with a confidence for its choice instead of generating text. It is fast, and the confidence can be used as a threshold.

3. **Vector Mode:** A vector DB measures the similarity between the query's embedding and the samples of each route. No model is in the loop, so it is the lightest of the three, and it matches on similarity alone.

The mode follows from what the router is given: a `model_client` with an LLM, a `model_client` with a decision model, or a `db_client`.

In this recipe, we will route queries between two components: a General Purpose LLM (for chatting) and a Go-to-X Component (for navigation commands) that we built in the previous [recipe](goto-navigation.md). The Go-to-X component resolves a place name by calling the `Memory` component's `locate` tool, so we set up `Vision` and `Memory` here too. Lets start by setting up our components.

## Setting up the components

In the following code snippet we will set up the perception side (`Vision` + `Memory`, which builds the spatial map) and our two query components.

```python
import re
from typing import Optional
import numpy as np

from agents.components import LLM, Memory, Vision
from agents.models import OllamaModel, VisionModel
from agents.config import LLMConfig, MemoryConfig, VisionConfig
from agents.clients import OllamaClient, RoboMLRESPClient
from agents.ros import Launcher, Topic, Route, MemLayer

# Reuse one (tool-capable) model for the generic LLM and the Go-to-X LLM
qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

# Embeddings for Memory (the spatial map)
embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)


# -- Perception: vision + memory build the map --
image0 = Topic(name="image_raw", msg_type="Image")
detections_topic = Topic(name="detections", msg_type="Detections")
position = Topic(name="odom", msg_type="Odometry")

vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=VisionConfig(threshold=0.5),
    model_client=RoboMLRESPClient(
        VisionModel(name="rtdetr", checkpoint="PekingU/rtdetr_r50vd_coco_o365")
    ),
    component_name="vision",
)

memory = Memory(
    layers=[MemLayer(subscribes_to=detections_topic)],
    position=position,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/go_to_x.db"),
    trigger=10.0,
    component_name="memory",
)


# Make a generic LLM component for general questions
llm_in = Topic(name="text_in_llm", msg_type="String")
llm_out = Topic(name="text_out_llm", msg_type="String")

llm = LLM(
    inputs=[llm_in],
    outputs=[llm_out],
    model_client=qwen_client,
    trigger=llm_in,
    component_name="generic_llm",
)

# Make a Go-to-X component — it looks places up via Memory's `locate` tool
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=qwen_client,
    trigger=goto_in,
    config=LLMConfig(),
    component_name="go_to_x",
)

goto.set_component_prompt(
    template=(
        "The user asks you to go to a place. Use the available tools to "
        "look up the place's location in memory. Pass the place name to "
        "the locate tool as the ``concept`` argument. User said: {{goto_in}}"
    )
)

# Register Memory's `locate` tool on the Go-to-X LLM so it can be called
memory.register_tools_on(goto, tools=["locate"], send_tool_response_to_model=False)


# pre-process the output before publishing to a topic of msg_type PoseStamped
_LOCATION_RE = re.compile(r"Location:\s*\(([^)]+)\)")


def locate_text_to_goal_point(output: str) -> Optional[np.ndarray]:
    """Pull the centroid coordinates out of Memory.locate's text output."""
    match = _LOCATION_RE.search(output)
    if not match:
        return
    try:
        coords = np.fromstring(match.group(1), sep=",", dtype=np.float64)
    except ValueError:
        return
    if coords.shape[0] == 2:
        coords = np.append(coords, 0.0)
    if coords.shape[0] != 3:
        return
    return coords


# add the pre-processing function to the goal_point output topic
goto.add_publisher_preprocessor(goal_point, locate_text_to_goal_point)
```

```{note}
We reused the same model and its client for both query components. The model must support tool calling, since the Go-to-X component calls Memory's `locate` tool.
```

```{note}
For a detailed explanation of the Go-to-X component — how `Memory` builds the map and exposes the `locate` tool — check the previous [recipe](goto-navigation.md).
```

```{important}
The Go-to-X LLM calls `Memory` **in-process**, so the router, the LLMs, and Memory must launch in the same process (no `multiprocessing=True` on `add_pkg`).
```

## Creating the SemanticRouter

The SemanticRouter takes an input _String_ topic and sends whatever is published on that topic to a _Route_. A _Route_ is a thin wrapper around _Topic_ and takes in the name of a topic to publish on and example queries, that would match a potential query that should be published to a particular topic. For example, if we ask our robot a general question, like "Whats the capital of France?", we do not want that question to be routed to a Go-to-X component, but to a generic LLM. Thus in its route, we would provide examples of general questions. The samples serve every mode: an LLM or a decision model reads them as the description of the route, and vector mode embeds them. Lets start by creating our routes for the input topics of the two components above.

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
The `routes_to` parameter of a `Route` can be a `Topic` or an `Action`. `Actions` can be system level functions (e.g. to restart a component), functions exposed by components (e.g. to start the VLA component for manipulation, or the 'say' method in TextToSpeech component) or arbitrary functions written in the recipe. `Actions` are a powerful concept in EMOS, because their arguments can come from any topic in the system. To learn more, check out [Events & Actions](../../concepts/events-and-actions.md).
```

## Option 1: LLM Mode (Agentic)

Give the router a `model_client` with an LLM and each route becomes a tool the model can call, described by the route's samples. The model reads the query and calls the route that fits, or none, in which case the input goes to `default_route`.

```{note}
We can use the same LLM (`model_client`) as we are using for our other Q&A components.
```

```python
from agents.components import SemanticRouter

router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    default_route=llm_route,  # Used when the model picks no route
    model_client=qwen_client,  # A model client with an LLM enables LLM Mode
    component_name="router",
)
```

The LLM mode also runs on the built-in local model: pass `config=LLMConfig(enable_local_model=True)` and leave `model_client` out, and the router deploys the local model when it configures.

## Option 2: Decision Mode

A decision model answers typed questions about a text in one forward pass, with a probability for every option, and generates no text. The router turns its routes into one such question, each route described by its samples, and the answer names a route and comes with a confidence. The route is used when the confidence is above `minimum_confidence` set in the config. Otherwise the input goes to `default_route`. Open decision models are served by llama.cpp and can be reached through the `GenericHTTPClient` client with a `GenericDecisionModel`; [Decision Models](decision-models.md) covers what these models are and how to serve one.

```python
from agents.clients import GenericHTTPClient
from agents.config import SemanticRouterConfig
from agents.models import GenericDecisionModel

# Served with: llama-server -m lev-Q8_0.gguf --alias lev --port 8090
decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)

router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    default_route=llm_route,  # Used when the confidence of the choice is below minimum_confidence
    config=SemanticRouterConfig(router_name="go-to-router", minimum_confidence=0.3),
    model_client=decision_client,  # A model client with a decision model enables Decision Mode
    component_name="router",
)
```

## Option 3: Vector Mode (Similarity)

In Vector mode, the router stores the route samples in a vector DB and measures the distance between an incoming query's embedding and theirs. The closest sample's route wins, and the input goes to `default_route` when none is within `maximum_distance`. The router needs a `db_client`, and `router_name` in the config names the collection the samples are stored in; `distance_func` and `maximum_distance` only apply in this mode.

```python
from agents.clients import ChromaClient
from agents.config import SemanticRouterConfig
from agents.vectordbs import ChromaDB

# Vector DB for the router -- it stores the route samples
chroma_client = ChromaClient(db=ChromaDB())

router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    default_route=llm_route,  # Used when no route is within the distance threshold
    config=SemanticRouterConfig(router_name="go-to-router", distance_func="l2"),
    db_client=chroma_client,  # A db client enables Vector Mode
    component_name="router",
)
```

And that is it. Whenever something is published on the input topic **question**, it will be routed, either to a Go-to-X component or an LLM component. We can now expose this topic to our command interface. The complete code for setting up the router is given below:

```{code-block} python
:caption: Semantic Routing
:linenos:
import re
from typing import Optional
import numpy as np

from agents.components import LLM, Memory, SemanticRouter, Vision
from agents.models import OllamaModel, VisionModel
from agents.config import LLMConfig, MemoryConfig, SemanticRouterConfig, VisionConfig
from agents.clients import OllamaClient, RoboMLRESPClient
from agents.ros import Launcher, Topic, Route, MemLayer

# Reuse one (tool-capable) model for the generic LLM and the Go-to-X LLM
qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

# Embeddings for Memory (the spatial map)
embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)


# -- Perception: vision + memory build the map --
image0 = Topic(name="image_raw", msg_type="Image")
detections_topic = Topic(name="detections", msg_type="Detections")
position = Topic(name="odom", msg_type="Odometry")

vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=VisionConfig(threshold=0.5),
    model_client=RoboMLRESPClient(
        VisionModel(name="rtdetr", checkpoint="PekingU/rtdetr_r50vd_coco_o365")
    ),
    component_name="vision",
)

memory = Memory(
    layers=[MemLayer(subscribes_to=detections_topic)],
    position=position,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/go_to_x.db"),
    trigger=10.0,
    component_name="memory",
)


# Make a generic LLM component for general questions
llm_in = Topic(name="text_in_llm", msg_type="String")
llm_out = Topic(name="text_out_llm", msg_type="String")

llm = LLM(
    inputs=[llm_in],
    outputs=[llm_out],
    model_client=qwen_client,
    trigger=llm_in,
    component_name="generic_llm",
)


# Make a Go-to-X component — it looks places up via Memory's `locate` tool
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=qwen_client,
    trigger=goto_in,
    config=LLMConfig(),
    component_name="go_to_x",
)

goto.set_component_prompt(
    template=(
        "The user asks you to go to a place. Use the available tools to "
        "look up the place's location in memory. Pass the place name to "
        "the locate tool as the ``concept`` argument. User said: {{goto_in}}"
    )
)

# Register Memory's `locate` tool on the Go-to-X LLM so it can be called
memory.register_tools_on(goto, tools=["locate"], send_tool_response_to_model=False)


# pre-process the output before publishing to a topic of msg_type PoseStamped
_LOCATION_RE = re.compile(r"Location:\s*\(([^)]+)\)")


def locate_text_to_goal_point(output: str) -> Optional[np.ndarray]:
    """Pull the centroid coordinates out of Memory.locate's text output."""
    match = _LOCATION_RE.search(output)
    if not match:
        return
    try:
        coords = np.fromstring(match.group(1), sep=",", dtype=np.float64)
    except ValueError:
        return
    if coords.shape[0] == 2:
        coords = np.append(coords, 0.0)
    if coords.shape[0] != 3:
        return
    return coords


# add the pre-processing function to the goal_point output topic
goto.add_publisher_preprocessor(goal_point, locate_text_to_goal_point)

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

# --- MODE 1: LLM ROUTING (Active) ---
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    default_route=llm_route,  # Used when the model picks no route
    model_client=qwen_client,  # LLM mode requires a model client with an LLM
    component_name="router",
)

# --- MODE 2: DECISION ROUTING (Commented Out) ---
# To route with a decision model, comment out the block above and uncomment this.
# The model is served by llama.cpp, e.g.: llama-server -m lev-Q8_0.gguf --alias lev --port 8090
#
# from agents.clients import GenericHTTPClient
# from agents.models import GenericDecisionModel
#
# decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)
# router = SemanticRouter(
#     inputs=[query_topic],
#     routes=[llm_route, goto_route],
#     default_route=llm_route,  # Used when the confidence of the choice is below minimum_confidence
#     config=SemanticRouterConfig(router_name="go-to-router", minimum_confidence=0.3),
#     model_client=decision_client,  # Decision mode requires a model client with a decision model
#     component_name="router",
# )

# --- MODE 3: VECTOR ROUTING (Commented Out) ---
# To route by similarity to the samples, comment out the active block and uncomment this:
#
# from agents.clients import ChromaClient
# from agents.vectordbs import ChromaDB
#
# chroma_client = ChromaClient(db=ChromaDB())  # stores the route samples
# router = SemanticRouter(
#     inputs=[query_topic],
#     routes=[llm_route, goto_route],
#     default_route=llm_route,  # Used when no route is within the distance threshold
#     config=SemanticRouterConfig(router_name="go-to-router", distance_func="l2"),
#     db_client=chroma_client,  # Vector mode requires a db client
#     component_name="router",
# )

# Launch the components — single process so the Go-to-X LLM can call Memory in-process
launcher = Launcher()
launcher.add_pkg(components=[vision, memory, llm, goto, router])
launcher.bringup()
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
