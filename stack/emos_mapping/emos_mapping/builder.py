"""The component that turns GLIM's map into the map files."""

from __future__ import annotations

import json
import os
import time
from typing import Any, Dict, Optional, Tuple

import numpy as np
from attrs import define, field
from numpy.lib.recfunctions import structured_to_unstructured
from ros_sugar.config import BaseComponentConfig
from ros_sugar.core import BaseComponent
from ros_sugar.io import Topic
from sensor_msgs_py import point_cloud2

from .backend import MAP_TOPIC, ODOM_TOPIC, read_dump
from .grid import (
    Grid,
    GridSpec,
    GroundSurface,
    build_grid,
    estimate_ground,
    estimate_ground_surface,
    render_preview,
    write_artifact,
    write_png,
)

PREVIEW_FILE = "preview.png"
STATE_FILE = "state.json"
METADATA_FILE = "map.json"

# NOTE: Floor height difference across a map threshold value. Past this value the operator is warned that
# the map is likely tilted or drifted, in metres
FLOOR_SPAN_WARNING = 0.3


@define(kw_only=True)
class MapBuilderConfig(BaseComponentConfig):
    """Configuration of MapBuilder. It looks for a new map at loop_rate."""

    # Directory the map files are written into
    output_dir: str = field(default="")
    # Grid cell size, in metres
    resolution: float = field(default=0.05)
    # Heights above the ground between which a point is an obstacle, in metres
    z_min: float = field(default=0.15)
    z_max: float = field(default=0.80)
    # Height of each sensor frame above the base frame, from the plugin's mounts
    mount_heights: Dict[str, float] = field(factory=dict)
    # Height of the base frame above the ground when the robot stands
    base_height: float = field(default=0.0)
    # Height of the map's origin w.r.t LiDAR frame.
    # 1 = the origin is 1 m above the LiDAR frame, -0.5 = 0.5 m below it.
    origin_offset: float = field(default=0.0)
    # ROS topic the LiDAR cloud is read from
    cloud_topic_name: str = field(default="")
    # Directory GLIM dumps its final map into when it shuts down
    dump_dir: str = field(default="")


