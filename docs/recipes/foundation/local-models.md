# Local Models

EMOS can run all AI components entirely on-device using built-in local models. No Ollama, no RoboML, no cloud API — just set `enable_local_model=True` in the component config and you're running inference locally.

This is useful for:

- **Offline robots** that operate without network access
- **Edge deployment** where latency to a remote server is unacceptable
- **Quick prototyping** when you don't want to set up a model serving platform

```{note}
Models are auto-downloaded from HuggingFace on first use. Subsequent runs load from cache.
```

## Dependencies

Local models require one additional pip package depending on the component type:

- **LLM / VLM**: `pip install llama-cpp-python`
- **STT / TTS**: `pip install sherpa-onnx`

These are pre-installed in EMOS Docker containers.

## Local LLM

The simplest possible EMOS agent — an LLM running entirely on-device with no model client:

```python
from agents.components import LLM
from agents.config import LLMConfig
from agents.ros import Topic, Launcher

config = LLMConfig(
    enable_local_model=True,
    device_local_model="cpu",  # or "cuda" for GPU
    ncpu_local_model=4,
)

query = Topic(name="user_query", msg_type="String")
response = Topic(name="response", msg_type="String")

llm = LLM(
    inputs=[query],
    outputs=[response],
    config=config,
    trigger=query,
    component_name="local_brain",
)

launcher = Launcher()
launcher.add_pkg(components=[llm])
launcher.bringup()
```

The default model is **Qwen3 0.6B** (GGUF format). To use a different model, set `local_model_path` to any HuggingFace GGUF repo ID or a local file path:

```python
config = LLMConfig(
    enable_local_model=True,
    local_model_path="Qwen/Qwen3-1.7B-GGUF",  # larger model
)
```

## Local VLM

A vision-language model that processes both text and images on-device:

```python
from agents.components import VLM
from agents.config import VLMConfig
from agents.ros import Topic, Launcher

config = VLMConfig(enable_local_model=True)

text_in = Topic(name="text_query", msg_type="String")
image_in = Topic(name="image_raw", msg_type="Image")
text_out = Topic(name="response", msg_type="String")

vlm = VLM(
    inputs=[text_in, image_in],
    outputs=[text_out],
    config=config,
    trigger=text_in,
    component_name="local_vision",
)

launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

The default model is **Moondream2** (GGUF format).

```{warning}
Streaming output (`stream=True`) is not supported with local VLM models. The component will return the complete response once inference finishes.
```

## Local Speech-to-Text

Convert spoken audio to text using an on-device Whisper model:

```python
from agents.components import SpeechToText
from agents.config import SpeechToTextConfig
from agents.ros import Topic, Launcher

config = SpeechToTextConfig(
    enable_local_model=True,
    enable_vad=True,  # voice activity detection
)

audio_in = Topic(name="audio0", msg_type="Audio")
text_out = Topic(name="transcription", msg_type="String")

stt = SpeechToText(
    inputs=[audio_in],
    outputs=[text_out],
    config=config,
    trigger=audio_in,
    component_name="local_stt",
)

launcher = Launcher()
launcher.add_pkg(components=[stt])
launcher.bringup()
```

The default model is **Whisper tiny.en** (via sherpa-onnx). For other languages or larger models, see the [sherpa-onnx pretrained models](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/index.html) and set `local_model_path` accordingly.

```{warning}
Streaming output (`stream=True`) is not supported with local STT models. Use a WebSocket client (e.g. RoboMLWSClient) if you need streaming transcription.
```

## Local Text-to-Speech

Synthesize speech on-device and play it through the robot's speakers:

```python
from agents.components import TextToSpeech
from agents.config import TextToSpeechConfig
from agents.ros import Topic, Launcher

config = TextToSpeechConfig(
    enable_local_model=True,
    play_on_device=True,  # play audio on the robot's speakers
)

text_in = Topic(name="text_input", msg_type="String")

tts = TextToSpeech(
    inputs=[text_in],
    outputs=[],
    config=config,
    trigger=text_in,
    component_name="local_tts",
)

launcher = Launcher()
launcher.add_pkg(components=[tts])
launcher.bringup()
```

The default model is **Kokoro EN** (via sherpa-onnx).

```{warning}
Streaming output (`stream=True`) is not supported with local TTS models.
```

## Complete Example: Local Conversational Agent

Here is a full conversational agent — speech-to-text, vision-language model, and text-to-speech — running entirely on local models. No Ollama, no RoboML, no cloud API.

```{code-block} python
:caption: Fully Local Conversational Agent
:linenos:
from agents.components import VLM, SpeechToText, TextToSpeech
from agents.config import SpeechToTextConfig, VLMConfig, TextToSpeechConfig
from agents.ros import Topic, Launcher

# --- Speech-to-Text (Whisper tiny.en via sherpa-onnx) ---
audio_in = Topic(name="audio0", msg_type="Audio")
text_query = Topic(name="text0", msg_type="String")

stt_config = SpeechToTextConfig(
    enable_local_model=True,
    enable_vad=True,
)

speech_to_text = SpeechToText(
    inputs=[audio_in],
    outputs=[text_query],
    config=stt_config,
    trigger=audio_in,
    component_name="speech_to_text",
)

# --- VLM (Moondream2 via llama-cpp-python) ---
image_in = Topic(name="image_raw", msg_type="Image")
text_answer = Topic(name="text1", msg_type="String")

vlm_config = VLMConfig(enable_local_model=True)

vlm = VLM(
    inputs=[text_query, image_in],
    outputs=[text_answer],
    config=vlm_config,
    trigger=text_query,
    component_name="vision_brain",
)

# --- Text-to-Speech (Kokoro via sherpa-onnx) ---
tts_config = TextToSpeechConfig(
    enable_local_model=True,
    play_on_device=True,
)

text_to_speech = TextToSpeech(
    inputs=[text_answer],
    outputs=[],
    config=tts_config,
    trigger=text_answer,
    component_name="text_to_speech",
)

# --- Launch ---
launcher = Launcher()
launcher.add_pkg(components=[speech_to_text, vlm, text_to_speech])
launcher.bringup()
```

This recipe creates the same pipeline as the [Conversational Agent](conversational-agent.md) tutorial, but runs fully offline. The trade-off is that on-device models are smaller and less capable than hosted alternatives — they work well for simple interactions but may struggle with complex reasoning.

```{seealso}
- [Built-in Local Models](../../intelligence/models.md#built-in-local-models) for the full table of default models and configuration options.
- [Fallback Recipes](../events-and-resilience/fallback-recipes.md) for using local models as automatic fallbacks when a remote server fails.
- [Conversational Agent](conversational-agent.md) for the server-based version using Ollama and RoboML.
```
