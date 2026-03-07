# EMOS CLI

The `emos` CLI is the main entry point for managing and running recipes on your robot. It handles installation, recipe discovery, download, and execution across container, native, and licensed deployment modes.

## Quick Reference

| Command | Description |
| :--- | :--- |
| `emos install` | Install EMOS (interactive mode selection) |
| `emos update` | Update EMOS to the latest version |
| `emos status` | Show installation status |
| `emos recipes` | List recipes available for download |
| `emos pull <name>` | Download a recipe |
| `emos ls` | List locally installed recipes |
| `emos run <name>` | Run a recipe |
| `emos map <cmd>` | Mapping tools (record, edit) |
| `emos version` | Show CLI version |

## Typical Workflow

```bash
# 1. Install EMOS (one-time setup)
emos install

# 2. Browse available recipes
emos recipes

# 3. Download one
emos pull vision_follower

# 4. Run it
emos run vision_follower
```

## Running Recipes

When you run `emos run <name>`, the CLI adapts its behavior to your installation mode:

**Container mode** (oss-container and licensed):

1. Starts the EMOS Docker container
2. Configures the ROS 2 middleware (Zenoh by default)
3. Launches robot hardware drivers and sensors (licensed mode only)
4. Verifies sensor topics are publishing
5. Executes the recipe inside the container

**Native mode:**

1. Verifies the ROS 2 environment and EMOS workspace
2. Configures the ROS 2 middleware
3. Verifies sensor topics are publishing
4. Executes the recipe directly on the host

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
  "sensor_topics": {
    "camera": ["/camera/image_raw"],
    "lidar": ["/scan"]
  },
  "autonomous_mode": false,
  "web_client": true,
  "rmw": "rmw_zenoh_cpp"
}
```

- {material-regular}`sensors;1.2em;sd-text-primary` **sensors**: List of sensor drivers to launch (e.g. `"camera"`, `"lidar"`). In licensed mode, EMOS looks for a matching `bringup_<sensor>.py` launch file. In container and native modes, sensors must be running externally.
- {material-regular}`topic;1.2em;sd-text-primary` **sensor_topics**: Maps each sensor to its expected ROS 2 topics. Used for verification in container and native modes.
- {material-regular}`gamepad;1.2em;sd-text-primary` **autonomous_mode**: Set to `true` if the recipe commands the robot to move.
- {material-regular}`web;1.2em;sd-text-primary` **web_client**: Set to `true` to start the auto-generated web UI.
- {material-regular}`settings;1.2em;sd-text-primary` **rmw**: ROS 2 middleware implementation to use.

### recipe.py

This is a standard EMOS Python script, the same code you write in the tutorials:

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
emos install                          # Interactive mode selection
emos install --mode container         # Container mode (no ROS required)
emos install --mode native            # Native mode (requires ROS 2)
emos install YOUR-LICENSE-KEY         # Licensed mode
```

Flags:

- `--mode`: Installation mode (`container`, `native`, or `licensed`)
- `--distro`: ROS 2 distribution (`jazzy`, `humble`, or `kilted`)

### emos update

```bash
emos update
```

Detects your installation mode and updates accordingly. Container modes pull the latest image and recreate the container. Native mode pulls the latest source and rebuilds the workspace.

### emos status

```bash
emos status
```

Displays the current installation mode, ROS 2 distro, and status. For container modes, shows whether the container is running. For native mode, shows ROS 2 availability.

### emos pull

```bash
emos pull <recipe_short_name>
```

Downloads a recipe from the Automatika recipe server and extracts it to `~/emos/recipes/<name>/`. Overwrites the existing version if present.

### emos run

```bash
emos run <recipe_short_name>
```

Launches a locally installed recipe. Optional flags:

```bash
emos run my_recipe --rmw rmw_cyclonedds_cpp    # Override RMW middleware
emos run my_recipe --skip-sensor-check         # Skip sensor topic verification
```

Supported RMW values: `rmw_zenoh_cpp` (default), `rmw_fastrtps_cpp`, `rmw_cyclonedds_cpp`.

### emos map

Mapping subcommands for creating and editing environment maps:

```bash
emos map record            # Record mapping data on the robot
emos map install-editor    # Install the map editor container
emos map edit <file.tar.gz>  # Process a ROS bag into a PCD map
```
