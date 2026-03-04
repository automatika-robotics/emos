# Inference Clients

Clients are execution backends that instantiate and call inference on ML models. Certain components in the EMOS Intelligence Layer deal with ML models, vector databases, or both. These components take in a model client or DB client as one of their initialization parameters. The reason for this abstraction is to enforce _separation of concerns_. Whether an ML model is running on the edge hardware, on a powerful compute node in the network, or in the cloud, the components running on the robot edge can always use the model (or DB) via a client in a standardized way.

This approach makes components independent of the model serving platforms, which may implement various inference optimizations depending on the model type. As a result, developers can choose an ML serving platform that offers the best latency/accuracy tradeoff based on the application's requirements.

All clients implement a connection check. ML clients must implement inference methods, and optionally model initialization and deinitialization methods. This supports scenarios where an embodied agent dynamically switches between models or fine-tuned versions based on environmental events. Similarly, vector DB clients implement standard CRUD methods tailored to vector databases.

The EMOS Intelligence Layer provides the following clients, designed to cover the most popular open-source model deployment platforms. Creating simple clients for other platforms is straightforward.

```{note}
Some clients may require additional dependencies, which are detailed in the table below. If these are not installed, users will be prompted at runtime.
```

```{list-table}
:widths: 20 20 60
:header-rows: 1
* - Platform
  - Client
  - Description

* - **Generic**
  - GenericHTTPClient
  - A generic client for interacting with OpenAI-compatible APIs, including vLLM, ms-swift, lmdeploy, Google Gemini, etc. Supports both standard and streaming responses, and works with LLMs and multimodal LLMs. Designed to be compatible with any API following the OpenAI standard. Supports tool calling.

* - **RoboML**
  - RoboMLHTTPClient
  - An HTTP client for interacting with ML models served on [RoboML](https://github.com/automatika-robotics/roboml). Supports streaming outputs.

* - **RoboML**
  - RoboMLWSClient
  - A WebSocket-based client for persistent interaction with [RoboML](https://github.com/automatika-robotics/roboml)-hosted ML models. Particularly useful for low-latency streaming of audio or text data.

* - **RoboML**
  - RoboMLRESPClient
  - A Redis Serialization Protocol (RESP) based client for ML models served via [RoboML](https://github.com/automatika-robotics/roboml).
    Requires `pip install redis[hiredis]`.

* - **Ollama**
  - OllamaClient
  - An HTTP client for interacting with ML models served on [Ollama](https://ollama.com). Supports LLMs/MLLMs and embedding models. Supports tool calling.
    Requires `pip install ollama`.

* - **LeRobot**
  - LeRobotClient
  - A gRPC-based asynchronous client for vision-language-action (VLA) policies served on LeRobot Policy Server. Supports various robot action policies available in the LeRobot package by HuggingFace.
    Requires:
    `pip install grpcio`
    `pip install torch --index-url https://download.pytorch.org/whl/cpu`

* - **ChromaDB**
  - ChromaClient
  - An HTTP client for interacting with a ChromaDB instance running as a server.
    Ensure that a ChromaDB server is active using:
    `pip install chromadb`
    `chroma run --path /db_path`
```
