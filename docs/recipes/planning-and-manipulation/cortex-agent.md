# Cortex: The Agentic Harness

**A single component, dropped on top of the rest of your recipe, that turns it into a self-directed agent.**

Most EMOS recipes you've seen so far are *programmed*: events trigger components, components publish to topics, fallbacks recover from failure -- and you, the recipe author, hand-wired every link. The Cortex component is a different shape. Drop a Cortex into your recipe and it discovers every other component you added, registers every method they expose as a callable tool, and -- given a high-level goal like *"track the person on the left and tell me what they're holding"* -- decomposes it into an ordered plan, dispatches each step, watches the feedback, and replans on failure. No orchestration glue from you.

If [Claude Code](https://claude.com/claude-code) is an agentic harness for software engineering, **Cortex is an agentic harness for embodied intelligence**. The capability components -- Vision, VLM, TTS, navigation, memory -- are the robot's limbs and senses. Cortex is the part that decides *what to do next* using these capabilities.

```{seealso}
For the conceptual model and the full list of capabilities Cortex auto-discovers, see [Cortex](../../intelligence/cortex.md). For Cortex paired with a graph-backed spatio-temporal memory, see [Memory and Cortex](cortex-memory.md). For Cortex orchestrating the navigation stack on top of all of that, see [Cortex Driving the Full Stack](cortex-navigation.md).
```

---

## The shape of the abstraction

A capability component such as `Vision` exposes its primary work as topics (`/detections`, `/trackings`) but it *also* exposes private methods decorated with `@component_action`:

```python
class Vision(Component):
    @component_action(description={...})
    def track(self, label: str): ...

    @component_action(description={...})
    def take_picture(self, save_path: str = "..."): ...
```

These actions are normally invisible -- they require explicit wiring through the events/actions system to be useful. **Cortex changes that.** When you drop a Cortex component into the launcher, on activation it walks every managed component and discovers:

| What gets discovered | What Cortex does with it |
|---|---|
| `@component_action` methods | Auto-registers as an LLM tool, namespaced as `{component}.{method}`. Each carries its OpenAI-format description so the planner knows what it does. |
| `@component_fallback` methods | Same as above -- exposed as callable recovery tools the planner can fall back to. |
| Additional ROS services (`get_ros_entrypoints()`) | Registered as `send_request_to_{name}`, with the request schema auto-translated to JSON properties so the LLM can fill the fields. |
| Additional ROS action servers | Registered as `send_goal_to_{name}` with the same schema translation. |
| The component's main action server | Registered the same way -- so a Planner running as `ActionServer` becomes a callable navigation tool. |
| Component config parameters | Reachable via the built-in `update_parameter(component, param_name, new_value)` execution tool -- the LLM can re-tune any parameter at runtime. |
| Component structure | Reachable via the built-in `inspect_component(name)` planning tool -- the LLM reads the recipe live before committing a plan. |

Every one of those tools is automatic. You write the components; Cortex makes them addressable.

---

## What we're building

A robot that, when you tell it *"describe what you see and then start tracking the person"*, will:

1. Plan three steps -- call `vlm.describe`, feed its answer to `tts.say`, then call `vision.track`.
2. Execute them in order -- describing the scene, speaking the description through `tts.say`, then asking `Vision` to start tracking the requested label.
3. Report each step's result back into the planning loop and close out the episode.

The recipe is short. There is no event wiring. There are no fallback policies. There are no topic-routed connections between the VLM, the TTS, and Cortex -- speech happens because Cortex calls `tts.say()` as a tool, not because some output topic is silently subscribed by TTS. We don't write a single prompt either -- Cortex's built-in prompts plus the auto-discovered tool descriptions are the prompt.

---

## Step 1: Bring up your capability components

We need eyes, a voice, and visual reasoning. Standard EMOS capability components:

```python
from agents.clients import OllamaClient, RoboMLRESPClient
from agents.components import TextToSpeech, VLM, Vision
from agents.config import TextToSpeechConfig, VisionConfig
from agents.models import OllamaModel, VisionModel
from agents.ros import Topic

# Vision — detection + tracking
detection_model = VisionModel(name="rtdetr", checkpoint="PekingU/rtdetr_r50vd_coco_o365")
detection_client = RoboMLRESPClient(detection_model)

image_in = Topic(name="/image_raw", msg_type="Image")
detections = Topic(name="detections", msg_type="Detections")
trackings = Topic(name="trackings", msg_type="Trackings")

vision = Vision(
    inputs=[image_in],
    outputs=[detections, trackings],
    model_client=detection_client,
    config=VisionConfig(threshold=0.5),
    trigger=0.5,
    component_name="vision",
)

# VLM — visual question answering. Cortex invokes it via ``vlm.describe``,
# and the action's return value comes back as the tool result.
vlm_model = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
vlm_client = OllamaClient(vlm_model)

vlm_query = Topic(name="vlm_query", msg_type="String")
vlm_response = Topic(name="vlm_response", msg_type="String")

vlm = VLM(
    inputs=[vlm_query, image_in],
    outputs=[vlm_response],
    model_client=vlm_client,
    trigger=vlm_query,
    component_name="vlm",
)

# TTS — speech happens via Cortex calling ``tts.say(text=...)``.
tts_input = Topic(name="tts_input", msg_type="String")

tts = TextToSpeech(
    inputs=[tts_input],
    config=TextToSpeechConfig(enable_local_model=True, play_on_device=True),
    trigger=tts_input,
    component_name="tts",
)
```

Nothing here is Cortex-specific. Each component has its own `@component_action` methods declared upstream -- `Vision.track`, `Vision.take_picture`, `VLM.describe`, `TTS.say` -- and Cortex will discover all of them on activation. Cortex sequences the components by calling their actions in turn.

---

## Step 2: Drop in Cortex

```python
from agents.components import Cortex
from agents.config import CortexConfig
from agents.ros import Action

# A planner LLM. Choose a chat-grade model — the smaller, the faster the loop.
planner_model = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
planner_client = OllamaClient(planner_model)

# Cortex publishes its text-only replies (cases where the planner decides no
# tool calls are needed) to this topic for downstream consumers (e.g. the Web
# UI). When the planner *does* want the robot to speak, it calls ``tts.say``
# as a tool -- it does not rely on this topic being subscribed by TTS.
cortex_output = Topic(name="cortex_output", msg_type="String")

cortex = Cortex(
    output=cortex_output,
    model_client=planner_client,
    config=CortexConfig(max_planning_steps=5, max_execution_steps=10),
    component_name="cortex",
)
```

**That's all.** No `actions=[…]` list -- the capability components contribute their own actions. No prompt -- the built-in prompt plus the discovered tool descriptions are the prompt. No fallback wiring -- Cortex confirms each step before executing it and replans on failure.

---

## Step 3: Adding your own custom action

A capability you want exposed to the planner that doesn't naturally live on a managed component? Pass it as a custom `Action`. Cortex registers it alongside everything else.

```python
led_on = False

def toggle_led():
    """Toggle an LED on the robot."""
    global led_on
    led_on = not led_on
    print(f"LED toggled {'ON' if led_on else 'OFF'}")

cortex = Cortex(
    actions=[
        Action(method=toggle_led, description="Toggle the robot's LED on or off."),
    ],
    output=cortex_output,
    model_client=planner_client,
    config=CortexConfig(max_planning_steps=5, max_execution_steps=10),
    component_name="cortex",
)
```

The description is mandatory -- it's what the planner sees when deciding whether to call this tool.

---

## Step 4: Launch

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.enable_ui(
    inputs=[cortex.ui_main_action_input],
    outputs=[cortex_output],
)
launcher.add_pkg(
    components=[vision, vlm, tts, cortex],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)
launcher.on_process_fail()      # process-level safety net
launcher.bringup()
```

`launcher.enable_ui` registers a goal-input field for Cortex's main action and a streaming output panel for `cortex_output`. The whole agent runs in a single launcher process tree.

---

## Talking to the agent

Open the Web UI at `http://localhost:5001` and send tasks in plain English:

| Goal | What Cortex plans |
|---|---|
| *"describe what you see"* | Two steps: `vlm.describe` produces a sentence; `tts.say` is called with that sentence as its `text` argument. |
| *"start tracking the person"* | One step: `vision.track(label="person")`. The Vision component's `@component_action` starts continuous tracking on the named label (results stream on the `trackings` topic) and returns a confirmation string immediately. |
| *"take a picture, describe it, then track whatever's in front of you"* | Three steps, sequenced. The third step's argument is bound from the second step's output -- Cortex resolves `<output from step 2>` placeholders at runtime. |
| *"toggle the LED"* | One step: the custom `toggle_led` action you registered. |
| *"are you ok?"* | No actions needed. The planner returns text only; the reply lands on the `cortex_output` topic. (If you want it spoken, your prompt can nudge the planner to always end with a `tts.say` call.) |

Or send a goal from another terminal directly to Cortex's action server:

```shell
ros2 action send_goal /cortex_<process_id>/cortex_input_command \
    automatika_embodied_agents/action/VisionLanguageAction \
    "{task: 'describe what you see and track the person'}"
```

Watch the launcher's main logging card to see the planning trace, the goals Cortex dispatches, and feedback streamed back from each component.

---

## What just happened

When you sent the goal *"describe what you see and then start tracking the person"*, Cortex:

1. Built a plan via the planning loop. The first iteration optionally called `inspect_component("vision")` to confirm the tool surface, then committed three execution tool calls.
2. Confirmed and called each step in turn. The first (`vlm.describe`) returned a text description; the second (`tts.say`) was called with that description bound as its `text` argument and the speaker spoke it; the third (`vision.track`) asked the Vision component to start continuous tracking on the named label and returned a confirmation string. Tracking results then streamed on the component's `trackings` topic for any downstream consumer to use.
3. With every step's tool result folded back into the trace, the episode closed and the plan returned `SUCCEEDED`.

Compare that to the equivalent recipe written without Cortex: bespoke event wiring for the trigger, hand-tuned prompts on each component, manual sequencing of the speech and tracking calls. **Cortex collapses all of that into the one component you just dropped in.**

```{tip}
For the long-running case -- where Cortex *should* dispatch a Kompass action server like the Controller's `track_vision_target` (or the Planner's `navigate_to_goal`) and watch its feedback stream until the goal completes -- add the `Controller` (or `Planner`) component to the launcher. Cortex auto-registers each one's main action server as `send_goal_to_<server>` and switches into asynchronous monitoring mode. See [Cortex Driving the Full Stack](cortex-navigation.md).
```

---

## Where next

- {doc}`Cortex Driving the Full Stack <cortex-navigation>` -- the showcase tutorial. Cortex orchestrates a navigation stack, vision, memory, and speech to handle compound natural-language goals like *"go to the kitchen and tell me what's on the counter"*.
- {doc}`Memory and Cortex <cortex-memory>` -- add a graph-backed spatio-temporal memory so Cortex can reason over past observations and the robot's own internal state.
- {doc}`Cortex concept page <../../intelligence/cortex>` -- the full reference for the planning loop, the confirmation step, RAG, async goal monitoring, and the Cortex-as-Monitor architecture.
