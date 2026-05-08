# Multiprocessing & Fault Tolerance

In the previous recipes we saw how to compose a complex graph of components into an intelligent embodied agent. In this recipe we look at the features EMOS provides to make the same graph **robust and production-ready** -- running every component in its own process for crash isolation, and adding component-level and process-level recovery so a transient failure doesn't bring the whole system down.

```{admonition} Prerequisites
:class: important

This recipe builds on [Complete Agent](../foundation/complete-agent.md). Memory needs the [eMEM](https://github.com/automatika-robotics/emem) package: `pip install emem`.
```

## Run Components in Separate Processes

By default the launcher runs each component in its own thread. ROS, however, was designed so that each functional unit -- a component in EMOS, mapped to a node in ROS -- runs in a **separate process**, such that failure of one process does not crash the rest of the system. We enable multiprocessing by passing `multiprocessing=True` and the ROS package name to `add_pkg`:

```python
launcher.add_pkg(
    components=all_components,
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
```

## Component-Level Fallbacks

When a component fails -- a model server drops, a sensor goes dead, an algorithm can't find a solution -- EMOS lets us register **fallback strategies** that the component executes automatically. The simplest one is *restart*: re-run the lifecycle so the component checks its inputs and connections again before returning to the active state. We attach this to every component in the recipe with a small loop:

```python
from agents.ros import Action

for component in all_components:
    component.on_fail(
        action=Action(component.restart),
        max_retries=2,
    )
    component.fallback_rate = 1 / 10  # 0.1 Hz -- check for failures every 10s
```

`on_fail` registers the action to take when the component reports an unhealthy state; `fallback_rate` controls how often EMOS retries while the component stays unhealthy.

```{seealso}
EMOS supports much richer fallback behaviour -- escalation ladders, custom handlers, the four-level health-status hierarchy. See [Status & Fallbacks](../../concepts/status-and-fallbacks.md) for the full picture.
```

## Process-Level Crash Recovery

Component-level fallbacks handle the case where a component is *running* but unhealthy. They cannot help if the entire process **crashes** -- a segfault, an OOM kill, an unhandled native exception. For that, EMOS provides `Launcher.on_process_fail`, which respawns any component process that exits with a non-zero status outside of a clean shutdown:

```python
launcher.on_process_fail(max_retries=3)
```

