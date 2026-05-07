# GoTo Navigation

In the previous [recipe](semantic-map.md) we built a graph-backed spatio-temporal memory using the `Memory` component. Memory tags every observation with the robot's pose, so for any concept the agent has encountered (an object class, a room label) we can ask Memory: *where did we see this?* It already exposes that lookup as a component action called `locate`, returning a centroid plus a radius for the most likely region.

In this recipe we wire `locate` into a Go-to-X component so that a command like *"Go to the kitchen"* turns into a `PoseStamped` goal point that the navigation stack can consume. We do this by **registering Memory's `locate` tool on an LLM** -- the LLM decides when to call the tool, Memory answers, and a small preprocessor converts the textual answer into a numpy coordinate that gets published as the goal.

```{seealso}
For the conceptual reference of Memory's full retrieval surface, see the [Memory page](../../intelligence/memory.md). For a generic introduction to LLM tool calling that is not tied to navigation, see the next recipe, [Tool Calling](tool-calling.md).
```

```{admonition} Prerequisites
:class: important

Memory needs the [eMEM](https://github.com/automatika-robotics/emem) package: `pip install emem`.
```

---

## What we're building

Three components in a single launcher:

| Component | Role |
|---|---|
| **Vision** | Object detector publishing `Detections` per frame. Feeds Memory. |
| **Memory** | Graph-backed spatio-temporal memory ingesting detections; tags each observation with the robot's pose from `/odom`. |
| **goto LLM** | A plain `LLM` component that takes a free-form *"go to X"* query, calls Memory's `locate` tool to look up the place, and publishes the resulting coordinates on `goal_point`. |

Memory's `locate` returns a textual answer like `"... Location: (10.3, 9.8, 0.0) ..."` (centroid plus a description). We wire a small regex preprocessor onto the goto LLM's output topic that picks the centroid out and converts it to an `np.ndarray`, which the framework publishes as the `PoseStamped` goal point.

---

## Step 1: Vision and Memory

```python
from agents.clients import OllamaClient, RoboMLRESPClient
from agents.components import Memory, Vision
from agents.config import MemoryConfig, VisionConfig
from agents.models import OllamaModel, VisionModel
from agents.ros import Launcher, MemLayer, Topic

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

embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)

memory = Memory(
    layers=[MemLayer(subscribes_to=detections_topic, temporal_change=True)],
    position=position,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/go_to_x.db"),
    trigger=10.0,
    component_name="memory",
)
```

Each detection becomes an `ObservationNode` in Memory tagged with the robot's pose at the moment the detection was made. After a few minutes of accumulation, eMEM's entity layer auto-merges nearby semantically-similar detections into persistent entities, and `locate("chair")` returns the centroid of the merged "chair" entity.

---

## Step 2: The Go-to-X LLM

```python
from agents.components import LLM
from agents.config import LLMConfig

qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

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
```

---

## Step 3: Register Memory's `locate` tool on the LLM

```python
memory.register_tools_on(goto, tools=["locate"], send_tool_response_to_model=False)
```

`register_tools_on` exposes Memory's component actions to the goto LLM as callable tools. We register a single tool, `locate`, since the LLM only needs the lookup capability for this recipe. The flag `send_tool_response_to_model=False` is what makes the recipe end-to-end: instead of feeding `locate`'s answer back into the LLM for a follow-up generation, the answer becomes the *output of the LLM component*. After preprocessing, that output is what gets published on `goal_point`.

---

## Step 4: Parse Memory's textual answer into coordinates

`locate` returns text formatted like:

```
Concept: kitchen
Location: (10.32, 9.84, 0.0)
Confidence: 0.91
Radius: 1.45m
...
```

We register a small preprocessor on the `goal_point` output topic that pulls the centroid out of that text and converts it to an `np.ndarray`. The framework then publishes the array as a `PoseStamped`.

```python
import re
from typing import Optional

import numpy as np

_LOCATION_RE = re.compile(r"Location:\s*\(([^)]+)\)")


def locate_text_to_goal_point(output: str) -> Optional[np.ndarray]:
    """Pull the centroid coordinates out of Memory.locate's text output."""
    match = _LOCATION_RE.search(output)
    if not match:
        return  # no match → nothing to publish
    try:
        coords = np.fromstring(match.group(1), sep=",", dtype=np.float64)
    except ValueError:
        return
    if coords.shape[0] == 2:
        coords = np.append(coords, 0.0)
    if coords.shape[0] != 3:
        return
    return coords


goto.add_publisher_preprocessor(goal_point, locate_text_to_goal_point)
```

If the LLM (correctly) called `locate`, the regex matches, the coordinates parse cleanly, and the goal point is published. If the LLM hallucinated or the place is unknown to Memory, the preprocessor returns `None` and **nothing is published** -- the navigation stack sees no spurious goal.

---

## Step 5: Launch

```python
launcher = Launcher()
launcher.add_pkg(components=[vision, memory, goto])
launcher.bringup()
```

---

## Full recipe code

```{code-block} python
:caption: Go-to-X with Memory tool calling
:linenos:
import re
from typing import Optional

import numpy as np

from agents.clients import OllamaClient, RoboMLRESPClient
from agents.components import LLM, Memory, Vision
from agents.config import LLMConfig, MemoryConfig, VisionConfig
from agents.models import OllamaModel, VisionModel
from agents.ros import Launcher, MemLayer, Topic


# -- Perception side: vision + memory --
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

embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)

memory = Memory(
    layers=[MemLayer(subscribes_to=detections_topic, temporal_change=True)],
    position=position,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/go_to_x.db"),
    trigger=10.0,
    component_name="memory",
)


# -- Go-to-X LLM --
qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

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

memory.register_tools_on(goto, tools=["locate"], send_tool_response_to_model=False)


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


goto.add_publisher_preprocessor(goal_point, locate_text_to_goal_point)


# -- Launch (single process so the LLM can call Memory in-process) --
launcher = Launcher()
launcher.add_pkg(components=[vision, memory, goto])
launcher.bringup()
```

---

## Where next

- {doc}`Tool Calling <tool-calling>` — generalises this pattern. Instead of registering Memory's pre-defined tools, write your own Python function as a custom tool and register it with `goto.register_tool(...)`.
- {doc}`Complete Agent <complete-agent>` — drops this Go-to-X pattern into a full multi-modal agent (speech I/O + vision + memory + Q&A + routing) defined in one Python script.
- {doc}`Cortex Driving the Full Stack <../planning-and-manipulation/cortex-navigation>` — the agentic-harness version: drop a [Cortex](../../intelligence/cortex.md) component on top of Memory and the navigation stack and the robot handles compound natural-language goals like *"go to the kitchen and tell me what's on the counter"* with no orchestration code from you.
