# Memory

**The first memory system for embodied agents built on neuroscience principles.** Memory is the EmbodiedAgents component that gives a robot a persistent, queryable sense of *place* and *history*. It is built on top of [eMEM](https://github.com/automatika-robotics/emem) -- a hybrid graph-based spatio-temporal memory designed specifically for situated agents. The core problem eMEM solves is the false dichotomy that has dominated robot memory until now:

> Vector databases discard spatial structure. Metric maps discard semantics. **eMEM unifies both.**

Memory accumulates a typed graph of everything the robot perceives -- detections, scene descriptions, sensor strings, internal-state readings -- indexed simultaneously by **meaning** (HNSW), **location** (R-tree), and **time** (SQLite indexes). Episodes group observations into task spans. Consolidation collapses old observations into searchable gists. Entity nodes track persistent objects across episodes via semantic-spatial merging. And **interoception** -- the robot's own internal state -- is a first-class memory dimension alongside world observations, not a separate metrics pipeline.

```{important}
`Memory` is the supported successor to **MapEncoding**, deprecated since EmbodiedAgents 0.7.1. The two are not API-compatible: Memory replaces a flat vector-DB store with episodic consolidation, entity tracking, and an interoception surface. See [Migration](#migration-from-mapencoding) below.
```

```{seealso}
For a hands-on intro to Memory by itself, see [Spatio-Temporal Memory](../recipes/foundation/semantic-map.md). For the deep pairing with Cortex (the agentic harness), see [Memory and Cortex](../recipes/planning-and-manipulation/cortex-memory.md). For eMEM internals, see the [eMEM repository](https://github.com/automatika-robotics/emem).
```

```{admonition} Installation
:class: note

`Memory` depends on the [eMEM](https://github.com/automatika-robotics/emem) package and raises `ImportError` at construction if it isn't available. Install it into the same environment as the EMOS launcher: `pip install emem`.
```

---

## Why it works

eMEM's design is grounded in how humans and other animals actually structure memory.

- {material-regular}`schema;1.2em;sd-text-primary` **Tiered consolidation** -- observations flow `working → short-term → long-term → archived`, mirroring the consolidation hierarchy seen in mammalian memory. Recent experience stays raw and queryable; older experience compresses into gists; ancient experience is archived (raw text removed, gist preserved). This bounds storage growth without losing the *meaning* of past activity.

- {material-regular}`account_tree;1.2em;sd-text-primary` **Episodic structure** -- experience is grouped into named episodes that can nest hierarchically (`SUBTASK_OF`) and chain temporally (`FOLLOWS`). When an episode ends, its observations are clustered, summarised by an LLM into a *gist* that preserves the semantic content, the spatial centroid, and the time span -- and the raw observations are archived.

- {material-regular}`hub;1.2em;sd-text-primary` **Entity persistence** -- detections of the same object across multiple episodes are auto-merged into a single `EntityNode` via semantic similarity (cosine) *and* spatial proximity (range threshold). The first time the robot sees "the red chair near the door", it's a new entity; the second time, it's recognised as the same one. `COOCCURS_WITH` edges capture which entities tend to be observed together.

- {material-regular}`favorite;1.2em;sd-text-primary` **Interoception as memory** -- internal body state (battery, temperature, joint health, fault flags) is stored in the same graph as world observations, not in a separate telemetry stream. This is what lets a query like *"what was happening when the battery dropped?"* work -- the spatial-temporal-interoceptive associations emerge naturally because everything lives in the same graph.

The whole stack runs **fully embedded** -- SQLite, hnswlib, and Rtree, with zero external services. A single `.db` file plus a `.hnsw.bin` file *is* the entire memory state. Reboot the robot, point it at the same files, and the prior session is remembered.

---

## Architecture at a glance

```
                    +-----------------+
                    |  Memory         |  (EmbodiedAgents component)
                    +--------+--------+
                             |  delegates to
                    +--------v--------+
                    | SpatioTemporal  |
                    |     Memory      |  (eMEM facade)
                    +--------+--------+
                             |
              +--------------+--------------+
              |              |              |
     +--------v--+   +-------v-----+  +-----v------+
     |  Working   |  |   Memory    |  | Consolida- |
     |  Memory    |  |   Tools     |  | tion       |
     |  (buffer)  |  | (10 tools)  |  | Engine     |
     +--------+---+  +------+------+  +----+-------+
              |              |              |
              +--------------+--------------+
                             |
                    +--------v--------+
                    |   MemoryStore   |
                    +--------+--------+
                             |
              +--------------+--------------+
              |              |              |
        +-----v----+   +-----v-----+  +-----v----+
        |  SQLite   |  |  hnswlib  |  |  R-tree  |
        | (nodes,   |  | (vector   |  | (spatial |
        |  edges,   |  |  search)  |  |  index)  |
        |  tiers)   |  |           |  |          |
        +-----------+  +-----------+  +----------+
```

Three complementary indexes, one shared graph.

### The graph

Four node types and six edge types compose the memory graph:

**Nodes**

| Node | Carries |
|---|---|
| `ObservationNode` | A single perception event: text, coordinates, timestamp, layer, source_type (`perception` or `interoception`), confidence. |
| `EpisodeNode` | A named task / activity span grouping related observations. |
| `GistNode` | A consolidated summary of multiple observations, with spatial extent and time range. Survives archival of the raw observations. |
| `EntityNode` | A persistent tracked object / landmark (auto-merged across episodes by similarity + spatial proximity). |

**Edges**

| Edge | Meaning |
|---|---|
| `BELONGS_TO` | Observation → Episode |
| `FOLLOWS` | Episode → Episode (temporal sequence) |
| `SUBTASK_OF` | Episode → Episode (hierarchical nesting) |
| `SUMMARIZES` | Gist → Observation(s) |
| `OBSERVED_IN` | Entity → Observation |
| `COOCCURS_WITH` | Entity ↔ Entity |

### Tiers

```
 working  →  short_term  →  long_term  →  archived
 (buffer)    (in store)     (promoted)    (text dropped,
                                           gist remains)
```

When an episode ends, eMEM consolidates its observations into a gist and archives them. For non-episodic streams that age past `consolidation_window`, time-window consolidation uses **DBSCAN** to spatially cluster old observations before summarising each cluster.

### Unified search

`semantic_search` queries observations and gists through a single HNSW lookup. After consolidation, knowledge isn't lost -- it's compressed into gists that remain searchable alongside recent observations.

---

## Public API

```python
from agents.components import Memory
from agents.config import MemoryConfig
from agents.ros import MemLayer, Topic
```

### Layers

A `MemLayer` is the smallest unit of input. Each layer subscribes to one topic; the topic's UI string representation is what gets stored.

```python
detections = Topic(name="detections", msg_type="Detections")
scene      = Topic(name="scene_description", msg_type="String")
battery    = Topic(name="/battery_level", msg_type="Float32")

# Perception layers
detections_layer = MemLayer(subscribes_to=detections, temporal_change=True)
scene_layer      = MemLayer(subscribes_to=scene)

# Interoception layer
battery_layer    = MemLayer(subscribes_to=battery, is_internal_state=True)
```

| Field | Meaning |
|---|---|
| `subscribes_to` | The topic to ingest from. |
| `temporal_change` | If `True`, observations on the same spatial position are timestamped and stored as a sequence. Useful for objects that move or reappear. |
| `is_internal_state` | If `True`, observations are routed through `add_body_state` -- they're invisible to perception retrieval tools and surface only through the dedicated `body_status` tool. Used for interoception. |

`MapLayer` is preserved as a backwards-compatible alias of `MemLayer`.

### Construction

```python
position = Topic(name="/odom", msg_type="Odometry")

memory = Memory(
    layers=[detections_layer, scene_layer, battery_layer],
    position=position,
    model_client=vlm_client,             # used for episode-consolidation summaries + entity extraction
    embedding_client=embedding_client,   # used for vector indexing
    config=MemoryConfig(db_path="/tmp/robot_memory.db"),
    trigger=10.0,                        # flush layer data every 10s
    component_name="memory",
)
```

| Argument | Purpose |
|---|---|
| `layers` | One `MemLayer` per perception or interoception topic. |
| `position` | An Odometry topic. Memory tags every observation with the robot's current `(x, y, z)` so spatial queries work. |
| `model_client` | Optional. Drives summarisation during episode consolidation and entity extraction. Without it, consolidation falls back to plain text concatenation. |
| `embedding_client` | Optional. Drives vector indexing (e.g. an Ollama client serving an embedding model). Without it, falls back to `sentence-transformers`. |
| `config` | A `MemoryConfig` -- see below. |
| `trigger` | When to flush observations to the working buffer. A topic, list of topics, frequency in Hz, or `Event`. |

### Configuration highlights

`MemoryConfig` exposes ~20 fields covering storage, consolidation, entity merging, and HNSW index tuning. The most reached-for ones in everyday recipes:

| Field | Default | Controls |
|---|---|---|
| `db_path` | `"memory.db"` | eMEM SQLite database path. The whole memory state lives in this file. |
| `auto_store` | `True` | Flush observations on every execution step. Set `False` to require explicit `store` calls. |
| `working_memory_size` | `50` | Buffer size before older observations are dropped. |
| `flush_interval` / `flush_batch_size` | `2.0s` / `5` | How aggressively the buffer is persisted. |
| `consolidation_window` | `1800.0s` | Maximum gap between observations in the same consolidation chunk. |
| `consolidation_spatial_eps` | `3.0m` | DBSCAN epsilon for time-window consolidation. |
| `archive_after_seconds` | `3600.0s` | When raw observation text is dropped; the gist remains searchable. |
| `entity_similarity_threshold` | `0.85` | Cosine threshold for merging a new detection with a known entity. |
| `entity_spatial_radius` | `5.0m` | Spatial radius for the same merge. |
| `recency_weight` | `0.0` | Boost recent observations in semantic search. `0.0` for pure semantic ordering. |

---

## Retrieval surface (10 tools)

Memory exposes ten retrieval tools, all decorated as `@component_action`. Nine are `phase=ActionPhase.PLANNING` (Cortex consumes them while *building* a plan); `body_status` is `ActionPhase.BOTH` because the executor may also need it at runtime.

| Tool | Purpose |
|---|---|
| `semantic_search` | Find observations by meaning. Queries observations *and* gists in a single HNSW lookup. |
| `spatial_query` | Find observations within a radius of a point. R-tree backed. |
| `temporal_query` | Find observations in a time range. Accepts relative strings like `"-10m"`. |
| `episode_summary` | Get the consolidated summary of one or more episodes. |
| `get_current_context` | Situational awareness -- nearby objects, area summaries, recent activity, latest body status. |
| `search_gists` | Search consolidated long-term memory only (faster than full semantic search). |
| `entity_query` | Find known entities by name, type, or location. |
| `locate` | Resolve a concept to a spatial position (returns centroid + radius from the entity graph). |
| `recall` | Cross-layer recall -- everything known about a concept across observations, gists, and entities. |
| `body_status` | Latest interoception readings, optionally filtered by layer. |

Two write-side actions are exposed as well:

- `store_specific_memory` (execution) -- write an arbitrary string into memory at runtime, e.g. for runtime annotations Cortex wants to preserve.
- `start_episode` / `end_episode` (execution) -- bracket episodes manually instead of relying on time-based consolidation.

When a `Memory` component is in the recipe, [Cortex](cortex.md) auto-discovers all of these and **augments its planning prompt** with detailed task-classification guidance (PERCEPTION QUERY vs BODY QUERY vs ACTION TASK) and instructions for episode wrapping. No tool registration is required.

### Registering tools on a plain LLM

If you want Memory's tools without Cortex (e.g. to give a plain `LLM` component tool-calling access for natural-language Q&A), use `register_tools_on`:

```python
memory.register_tools_on(llm, send_tool_response_to_model=True)
# Or register a subset:
memory.register_tools_on(llm, tools=["semantic_search", "locate", "get_current_context"])
```

---

## Persistence

The entire memory state lives in two files: the SQLite DB at `db_path` and an HNSW index file alongside it. **Reboot the robot, point the next session at the same paths, and the agent picks up where it left off.** Episodes from yesterday are still in the graph; entities are still merged; gists are still searchable. Cross-session continuity is the default, not a feature you opt into.

This is what makes eMEM-on-Cortex behave as a cognitive system rather than a session-scoped vector DB.

---

## Migration from MapEncoding

| MapEncoding | Memory |
|---|---|
| `MapEncoding` component | `Memory` component |
| `MapLayer` | `MemLayer` (alias preserved) |
| Flat vector DB (Chroma) | Graph + episodic + entity index, eMEM-backed |
| `db_client=ChromaClient(...)` | `model_client=...`, `embedding_client=...` |
| `map_topic=OccupancyGrid` (required) | None -- Memory uses real-world coords from Odometry directly |
| `MapConfig(map_name=…)` | `MemoryConfig(db_path=…)` |
| Free-form text retrieval via tool calling | Ten typed retrieval tools auto-registered with Cortex |
| No interoception | `is_internal_state=True` on a layer surfaces it via `body_status` |
| No episode structure | `start_episode` / `end_episode`, hierarchical nesting, automatic consolidation |
| No entity tracking | Persistent `EntityNode` with semantic-spatial auto-merge |
| Session-scoped | Cross-session persistent (single `.db` file) |

Existing recipes built on `MapEncoding` still load with a deprecation warning. New recipes should use `Memory`.

---

## Recipes

- {doc}`Spatio-Temporal Memory <../recipes/foundation/semantic-map>` -- introductory recipe. Memory built from a Vision component's detections and an MLLM's introspective answers, no Cortex on top.
- {doc}`Memory and Cortex <../recipes/planning-and-manipulation/cortex-memory>` -- the deep pairing. Cortex auto-discovers Memory's retrieval surface, the planning prompt is automatically augmented with task classification, the robot becomes addressable as *"how are you?"*, *"what did you see?"*, and *"go remember the cat for me"*.
- {doc}`Cortex Driving the Full Stack <../recipes/planning-and-manipulation/cortex-navigation>` -- adds a Kompass navigation stack on top of the Memory + Cortex pair so the robot can act on memory queries instead of just answering them.
