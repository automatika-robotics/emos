# Memory and Cortex

> _"What kind of place have I been in today?"_
>
> _"Where did I last see a person?"_
>
> _"Are you doing OK? Anything wrong?"_
>
> _"Tell me everything you know about the fridge."_

When [Cortex](../../intelligence/cortex.md) and [Memory](../../intelligence/memory.md) share a recipe, the robot becomes addressable in the same way you'd talk to a person who has _been there all day_. Memory accumulates a structured graph of everything the robot has seen, where it saw it, when, and how it was feeling at the time. Cortex auto-discovers Memory's retrieval surface, augments its planning prompt to handle distinct kinds of question, and orchestrates the right tools for each. None of that wiring is yours to write.

This is the recipe that shows what that combination is capable of -- the deep pairing of **the agentic harness** and **the first neuroscience-grounded memory system for embodied agents**.

```{seealso}
For Memory by itself with no agent on top, start with [Spatio-Temporal Memory](../foundation/semantic-map.md). For Cortex by itself with no memory, start with [Cortex: The Agentic Harness](cortex-agent.md). For the full multi-system showcase that adds navigation on top of this pair, see [Cortex Driving the Full Stack](cortex-navigation.md).
```

```{admonition} Prerequisites
:class: important

The `Memory` component requires the [eMEM](https://github.com/automatika-robotics/emem) Python package, which `emos install` does not add. The install command depends on your mode (Pixi: `pixi add --pypi emem`; Native: `pip install emem`; Container: install inside the container) — see [Memory installation](../../intelligence/memory.md).
```

---

## What we're building

A robot that, over the course of a session, accumulates a hierarchical record of what it has been doing — and that you can query in three distinct registers:

