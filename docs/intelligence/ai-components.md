# AI Components

A **Component** is the primary execution unit in EmbodiedAgents, the EMOS intelligence framework. Components represent functional behaviors -- for example, the ability to process text, understand images, or synthesize speech. Components can be combined arbitrarily to create more complex systems such as multi-modal agents with perception-action loops. Conceptually, each component is a layer of syntactic sugar over a ROS2 Lifecycle Node, inheriting all its lifecycle behaviors while also offering allied functionality to manage inputs and outputs and simplify development. Components receive one or more ROS topics as inputs and produce outputs on designated topics. The specific types and formats of these topics depend on the component's function.

```{note}
To learn more about the internal structure and lifecycle behavior of components, check out the documentation of [Sugarcoat](https://automatika-robotics.github.io/sugarcoat/design/component.html).
```

## Available Components

EmbodiedAgents provides a suite of ready-to-use components. These can be composed into flexible execution graphs for building autonomous, perceptive, and interactive robot behavior. Each component focuses on a particular modality or functionality, from vision and speech to map reasoning and VLA-based manipulation.

```{list-table}
:widths: 20 80
:header-rows: 1

* - Component Name
  - Description

* - **LLM**
  - Uses large language models (e.g., LLaMA) to process text input. Can be used for reasoning, tool calling, instruction following, or dialogue. It can also utilize vector DBs for storing and retrieving contextual information.

* - **VLM**
  - Leverages multimodal LLMs (e.g., Llava) for understanding and processing both text and image data. Inherits all functionalities of the LLM component. It can also utilize multimodal LLM based planning models for task-specific outputs (e.g. pointing, grounding, affordance etc.). **This component is also called MLLM**.

* - **VLA**
  - Provides an interface to utilize Vision Language Action (VLA) models for manipulation and control tasks. It can use VLA Policies (such as SmolVLA, Pi0 etc.) served with HuggingFace LeRobot Async Policy Server and publish them to common topic formats in MoveIt Servo and ROS2 Control.

* - **SpeechToText**
  - Converts spoken audio into text using speech-to-text models (e.g., Whisper). Suitable for voice command recognition. It also implements small on-board models for Voice Activity Detection (VAD) and Wakeword recognition, using audio capture devices onboard the robot.

* - **TextToSpeech**
  - Synthesizes audio from text using TTS models (e.g., SpeechT5, Bark). Output audio can be played using the robot's speakers or published to a topic. Implements `say(text)` and `stop_playback` functions to play/stop audio based on events from other components or the environment.

* - **MapEncoding**
  - Provides a spatio-temporal working memory by converting semantic outputs (e.g., from MLLMs or Vision) into a structured map representation. Uses robot localization data and output topics from other components to store information in a vector DB.

* - **SemanticRouter**
  - Routes information between topics based on semantic content and predefined routing rules. Uses a vector DB for semantic matching or an LLM for decision-making. This allows for creating complex graphs of components where a single input source can trigger different information processing pathways.

* - **Vision**
  - An essential component in all vision-powered robots. Performs object detection and tracking on incoming images. Outputs object classes, bounding boxes, and confidence scores. It implements a low-latency small on-board classification model as well.

* - **VideoMessageMaker**
  - Generates ROS video messages from input image messages. A video message is a collection of image messages that have a perceivable motion. The primary task of this component is to make intentionality decisions about what sequence of consecutive images should be treated as one coherent temporal sequence. The chunking method used for selecting images for a video can be configured in component config. Useful for sending videos to ML models that take image sequences.
```

```{seealso}
For details on Topics, component configuration, run types, health checks, and fallback behaviors, see the [Core Concepts](../concepts/components.md) section.
```
