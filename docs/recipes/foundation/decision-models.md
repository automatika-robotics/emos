# Decision Models

Many of the questions a robot puts to a model are not really requests for text. Was that sentence meant for me? Has the task finished? Is there anyone in this room? When you ask an LLM or a VLM, you pay for a full generation and then have to parse the words that come back, so in practice these questions get asked rarely, late, or not at all.

A decision model is made for this kind of question. You declare the possible answers up front, a yes or a no, one of a fixed set of options, or a level on a scale, and the model reads the state, scores every answer in a single forward pass, and returns a probability for each. Because it does not generate text, it cannot answer outside the answers you declared, there is nothing to parse, and the whole thing takes milliseconds. In a recipe that means the answer can be treated like any other sensor reading. An event can fire when `noul` goes above 0.8 or when `choice` becomes `"kitchen"`, and the `confidence` can be used to hand doubtful cases to a slower model or to a person. The thresholds, and what happens when they are crossed, stay in your recipe; the model only provides the numbers.

TypeSafe, who defined the API these models speak, call them System One models after the fast, intuitive System 1 of the psychology literature, as opposed to the slow deliberation of an LLM. _llama.cpp_ serves the open ones, and in EMOS they are used through `DecisionComponent`. The component asks a set of standing questions every time it is triggered and publishes each answer on its own topic. It can also answer a single question on demand through its `ask` action, which makes it available to Cortex as a tool. This tutorial builds four recipes with it: a robot that only answers when it is spoken to, a robot that works out what room it is in, a planner that checks a fact before acting on it, and an arm that stops as soon as its task is done.

## What a decision model answers

Every question has a `type` and its `instructions`. Two of the types also take `criteria`, the answers to choose from:

| Type     | You write                                                                   | You get back                                                              |
| :------- | :-------------------------------------------------------------------------- | :------------------------------------------------------------------------ |
| `noul`   | A yes/no question.                                                          | The probability of yes.                                                   |
| `choice` | The question and the options, each with a short description or none.        | The most likely option, and a probability per option.                     |
| `score`  | The question and two to ten ordered levels, lowest first.                   | The expected level, which can fall between two, and a probability per level. |

Each answer also comes with a `confidence`, which is 0 when all the options are equally likely and 1 when one of them takes all the probability. In a recipe, an answer arrives as a `Decision` message that carries the question's `id` and `type`, the answer itself in the `noul`, `choice` or `score` field, the `confidence`, and the list of `options` with their `probabilities`. These are plain numbers and strings, so an event condition can use them directly, as in `topic.msg.noul > 0.8`, `topic.msg.choice == "kitchen"` or `topic.msg.score > 1.5`.

## Serving a decision model

The open decision models are served with llama.cpp's `llama-server`, which implements the `/v1/systemone` API they use. The text-only models are small, between 144M and 4B parameters, and run fine on a CPU. The two that can read images, OpenJev and Clef, are 27B models and require a GPU. The full list is in [Models](../../intelligence/models.md#decision-models). The examples in this tutorial use `lev` for text and OpenJev for images; start whichever one the recipe you are running needs:

```bash
llama-server -m lev-Q8_0.gguf --alias lev --port 8090
llama-server -m OpenJev-Q4_K_M.gguf --mmproj mmproj-OpenJev-Q8_0.gguf --alias openjev --port 8090
```

```{tip}
`llama-server` comes with [llama.cpp](https://github.com/ggml-org/llama.cpp). You can download a [release](https://github.com/ggml-org/llama.cpp/releases) or [build it](https://github.com/ggml-org/llama.cpp/blob/master/docs/build.md) yourself; build with a GPU backend if you intend to serve one of the image models. The model files are in the `ggml-org` repositories on Hugging Face, and llama.cpp can download one for you if you pass the repository name with `-hf ggml-org/lev-GGUF` instead of a file with `-m`.
```

In the recipe, the server is reached through the generic HTTP client with a `GenericDecisionModel`. The model's checkpoint is the alias you gave the server:

```python
from agents.clients import GenericHTTPClient
from agents.models import GenericDecisionModel

decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)
```

```{note}
OpenJev's weights are released under CC BY-NC 4.0, which allows non-commercial use only. Clef, the other model that reads images, is Apache 2.0, as are the text-only models.
```

## Example 1: A robot that answers when spoken to

