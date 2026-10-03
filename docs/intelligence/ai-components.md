# AI Components

EmbodiedAgents, the intelligence layer of EMOS, is a set of components, each one a capability the robot can have: understanding speech, describing what the camera sees, detecting objects, remembering what it has seen, moving an arm. They are ordinary EMOS [components](../concepts/components.md), so they have the same lifecycle, inputs and outputs, health status and fallbacks as everything else in a recipe, and they combine freely into perception and action loops.

Most of them wrap a model, and where the model runs is a separate choice: a cloud API, a model server on the network, or a local model on the robot itself. [Clients](clients.md) covers the first two and [Models](models.md) the local ones. One component stands apart: [Cortex](cortex.md) uses the others. It takes a goal in natural language and works out which capabilities to call, in what order, to reach it.

## The components

```{list-table}
:widths: 20 80
:header-rows: 1

* - Component
  - What it does

* - **LLM**
  - Runs a large language model on text: reasoning, instruction following, dialogue and tool calling. It can draw on a vector database for context and falls back to a local model when its server is out of reach.

* - **VLM**
  - The same for images and text together, on a multimodal model. Beyond describing a scene and answering questions about it, it runs task models for pointing, grounding and affordance, and with a depth source it turns what it grounds into 3D boxes. Its `describe` and `run_task` actions let an event or Cortex ask a question or run a task on the current frame. Also known as **MLLM**.

* - **Vision**
  - Object detection and tracking on images, with classes, boxes and confidences, from a model server or a small on-board classifier. Given depth, from an RGBD camera, a depth image or a point cloud, it publishes metric 3D boxes in the frame you choose, which is what MoveIt and Memory consume.

* - **VLA**
  - Drives manipulation and control with a vision-language-action policy served by LeRobot's policy server: SmolVLA, Pi0 and Pi0.5, GR00T, ACT, Diffusion and others. Camera and joint-state inputs go in, joint commands come out in the formats MoveIt Servo and ROS 2 Control expect, and a goal on its action server runs one task.

* - **MoveIt**
  - Motion planning and execution for an arm through a running MoveIt 2 `move_group`, as an action server with pose, joint, named, Cartesian, pick and place goals, and gripper control. Objects that Vision detects in 3D are placed in the planning scene as obstacles. Every one of its actions is a tool for Cortex.

* - **SpeechToText**
  - Turns spoken audio into text. It runs its own voice activity detection and a wakeword spotter with a phrase you choose, on audio captured on the robot, and transcribes with a model server or a local model.

* - **TextToSpeech**
  - Turns text into speech, played on the robot's speakers or published as audio, from a model server or one of several local model families. Its `say` and `stop_playback` actions are made for events: announce a warning, stop talking when someone speaks.

* - **Memory**
  - A graph-backed spatio-temporal memory of what the robot has seen and felt, built on [eMEM](https://github.com/automatika-robotics/emem): detections, scene descriptions and internal state, each at a place and a time, with retrieval tools for the recipe and for Cortex. See [Memory](memory.md).

* - **Cortex**
  - The agent. It discovers every action, action server, service, routine and plugin action in the recipe, plans a sequence of calls for a natural-language goal, and runs it while watching the outputs. See [Cortex](cortex.md).

* - **SemanticRouter**
  - Sends an input to one of several destinations by what it says, using a vector database of examples or an LLM to decide, so one microphone or one text input can drive several pipelines.

* - **MotionDetector**
  - Detects motion in an image stream or a point cloud stream and publishes a boolean, which makes it an event source, along with the frames of the episode as a video or the motion centres as poses. Given the robot's odometry it ignores the robot's own movement. It replaces the earlier VideoMessageMaker.
```

```{seealso}
- [Clients](clients.md) and [Models](models.md) for where the models run.
- [Cortex](cortex.md) and [Memory](memory.md) for the two components with pages of their own.
- The [foundation](../recipes/foundation/index.md) and [planning and manipulation](../recipes/planning-and-manipulation/index.md) recipes for these components at work.
```
