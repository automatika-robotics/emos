# Topics

**The connective tissue of your system.**

Topics are defined in EMOS with a `Topic` class that specifies the **Data Contract** (Type/Name of the ROS2 topic), the **Behavior** (QoS), and the **Freshness Constraints** (Timeout) for a specific stream of information.

Topics act as the bridge for both:

1.  **Component I/O:** They define what data a Component produces or consumes.
2.  **Event Triggers:** They act as the "Sensors" for the Event-Driven system, feeding data into the Blackboard.

## Why Use EMOS Topics?

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`link;1.5em;sd-text-primary` Declarative Wiring</span> - No more hardcoded strings buried in your components. Define your Topics as shared resources and pass them into Components during configuration.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`timer;1.5em;sd-text-primary` Freshness Monitoring</span> - An EMOS Topic can enforce a `data_timeout`. If the data is too old, the Event system knows to ignore it, preventing "Stale Data" bugs.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`auto_awesome;1.5em;sd-text-primary` Lazy Type Resolution</span> - You don't need to import message classes at the top of every file. EMOS resolves types like `'OccupancyGrid'` or `'Odometry'` at runtime, keeping your code clean and decoupling dependencies.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`tune;1.5em;sd-text-primary` QoS Abstraction</span> - Quality of Service profiles are configured via simple Python objects directly in your recipe.

## Usage Example

```python
from ros_sugar.config import QoSConfig
from ros_sugar.io import Topic

qos_conf = QoSConfig(
    history=qos.HistoryPolicy.KEEP_LAST,
    queue_size=20,
    reliability=qos.ReliabilityPolicy.BEST_EFFORT,
    durability=qos.DurabilityPolicy.TRANSIENT_LOCAL
)

topic = Topic(name='/local_map', msg_type='OccupancyGrid', qos_profile=qos_conf)
```

## Advanced: Smart Type Resolution

One of EMOS's most convenient features is **String-Based Type Resolution**. In standard ROS2, you must import the specific message class (`from geometry_msgs.msg import Twist`) to create a publisher or subscriber. EMOS handles this import for you dynamically.

```python
from ros_sugar.io import Topic
from std_msgs.msg import String

# Method 1: The Standard Way (Explicit Class)
# Requires 'from std_msgs.msg import String'
topic_1 = Topic(name='/chatter', msg_type=String)

# Method 2: The EMOS Way (String Literal)
# No import required. EMOS finds 'std_msgs/msg/String' automatically.
topic_2 = Topic(name='/chatter', msg_type='String')
```

:::{seealso}
See the full list of automatically supported message types in the advanced types reference.
:::

## Component Integration

Once defined, Topics are passed to [Components](./components.md) to automatically generate the ROS2 infrastructure.

```python
from ros_sugar.core import BaseComponent
from ros_sugar.io import Topic

# When this component starts, it automatically creates:
# - A Subscriber to '/scan' (LaserScan)
# - A Publisher to '/cmd_vel' (Twist)
my_node = BaseComponent(
    component_name="safety_controller",
    inputs=[Topic(name="/scan", msg_type="LaserScan")],
    outputs=[Topic(name="/cmd_vel", msg_type="Twist")]
)
```
