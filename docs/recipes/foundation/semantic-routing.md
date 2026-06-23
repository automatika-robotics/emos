# Semantic Routing

The SemanticRouter component in EMOS allows you to route text queries to specific [components](../../intelligence/ai-components.md) based on the user's intent or the output of a preceding component.

The router operates in two distinct modes:

1. **Vector Mode (Default):** This mode uses a Vector DB to calculate the mathematical similarity (distance) between the incoming query and the samples defined in your routes. It is extremely fast and lightweight.

2. **LLM Mode (Agentic):** This mode uses an LLM to intelligently analyze the intent of the query and triggers routes accordingly. This is more computationally expensive but can handle complex nuances, context, and negation (e.g., "Don't go to the kitchen" might be routed differently by an agent than a simple vector similarity search).

In this recipe, we will route queries between two components: a General Purpose LLM (for chatting) and a Go-to-X Component (for navigation commands) that we built in the previous [recipe](goto-navigation.md). The Go-to-X component resolves a place name by calling the `Memory` component's `locate` tool, so we set up `Vision` and `Memory` here too. Lets start by setting up our components.

## Setting up the components

In the following code snippet we will set up the perception side (`Vision` + `Memory`, which builds the spatial map) and our two query components.

```python
import re
from typing import Optional
import numpy as np

from agents.components import LLM, Memory, Vision
from agents.models import OllamaModel, VisionModel
from agents.vectordbs import ChromaDB
from agents.config import LLMConfig, MemoryConfig, VisionConfig
from agents.clients import ChromaClient, OllamaClient, RoboMLRESPClient
from agents.ros import Launcher, Topic, Route, MemLayer

# Reuse one (tool-capable) model for the generic LLM and the Go-to-X LLM
qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

# Embeddings for Memory (the spatial map)
embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)

# Vector DB for the SemanticRouter — it stores the route samples
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)


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
The `routes_to` parameter of a `Route` can be a `Topic` or an `Action`. `Actions` can be system level functions (e.g. to restart a component), functions exposed by components (e.g. to start the VLA component for manipulation, or the 'say' method in TextToSpeech component) or arbitrary functions written in the recipe. `Actions` are a powerful concept in EMOS, because their arguments can come from any topic in the system. To learn more, check out [Events & Actions](../../concepts/events-and-actions.md).
```

## Option 1: Vector Mode (Similarity)

This is the standard approach. In Vector mode, the SemanticRouter component stores the route samples in a vector DB. Distance is calculated between an incoming query's embedding and the embeddings of the example queries to determine which _Route_(_Topic_) the query should be sent on. We pass the `chroma_client` set up above as the `db_client`, and specify a router name in the config, which acts as a _collection_name_ in the database.

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

Alternatively, we can use an LLM to make routing decisions. This is useful if your routes require "understanding" rather than just similarity. We simply provide a `model_client` instead of a `db_client` (no ChromaDB needed in this mode).

```{note}
We can even use the same LLM (`model_client`) as we are using for our other Q&A components.
```

```python
# No SemanticRouterConfig needed, we can use LLMConfig or let it be default
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route],
    model_client=qwen_client, # Providing model_client enables LLM Mode
    component_name="smart_router"
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
from agents.vectordbs import ChromaDB
from agents.config import LLMConfig, MemoryConfig, SemanticRouterConfig, VisionConfig
from agents.clients import ChromaClient, OllamaClient, RoboMLRESPClient
from agents.ros import Launcher, Topic, Route, MemLayer

# Reuse one (tool-capable) model for the generic LLM and the Go-to-X LLM
qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

# Embeddings for Memory (the spatial map)
embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)

# Vector DB for the SemanticRouter — it stores the route samples
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)


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
#     model_client=qwen_client, # LLM mode requires model_client
#     component_name="router",
# )

# Launch the components — single process so the Go-to-X LLM can call Memory in-process
launcher = Launcher()
launcher.add_pkg(components=[vision, memory, llm, goto, router])
launcher.bringup()
```

---

```{tip}
**Promote this recipe to production.** While you're shaping it, the script runs straight with `python recipe.py`. Once it's solid, drop it at `~/emos/recipes/<your_name>/recipe.py` and run `emos run <your_name>` -- you'll get sensor pre-flight checks, persistent logs, and a card on the dashboard so an operator can launch it from a browser. See [Running Recipes](../../getting-started/running-recipes.md) for the full development-vs-production comparison and install-mode pitfalls (especially in Container mode).
```
