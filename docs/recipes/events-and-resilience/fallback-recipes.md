# Runtime Model Fallback

In the real world, connections drop, APIs time out, and servers crash. Sticking with the theme of robustness, a "Production Ready" agent cannot simply freeze when its internet connection is lost.

In this tutorial, we will demonstrate the self-referential capabilities of EMOS. We will build an agent that uses a high-intelligence model (hosted remotely) as its primary _brain_, but automatically switches to a smaller, local model if the primary one fails.

## The Strategy: Plan A and Plan B

Our strategy is simple:

1. **Plan A (Primary):** Use a powerful model hosted via RoboML (or a cloud provider) for high-quality reasoning.
2. **Plan B (Backup):** Keep a smaller, quantized model (like Llama 3.2 3B) loaded locally via Ollama.
3. **The Trigger:** If the Primary model fails to respond (latency, disconnection, or server error), automatically swap the component's internal client to the Backup.

### 1. Defining the Models

First, we need to define our two distinct model clients.

```python
from agents.components import LLM
from agents.models import OllamaModel, TransformersLLM
from agents.clients import OllamaClient, RoboMLHTTPClient
from agents.config import LLMConfig
from agents.ros import Launcher, Topic, Action

# --- Plan A: The Powerhouse ---
# A powerful model hosted remotely (e.g., via RoboML).
# NOTE: This is illustrative for executing on a local machine.
# For a production scenario, you might use a GenericHTTPClient pointing to
# GPT-5, Gemini, HuggingFace Inference etc.
primary_model = TransformersLLM(
    name="qwen_heavy",
    checkpoint="Qwen/Qwen2.5-1.5B-Instruct"
)
primary_client = RoboMLHTTPClient(model=primary_model)

# --- Plan B: The Safety Net ---
# A smaller model running locally (via Ollama) that works offline.
backup_model = OllamaModel(name="llama_local", checkpoint="llama3.2:3b")
backup_client = OllamaClient(model=backup_model)
```

### 2. Configuring the Component

Next, we set up the standard `LLM` component. We initialize it using the `primary_client`.

However, the magic happens in the `additional_model_clients` attribute. This dictionary allows the component to hold references to other valid clients that are waiting in the wings.

```python
# Define Topics
user_query = Topic(name="user_query", msg_type="String")
llm_response = Topic(name="llm_response", msg_type="String")

# Configure the LLM Component with the PRIMARY client initially
llm_component = LLM(
    inputs=[user_query],
    outputs=[llm_response],
    model_client=primary_client,
    component_name="brain",
    config=LLMConfig(stream=True),
)

# Register the Backup Client
# We store the backup client in the component's internal registry.
# We will use the key 'local_backup_client' to refer to this later.
llm_component.additional_model_clients = {"local_backup_client": backup_client}
```

### 3. Creating the Fallback Action

Now we need an **Action**. In EMOS, components have built-in methods to reconfigure themselves. The `LLM` component (like all other components that take a model client) has a method called `change_model_client`.

We wrap this method in an `Action` so it can be triggered by an event.

```{note}
All components implement some default actions as well as component specific actions. In this case we are implementing a component specific action.
```

```{seealso}
To see a list of default actions available to all components, checkout the [Actions](../../concepts/events-and-actions.md) documentation.
```

```python
# Define the Fallback Action
# This action calls the component's internal method `change_model_client`.
# We pass the key ('local_backup_client') defined in the previous step.
switch_to_backup = Action(
    method=llm_component.change_model_client,
    args=("local_backup_client",)
)
```

### 4. Wiring Failure to Action

Finally, we tell the component _when_ to execute this action. We don't need to write complex `try/except` blocks in our business logic. Instead, we attach the action to the component's lifecycle hooks:

- **`on_component_fail`**: Triggered if the component crashes or fails to initialize (e.g., the remote server is down when the robot starts).
- **`on_algorithm_fail`**: Triggered if the component is running, but the inference fails (e.g., the WiFi drops mid-conversation).

```python
# Bind Failures to the Action
# If the component fails (startup) or the algorithm crashes (runtime),
# it will attempt to switch clients.
llm_component.on_component_fail(action=switch_to_backup, max_retries=3)
llm_component.on_algorithm_fail(action=switch_to_backup, max_retries=3)
```

```{note}
**Why `max_retries`?** Sometimes a fallback can temporarily fail as well. The system will attempt to restart the component or algorithm up to 3 times while applying the action (switching the client) to resolve the error. This is an _optional_ parameter.
```

## The Complete Recipe

Here is the full code. To test this, you can try shutting down your RoboML server (or disconnecting the internet) while the agent is running, and watch it seamlessly switch to the local Llama model.

```python
from agents.components import LLM
from agents.models import OllamaModel, TransformersLLM
from agents.clients import OllamaClient, RoboMLHTTPClient
from agents.config import LLMConfig
from agents.ros import Launcher, Topic, Action

# 1. Define the Models and Clients
# Primary: A powerful model hosted remotely
primary_model = TransformersLLM(
    name="qwen_heavy", checkpoint="Qwen/Qwen2.5-1.5B-Instruct"
)
primary_client = RoboMLHTTPClient(model=primary_model)

# Backup: A smaller model running locally
backup_model = OllamaModel(name="llama_local", checkpoint="llama3.2:3b")
backup_client = OllamaClient(model=backup_model)

# 2. Define Topics
user_query = Topic(name="user_query", msg_type="String")
llm_response = Topic(name="llm_response", msg_type="String")

# 3. Configure the LLM Component
llm_component = LLM(
    inputs=[user_query],
    outputs=[llm_response],
    model_client=primary_client,
    component_name="brain",
    config=LLMConfig(stream=True),
)

# 4. Register the Backup Client
llm_component.additional_model_clients = {"local_backup_client": backup_client}

# 5. Define the Fallback Action
switch_to_backup = Action(
    method=llm_component.change_model_client,
    args=("local_backup_client",)
)

# 6. Bind Failures to the Action
llm_component.on_component_fail(action=switch_to_backup, max_retries=3)
llm_component.on_algorithm_fail(action=switch_to_backup, max_retries=3)

# 7. Launch
launcher = Launcher()
launcher.add_pkg(
    components=[llm_component],
    multiprocessing=True,
    package_name="automatika_embodied_agents",
)
launcher.bringup()
```
