"""Occupancy grids from a 3D map cloud.

The final map and the live preview come from one raster: a height slice of
the SLAM backend's global point cloud, written in the ROS map_server format
that Kompass's MapServer loads. No ROS here, so it runs and tests anywhere.
"""

from __future__ import annotations

import os
import struct
import zlib
from typing import Dict, Optional, Tuple

import numpy as np
from attrs import define, field

# Pixel values of a map_server PGM with negate 0.
FREE = 254
UNKNOWN = 205
OCCUPIED = 0


@define(frozen=True, kw_only=True)
class GridSpec:
    """How the cloud is sliced into a grid.

    z_min and z_max bound the obstacle band, in metres above the ground plane.
    Points between floor_depth below the ground and z_min are floor returns.
    """

    resolution: float = 0.05
    z_min: float = 0.15
    z_max: float = 0.80
    floor_depth: float = 0.10
    # Obstacle points that make a cell occupied
    min_points: int = 2
    # Unknown margin around the mapped area, in metres
    padding: float = 0.5


@define(frozen=True)
class Ground:
    """Height of the ground plane in the map frame, and how it was found."""

    z: float
    source: str  # "expected" or "estimated"


@define
class Grid:
    """A grid in map_server layout: row 0 is the top of the map (largest y),
    and origin is the world position of the bottom-left cell."""

    data: np.ndarray = field(
        eq=False
    )  # (height, width) uint8 of FREE, UNKNOWN or OCCUPIED
    origin: Tuple[float, float]
    resolution: float

    @property
    def height(self) -> int:
        return self.data.shape[0]

    @property
    def width(self) -> int:
        return self.data.shape[1]

    def cell(self, x: float, y: float) -> int:
        """The value at world position (x, y), UNKNOWN outside the grid."""
        col = int((x - self.origin[0]) / self.resolution)
        row = self.height - 1 - int((y - self.origin[1]) / self.resolution)
        if 0 <= row < self.height and 0 <= col < self.width:
            return int(self.data[row, col])
        return UNKNOWN

    def counts(self) -> Dict[str, int]:
        return {
            "occupied": int((self.data == OCCUPIED).sum()),
            "free": int((self.data == FREE).sum()),
            "unknown": int((self.data == UNKNOWN).sum()),
        }


def estimate_ground(
    z: np.ndarray,
    expected: Optional[float] = None,
    window: float = 0.15,
    bin_size: float = 0.05,
) -> Ground:
    """Find the ground plane's height among the points' z values.

    With an expected height (from the robot's LiDAR mount), the band within
    window of it that stands out as a plane is taken. A floor concentrates in
    one band, where a wall spreads evenly over many. Otherwise, the ground is the
    lowest band holding a large share of the points.
    """
    z = np.asarray(z, dtype=np.float64)
    z = z[np.isfinite(z)]
    if z.size == 0:
        raise ValueError("no points to find the ground in")
    edges = np.arange(z.min(), z.max() + 2 * bin_size, bin_size)
    counts, _ = np.histogram(z, bins=edges)
    centres = edges[:-1] + bin_size / 2

    if expected is not None:
        near = np.flatnonzero(np.abs(centres - expected) <= window)
        if near.size:
            best = near[np.argmax(counts[near])]
            others = counts[near][counts[near] != counts[best]]
            plane = counts[best] >= max(20, 0.01 * z.size) and (
                others.size == 0 or counts[best] >= 2.5 * others.mean()
            )
            if plane:
                return Ground(_refine(z, float(centres[best]), bin_size), "expected")

    dense = np.flatnonzero(counts >= 0.25 * counts.max())
    return Ground(_refine(z, float(centres[dense[0]]), bin_size), "estimated")


def _refine(z: np.ndarray, centre: float, bin_size: float) -> float:
    """The median height of the points within a bin of centre"""
    band = z[(z >= centre - bin_size) & (z <= centre + bin_size)]
    return float(np.median(band))