A conversational robot with an open microphone hears everything that is said in the room, and most of it is not meant for it. The usual answer is a wake word. A decision model can do the same job without one, and it can also catch a "stop" in the middle of a sentence. In this recipe the microphone feeds a local speech-to-text component, every utterance is sent to the decision model along with two standing questions, and the answers drive the rest of the recipe.

```python
from agents.clients import GenericHTTPClient
from agents.components import DecisionComponent, SpeechToText
from agents.config import SpeechToTextConfig
from agents.models import GenericDecisionModel
from agents.ros import Topic

audio = Topic(name="audio0", msg_type="Audio")
speech = Topic(name="speech", msg_type="String")

listener = SpeechToText(
    inputs=[audio],
    outputs=[speech],
    config=SpeechToTextConfig(enable_local_model=True, enable_vad=True),
    trigger=audio,
    component_name="listener",
)

decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)

decider = DecisionComponent(
    inputs=[speech],
    questions={
        "addressed": {"type": "noul", "instructions": "Is this speech directed at the robot?"},
        "stop": {"type": "noul", "instructions": "Is the person telling the robot to stop?"},
    },
    model_client=decision_client,
    trigger=speech,
    component_name="decider",
)

# Each question is answered on its own topic, <component_name>/<question_id>
addressed = Topic(name="decider/addressed", msg_type="Decision")
stop = Topic(name="decider/stop", msg_type="Decision")
```

```{note}
The speech components in this recipe run their models on the robot, which needs `sherpa-onnx`, and the LLM further down runs on `llama-cpp-python`. Both come with the container and pixi installs of EMOS; on a native install you add them with pip, see [Local Models](local-models.md).
```

Notice that the component has no outputs. Each question is keyed by an id, and the component publishes its answers on a topic named `<component_name>/<question_id>`, of type `Decision`, which is what the rest of the recipe subscribes to. The speech topic is the trigger, so each utterance is judged once, and all the standing questions are sent to the model together in a single request.

The answers are then put to use. The `addressed` answer becomes the trigger of the LLM. When a component is triggered by an event, it runs each time the event fires, using whatever its inputs hold at that moment, which here is the sentence that was just judged. The robot therefore only composes a reply for speech that was meant for it, and spends nothing on the rest. The `stop` answer is wired as an event to the speaker's `stop_playback` action, so a "stop" cuts the robot off mid-sentence:

```python
from agents.components import LLM, TextToSpeech
from agents.config import LLMConfig, TextToSpeechConfig
from agents.ros import Action, Event, Launcher

reply = Topic(name="reply", msg_type="String")

assistant = LLM(
    inputs=[speech],
    outputs=[reply],
    config=LLMConfig(enable_local_model=True),
    trigger=Event(addressed.msg.noul > 0.7),  # Runs only when the speech was meant for the robot
    component_name="assistant",
)

voice = TextToSpeech(
    inputs=[reply],
    outputs=[],
    config=TextToSpeechConfig(enable_local_model=True, play_on_device=True),
    trigger=reply,
    component_name="voice",
)

stop_event = Event(stop.msg.noul > 0.8)

launcher = Launcher()
launcher.enable_ui(outputs=[speech, addressed, stop, reply])
launcher.add_pkg(
    components=[listener, decider, assistant, voice],
    events_actions={stop_event: Action(voice.stop_playback)},
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

In the web interface each answer shows up as a line of text, for example `stop: yes (0.93)`, next to what was heard and what the robot replied. That is the quickest way to see how the model reads the room and where the thresholds should go. The values used here, 0.7 and 0.8, are starting points; [Thresholds and triggers](#thresholds-and-triggers) at the end of the tutorial explains how to settle on them.

<!-- TODO screenshot: the web interface with a heard sentence, the addressed and stop answers, and the reply -->

```{code-block} python
:caption: A robot that answers when spoken to
:linenos:

from agents.clients import GenericHTTPClient
from agents.components import LLM, DecisionComponent, SpeechToText, TextToSpeech
from agents.config import LLMConfig, SpeechToTextConfig, TextToSpeechConfig
from agents.models import GenericDecisionModel
from agents.ros import Action, Event, Launcher, Topic

# --- Hearing: the microphone to text ---
audio = Topic(name="audio0", msg_type="Audio")
speech = Topic(name="speech", msg_type="String")

