# EmbodiedAgents

**The intelligence layer of EMOS --** <span class="text-red-strong">production-grade orchestration for Physical AI</span>

[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents) enables you to create interactive, physical agents that do not just chat, but **understand**, **move**, **manipulate**, and **adapt** to their environment. It bridges the gap between foundation AI models and real-world robotic deployment, offering a structured yet flexible programming model for building adaptive intelligence.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`smart_toy;1.2em;sd-text-primary` Production-Ready Physical Agents</span> -- Designed for autonomous systems in dynamic, real-world environments. Components are built around ROS2 Lifecycle Nodes with deterministic startup, shutdown, and error-recovery. Health monitoring, fallback behaviors, and graceful degradation are built in from the ground up.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`autorenew;1.2em;sd-text-primary` Self-Referential and Event-Driven</span> -- Agents can start, stop, or reconfigure their own components based on internal and external events. Switch from cloud to local inference, swap planners based on vision input, or adjust behavior on the fly. In the spirit of [Godel machines](https://en.wikipedia.org/wiki/G%C3%B6del_machine), agents become capable of introspecting and modifying their own execution graph at runtime.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`hub;1.2em;sd-text-primary` Semantic Memory</span> -- Hierarchical spatio-temporal memory and semantic routing for arbitrarily complex agentic information flow. Components like MapEncoding and SemanticRouter let robots maintain structured, queryable representations of their environment over time -- no bloated GenAI frameworks required.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`code;1.2em;sd-text-primary` Pure Python, Native ROS2</span> -- Define complex asynchronous execution graphs in standard Python without touching XML launch files. Underneath, everything is pure ROS2 -- fully compatible with the entire ecosystem of hardware drivers, simulation tools, and visualization suites.

## What You Can Build

::::{grid} 1 2 2 2
:gutter: 3

:::{grid-item-card} {material-regular}`chat;1.2em;sd-text-primary` Conversational Robots
:link: ../recipes/foundation/conversational-agent
:link-type: doc

Speech-to-text, LLM reasoning, and text-to-speech pipelines for natural dialogue.
:::

:::{grid-item-card} {material-regular}`precision_manufacturing;1.2em;sd-text-primary` Vision-Guided Manipulation
:link: ../recipes/planning-and-manipulation/vla-manipulation
:link-type: doc

VLMs for high-level planning and VLAs for end-to-end motor control.
:::

:::{grid-item-card} {material-regular}`map;1.2em;sd-text-primary` Semantic Navigation
:link: ../recipes/foundation/goto-navigation
:link-type: doc

Map encoding and spatio-temporal memory for context-aware movement.
:::

:::{grid-item-card} {material-regular}`alt_route;1.2em;sd-text-primary` Multi-Modal Agents
:link: ../recipes/foundation/semantic-routing
:link-type: doc

Dynamically route information between perception, reasoning, and action based on semantic content.
:::

::::

## Next Steps

- {material-regular}`widgets;1.2em;sd-text-primary` {doc}`ai-components` -- The core building blocks: components and topics.
- {material-regular}`cloud;1.2em;sd-text-primary` {doc}`clients` -- How inference backends connect to components.
- {material-regular}`model_training;1.2em;sd-text-primary` {doc}`models` -- Available model wrappers and vector databases.
