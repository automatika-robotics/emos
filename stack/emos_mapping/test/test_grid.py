import os
import struct

import numpy as np

from emos_mapping.grid import (
    FREE,
    OCCUPIED,
    UNKNOWN,
    GridSpec,
    Ground,
    build_grid,
    estimate_ground,
    write_artifact,
)

LIDAR_HEIGHT = 0.40  # the map frame's origin is the LiDAR; the floor is below it


def surface(x0, x1, y0, y1, z, step=0.02, rng=None):
    xs = np.arange(x0, x1, step)
    ys = np.arange(y0, y1, step)
    x, y = np.meshgrid(xs, ys)
    pts = np.column_stack([x.ravel(), y.ravel(), np.full(x.size, z)])
    if rng is not None:
        pts[:, 2] += rng.normal(0, 0.005, len(pts))
    return pts


def wall(x0, x1, y0, y1, z0, z1, step=0.02):
    """A vertical surface along the line from (x0, y0) to (x1, y1)"""
    n = max(int(np.hypot(x1 - x0, y1 - y0) / step), 1)
    t = np.linspace(0, 1, n)
    zs = np.arange(z0, z1, step)
    x = np.repeat(x0 + t * (x1 - x0), len(zs))
    y = np.repeat(y0 + t * (y1 - y0), len(zs))
    z = np.tile(zs, n)
    return np.column_stack([x, y, z])


def room():
    """A 10 x 8 m room seen from a LiDAR 0.4 m above the floor: floor, four
    walls, a table, a cable on the floor and a ceiling."""
    rng = np.random.default_rng(0)
    g = -LIDAR_HEIGHT
    parts = [
        surface(-5, 5, -4, 4, g, rng=rng),  # floor
        wall(-5, 5, -4, -4, g, g + 2.5),
        wall(-5, 5, 4, 4, g, g + 2.5),
        wall(-5, -5, -4, 4, g, g + 2.5),
        wall(5, 5, -4, 4, g, g + 2.5),
        surface(1, 2, 1, 2, g + 0.75),  # table top, inside the obstacle band
        surface(-3, -2, -3, -2.95, g + 0.05),  # a cable: below z_min, so floor
        surface(-5, 5, -4, 4, g + 2.5, step=0.05),  # ceiling, above the band
    ]
    return np.vstack(parts)


def test_ground_is_found_as_the_lowest_dense_band():
    ground = estimate_ground(room()[:, 2])
    assert ground.source == "estimated"
    assert abs(ground.z - (-LIDAR_HEIGHT)) < 0.02


def test_an_expected_height_is_used_when_the_floor_is_there():
    ground = estimate_ground(room()[:, 2], expected=-LIDAR_HEIGHT + 0.05)
    assert ground.source == "expected"
    assert abs(ground.z - (-LIDAR_HEIGHT)) < 0.02


def test_a_wrong_expected_height_falls_back_to_the_estimate():
    ground = estimate_ground(room()[:, 2], expected=1.5)
    assert ground.source == "estimated"
    assert abs(ground.z - (-LIDAR_HEIGHT)) < 0.02


def test_sparse_returns_below_the_floor_are_not_the_ground():
    pts = room()
    below = surface(-1, 1, -1, 1, -LIDAR_HEIGHT - 0.6, step=0.5)  # a few reflections
    ground = estimate_ground(np.vstack([pts, below])[:, 2])
    assert abs(ground.z - (-LIDAR_HEIGHT)) < 0.02


def test_grid_classifies_walls_table_cable_and_ceiling():
    pts = room()
    ground = Ground(-LIDAR_HEIGHT, "expected")
    grid = build_grid(pts, GridSpec(z_min=0.15, z_max=0.80), ground)

    assert grid.cell(0.0, 0.0) == FREE  # open floor
    assert grid.cell(5.0, 0.0) == OCCUPIED  # wall
    assert grid.cell(0.0, -4.0) == OCCUPIED
    assert grid.cell(1.5, 1.5) == OCCUPIED  # table top is in the band
    assert grid.cell(-2.5, -2.98) == FREE  # cable is below z_min
    assert grid.cell(7.0, 7.0) == UNKNOWN  # outside the room
    # The ceiling is above the band, so it neither occupies nor frees cells
    counts = grid.counts()
    assert counts["occupied"] > 0 and counts["free"] > counts["occupied"]


def test_grid_layout_matches_map_server():
    pts = room()
    grid = build_grid(pts, GridSpec(), Ground(-LIDAR_HEIGHT, "expected"))
    # The origin is the bottom-left, padded, snapped to the resolution.
    assert grid.origin[0] <= -5.5 and grid.origin[1] <= -4.5
    assert abs(grid.origin[0] / grid.resolution - round(grid.origin[0] / grid.resolution)) < 1e-9
    # Row 0 is the top of the map: the far wall at y = 4 is in the top rows.
    top_rows = grid.data[: int(1.0 / grid.resolution)]
    bottom_rows = grid.data[-int(1.0 / grid.resolution) :]
    assert (top_rows == OCCUPIED).any() and (bottom_rows == OCCUPIED).any()
    assert grid.width >= 10 / grid.resolution and grid.height >= 8 / grid.resolution


def test_a_single_stray_return_does_not_occupy_a_cell():
    pts = np.vstack([
        surface(-2, 2, -2, 2, -LIDAR_HEIGHT),
        np.array([[0.0, 0.0, 0.0]]),  # one point in the obstacle band
    ])
    grid = build_grid(pts, GridSpec(min_points=2), Ground(-LIDAR_HEIGHT, "expected"))
    assert grid.cell(0.0, 0.0) == FREE


def test_no_points_in_range_gives_an_unknown_grid():
    pts = surface(-1, 1, -1, 1, 5.0)  # everything far above the band
    grid = build_grid(pts, GridSpec(), Ground(0.0, "expected"))
    assert grid.counts() == {"occupied": 0, "free": 0, "unknown": 1}


def test_artifact_files_are_what_map_server_and_pcl_read(tmp_path):
    pts = room()
    grid = build_grid(pts, GridSpec(), Ground(-LIDAR_HEIGHT, "expected"))
    paths = write_artifact(str(tmp_path / "room-20260917-120000"), pts, grid)

    with open(paths["grid"], "rb") as f:
        magic, size, maxval = f.readline(), f.readline(), f.readline()
        pixels = f.read()
    assert magic == b"P5\n" and maxval == b"255\n"
    assert size == f"{grid.width} {grid.height}\n".encode()
    assert len(pixels) == grid.width * grid.height

    yaml = open(paths["yaml"]).read()
    assert "image: occ_grid.pgm\n" in yaml
    assert f"resolution: {grid.resolution}\n" in yaml
    assert f"origin: [{grid.origin[0]:.4f}, {grid.origin[1]:.4f}, 0.0]\n" in yaml
    assert "negate: 0\n" in yaml and "occupied_thresh: 0.65\n" in yaml

    with open(paths["cloud"], "rb") as f:
        header = b"".join(iter(f.readline, b"DATA binary\n"))
        body = f.read()
    assert f"POINTS {len(pts)}\n".encode() in header and b"FIELDS x y z\n" in header
    assert len(body) == len(pts) * 3 * 4
    assert np.allclose(np.frombuffer(body, dtype=np.float32).reshape(-1, 3)[:5], pts[:5], atol=1e-5)

    with open(paths["preview"], "rb") as f:
        png = f.read()
    assert png[:8] == b"\x89PNG\r\n\x1a\n"
    width, height = struct.unpack(">II", png[16:24])
    assert (width, height) == (grid.width, grid.height)
    assert os.path.getsize(paths["preview"]) > 100
