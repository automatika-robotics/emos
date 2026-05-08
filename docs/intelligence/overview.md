# EmbodiedAgents

**The intelligence layer of EMOS --** <span class="text-red-strong">production-grade orchestration for Physical AI</span>

[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) enables you to create interactive, physical agents that do not just chat, but **understand**, **move**, **manipulate**, and **adapt** to their environment. It bridges the gap between foundation AI models and real-world robotic deployment, offering a structured yet flexible programming model for building adaptive intelligence.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`smart_toy;1.2em;sd-text-primary` Production-Ready Physical Agents</span> -- Designed for autonomous systems in dynamic, real-world environments. Components are built around ROS2 Lifecycle Nodes with deterministic startup, shutdown, and error-recovery. Health monitoring, fallback behaviors, and graceful degradation are built in from the ground up.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`autorenew;1.2em;sd-text-primary` Self-Referential and Event-Driven</span> -- Agents can start, stop, or reconfigure their own components based on internal and external events. Switch from cloud to local inference, swap planners based on vision input, or adjust behavior on the fly. In the spirit of [Godel machines](https://en.wikipedia.org/wiki/G%C3%B6del_machine), agents become capable of introspecting and modifying their own execution graph at runtime.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`hub;1.2em;sd-text-primary` Semantic Memory & Agentic Planning</span> -- Hierarchical spatio-temporal memory and semantic routing for arbitrarily complex agentic information flow. The graph-backed [Memory](memory.md) component keeps an episodic, entity-aware record of what the robot perceives *and* of its own internal state, while [Cortex](cortex.md) turns plain-language goals into ordered calls against every component in the graph -- no bloated GenAI frameworks required.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`code;1.2em;sd-text-primary` Pure Python, Native ROS2</span> -- Define complex asynchronous execution graphs in standard Python without touching XML launch files. Underneath, everything is pure ROS2 -- fully compatible with the entire ecosystem of hardware drivers, simulation tools, and visualization suites.

## What You Can Build

::::{grid} 1 2 3 3
:gutter: 3

:::{grid-item-card} {material-regular}`record_voice_over;1.2em;sd-text-primary` Robots You Hold a Conversation With
:link: ../recipes/foundation/conversational-agent
:link-type: doc

Robots that listen, see, and speak -- microphone in, visually-grounded answer out, all in one Python recipe. Ask *"what's on the table?"* and get a real answer in real time.
:::

:::{grid-item-card} {material-regular}`alt_route;1.2em;sd-text-primary` Robots That Pick the Right Brain
:link: ../recipes/foundation/semantic-routing
:link-type: doc

One sentence in, the right capability fires. *"How tall is Everest?"* wakes the LLM. *"What do you see?"* wakes the VLM. *"Take me to the kitchen."* dispatches the navigation stack. Behavior emerges from intent.
:::

:::{grid-item-card} {material-regular}`precision_manufacturing;1.2em;sd-text-primary` Robots That Pick Up What You Mean
:link: ../recipes/planning-and-manipulation/vla-manipulation
:link-type: doc

A robot arm that grabs *"the red mug next to the laptop"* without you writing a state machine for which mug. A VLM grounds the description; a VLA model translates straight to joint commands.
:::

:::{grid-item-card} {material-regular}`memory;1.2em;sd-text-primary` Robots That Remember
:link: ../recipes/foundation/semantic-map
:link-type: doc

Every detection, every scene caption, every internal reading is folded into a graph indexed by *meaning*, *place*, and *time* -- and persists across reboots. The robot starts knowing your space the way you do.
:::

:::{grid-item-card} {material-regular}`smart_toy;1.2em;sd-text-primary` Robots You Give Missions To
:link: ../recipes/planning-and-manipulation/cortex-agent
:link-type: doc

Drop a [Cortex](cortex.md) component into your recipe and your robot starts running *missions*, not commands. *"Patrol the workshop and tell me if any lights are off."* Cortex auto-discovers every capability as an LLM tool, plans the steps, dispatches them, watches feedback, and replans on failure -- with no orchestration code from you.
:::

:::{grid-item-card} {material-regular}`route;1.2em;sd-text-primary` Robots That Reason About the World
:link: ../recipes/planning-and-manipulation/cortex-navigation
:link-type: doc

Compound goals like *"go to the kitchen and tell me what's on the counter"* fall out of a single recipe. The robot recalls where the kitchen is from memory, navigates there with Kompass, looks at the counter, narrates the answer. End-to-end embodied reasoning, no behavior trees.
:::

::::

## Next Steps

- {material-regular}`widgets;1.2em;sd-text-primary` {doc}`ai-components` -- The core building blocks: components and topics.
- {material-regular}`psychology;1.2em;sd-text-primary` {doc}`cortex` -- The agentic planner-executor that drives the rest of the graph from natural-language goals.
- {material-regular}`memory;1.2em;sd-text-primary` {doc}`memory` -- Graph-backed spatio-temporal memory with perception and interoception layers.
- {material-regular}`cloud;1.2em;sd-text-primary` {doc}`clients` -- How inference backends connect to components.
- {material-regular}`model_training;1.2em;sd-text-primary` {doc}`models` -- Available model wrappers and vector databases.
