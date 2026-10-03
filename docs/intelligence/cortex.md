# Cortex

Cortex is the component that turns the rest of a recipe into an agent. Every action, action server, service, routine and plugin action the recipe has becomes a tool it can call, and a goal in plain language becomes a plan of those calls, carried out while it watches the results. If an agentic harness is what lets a language model work a codebase, Cortex is the same idea applied to a robot: read the environment, plan, dispatch tools, watch what happens, replan.

A recipe with a Cortex stops being a programmed pipeline and becomes something you talk to.

```{seealso}
For the introductory walkthrough, start with [Cortex: The Agentic Harness](../recipes/planning-and-manipulation/cortex-agent.md). For Cortex paired with spatio-temporal memory, see [Memory and Cortex](../recipes/planning-and-manipulation/cortex-memory.md), and for the full showcase with navigation on top, [Cortex Driving the Full Stack](../recipes/planning-and-manipulation/cortex-navigation.md).
```

---

## What Cortex replaces

A recipe without Cortex earns each behaviour by wiring it: a vision component publishes detections, an event matches a class, an LLM step parses what the user said into a typed goal, and every link is yours to write and keep working. A recipe with Cortex has the same components and none of the wiring:

| Without Cortex                                                           | With Cortex                                                                                                                   |
| :----------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------------- |
| One event and action pair per behaviour, wired in the recipe.            | No behavioural wiring. Cortex plans the behaviour at run time from the goal it is given.                                       |
| Each capability needs a handler that knows when to trigger it.           | Each capability is an action on its component, and Cortex finds them all when it activates.                                   |
| User input goes through a bespoke LLM step into a typed goal.            | User input is a sentence sent to Cortex's action server.                                                                      |
| Recovery is wired per component.                                         | A confirmation before each step handles the step, and replanning handles the plan.                                            |
| You write the orchestration.                                             | You write the components. Cortex does the orchestration.                                                                      |

---

## How it works

Every task runs through the same loop: plan, then execute with a check before each step, then replan if the plan did not get to the end.

### Planning

The planner is given two sets of tools. Planning tools are for research and change nothing: `inspect_component`, any component action marked for the planning phase, and, when events are enabled, `list_events`. Execution tools are everything that does something: component actions, goals to action servers, service requests, routines, plugin actions, custom actions, and Cortex's own `update_parameter` and `wait`.

On each iteration the model either calls planning tools to learn what it needs, which appends the results to the conversation and continues, or calls execution tools, which commits the plan as an ordered list of steps and ends planning, or answers in text alone, which is published on `output` and ends the task. Up to `max_planning_steps` rounds of research are allowed before the planner has to commit, and a plan longer than `max_execution_steps` is cut there.

### Execution

Steps are dispatched in order. Before each one, a short confirmation call decides what happens to it:

| Decision   | Effect                                                                                                                                            |
| :--------- | :------------------------------------------------------------------------------------------------------------------------------------------------ |
| `EXECUTE`  | Run the step. The confirmation may also fill in arguments, for instance binding a placeholder like `<output from step 1>` to what step 1 returned. |
| `SKIP`     | Leave this step out and go on.                                                                                                                    |
| `ABORT`    | Stop the plan.                                                                                                                                    |
| `CONTINUE` | Wait for goals still in flight before deciding.                                                                                                   |

`CONTINUE` is what makes long tasks work. When a step sends a goal to an action server and does not wait for it, Cortex keeps reporting the goal's status and latest feedback to the model, which can hold until the goal succeeds before moving on.

Consecutive steps that need no such decisions are not run one at a time. Two or more component actions, plugin actions, awaited goals and waits in a row, with all their arguments known, are compiled into one [routine](../concepts/routines.md) hosted by the Monitor and run as a unit, with each step's message reported back as its result. A step that depends on an earlier result, a service call, or a routine tool breaks the run and goes through confirmation as before. `compile_routines=False` turns this off, and `step_timeout` bounds each compiled step.

### Replanning

If the plan ends before its last step, because a step was aborted or a goal was still running when the steps ran out, Cortex plans again from where it stopped, with the results so far in the conversation. A task like "patrol until you see a person" is a sequence of such rounds.

---

## What it can call

Cortex does not search the recipe itself. The launcher builds a registry of everything in the recipe that can be asked to do something by name, from every component in every process, and hands it to Cortex, which turns the entries into tools when it activates. You register nothing.

