"""The component that turns GLIM's map cloud into the map files."""

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

from .backend import MAP_TOPIC
from .grid import (
    Grid,
    GridSpec,
    Ground,
    build_grid,
    estimate_ground,
    render_preview,
    write_artifact,
    write_png,
)

PREVIEW_FILE = "preview.png"
STATE_FILE = "state.json"
METADATA_FILE = "map.json"


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


class MapBuilder(BaseComponent):
    """Builds the map files from the map cloud GLIM publishes during a session.

    Each run should end in an area that is already mapped.

    cloud_topic is the plugin's LiDAR feedback. As an input it makes the
    plugin start the LiDAR driver, and its messages name the LiDAR frame.
    """

    def __init__(
        self,
        component_name: str,
        cloud_topic: Optional[Topic] = None,
        config: Optional[MapBuilderConfig] = None,
        **kwargs,
    ):
        self.map_topic = Topic(name=MAP_TOPIC, msg_type="PointCloud2")
        inputs = [self.map_topic] + ([cloud_topic] if cloud_topic else [])
        config = config or MapBuilderConfig()
        super().__init__(component_name, inputs=inputs, config=config, **kwargs)
        self.config: MapBuilderConfig
        self.spec = GridSpec(
            resolution=config.resolution, z_min=config.z_min, z_max=config.z_max
        )
        self.points: Optional[np.ndarray] = None
        self.expected_ground: Optional[float] = None
        self._map_msg = None
        self.started_at = time.time()

    def _execution_step(self) -> None:
        self._place_ground()
        if self._take_map():
            self.write_preview()

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
                height = self.config.mount_heights[frame] + self.config.base_height
                self.expected_ground = -height
                self.get_logger().info(
                    f"Expecting the ground {height:.2f} m below the LiDAR frame '{frame}'"
                )
            return

    def _build(self) -> Optional[Tuple[Ground, Grid]]:
        """The ground found in the map's points and their grid, or None
        without points."""
        if self.points is None or len(self.points) == 0:
            return None
        ground = estimate_ground(self.points[:, 2], expected=self.expected_ground)
        return ground, build_grid(self.points, self.spec, ground)

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
            "ground": {"z": round(ground.z, 3), "source": ground.source},
            **grid.counts(),
            "elapsed_s": round(time.time() - self.started_at, 1),
        }
        with open(os.path.join(self.config.output_dir, STATE_FILE), "w") as f:
            json.dump(state, f)

    def finish(self, metadata: Dict[str, Any]) -> Optional[Dict[str, str]]:
        """Write the map files and map.json, and return their paths by role.
        None when GLIM never published a map."""
        self._take_map()  # one may have arrived since the last step
        built = self._build()
        if built is None:
            return None
        ground, grid = built
        paths = write_artifact(self.config.output_dir, self.points, grid)
        record = {
            "schema_version": 1,
            **metadata,
            "duration_s": round(time.time() - self.started_at, 1),
            "grid": {
                "resolution": grid.resolution,
                "origin": [round(grid.origin[0], 4), round(grid.origin[1], 4), 0.0],
                "width": grid.width,
                "height": grid.height,
                "ground": {"z": round(ground.z, 3), "source": ground.source},
            },
            "points": len(self.points),
        }
        paths["metadata"] = os.path.join(self.config.output_dir, METADATA_FILE)
        with open(paths["metadata"], "w") as f:
            json.dump(record, f, indent=2)
            f.write("\n")
        return paths
