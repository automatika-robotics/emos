# Self-Healing with Fallbacks

In the real world, connections drop, APIs time out, solvers fail to converge, and serial cables vibrate loose. A "Production Ready" agent cannot simply freeze when something goes wrong.

EMOS provides a unified fallback API that works identically across the **intelligence layer** (model clients) and the **navigation layer** (algorithms and hardware). In this recipe, we demonstrate both.

---

## Intelligence Layer: Model Fallback

We build an agent that uses a high-intelligence model (hosted remotely) as its primary _brain_, but automatically switches to a smaller, local model if the primary one fails.

### The Strategy: Plan A and Plan B

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

### The Complete Intelligence Fallback Recipe

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

---

## Navigation Layer: Algorithm & System Fallback

Navigation components face a different class of failures: optimization solvers that fail to converge, serial cables that vibrate loose, and robots that get boxed into corners. The same `on_*_fail` API handles all of these.

### Algorithm Failure: Switch Controllers

If the primary high-performance algorithm (e.g., `DWA`) fails, we can switch to a simpler "safety" algorithm (like `PurePursuit`).

```python
from kompass.components import Controller, DriveManager
from kompass.control import ControllersID
from kompass.ros import Action

# Select the primary control algorithm
controller = Controller(component_name="controller")
controller.algorithm = ControllersID.DWA

# Define the fallback: switch to PurePursuit
switch_algorithm_action = Action(
    method=controller.set_algorithm,
    args=(ControllersID.PURE_PURSUIT,)
)

# Fallback sequence: restart first, then switch algorithm if it fails again
controller.on_algorithm_fail(
    action=[Action(controller.restart), switch_algorithm_action],
    max_retries=1
)
```

### System Failure: Restart Hardware Connection

The `DriveManager` talks directly to low-level hardware (micro-controller/motor drivers). Transient failures -- loose USB cables, electromagnetic interference, watchdog trips -- are common and often resolved by a simple restart.

```python
driver = DriveManager(component_name="drive_manager")

# Restart on system failure (unlimited retries for transient hardware glitches)
driver.on_system_fail(Action(driver.restart))
```

### Global Catch-All

For any component that doesn't have specific fallback logic, the Launcher provides a blanket policy.

```python
launcher.on_fail(action_name="restart")
launcher.fallback_rate = 1 / 10  # 0.1 Hz (one retry every 10 seconds)
```

```{seealso}
The [Multiprocessing & Fault Tolerance](multiprocessing.md) recipe shows how to combine `launcher.on_fail()` with process isolation for a complete production setup.
```

## The Same API, Both Layers

The key insight is that **the same three hooks** work everywhere in EMOS:

| Hook | Triggers When | Intelligence Example | Navigation Example |
|---|---|---|---|
| `on_component_fail` | Component crashes or fails to initialize | Remote model server is down | Serial port unavailable |
| `on_algorithm_fail` | Inference or computation fails at runtime | WiFi drops mid-conversation | DWA solver can't converge |
| `on_system_fail` | External dependency is lost | API key revoked | Motor controller resets |

Each hook accepts an `Action` (or list of actions) and an optional `max_retries` parameter. This consistency means you can apply the same resilience patterns regardless of which layer you're working in.
