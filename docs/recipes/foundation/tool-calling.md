# Tool Calling

In the [previous recipe](goto-navigation.md) we registered one of Memory's built-in tools (`locate`) on an LLM and let the LLM decide when to call it. That's the most direct path when the tool you want is already a `@component_action` on a managed component. **In this recipe we generalise the pattern**: we write our own Python function and register it as a tool on the LLM. The function can do anything Python can do -- read a sensor, hit a web service, toggle a piece of hardware, run a calculation -- and the LLM picks it up as a callable tool with the schema we declare.

The key idea: when the LLM is configured with one or more registered tools, it produces *structured tool calls* instead of raw text. A tool call has a name and JSON-serialisable arguments that match the tool's declared schema. The framework dispatches the call into our Python function, and we choose what happens to the return value -- either:

- `send_tool_response_to_model=True` — the function's return is fed back to the LLM as a tool result, and the LLM produces a follow-up generation that uses it. The right mode when you want the LLM to *react* to the tool's answer.
- `send_tool_response_to_model=False` — the function's return value *is* the LLM component's output, and gets published on the output topic directly. The right mode when you just want the LLM to extract structured arguments and you handle the rest. This was what we used in the [GoTo Navigation](goto-navigation.md) recipe.

Tool calling works with any client that supports it -- the `OllamaClient` and the `GenericHTTPClient` both do, as long as the underlying model is tool-trained.

---

## What we're building

