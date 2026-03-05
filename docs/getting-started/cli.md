# EMOS CLI

The `emos` CLI is the main entry point for managing and running recipes on your robot. It handles environment setup, recipe discovery, download, and execution -- all from the terminal.

## Quick Reference

| Command | Description |
| :--- | :--- |
| `emos install <key>` | Install EMOS using your license key |
| `emos update` | Update the CLI and EMOS container |
| `emos status` | Show container status |
| `emos recipes` | List recipes available for download |
| `emos pull <name>` | Download a recipe |
| `emos ls` | List locally installed recipes |
| `emos run <name>` | Run a recipe |
| `emos map <cmd>` | Mapping tools (record, edit) |

## Typical Workflow

```bash
# 1. Install EMOS (one-time setup)
emos install YOUR-LICENSE-KEY

# 2. Browse available recipes
emos recipes

# 3. Download one
emos pull vision_follower

# 4. Run it
emos run vision_follower
```

## Running Recipes

When you run `emos run <name>`, the CLI:

1. Starts the EMOS Docker container
2. Configures the ROS2 middleware (Zenoh by default)
3. Launches robot hardware drivers and sensors declared in the recipe's `manifest.json`
4. Verifies all required ROS2 nodes are active
5. Executes the recipe's `recipe.py` inside the container

All output is streamed to your terminal and saved to `~/emos/logs/`.

## Writing Custom Recipes

A recipe is a directory under `~/emos/recipes/` with the following structure:

```
~/emos/recipes/
  my_recipe/
    recipe.py          # Main entry point (required)
    manifest.json      # Declares sensors, modes, and metadata (required)
    *_config.yaml      # Optional per-sensor configuration overrides
```

### manifest.json

The manifest tells EMOS what hardware your recipe needs:

```json
{
  "name": "My Custom Recipe",
  "sensors": ["camera", "lidar"],
  "autonomous_mode": false,
  "web_client": true
}
```

- **sensors**: List of sensor drivers to launch (e.g. `"camera"`, `"lidar"`). EMOS looks for a matching `bringup_<sensor>.py` launch file.
- **autonomous_mode**: Set to `true` if the recipe commands the robot to move.
- **web_client**: Set to `true` to start the auto-generated web UI.

### recipe.py

This is a standard EMOS Python script -- the same code you write in the tutorials:

```python
from agents.clients.ollama import OllamaClient
from agents.components import VLM
from agents.models import OllamaModel
from agents.ros import Topic, Launcher

text_in = Topic(name="text0", msg_type="String")
image_in = Topic(name="image_raw", msg_type="Image")
text_out = Topic(name="text1", msg_type="String")

model = OllamaModel(name="qwen_vl", checkpoint="qwen2.5vl:latest")
client = OllamaClient(model)

vlm = VLM(
    inputs=[text_in, image_in],
    outputs=[text_out],
    model_client=client,
    trigger=text_in,
)

launcher = Launcher()
launcher.add_pkg(components=[vlm])
launcher.bringup()
```

### Sensor Configuration Overrides

If a sensor needs non-default parameters, place a config file named `<sensor>_config.yaml` (or `.json` / `.toml`) in your recipe directory. EMOS will pass it to the sensor's launch file automatically.

### Running Your Custom Recipe

Once your recipe directory is in place:

```bash
emos ls              # Verify it appears
emos run my_recipe   # Launch it
```

## Command Details

### emos install

```bash
emos install <license_key>
```

Validates your license, pulls the EMOS Docker image, starts the container, and creates a systemd service for auto-restart on boot. Run this once on each machine.

### emos update

```bash
emos update
```

Updates the CLI tools, re-validates your license, pulls the latest container image, and redeploys robot configuration files.

### emos pull

```bash
emos pull <recipe_short_name>
```

Downloads a recipe from the Automatika recipe server and extracts it to `~/emos/recipes/<name>/`. Overwrites the existing version if present.

### emos run

```bash
emos run <recipe_short_name>
```

Launches a locally installed recipe. Accepts an optional `--rmw_implementation` argument to override the default ROS2 middleware:

```bash
emos run my_recipe --rmw_implementation=rmw_cyclonedds_cpp
```

Supported values: `rmw_zenoh_cpp` (default), `rmw_fastrtps_cpp`, `rmw_cyclonedds_cpp`.

### emos map

Mapping subcommands for creating and editing environment maps:

```bash
emos map record            # Record mapping data on the robot
emos map install-editor    # Install the map editor container
emos map edit <file.tar.gz>  # Process a ROS bag into a PCD map
```
