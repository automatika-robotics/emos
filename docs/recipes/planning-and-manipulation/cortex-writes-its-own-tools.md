# Cortex Writes Its Own Tools

In the Cortex tutorials so far, the planner has been given a set of tools and has chosen among them, which works as long as the recipe has a tool for everything a task might need. It rarely does. Sooner or later a task needs something nobody wired in, such as a way to send an email or a sensor reading that no component publishes, and until now the only answer was to stop the robot, write the missing component or action, and start again. This tutorial turns on `enable_scratch_functions`, which lets the planner write the missing piece itself as a Python function and put it to use immediately.

To see it at work, we build a small recipe with a camera, object detection and a voice, and then give Cortex two standing instructions that it cannot carry out with what the recipe provides:

- *"Whenever a person is detected, email me about it."* There is no email action anywhere in the recipe.
- *"Whenever the CPU temperature goes above 80 degrees, say so out loud."* No topic publishes the CPU temperature.

The planner handles the first one by writing an action and installing an event on the detections topic that runs it, and the second by writing a condition and installing an event that polls that condition and calls the voice. Both therefore combine the standing-instruction tools with the new function-writing tools, each of which the [Cortex](../../intelligence/cortex.md) page explains on its own, under [Standing instructions](../../intelligence/cortex.md#standing-instructions) and [Functions the planner writes](../../intelligence/cortex.md#functions-the-planner-writes).

```{warning}
A function the planner writes runs in the launcher process, with that process's privileges, and it can do anything Python can do. That is why the feature is off by default, and why it should only be turned on with a planning model and on a deployment that you trust to that degree.
```

## Step 1: The capability components

The recipe has two ordinary components: a `Vision` component that publishes detections from the camera, and a `TextToSpeech` component that speaks whatever arrives on its input topic and also offers a `say` action, which is what the planner will end up calling. Both use models served by RoboML, as in [Cortex: The Agentic Harness](cortex-agent.md).

```python
from agents.clients import RoboMLRESPClient
from agents.components import TextToSpeech, Vision
from agents.config import TextToSpeechConfig, VisionConfig
from agents.models import TransformersTTS, VisionModel
from agents.ros import Topic

detection_client = RoboMLRESPClient(
    VisionModel(name="rtdetr", checkpoint="PekingU/rtdetr_r50vd_coco_o365")
)
tts_client = RoboMLRESPClient(TransformersTTS(name="speecht5"))

image_in = Topic(name="/image_raw", msg_type="Image")
detections_out = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[image_in],
    outputs=[detections_out],
    model_client=detection_client,
    config=VisionConfig(threshold=0.5),
    trigger=0.5,
    component_name="vision",
)

text_in = Topic(name="text_in", msg_type="String")

tts = TextToSpeech(
    inputs=[text_in],
    model_client=tts_client,
    config=TextToSpeechConfig(play_on_device=True),
    trigger=text_in,
    component_name="tts",
)
```

## Step 2: A planner that may write code

Three settings on `CortexConfig` make the difference. `enable_events` gives the planner the standing-instruction tools, which both of our instructions need, and `enable_scratch_functions` lets it write functions. The third, `scratch_notes`, tells the planner what the environment holds, and it matters more than it looks, because the planner cannot see your environment variables or your filesystem. This note is where it learns that an SMTP account is available and where the CPU temperature can be read, and without it the code it writes would be guesswork.

```python
from agents.clients import OllamaClient
from agents.components import Cortex
from agents.config import CortexConfig
from agents.models import OllamaModel

planner_client = OllamaClient(
    OllamaModel(name="qwen", checkpoint="qwen3.5:latest", think=False),
    inference_timeout=120,
)

cortex = Cortex(
    model_client=planner_client,
    config=CortexConfig(
        enable_events=True,
        enable_scratch_functions=True,
        scratch_notes=(
            "The process environment holds SMTP_HOST, SMTP_PORT, SMTP_USER and "
            "SMTP_PASSWORD, an SMTP server and account for sending mail, and "
            "ALERT_EMAIL, the operator's address. The CPU temperature in "
            "millidegrees Celsius can be read from "
            "/sys/class/thermal/thermal_zone0/temp."
        ),
        max_new_tokens=2000,
    ),
    component_name="cortex",
)
```

Two settings on the model side matter just as much. Since a written function's source has to arrive in a single reply, `max_new_tokens` needs a generous budget, and 2000 is a sensible floor, with more doing no harm. And because a thinking model that is left to think spends that same budget before it starts answering, and plans slowly besides, `think=False` on the `OllamaModel` turns thinking off for this job.

```{note}
`scratch_notes` only names the environment variables; it never holds their values. The credentials stay in the environment of the process that runs the recipe, and a written function reads them from there with `os.environ`.
```

## Step 3: Launch

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.add_pkg(
    components=[vision, tts, cortex],
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
launcher.bringup()
```

The recipe expects the SMTP settings and the operator's address in the environment it is started from, so export them first:

```bash
export SMTP_HOST=mail.example.org SMTP_PORT=587 SMTP_USER=robot@example.org
export SMTP_PASSWORD=... ALERT_EMAIL=you@example.org
python3 recipe.py
```

## The first instruction: email when a person appears

Send the standing instruction as a goal, from a terminal or from the web interface:

```shell
ros2 action send_goal /cortex_input_command \
    automatika_embodied_agents/action/VisionLanguageAction \
    "{task: 'Whenever a person is detected, email me about it.'}"
```

The planner looks through its tools and finds the detections topic, the `tts-say` action and the event tools, but nothing that sends mail, so it writes an action. Exactly what it writes depends on the model, but the contract it has to keep does not: the function has typed parameters and a docstring, returns `(True, message)` or `(False, why)` and never raises, imports what it needs, and reads the environment through `os.environ`. It will look something like this:

```python
def email_operator(subject: str, body: str) -> tuple[bool, str]:
    """Email the operator at ALERT_EMAIL through the SMTP account in the environment.

    subject: The subject line
    body: The text of the message
    """
    import os
    import smtplib
    from email.message import EmailMessage

    message = EmailMessage()
    message["From"] = os.environ["SMTP_USER"]
    message["To"] = os.environ["ALERT_EMAIL"]
    message["Subject"] = subject
    message.set_content(body)
    try:
        with smtplib.SMTP(os.environ["SMTP_HOST"], int(os.environ["SMTP_PORT"])) as smtp:
            smtp.starttls()
            smtp.login(os.environ["SMTP_USER"], os.environ["SMTP_PASSWORD"])
            smtp.send_message(message)
    except Exception as e:
        return False, f"Could not send the email: {e}"
    return True, f"Emailed {message['To']}"
```

Cortex checks the source as soon as it arrives, and if it does not parse, does not keep the signature, has no docstring, or uses a name it did not import, the planner is told why and tries again. Once it passes, the log shows the source under `Written action 'email_operator'`, and the planner learns that its function is now the tool `scratch-email_operator`, with the docstring as the tool's description and the `subject: ...` and `body: ...` lines as the descriptions of its parameters.

The planner then installs the event, with a condition on the detections topic and the new tool as the action, and since the instruction says *whenever*, the event is kept rather than fired once:

```json
{
  "event_id": "person_detected_email",
  "conditions": [{"topic": "detections", "field": "labels", "operator": "contains", "value": "person"}],
  "actions": [{"tool": "scratch-email_operator", "arguments": {"subject": "Person detected", "body": "The camera has detected a person."}}],
  "once": false
}
```

From then on the event lives in the recipe's monitor and fires on its own each time a person shows up in the detections, whether or not a task is running, and like every standing instruction it outlives the task that installed it.

## The second instruction: the CPU temperature

```shell
ros2 action send_goal /cortex_input_command \
    automatika_embodied_agents/action/VisionLanguageAction \
    "{task: 'Whenever the CPU temperature goes above 80 degrees, say so out loud.'}"
```

This time the action exists, `tts-say`, but there is nothing to install an event on, because no topic carries the temperature. So the planner writes a condition instead: a function with no parameters that returns a bool, is quick enough to be polled, and returns `False` rather than raising when it cannot tell:

```python
def cpu_too_hot() -> bool:
    """Whether the CPU temperature is above 80 degrees Celsius."""
    try:
        with open("/sys/class/thermal/thermal_zone0/temp") as f:
            return int(f.read()) > 80000
    except OSError:
        return False
```

Since a written condition is not a topic, it is given to `add_event` as a `plugin_condition`, just like a condition that a plugin provides, such as a low battery, together with the rate at which it should be polled:

```json
{
  "event_id": "cpu_too_hot",
  "plugin_condition": {"name": "scratch-cpu_too_hot", "arguments": {"check_rate": 1.0}},
  "actions": [{"tool": "tts-say", "arguments": {"text": "The CPU is running hot."}}],
  "once": false
}
```

The monitor polls the function once a second and fires the event when its answer turns true, which is the same mechanism as a condition on a Python object in [Internal State Events](../events-and-resilience/internal-state-events.md), only with the planner writing the function. A condition that raises, returns something other than a bool, or takes longer than its polling period counts as not met, and the log says so once.

## Looking at what was written

The planner can list, read and remove its functions, and you can make it do so simply by asking: *"list the functions you have written"* makes it call `list_functions`, and the answer names each function with its kind and signature. Writing a name again replaces the function, though an event that was installed with the earlier version keeps that version until the event is removed and added again, and `remove_function` forgets a function without touching the events that use it.

A written action can also call the recipe's existing actions through `run_action("<tool name>", **arguments)`, which returns `(success, message)` like any other action, so a written function can build on what is already there, for instance by saying a message out loud before sending it. The one exception is goals to action servers, which a written function cannot send; the planner is told to plan such a goal as a step instead.

<!-- TODO screenshot: the web interface's main log while the first instruction runs: the written source, "Written action 'email_operator'", and the add_event call -->

## Complete code

```{code-block} python
:caption: Cortex writing its own actions and event conditions
:linenos:

from agents.clients import OllamaClient, RoboMLRESPClient
from agents.components import Cortex, TextToSpeech, Vision
from agents.config import CortexConfig, TextToSpeechConfig, VisionConfig
from agents.models import OllamaModel, TransformersTTS, VisionModel
from agents.ros import Launcher, Topic

# --- Model clients ---
# A thinking model left to think plans slowly and can use up its token budget
# before it answers
planner_client = OllamaClient(
    OllamaModel(name="qwen", checkpoint="qwen3.5:latest", think=False),
    inference_timeout=120,
)
detection_client = RoboMLRESPClient(
    VisionModel(name="rtdetr", checkpoint="PekingU/rtdetr_r50vd_coco_o365")
)
tts_client = RoboMLRESPClient(TransformersTTS(name="speecht5"))

# --- Vision: detections the planner can install events on ---
image_in = Topic(name="/image_raw", msg_type="Image")
detections_out = Topic(name="detections", msg_type="Detections")

vision = Vision(
    inputs=[image_in],
    outputs=[detections_out],
    model_client=detection_client,
    config=VisionConfig(threshold=0.5),
    trigger=0.5,
    component_name="vision",
)

# --- TextToSpeech: its `say` action is a tool for the planner ---
text_in = Topic(name="text_in", msg_type="String")

tts = TextToSpeech(
    inputs=[text_in],
    model_client=tts_client,
    config=TextToSpeechConfig(play_on_device=True),
    trigger=text_in,
    component_name="tts",
)

# --- Cortex: may write functions, and knows what the environment offers ---
cortex = Cortex(
    model_client=planner_client,
    config=CortexConfig(
        enable_events=True,  # standing instructions need runtime events
        enable_scratch_functions=True,  # read the warning above first
        scratch_notes=(
            "The process environment holds SMTP_HOST, SMTP_PORT, SMTP_USER and "
            "SMTP_PASSWORD, an SMTP server and account for sending mail, and "
            "ALERT_EMAIL, the operator's address. The CPU temperature in "
            "millidegrees Celsius can be read from "
            "/sys/class/thermal/thermal_zone0/temp."
        ),
        max_new_tokens=2000,  # a written function's source travels in one reply
    ),
    component_name="cortex",
)

launcher = Launcher()
launcher.add_pkg(
    components=[vision, tts, cortex],
    package_name="automatika_embodied_agents",
    multiprocessing=True,
)
launcher.bringup()
```

## Where next

- [Cortex](../../intelligence/cortex.md) -- the reference for standing instructions and for the functions the planner writes, including the full contract a function has to keep.
- [Events & Actions](../../concepts/events-and-actions.md) -- what an event can watch and what it can run, which is what the planner's `add_event` calls are built from.
- [Internal State Events](../events-and-resilience/internal-state-events.md) -- a polled condition on a Python object, written by hand; a written condition is the same thing written by the planner.
- [Event-Driven Cognition](../events-and-resilience/event-driven-cognition.md) -- the recipes where events wake expensive models only when needed, the pattern the first instruction reproduces at run time.

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
