# Motion Detection

A robot that watches a room does not need a vision-language model looking at every frame. It needs to know when something moved, and then to look. The **MotionDetector** component turns motion into an event source: it watches an image stream or a point cloud, publishes a boolean that flips when motion starts and stops, and collects the frames of each motion episode into a video. The rest of the recipe reacts through events, so the expensive components only run when there is something to see.

In this tutorial a camera watches an area, a spoken alert plays when motion starts, and a VLM describes what it sees at that moment.

## The watcher

The detector takes the camera stream as its input and its trigger, so it runs on every frame, and publishes two outputs: the motion state, and the video of each episode once the motion has ended.

```python
from agents.components import MotionDetector
from agents.config import MotionDetectorConfig
from agents.ros import Topic

camera_image = Topic(name="/image_raw", msg_type="Image")
motion_state = Topic(name="/motion", msg_type="Bool")
motion_video = Topic(name="/motion_video", msg_type="Video")

motion_detector = MotionDetector(
    inputs=[camera_image],
    outputs=[motion_state, motion_video],
    config=MotionDetectorConfig(
        motion_estimation_func="frame_difference",
        threshold=0.5,
        motion_stop_delay=10,   # the episode ends after ten still frames
    ),
    trigger=camera_image,
    component_name="motion_detector",
)
```

Frame differencing is cheap and fine for a fixed camera. `optical_flow` costs more and copes better with lighting changes and slow movement. The main settings:

| Field                        | Default            | What it does                                                                                 |
| :--------------------------- | :----------------- | :------------------------------------------------------------------------------------------- |
| `motion_estimation_func`     | `frame_difference` | `frame_difference` or `optical_flow`.                                                        |
| `threshold`                  | 0.3                | How much change counts as motion.                                                            |
| `motion_stop_delay`          | 8                  | Still frames before an episode is over.                                                      |
| `image_scale`                | 0.5                | Frames are downscaled by this before processing.                                             |
| `roi_ignore_polygon`         |                    | A region to ignore, such as a road outside the window.                                       |
| `min_video_frames`, `video_preroll_frames`, `max_video_frames` | 15, 5, 600 | The shortest episode worth a video, how many frames before the motion are kept, and the longest video. |
| `publish_bool_on_change_only`| false              | Publish the state only when it flips, rather than every frame.                              |

The video is a `Video` message, a sequence of frames with the pre-roll in front, published once when the episode ends. It can be recorded, shown in the web interface, or handed to a model that takes image sequences.

## The event

Motion is a state, and we want to act when it begins, not on every frame during which it lasts. `on_change=True` makes the event fire once, when the state turns true:

```python
from agents.ros import Event

motion_started = Event(motion_state.msg.data == True, on_change=True)  # noqa: E712
```

## The reactions

Two components react, and neither runs until the event fires. The event is their trigger, which is how a component in EMOS is woken by something rather than by a timer or a topic.

The first speaks an alert, with a fixed text, on the robot's speakers:

```python
from agents.clients import RoboMLWSClient
from agents.components import TextToSpeech
from agents.config import TextToSpeechConfig
from agents.models import TransformersTTS
from agents.ros import FixedInput

alert_text = FixedInput(
    name="alert",
    msg_type="String",
    fixed="Attention: motion detected in the monitored area.",
)

alert_speaker = TextToSpeech(
    inputs=[alert_text],
    trigger=motion_started,
    model_client=RoboMLWSClient(TransformersTTS(name="tts")),
    config=TextToSpeechConfig(play_on_device=True),
    component_name="alert_speaker",
)
```

The second is the model that would have been too expensive to run all the time. Woken by the same event, it looks at the current frame and says what is moving:

```python
from agents.clients import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel

question = FixedInput(
    name="question",
    msg_type="String",
    fixed="Something just moved in this image. Describe what it is and what it is doing, in one sentence.",
)
description = Topic(name="/motion_description", msg_type="String")

describer = VLM(
    inputs=[question, camera_image],
    outputs=[description],
    model_client=OllamaClient(OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")),
    trigger=motion_started,
    component_name="describer",
)
```

The description could feed a text-to-speech component, a log, or [Memory](../../intelligence/memory.md), where it would be stored at the robot's position and time.

