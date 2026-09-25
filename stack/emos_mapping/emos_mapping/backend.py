"""The GLIM backend configuration for a session, and how it is launched."""

from __future__ import annotations

import json
import os
import shutil
import xml.etree.ElementTree as ET
from typing import Any, Dict, Optional, Sequence, Tuple

import numpy as np
from ament_index_python.packages import (
    PackageNotFoundError,
    get_package_prefix,
    get_package_share_directory,
)
from ros_sugar.robot.mount import quaternion_from_euler

GLIM_PACKAGE = "glim_ros"
GLIM_CORE_PACKAGE = "glim"
# The odometry module GLIM's GPU configuration loads (only on CUDA build)
CUDA_ODOMETRY_MODULE = "libodometry_estimation_gpu.so"
GLIM_EXECUTABLE = "glim_rosnode"
GLIM_NODE = "glim"
# GLIM's rviz_viewer module publishes every finished submap here, merged at its
# optimised pose, at most every 10 s
MAP_TOPIC = f"/{GLIM_NODE}/map"


def backend_has_cuda() -> bool:
    """Whether the installed GLIM was built with its CUDA modules."""
    return os.path.isfile(
        os.path.join(get_package_prefix(GLIM_CORE_PACKAGE), "lib", CUDA_ODOMETRY_MODULE)
    )


def backend_version() -> str:
    """The installed GLIM's version, from its package manifest. Raises
    PackageNotFoundError when GLIM is not installed."""
    manifest = os.path.join(get_package_share_directory(GLIM_PACKAGE), "package.xml")
    return ET.parse(manifest).getroot().findtext("version")


def templates_dir() -> str:
    """GLIM's configuration templates from the installed package, or from
    the source tree when running from a checkout."""
    try:
        return os.path.join(get_package_share_directory("emos_mapping"), "glim_config")
    except PackageNotFoundError:
        return os.path.join(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "glim_config"
        )


def write_glim_config(
    directory: str,
    points_topic: str,
    imu_topic: Optional[str],
    gpu: bool = False,
    lidar_imu: Optional[Tuple[Sequence[float], Sequence[float]]] = None,
    base_frame: Optional[str] = None,
) -> str:
    """Write GLIM's configuration for a session into directory and return it.
    GLIM's window viewer needs a display. base_frame is the robot's base frame,
    which GLIM then publishes odom -> base_frame for, looking the IMU's mount
    up in TF.
    """
    os.makedirs(directory, exist_ok=True)
    templates = templates_dir()
    for name in os.listdir(templates):
        if name.endswith(".json"):
            shutil.copy(os.path.join(templates, name), directory)

    variant = "gpu" if gpu else "cpu"
    config = _read(os.path.join(directory, "config.json"))
    config["global"].update(
        {
            # use lidar odometery when available, else glim's default
            "config_odometry": "config_odometry_ct.json"
            if imu_topic is None
            else f"config_odometry_{variant}.json",
            "config_sub_mapping": f"config_sub_mapping_{variant}.json",
            "config_global_mapping": f"config_global_mapping_{variant}.json",
        }
    )
    _write(os.path.join(directory, "config.json"), config)

    ros = _read(os.path.join(directory, "config_ros.json"))
    ros["glim_ros"].update(
        {
            "points_topic": points_topic,
            "imu_topic": imu_topic or "",
            "base_frame_id": base_frame or "",
            # The IMU is the LiDAR's own, so the two frames are one
            "publish_imu2lidar": False,
            "extension_modules": ["librviz_viewer.so"],
        }
    )
    _write(os.path.join(directory, "config_ros.json"), ros)

    if lidar_imu is not None:
        xyz, rpy = lidar_imu
        sensors = _read(os.path.join(directory, "config_sensors.json"))
        sensors["sensors"]["T_lidar_imu"] = [float(v) for v in xyz] + list(
            quaternion_from_euler(*rpy)
        )
        _write(os.path.join(directory, "config_sensors.json"), sensors)
    return directory


def glim_node(config_dir: str, dump_dir: str) -> Dict[str, Any]:
    """Keyword arguments for Launcher to start GLIM. It saves its
    dump into dump_dir when it shuts down."""
    return {
        "package": GLIM_PACKAGE,
        "executable": GLIM_EXECUTABLE,
        "name": GLIM_NODE,
        "parameters": [{"config_path": config_dir, "dump_path": dump_dir}],
        "output": "screen",
    }


def read_dump(dump_dir: str) -> Optional[np.ndarray]:
    """The map from the dump GLIM writes on shutdown, as (N, 3) float32 in
    the map frame, or None without a submap. Each submap is a numbered
    directory holding its optimised pose in data.txt and its merged points,
    in its own frame, in points_compact.bin."""
    if not os.path.isdir(dump_dir):
        return None
    clouds = []
    for name in sorted(os.listdir(dump_dir)):
        submap = os.path.join(dump_dir, name)
        data_file = os.path.join(submap, "data.txt")
        points_file = os.path.join(submap, "points_compact.bin")
        if not (
            name.isdigit() and os.path.isfile(data_file) and os.path.isfile(points_file)
        ):
            continue
        pose = _submap_pose(data_file).astype(np.float32)
        points = np.fromfile(points_file, dtype=np.float32)
        points = points[: len(points) - len(points) % 3].reshape(-1, 3)
        points = points[np.isfinite(points).all(axis=1)]
        clouds.append(points @ pose[:3, :3].T + pose[:3, 3])
    if not clouds:
        return None
    return np.vstack(clouds)


def _submap_pose(data_file: str) -> np.ndarray:
    """T_world_origin from a submap's data.txt: the 4 x 4 matrix under its label."""
    with open(data_file) as f:
        lines = f.read().splitlines()
    for i, line in enumerate(lines):
        if line.startswith("T_world_origin:"):
            return np.array(
                [[float(v) for v in row.split()] for row in lines[i + 1 : i + 5]]
            )
    raise ValueError(f"no T_world_origin in {data_file}")


def strip_json_comments(text: str) -> str:
    """Remove // and /* */ comments outside strings that GLIM's files carry."""
    out = []
    i, n = 0, len(text)
    while i < n:
        c = text[i]
        if c == '"':
            j = i + 1
            while j < n and text[j] != '"':
                j += 2 if text[j] == "\\" else 1
            out.append(text[i : j + 1])
            i = j + 1
        elif text.startswith("//", i):
            i = text.find("\n", i)
            i = n if i < 0 else i
        elif text.startswith("/*", i):
            end = text.find("*/", i + 2)
            i = n if end < 0 else end + 2
        else:
            out.append(c)
            i += 1
    return "".join(out)


def _read(path: str) -> Dict[str, Any]:
    with open(path) as f:
        return json.loads(strip_json_comments(f.read()))


def _write(path: str, config: Dict[str, Any]) -> None:
    with open(path, "w") as f:
        json.dump(config, f, indent=2)
        f.write("\n")