listener = SpeechToText(
    inputs=[audio],
    outputs=[speech],
    config=SpeechToTextConfig(enable_local_model=True, enable_vad=True),
    trigger=audio,
    component_name="listener",
)

# --- Deciding: two standing questions about every utterance ---
# Served with: llama-server -m lev-Q8_0.gguf --alias lev --port 8090
decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)

decider = DecisionComponent(
    inputs=[speech],
    questions={
        "addressed": {"type": "noul", "instructions": "Is this speech directed at the robot?"},
        "stop": {"type": "noul", "instructions": "Is the person telling the robot to stop?"},
    },
    model_client=decision_client,
    trigger=speech,
    component_name="decider",
)

# Each question is answered on its own topic, <component_name>/<question_id>
addressed = Topic(name="decider/addressed", msg_type="Decision")
stop = Topic(name="decider/stop", msg_type="Decision")

# --- Answering: an LLM that only runs when the speech was meant for the robot ---
reply = Topic(name="reply", msg_type="String")

assistant = LLM(
    inputs=[speech],
    outputs=[reply],
    config=LLMConfig(enable_local_model=True),
    trigger=Event(addressed.msg.noul > 0.7),
    component_name="assistant",
)

voice = TextToSpeech(
    inputs=[reply],
    outputs=[],
    config=TextToSpeechConfig(enable_local_model=True, play_on_device=True),
    trigger=reply,
    component_name="voice",
)

# --- "Stop" cuts the robot off mid-sentence ---
stop_event = Event(stop.msg.noul > 0.8)

launcher = Launcher()
launcher.enable_ui(outputs=[speech, addressed, stop, reply])
launcher.add_pkg(
    components=[listener, decider, assistant, voice],
    events_actions={stop_event: Action(voice.stop_playback)},
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

## Example 2: What kind of place is this?

The first example asked yes/no questions about text. This one asks a choice and a score about what the robot sees. The choice is a question that has come up before in these tutorials. In [Spatio-Temporal Memory](semantic-map.md), a VLM is asked every fifteen seconds what kind of room it is in, office, bedroom or kitchen, and told to answer in one word, and a pre-processor then keeps whichever of the three words it can find in the reply. With a decision model the same thing becomes a `choice` question. The three rooms are the options, the answer is guaranteed to be one of them, and it comes with a probability for each, so the validator is no longer needed. It does not even need a model that reads images. A `Vision` component detects objects in the camera stream, and the decision model is asked about the detections, which it receives as the labels of what was found, something like `person, chair, cup`.

```python
from agents.clients import GenericHTTPClient
from agents.components import DecisionComponent, Vision
from agents.config import VisionConfig
from agents.models import GenericDecisionModel
from agents.ros import Topic

camera = Topic(name="/camera/image_raw", msg_type="Image")
detections = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[camera],
    outputs=[detections],
    config=VisionConfig(enable_local_classifier=True),  # the on-board detector
    trigger=camera,
    component_name="vision",
)

decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)

scene = DecisionComponent(
    inputs=[detections],
    questions={
        "room": {
            "type": "choice",
            "instructions": "Judging by the objects detected, what kind of room is the robot in?",
            "criteria": {
                "office": "desks, screens and chairs",
                "bedroom": "a bed and wardrobes",
                "kitchen": "cooking and eating",
            },
        },
        "crowd": {
            "type": "score",
            "instructions": "How many people are around the robot?",
            "criteria": ["nobody", "one or two", "a group", "a crowd"],
        },
    },
    model_client=decision_client,
    trigger=2.0,  # every two seconds
    component_name="scene",
)

room = Topic(name="scene/room", msg_type="Decision")
crowd = Topic(name="scene/crowd", msg_type="Decision")
```

A number as the trigger is a period in seconds, so the scene is judged every two seconds regardless of the camera's frame rate. The `choice` question lists its options in a dictionary, each with a short description for the model to read, or `None` if there is nothing to add. The `score` question lists its levels from the lowest up, and the answer is the expected level, so 1.0 means "one or two" and 1.5 is halfway between that and "a group". When a detection message is empty the model has nothing to read, so nothing is asked on that tick and the answers keep their last values. If you have questions that the labels alone cannot answer, the camera itself can be the input instead, with a model that reads images, as in the last example of this tutorial.

The events read the fields that each question type fills. For the choice it is worth adding `confidence` as a second condition, because the most likely option can win by a narrow margin, and `on_change=True` makes the event fire once when the robot enters the kitchen rather than every two seconds for as long as it stays there. For the score, the threshold goes between two levels:

```python
from agents.components import TextToSpeech
from agents.config import TextToSpeechConfig
from agents.ros import Action, Event, Launcher

announcement = Topic(name="announcement", msg_type="String")
voice = TextToSpeech(
    inputs=[announcement],
    outputs=[],
    config=TextToSpeechConfig(enable_local_model=True, play_on_device=True),
    trigger=announcement,
    component_name="voice",
)

in_kitchen = Event((room.msg.choice == "kitchen") & (room.msg.confidence > 0.5), on_change=True)
crowded = Event(crowd.msg.score > 1.5, on_change=True)

launcher = Launcher()
launcher.enable_ui(outputs=[detections, room, crowd])
launcher.add_pkg(
    components=[vision, scene, voice],
    events_actions={
        in_kitchen: Action(voice.say, kwargs={"text": "We are in the kitchen"}),
        crowded: Action(voice.say, kwargs={"text": "It is getting crowded, I will slow down"}),
    },
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

These events can of course do anything else an event can do, such as lowering a speed limit on the drive manager, switching the planner, or waking a VLM to describe the crowd. See [Events & Actions](../../concepts/events-and-actions.md) for everything that is available.

```{code-block} python
:caption: What kind of place is this?
:linenos:

from agents.clients import GenericHTTPClient
from agents.components import DecisionComponent, TextToSpeech, Vision
from agents.config import TextToSpeechConfig, VisionConfig
from agents.models import GenericDecisionModel
from agents.ros import Action, Event, Launcher, Topic

# --- Seeing: an on-board detector on the camera stream ---
camera = Topic(name="/camera/image_raw", msg_type="Image")
detections = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[camera],
    outputs=[detections],
    config=VisionConfig(enable_local_classifier=True),
    trigger=camera,
    component_name="vision",
)

