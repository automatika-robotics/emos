# Motion Server

**System validation, calibration, and motion data recording.**

Unlike the core navigation components, the Motion Server does not plan paths or avoid obstacles. Instead, it provides essential utilities for validating your robot's physical performance and tuning its control parameters.

It serves two primary purposes:

1. **Automated Motion Tests:** Executing pre-defined maneuvers (step response, circles) to calibrate the robot's motion model on new terrain.
2. **Black Box Recording:** Capturing synchronized control commands and robot responses (Pose/Velocity) during operation for post-analysis.

## Key Capabilities

- **Motion Calibration** — Execute step inputs or circular paths automatically to measure the robot's real-world response vs. the theoretical model.
- **Data Recording** — Record exact control inputs and odometry outputs synchronized in time. Essential for tuning controller gains or debugging tracking errors.
- **Closed-Loop Validation** — Can act as both the source of commands (during tests) and the sink for recording, allowing you to validate the entire control pipeline.
- **Event-Triggered** — Start recording or launch a calibration sequence automatically based on external events (e.g., "Terrain Changed" or "Slip Detected").

## Run Types

| Mode | Description |
| :--- | :--- |
| **Timed** | Automatically launches configured motion tests periodically after the component starts. |
| **Event** | Waits for a `True` signal on the `run_tests` input topic to launch the calibration sequence. |
| **Action Server** | On-demand recording. Offers a `MotionRecording` ROS2 Action to start/stop recording for a set duration. |

## Interface

### Inputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **run_tests** | `Bool` | `/run_tests` |
| **command** | `Twist`, `TwistStamped` | `/cmd_vel` |
| **location** | `Odometry`, `PoseStamped`, `Pose` | `/odom` |

### Outputs

| Key Name | Allowed Types | Default |
| :--- | :--- | :--- |
| **robot_command** | `Twist`, `TwistStamped` | `/cmd_vel` |

:::{note}
The **command** topic appears in both Inputs and Outputs but serves different roles:
- **Output (`robot_command`):** Used when the Motion Server is *generating* commands (running tests).
- **Input (`command`):** Used when the Motion Server is *listening* (recording).

You can wire these differently to test specific components. For example, connect the Motion Server output to the Drive Manager's input, and the Drive Manager's output back to the Motion Server input to record exactly how the Drive Manager modifies your commands.
:::

## Usage Example

```python
from kompass.components import MotionServer, MotionServerConfig
from kompass.ros import Topic

# 1. Configuration
my_config = MotionServerConfig(
    step_test_velocity=1.0,
    step_test_duration=5.0
)

# 2. Instantiate
motion_server = MotionServer(component_name="motion_server", config=my_config)

# 3. Setup for Event-Based Testing
motion_server.run_type = "Event"
motion_server.inputs(run_tests=Topic(name="/start_calibration", msg_type="Bool"))
```