## Motion from a point cloud

With a LiDAR or a depth camera the detector works on the cloud instead. It differences voxel occupancy against an accumulated window, keeps only coherent clusters, and publishes the centres of the moving regions as poses rather than a video. Given the robot's odometry as `position`, it subtracts the robot's own motion and publishes the centres in the odometry frame, so it also works while the robot drives. With an image stream, the same `position` input pauses detection while the robot moves or turns, since a moving camera sees motion everywhere.

```python
cloud = Topic(name="/points", msg_type="PointCloud2")
odom = Topic(name="/odom", msg_type="Odometry")
motion_centers = Topic(name="/motion_centers", msg_type="PoseArray")

cloud_motion_detector = MotionDetector(
    inputs=[cloud],
    outputs=[motion_state, motion_centers],
    config=MotionDetectorConfig(voxel_size=0.15, changed_voxel_threshold=10),
    trigger=cloud,
    position=odom,
    component_name="motion_detector",
)
```

`voxel_size`, `changed_voxel_threshold`, `accumulation_window` and `min_cluster_size` tune the cloud path, and `min_range`, `max_range`, `z_min` and `z_max` bound the volume it looks at.

## Launching

The components run as separate processes here, so the detector's frame rate is not held up by a model's inference. The web interface shows the motion state and each episode's video.

```python
from agents.ros import Launcher

launcher = Launcher()
launcher.enable_ui(outputs=[motion_state, motion_video, description])
launcher.add_pkg(
    components=[motion_detector, alert_speaker, describer],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

<!-- TODO screenshot: the recipe's web UI with the motion state, a motion video and a description -->

## Complete code

```{code-block} python
:caption: Motion alerts and a VLM that wakes on motion
:linenos:

from agents.clients import OllamaClient, RoboMLWSClient
from agents.components import MotionDetector, TextToSpeech, VLM
from agents.config import MotionDetectorConfig, TextToSpeechConfig
from agents.models import OllamaModel, TransformersTTS
from agents.ros import Event, FixedInput, Launcher, Topic

# --- Topics ---
camera_image = Topic(name="/image_raw", msg_type="Image")
motion_state = Topic(name="/motion", msg_type="Bool")
motion_video = Topic(name="/motion_video", msg_type="Video")
description = Topic(name="/motion_description", msg_type="String")

# --- The watcher ---
motion_detector = MotionDetector(
    inputs=[camera_image],
    outputs=[motion_state, motion_video],
    config=MotionDetectorConfig(motion_estimation_func="frame_difference", threshold=0.5, motion_stop_delay=10),
    trigger=camera_image,
    component_name="motion_detector",
)

# --- The event: motion has started ---
motion_started = Event(motion_state.msg.data == True, on_change=True)  # noqa: E712

# --- The alarm ---
alert_text = FixedInput(name="alert", msg_type="String", fixed="Attention: motion detected in the monitored area.")
alert_speaker = TextToSpeech(
    inputs=[alert_text],
    trigger=motion_started,
    model_client=RoboMLWSClient(TransformersTTS(name="tts")),
    config=TextToSpeechConfig(play_on_device=True),
    component_name="alert_speaker",
)

# --- The model that wakes on motion ---
question = FixedInput(
    name="question",
    msg_type="String",
    fixed="Something just moved in this image. Describe what it is and what it is doing, in one sentence.",
)
describer = VLM(
    inputs=[question, camera_image],
    outputs=[description],
    model_client=OllamaClient(OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")),
    trigger=motion_started,
    component_name="describer",
)

# --- Launch ---
launcher = Launcher()
launcher.enable_ui(outputs=[motion_state, motion_video, description])
launcher.add_pkg(
    components=[motion_detector, alert_speaker, describer],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```

---

```{tip}
**Promote this recipe to production.** While you are shaping it, run the script directly with `python recipe.py`. Once it is solid, drop it at `~/emos/recipes/<name>/recipe.py` and start it with `emos run <name>`, or from the dashboard. Either way every run is logged under `~/emos/logs`, and an operator gets a card to launch it from a browser. [Running Recipes](../../getting-started/running-recipes.md) covers the two ways of running a recipe and what differs per install mode.
```
