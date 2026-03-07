# EMOS CLI

The command-line interface for [EMOS](https://github.com/automatika-robotics/emos), the Embodied Operating System. It handles installation, recipe management, and execution across three deployment modes.

## Installation

Download and install the latest release:

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

This detects your architecture (amd64/arm64) and installs the `emos` binary to `/usr/local/bin`.

### Building from Source

Requires Go 1.23+.

```bash
cd stack/emos-cli
make build
sudo make install
```

## Deployment Modes

EMOS supports three installation modes depending on your setup.

### Container Mode

Runs EMOS inside a Docker container using the public GHCR image. No ROS 2 installation required. Sensor drivers must be running externally on the host or in separate containers.

```bash
emos install --mode container --distro jazzy
```

### Native Mode

Installs EMOS packages directly into a local ROS 2 workspace. Requires an existing ROS 2 installation (Humble, Jazzy, or Kilted). Sensor drivers must be running on the host.

```bash
emos install --mode native
```

The installer will detect your ROS 2 installation, clone the EMOS source, fetch dependencies, and build the workspace at `~/emos/ros_ws`.

### Licensed Mode

For robot-specific deployments with a license key. Includes sensor drivers, hardware bringup files, and a private container image.

```bash
emos install YOUR-LICENSE-KEY
```

### Interactive Mode

Run `emos install` without arguments for an interactive menu that guides you through the options.

## Commands

| Command | Description |
| :--- | :--- |
| `emos install` | Install EMOS (interactive, container, native, or licensed) |
| `emos update` | Update EMOS to the latest version |
| `emos status` | Display current installation status |
| `emos recipes` | List available recipes from the server |
| `emos pull <recipe>` | Download and install a recipe |
| `emos run <recipe>` | Execute a recipe |
| `emos map record` | Start recording a new map |
| `emos map process` | Process a recorded map |
| `emos version` | Show CLI version |

### Install Flags

```
--mode      Installation mode: container, native, or licensed
--distro    ROS 2 distribution: jazzy (default), humble, kilted
```

### Run Flags

```
--rmw                  RMW implementation (default: rmw_zenoh_cpp)
--skip-sensor-check    Skip sensor topic verification before launching
```

## Recipe Structure

Recipes live in `~/emos/recipes/<name>/` and contain:

```
recipe_name/
  recipe.py         # Main Python script
  manifest.json     # Sensors, RMW, and configuration metadata
```

The manifest tells the CLI which sensors to verify and how to configure the middleware:

```json
{
  "sensors": ["camera", "lidar"],
  "sensor_topics": {
    "camera": ["/camera/image_raw"],
    "lidar": ["/scan"]
  },
  "rmw": "rmw_zenoh_cpp",
  "web_client": true
}
```

## Configuration

EMOS stores its configuration at `~/.config/emos/config.json`. This file is managed automatically by the CLI and tracks the installation mode, ROS distro, and other settings.

Data directories:

```
~/emos/
  recipes/    # Downloaded recipes
  logs/       # Execution logs
  ros_ws/     # Native mode workspace (native mode only)
```

## Development

```bash
make build       # Build for current platform
make build-all   # Cross-compile for linux/amd64 and linux/arm64
make tidy        # Run go mod tidy
make clean       # Remove build artifacts
```

## License

Copyright 2024-2026 [Automatika Robotics](https://automatikarobotics.com/). See the [LICENSE](../../LICENSE) file for details.
