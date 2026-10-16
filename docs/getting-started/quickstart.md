# Quick Start

## Your First EMOS Recipe

EMOS lets you describe complete robot behaviors as **recipes** -- pure Python scripts that wire together components, models, and ROS topics using a declarative style.

In this quickstart you will build a simple Visual Question Answering recipe: a robot that sees through its camera and answers questions about what it observes. By the end, you'll have run it end-to-end and (optionally) opened a small web UI to talk to it.

```{important}
This guide assumes you have already installed EMOS. If not, see the [Installation guide](installation.md) first.
```

## The Recipe

Save the following as `my_first_recipe.py`. We'll walk through what each section does, then list what needs to be running before you launch it.

```python
from agents.clients.ollama import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel
from agents.ros import Topic, Launcher

# Define input and output topics (pay attention to msg_type)
text0 = Topic(name="text0", msg_type="String")
image0 = Topic(name="image_raw", msg_type="Image")
text1 = Topic(name="text1", msg_type="String")

# Define a model client (Ollama in this case)
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)

# Define a VLM component (a node with a particular functionality)
vlm = VLM(
    inputs=[text0, image0],
    outputs=[text1],
    model_client=qwen_client,
    trigger=text0,
    component_name="vqa",
)
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
text0 = Topic(name="text0", msg_type="String")
image0 = Topic(name="image_raw", msg_type="Image")
text1 = Topic(name="text1", msg_type="String")
```

```{note}
On a real robot, change `image0`'s name to match the topic your camera driver actually publishes (e.g. `/camera/color/image_raw`). The "Before You Run" section below explains how to confirm what's available.
```

### Create a Model Client

EMOS is model-agnostic. Here we create a client that uses [Qwen2.5vl](https://ollama.com/library/qwen2.5vl) served by [Ollama](https://ollama.com):

```python
qwen_vl = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
qwen_client = OllamaClient(qwen_vl)
```

````{tip}
If Ollama is running on a different machine on your network, specify the host and port:

```python
qwen_client = OllamaClient(qwen_vl, host="127.0.0.1", port=8000)
```
````

### Configure the Component

Components are the functional building blocks of EMOS recipes. The VLM component lets you set topic-level prompts using Jinja2 templates so you can shape the model's behavior per-input:

```python
vlm = VLM(
    inputs=[text0, image0],
    outputs=[text1],
    model_client=qwen_client,
    trigger=text0,
    component_name="vqa",
)
vlm.set_topic_prompt(text0, template="""You are an amazing and funny robot.
    Answer the following about this image: {{ text0 }}"""
)
```

### Launch

Finally, bring the recipe up:

```python
launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

## Before You Run

The recipe needs two things before it can do anything useful: a model to talk to and a camera to look through.

**Ollama is running and the model is pulled.** The recipe expects Ollama on the same machine; if it runs elsewhere, give the client its host and port as shown above.

```bash
curl http://localhost:11434/api/tags    # is Ollama up?
ollama pull qwen2.5vl:latest            # pre-fetch the model
```

**Something is publishing on the camera topic.** On a robot with a plugin installed, the plugin brings the camera with it, and `emos info` tells you the name of its image topic; see [Plugins](plugins.md). On a laptop, a webcam driver such as [ROS 2 USB Cam](https://github.com/klintan/ros2_usb_camera) will do. Either way, check that frames are actually arriving:

```bash
ros2 topic list
ros2 topic hz /image_raw                 # or whatever name you set in the recipe
```

If the camera publishes under a different name than `image_raw`, change `image0` in the recipe to match.

Some clients need an extra Python package the first time you use them. When that happens the recipe stops with an error that names the package to install.

## Run It

There are two ways to run a recipe, and they suit different moments. While you are still shaping it, run the script directly and keep the edit-and-run loop as short as possible. Once it works, hand it to EMOS: `emos run` sets up the environment for you, keeps a log of every run, and puts the recipe on the dashboard where anyone can start it from a browser.

### Option A: just run the script

This works in pixi and native mode, where the EMOS packages are importable from your shell.

::::{tab-set}

:::{tab-item} pixi

```bash
# Activate the EMOS environment without leaving your recipe's directory
pixi shell --manifest-path ~/.local/share/emos/pixi.toml
source ~/.local/share/emos/install/setup.sh   # adds the built EMOS packages to your env
python3 my_first_recipe.py
```

:::

:::{tab-item} native

```bash
source /opt/ros/jazzy/setup.bash       # or your installed distro
python3 my_first_recipe.py
```

:::

:::{tab-item} container

In container mode the EMOS packages live inside the container, so running the script from the host does not work. Use option B.

:::

::::

### Option B: run it through EMOS

Put the recipe where EMOS looks for recipes, under its own directory and named `recipe.py`, then run it by name:

```bash
mkdir -p ~/emos/recipes/my_first_recipe
cp my_first_recipe.py ~/emos/recipes/my_first_recipe/recipe.py
emos run my_first_recipe
```

The recipe now shows up on the dashboard's **Recipes → Installed** tab, and every run writes a log to `~/emos/logs/my_first_recipe_<timestamp>.log`. Press Ctrl+C to stop it.

```{seealso}
[Running Recipes](running-recipes.md) compares the two flows in detail and describes the optional `manifest.json` that gives a recipe a display name and description.
```

## Verify It Is Running

From a second terminal, use the usual ROS 2 commands to confirm the node and its topics are up:

```bash
ros2 node list                          # should list the `vqa` node
ros2 topic list                         # should list text0, image_raw, text1
```

To trigger a single inference by hand, publish a question on `text0` and watch `text1` for the reply:

```bash
ros2 topic pub --once /text0 std_msgs/String "{data: 'what do you see?'}"
ros2 topic echo /text1
```

## Add a Web UI

You do not need a terminal to talk to the recipe. EMOS can generate a web UI for any recipe from a single line: tell the launcher which topics to show, and it builds the page.

```python
launcher = Launcher()
launcher.enable_ui(inputs=[text0], outputs=[text1, image0])  # <-- specify UI
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

When the recipe starts, it prints the address of the UI, `https://<ROBOT_IP>:5001`. The page is served over HTTPS with the robot's own certificate, so your browser shows a warning the first time; continue past it and you are on the page. Type a question into the input field and the reply from Qwen2.5vl appears next to the camera image.

![Demo screencast](https://automatikarobotics.com/docs/ui_agents_vlm.gif)

```{note}
Every EMOS install already includes what the web UI needs. If you built the stack from source yourself, install the two UI packages with `pip install python-fasthtml monsterui`.
```

## Where Next

- **Open the dashboard.** It runs at `https://emos.local:8765` (or scan the QR that `emos serve` prints). Pair a browser once and you can pull recipes, launch them and watch their logs from anywhere on the network. See [Dashboard](dashboard.md).
- **Connect your robot.** A robot plugin gives recipes the robot's sensors, actions and events. See [Plugins](plugins.md).
- **Customize this recipe.** [Running Recipes](running-recipes.md) covers the recipe layout, the manifest and how each install mode runs recipes.
- **Build something more capable.** The [Recipes & Tutorials](../recipes/overview.md) section walks through conversational agents, semantic memory, navigation, manipulation, and the Cortex agentic harness.
- **Hit a snag?** [Troubleshooting](troubleshooting.md) collects the common problems with sensors, model servers and each install mode.
