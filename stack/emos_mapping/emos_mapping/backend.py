"""The GLIM backend configuration for a session, and how it is launched."""

from __future__ import annotations

import json
import os
import shutil
from typing import Any, Dict, Optional, Sequence, Tuple

from ament_index_python.packages import (
    PackageNotFoundError,
    get_package_share_directory,
)
from ros_sugar.robot.mount import quaternion_from_euler

GLIM_PACKAGE = "glim_ros"
GLIM_EXECUTABLE = "glim_rosnode"
GLIM_NODE = "glim"
# GLIM's rviz_viewer module publishes every finished submap here, merged at its
# optimised pose, at most every 10 s
MAP_TOPIC = f"/{GLIM_NODE}/map"


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
) -> str:
    """Write GLIM's configuration for a session into directory and return it.
    GLIM's window viewer needs a display.
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
