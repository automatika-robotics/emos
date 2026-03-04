# Supported Types

EMOS components automatically create subscribers and callbacks for all inputs, and publishers for all outputs. This page provides a comprehensive reference of all natively supported ROS 2 message types across the EMOS stack.

```{tip}
Access all callbacks in a `BaseComponent` via `self.callbacks: Dict[str, GenericCallback]` and get the topic incoming message using the `get_output` method in the `GenericCallback` class.
```

```{tip}
Access all publishers in a `BaseComponent` via `self.publishers_dict: Dict[str, Publisher]` and publish a new message to the topic using the `publish` method in the `Publisher` class.
```

Many supported message types come with pre-defined callback and publisher classes that convert ROS 2 messages to and from Python types. Below is the complete list of supported messages, the types returned by their callback `get_output` method, and the types accepted by their publisher `publish` method.

## Supported ROS 2 Messages

```{list-table}
:widths: 15 20 20 20
:header-rows: 1
* - Message
  - ROS 2 Package
  - Callback Return Type
  - Publisher Converts From

* - **String**
  - std_msgs
  - `str`
  - `str`

* - **Bool**
  - std_msgs
  - `bool`
  - `bool`

* - **Float32**
  - std_msgs
  - `float`
  - `float`

* - **Float32MultiArray**
  - std_msgs
  - `numpy.ndarray[float]`
  - `numpy.ndarray[float]`

* - **Float64**
  - std_msgs
  - `float`
  - `float`

* - **Float64MultiArray**
  - std_msgs
  - `numpy.ndarray[float]`
  - `numpy.ndarray[float]`

* - **Point**
  - geometry_msgs
  - `numpy.ndarray`
  - `numpy.ndarray`

* - **PointStamped**
  - geometry_msgs
  - `numpy.ndarray`
  - `numpy.ndarray`

* - **Pose**
  - geometry_msgs
  - `numpy.ndarray`
  - `numpy.ndarray`

* - **PoseStamped**
  - geometry_msgs
  - `numpy.ndarray`
  - `numpy.ndarray`

* - **Twist**
  - geometry_msgs
  - `geometry_msgs.msg.Twist`
  - `geometry_msgs.msg.Twist`

* - **TwistStamped**
  - geometry_msgs
  - --
  - --

* - **TwistArray**
  - kompass_interfaces
  - --
  - --

* - **Image**
  - sensor_msgs
  - `numpy.ndarray`
  - `sensor_msgs.msg.Image | numpy.ndarray`

* - **CompressedImage**
  - sensor_msgs
  - `numpy.ndarray`
  - `sensor_msgs.msg.CompressedImage | numpy.ndarray`

* - **Audio**
  - sensor_msgs
  - `bytes`
  - `str | bytes`

* - **LaserScan**
  - sensor_msgs
  - `sensor_msgs.msg.LaserScan`
  - `sensor_msgs.msg.LaserScan`

* - **PointCloud2**
  - sensor_msgs
  - --
  - --

* - **Odometry**
  - nav_msgs
  - `numpy.ndarray`
  - `nav_msgs.msg.Odometry`

* - **Path**
  - nav_msgs
  - `nav_msgs.msg.Path`
  - `nav_msgs.msg.Path`

* - **MapMetaData**
  - nav_msgs
  - `Dict`
  - `nav_msgs.msg.MapMetaData`

* - **OccupancyGrid**
  - nav_msgs
  - `nav_msgs.msg.OccupancyGrid | np.ndarray | Dict`
  - `numpy.ndarray`

* - **ComponentStatus**
  - ros_sugar_interfaces
  - `ros_sugar_interfaces.msg.ComponentStatus`
  - `ros_sugar_interfaces.msg.ComponentStatus`

* - **Detections**
  - automatika_agents_interfaces
  - --
  - --

* - **Trackings**
  - automatika_agents_interfaces
  - --
  - --

```

```{note}
Types marked with `--` for callback/publisher columns are supported for subscription and publishing but use direct ROS 2 message types without automatic conversion. Refer to the API documentation for details on specific type handling.
```
