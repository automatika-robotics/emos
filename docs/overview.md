# EMOS -- The Embodied Operating System

**The open-source unified orchestration layer for Physical AI.**

EMOS transforms robots into <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">Physical AI Agents</span>. It provides a hardware-agnostic runtime that lets robots **see**, **think**, **move**, and **adapt** -- all orchestrated from pure Python scripts called <span class="text-red-strong">Recipes</span>.

Write a _Recipe_ once, deploy it on any robot -- from wheeled AMRs to humanoids -- without rewriting code.

:::{image} _static/images/diagrams/emos_robot_stack_light.png
:align: center
:width: 70%
:class: light-only
:::

:::{image} _static/images/diagrams/emos_robot_stack_dark.png
:align: center
:width: 70%
:class: dark-only
:::

<br/>

<p align="center">
  <a href="./getting-started/installation.md">Get Started</a> •
  <a href="./why-emos.md">Why EMOS?</a> •
  <a href="https://github.com/automatika-robotics/emos">View on GitHub</a>
</p>

---

## What You Can Build

::::{grid} 1 2 2 2
:gutter: 3

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`psychology;1.2em;sd-text-primary` Intelligent Agents</span>
Wire together vision, language, speech, and memory components into **agentic workflows**. Route queries by intent, answer questions about the environment, or build a semantic map -- all from a single Python script.

[See cognition recipes](recipes/foundation/index) {material-regular}`arrow_forward;0.9em`
:::

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`route;1.2em;sd-text-primary` Autonomous Navigation</span>
**GPU-accelerated** planning and control for real-world mobility. Point-to-point navigation, path recording, and vision-based target following -- across differential drive, Ackermann, and omnidirectional platforms.

[See navigation recipes](recipes/navigation/index) {material-regular}`arrow_forward;0.9em`
:::

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`sync_alt;1.2em;sd-text-primary` Runtime Adaptivity</span>
**Event-driven** architecture lets agents reconfigure themselves at runtime. Hot-swap ML models on network failure, switch navigation algorithms when stuck, trigger recovery maneuvers from sensor events, or compose complex behaviors with logic gates.

[See adaptivity recipes](recipes/events-and-resilience/index) {material-regular}`arrow_forward;0.9em`
:::

:::{grid-item-card} <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`precision_manufacturing;1.2em;sd-text-primary` Planning & Manipulation</span>
Use **VLMs** for high-level task decomposition and **VLAs** for end-to-end manipulation. Closed-loop control where a VLM referee stops actions on visual task completion.

[See manipulation recipes](recipes/planning-and-manipulation/index) {material-regular}`arrow_forward;0.9em`
:::

::::

---

## What's Inside

EMOS is built on three open-source components:

| Component | Role |
| :--- | :--- |
| **[EmbodiedAgents](https://github.com/automatika-robotics/embodied-agents)** | <span class="text-red-strong" style="font-size: 1em;">Intelligence layer</span> -- agentic graphs of ML models with semantic memory and event-driven reconfiguration |
| **[Kompass](https://github.com/automatika-robotics/kompass)** | <span class="text-red-strong" style="font-size: 1em;">Navigation layer</span> -- GPU-powered planning and control for real-world mobility |
| **[Sugarcoat](https://github.com/automatika-robotics/sugarcoat)** | <span class="text-red-strong" style="font-size: 1em;">Architecture layer</span> -- event-driven system primitives and imperative launch API |

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

:::{grid-item-card} {material-regular}`architecture;1.2em;sd-text-primary` Core Concepts
:link: concepts/architecture
:link-type: doc

Understand the architecture, components, events, and fallbacks.
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

::::
