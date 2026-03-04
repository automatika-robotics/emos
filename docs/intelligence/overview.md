# Intelligence Layer

**The production-grade framework for deploying Physical AI on real-world robots.**

The Intelligence Layer is the cognitive core of EMOS -- the Embodied Operating System by Automatika Robotics. Powered by the EmbodiedAgents framework, it provides the orchestration infrastructure needed to create interactive, physical agents that do not just chat, but **understand**, **move**, **manipulate**, and **adapt** to their environment.

While the Hardware Layer manages sensors, actuators, and drivers, the Intelligence Layer sits above it, giving robots the ability to perceive, reason, plan, and act autonomously. It bridges the gap between foundation AI models and real-world robotic deployment, offering a structured yet flexible programming model for building adaptive intelligence.

## Key Differentiators

### Production-Ready Physical Agents

The Intelligence Layer is designed for autonomous systems operating in dynamic, real-world environments. It provides a complete orchestration layer for **Adaptive Intelligence**, making Physical AI simple to deploy. Components are built around ROS2 Lifecycle Nodes, offering deterministic startup, shutdown, and error-recovery semantics that production robots demand. Health monitoring, fallback behaviors, and graceful degradation are built in from the ground up.

### Self-Referential and Event-Driven

Agents built with the Intelligence Layer can start, stop, or reconfigure their own components based on internal or external events. This makes it trivial to switch from cloud-based to local ML inference, swap planners based on location or vision input, or dynamically adjust behavior in response to environmental changes. In the spirit of [Godel machines](https://en.wikipedia.org/wiki/G%C3%B6del_machine), agents become self-referential -- capable of introspecting and modifying their own execution graph at runtime.

### Semantic Memory

The Intelligence Layer provides embodiment primitives such as hierarchical spatio-temporal memory and semantic routing to build arbitrarily complex graphs for agentic information flow. Components like MapEncoding and SemanticRouter allow robots to maintain structured, queryable representations of their environment over time -- without relying on bloated general-purpose GenAI frameworks.

### Pure Python, Native ROS2

Complex asynchronous execution graphs can be defined in standard Python without touching XML launch files. Yet, underneath, everything is pure ROS2 -- fully compatible with the entire ecosystem of hardware drivers, simulation tools, and visualization suites. This means developers get the ergonomics of a modern Python framework with the reliability and interoperability of the ROS2 middleware.

## What You Can Build

With the Intelligence Layer, you can compose components into flexible execution graphs for building autonomous, perceptive, and interactive robot behaviors. Common use cases include:

- **Conversational robots** with speech-to-text, LLM reasoning, and text-to-speech pipelines.
- **Vision-guided manipulation** using VLMs for planning and VLAs for control.
- **Semantic navigation** with map encoding and spatio-temporal memory.
- **Multi-modal agents** that dynamically route information between perception, reasoning, and action components based on semantic content.

## Next Steps

- {doc}`ai-components` -- Learn about the core building blocks: components and topics.
- {doc}`clients` -- Understand how inference backends connect to components.
- {doc}`models` -- Explore available model wrappers and vector databases.
