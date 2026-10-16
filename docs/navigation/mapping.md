# Mapping & Localization

This page covers how a robot gets its global map and its position on it, the **Map Server** that serves that map to the planner, and the **Local Mapper** that tracks the robot's immediate surroundings in real time.

## Global Mapping & Localization

The global map itself is built with `emos map`: EMOS drives the robot's own mapping software when it has one, and otherwise builds the map itself from the robot's LiDAR with [GLIM](https://koide3.github.io/glim/), a GPU-accelerated 3D LiDAR-inertial SLAM. The active map's occupancy grid is what the Map Server serves. See [Mapping](../getting-started/mapping.md) for the workflow, including bringing in a map made with other equipment.

Localization, keeping track of where the robot is on that map, comes from the robot plugin. Robots with their own localization software expose its pose estimate as a feed. For the others, the plugin computes one, typically by fusing the robot's odometry with its IMU, and the plugin takes care of everything that involves: it declares the packages it needs, starts the fusion node when a recipe asks for the feed, and publishes the `map` to `odom` to base transforms. A recipe binds that feed as its location input and has nothing to install or launch:

```python
launcher.inputs(
    location=Topic(name="odometry_filtered", msg_type="Odometry", use_plugin=True)
)
```

The feed's name is the plugin's, and `emos plugin inspect` lists it with the rest of the plugin's interface.

:::{tip}
The [Map Server](#map-server) reads 3D point clouds (`.pcd`) directly, so a map from GLIM or a survey scanner can be used as it is, without converting it to a 2D grid first.
:::

## Map Server

**Static global map management and 3D-to-2D projection.**

The Map Server is the source of ground-truth for the navigation system. It reads static map files, processes them, and publishes the global `OccupancyGrid` required by the Planner and Localization components.

Unlike standard ROS2 map servers, the EMOS Map Server supports **native 3D Point Cloud (PCD)** files, automatically slicing and projecting them into 2D navigable grids based on configurable height limits.

### Key Features

- {material-regular}`swap_horiz;1.2em;sd-text-primary` **Map Data Conversion** — Reads map files in either 2D (YAML) or 3D (PCD) format and converts the data into usable global map formats (OccupancyGrid).

- {material-regular}`public;1.2em;sd-text-primary` **Global Map Serving** — Once map data is loaded and processed, the MapServer publishes the global map as an `OccupancyGrid` message. It is published once per loaded map with latched QoS (reliable, transient local), so any subscriber that joins later still receives it; subscribers need a matching QoS.

- {material-regular}`view_in_ar;1.2em;sd-text-primary` **Point Cloud to Grid Conversion** — If the map data is provided as a PCD file, the MapServer generates an occupancy grid from the point cloud using the provided grid resolution and ground limits.

- {material-regular}`crop_free;1.2em;sd-text-primary` **Custom Frame Handling** — Configurable reference frames ensuring the map aligns with your robot's TF tree.

- {material-regular}`save;1.2em;sd-text-primary` **Map Saving** — Supports saving both 2D and 3D maps to files via `Save2dMapToFile` and `Save3dMapToFile` services.

- {material-regular}`update;1.2em;sd-text-primary` **Map Update Frequency Control** — Control how often map data is read and converted via the `map_file_read_rate` parameter.

### Outputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - global_map
  - [`nav_msgs.msg.OccupancyGrid`](http://docs.ros.org/en/noetic/api/nav_msgs/html/msg/OccupancyGrid.html)
  - 1
  - `Topic(name="/map", msg_type="OccupancyGrid")`

* - spatial_sensor
  - [`sensor_msgs.msg.PointCloud2`](http://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/PointCloud2.html)
  - 1, optional
  - `Topic(name="/row_point_cloud", msg_type="PointCloud2")`
```

### Usage Example

```python
from kompass.components import MapServer, MapServerConfig
from kompass.ros import Topic

my_config = MapServerConfig(
    map_file_path="/path/to/environment.pcd",
    map_file_read_rate=5.0,
    grid_resolution=0.1,
    pc_publish_row=False
)

my_map_server = MapServer(component_name="map_server", config=my_config)
```

## Local Mapper

**Real-time, ego-centric occupancy grid generation.**

While the global map provides a static long-term view, the Local Mapper builds a dynamic, short-term map of the robot's immediate surroundings based on real-time sensor data. It captures moving obstacles (people, other robots) and temporary changes, serving as the primary input for the [Controller](control.md) to enable fast reactive navigation.

At its core, the Local Mapper uses the Bresenham line drawing algorithm in C++ to efficiently update an occupancy grid from incoming sensor data, a single LaserScan or point clouds from up to eleven sensors. Several point clouds, say a front and a rear 3D LiDAR, are fused into one grid: each sensor's mount transform is read from TF, and free space is carved from each sensor's own origin. This approach ensures fast and accurate raycasting to determine free and occupied cells in the local grid.

To maximize performance and adaptability, the implementation **supports both CPU and GPU execution**:

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`memory;1.5em;sd-text-primary` SYCL GPU Acceleration</span> — Vendor-agnostic GPU acceleration compatible with Nvidia, AMD, Intel, Arm Mali and any other GPGPU-capable devices.

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`developer_board;1.5em;sd-text-primary` Multi-Threaded CPU</span> — Falls back to a highly optimized multi-threaded CPU implementation if no GPU is available.


### Inputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - sensor_data
  - [`sensor_msgs.msg.LaserScan`](https://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/LaserScan.html), [`sensor_msgs.msg.PointCloud2`](https://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/PointCloud2.html)
  - 1 LaserScan, or 1 to 11 PointCloud2
  - `Topic(name="/scan", msg_type="LaserScan")`

* - location
  - [`nav_msgs.msg.Odometry`](https://docs.ros.org/en/noetic/api/nav_msgs/html/msg/Odometry.html), [`geometry_msgs.msg.PoseStamped`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/PoseStamped.html), [`geometry_msgs.msg.Pose`](http://docs.ros.org/en/jade/api/geometry_msgs/html/msg/Pose.html)
  - 1
  - `Topic(name="/odom", msg_type="Odometry")`
```

### Outputs

```{list-table}
:widths: 10 40 10 40
:header-rows: 1

* - Key Name
  - Allowed Types
  - Number
  - Default

* - local_map
  - `nav_msgs.msg.OccupancyGrid`
  - 1
  - `Topic(name="/local_map/occupancy_layer", msg_type="OccupancyGrid")`
```

A point cloud older than `sensor_data_timeout` (0.2 s by default) is skipped for that update, so a sensor that stalls does not freeze the map. The height band of the scan model, `min_height` and `max_height`, applies in the robot's body frame for point clouds.

### Usage Example

```python
from kompass.components import LocalMapper, LocalMapperConfig
from kompass.control import MapConfig
from kompass.ros import Topic

# Select map parameters: 5m x 5m rolling window with 20cm resolution
map_params = MapConfig(width=5.0, height=5.0, resolution=0.2)

# Setup custom component configuration
my_config = LocalMapperConfig(loop_rate=10.0, map_params=map_params)

# Init a mapper
my_mapper = LocalMapper(component_name="mapper", config=my_config)

# Fuse two 3D LiDARs instead of the default LaserScan
my_mapper.inputs(
    sensor_data=[
        Topic(name="/lidar_front/points", msg_type="PointCloud2"),
        Topic(name="/lidar_rear/points", msg_type="PointCloud2"),
    ]
)
```