The two layers compose: the in-process fallback tries to restart the component up to 2 times; if those fail and the process actually exits, the launcher respawns the process up to 3 times. See [Process-Level Recovery](../../concepts/status-and-fallbacks.md#process-level-recovery) for the full discussion.

## The Complete Recipe

Putting it all together:

```python
import re
from typing import Optional

import numpy as np

from agents.clients import (
    ChromaClient,
    OllamaClient,
    RoboMLHTTPClient,
    RoboMLRESPClient,
)
from agents.components import (
    LLM,
    VLM,
    Memory,
    SemanticRouter,
    SpeechToText,
    TextToSpeech,
    Vision,
)
from agents.config import (
    LLMConfig,
    MemoryConfig,
    SemanticRouterConfig,
    TextToSpeechConfig,
    VisionConfig,
)
from agents.models import OllamaModel, TransformersTTS, VisionModel, Whisper
from agents.ros import Action, FixedInput, Launcher, MemLayer, Route, Topic
from agents.vectordbs import ChromaDB


### Models and shared clients ###
whisper_client = RoboMLHTTPClient(Whisper(name="whisper"))
tts_client = RoboMLHTTPClient(TransformersTTS(name="tts"))
detection_client = RoboMLRESPClient(
    VisionModel(name="rtdetr", checkpoint="PekingU/rtdetr_r50vd_coco_o365")
)
qwen_vl_client = OllamaClient(
    OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
)
qwen_client = OllamaClient(OllamaModel(name="qwen", checkpoint="qwen3:0.6b"))
embedding_client = OllamaClient(
    OllamaModel(name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest")
)
# ChromaDB is still used by SemanticRouter for route embeddings.
chroma_client = ChromaClient(db=ChromaDB(), port=8080)


### Speech I/O ###
audio_in = Topic(name="audio0", msg_type="Audio")
query_topic = Topic(name="question", msg_type="String")
query_answer = Topic(name="answer", msg_type="String")

speech_to_text = SpeechToText(
    inputs=[audio_in],
    outputs=[query_topic],
    model_client=whisper_client,
    trigger=audio_in,
    component_name="speech_to_text",
)

text_to_speech = TextToSpeech(
    inputs=[query_answer],
    trigger=query_answer,
    model_client=tts_client,
    config=TextToSpeechConfig(play_on_device=True),
    component_name="text_to_speech",
)


### Vision (object detection) ###
image0 = Topic(name="image_raw", msg_type="Image")
detections_topic = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[image0],
    outputs=[detections_topic],
    trigger=image0,
    config=VisionConfig(threshold=0.5),
    model_client=detection_client,
    component_name="object_detection",
)


### VQA VLM ###
mllm_query = Topic(name="mllm_query", msg_type="String")

mllm = VLM(
    inputs=[mllm_query, image0, detections_topic],
    outputs=[query_answer],
    model_client=qwen_vl_client,
    trigger=mllm_query,
    component_name="visual_q_and_a",
)
mllm.set_component_prompt(
    template=(
        "Imagine you are a robot. This image has the following items: "
        "{{ detections }}. Answer the following about this image: {{ text0 }}"
    )
)


### Introspection VLM (room classification feeding the memory) ###
introspection_query = FixedInput(
    name="introspection_query",
    msg_type="String",
    fixed=(
        "What kind of a room is this? Is it an office, a bedroom or a "
        "kitchen? Give a one word answer, out of the given choices"
    ),
)
introspection_answer = Topic(name="introspection_answer", msg_type="String")

introspector = VLM(
    inputs=[introspection_query, image0],
    outputs=[introspection_answer],
    model_client=qwen_vl_client,
    trigger=10.0,
    component_name="introspector",
)


def introspection_validation(output: str) -> Optional[str]:
    for option in ["office", "bedroom", "kitchen"]:
        if option in output.lower():
            return option


introspector.add_publisher_preprocessor(introspection_answer, introspection_validation)


### Memory (graph-backed spatio-temporal memory) ###
position = Topic(name="odom", msg_type="Odometry")

memory = Memory(
    layers=[
        MemLayer(subscribes_to=detections_topic),
        MemLayer(subscribes_to=introspection_answer),
    ],
    position=position,
    model_client=qwen_client,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/complete_agent_multiprocessing.db"),
    trigger=15.0,
    component_name="memory",
)


### Generic LLM (general Q&A) ###
llm_query = Topic(name="llm_query", msg_type="String")

llm = LLM(
    inputs=[llm_query],
    outputs=[query_answer],
    model_client=qwen_client,
    trigger=[llm_query],
    component_name="general_q_and_a",
)


### Go-to-X using LLM tool calling on Memory.locate ###
goto_query = Topic(name="goto_query", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

goto = LLM(
    inputs=[goto_query],
    outputs=[goal_point],
    model_client=qwen_client,
    trigger=goto_query,
    config=LLMConfig(),
    component_name="go_to_x",
)
goto.set_component_prompt(
    template=(
        "The user asks you to go to a place. Use the available tools to "
        "look up the place's location in memory. Pass the place name to "
        "the locate tool as the ``concept`` argument. User asked: {{goto_query}}"
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


### Semantic router (uses ChromaDB for the route embeddings) ###
goto_route = Route(
    routes_to=goto_query,
    samples=[
        "Go to the door",
        "Go to the kitchen",
        "Get me a glass",
        "Fetch a ball",
        "Go to hallway",
    ],
)
llm_route = Route(
    routes_to=llm_query,
    samples=[
        "What is the capital of France?",
        "Is there life on Mars?",
        "How many tablespoons in a cup?",
        "How are you today?",
        "Whats up?",
    ],
)
mllm_route = Route(
    routes_to=mllm_query,
    samples=[
        "Are we indoors or outdoors",
        "What do you see?",
        "Whats in front of you?",
        "Where are we",
        "Do you see any people?",
        "How many things are in front of you?",
        "Is this room occupied?",
    ],
)

router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route, mllm_route],
    default_route=llm_route,
    config=SemanticRouterConfig(router_name="go-to-router", distance_func="l2"),
    db_client=chroma_client,
    component_name="router",
)


### Per-component fallback strategies ###
all_components = [
    mllm,
    llm,
    goto,
    introspector,
    memory,
    router,
    speech_to_text,
    text_to_speech,
    vision,
]
for component in all_components:
    component.on_fail(
        action=Action(component.restart),
        max_retries=2,
    )
    component.fallback_rate = 1 / 10  # 0.1 Hz -- check for failures every 10s


### Launch (multi-process) ###
launcher = Launcher()
launcher.enable_ui(
    inputs=[query_topic, audio_in], outputs=[detections_topic, query_answer, goal_point]
)
launcher.add_pkg(
    components=all_components,
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
# Process-level crash recovery: respawn any multi-process component whose
# process exits unexpectedly, up to ``max_retries`` times.
launcher.on_process_fail(max_retries=3)
launcher.bringup()
```

With these modifications, the same complex agent graph from [Complete Agent](../foundation/complete-agent.md) runs **as nine isolated processes**, each with its own restart policy and its own process-level safety net. A model-server outage triggers a component restart; an unrecoverable process exit triggers a process respawn. The graph as a whole keeps running.
