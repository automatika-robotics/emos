# Motion Server

**System validation, calibration, and motion data recording.**

Unlike the core navigation components, the Motion Server does not plan paths or avoid obstacles. Instead, it provides essential utilities for validating your robot's physical performance and tuning its control parameters.

It serves two primary purposes:

1. **Automated Motion Tests:** Executing pre-defined maneuvers (step response, circles) to calibrate the robot's motion model on new terrain.
2. **Black Box Recording:** Capturing synchronized control commands and robot responses (Pose/Velocity) during operation for post-analysis.

## Key Capabilities

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`tune;1.5em;sd-text-primary` Motion Calibration</span> — Execute step inputs or circular paths automatically to measure the robot's real-world response vs. the theoretical model.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`fiber_manual_record;1.5em;sd-text-primary` Data Recording</span> — Record exact control inputs and odometry outputs synchronized in time. Essential for tuning controller gains or debugging tracking errors.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`loop;1.5em;sd-text-primary` Closed-Loop Validation</span> — Can act as both the source of commands (during tests) and the sink for recording, allowing you to validate the entire control pipeline.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`flash_on;1.5em;sd-text-primary` Event-Triggered</span> — Start recording or launch a calibration sequence automatically based on external events (e.g., "Terrain Changed" or "Slip Detected").

```{note}
The available motion tests include Step tests and Circle test and can be configured by adjusting the MotionServerConfig.
```

## Available Run Types

```{list-table}
:widths: 10 80

* - **Timed**
  - Launches automated tests periodically after the component starts.

* - **Event**
  - Launches automated testing when a trigger is received on the `run_tests` input topic.

* - **ActionServer**
  - Offers a `MotionRecording` ROS2 Action to start/stop recording for a set duration.
```

```{tip}
Launch the MotionServer as a **Timed** component to launch the basic motion tests automatically, or as an **Event** component to launch the tests when a trigger is received.
```

```{tip}
Launch the MotionServer as an **ActionServer** component and send a request to record your robot's motion at any time during the navigation.
```

## Inputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - run_tests
  - `std_msgs.msg.Bool`
  - 1
  - `Topic(name="/run_tests", msg_type="Bool")`

* - command
  - [`geometry_msgs.msg.Twist`](http://docs.ros.org/en/noetic/api/geometry_msgs/html/msg/Twist.html)
  - 1
  - `Topic(name="/cmd_vel", msg_type="Twist")`

* - location
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.Pose`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/Pose.html)
  - 1
  - `Topic(name="/odom", msg_type="Odometry")`
```

## Outputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - robot_command
  - [`geometry_msgs.msg.Twist`](http://docs.ros.org/en/noetic/api/geometry_msgs/html/msg/Twist.html)
  - 1
  - `Topic(name="/cmd_vel", msg_type="Twist")`
```

```{note}
Topic for *Control Command* is both in MotionServer inputs and outputs:
- The output is used when running automated testing (i.e. sending the commands directly from the MotionServer).
- The input is used to purely record motion and control from external sources (example: recording output from Controller).
- Different command topics can be configured for the input and the output. For example: to test the DriveManager, the control command from MotionServer output can be sent to the DriveManager, then the DriveManager output can be configured as the MotionServer input for recording.
```

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