# --- Deciding: a choice and a score about what was detected ---
# Served with: llama-server -m lev-Q8_0.gguf --alias lev --port 8090
decision_client = GenericHTTPClient(GenericDecisionModel(name="lev", checkpoint="lev"), port=8090)

scene = DecisionComponent(
    inputs=[detections],
    questions={
        "room": {
            "type": "choice",
            "instructions": "Judging by the objects detected, what kind of room is the robot in?",
            "criteria": {
                "office": "desks, screens and chairs",
                "bedroom": "a bed and wardrobes",
                "kitchen": "cooking and eating",
            },
        },
        "crowd": {
            "type": "score",
            "instructions": "How many people are around the robot?",
            "criteria": ["nobody", "one or two", "a group", "a crowd"],
        },
    },
    model_client=decision_client,
    trigger=2.0,  # every two seconds
    component_name="scene",
)

room = Topic(name="scene/room", msg_type="Decision")
crowd = Topic(name="scene/crowd", msg_type="Decision")

# --- Speaking up on what the answers say ---
announcement = Topic(name="announcement", msg_type="String")
voice = TextToSpeech(
    inputs=[announcement],
    outputs=[],
    config=TextToSpeechConfig(enable_local_model=True, play_on_device=True),
    trigger=announcement,
    component_name="voice",
)

in_kitchen = Event((room.msg.choice == "kitchen") & (room.msg.confidence > 0.5), on_change=True)
crowded = Event(crowd.msg.score > 1.5, on_change=True)