def build_grid(points: np.ndarray, spec: GridSpec, ground: Ground) -> Grid:
    """Slice a cloud (N, 3) in the map frame into a grid.

    A cell is occupied when at least min_points obstacle-band points fall in
    it, free when it holds floor returns and no obstacle, unknown otherwise.
    """
    pts = np.asarray(points, dtype=np.float64).reshape(-1, 3)
    pts = pts[np.isfinite(pts).all(axis=1)]
    z = pts[:, 2] - ground.z
    obstacle = (z >= spec.z_min) & (z <= spec.z_max)
    floor = (z >= -spec.floor_depth) & (z < spec.z_min)
    used = obstacle | floor
    if not used.any():
        return Grid(
            np.full((1, 1), UNKNOWN, dtype=np.uint8), (0.0, 0.0), spec.resolution
        )

    res = spec.resolution
    xy = pts[used, :2]
    x0 = np.floor((xy[:, 0].min() - spec.padding) / res) * res
    y0 = np.floor((xy[:, 1].min() - spec.padding) / res) * res
    width = int(np.ceil((xy[:, 0].max() + spec.padding - x0) / res)) + 1
    height = int(np.ceil((xy[:, 1].max() + spec.padding - y0) / res)) + 1

    cols = ((xy[:, 0] - x0) / res).astype(np.int64)
    rows = ((xy[:, 1] - y0) / res).astype(np.int64)
    index = rows * width + cols
    obstacle_counts = np.bincount(index[obstacle[used]], minlength=height * width)
    floor_counts = np.bincount(index[floor[used]], minlength=height * width)

    data = np.full(height * width, UNKNOWN, dtype=np.uint8)
    data[floor_counts >= 1] = FREE
    data[obstacle_counts >= spec.min_points] = OCCUPIED
    # Rows are built bottom-up; the image has the top of the map first.
    return Grid(data.reshape(height, width)[::-1].copy(), (float(x0), float(y0)), res)


# --- Preview -----------------------------------------------------------------

PREVIEW_COLOURS = {
    FREE: (245, 245, 245),
    UNKNOWN: (160, 160, 160),
    OCCUPIED: (30, 30, 30),
}


def render_preview(grid: Grid) -> np.ndarray:
    """The grid as an RGB image (height, width, 3), for the dashboard."""
    rgb = np.zeros(grid.data.shape + (3,), dtype=np.uint8)
    for value, colour in PREVIEW_COLOURS.items():
        rgb[grid.data == value] = colour
    return rgb


# --- Files -------------------------------------------------------------------

GRID_IMAGE = "occ_grid.pgm"
GRID_YAML = "occ_grid.yaml"
CLOUD_FILE = "full_cloud.pcd"
PREVIEW_FILE = "preview.png"


def write_artifact(directory: str, points: np.ndarray, grid: Grid) -> Dict[str, str]:
    """Write the map files into directory, and return them by role."""
    os.makedirs(directory, exist_ok=True)
    paths = {
        "grid": os.path.join(directory, GRID_IMAGE),
        "yaml": os.path.join(directory, GRID_YAML),
        "cloud": os.path.join(directory, CLOUD_FILE),
        "preview": os.path.join(directory, PREVIEW_FILE),
    }
    write_pgm(paths["grid"], grid.data)
    write_yaml(paths["yaml"], GRID_IMAGE, grid.resolution, grid.origin)
    write_pcd(paths["cloud"], points)
    write_png(paths["preview"], render_preview(grid))
    return paths


def write_pgm(path: str, data: np.ndarray) -> None:
    data = np.ascontiguousarray(data, dtype=np.uint8)
    with open(path, "wb") as f:
        f.write(f"P5\n{data.shape[1]} {data.shape[0]}\n255\n".encode())
        f.write(data.tobytes())


def write_yaml(
    path: str, image: str, resolution: float, origin: Tuple[float, float]
) -> None:
    """A map_server YAML, with the image named relative to it"""
    with open(path, "w") as f:
        f.write(
            f"image: {image}\n"
            f"resolution: {resolution}\n"
            f"origin: [{origin[0]:.4f}, {origin[1]:.4f}, 0.0]\n"
            "negate: 0\n"
            "occupied_thresh: 0.65\n"
            "free_thresh: 0.196\n"
        )


def write_pcd(path: str, points: np.ndarray) -> None:
    """A binary PCD of the cloud's x, y, z"""
    pts = np.ascontiguousarray(np.asarray(points, dtype=np.float32).reshape(-1, 3))
    header = (
        "# .PCD v0.7 - Point Cloud Data file format\n"
        "VERSION 0.7\n"
        "FIELDS x y z\n"
        "SIZE 4 4 4\n"
        "TYPE F F F\n"
        "COUNT 1 1 1\n"
        f"WIDTH {len(pts)}\n"
        "HEIGHT 1\n"
        "VIEWPOINT 0 0 0 1 0 0 0\n"
        f"POINTS {len(pts)}\n"
        "DATA binary\n"
    )
    with open(path, "wb") as f:
        f.write(header.encode())
        f.write(pts.tobytes())


def write_png(path: str, rgb: np.ndarray) -> None:
    """An RGB PNG"""
    rgb = np.ascontiguousarray(rgb, dtype=np.uint8)
    height, width = rgb.shape[:2]
    raw = b"".join(b"\x00" + rgb[row].tobytes() for row in range(height))

    def chunk(kind: bytes, body: bytes) -> bytes:
        return (
            struct.pack(">I", len(body))
            + kind
            + body
            + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)
        )

    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)))
        f.write(chunk(b"IDAT", zlib.compress(raw)))
        f.write(chunk(b"IEND", b""))
