# EMOS CLI

The `emos` CLI is the main entry point for managing and running recipes on your robot. It handles installation, recipe discovery, download, and execution across container and native deployment modes.

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
| `emos info <name>` | Show sensor/topic requirements for a recipe |
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

# 4. Check what sensors it needs
emos info vision_follower

# 5. Run it
emos run vision_follower
```

## Running Recipes

When you run `emos run <name>`, the CLI adapts its behavior to your installation mode:

**Container mode:**

1. Starts the EMOS Docker container
2. Configures the ROS 2 middleware (Zenoh by default)
3. Verifies sensor topics are publishing
4. Executes the recipe inside the container

**Native mode:**

1. Verifies the ROS2 environment (EMOS packages are installed into `/opt/ros/{distro}/`)
2. Configures the ROS2 middleware
3. Verifies sensor topics are publishing
4. Executes the recipe directly on the host

In native mode, you can also run recipes directly without the CLI: `python3 ~/emos/recipes/<recipe>/recipe.py` (as long as you've sourced `/opt/ros/{distro}/setup.bash`).

All output is streamed to your terminal and saved to `~/emos/logs/`.

## Writing Custom Recipes

A recipe is a directory under `~/emos/recipes/` with the following structure:

```
~/emos/recipes/
  my_recipe/
    recipe.py          # Main entry point (required)
    manifest.json      # Optional Zenoh config
```

### manifest.json

The manifest provides optional configuration for your recipe:

```json
{
  "zenoh_router_config_file": "my_recipe/zenoh_config.json5"
}
```

- {material-regular}`settings;1.2em;sd-text-primary` **zenoh_router_config_file**: Path (relative to `~/emos/recipes/`) to a Zenoh router `.json5` config file. Only needed when using `rmw_zenoh_cpp`.

:::{note}
Sensor requirements are automatically extracted from your `recipe.py` by analyzing `Topic(name=..., msg_type=...)` declarations. You don't need to list them in the manifest. Run `emos info <recipe>` to see what sensors your recipe needs.
:::

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

### Running Your Custom Recipe

Once your recipe directory is in place:

```bash
emos ls              # Verify it appears
emos info my_recipe  # Check sensor requirements
emos run my_recipe   # Launch it
```

## Command Details

### emos install

```bash
emos install                          # Interactive mode selection
emos install --mode container         # Container mode (no ROS required)
emos install --mode native            # Native mode (requires ROS 2)
```

Flags:

- `--mode`: Installation mode (`container` or `native`)
- `--distro`: ROS 2 distribution (`jazzy`, `humble`, or `kilted`)

### emos update

```bash
emos update
```

Detects your installation mode and updates accordingly. Container mode pulls the latest image and recreates the container. Native mode pulls the latest source, rebuilds, and re-installs into `/opt/ros/{distro}/`.

### emos status

```bash
emos status
```

Displays the current installation mode, ROS 2 distro, and status. For container mode, shows whether the container is running. For native mode, shows that EMOS packages are installed in `/opt/ros/{distro}/`.

### emos pull

```bash
emos pull <recipe_short_name>
```

Downloads a recipe from the Automatika recipe server and extracts it to `~/emos/recipes/<name>/`. Overwrites the existing version if present.

### emos info

```bash
emos info <recipe_name_or_path>
```

Inspects a recipe's Python source code to show its sensor and topic requirements. Accepts either a recipe name (looked up in `~/emos/recipes/`) or a direct path to a `.py` file:

```bash
emos info vision_follower          # looks up ~/emos/recipes/vision_follower/recipe.py
emos info ./my_recipe.py           # direct file path
```

The output shows:
- **Required Sensors** — topics with hardware sensor types (Image, LaserScan, etc.) and what hardware they need
- **Suggested packages** — apt packages for common sensor drivers, tailored to your ROS 2 distro
- **Other Topics** — non-sensor topics used by the recipe (e.g. String, Bool)

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

#### What happens during `emos run`

1. Reads `recipe.py` and extracts `Topic(...)` declarations via AST parsing
2. Identifies sensor topics (Image, LaserScan, Odometry, etc.)
3. Starts the Zenoh router (if using `rmw_zenoh_cpp`)
4. Launches `~/emos/robot/launch/bringup_robot.py` if it exists (native/pixi only; skipped in container mode)
5. Verifies each sensor topic is publishing (polls `ros2 topic list` for up to 10 seconds)
6. Executes the recipe — output streams to the terminal and is saved to `~/emos/logs/`

#### Using `--skip-sensor-check`

Skip the sensor verification step when:

- **Sensors publish on-demand** — e.g. a service-based camera that only starts publishing when triggered
- **Testing with rosbag replay** — topic names may differ from what the recipe declares
- **Pure AI recipes** — LLM chat or TTS recipes that don't use sensor hardware

```{warning}
If you skip the check and a sensor topic never arrives, the recipe may hang silently waiting for data. Use `ros2 topic hz /topic_name` to diagnose.
```

```{seealso}
See [Troubleshooting](troubleshooting.md) for common errors during recipe execution.
```

### emos map

Mapping subcommands for creating and editing environment maps:

```bash
emos map record            # Record mapping data on the robot
emos map install-editor    # Install the map editor container
emos map edit <file.tar.gz>  # Process a ROS bag into a PCD map
```
