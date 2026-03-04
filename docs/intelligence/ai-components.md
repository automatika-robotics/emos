# AI Components

A **Component** is the primary execution unit in the EMOS Intelligence Layer. Components represent functional behaviors -- for example, the ability to process text, understand images, or synthesize speech. Components can be combined arbitrarily to create more complex systems such as multi-modal agents with perception-action loops. Conceptually, each component is a layer of syntactic sugar over a ROS2 Lifecycle Node, inheriting all its lifecycle behaviors while also offering allied functionality to manage inputs and outputs and simplify development. Components receive one or more ROS topics as inputs and produce outputs on designated topics. The specific types and formats of these topics depend on the component's function.

```{note}
To learn more about the internal structure and lifecycle behavior of components, check out the documentation of [Sugarcoat](https://automatika-robotics.github.io/sugarcoat/design/component.html).
```

## Available Components

The EMOS Intelligence Layer provides a suite of ready-to-use components. These can be composed into flexible execution graphs for building autonomous, perceptive, and interactive robot behavior. Each component focuses on a particular modality or functionality, from vision and speech to map reasoning and VLA-based manipulation.

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

## Topic

A **Topic** is an idiomatic wrapper for a ROS2 topic. Topics can be given as inputs or outputs to components. When given as inputs, components automatically create listeners for the topics upon their activation. When given as outputs, components create publishers for publishing to the topic. Each topic has a name and a data type, defining its listening callback and publishing behavior. The data type can be provided to the topic as a string. The list of supported data types is below.

```{note}
Learn more about Topics in [Sugarcoat](https://automatika-robotics.github.io/sugarcoat/).
```

```{list-table}
:widths: 20 40 40
:header-rows: 1

* - Message
  - ROS2 Package
  - Description

* - **String**
  - std_msgs
  - Standard text message.

* - **Bool**
  - std_msgs
  - Boolean value (True/False).

* - **Float32**
  - std_msgs
  - Single-precision floating point number.

* - **Float32MultiArray**
  - std_msgs
  - Array of single-precision floating point numbers.

* - **Float64**
  - std_msgs
  - Double-precision floating point number.

* - **Float64MultiArray**
  - std_msgs
  - Array of double-precision floating point numbers.

* - **Twist**
  - geometry_msgs
  - Velocity expressed as linear and angular components.

* - **Image**
  - sensor_msgs
  - Raw image data.

* - **CompressedImage**
  - sensor_msgs
  - Compressed image data (e.g., JPEG, PNG).

* - **Audio**
  - sensor_msgs
  - Audio stream data.

* - **Path**
  - nav_msgs
  - An array of poses representing a navigation path.

* - **OccupancyGrid**
  - nav_msgs
  - 2D grid map where each cell represents occupancy probability.

* - **ComponentStatus**
  - automatika_ros_sugar
  - Lifecycle status and health information of a component.

* - **StreamingString**
  - automatika_embodied_agents
  - String chunk for streaming applications (e.g., LLM tokens).

* - **Video**
  - automatika_embodied_agents
  - A sequence of image frames.

* - **Detections**
  - automatika_embodied_agents
  - 2D bounding boxes with labels and confidence scores.

* - **DetectionsMultiSource**
  - automatika_embodied_agents
  - List of 2D detections from multiple input sources.

* - **PointsOfInterest**
  - automatika_embodied_agents
  - Specific 2D coordinates of interest within an image.

* - **Trackings**
  - automatika_embodied_agents
  - Object tracking data including IDs, labels, and trajectories.

* - **TrackingsMultiSource**
  - automatika_embodied_agents
  - Object tracking data from multiple sources.

* - **RGBD**
  - realsense2_camera_msgs
  - Synchronized RGB and Depth image pair.

* - **JointTrajectoryPoint**
  - trajectory_msgs
  - Position, velocity, and acceleration for joints at a specific time.

* - **JointTrajectory**
  - trajectory_msgs
  - A sequence of waypoints for joint control.

* - **JointJog**
  - control_msgs
  - Immediate displacement or velocity commands for joints.

* - **JointState**
  - sensor_msgs
  - Instantaneous position, velocity, and effort of joints.
```

## Component Config

Each component can optionally be configured using a `config` object. Configs are generally built using [attrs](https://www.attrs.org/en/stable/) and include parameters controlling model inference, thresholds, topic remapping, and other component-specific behavior. Components involving ML models define their inference options here.

## Component RunType

In the EMOS Intelligence Layer, components can operate in one of three modes:

```{list-table}
:widths: 10 80
* - **Timed**
  - Executes its main function at regular time intervals (e.g., every N milliseconds).
* - **Reactive**
  - Executes in response to a trigger. A trigger can be either incoming messages on one or more trigger topics, OR an `Event`.
* - **Action Server**
  - Executes in response to an action request. Components of this type execute a long-running task (action) and can return feedback while the execution is ongoing.
```

## Health Check and Fallback

Each component maintains an internal health state. This is used to support fallback behaviors and graceful degradation in case of errors or resource unavailability. Health monitoring is essential for building reliable and resilient autonomous agents, especially in real-world environments.

Fallback behaviors can include retry mechanisms, switching to alternate inputs, or deactivating the component safely. For deeper understanding, refer to [Sugarcoat](https://automatika-robotics.github.io/sugarcoat/design/fallbacks.html), which underpins the lifecycle and health management logic.
