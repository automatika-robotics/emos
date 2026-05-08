# Models

Clients in EmbodiedAgents take as input a **model** or **vector database (DB)** specification. These are in most cases generic wrappers around a class of models or databases (e.g. Transformers-based LLMs) defined as [attrs](https://www.attrs.org/en/stable/) classes and include initialization parameters such as quantization schemes, inference options, embedding model (in case of vector DBs) etc. These specifications aim to standardize model initialization across diverse deployment platforms.

## Available Model Wrappers

```{list-table}
:widths: 20 80
:header-rows: 1
* - Model Name
  - Description

* - **GenericLLM**
  - A generic wrapper for LLMs served via OpenAI-compatible `/v1/chat/completions` APIs (e.g., vLLM, LMDeploy, OpenAI). Supports configurable inference options like temperature and max tokens. This wrapper must be used with the **GenericHTTPClient**.

* - **GenericMLLM**
  - A generic wrapper for Multimodal LLMs (Vision-Language models) served via OpenAI-compatible APIs. Supports image inputs alongside text. This wrapper must be used with the **GenericHTTPClient**.

* - **GenericTTS**
  - A generic wrapper for Text-to-Speech models served via OpenAI-compatible `/v1/audio/speech` APIs. Supports voice selection (`voice`) and speed (`speed`) configuration. This wrapper must be used with the **GenericHTTPClient**.

* - **GenericSTT**
  - A generic wrapper for Speech-to-Text models served via OpenAI-compatible `/v1/audio/transcriptions` APIs. Supports language hints (`language`) and temperature settings. This wrapper must be used with the **GenericHTTPClient**.

* - **OllamaModel**
  - A LLM/VLM model loaded from an Ollama checkpoint. Supports configurable generation and deployment options available in Ollama API. Complete list of Ollama models [here](https://ollama.com/library). This wrapper must be used with the **OllamaClient**.

* - **TransformersLLM**
  - LLM models from HuggingFace/ModelScope based checkpoints. Supports quantization ("4bit", "8bit") specification. This model wrapper can be used with the **GenericHTTPClient** or any of the RoboML clients.

* - **TransformersMLLM**
  - Multimodal LLM models from HuggingFace/ModelScope checkpoints for image-text inputs. Supports quantization. This model wrapper can be used with the **GenericHTTPClient** or any of the RoboML clients.

* - **LeRobotPolicy**
  - Provides an interface for loading and running LeRobot policies -- vision-language-action (VLA) models trained for robotic manipulation tasks. Supports automatic extraction of feature and action specifications directly from dataset metadata, as well as flexible configuration of policy behavior. The policy can be instantiated from any compatible LeRobot checkpoint hosted on HuggingFace, making it easy to load pretrained models such as `smolvla_base` or others. This wrapper must be used with the gRPC-based **LeRobotClient**.

* - **RoboBrain2**
  - [RoboBrain 2.0 by BAAI](https://github.com/FlagOpen/RoboBrain2.0) supports interactive reasoning with long-horizon planning and closed-loop feedback, spatial perception for precise point and bbox prediction from complex instructions, and temporal perception for future trajectory estimation. Checkpoint defaults to `"BAAI/RoboBrain2.0-3B"`; larger variants (7B, 32B) are at the same [collection](https://huggingface.co/collections/BAAI/robobrain20-6841eeb1df55c207a4ea0036). This wrapper can be used with any of the RoboML clients.

* - **Whisper**
  - OpenAI's automatic speech recognition (ASR) model served via [faster-whisper](https://github.com/SYSTRAN/faster-whisper). Default checkpoint `"small.en"`; configurable `compute_type` (`"int8"`, `"float16"`, `"float32"`). Available on the [RoboML](https://github.com/automatika-robotics/roboml) platform and can be used with any RoboML client. Recommended: **RoboMLWSClient**.

* - **TransformersTTS**
  - A unified wrapper for HuggingFace Transformers TTS models. Automatically detects whether to use generative inference (Bark, SpeechT5) or forward-only inference (VITS). Default checkpoint `"facebook/mms-tts-eng"`; other examples include `"suno/bark-small"` (Bark) and `"microsoft/speecht5_tts"` (SpeechT5, paired with `vocoder_checkpoint="microsoft/speecht5_hifigan"`). For Bark, `voice` selects a voice preset (e.g. `"v2/en_speaker_6"`). Available on the [RoboML](https://github.com/automatika-robotics/roboml) platform and can be used with any RoboML client. Recommended: **RoboMLWSClient**.

* - **VisionModel**
  - A generic wrapper for object detection and tracking models from [HuggingFace Transformers](https://huggingface.co/models?pipeline_tag=object-detection) (RT-DETR, DETR, Grounding DINO, YOLOS, etc.). Default checkpoint `"PekingU/rtdetr_r50vd_coco_o365"`. Tracking via [ByteTrack](https://github.com/roboflow/trackers) is included; enable with `setup_trackers=True` and tune via `tracking_distance_threshold`. Available on the [RoboML](https://github.com/automatika-robotics/roboml) platform and can be used with any RoboML client. Recommended: **RoboMLRESPClient**.
```

## Built-in Local Models

EmbodiedAgents includes lightweight models that run directly on the robot without needing an external model server (Ollama, RoboML, etc.). These are ideal for offline operation, edge deployment, or as automatic fallbacks when a remote server becomes unavailable.

```{list-table}
:widths: 15 15 15 30 25
:header-rows: 1

* - Local Model
  - Component
  - Framework
  - Default Checkpoint
  - Dependency

* - **LocalLLM**
  - LLM
  - llama-cpp-python
  - Qwen/Qwen3-0.6B-GGUF
  - `pip install llama-cpp-python`

* - **LocalVLM**
  - VLM
  - llama-cpp-python
  - ggml-org/moondream2-20250414-GGUF
  - `pip install llama-cpp-python`

* - **LocalSTT**
  - SpeechToText
  - sherpa-onnx
  - csukuangfj/sherpa-onnx-whisper-tiny.en
  - `pip install sherpa-onnx`

* - **LocalTTS**
  - TextToSpeech
  - sherpa-onnx
  - csukuangfj/kokoro-en-v0_19
  - `pip install sherpa-onnx`

* - **LocalVision**
  - Vision
  - onnxruntime
  - DEIM detector
  - `pip install onnxruntime`
```

```{note}
- Local models can be activated via `enable_local_model=True` in the component config or `fallback_to_local()` action.
- Model weights are auto-downloaded from HuggingFace on first use. To use a custom model, set `local_model_path` in the component config.
- GPU-accelerated variants are available for `llama-cpp-python` (CUDA/Metal builds) and `onnxruntime` (`onnxruntime-gpu`).
- Dependencies are pre-installed in EMOS Docker containers. For native installations, install them manually as shown above.
```

## Available Vector Databases

```{list-table}
:widths: 20 80
:header-rows: 1
* - Vector DB
  - Description

* - **ChromaDB**
  - [Chroma](https://www.trychroma.com/) is an open-source AI application database with support for vector search, full-text search, and multi-modal retrieval. Supports "ollama" and "sentence-transformers" embedding backends. Can be used with the **ChromaClient**.
```

````{note}
For `ChromaDB`, make sure you install required packages:

```bash
pip install ollama  # For Ollama backend (requires Ollama runtime)
pip install sentence-transformers  # For Sentence-Transformers backend
```
````

To use Ollama embedding models ([available models](https://ollama.com/search?c=embedding)), ensure the Ollama server is running and accessible via specified `host` and `port`.
