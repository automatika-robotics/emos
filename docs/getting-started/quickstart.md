# Quick Start

## Your First EMOS Recipe

EMOS lets you describe complete robot behaviors as **recipes** -- pure Python scripts that wire together components, models, and ROS topics using a declarative style powered by [Sugarcoat](https://automatika-robotics.github.io/sugarcoat/).

In this quickstart you will build a simple Visual Question Answering recipe: a robot that sees through its camera and answers questions about what it observes.

```{important}
Depending on the components and clients you use, EMOS will prompt you for extra Python packages. The script will throw an error and tell you exactly what to install.
```

## The Recipe

Copy the following into a Python script (e.g. `my_first_recipe.py`) and run it:

```python
from agents.clients.ollama import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel
from agents.ros import Topic, Launcher

# Define input and output topics (pay attention to msg_type)
text0 = Topic(name="text0", msg_type="String")
image0 = Topic(name="image_raw", msg_type="Image")
text1 = Topic(name="text1", msg_type="String")

# Define a model client (working with Ollama in this case)
# OllamaModel is a generic wrapper for all Ollama models
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)

# Define a VLM component (A component represents a node with a particular functionality)
vlm = VLM(
    inputs=[text0, image0],
    outputs=[text1],
    model_client=qwen_client,
    trigger=text0,
    component_name="vqa"
)
# Additional prompt settings
vlm.set_topic_prompt(text0, template="""You are an amazing and funny robot.
    Answer the following about this image: {{ text0 }}"""
)

# Launch the component
launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

## Step-by-Step Breakdown

### Define Topics

Every EMOS recipe starts by declaring the ROS topics that connect components together. Components automatically create listeners for input topics and publishers for output topics.

```python
# Define input and output topics (pay attention to msg_type)
text0 = Topic(name="text0", msg_type="String")
image0 = Topic(name="image_raw", msg_type="Image")
text1 = Topic(name="text1", msg_type="String")
```

````{important}
If you are running EMOS on a robot, change the topic name to match the topic your robot's camera publishes RGB images on:

```python
image0 = Topic(name="NAME_OF_THE_TOPIC", msg_type="Image")
````

```{note}
If you are running EMOS on a development machine with a webcam, you can install [ROS2 USB Cam](https://github.com/klintan/ros2_usb_camera). Make sure you use the correct image topic name as above.
```

### Create a Model Client

EMOS is model-agnostic. Here we create a client that uses [Qwen2.5vl](https://ollama.com/library/qwen2.5vl) served by [Ollama](https://ollama.com):

```python
# Define a model client (working with Ollama in this case)
# OllamaModel is a generic wrapper for all Ollama models
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)
```

````{important}
If Ollama is running on a different machine on your network, specify the host and port:
```python
qwen_client = OllamaClient(qwen_vl, host="127.0.0.1", port=8000)
````

```{note}
If the use of Ollama as a model serving platform is unclear, see the [Installation guide](installation.md) for setup options.
```

### Configure the Component

Components are the functional building blocks of EMOS recipes. The VLM component also lets you set topic-level prompts using Jinja2 templates:

```python
# Define a VLM component (A component represents a node with a particular functionality)
mllm = VLM(
    inputs=[text0, image0],
    outputs=[text1],
    model_client=qwen_client,
    trigger=text0,
    component_name="vqa"
)
# Additional prompt settings
mllm.set_topic_prompt(text0, template="""You are an amazing and funny robot.
    Answer the following about this image: {{ text0 }}"""
)
```

### Launch

Finally, bring the recipe up:

```python
# Launch the component
launcher = Launcher()
launcher.add_pkg(components=[mllm])
launcher.bringup()
```

## Verify It Is Running

From a new terminal, use standard ROS 2 commands to confirm the node and its topics are active:

```shell
ros2 node list
ros2 topic list
```

## Enable the Web UI

EMOS can dynamically generate a web-based UI for any recipe. Add one line before `bringup()` to tell the launcher which topics to render:

```python
# Launch the component
launcher = Launcher()
launcher.enable_ui(inputs=[text0], outputs=[text1, image0])  # <-- specify UI
launcher.add_pkg(components=[mllm])
launcher.bringup()
```

````{note}
The web UI requires two additional packages:
```shell
pip install python-fasthtml monsterui
````

The UI is served at **http://localhost:5001** (or **http://<ROBOT_IP>:5001** if running on a robot). Open it in your browser, configure component settings with the settings button, and send a question -- you should get a reply generated by the Qwen2.5vl model.

![Demo screencast](https://automatikarobotics.com/docs/ui_agents_vlm.gif)
