"""A native mapping session with GLIM on the robot plugin's LiDAR and IMU, and the
map built from its clouds."""

from __future__ import annotations

import argparse
import faulthandler
import os
import signal
import sys
import time
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

from ament_index_python.packages import PackageNotFoundError
from ros_sugar import Launcher
from ros_sugar.io import Topic
from ros_sugar.robot import RobotPlugin, RosTopicTransport
from ros_sugar.robot.cli import load_plugin_class
from ros_sugar.robot.mapping import NativeMapping

from .backend import backend_has_cuda, backend_version, glim_node, write_glim_config
from .builder import MapBuilder, MapBuilderConfig


def feedback_topic(plugin, key: Optional[str]) -> Optional[str]:
    """The ROS topic the plugin's feedback key is published on."""
    if key is None:
        return None
    feedback = plugin.feedbacks.get(key)
    if feedback is None:
        raise ValueError(f"the plugin has no feedback '{key}'")
    if not isinstance(feedback.transport, RosTopicTransport):
        raise TypeError(
            f"feedback '{key}' is not a ROS topic; native mapping reads the sensors from ROS"
        )
    return feedback.transport.topic_name


def mount_heights(plugin) -> Dict[str, float]:
    """Height above the base frame of each sensor frame the plugin mounts."""
    return {
        mount.child: float(mount.xyz[2])
        for mount in plugin.mounts
        if isinstance(mount.child, str)
    }


def imu_offset(declaration) -> Optional[Tuple[Tuple[float, ...], Tuple[float, ...]]]:
    """The IMU's (xyz, rpy) in the LiDAR frame, when the declaration gives it."""
    if declaration.imu_xyz is None:
        return None
    return declaration.imu_xyz, declaration.imu_rpy


def map_directory(store: str, name: str) -> str:
    return os.path.join(store, f"{name}-{datetime.now():%Y%m%d-%H%M%S}")


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(prog="session", description=__doc__)
    parser.add_argument(
        "--plugin",
        required=True,
        help="robot plugin, as '<package.module>:<ClassName>'",
    )
    parser.add_argument(
        "--name",
        required=True,
        help="map name; the directory gets a timestamp appended",
    )
    parser.add_argument(
        "--store", help="maps directory (default: the plugin's declared store)"
    )
    args = parser.parse_args(argv)

    # get plugin and its mapping declaration
    try:
        plugin = load_plugin_class(args.plugin)()
    except (ImportError, AttributeError, ValueError) as e:
        print(f"could not load plugin '{args.plugin}': {e}", file=sys.stderr)
        return 2
    if not isinstance(plugin, RobotPlugin) or not isinstance(
        plugin.MAPPING, NativeMapping
    ):
        print(f"{args.plugin} does not declare native mapping", file=sys.stderr)
        return 2
    declaration = plugin.MAPPING
    # get mapping topics
    try:
        points_topic = feedback_topic(plugin, declaration.cloud)
        imu_topic = feedback_topic(plugin, declaration.imu)
    except (ValueError, TypeError) as e:
        print(f"cannot map with {args.plugin}: {e}", file=sys.stderr)
        return 2

    try:
        version = backend_version()
        gpu = backend_has_cuda()
    except PackageNotFoundError:
        print("the mapping backend (GLIM) is not installed", file=sys.stderr)
        return 2

    # specify map store
    store = os.path.expanduser(args.store or declaration.store)
    directory = map_directory(store, args.name)
    os.makedirs(directory)
    # setup glim
    glim_dir = os.path.join(directory, "glim")
    lidar_imu = imu_offset(declaration)
    config_dir = write_glim_config(
        os.path.join(glim_dir, "config"),
        points_topic,
        imu_topic,
        gpu=gpu,
        lidar_imu=lidar_imu,
        base_frame=plugin.base_frame,
    )
    dump_dir = os.path.join(glim_dir, "dump")

    # setup map builder node
    builder = MapBuilder(
        "map_builder",
        cloud_topic=Topic(
            name=declaration.cloud, msg_type="PointCloud2", use_plugin=True
        ),
        config=MapBuilderConfig(
            loop_rate=1.0,
            output_dir=directory,
            resolution=declaration.resolution,
            z_min=declaration.z_min,
            z_max=declaration.z_max,
            mount_heights=mount_heights(plugin),
            base_height=plugin.base_height if plugin.base_height is not None else 0.0,
            cloud_topic_name=points_topic or "",
        ),
    )
    # launch
    launcher = Launcher(robot_plugin=plugin)
    launcher.add_pkg(
        components=[builder], package_name="emos_mapping", multiprocessing=False
    )
    launcher.add_ros_node(**glim_node(config_dir, dump_dir))

    metadata: Dict[str, Any] = {
        "name": args.name,
        "created_at": datetime.now().astimezone().isoformat(timespec="seconds"),
        "provider": "native",
        "robot": {"plugin": args.plugin, "name": plugin.metadata.name},
        "backend": {"name": "glim", "version": version, "gpu": gpu},
        "inputs": {"cloud": points_topic, "imu": imu_topic, "lidar_imu": lidar_imu},
        "band": {"z_min": declaration.z_min, "z_max": declaration.z_max},
    }
    builder.metadata = metadata
    # A hung session dumps every thread's stack into the log on SIGUSR1
    faulthandler.register(signal.SIGUSR1, all_threads=True)

    # Operator guidance is the CLI's; this only states the facts it needs.
    print(f"Map directory: {directory}", flush=True)
    started = time.time()
    try:
        launcher.bringup()
    finally:
        # Cover a launch that ended without tearing it down
        saved = builder.finish()
        if saved:
            print(f"Map saved: {directory} ({time.time() - started:.0f} s)", flush=True)
        else:
            print(
                "No map written: GLIM published no map",
                file=sys.stderr,
                flush=True,
            )
    return 0 if saved else 1


if __name__ == "__main__":
    sys.exit(main())
