# Complete Agent

This is the capstone recipe. Everything we have built in the previous tutorials -- conversational interfaces, prompt engineering, spatio-temporal memory, memory-aware navigation, and semantic routing -- comes together here into a single EMOS recipe: a fully capable embodied agent defined in one Python script.

This is what EMOS is designed for. Instead of stitching together dozens of ROS nodes, launch files, and custom middleware, you define a complete agentic workflow as a graph of [Components](../../intelligence/ai-components.md) connected through [Topics](../../concepts/topics.md), and bring it up with a single call. The result is a robot that can listen, see, think, remember, navigate, and speak -- all orchestrated by EMOS.

```{seealso}
For the multiprocessing-and-fault-tolerant variant of this recipe, see [Multiprocessing & Fault Tolerance](../events-and-resilience/multiprocessing.md). For the agentic-harness variant where a single [Cortex](../../intelligence/cortex.md) component takes charge of an entire graph like this, see [Memory and Cortex](../planning-and-manipulation/cortex-memory.md) and [Embodied Reasoning with Cortex](../planning-and-manipulation/cortex-navigation.md).
```

```{admonition} Prerequisites
:class: important

This recipe uses the `Memory` component for spatio-temporal memory. Memory needs the [eMEM](https://github.com/automatika-robotics/emem) package: `pip install emem`. Audio playback also needs `pip install soundfile sounddevice`.
```

## The Graph

```{mermaid}
flowchart LR
    %% --- External I/O ---
    query([query])
    Kompass([Kompass])

    %% --- Speech I/O ---
    speech_to_text[speech_to_text]:::component
    text_to_speech[text_to_speech]:::component
    Whisper[Whisper]
    TransformersTTS[TransformersTTS]

    %% --- Model / DB backends ---
    ChromaDB[ChromaDB]

    %% --- Routing ---
    router[router]:::component

    %% --- Vision ---
    object_detection[object_detection]:::component
    RT_DETR[RT-DETR]

    %% --- VLM (VQA + introspection) ---
    visual_q_and_a[visual_q_and_a]:::component
    introspector[introspector]:::component
    qwen_vl[qwen_vl Ollama]

    %% --- LLM brains ---
    general_q_and_a[general_q_and_a]:::component
    go_to_x[go_to_x]:::component
    qwen[qwen Ollama]

    %% --- Memory ---
    memory[memory]:::component
    embeddings[embeddings Ollama]

    %% --- Wiring: input → router → routes ---
    query --> speech_to_text
    Whisper <--> speech_to_text
    speech_to_text --> router
    router <--> ChromaDB
    router --> visual_q_and_a
    router --> general_q_and_a
    router --> go_to_x

    %% --- Vision feeds VQA + memory ---
    RT_DETR <--> object_detection
    object_detection --> visual_q_and_a
    object_detection --> memory

    %% --- VLM clients ---
    qwen_vl <--> visual_q_and_a
    qwen_vl <--> introspector
    introspector --> memory

    %% --- LLM clients ---
    qwen <--> general_q_and_a
    qwen <--> go_to_x
    qwen <--> memory

    %% --- Memory's own backbone ---
    embeddings <--> memory
    memory --> go_to_x

    %% --- Outputs back to speech / navigation ---
    visual_q_and_a --> text_to_speech
    general_q_and_a --> text_to_speech
    TransformersTTS <--> text_to_speech
    go_to_x --> Kompass

    classDef component fill:#e07a7a,stroke:#a64545,stroke-width:1.5px,color:#000000
```

Rectangular boxes are EMOS components, their model backends, and the embedded ChromaDB the router uses for route-embedding lookups. The rounded nodes are the things outside this recipe -- the user's input on one side and the Kompass navigation stack on the other.

## The Complete Recipe

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
from agents.ros import FixedInput, Launcher, MemLayer, Route, Topic
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
    trigger=15.0,
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
        MemLayer(subscribes_to=detections_topic, temporal_change=True),
        MemLayer(subscribes_to=introspection_answer),
    ],
    position=position,
    model_client=qwen_client,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/complete_agent.db"),
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
        "the locate tool as the ``concept`` argument. "
        "The user said: {{goto_query}}"
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
        "How many things are infront of you?",
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


### Launch (single process so goto can call memory in-process) ###
launcher = Launcher()
launcher.add_pkg(
    components=[
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
)
launcher.bringup()
```

```{note}
The same `qwen_client` (Ollama, `qwen3:0.6b`) drives general Q&A, the goto tool-caller, and Memory's episodic-consolidation summaries. The VLM (`qwen_vl_client`) is shared between the VQA path and the introspector.
```

## What We Have Built

In this single recipe, we have assembled a fully capable embodied agent with the following capabilities:

- {material-regular}`record_voice_over;1.2em;sd-text-primary` **A conversational interface** using speech-to-text and text-to-speech models that uses the robot's microphone and playback speaker. (See: [Conversational Agent](conversational-agent.md))
- {material-regular}`visibility;1.2em;sd-text-primary` **Contextual visual question answering** based on the robot's camera, using a multimodal LLM enriched with object detection output. (See: [Prompt Engineering](prompt-engineering.md))
- {material-regular}`chat;1.2em;sd-text-primary` **General knowledge Q&A** using a text-only LLM for non-visual queries.
- {material-regular}`memory;1.2em;sd-text-primary` **A graph-backed spatio-temporal memory** that acts as the robot's long-term memory, continuously updated with object detections and room-type introspection, indexed simultaneously by meaning, location, and time. Built on [eMEM](https://github.com/automatika-robotics/emem). (See: [Spatio-Temporal Memory](semantic-map.md))
- {material-regular}`route;1.2em;sd-text-primary` **Memory-aware Go-to-X navigation** -- a tool-calling LLM that asks Memory to `locate` a place and publishes the result as a goal point. (See: [GoTo Navigation](goto-navigation.md), [Tool Calling](tool-calling.md))
- {material-regular}`alt_route;1.2em;sd-text-primary` **Intent-based semantic routing** through a single input interface that directs queries to the correct component based on content. (See: [Semantic Routing](semantic-routing.md))

This is the EMOS developer experience: a sophisticated, multi-capability embodied agent defined entirely in a single Python script. Every component -- perception, reasoning, memory, navigation, and speech -- is wired together through Topics and launched with one call to `bringup()`. The same recipe runs on any robot that EMOS supports, from wheeled AMRs to quadrupeds, without modification.

To run this same graph in **multi-process mode with fault tolerance**, see [Multiprocessing & Fault Tolerance](../events-and-resilience/multiprocessing.md). For runtime resilience -- fallback logic, recovery maneuvers, algorithm switching -- see the [Events & Actions](../../concepts/events-and-actions.md) documentation.
