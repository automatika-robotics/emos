# Spatio-Temporal Memory

Autonomous Mobile Robots (AMRs) keep a representation of their environment in the form of occupancy maps. Such maps are fine for navigation but are *amnesic*: a robot doesn't *know* the objects, the rooms, the situations, or its own history. The [Memory](../../intelligence/memory.md) component gives an EMOS agent a structured place to store everything it perceives, indexed by **meaning**, **location**, and **time** — and to consolidate that stream of observations into long-term memory the way humans do.

In this recipe we wire perception into Memory: an object detector publishing `Detections`, a VLM publishing periodic introspective answers, both feeding `Memory` as separate layers. **Memory** runs on [eMEM](https://github.com/automatika-robotics/emem), a hybrid graph-based spatio-temporal memory built on neuroscience principles: tiered consolidation, episodic structure, entity persistence, and interoception as a first-class memory dimension.

```{seealso}
For the conceptual background on Memory and the neuroscience principles behind it, see the [Memory page](../../intelligence/memory.md). Once you've built a memory and want to *reason over it* in plain English, the [Memory and Cortex](../planning-and-manipulation/cortex-memory.md) recipe shows how Cortex auto-discovers Memory's tools and uses them in its orchestration.
```

```{admonition} Prerequisites
:class: important

The `Memory` component requires the [eMEM](https://github.com/automatika-robotics/emem) Python package, which `emos install` does not add. The install command depends on your mode (Pixi: `pixi add --pypi emem`; Native: `pip install emem`; Container: install inside the container) — see [Memory installation](../../intelligence/memory.md).
```

---

## Setting up a Vision Component

```python
from agents.components import Vision
from agents.config import VisionConfig
from agents.ros import Topic

# Define the image input topic
image0 = Topic(name="image_raw", msg_type="Image")
# Create a detection topic
detections_topic = Topic(name="detections", msg_type="Detections")
```

Additionally the component requires a model client with an object detection model. We will use the RESP client for [RoboML](https://github.com/automatika-robotics/roboml) and the `VisionModel` wrapper, which initialises any [HuggingFace Transformers object detection model](https://huggingface.co/models?pipeline_tag=object-detection) (RT-DETR, DETR, Grounding DINO, YOLOS, ...) by checkpoint name.

```{note}
Learn about setting up RoboML with vision [here](https://github.com/automatika-robotics/roboml/blob/main/README.md#vision-model-support).
```

```python
from agents.models import VisionModel
from agents.clients import RoboMLRESPClient

# Add an object detection model
object_detection = VisionModel(
    name="object_detection",
    checkpoint="PekingU/rtdetr_r50vd_coco_o365",
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
```

The vision component will provide us with semantic information to add to memory. However, object names are only the most basic semantic element of the scene. One can view such basic elements in aggregate to create more abstract semantic associations. This is where multimodal LLMs come in.

---

## Setting up a VLM Component

With multimodal LLMs we can ask higher-level introspective questions about what the robot is currently seeing and store the answers in memory alongside the raw detections. We'll set up a VLM component that periodically asks itself the same question — what kind of room am I in? — using two EMOS concepts: a `FixedInput` (a simulated [Topic](../../concepts/topics.md) whose value is a constant string) and a *timed* component (one whose `trigger` is a frequency rather than an input topic).

```python
from agents.components import VLM
from agents.clients import OllamaClient
from agents.models import OllamaModel
from agents.ros import FixedInput

# Define a model client (working with Ollama in this case)
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)

# Define a fixed input for the component
introspection_query = FixedInput(
    name="introspection_query",
    msg_type="String",
    fixed=(
        "What kind of a room is this? Is it an office, a bedroom or a kitchen? "
        "Give a one word answer, out of the given choices"
    ),
)
# Define output of the component
introspection_answer = Topic(name="introspection_answer", msg_type="String")

# Start a timed (periodic) component using the mllm model defined earlier
# This component answers the same question every 15 seconds
introspector = VLM(
    inputs=[introspection_query, image0],   # we use image0 from earlier
    outputs=[introspection_answer],
    model_client=qwen_client,
    trigger=15.0,                           # frequency in seconds
    component_name="introspector",
)
```

LLM/VLM model outputs can be unpredictable. Before publishing the answer of our question to the output topic, we want to ensure that the model has indeed provided a one word answer, and that this answer is one of the expected choices. EMOS allows arbitrary pre-processor functions on data being published; we'll add a tiny validator that drops anything outside the expected vocabulary:

```python
from typing import Optional

def introspection_validation(output: str) -> Optional[str]:
    for option in ["office", "bedroom", "kitchen"]:
        if option in output.lower():
            return option

introspector.add_publisher_preprocessor(introspection_answer, introspection_validation)
```

Now `introspection_answer` only carries clean one-word labels.

---

## Building Memory

The final step is to wire those two streams into a `Memory` component. Memory's input surface is a list of `MemLayer`s — each layer subscribes to a topic, and observations from that layer are tagged with a layer name in the underlying graph so you can later query *only* perception, *only* internal state, etc.

```python
from agents.ros import MemLayer

# Object detection output from vision component
layer1 = MemLayer(subscribes_to=detections_topic)
# Introspection output from mllm component
layer2 = MemLayer(subscribes_to=introspection_answer)
```

```{tip}
Memory also models **interoception** — internal body state — as a first-class memory dimension. Adding a layer with `is_internal_state=True` (e.g. `MemLayer(subscribes_to=battery_topic, is_internal_state=True)`) routes those observations through `add_body_state` instead of `add`. They're queryable through the dedicated `body_status` tool and surface naturally alongside perception observations in `get_current_context`. We'll exercise this in the [Memory and Cortex](../planning-and-manipulation/cortex-memory.md) recipe.
```

Memory needs the robot's pose (so every observation is tagged with where it was made) and two model clients — one for consolidation summarisation, one for embedding generation:

```python
from agents.components import Memory
from agents.config import MemoryConfig

# Localization input — Memory uses these coordinates directly, no occupancy grid required
position = Topic(name="odom", msg_type="Odometry")

# Embedding client for vector indexing of every observation
embedding_model = OllamaModel(
    name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest"
)
embedding_client = OllamaClient(embedding_model)

memory = Memory(
    layers=[layer1, layer2],
    position=position,
    model_client=qwen_client,        # used to summarise episodes into gists
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/robot_memory.db"),
    trigger=15.0,                    # flush layer data into memory every 15s
    component_name="memory",
)
```

That single `Memory` component maintains:

- A **typed graph** with four node types (Observation, Episode, Gist, Entity) and six edge types -- so the agent's memory is a structured object, not a flat blob of vectors.
- **Tiered storage**, working &rarr; short-term &rarr; long-term &rarr; archived. Observations move through the tiers automatically as time passes; raw text is dropped after archival but the consolidated gist remains searchable.
- **Three complementary indexes** sharing the graph: HNSW for semantic search, R-tree for spatial queries, SQLite indexes for temporal queries -- queryable independently or simultaneously.
- **Automatic entity merging**: a new detection of "red chair" near a known "red chair" entity is recognised as the same entity rather than a new one, with cosine similarity *and* spatial proximity controlling the merge.

You don't see any of this in the recipe — you wire layers in, and the structure emerges. See the [Memory page](../../intelligence/memory.md) for the architecture in detail.

---

## Wrapping Tasks in Episodes

The VLM-introspector + detector pair is a good demonstration of layered memory ingestion, but in a real recipe you'd usually want to **bracket** the activity in an *episode*. Episodes are how Memory groups observations into task spans for consolidation: when an episode ends, eMEM clusters the observations made during it, asks the LLM to summarise each cluster into a *gist*, and archives the raw text -- the gist remains fully searchable in long-term memory.

`Memory` exposes `start_episode` and `end_episode` as component actions — the simplest way to call them from a recipe is via [Events & Actions](../../concepts/events-and-actions.md). For example, you might trigger `start_episode` whenever the robot enters a new region and `end_episode` when it leaves. We'll show this pattern fully in [Memory and Cortex](../planning-and-manipulation/cortex-memory.md) where Cortex wraps every action task in an episode automatically.

For now, every observation we feed in lives in working memory, gets flushed to short-term memory on the trigger schedule, and migrates to long-term as time accumulates.

---

## Launching the Components

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.add_pkg(
    components=[vision, introspector, memory],
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
launcher.bringup()
```

That's it. The robot is now accumulating a structured spatio-temporal memory of everything Vision detects and everything the VLM introspects, indexed by where it happened and when.

---

## Full Recipe Code

```{code-block} python
:caption: Spatio-Temporal Memory with Vision and an Introspecting VLM
:linenos:
from typing import Optional

from agents.components import Memory, VLM, Vision
from agents.config import MemoryConfig, VisionConfig
from agents.models import OllamaModel, VisionModel
from agents.clients import OllamaClient, RoboMLRESPClient
from agents.ros import FixedInput, Launcher, MemLayer, Topic


# --- Vision: object detection ---
image0 = Topic(name="image_raw", msg_type="Image")
detections_topic = Topic(name="detections", msg_type="Detections")

object_detection = VisionModel(
    name="object_detection", checkpoint="PekingU/rtdetr_r50vd_coco_o365"
)
roboml_detection = RoboMLRESPClient(object_detection)

vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=VisionConfig(threshold=0.5),
    model_client=roboml_detection,
    component_name="detection_component",
)


# --- VLM: periodic room-type introspection ---
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)

introspection_query = FixedInput(
    name="introspection_query",
    msg_type="String",
    fixed=(
        "What kind of a room is this? Is it an office, a bedroom or a kitchen? "
        "Give a one word answer, out of the given choices"
    ),
)
introspection_answer = Topic(name="introspection_answer", msg_type="String")

introspector = VLM(
    inputs=[introspection_query, image0],
    outputs=[introspection_answer],
    model_client=qwen_client,
    trigger=15.0,
    component_name="introspector",
)


def introspection_validation(output: str) -> Optional[str]:
    for option in ["office", "bedroom", "kitchen"]:
        if option in output.lower():
            return option


introspector.add_publisher_preprocessor(introspection_answer, introspection_validation)


# --- Memory: graph-backed spatio-temporal store ---
embedding_model = OllamaModel(
    name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest"
)
embedding_client = OllamaClient(embedding_model)

position = Topic(name="odom", msg_type="Odometry")

layer1 = MemLayer(subscribes_to=detections_topic)
layer2 = MemLayer(subscribes_to=introspection_answer)

memory = Memory(
    layers=[layer1, layer2],
    position=position,
    model_client=qwen_client,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/robot_memory.db"),
    trigger=15.0,
    component_name="memory",
)


# --- Launch ---
launcher = Launcher()
launcher.add_pkg(
    components=[vision, introspector, memory],
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
launcher.bringup()
```

---

## Where next

- {doc}`Memory and Cortex <../planning-and-manipulation/cortex-memory>` — once you've built a memory, *reason* over it. Cortex auto-discovers all of Memory's retrieval tools and answers questions in plain English: *"where did you last see the cat?"*, *"summarise the last episode"*, *"is the kitchen messy right now?"*.
- {doc}`Cortex: The Agentic Harness <../planning-and-manipulation/cortex-agent>` — the Cortex introduction, if you haven't met it yet.
- {doc}`Memory concept page <../../intelligence/memory>` — the full architectural reference for eMEM: nodes, edges, tiers, consolidation, the ten retrieval tools.

---

```{tip}
**Promote this recipe to production.** While you're shaping it, the script runs straight with `python recipe.py`. Once it's solid, drop it at `~/emos/recipes/<your_name>/recipe.py` and run `emos run <your_name>` -- you'll get sensor pre-flight checks, persistent logs, and a card on the dashboard so an operator can launch it from a browser. See [Running Recipes](../../getting-started/running-recipes.md) for the full development-vs-production comparison and install-mode pitfalls (especially in Container mode).
```