launcher = Launcher()
launcher.enable_ui(outputs=[detections, room, crowd])
launcher.add_pkg(
    components=[vision, scene, voice],
    events_actions={
        in_kitchen: Action(voice.say, kwargs={"text": "We are in the kitchen"}),
        crowded: Action(voice.say, kwargs={"text": "It is getting crowded, I will slow down"}),
    },
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

## Example 3: Asking on demand

Standing questions are asked whenever the component is triggered. The other way to use the component is its `ask` action, which answers a single question when it is called, either about the component's inputs or about a state that the caller passes in. It is a component action with a tool description, so [Cortex](../../intelligence/cortex.md) gets it in both of its phases, and the planner can check a fact in a few milliseconds instead of reasoning about it. It can also be registered on a plain LLM like any other component action.

With Cortex nothing needs to be wired. Add a decision component to the recipe and the planner finds `checker-ask` among its tools, with the question's `instructions`, `type` and `criteria` as parameters, plus an optional `state`:

```python
from agents.components import Cortex, DecisionComponent

checker = DecisionComponent(
    inputs=[detections],  # what `ask` is about when the caller gives no state
    model_client=decision_client,
    component_name="checker",
)

cortex = Cortex(output=cortex_output, model_client=planner_client, component_name="cortex")
```

Given the task *"go to the kitchen and tell me whether anyone is there"*, the planner can send the navigation goal and then call `checker-ask` with `instructions="Is anyone in the room?"`. The answer comes back as JSON, for example `{"type": "noul", "noul": 0.07}`. The rest of the recipe, the components and how the goal is sent, is the same as in [Cortex: The Agentic Harness](../planning-and-manipulation/cortex-agent.md). A decision component with no inputs and no questions works purely as a tool; in that case the caller has to pass the `state` it wants a decision about.

A plain LLM gets the same tool through `register_tool`, together with a description of what it is allowed to ask. The function name in the description has to be `ask`, because that is the action the call is routed to:

```python
ask_tool = {
    "type": "function",
    "function": {
        "name": "ask",
        "description": "Ask a decision model a yes/no question about what the robot currently sees.",
        "parameters": {
            "type": "object",
            "properties": {"instructions": {"type": "string", "description": "The question"}},
            "required": ["instructions"],
        },
    },
}
assistant.register_tool(tool=checker.ask, tool_description=ask_tool, send_tool_response_to_model=True)
```

See [Tool Calling](tool-calling.md) for what `send_tool_response_to_model` does with the answer.

## Example 4: An arm that knows when it is done

In [Event-Driven VLA](../planning-and-manipulation/event-driven-vla.md) a VLM acts as the referee. It looks at the camera every two minutes and answers in words, and the recipe searchs the answer for a YES. A decision model makes a better referee. It answers every second with a probability, and it does not even need to be told what the task is, because `action_states` lets it ask the VLA for the task of the goal that is currently running and add that to the state the question is asked about. The player and the simulation are the same as in that recipe; only the referee changes.

```python
from agents.clients import GenericHTTPClient
from agents.components import DecisionComponent
from agents.models import GenericDecisionModel
from agents.ros import Event, Topic

# A model that reads images, served with:
# llama-server -m OpenJev-Q4_K_M.gguf --mmproj mmproj-OpenJev-Q8_0.gguf --alias openjev --port 8090
checker_client = GenericHTTPClient(GenericDecisionModel(name="openjev", checkpoint="openjev"), port=8090)

checker = DecisionComponent(
    inputs=[front_camera],
    action_states={"task": player.get_current_task},
    questions={
        "done": {"type": "noul", "instructions": "Has the robot completed the task given in the state?"},
    },
    model_client=checker_client,
    trigger=1.0,
    component_name="checker",
)

task_done = Topic(name="checker/done", msg_type="Decision")
player.set_termination_trigger(mode="event", stop_event=Event(task_done.msg.noul > 0.8), max_timesteps=1800)
```

```{important}
The models that read images are large. OpenJev's 4-bit file alone is 16 GB, so next to a simulator and a policy server it is best served from another machine, which the client reaches through its `host` argument.
```

An image input is passed to the model as a picture, which is why this example needs a model that can read images. `action_states` maps a name to a component action. On every trigger the checker calls that action, whichever process its component runs in, and stores the result in the state under the given name, so the model sees `{"task": "Grab orange and place into plate"}` next to the picture. The VLA's `get_current_task` fails while no goal is running, and when an action state fails the questions are not asked on that tick, so the checker sits idle between goals without any extra wiring. The threshold of 0.8 was set by looking at recorded frames of this task; a different task, or a different wording of the question, will move it.

<!-- TODO screenshot: the web interface during a pick, with the front camera and the checker's `done` answers rising to yes -->

```{code-block} python
:caption: An arm that knows when it is done
:linenos:

from agents.clients import GenericHTTPClient, LeRobotClient
from agents.components import DecisionComponent, VLA
from agents.config import VLAConfig
from agents.models import GenericDecisionModel, LeRobotPolicy
from agents.ros import Event, Launcher, Topic

# --- Topics from the simulation bridge ---
joint_states = Topic(name="/so101/joint_states", msg_type="JointState")
front_camera = Topic(name="/so101/front/image_raw", msg_type="Image")
wrist_camera = Topic(name="/so101/wrist/image_raw", msg_type="Image")
joint_cmd = Topic(name="/so101/joint_cmd", msg_type="JointState")

# --- The player, as in the VLA recipes ---
policy = LeRobotPolicy(
    name="pick_orange_gr00t",
    policy_type="groot",
    checkpoint="aleph-ra/gr00t17_pick_orange_lora",
    dataset_info_file="https://huggingface.co/datasets/LightwheelAI/leisaac-pick-orange/resolve/main/meta/info.json",
    actions_per_chunk=16,
)
client = LeRobotClient(model=policy, host="127.0.0.1", port=8080)

SO101_JOINTS = ["shoulder_pan", "shoulder_lift", "elbow_flex", "wrist_flex", "wrist_roll", "gripper"]
limits = {joint: {"lower": -100.0, "upper": 100.0} for joint in SO101_JOINTS[:-1]}
limits["gripper"] = {"lower": 0.0, "upper": 100.0}

config = VLAConfig(
    joint_names_map={f"{joint}.pos": joint for joint in SO101_JOINTS},
    camera_inputs_map={"front": front_camera, "wrist": wrist_camera},
    joint_limits=limits,
    observation_sending_rate=0.55,
    action_sending_rate=10.0,
    aggregate_fn_name="latest_only",
)

player = VLA(
    inputs=[joint_states, front_camera, wrist_camera],
    outputs=[joint_cmd],
    model_client=client,
    config=config,
    component_name="vla_sim",
)

# --- The referee: a decision model that reads the front camera ---
# Served with: llama-server -m OpenJev-Q4_K_M.gguf --mmproj mmproj-OpenJev-Q8_0.gguf --alias openjev --port 8090
checker_client = GenericHTTPClient(GenericDecisionModel(name="openjev", checkpoint="openjev"), port=8090)

checker = DecisionComponent(
    inputs=[front_camera],
    action_states={"task": player.get_current_task},  # the task of the running goal joins the state
    questions={
        "done": {"type": "noul", "instructions": "Has the robot completed the task given in the state?"},
    },
    model_client=checker_client,
    trigger=1.0,
    component_name="checker",
)

# --- The event that closes the loop ---
task_done = Topic(name="checker/done", msg_type="Decision")
player.set_termination_trigger(mode="event", stop_event=Event(task_done.msg.noul > 0.8), max_timesteps=1800)

# --- Launch ---
launcher = Launcher()
launcher.enable_ui(inputs=[player.ui_main_action_input], outputs=[task_done, front_camera])
launcher.add_pkg(components=[player, checker])
launcher.bringup()
```

## Thresholds and triggers

The probabilities a decision model returns depend on how the question is worded, so a threshold belongs to a particular question. The practical way to set one is to look at real cases. Run the recipe, watch the answers in the web interface for situations that should fire and for ones that should not, and put the line between them. If you reword the question, check the numbers again. For a choice, `confidence` makes a good second condition, since the winning option may have won by very little.

Because standing questions are answered on every trigger, it matters how an event on them is set up. Without `on_change`, the event fires on every answer that satisfies it, which is what you want for a command like "stop", where every one counts. With `on_change=True` it fires only when the answer crosses the threshold, which is what you want for a state like "we are in the kitchen".

The trigger decides when the questions are asked. A topic trigger asks once per message, a number asks every so many seconds, and an event asks when it fires. Text and detection inputs make up the state, images are sent as pictures, and all the questions go out in the one request a trigger sends. If the server does not answer, the failure is reported in the component's health status as it is for any model component, so whatever [fallbacks](../../concepts/status-and-fallbacks.md) the recipe has set apply.

## Where else decision models appear

The [SemanticRouter](semantic-routing.md#option-2-decision-mode) can route with a decision model as well. In its decision mode the routes become the options of a single choice question, and `minimum_confidence` is the threshold.

```{seealso}
- [Models](../../intelligence/models.md#decision-models) for the models that can be served, and [Clients](../../intelligence/clients.md) for the client that reaches them.
- [Events & Actions](../../concepts/events-and-actions.md) for everything an answer can trigger.
- [Cortex](../../intelligence/cortex.md) for how `ask` reaches the planner.
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
