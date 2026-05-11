# Cortex Driving the Full Stack

This is the tutorial that shows what [Cortex](../../intelligence/cortex.md) is *for*. We give a robot eyes, a voice, a body that can move, and a memory that learns. Then we drop a Cortex on top of all of that and start typing compound, free-form goals into the Web UI:

> *"Go to the kitchen and tell me what's on the counter."*
>
> *"Find the closest chair, take a picture there, and come back here."*
>
> *"Patrol the room until you see a person, then announce their location."*

There is no parser, no state machine, no sequence diagram in this recipe. Cortex inspects the running graph, plans the steps, dispatches navigation goals, watches their feedback, calls a VLM to look at the world when it arrives, queries Memory for what it has seen before, and routes its replies through TTS so the robot speaks. **The recipe is the graph; the agent is one master component.**

```{seealso}
For the introduction to the Cortex abstraction itself, start with [Cortex: The Agentic Harness](cortex-agent.md). For the conceptual reference covering the full feature set Cortex auto-discovers, see [Cortex](../../intelligence/cortex.md). For Cortex without a navigation stack, focused on the deep pairing with spatio-temporal memory, see [Memory and Cortex](cortex-memory.md).
```

---

## What we're orchestrating

Five subsystems, all running in one launcher:

| Subsystem | Components | What Cortex does with it |
|---|---|---|
| **Perception** | `Vision`, `VLM` | Detection feeds Memory; VLM answers visual questions on demand. |
| **Spatial memory** | `Memory` | Stores detections + scene captions tagged with position. Exposes `locate`, `recall`, `semantic_search`, `body_status`, ten retrieval tools in total. |
| **Voice** | `TextToSpeech` | The robot's mouthpiece. Cortex routes its replies straight through. |
| **Navigation** | `Planner`, `Controller`, `LocalMapper`, `DriveManager` | The Kompass quartet. Cortex sends action goals to `Planner`'s main action server. |
| **The agent** | `Cortex` | Discovers all of the above, exposes them as LLM tools, plans and executes against natural-language goals. |

The wiring is conventional EMOS. The Cortex section at the end is what turns the whole stack into a single self-directing agent.

---

## Step 1: Robot configuration

Standard differential-drive setup; adjust to your platform.

```python
import numpy as np
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotConfig,
    RobotFrames,
    RobotGeometry,
    RobotType,
)

my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,
    geometry_type=RobotGeometry.Type.CYLINDER,
    geometry_params=np.array([0.1, 0.3]),
    ctrl_vx_limits=LinearCtrlLimits(max_vel=0.4, max_acc=1.5, max_decel=2.5),
    ctrl_omega_limits=AngularCtrlLimits(
        max_vel=0.4, max_acc=2.0, max_decel=2.0, max_steer=np.pi / 3
    ),
)
```

---

## Step 2: The navigation stack

```python
from kompass.components import (
    Controller,
    DriveManager,
    LocalMapper,
    LocalMapperConfig,
    Planner,
    PlannerConfig,
)
from kompass.control import ControllersID, MapConfig
from kompass.ros import Topic

# Planner — runs as an Action Server so Cortex can dispatch goals into it.
planner = Planner(component_name="planner", config=PlannerConfig(loop_rate=1.0))
planner.run_type = "ActionServer"

# Controller — DWA, eats from the local map.
controller = Controller(component_name="controller")
controller.algorithm = ControllersID.DWA
controller.direct_sensor = False

# DriveManager
driver = DriveManager(component_name="drive_manager")
driver.outputs(robot_command=Topic(name="/cmd_vel", msg_type="Twist"))

# Local Mapper — fed by a 2D laser
local_mapper = LocalMapper(
    component_name="mapper",
    config=LocalMapperConfig(
        map_params=MapConfig(width=4.0, height=4.0, resolution=0.1),
    ),
)
local_mapper.inputs(sensor_data=Topic(name="/scan", msg_type="LaserScan"))
```

---

## Step 3: Perception

```python
from agents.clients import OllamaClient
from agents.components import VLM, Vision
from agents.config import VisionConfig
from agents.models import OllamaModel
from agents.ros import FixedInput

image_in = Topic(name="/image_raw", msg_type="Image")
detections_out = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[image_in],
    outputs=[detections_out],
    config=VisionConfig(threshold=0.5, enable_local_classifier=True),
    trigger=1.0,
    component_name="vision",
)

# A VLM that captions every 10 seconds — the captions feed Memory and TTS.
scene_query = FixedInput(
    name="scene_query",
    msg_type="String",
    fixed="Describe the scene in one concise sentence: room type and notable objects.",
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

The VLM's `describe` and Vision's `track` / `take_picture` are `@component_action`s already declared upstream. **Cortex will discover them all.** We don't wire any of them by hand.

---

## Step 4: Memory

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
        MemLayer(subscribes_to=detections_out),
        MemLayer(subscribes_to=scene_description),
    ],
    position=position,
    model_client=vlm_client,
    embedding_client=embedding_client,
    config=MemoryConfig(db_path="/tmp/embodied_memory.db"),
    trigger=10.0,
    component_name="memory",
)
```