| Source                                          | Tool                                                                                                                                                                  |
| :---------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A component action or fallback with a description | `<component>-<method>`, for example `vision-take_picture` or `tts-say`. The description is what the planner reads, so a method without one is not offered.          |
| A component's action server                     | `send_goal_to_<component>_<server>`, with the goal's fields as parameters and a `wait_to_finish` flag, true by default. Both the main server of a component run as an action server and any it reports besides. |
| A component's service                           | `send_request_to_<component>_<service>`, the same way.                                                                                                                |
| A plugin's actions                              | `<plugin id>-<action>`, for example `lite3-stand_up`, with the plugin's own description.                                                                             |
| A routine the Monitor hosts                     | `routine-<name>`, plus `pause_routine`, `resume_routine` and `abort_routine`.                                                                                        |
| Cortex itself                                   | `inspect_component` for planning, and `update_parameter` and `wait` for execution.                                                                                   |

Lifecycle methods such as `start`, `stop` and `restart` are left out, since the Monitor manages those, and so are Cortex's own actions.

Which phase a component action belongs to is set in its decorator. The default is execution; `phase=ActionPhase.PLANNING` makes it a research tool, and `ActionPhase.BOTH` offers it in both phases, which suits retrieval such as `describe` or `locate`:

```python
from agents.ros import ActionPhase, ActionReturnType, component_action

class MyComponent(BaseComponent):
    @component_action(description={...}, phase=ActionPhase.BOTH)
    def locate(self, name: str) -> ActionReturnType: ...
```

A component's action server takes one goal at a time. When Cortex sends a goal to a server that is still running one of its own, it cancels that goal, waits for the server to return, and sends the new one. A goal that some other client started is not touched: the planner is told the server is busy and shown the component's `cancel_main_goal` tool, so stopping it is its decision.

Capabilities that live outside any component, a light to toggle, a database to query, an external API to call, are passed in as custom actions, each with a description:

```python
from agents.ros import Action

cortex = Cortex(
    actions=[
        Action(method=toggle_led, description="Toggle the robot's LED on or off."),
        Action(method=query_inventory, description="Look up the current item inventory."),
    ],
    ...,
)
```

When a robot plugin is attached, the plugin's actions arrive through the same registry, and what the robot is and who makes it is put in front of the planner, as are the sensor plugins attached to the recipe. [Robot Plugins](../concepts/robot-plugins.md) covers what a plugin brings.

---

## Routines as skills

A [routine](../concepts/routines.md) hosted by the Monitor is a skill the planner can use: one tool that runs a whole procedure with its own success tests and retries. Routines reach the Monitor when an event triggers them, when they are passed to `enable_ui`, or when they are given to Cortex directly, which needs neither:

```python
cortex = Cortex(routines=[pick_object, patrol], ...)
```

A routine given to Cortex needs a `description`, since that is what the planner reads. Starting one returns at once; Cortex then follows its progress and reports its status, active step and last message to the planner until it completes, fails or is aborted. Routines a task started are aborted when the task ends, as its running goals are cancelled.

---

## Standing instructions

"Whenever the battery is low, go to the dock" is not a step in a plan but an event, and Cortex can install one. The tools for it are off by default because they are the most involved it offers; `CortexConfig(enable_events=True)` turns them on. The planner then has `add_event`, `remove_event` and `list_events`, and adds an event with an id, one or more conditions on the fields of a topic a component reads or writes, joined by all or any, the tool calls to run, and whether it fires once or every time. Instead of topic conditions an event can name a condition a plugin offers, such as a low battery with its threshold. A topic or field that does not exist is refused before anything is installed. Events outlive the task that installed them.

---

## Public API

```python
from agents.components import Cortex
from agents.config import CortexConfig
from agents.ros import Action, Topic

cortex_output = Topic(name="cortex_output", msg_type="StreamingString")

cortex = Cortex(
    actions=[Action(method=toggle_led, description="...")],
    routines=[patrol],
    output=cortex_output,
    model_client=planner_client,
    db_client=chroma_client,                          # optional: enables RAG context
    config=CortexConfig(
        max_planning_steps=5,
        max_execution_steps=15,
        enable_rag=True,
        collection_name="robot_manual",
    ),
    component_name="cortex",
)
```

| Argument       | Purpose                                                                                                                                                                   |
| :------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `actions`      | Custom actions to offer as execution tools.                                                                                                                               |
| `routines`     | Routines to host and offer as skills.                                                                                                                                     |
| `output`       | The topic Cortex publishes to when a goal can be answered in text alone. Wire it into TextToSpeech to give the robot a voice, or into the web interface to show replies.  |
| `model_client` | The client of the model used for planning and confirmation. Optional with `enable_local_model=True`.                                                                      |
| `db_client`    | A vector database client. When set, Cortex queries it before each planning call and puts the result in front of the planner.                                              |
| `config`       | A `CortexConfig`, which extends `LLMConfig`.                                                                                                                              |

