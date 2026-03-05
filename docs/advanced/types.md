# Supported Types

EMOS components automatically create subscribers and publishers for all inputs and outputs. This page provides a comprehensive reference of all natively supported ROS 2 message types across the full EMOS stack -- the orchestration layer (Sugarcoat), intelligence layer (EmbodiedAgents), and navigation layer (Kompass).

When defining a [Topic](../concepts/topics.md), you pass the message type as a string (e.g., `Topic(name="/image", msg_type="Image")`). The framework handles all serialization, callback creation, and type conversion automatically.

## Standard Messages

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **String** | std_msgs | Standard text message |
| **Bool** | std_msgs | Boolean value |
| **Float32** | std_msgs | Single-precision float |
| **Float32MultiArray** | std_msgs | Array of single-precision floats |
| **Float64** | std_msgs | Double-precision float |
| **Float64MultiArray** | std_msgs | Array of double-precision floats |

## Geometry Messages

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **Point** | geometry_msgs | 3D point (x, y, z) |
| **PointStamped** | geometry_msgs | Timestamped 3D point |
| **Pose** | geometry_msgs | Position + orientation |
| **PoseStamped** | geometry_msgs | Timestamped pose |
| **Twist** | geometry_msgs | Linear + angular velocity |
| **TwistStamped** | geometry_msgs | Timestamped velocity |

## Sensor Messages

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **Image** | sensor_msgs | Raw image data |
| **CompressedImage** | sensor_msgs | Compressed image (JPEG, PNG) |
| **Audio** | sensor_msgs | Audio stream data |
| **LaserScan** | sensor_msgs | 2D lidar scan |
| **PointCloud2** | sensor_msgs | 3D point cloud |
| **CameraInfo** | sensor_msgs | Camera calibration and metadata |
| **JointState** | sensor_msgs | Instantaneous joint position, velocity, and effort |

## Navigation Messages

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **Odometry** | nav_msgs | Robot position and velocity |
| **Path** | nav_msgs | Array of poses for navigation |
| **MapMetaData** | nav_msgs | Map resolution, size, origin |
| **OccupancyGrid** | nav_msgs | 2D grid map with occupancy probabilities |

## Intelligence Messages

These types are defined by EmbodiedAgents for AI component communication.

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **StreamingString** | automatika_embodied_agents | String chunk for streaming applications (e.g., LLM tokens) |
| **Video** | automatika_embodied_agents | A sequence of image frames |
| **Detections** | automatika_embodied_agents | 2D bounding boxes with labels and confidence scores |
| **DetectionsMultiSource** | automatika_embodied_agents | Detections from multiple input sources |
| **PointsOfInterest** | automatika_embodied_agents | Specific 2D coordinates of interest within an image |
| **Trackings** | automatika_embodied_agents | Object tracking data including IDs, labels, and trajectories |
| **TrackingsMultiSource** | automatika_embodied_agents | Object tracking data from multiple sources |

## Navigation-Specific Messages

These types are defined by Kompass for navigation component communication.

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **TwistArray** | kompass_interfaces | Array of velocity commands for trajectory candidates |

## Hardware Interface Messages

| Message | ROS 2 Package | Description |
|:---|:---|:---|
| **RGBD** | realsense2_camera_msgs | Synchronized RGB and Depth image pair |
| **JointTrajectoryPoint** | trajectory_msgs | Position, velocity, and acceleration for joints at a specific time |
| **JointTrajectory** | trajectory_msgs | A sequence of waypoints for joint control |
| **JointJog** | control_msgs | Immediate displacement or velocity commands for joints |
