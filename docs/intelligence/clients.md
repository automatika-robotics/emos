# Inference Clients

A component that uses a model does not run the model itself. It talks to a client, and the client talks to wherever the model is: a serving platform on a compute node in the network, a cloud API, or Ollama on the robot. The component's code is the same in every case, so the choice of where a model runs, and the latency and accuracy that come with it, is made in the recipe by picking a client, not by changing a component. Components that use a vector database take a database client in the same way.

Every client checks its connection when the component starts. Model clients run inference, and can initialise and release a model, which is what lets a recipe switch models on an event. Database clients implement the usual create, read, update and delete operations.

```{note}
Some clients need an extra package, listed in the table. When it is missing, the component says so when it starts.
```

```{list-table}
:widths: 20 20 60
:header-rows: 1
* - Platform
  - Client
  - Description

* - **Generic**
  - GenericHTTPClient
  - Any OpenAI-compatible API: vLLM, llama.cpp, SGLang, Google Gemini, OpenAI itself and the like. Handles standard and streaming responses, language and multimodal models, speech in both directions, and tool calling. It can also be used for (System 1) decision models for typed questions.

* - **RoboML**
  - RoboMLHTTPClient
  - HTTP client for models served on [RoboML](https://github.com/automatika-robotics/roboml). Streams outputs.

* - **RoboML**
  - RoboMLWSClient
  - WebSocket client for a persistent connection to RoboML, the right choice for low-latency streaming of audio or text. Uses `wss` when the host is `https`.

* - **RoboML**
  - RoboMLRESPClient
  - Redis-protocol client for RoboML. Needs `pip install redis[hiredis]`.

* - **Ollama**
  - OllamaClient
  - HTTP client for models served on [Ollama](https://ollama.com): language and multimodal models and embeddings, with tool calling. Needs `pip install ollama`.

* - **LeRobot**
  - LeRobotClient
  - gRPC client for vision-language-action policies served by LeRobot's async policy server, version 0.6.0 or later, which covers every policy the server can host, GR00T included. The host is a bare address without a scheme. Needs `pip install grpcio` and a PyTorch build, for instance `pip install torch --index-url https://download.pytorch.org/whl/cpu`.

* - **ChromaDB**
  - ChromaClient
  - HTTP client for a ChromaDB server, started with `pip install chromadb` and `chroma run --path /db_path`.
```

The clients also differ in what they can do for a component that keeps a conversation. Such a component trims the conversation so that it fits in the model's context window, and it can only do that when it knows how big the window is. The Ollama client and the built-in local models tell it, so nothing needs to be configured. An OpenAI-compatible server or RoboML gives no such figure, so with `GenericHTTPClient` or a RoboML client the size has to be set in the component's config, as `history_budget`, or the conversation is only limited by its number of turns. RoboML has two more limits of its own: it takes images with the current message only, so earlier turns reach it without their pictures, and over its websocket client the turns dropped from a conversation cannot be summarized, so they are simply forgotten.

## Keys and certificates

A client that reaches a cloud API needs a key, and the key never goes into the recipe. `GenericHTTPClient` reads it from an environment variable, `OPENAI_API_KEY` unless `api_key_env` names another, in the process the component runs in, so the key is neither written into the recipe nor serialised with it. Naming a variable that is not set fails at once rather than at the first request.

```python
from agents.clients import GenericHTTPClient
from agents.models import GenericMLLM

vlm_client = GenericHTTPClient(
    model=GenericMLLM(name="vlm", checkpoint="Qwen/Qwen3-VL-8B-Instruct"),
    host="https://models.example.com",      # a host with a scheme is used as given
    api_key_env="MODELS_API_KEY",
)
```

Every client takes a `ca_cert`, the path of a PEM certificate, for a self-hosted server with a certificate of its own. And every client warns in the log when it connects unencrypted to anything but the local machine, with a second warning from `GenericHTTPClient` when a key would travel in the clear.

```{seealso}
- [Models](models.md) for the model and database specifications a client is given.
- [Local Models](../recipes/foundation/local-models.md) for running without any server at all.
```