When Memory is in the recipe, Cortex **automatically augments itself** with task-classification guidance. Action tasks get wrapped in episodes -- Cortex begins them with `start_episode` and ends with `end_episode` so the observations made during the task get consolidated into the long-term graph.

```{tip}
Add an interoception layer (`MemLayer(subscribes_to=battery_topic, is_internal_state=True)`) and Cortex starts every action plan with a `body_status` check -- it might refuse to navigate when the battery is below a threshold, with a clear text explanation. See [Memory and Cortex](cortex-memory.md) for the full pattern.
```

---

## Step 5: Voice

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

## Step 6: Cortex

```python
from agents.components import Cortex
from agents.config import CortexConfig

planner_model = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
planner_client = OllamaClient(planner_model)

cortex = Cortex(
    output=tts_in,                # Cortex streams replies into TTS
    model_client=planner_client,
    config=CortexConfig(
        max_planning_steps=5,
        max_execution_steps=15,
    ),
    component_name="cortex",
)
```

That's the whole agent. **No `actions=[...]` is needed** -- the agent's tool palette is everything the other components contribute:

If you want to add a private capability that isn't on a managed component -- a database call, a custom servo on a peripheral, an external API -- pass it as a custom `Action(method=..., description=...)` in `actions=[...]` and Cortex registers it alongside the rest.

---

## Step 7: Launch

```python
from kompass.ros import Launcher

launcher = Launcher()

# Navigation stack
launcher.add_pkg(
    components=[planner, controller, driver, local_mapper],
    multiprocessing=True,
    package_name="kompass",
)

# Intelligence stack
launcher.add_pkg(
    components=[vision, captioner, memory, tts, cortex],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)

# Shared inputs
launcher.inputs(location=position)

# Robot config + frames
launcher.robot = my_robot
launcher.frames = RobotFrames(world="map", odom="odom", scan="base_scan")

launcher.enable_ui(
    inputs=[cortex.ui_main_action_input],
    outputs=[tts_in],
)

launcher.on_process_fail()
launcher.bringup()
```

---

## Driving the agent

Run the recipe and let the robot wander around for a few minutes. Memory accumulates detections and scene captions tagged with positions. Then open `http://localhost:5001` and start typing.

### Single-step goals

> *"What is currently around you?"* — the robot describes its surroundings.
>
> *"Where did you last see the chair?"* — gives the location and roughly when.
>
> *"How are you?"* — reads its body state and replies.

### Compound goals -- the interesting case

> *"Go to the chair."*

The robot recalls where the chair is from memory, dispatches a navigation goal, and reports when it has arrived.

> *"Go to the kitchen and tell me what's on the counter."*

The robot navigates to the kitchen using its memory of where that is, looks at the counter once it arrives, narrates what it sees, and stores the new observation in memory for the next session.

> *"Patrol the room. If you see a person, stop and tell me where you found them."*

A long-horizon mission. The robot dispatches successive navigation goals around the room and watches its detections. The moment a person appears, it cancels the current goal and reports their location.

None of these required you to write orchestration code, prompts, or a state machine. The behaviour emerges from Cortex's read of the auto-discovered tool surface and the memory-aware planning it installs when it sees a Memory component in the recipe.

---

## What this looks like in the Web UI

Send a goal, and the action goal Cortex dispatches into the Planner appears live as feedback in the **main logging card** -- both the Planner's path-tracking feedback and Cortex's own confirmation decisions stream side by side.

---

## What you didn't have to write

- An event triggering "go to X" when the user types it.
- An LLM prompt parsing "go to X" into a destination.
- A goal-builder for the Planner action.
- An action client construction with feedback callbacks and cancellation logic.
- A retry policy for the navigation goal.
- A wait-loop that blocks until SUCCEEDED before invoking the VLM.
- A handoff from the VLM output to the TTS input.
- A memory write that records "I went to the kitchen and saw X".

That entire stack of orchestration is replaced by the auto-discovery, the two-phase loop, and the Memory-aware prompt augmentation. **You wrote the components. Cortex wrote the recipe.**

---

## Where next

- {doc}`Cortex concept page <../../intelligence/cortex>` -- a deeper look at the auto-discovery, the planning/execution loop, RAG, and the Cortex-as-Monitor model.
- {doc}`Memory and Cortex <cortex-memory>` -- the same agentic harness focused entirely on memory: episodic consolidation, entity tracking, interoception layers, and how Cortex reasons over them.
- {doc}`Cortex: The Agentic Harness <cortex-agent>` -- the introductory tutorial if you want to start with a smaller graph before scaling up to navigation.
- {doc}`Visualizing the System Graph <../events-and-resilience/visualizing-system-graph>` -- watch the System Graph render Cortex's tool palette and the goal-status events flowing through it.

---

```{tip}
**Promote this recipe to production.** While you're shaping it, the script runs straight with `python recipe.py`. Once it's solid, drop it at `~/emos/recipes/<your_name>/recipe.py` and run `emos run <your_name>` -- you'll get sensor pre-flight checks, persistent logs, and a card on the dashboard so an operator can launch it from a browser. See [Running Recipes](../../getting-started/running-recipes.md) for the full development-vs-production comparison and install-mode pitfalls (especially in Container mode).
```

