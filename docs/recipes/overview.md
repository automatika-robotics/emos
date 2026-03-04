# Recipes & Tutorials

## What Are EMOS Recipes?

An EMOS **Recipe** is a reusable, hardware-agnostic application package that defines a robot behavior. Recipes replace the brittle, task-specific ROS projects of the past with composable, declarative Python scripts that combine EMOS components -- intelligence, navigation, perception, and memory -- into a single agentic workflow.

A Recipe is not a "script" in the traditional sense. It is a complete application: a graph of [Components](../intelligence/ai-components.md) wired together through [Topics](../concepts/topics.md), enriched with [Events & Actions](../concepts/events-and-actions.md) for runtime adaptivity, and launched with a single call to `Launcher.bringup()`.

### Write Once, Run Anywhere

The core promise of EMOS Recipes is hardware independence. A "Security Patrol" recipe written for a wheeled AMR runs identically on a quadruped -- EMOS handles the kinematic translation and action commands beneath the surface. This decoupling of the robot's **Mind** from its **Body** means that:

- The same Recipe can be deployed across an entire heterogeneous fleet.
- Recipes can be shared, versioned, and composed just like software packages.
- The same robot can run multiple Recipes, switching between them as conditions demand.

## Tutorial Structure

The tutorials in this section follow a graduated structure, building from simple single-component examples to a full agentic system.

### Foundation Recipes (Intelligence)

These recipes focus on the EMOS intelligence layer -- wiring up ML models, prompts, and memory.

| Recipe | Description |
| --- | --- |
| [Conversational Agent](conversational-agent.md) | Audio-in, audio-out conversation using speech-to-text, a VLM, and text-to-speech. |
| [Prompt Engineering](prompt-engineering.md) | Enrich VLM prompts with object detection output. |
| [Semantic Map](semantic-map.md) | Build a spatio-temporal semantic memory using vector databases. |
| [GoTo Navigation](goto-navigation.md) | RAG-powered "Go to X" using the semantic map. |
| [Tool Calling](tool-calling.md) | Structured output via LLM tool calling for navigation goals. |
| [Semantic Routing](semantic-routing.md) | Route user intent to the correct component with a Semantic Router. |

### Capstone

| Recipe | Description |
| --- | --- |
| [Complete Agent](complete-agent.md) | Combine every foundation recipe into a single, fully capable embodied agent. |

## Recipe Examples

Below are four real-world examples that illustrate what EMOS Recipes look like in practice. Each is a self-contained Python snippet that defines an agentic workflow.

### 1. The General Purpose Assistant

*A robot that intelligently routes verbal commands to the correct capability.*

This recipe uses a **Semantic Router** to analyze user intent. "Go to the kitchen" routes to the navigation layer, while "What tool is this?" routes to a VLM for visual question answering.

```python
# Define routing logic based on semantic meaning, not just keywords
llm_route = Route(samples=["What is the torque for M6?", "Convert inches to mm"])
mllm_route = Route(samples=["What tool is this?", "Is the safety light on?"])
goto_route = Route(samples=["Go to the CNC machine", "Move to storage"])

# The Semantic Router directs traffic based on intent
router = SemanticRouter(
    inputs=[query_topic],
    routes=[llm_route, goto_route, mllm_route], # Routes to Chat, Nav, or Vision
    default_route=llm_route,
    config=router_config
)
```

### 2. The Resilient "Always-On" Agent

*Ensuring uptime by falling back to local compute when the internet fails.*

This demonstrates **Runtime Robustness**. We bind an `on_algorithm_fail` [event](../concepts/events-and-actions.md) to the intelligence component. If the cloud API disconnects, the Recipe triggers a specific recovery action rather than crashing.

```python
# If the cloud API fails (runtime), instantly switch to the local backup model
llm_component.on_algorithm_fail(
    action=switch_to_backup,
    max_retries=3
)
```

### 3. The Self-Recovering Warehouse Robot

*A robot that unjams itself without human intervention.*

Instead of a "Red Light" error, the robot uses an [event/action pair](../concepts/events-and-actions.md) to trigger a specific maneuvering routine when the planner gets stuck.

```python
events_actions = {
    # If the emergency stop triggers, restart the planner and back away
    event_emergency_stop: [
        ComponentActions.restart(component=planner),
        unblock_action,
    ],
    # If the controller algorithm fails, attempt unblocking maneuver
    event_controller_fail: unblock_action,
}
```

### 4. The "Off-Grid" Field Mule

*Follow-me functionality in unmapped environments.*

Using the `VisionRGBDFollower`, the robot fuses depth data with visual detection to "lock on" to a human guide, ignoring the need for GPS or SLAM.

```python
# Setup controller to use Depth + Vision Fusion
controller.inputs(
    vision_detections=detections_topic,
    depth_camera_info=depth_cam_info_topic
)
controller.algorithm = "VisionRGBDFollower"
```