| Register             | Examples                                                                                                                  | What Cortex does                                                                                                                                                                                                                       |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Perception query** | _"Where did you last see the chair?"_, _"What rooms have you been in today?"_, _"Summarise the last 10 minutes"_          | Plans a chain of perception-retrieval tools (`semantic_search`, `temporal_query`, `episode_summary`, `locate`, ...), reads memory's graph, replies in text. **No new observations, no episode wrapping.**                              |
| **Body query**       | _"How are you feeling?"_, _"What's your battery level?"_, _"Anything overheating?"_                                       | Plans a single `body_status` call. Routes through the dedicated interoception surface (Memory's `is_internal_state=True` layers) — perception tools never see body state and vice versa.                                               |
| **Action task**      | _"Take a picture of the fridge"_, _"Take a picture of the table and remember what you saw on it"_, _"Patrol the kitchen"_ | **Begins with `body_status`** (refuse the task if internal state says no). Wraps execution in `start_episode` / `end_episode`. Optionally consults perception memory while planning. Stores derived facts via `store_specific_memory`. |

That whole protocol — three registers, mandatory body checks for action tasks, episodic wrapping — is the **Memory-aware planning** Cortex installs automatically when it detects a Memory component in the recipe. You write zero lines of orchestration to get it.

---

## Step 1: Perception

Same Vision + VLM pair as the [foundation memory recipe](../foundation/semantic-map.md) — detections every second, scene captions every ten:

```python
from agents.clients import OllamaClient
from agents.components import VLM, Vision
from agents.config import VisionConfig
from agents.models import OllamaModel
from agents.ros import FixedInput, Topic

# Vision component (on-device classifier for low-latency detections)
image_in = Topic(name="/image_raw", msg_type="Image")
detections_out = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[image_in],
    outputs=[detections_out],
    config=VisionConfig(threshold=0.5, enable_local_classifier=True),
    trigger=1.0,
    component_name="vision",
)

# VLM scene captioner — periodic introspective description
scene_query = FixedInput(
    name="scene_query",
    msg_type="String",
    fixed=(
        "Describe what you see in one concise sentence: room type, notable "
        "objects, and any people present."
    ),
)
scene_description = Topic(name="scene_description", msg_type="String")

vlm_model = OllamaModel(name="gemma4", checkpoint="gemma4:latest")
vlm_client = OllamaClient(vlm_model)

captioner = VLM(
    inputs=[scene_query, image_in],
    outputs=[scene_description],
    model_client=vlm_client,
    trigger=10.0,
    component_name="captioner",
)
```

These are the **perception layers** Memory will track.

---

## Step 2: Interoception

This is where the Cortex + Memory pairing pulls something the original semantic-map recipe couldn't. We give the robot the ability to feel itself: a battery sensor, a CPU temperature, a joint-health flag — each routed through Memory as an **interoception layer** (`is_internal_state=True`):

```python
battery_topic = Topic(name="/battery_level", msg_type="Float32")
cpu_temp_topic = Topic(name="/cpu_temp", msg_type="Float32")
joint_health_topic = Topic(name="/joint_health", msg_type="String")
```

```{tip}
If you don't have a real battery sensor, fake one from another terminal so you can try the body queries below:
\
`ros2 topic pub /battery_level std_msgs/msg/Float32 "{data: 42.0}" -r 1`
```

In Memory, an interoception layer inherits the robot's current pose at write time -- so a low-battery reading taken on a steep ramp is later spatially associated with that ramp.

---

## Step 3: Memory

```python
from agents.components import Memory
from agents.config import MemoryConfig
from agents.ros import MemLayer

position = Topic(name="/odometry/filtered", msg_type="Odometry")

embedding_model = OllamaModel(
    name="embeddings", checkpoint="nomic-embed-text-v2-moe:latest"
)
embedding_client = OllamaClient(embedding_model)

memory = Memory(
    layers=[
        # --- Perception layers ---
        MemLayer(subscribes_to=detections_out),
        MemLayer(subscribes_to=scene_description),

        # --- Interoception layers ---
        MemLayer(subscribes_to=battery_topic,      is_internal_state=True),
        MemLayer(subscribes_to=cpu_temp_topic,     is_internal_state=True),
        MemLayer(subscribes_to=joint_health_topic, is_internal_state=True),
    ],
    position=position,
    model_client=vlm_client,           # used for episode-consolidation gist generation + entity extraction
    embedding_client=embedding_client, # used for semantic search vector indexing
    config=MemoryConfig(
        db_path="/tmp/cortex_memory.db",
        consolidation_window=300.0,    # short window for the demo so you see consolidation kick in
        archive_after_seconds=1800.0,
    ),
    trigger=10.0,
    component_name="memory",
)
```

A few things worth seeing here:

- **Perception and interoception are peers** — they share the same node/edge graph but get tagged differently so retrieval surfaces stay clean.
- **`model_client` and `embedding_client` are different clients.** The first writes prose (gists), the second writes vectors. eMEM uses both during consolidation.
- **`db_path` is the entire memory state.** SQLite-backed eMEM means everything the robot has seen, all entities, all gists, all interoception readings, persist in a single file. **Reboot the robot, point the next session at the same `db_path`, and the agent picks up where it left off.**

---

## Step 4: Voice

Cortex will route its replies straight through TTS so the robot speaks its answers:

```python
from agents.components import TextToSpeech
from agents.config import TextToSpeechConfig

tts_in = Topic(name="cortex_output", msg_type="StreamingString")

tts = TextToSpeech(
    inputs=[tts_in],
    config=TextToSpeechConfig(enable_local_model=True, play_on_device=True),
    trigger=tts_in,
    component_name="tts",
)
```

---

## Step 5: Cortex

```python
from agents.components import Cortex
from agents.config import CortexConfig

planner_model = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
planner_client = OllamaClient(planner_model)

cortex = Cortex(
    output=tts_in,
    model_client=planner_client,
    config=CortexConfig(max_planning_steps=5, max_execution_steps=15),
    component_name="cortex",
)
```

That is the entire agent. When the launcher activates Cortex:

1. It walks every managed component and registers their capabilities.
2. It detects Memory in the recipe and augments itself to utilize it.

---

## Step 6: Launch

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.enable_ui(
    inputs=[cortex.ui_main_action_input],
    outputs=[tts_in],
)
launcher.add_pkg(
    components=[vision, captioner, memory, tts, cortex],
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
launcher.on_process_fail()
launcher.bringup()
```

---

## Talking to the robot

Run the recipe and let it sit for a few minutes -- detections accumulate, scene captions roll in every ten seconds, body-state readings are recorded continuously. Open the Web UI at `http://localhost:5001` and start asking questions.

### Perception queries

> _"What is currently around you?"_

The robot replies with what's nearby, drawing on its accumulated memory of the space rather than just the current camera frame.

> _"Where did you last see the chair?"_

You get the location (and roughly when) the chair was last observed.

> _"Summarise everything you've done in the last episode."_

A short summary of the most recent activity span.

> _"Tell me everything you know about the fridge."_

A consolidated answer that fuses every observation, summary, and recognised entity record about the fridge -- across all sessions, including the ones from earlier days.

### Body queries

> _"How are you?"_

The robot reports current battery, CPU temperature, joint health, and any other interoception layer you've wired in.

> _"Is your battery low?"_

Same, filtered to just the battery.

### Action tasks

> _"Walk over and describe the fridge."_

(Assuming you've added a navigation stack -- see [Cortex Driving the Full Stack](cortex-navigation.md) for that pairing.)

The robot first checks its own body state; if the battery is too low or a fault flag is set, it refuses the task and tells you why. Otherwise it recalls where the fridge was last seen, navigates there, takes a fresh look, narrates what it sees, and stores the new description in memory for the next session.

---

## Persistence across sessions

Stop the recipe. Restart it pointing at the same `db_path`. Memory loads the prior session's graph; Cortex re-augments its prompt with the existing layer set; the agent **remembers**.

This is the part of the design that makes eMEM-on-Cortex an actual cognitive system rather than a session-scoped vector DB. _Yesterday's robot is today's robot._ The episodes from yesterday are still in the graph, the entity for the fridge is still there, the gists are still searchable. New observations slot into the same structure.

---

## Where next

- {doc}`Cortex Driving the Full Stack <cortex-navigation>` — pairs this Cortex + Memory recipe with a Kompass navigation stack so the robot can act on memory queries instead of just answering them. Compound goals like _"go to the kitchen and tell me what's on the counter"_ fall out naturally.
- {doc}`Cortex: The Agentic Harness <cortex-agent>` — the introductory tutorial focused on Cortex's auto-discovery and tool surface, without Memory.
- {doc}`Memory concept page <../../intelligence/memory>` — the full architectural reference for eMEM.
- [eMEM on GitHub](https://github.com/automatika-robotics/emem) — the underlying memory library, including a standalone testing harness with a MiniGrid environment and a ReAct agent that exercises memory's tools end-to-end.

---

```{tip}
**Promote this recipe to production.** While you're shaping it, the script runs straight with `python recipe.py`. Once it's solid, drop it at `~/emos/recipes/<your_name>/recipe.py` and run `emos run <your_name>` -- you'll get sensor pre-flight checks, persistent logs, and a card on the dashboard so an operator can launch it from a browser. See [Running Recipes](../../getting-started/running-recipes.md) for the full development-vs-production comparison and install-mode pitfalls (especially in Container mode).
```