| `CortexConfig` field         | Default | Effect                                                                     |
| :--------------------------- | :------ | :------------------------------------------------------------------------- |
| `max_planning_steps`         | 10      | Rounds of research before the planner must commit.                         |
| `max_execution_steps`        | 10      | The longest plan that is run.                                              |
| `confirmation_temperature`   | 0.3     | Sampling temperature of the confirmation call.                             |
| `confirmation_max_tokens`    | 500     | Length budget of the confirmation call.                                    |
| `monitoring_interval`        | 2.0 s   | How often running goals and routines are reported to the planner.          |
| `compile_routines`           | true    | Run consecutive compilable steps as one routine.                           |
| `step_timeout`               | 60 s    | Time allowed for a component or plugin action inside a compiled run.       |
| `enable_events`              | false   | Offer the standing-instruction tools.                                      |

The fields of `LLMConfig` apply too, among them `enable_local_model`, `enable_rag`, `collection_name`, `n_results`, `temperature` and `max_new_tokens`, whose default is 512.

Cortex always runs as an action server, so `run_type` is not yours to set. Its action type is `VisionLanguageAction` and the goal carries the task as a string.

---

## Cortex is also the Monitor

When a recipe contains a Cortex, the launcher installs it in place of the default [Monitor](../concepts/launcher.md#the-monitor). It hosts the recipe's events, actions and routines, holds the lifecycle clients of every component, and serves the runtime API, on top of being the agent. That is why it needs no list of components to work with: the launcher gives it the whole recipe.

---

## Memory-aware planning

When a [Memory](memory.md) component is in the recipe, Cortex's planning prompt is extended on activation, without any wiring:

- Memory's retrieval tools, `semantic_search`, `locate`, `recall` and the rest, and its body-status tool are listed, so the planner knows the difference between what the robot perceived and what it felt.
- Memory's `inspect_component` output is computed ahead and put in the prompt, so the planner knows the layer names without spending a planning step on them.
- Every task is first classified as a perception query, a body query or an action task, each with its own protocol. A perception query opens no episode; an action task wraps itself in `start_episode` and `end_episode` and checks the body status first.
- The body-status check is mandatory for action tasks, so a Cortex with an interoception layer, battery or fault flags, can decline a mission on the readings rather than discover the problem half way.

[Memory and Cortex](../recipes/planning-and-manipulation/cortex-memory.md) shows the pattern.

---

## Observability

Cortex publishes feedback on its action server as it works: each step with its tool name, whether it was executed, skipped or aborted or is waiting, the status and latest feedback of goals in flight with stall warnings, the progress of routines it started, and the results of a compiled run. The recipe's [web interface](../concepts/web-ui.md) shows these lines in the main log next to the components' own, so an operator sees the agent's reasoning and the planner's path-tracking feedback side by side.

---

## RAG context

With a `db_client`, Cortex queries a vector database with the user's task before each planning call and prepends what it finds:

```python
from agents.clients import ChromaClient
from agents.vectordbs import ChromaDB

cortex = Cortex(
    output=cortex_output,
    model_client=planner_client,
    db_client=ChromaClient(db=ChromaDB(), host="localhost", port=8000),
    config=CortexConfig(
        enable_rag=True,
        collection_name="building_layout",
        n_results=5,
        add_metadata=True,
    ),
    component_name="cortex",
)

# Populate the database ahead of time
cortex.add_documents(
    ids=["floor1", "floor2"],
    metadatas=[{"floor": 1}, {"floor": 2}],
    documents=[
        "Floor 1 contains the kitchen, dining room, and main entrance.",
        "Floor 2 contains the bedrooms and the office.",
    ],
)
```

This is for facts the planner should not have to learn from the running recipe: robot manuals, building layouts, earlier conversations. What the robot learns as it runs belongs in [Memory](memory.md).

---

## Recipes

- {doc}`Cortex: The Agentic Harness <../recipes/planning-and-manipulation/cortex-agent>`, the introductory tutorial. Vision, a VLM, speech and a custom action, addressed in plain English with no orchestration code.
- {doc}`Cortex Driving the Full Stack <../recipes/planning-and-manipulation/cortex-navigation>`, the showcase. Cortex orchestrates a Kompass navigation stack, Vision, a VLM, Memory and speech for compound goals.
- {doc}`Memory and Cortex <../recipes/planning-and-manipulation/cortex-memory>`, spatio-temporal memory wired into the planner, with perception and interoception layers, episodes and persistence across sessions.
