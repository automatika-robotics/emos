# Cortex

**The agentic harness for embodied intelligence.** Cortex is the EmbodiedAgents component that turns the rest of your recipe into an agent: it discovers every component you added, registers their methods as LLM tools, and lets the user address the whole system in plain language. If [Claude Code](https://claude.com/claude-code) is an agentic harness for software engineering, Cortex is its analogue for robots -- the same primitives (read the environment, plan, dispatch tools, watch results, replan) applied to a physical system.

A recipe with a Cortex stops being a programmed pipeline and starts being something you talk to.

```{seealso}
For the introductory walkthrough, start with [Cortex: The Agentic Harness](../recipes/planning-and-manipulation/cortex-agent.md). For Cortex paired with spatio-temporal memory, see [Memory and Cortex](../recipes/planning-and-manipulation/cortex-memory.md). For the full multi-system showcase that adds navigation on top, see [Cortex Driving the Full Stack](../recipes/planning-and-manipulation/cortex-navigation.md).
```

---

## What Cortex replaces

A non-Cortex EMOS recipe earns each capability by hand-wiring it: a vision component publishes detections, an event matches the detection class, a fallback restarts the camera if it stalls, a separate LLM component parses the user's input into structured goals, and so on. Every link is yours to author and maintain.

A Cortex recipe is the same components -- minus the wiring. Cortex inspects the running graph at activation, registers every available capability as a callable tool, and accepts the user's intent directly:

| Without Cortex                                                           | With Cortex                                                                                                                   |
| ------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| One event-action pair per behaviour, hand-wired in the recipe.           | The recipe has no behavioural wiring. Cortex plans the behaviour at runtime from the user's goal.                             |
| Each capability needs an explicit handler that knows when to trigger it. | Each capability is a `@component_action` on its component. Cortex discovers them all on activation.                           |
| User input is parsed by a bespoke LLM step into a typed goal.            | User input is a free-form string sent to Cortex's main action server.                                                         |
| Fallback policies wired per component.                                   | Cortex's confirmation step (EXECUTE / SKIP / ABORT / CONTINUE) handles per-step recovery; replan handles plan-level recovery. |
| You write the orchestration code.                                        | You write the components. Cortex writes the recipe.                                                                           |

---

## How it works

Cortex runs a **two-phase loop** for every task.

### Phase 1 -- Planning (multi-step)

The planner LLM is handed two tool sets:

- **Planning tools** -- read-only research tools. The built-in `inspect_component` plus any `@component_action(phase=ActionPhase.PLANNING)` methods on managed components.
- **Execution tools** -- everything that _does_ something. Custom `Action` objects, `@component_action(phase=ActionPhase.EXECUTION)` methods, action-server goal tools, service-request tools, and the built-in `update_parameter`.

On each iteration, the LLM may:

1. Call **planning tools** to gather information -- inspect a component, query memory, look up a fact in a vector DB. Results are appended to the conversation and the loop continues.
2. Call **execution tools** -- this commits a plan as an ordered list of steps, and the loop ends.
3. Respond with **text only** -- no actions needed; the text is published on `output` and the task is done.

Up to `max_planning_steps` iterations of research are allowed before the planner must commit. Plans longer than `max_execution_steps` are truncated.

### Phase 2 -- Execution with confirmation

Each step is dispatched in turn. **Before** each one, a brief confirmation LLM call returns one of:

| Decision   | Effect                                                                                                                                                                                      |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `EXECUTE`  | Run the next step. The confirmation may also return a tool call with **resolved arguments** -- e.g. binding a placeholder like `<output from step 1>` to the actual return value of step 1. |
| `SKIP`     | Skip this step and continue.                                                                                                                                                                |
| `ABORT`    | Abort the entire plan.                                                                                                                                                                      |
| `CONTINUE` | Wait for in-flight async actions to finish before deciding.                                                                                                                                 |

`CONTINUE` is the key to long-horizon tasks. When Cortex dispatches an action goal (e.g. to a Planner action server), it tracks the client in `_active_action_clients` and the confirmation prompt includes the action's live feedback. The LLM can `CONTINUE` to wait, watch how the goal is progressing, and only `EXECUTE` the next step when the action reports SUCCEEDED.

### Replan on incomplete execution

If the plan exits before reaching its terminal step (because a step `ABORT`-ed, or because the executor ran out of steps with goals still in flight), Cortex composes a fresh plan from where it left off, with the partial-execution results fed back into the planning conversation. Long-horizon tasks ("patrol until you see a person") express naturally: each replan is one outer iteration.

---

## What gets auto-discovered

When the launcher activates Cortex, it walks every component in the recipe and registers tools for everything it finds. **You write none of this registration -- it happens once on activation.**

### Built-in tools

| Tool                                                 | Phase     | Purpose                                                                                                                                                                                                         |
| ---------------------------------------------------- | --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `inspect_component(component)`                       | Planning  | Returns the component's full structure: input/output topics, config, additional model clients, and the actions Cortex has registered for it. The planner uses this to discover topic names and ground its plan. |
| `update_parameter(component, param_name, new_value)` | Execution | Re-tunes any config parameter on any managed component at runtime. The LLM can lower `Vision.threshold` mid-task or flip `Controller.direct_sensor` if the mission demands it.                                  |

### Component capabilities

For every managed component, Cortex discovers:

- **`@component_action` methods** -- registered as namespaced tools `{component_name}.{method_name}` with the OpenAI-format description from the decorator. Classified as planning, execution, or both via the `phase=` argument:

  ```python
  from agents.ros import ActionPhase, component_action

  class MyComponent(BaseComponent):
      @component_action(description={...}, phase=ActionPhase.PLANNING)
      def look_up_thing(self): ...    # planner-only research tool

      @component_action(description={...}, phase=ActionPhase.EXECUTION)
      def grasp(self): ...             # executor-only state-changing action

      @component_action(description={...}, phase=ActionPhase.BOTH)
      def describe_scene(self): ...    # both phases
  ```

  Bare `@component_action` defaults to `ActionPhase.EXECUTION`, preserving historical behaviour.

- **`@component_fallback` methods** -- registered the same way, exposed as recovery tools the planner can fall back to.

- **Additional ROS services** (returned by `get_ros_entrypoints()["services"]`) -- registered as `send_request_to_{name}` execution tools. The request type is auto-translated to JSON properties so the LLM fills request fields directly; Cortex constructs the message and sends it.

- **Additional ROS action servers** (returned by `get_ros_entrypoints()["actions"]`) -- registered as `send_goal_to_{name}` execution tools, with the goal type auto-translated the same way. The Controller's `track_vision_target` action server is auto-discovered this way, for instance.

- **The component's main action server** (when `run_type = "ActionServer"`) -- registered the same way. Setting `Planner.run_type = "ActionServer"` is what makes the Planner's main goal callable as an LLM tool.

### Custom actions

Capabilities that don't naturally live on a managed component (a peripheral toggle, a database call, an external API hit) can be passed in via `actions=[...]`:

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

Each `Action` must carry a description -- it's what the planner sees when deciding whether to call the tool.

---

## Public API

```python
from agents.components import Cortex
from agents.config import CortexConfig
from agents.ros import Action, Topic, Launcher
```

```python
cortex_output = Topic(name="cortex_output", msg_type="StreamingString")

cortex = Cortex(
    actions=[Action(method=toggle_led, description="...")],
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

| Argument       | Purpose                                                                                                                                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `actions`      | Custom `Action` objects to expose as execution tools. Optional.                                                                                                                                                                |
| `output`       | A topic Cortex publishes to when the goal can be answered with text alone (no plan needed). Wire it into TTS to give the robot a voice for its own thoughts, or into the UI to render replies inline.                          |
| `model_client` | The LLM client used for planning and confirmation. Optional if `enable_local_model=True`.                                                                                                                                      |
| `db_client`    | Optional vector-DB client (e.g. `ChromaClient`). When set, Cortex queries it before each planning call and injects the result as RAG context. Used for domain knowledge: robot manuals, environment maps, prior conversations. |
| `config`       | A `CortexConfig`. Most-tuned fields are `max_planning_steps`, `max_execution_steps`, `enable_local_model`, `enable_rag`, `collection_name`, and `confirmation_temperature`.                                                    |

Cortex always runs as a ROS2 **action server** -- you don't configure `run_type`. Its action type is `VisionLanguageAction` and the goal field is `task: string`.

---

## Cortex is also the Monitor

When the launcher detects a Cortex component in the recipe, it is **also used as the Monitor** -- the central node that hosts events and actions, tracks every component's health, manages lifecycle transitions, and coordinates fallbacks. Practical implications:

This is why Cortex needs no list of components to monitor -- the launcher feeds it the full graph.

---

## Memory-aware planning

When a [Memory](memory.md) component is in the recipe, Cortex's planning prompt is **automatically augmented** on activation. The augmentation:

- Lists Memory's perception-retrieval tools (`semantic_search`, `locate`, `recall`, ...) and its body-status tool, so the planner knows the difference between perception and interoception layers.
- Pre-computes Memory's `inspect_component` output and embeds it in the system prompt, so the planner already knows the layer names without having to spend a planning step researching.
- Adds a **task classification** step: every incoming task is assigned to (A) PERCEPTION QUERY, (B) BODY QUERY, or (C) ACTION TASK. Each class has a specific protocol (perception queries don't open episodes; action tasks always wrap themselves in `start_episode` / `end_episode` and check `body_status` first).
- Body-status checks become **mandatory** for action tasks, so a Cortex with an interoception layer (battery, fault flags) might refuse missions when the readings indicate a problem rather than running them and discovering the issue mid-flight.

No extra wiring. Drop a Memory component into the launcher and the augmentation kicks in. See [Memory and Cortex](../recipes/planning-and-manipulation/cortex-memory.md) for the full pattern.

---

## Observability

Cortex publishes feedback on its action server during execution. Each plan step generates a feedback line carrying:

- Step number and tool name.
- Whether the step is being EXECUTE-d, SKIP-ped, ABORT-ed, or whether confirmation said CONTINUE.
- Live status of any action goals in flight (running for _N_ seconds, latest feedback, stall warnings).

The launcher's Web UI shows these feedback lines in the **main logging card** alongside the components' own logs, so the operator sees the agent's reasoning trace and the Planner's path-tracking feedback side by side.

---

## RAG context

Setting `db_client` on Cortex enables retrieval-augmented planning. Before each planning call, Cortex queries the configured vector DB with the user's task and prepends the results to the planning prompt:

```python
from agents.clients import ChromaClient

cortex = Cortex(
    output=cortex_output,
    model_client=planner_client,
    db_client=ChromaClient(host="localhost", port=8000),
    config=CortexConfig(
        enable_rag=True,
        collection_name="building_layout",
        n_results=5,
        add_metadata=True,
    ),
    component_name="cortex",
)

# Populate the DB with domain knowledge ahead of time:
cortex.add_documents(
    ids=["floor1", "floor2"],
    metadatas=[{"floor": 1}, {"floor": 2}],
    documents=[
        "Floor 1 contains the kitchen, dining room, and main entrance.",
        "Floor 2 contains the bedrooms and the office.",
    ],
)
```

Use this for static facts the planner shouldn't have to learn from the live recipe -- robot manuals, environment maps, prior conversation transcripts. For _dynamic_ facts the robot acquires at runtime, use [Memory](memory.md).

---

## Recipes

- {doc}`Cortex: The Agentic Harness <../recipes/planning-and-manipulation/cortex-agent>` -- introductory tutorial. Vision + VLM + TTS + a custom action, all addressed in plain English with no orchestration code.
- {doc}`Cortex Driving the Full Stack <../recipes/planning-and-manipulation/cortex-navigation>` -- the showcase. Cortex orchestrates a Kompass navigation stack, Vision, VLM, Memory, and TTS to handle compound natural-language goals.
- {doc}`Memory and Cortex <../recipes/planning-and-manipulation/cortex-memory>` -- spatio-temporal memory wired into a Cortex planner; perception layers + interoception layers; episode-based consolidation; cross-session persistence.
