# Configuration

EMOS is built for flexibility -- and that starts with how you configure your components.

Whether you are scripting in Python, editing clean and readable YAML, crafting elegant TOML files, or piping in JSON from a toolchain, EMOS lets you do it your way. No rigid formats or boilerplate structures. Just straightforward, expressive configuration -- however you like to write it.

## Configuration Formats

EMOS supports four configuration methods:

- [Python API](#python-api)
- [YAML](#yaml)
- [TOML](#toml)
- [JSON](#json)

Pick your format. Plug it in. Go.

## Python API

Use the full power of the Pythonic API to configure your components when you need dynamic logic, computation, or tighter control.

```python
from kompass.components import Planner, PlannerConfig
from kompass.ros import Topic
from kompass.robot import (
    AngularCtrlLimits,
    LinearCtrlLimits,
    RobotGeometry,
    RobotType,
    RobotConfig,
    RobotFrames
)
import numpy as np
import math

# Define your robot's physical and control characteristics
my_robot = RobotConfig(
    model_type=RobotType.DIFFERENTIAL_DRIVE,            # Type of robot motion model
    geometry_type=RobotGeometry.Type.CYLINDER,          # Shape of the robot
    geometry_params=np.array([0.1, 0.3]),                # Diameter and height of the cylinder
    ctrl_vx_limits=LinearCtrlLimits(                     # Linear velocity constraints
        max_vel=0.4,
        max_acc=1.5,
        max_decel=2.5
    ),
    ctrl_omega_limits=AngularCtrlLimits(                 # Angular velocity constraints
        max_vel=0.4,
        max_acc=2.0,
        max_decel=2.0,
        max_steer=math.pi / 3                            # Steering angle limit (radians)
    ),
)

# Define the robot's coordinates frames
my_frames = RobotFrames(
    world="map",
    odom="odom",
    robot_base="body",
    scan="lidar_link"
)

# Create the planner config using your robot setup
config = PlannerConfig(
    robot=my_robot,
    loop_rate=1.0
)

# Instantiate the Planner component
planner = Planner(
    component_name="planner",
    config=config
)

# Additionally configure the component's inputs or outputs
planner.inputs(
    map_layer=Topic(name="/map", msg_type="OccupancyGrid"),
    goal_point=Topic(name="/clicked_point", msg_type="PointStamped")
)
```

## YAML

Similar to traditional ROS 2 launch, you can maintain all your configuration parameters in a YAML file. EMOS simplifies the standard ROS 2 YAML format -- just drop the `ros__parameters` noise:

```yaml
/**: # Common parameters for all components
  frames:
    robot_base: "body"
    odom: "odom"
    world: "map"
    scan: "lidar_link"

  robot:
    model_type: "DIFFERENTIAL_DRIVE"
    geometry_type: "CYLINDER"
    geometry_params: [0.1, 0.3]

    ctrl_vx_limits:
      max_vel: 0.4
      max_acc: 1.5
      max_decel: 2.5

    ctrl_omega_limits:
      max_vel: 0.4
      max_acc: 2.0
      max_decel: 2.0
      max_steer: 1.0472  # ~ pi / 3

planner:
  inputs:
    map_layer:
      name: "/map"
      msg_type: "OccupancyGrid"
    goal_point:
      name: "/clicked_point"
      msg_type: "PointStamped"
  loop_rate: 1.0
```

Common parameters placed under the `/**` key are shared across all components. Component-specific parameters are placed under the component name.

## TOML

Not a fan of YAML? EMOS lets you configure your components using TOML too. TOML offers clear structure and excellent tooling support, making it ideal for clean, maintainable configs.

```toml
["/**".frames]
robot_base = "body"
odom = "odom"
world = "map"
scan = "lidar_link"

["/**".robot]
model_type = "DIFFERENTIAL_DRIVE"
geometry_type = "CYLINDER"
geometry_params = [0.1, 0.3]

["/**".robot.ctrl_vx_limits]
max_vel = 0.4
max_acc = 1.5
max_decel = 2.5

["/**".robot.ctrl_omega_limits]
max_vel = 0.4
max_acc = 2.0
max_decel = 2.0
max_steer = 1.0472  # ~ pi / 3

[planner]
loop_rate = 1.0

[planner.inputs.map_layer]
name = "/map"
msg_type = "OccupancyGrid"

[planner.inputs.goal_point]
name = "/clicked_point"
msg_type = "PointStamped"
```

## JSON

Prefer curly braces? Or looking to pipe configs from an ML model or external toolchain? JSON is machine-friendly and widely supported -- perfect for automating your EMOS configuration with generated files.

```json
{
  "/**": {
    "frames": {
      "robot_base": "body",
      "odom": "odom",
      "world": "map",
      "scan": "lidar_link"
    },
    "robot": {
      "model_type": "DIFFERENTIAL_DRIVE",
      "geometry_type": "CYLINDER",
      "geometry_params": [0.1, 0.3],
      "ctrl_vx_limits": {
        "max_vel": 0.4,
        "max_acc": 1.5,
        "max_decel": 2.5
      },
      "ctrl_omega_limits": {
        "max_vel": 0.4,
        "max_acc": 2.0,
        "max_decel": 2.0,
        "max_steer": 1.0472
      }
    }
  },
  "planner": {
    "loop_rate": 1.0,
    "inputs": {
      "map_layer": {
        "name": "/map",
        "msg_type": "OccupancyGrid"
      },
      "goal_point": {
        "name": "/clicked_point",
        "msg_type": "PointStamped"
      }
    }
  }
}
```

## Minimal Configuration Examples

For simple components that do not require full robot configuration, the config files are even more concise:

::::{tab-set}

:::{tab-item} YAML

```yaml
/**:
  common_int_param: 0

my_component_name:
  float_param: 1.5
  boolean_param: true
```
:::

:::{tab-item} TOML

```toml
["/**"]
common_int_param = 0

[my_component_name]
float_param = 1.5
boolean_param = true
```
:::

:::{tab-item} JSON

```json
{
    "/**": {
        "common_int_param": 0
    },
    "my_component_name": {
        "float_param": 1.5,
        "boolean_param": true
    }
}
```
:::

::::

```{note}
Make sure to pass your config file to the component on initialization or to the Launcher.
```

:::{seealso}
You can check complete examples of detailed configuration files in the [EMOS navigation params](https://github.com/automatika-robotics/kompass/tree/main/kompass/params).
:::
