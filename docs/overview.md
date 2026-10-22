# EMOS -- The Embodied Operating System

**The open-source runtime for Physical AI.**

EMOS turns a robot into a <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">Physical AI Agent</span>: one that sees, thinks, moves and adapts. The whole behavior lives in a single Python file called a <span class="text-red-strong">Recipe</span>, and the same Recipe runs on a wheeled AMR, a quadruped or a humanoid without a line changed.

:::{image} _static/images/diagrams/emos_any_robot_light.png
:align: center
:width: 100%
:class: light-only
:::

:::{image} _static/images/diagrams/emos_any_robot_dark.png
:align: center
:width: 100%
:class: dark-only
:::

<p align="center">
  <a class="sd-sphinx-override sd-btn sd-btn-primary sd-px-4 sd-mx-1" href="getting-started/installation.html">Install EMOS</a>
  <a class="sd-sphinx-override sd-btn sd-btn-outline-primary sd-px-4 sd-mx-1" href="why-emos.html">Why EMOS?</a>
  <a class="sd-sphinx-override sd-btn sd-btn-outline-primary sd-px-4 sd-mx-1" href="https://github.com/automatika-robotics/emos">View on GitHub</a>
</p>

---

## What You Can Build

Recipes cover every part of a Physical AI agent, from a conversation to a patrol.

::::{grid} 1 2 2 2
:gutter: 3

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`psychology;1.2em;sd-text-primary` Intelligent Agents</span>
Wire vision, language, speech and memory into **agentic workflows**. Route requests by intent, answer questions about the surroundings, build a semantic map of the site, or let a decision model judge what to do next, all from one Python script.

[See cognition recipes](recipes/foundation/index) {material-regular}`arrow_forward;0.9em`
:::

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`precision_manufacturing;1.2em;sd-text-primary` Planning & Manipulation</span>
**Cortex**, the agentic harness, turns a Recipe into an agent you talk to. It finds every capability the Recipe has, plans a goal as a sequence of tool calls, replans when one fails and writes the tools it is missing. **VLAs** handle end-to-end manipulation, with a VLM referee that stops an action once the task is done.

[See planning recipes](recipes/planning-and-manipulation/index) {material-regular}`arrow_forward;0.9em`
:::

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`route;1.2em;sd-text-primary` Autonomous Navigation</span>
**GPU-accelerated** planning and control for real-world mobility. Point-to-point navigation, multi-waypoint missions, path recording and vision-based following, across differential drive, Ackermann and omnidirectional platforms.

[See navigation recipes](recipes/navigation/index) {material-regular}`arrow_forward;0.9em`
:::

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`sync_alt;1.2em;sd-text-primary` Runtime Adaptivity</span>
**Event-driven** architecture lets agents reconfigure themselves at runtime. Hot-swap ML models on network failure, switch navigation algorithms when stuck, trigger recovery maneuvers from sensor events, or compose complex behaviors with logic gates.

[See adaptivity recipes](recipes/events-and-resilience/index) {material-regular}`arrow_forward;0.9em`
:::

::::

---

## A Recipe Is One Python File

This is a complete Physical AI agent. It sees through a camera, reasons with a vision-language model and answers in text, and it runs as a managed ROS 2 lifecycle node:

```python
from agents.clients.ollama import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel
from agents.ros import Topic, Launcher

text_in  = Topic(name="text0", msg_type="String")
image_in = Topic(name="image_raw", msg_type="Image")
text_out = Topic(name="text1", msg_type="String")

model  = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
client = OllamaClient(model)

vlm = VLM(
    inputs=[text_in, image_in],
    outputs=[text_out],
    model_client=client,
    trigger=text_in,
)

launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

A bigger Recipe is the same thing with more components wired together, and with the scenarios you want the robot to handle written next to them. The web UI comes for free, generated from the Recipe itself.

:::{figure} _static/images/diagrams/emos_recipe_graph_light.png
:align: center
:width: 100%
:figclass: light-only

Solid arrows show which component feeds which. Dashed red flags show what the robot should do when something happens.
:::

:::{figure} _static/images/diagrams/emos_recipe_graph_dark.png
:align: center
:width: 100%
:figclass: dark-only

Solid arrows show which component feeds which. Dashed red flags show what the robot should do when something happens.
:::

---

## What's Inside

EMOS is built on three open-source components:

| Component | Role |
| :--- | :--- |
| **[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents)** | <span class="text-red-strong" style="font-size: 1em;">Intelligence layer</span> -- perception, language, memory, Cortex and manipulation components |
| **[Kompass](https://github.com/automatika-robotics/kompass)** | <span class="text-red-strong" style="font-size: 1em;">Navigation layer</span> -- GPU-accelerated planning and control for any motion model |
| **[Sugarcoat](https://github.com/automatika-robotics/sugarcoat)** | <span class="text-red-strong" style="font-size: 1em;">Architecture layer</span> -- the components, events and launch API everything else is built on |

Every component in a Recipe comes from one of the three, and the `emos` CLI installs, updates and runs the lot.

---

::::{grid} 1 2 3 3
:gutter: 3

:::{grid-item-card} {material-regular}`lightbulb;1.2em;sd-text-primary` Why EMOS
:link: why-emos
:link-type: doc

The problem EMOS solves -- from custom R&D projects to universal, adaptive robot apps.
:::

:::{grid-item-card} {material-regular}`rocket_launch;1.2em;sd-text-primary` Getting Started
:link: getting-started/installation
:link-type: doc

Install EMOS and run your first Recipe in minutes.
:::

:::{grid-item-card} {material-regular}`menu_book;1.2em;sd-text-primary` Recipes & Tutorials
:link: recipes/overview
:link-type: doc

Build intelligent robot behaviors with step-by-step guides.
:::

:::{grid-item-card} {material-regular}`extension;1.2em;sd-text-primary` Plugins & Mapping
:link: getting-started/plugins
:link-type: doc

Put EMOS on your robot with a plugin, and map the site with the robot itself.
:::

:::{grid-item-card} {material-regular}`architecture;1.2em;sd-text-primary` Core Concepts
:link: concepts/architecture
:link-type: doc

Understand the architecture, components, events, and fallbacks.
:::

:::{grid-item-card} {material-regular}`dashboard;1.2em;sd-text-primary` Dashboard
:link: getting-started/dashboard
:link-type: doc

Zero-touch web console on the robot for browsing and running recipes.
:::

:::{grid-item-card} {material-regular}`terminal;1.2em;sd-text-primary` CLI & Deployment
:link: getting-started/cli
:link-type: doc

Package and run Recipes with the `emos` CLI.
:::

:::{grid-item-card} {material-regular}`smart_toy;1.2em;sd-text-primary` AI-Assisted Coding
:link: llms.txt

Get the `llms.txt` for your coding agent and let it write recipes for you.
:::

:::{grid-item-card} {material-regular}`code;1.2em;sd-text-primary` API Reference
:link: apidocs/index
:link-type: doc

Every component, client and model, documented from the source.
:::

::::
