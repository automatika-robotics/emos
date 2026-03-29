# Troubleshooting

Common issues when running EMOS recipes, organized by error message.

---

## "Sensor topic not found within 10s"

The most common error. `emos run` checks that every sensor topic declared in the recipe is actually publishing before launching.

**Possible causes:**

| Cause | Fix |
| :--- | :--- |
| Driver not installed | Run `emos info <recipe>` for suggested packages, then `sudo apt install <package>` |
| Driver installed but not running | Start the driver node in a separate terminal before running the recipe |
| Topic name mismatch | Compare `ros2 topic list` output with `Topic(name=...)` in your recipe and correct the name |
| **Container mode**: driver not installed in container | Install the driver inside the container: `docker exec -it emos bash -c "apt-get update && apt-get install -y ros-jazzy-usb-cam"` and launch it |

**Diagnosis:**

```bash
# Check what topics exist
ros2 topic list

# Check if a specific topic has data
ros2 topic hz /image_raw
```

If the topic exists but shows 0 Hz, the driver is running but not producing data (check hardware connections).

To temporarily bypass this check while debugging, use `emos run <recipe> --skip-sensor-check`.

---

## "ImportError: No module named 'agents'"

EMOS Python packages are not on the Python path. The fix depends on your install mode:

| Mode | Fix |
| :--- | :--- |
| **Container** | Run the recipe via `emos run`, not directly with `python3`. The CLI executes inside the container where packages are installed. |
| **Native** | Source the ROS 2 environment first: `source /opt/ros/jazzy/setup.bash` |
| **Pixi** | Enter the pixi shell first: `pixi shell`, then `source install/setup.sh` |

---

## "Robot plugin not found"

A recipe uses `Launcher(robot_plugin="some_plugin")` but the plugin package is not installed.

**Fix:**

1. Check if the plugin is available: `ros2 pkg list | grep some_plugin`
2. If missing, build and install the plugin package into your ROS 2 workspace:
   ```bash
   cd ~/ros2_ws/src
   git clone <plugin_repo_url>
   cd ~/ros2_ws && colcon build
   source install/setup.bash
   ```

```{seealso}
See [Robot Plugins](../concepts/robot-plugins.md) for how plugins work and how to create one.
```

---

## "Zenoh router failed to start"

The Zenoh router (used by `rmw_zenoh_cpp`) typically fails when port 7447 is already in use from a previous run.

**Fix:**

```bash
# Kill any existing Zenoh routers
pkill -f rmw_zenohd

# Then retry
emos run <recipe>
```

To use a different RMW and skip Zenoh entirely:

```bash
emos run <recipe> --rmw rmw_cyclonedds_cpp
```

---

## "Recipe hangs with no output"

The recipe started but nothing happens. Two common causes:

**1. Sensor topic exists but has no data (0 Hz)**

The driver node is running but the hardware isn't producing data. Check:

```bash
ros2 topic hz /image_raw    # Should show non-zero rate
```

If 0 Hz: check physical connections (USB cable, power), device permissions (`sudo chmod 666 /dev/video0`), or driver configuration.

**2. Model inference failing**

EMOS verifies that the model server is reachable when the recipe starts. If the recipe hangs after that, the server is running but inference itself is failing — for example, the model isn't loaded, is out of memory, or the request format is wrong.

Check your model server's logs for errors. Common causes:

- **Model not pulled/loaded** — e.g. for Ollama, run `ollama pull qwen2.5vl:latest` before starting the recipe
- **Out of memory** — the model is too large for available RAM/VRAM. Try a smaller checkpoint.
- **Timeout on cloud endpoint** — network latency or rate limiting. Check the provider's status page.

---

## "Container 'emos' not found"

The Docker container was removed or never created.

**Fix:**

```bash
emos install --mode container
```

This recreates the container. Your recipes in `~/emos/recipes/` are preserved (they're mounted from the host).

---

## "No EMOS installation found"

The config file at `~/.config/emos/config.json` is missing.

**Fix:** Run `emos install` to set up EMOS. If you're using the pixi install mode, run `pixi run setup` from the EMOS repo root to register the installation.
