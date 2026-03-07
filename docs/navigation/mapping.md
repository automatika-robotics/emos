# Mapping & Localization

This page covers the mapping components in EMOS: the **Local Mapper** for real-time obstacle detection and the **Map Server** for static global maps, along with recommended community packages for localization.

## Local Mapper

**Real-time, ego-centric occupancy grid generation.**

While the global map provides a static long-term view, the Local Mapper builds a dynamic, short-term map of the robot's immediate surroundings based on real-time sensor data. It captures moving obstacles (people, other robots) and temporary changes, serving as the primary input for the [Controller](control.md) to enable fast reactive navigation.

At its core, the Local Mapper uses the Bresenham line drawing algorithm in C++ to efficiently update an occupancy grid from incoming LaserScan data. This approach ensures fast and accurate raycasting to determine free and occupied cells in the local grid.

To maximize performance and adaptability, the implementation **supports both CPU and GPU execution**:

- <span class="sd-text-primary" style="font-weight: bold; font-size: 1.1em;">{material-regular}`memory;1.5em;sd-text-primary` SYCL GPU Acceleration</span> — Vendor-agnostic GPU acceleration compatible with Nvidia, AMD, Intel, and any other GPGPU-capable devices.

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
  - [`sensor_msgs.msg.LaserScan`](https://docs.ros.org/en/noetic/api/sensor_msgs/html/msg/LaserScan.html)
  - 1
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

```{note}
Current implementation supports LaserScan sensor data to create an Occupancy Grid local map. PointCloud and semantic information will be supported in an upcoming release.
```

### Usage Example

```python
from kompass_core.mapping import LocalMapperConfig
from kompass.components import LocalMapper, MapperConfig

# Select map parameters: 5m x 5m rolling window with 20cm resolution
map_params = MapperConfig(width=5.0, height=5.0, resolution=0.2)

# Setup custom component configuration
my_config = LocalMapperConfig(loop_rate=10.0, map_params=map_params)

# Init a mapper
my_mapper = LocalMapper(component_name="mapper", config=my_config)
```

## Map Server

**Static global map management and 3D-to-2D projection.**

The Map Server is the source of ground-truth for the navigation system. It reads static map files, processes them, and publishes the global `OccupancyGrid` required by the Planner and Localization components.

Unlike standard ROS2 map servers, the EMOS Map Server supports **native 3D Point Cloud (PCD)** files, automatically slicing and projecting them into 2D navigable grids based on configurable height limits.

### Key Features

- {material-regular}`swap_horiz;1.2em;sd-text-primary` **Map Data Conversion** — Reads map files in either 2D (YAML) or 3D (PCD) format and converts the data into usable global map formats (OccupancyGrid).

- {material-regular}`public;1.2em;sd-text-primary` **Global Map Serving** — Once map data is loaded and processed, the MapServer publishes the global map as an `OccupancyGrid` message, continuously available for path planning, localization, and obstacle detection.

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

## Global Mapping & Localization

EMOS is designed to be modular. While it handles core navigation, it relies on standard community packages for global localization and mapping. Recommended solutions:

| Package | Purpose |
| :--- | :--- |
| **[Robot Localization](https://github.com/cra-ros-pkg/robot_localization)** | Sensor Fusion (EKF) — Fuse IMU, Odometry, and GPS data for robust `odom` → `base_link` transforms. |
| **[SLAM Toolbox](https://github.com/SteveMacenski/slam_toolbox)** | 2D SLAM & Localization — Generate initial maps or perform "Lifelong" mapping in changing environments. |
| **[Glim](https://koide3.github.io/glim/)** | 3D LiDAR-Inertial Mapping — GPU-accelerated 3D SLAM using LiDAR and IMU data. |

:::{tip}
Remember that EMOS includes its own [3D-capable Map Server](#map-server) if you need to work directly with Point Cloud (`.pcd`) files generated by tools like Glim.
:::
