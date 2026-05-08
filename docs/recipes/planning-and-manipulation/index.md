# Planning & Manipulation Overview

Bridge the gap between high-level reasoning and physical actuation. These recipes cover [Cortex](../../intelligence/cortex.md) -- the EMOS planner-executor that decomposes goals into ordered tool calls, dispatches them, and replans on failure, all on top of capability components you wrote in isolation -- alongside VLM-based planning and VLA-based end-to-end manipulation for direct motor control.

---

::::{grid} 1 2 3 3
:gutter: 3

:::{grid-item-card} {material-regular}`smart_toy;1.2em;sd-text-primary` Cortex: The Agentic Harness
:link: cortex-agent
:link-type: doc

A component that sits on top of the rest of your recipe and turns it into a self-directing agent. Cortex auto-discovers every component's capabilities and uses them to achieve a high-level goal.
:::

:::{grid-item-card} {material-regular}`memory;1.2em;sd-text-primary` Memory and Cortex
:link: cortex-memory
:link-type: doc

Cortex paired with graph-backed spatio-temporal memory. It recalls past observations, reasons about internal state via interoception, and wraps action tasks in episodes that consolidate into long-term memory.
:::

:::{grid-item-card} {material-regular}`route;1.2em;sd-text-primary` Cortex Driving the Full Stack
:link: cortex-navigation
:link-type: doc

The full stack. Cortex orchestrates Vision, VLM, Memory, the Kompass navigation stack, and TTS end-to-end. Compound goals fulfilled by a single agent -- no behavior trees, no state machines.
:::

:::{grid-item-card} {material-regular}`psychology;1.2em;sd-text-primary` Multimodal Planning
:link: planning-models
:link-type: doc

Navigation guided by sight, not maps. A planning VLM grounds free-form descriptions like *"the yellow chair"* in the live camera frame and projects them into goal points the navigation stack acts on.
:::

:::{grid-item-card} {material-regular}`precision_manufacturing;1.2em;sd-text-primary` VLA Manipulation
:link: vla-manipulation
:link-type: doc

End-to-end neural manipulation. Use one of the latest VLA foundation models -- SmolVLA, Pi0, or any other policy from the HuggingFace LeRobot ecosystem -- and go straight from camera frames + text to joint commands.
:::

:::{grid-item-card} {material-regular}`loop;1.2em;sd-text-primary` Event-Driven VLA
:link: event-driven-vla
:link-type: doc

Closed-loop manipulation from an open-loop policy. A VLM watches the camera during execution and stops the VLA the moment it sees the task complete -- or sees it going wrong.
:::

::::