class MapBuilder(BaseComponent):
    """Builds the map files from GLIM's map: the cloud it publishes during
    the session, for the preview, and the dump it writes when the session
    ends, for the map itself.

    cloud_topic is the plugin's LiDAR feedback. As an input it makes the
    plugin start the LiDAR driver, and its messages name the LiDAR frame.
    """

    # Seconds without a point cloud after which the operator is warned
    NO_CLOUD_WARNING_AFTER = 15.0

    def __init__(
        self,
        component_name: str,
        cloud_topic: Optional[Topic] = None,
        config: Optional[MapBuilderConfig] = None,
        **kwargs,
    ):
        self.map_topic = Topic(name=MAP_TOPIC, msg_type="PointCloud2")
        self.odom_topic = Topic(name=ODOM_TOPIC, msg_type="Odometry")
        inputs = [self.map_topic, self.odom_topic]
        inputs += [cloud_topic] if cloud_topic else []
        config = config or MapBuilderConfig()
        super().__init__(component_name, inputs=inputs, config=config, **kwargs)
        self.config: MapBuilderConfig
        self.spec = GridSpec(
            resolution=config.resolution, z_min=config.z_min, z_max=config.z_max
        )
        self.points: Optional[np.ndarray] = None
        self.expected_ground: Optional[float] = None
        self._map_msg = None
        self._cloud_topic = cloud_topic
        self._cloud_warned = False
        self._tracking = False
        # What finish() records in map.json besides the grid. Set by session.
        self.metadata: Dict[str, Any] = {}
        self._saved: Optional[Dict[str, str]] = None
        self._finished = False
        self.started_at = time.time()

    def destroy_node(self):
        # Keep the last live map. The dump is only complete once GLIM has
        # exited, so the session calls finish() after the launcher returns.
        self._take_map()
        super().destroy_node()

    def _execution_step(self) -> None:
        self._check_cloud()
        self._check_tracking()
        self._place_ground()
        if self._take_map():
            self.write_preview()

    def _check_cloud(self) -> None:
        """Warn the operator, on stdout for the CLI to show, when the LiDAR has
        published nothing since the session started."""
        if self._cloud_topic is None:
            return
        topic = self.config.cloud_topic_name or self._cloud_topic.name
        if self.callbacks[self._cloud_topic.name].msg is not None:
            if self._cloud_warned:
                print(
                    f"Mapping warning: point clouds are arriving on {topic} now.",
                    flush=True,
                )
                self._cloud_warned = False
                self._cloud_topic = None
            return
        if (
            not self._cloud_warned
            and time.time() - self.started_at > self.NO_CLOUD_WARNING_AFTER
        ):
            print(
                f"Mapping warning: no point cloud on {topic} after "
                f"{self.NO_CLOUD_WARNING_AFTER:.0f} s; is the LiDAR driver running?",
                flush=True,
            )
            self._cloud_warned = True

    def _check_tracking(self) -> None:
        """Tell the operator, once, that GLIM is tracking the robot: it has
        found gravity while the robot stood still, so it can be driven."""
        if self._tracking or self.callbacks[self.odom_topic.name].msg is None:
            return
        self._tracking = True
        print("Mapping ready: GLIM is tracking the robot.", flush=True)

    def _take_map(self) -> bool:
        """Read GLIM's map if a new one has arrived."""
        msg = self.callbacks[self.map_topic.name].msg
        if msg is None or msg is self._map_msg:
            return False
        self._map_msg = msg
        self.points = self._points(msg)
        return True

    @staticmethod
    def _points(msg) -> np.ndarray:
        """The cloud's x, y, z as (N, 3) float32"""
        xyz = point_cloud2.read_points(msg, field_names=["x", "y", "z"], skip_nans=True)
        return structured_to_unstructured(xyz).astype(np.float32)

    def _place_ground(self) -> None:
        """Set the expected ground height from the mount of the LiDAR's frame."""
        if self.expected_ground is not None:
            return
        for callback in self.callbacks.values():
            if not callback.input_topic.use_plugin or callback.msg is None:
                continue
            frame = callback.msg.header.frame_id
            if frame in self.config.mount_heights:
                height = (
                    self.config.mount_heights[frame]
                    + self.config.base_height
                    + self.config.origin_offset
                )
                self.expected_ground = -height
                self.get_logger().info(
                    f"Expecting the ground {height:.2f} m below the map origin "
                    f"(LiDAR frame '{frame}', offset {self.config.origin_offset:+.2f} m)"
                )
            return

    def _build(self) -> Optional[Tuple[GroundSurface, Grid]]:
        """The floor found in the map's points and their grid, or None
        without points."""
        if self.points is None or len(self.points) == 0:
            return None
        reference = estimate_ground(self.points[:, 2], expected=self.expected_ground)
        ground = estimate_ground_surface(self.points, self.spec, reference)
        return ground, build_grid(self.points, self.spec, ground)

    @staticmethod
    def _ground_record(ground: GroundSurface) -> Dict[str, Any]:
        """What state.json and map.json say about the floor"""
        return {
            "z": round(ground.reference.z, 3),
            "source": ground.reference.source,
            "tiles": ground.found,
            "span": round(ground.span, 3),
        }

    def write_preview(self) -> None:
        """Rewrite preview.png and state.json from the latest map."""
        built = self._build()
        if built is None:
            return
        ground, grid = built
        write_png(
            os.path.join(self.config.output_dir, PREVIEW_FILE), render_preview(grid)
        )
        state = {
            "points": len(self.points),
            "ground": self._ground_record(ground),
            **grid.counts(),
            "elapsed_s": round(time.time() - self.started_at, 1),
        }
        with open(os.path.join(self.config.output_dir, STATE_FILE), "w") as f:
            json.dump(state, f)

    def finish(
        self, metadata: Optional[Dict[str, Any]] = None
    ) -> Optional[Dict[str, str]]:
        """Write the map files and map.json, and return their paths by role.
        None when GLIM produced no map. The dump holds every submap at its
        final pose, including the last, which GLIM never publishes. Called
        once the session has ended, so the dump is complete."""
        if self._finished:
            return self._saved
        self._finished = True
        self._take_map()  # one may have arrived since the last step
        dumped = read_dump(self.config.dump_dir) if self.config.dump_dir else None
        if dumped is not None:
            self.points = dumped
        built = self._build()
        if built is None:
            return None
        ground, grid = built
        source = "dump" if dumped is not None else "live"
        if dumped is None and self.config.dump_dir:
            print(
                f"Mapping warning: GLIM left no map in {self.config.dump_dir}. "
                "The map is built from the last one it published, which is "
                "thinned out and misses the last few metres driven.",
                flush=True,
            )
        if ground.found == 0:
            print(
                "Mapping warning: no floor was found in the map; the grid is "
                f"sliced at one ground height ({ground.reference.z:.2f} m, "
                f"{ground.reference.source}). Check it before navigating on it.",
                flush=True,
            )
        elif ground.span > FLOOR_SPAN_WARNING:
            print(
                f"Mapping warning: the floor's height varies by {ground.span:.1f} m "
                "across the map, so the 3D map is likely tilted or has drifted. "
                "The grid follows the floor, but check it before navigating on it, "
                "or map again.",
                flush=True,
            )
        paths = write_artifact(self.config.output_dir, self.points, grid)
        record = {
            "schema_version": 1,
            **(metadata if metadata is not None else self.metadata),
            "duration_s": round(time.time() - self.started_at, 1),
            "grid": {
                "resolution": grid.resolution,
                "origin": [round(grid.origin[0], 4), round(grid.origin[1], 4), 0.0],
                "width": grid.width,
                "height": grid.height,
                "ground": self._ground_record(ground),
            },
            "points": len(self.points),
            "cloud_source": source,
        }
        paths["metadata"] = os.path.join(self.config.output_dir, METADATA_FILE)
        with open(paths["metadata"], "w") as f:
            json.dump(record, f, indent=2)
            f.write("\n")
        self._saved = paths
        return paths