A tiny voice-of-the-world agent: an LLM component that can answer questions about the **current weather anywhere on Earth** by calling a custom function that hits the public [Open-Meteo](https://open-meteo.com) API. Weather is the canonical case for tool calling -- the LLM categorically can't know it from training data, so the tool earns its place by giving the model live information it would otherwise have to invent.

| Component | Role |
|---|---|
| **LLM** | Receives a free-form question, decides whether to call our `get_weather` tool, and composes a natural-language reply. |
| **`get_weather` tool** | A regular Python function that geocodes a city name, queries Open-Meteo, and returns the current conditions. |

This shows the *full* tool-calling loop: tool result back to LLM → LLM phrases an answer in natural language. Anywhere we'd like the LLM to *use* live data instead of hallucinating it -- a sensor reading, a database row, a web fetch, a sub-process -- follows the same shape.

---

## Step 1: The LLM

A plain `LLM` component using a tool-capable Ollama model.

```python
from agents.clients import OllamaClient
from agents.components import LLM
from agents.config import LLMConfig
from agents.models import OllamaModel
from agents.ros import Launcher, Topic

qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

question = Topic(name="question", msg_type="String")
answer = Topic(name="answer", msg_type="String")

assistant = LLM(
    inputs=[question],
    outputs=[answer],
    model_client=qwen_client,
    trigger=question,
    config=LLMConfig(),
    component_name="assistant",
)

assistant.set_component_prompt(
    template=(
        "You are a friendly assistant. If the user asks about the current "
        "weather, temperature, or wind in a location, call the "
        "``get_weather`` tool with the city name and answer using the data "
        "it returns. The user said: {{question}}"
    )
)
```

The prompt nudges the model toward the tool when the question is weather-related. For unrelated questions the LLM will just answer directly without calling anything.

---

## Step 2: Define the tool

A regular Python function that takes a city name, geocodes it, and queries the Open-Meteo current-conditions endpoint. No API key required.

```python
from typing import Dict, Union

import httpx


def get_weather(city: str) -> Dict[str, Union[str, float]]:
    """Look up the current weather for *city* via the Open-Meteo public API.

    Returns a dict with the city's resolved name, temperature, and wind
    speed -- or ``{"error": "..."}`` if the city couldn't be resolved.
    """
    # Geocoding: city name -> (lat, lon)
    geo = httpx.get(
        "https://geocoding-api.open-meteo.com/v1/search",
        params={"name": city, "count": 1},
        timeout=10.0,
    ).json()
    if not geo.get("results"):
        return {"error": f"could not find location '{city}'"}

    place = geo["results"][0]
    lat, lon = place["latitude"], place["longitude"]

    # Current weather at those coordinates
    weather = httpx.get(
        "https://api.open-meteo.com/v1/forecast",
        params={
            "latitude": lat,
            "longitude": lon,
            "current": "temperature_2m,wind_speed_10m",
        },
        timeout=10.0,
    ).json()["current"]

    return {
        "city": place.get("name", city),
        "country": place.get("country", ""),
        "temperature_c": weather["temperature_2m"],
        "wind_speed_kmh": weather["wind_speed_10m"],
    }
```

`httpx` is already a dependency of EMOS, so no extra install. The function:

- takes one string argument (`city`),
- does its own work (two HTTP calls -- geocoding, then weather),
- returns a small dict the LLM can phrase prose around.

---

## Step 3: Declare the tool's schema

The LLM needs an OpenAI-format description so it knows the tool's name, what it does, and what arguments to provide.

```python
get_weather_description = {
    "type": "function",
    "function": {
        "name": "get_weather",
        "description": (
            "Look up the current weather (temperature in Celsius and wind "
            "speed in km/h) for a given city. Call this whenever the user "
            "asks about live weather conditions anywhere in the world."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "city": {
                    "type": "string",
                    "description": (
                        "City name, optionally with country (e.g. 'Berlin', "
                        "'Tokyo', 'Paris, France')."
                    ),
                },
            },
            "required": ["city"],
        },
    },
}
```

Spend the description budget on telling the model **when** to call the tool and what the arguments mean. The LLM doesn't see the function body; it only sees the description.

---

## Step 4: Register the tool

```python
assistant.register_tool(
    tool=get_weather,
    tool_description=get_weather_description,
    send_tool_response_to_model=True,
)
```

`send_tool_response_to_model=True` is what makes this recipe a conversational loop:

1. User asks *"what's the weather in Berlin right now?"*.
2. LLM emits a structured `get_weather(city="Berlin")` tool call.
3. Framework dispatches the call, the function geocodes Berlin and hits Open-Meteo.
4. The result -- `{"city": "Berlin", "country": "Germany", "temperature_c": 18.3, "wind_speed_kmh": 11.5}` -- is fed back into the LLM as a tool message.
5. The LLM produces the final user-facing reply: *"It's currently 18°C in Berlin with a light breeze around 12 km/h."*
6. That reply gets published on `answer`.

If the user asks something off-topic (*"what's the capital of Peru?"*), the LLM never calls the tool and answers from its own knowledge.

---

## Step 5: Launch

```python
launcher = Launcher()
launcher.add_pkg(components=[assistant])
launcher.bringup()
```

---

## Full recipe code

```{code-block} python
:caption: Tool calling on an LLM with a custom weather-lookup tool
:linenos:
from typing import Dict, Union

import httpx

from agents.clients import OllamaClient
from agents.components import LLM
from agents.config import LLMConfig
from agents.models import OllamaModel
from agents.ros import Launcher, Topic


# -- LLM --
qwen = OllamaModel(name="qwen", checkpoint="qwen3.5:latest")
qwen_client = OllamaClient(qwen)

question = Topic(name="question", msg_type="String")
answer = Topic(name="answer", msg_type="String")

assistant = LLM(
    inputs=[question],
    outputs=[answer],
    model_client=qwen_client,
    trigger=question,
    config=LLMConfig(),
    component_name="assistant",
)

assistant.set_component_prompt(
    template=(
        "You are a friendly assistant. If the user asks about the current "
        "weather, temperature, or wind in a location, call the "
        "``get_weather`` tool with the city name and answer using the data "
        "it returns. The user said: {{question}}"
    )
)


# -- Custom tool: live weather via Open-Meteo --
def get_weather(city: str) -> Dict[str, Union[str, float]]:
    """Look up the current weather for *city* via the Open-Meteo public API."""
    geo = httpx.get(
        "https://geocoding-api.open-meteo.com/v1/search",
        params={"name": city, "count": 1},
        timeout=10.0,
    ).json()
    if not geo.get("results"):
        return {"error": f"could not find location '{city}'"}

    place = geo["results"][0]
    lat, lon = place["latitude"], place["longitude"]

    weather = httpx.get(
        "https://api.open-meteo.com/v1/forecast",
        params={
            "latitude": lat,
            "longitude": lon,
            "current": "temperature_2m,wind_speed_10m",
        },
        timeout=10.0,
    ).json()["current"]

    return {
        "city": place.get("name", city),
        "country": place.get("country", ""),
        "temperature_c": weather["temperature_2m"],
        "wind_speed_kmh": weather["wind_speed_10m"],
    }


get_weather_description = {
    "type": "function",
    "function": {
        "name": "get_weather",
        "description": (
            "Look up the current weather (temperature in Celsius and wind "
            "speed in km/h) for a given city. Call this whenever the user "
            "asks about live weather conditions anywhere in the world."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "city": {
                    "type": "string",
                    "description": (
                        "City name, optionally with country (e.g. 'Berlin', "
                        "'Tokyo', 'Paris, France')."
                    ),
                },
            },
            "required": ["city"],
        },
    },
}

assistant.register_tool(
    tool=get_weather,
    tool_description=get_weather_description,
    send_tool_response_to_model=True,
)


# -- Launch --
launcher = Launcher()
launcher.add_pkg(components=[assistant])
launcher.bringup()
```

---

## When to use which pattern

| Need | Pattern |
|---|---|
| Use a tool that's already a `@component_action` on a managed component, with its existing schema. | `memory.register_tools_on(llm, tools=[...])` from [GoTo Navigation](goto-navigation.md). |
| Custom Python function as the tool, with a schema you define. | `llm.register_tool(your_function, your_description, send_tool_response_to_model=...)` -- this recipe. |
| Multiple capabilities orchestrated by an LLM that decides what to do. | A [Cortex](../../intelligence/cortex.md) component, which auto-discovers every `@component_action` on every managed component as a tool with no per-tool registration. |

And within the custom-function pattern itself:

| `send_tool_response_to_model` | When |
|---|---|
| `True` (this recipe) | The LLM should *react* to the tool's output -- compose a sentence, decide what to do next, ask a follow-up. Conversational and reasoning agents. |
| `False` ([GoTo Navigation](goto-navigation.md)) | The tool's return value is what should be published. You're using the LLM to extract structured arguments, and the function does the real work. |
