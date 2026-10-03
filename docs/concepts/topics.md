# Topics

A `Topic` is how a recipe names a stream of data. It carries the ROS topic's name and message type, the quality of service it is read or written with, how long a message stays fresh, and, when the data comes from the robot's plugin rather than from a ROS publisher, which plugin serves it. The same object does two jobs: it declares a component's inputs and outputs, and it is what [event conditions](events-and-actions.md) and action arguments are written against.

```python
from ros_sugar.io import Topic

scan = Topic(name="/scan", msg_type="LaserScan")
cmd = Topic(name="/cmd_vel", msg_type="Twist")

controller = Controller(component_name="controller", inputs=[scan], outputs=[cmd])
```

When the component starts it creates the subscriber, the publisher and the type conversion for each, so the component's code works with images as arrays and scans as scan objects rather than raw messages.

---

## Message types

`msg_type` names one of the types EMOS knows how to convert. It is given as a string, so no message class has to be imported into the recipe, or as the type itself from `ros_sugar.io.supported_types`:

```python
image = Topic(name="/camera/rgb", msg_type="Image")
odom = Topic(name="/odom", msg_type="Odometry")
```

The base set covers the common ROS message families: strings, booleans and numbers, images and compressed images, audio, laser scans, point clouds, occupancy grids, odometry, IMU, GNSS, range, joint states, camera info, poses, points and paths, and twists. Kompass and EmbodiedAgents register more of their own, such as detections, and a plugin can wrap a robot's custom message with `create_supported_type`. A raw ROS message class is not accepted: a type that EMOS cannot convert cannot be an input or an output. The full list, and how to add a type, is in [Types](../advanced/types.md).

---

## Quality of service

QoS is set with a `QoSConfig`, in the recipe, with the same policies ROS uses:

```python
from rclpy import qos
from ros_sugar.config import QoSConfig
from ros_sugar.io import Topic

latched = QoSConfig(
    history=qos.HistoryPolicy.KEEP_LAST,
    queue_size=20,
    reliability=qos.ReliabilityPolicy.BEST_EFFORT,
    durability=qos.DurabilityPolicy.TRANSIENT_LOCAL,
)

local_map = Topic(name="/local_map", msg_type="OccupancyGrid", qos_profile=latched)
```

The defaults are keep-last with a queue of ten, reliable delivery, and durability left to the middleware, which means volatile. A sensor stream that must not hold up its publisher is the usual reason to choose best effort, and a map that late subscribers should still receive is the usual reason to choose transient local.

---

## Freshness

`data_timeout`, in seconds, says how long a message counts as current. It matters most in events: a condition that combines a proximity reading with the robot's mode should not fire on a reading that stopped arriving a minute ago. The default is one second.

```python
proximity = Topic(name="/radar_front", msg_type="Float32", data_timeout=0.2)
```

---

## Topics served by a plugin

When the data comes from the robot's own plugin rather than from a ROS publisher, `use_plugin` says so. `True` means the robot plugin; a string names a sensor plugin by its id. The topic's name is then the plugin's feed key, or its standard type when the plugin offers only one feed of that type:

```python
odom = Topic(name="Odometry", msg_type="Odometry", use_plugin=True)          # by type
front = Topic(name="ultrasound_front", msg_type="Range", use_plugin=True)   # by key
thermal = Topic(name="thermal_image", msg_type="Image", use_plugin=camera.id)
```

At bringup the launcher matches each such topic against the attached plugins and fails early if a name is unknown. A feed that is already a ROS topic just gets the subscriber pointed at it; anything else is bridged through the plugin's bus, with large images and clouds passed through shared memory, and the component never sees the difference. Topics without `use_plugin` are left alone, so a recipe can mix plugin feeds with plain ROS topics freely. [Robot Plugins](robot-plugins.md) explains what a plugin offers.

```{seealso}
- [Components](components.md) for reading inputs and publishing outputs inside a component.
- [Events and Actions](events-and-actions.md) for conditions written against a topic's message.
- [Types](../advanced/types.md) for every supported message type and how to add one.
```
