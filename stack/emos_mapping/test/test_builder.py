import json
import os

import numpy as np
import pytest

pytest.importorskip("ros_sugar")
import rclpy  # noqa: E402
from ros_sugar.io import Topic  # noqa: E402
from sensor_msgs.msg import PointField  # noqa: E402
from sensor_msgs_py import point_cloud2  # noqa: E402
from std_msgs.msg import Header  # noqa: E402

from emos_mapping.builder import PREVIEW_FILE, STATE_FILE, MapBuilder, MapBuilderConfig  # noqa: E402
from emos_mapping.grid import FREE, OCCUPIED  # noqa: E402

from test_grid import LIDAR_HEIGHT, room, surface  # noqa: E402


def cloud_msg(points, frame_id="map"):
    return point_cloud2.create_cloud_xyz32(Header(frame_id=frame_id), points)


def make_builder(tmp_path, **config):
    """A MapBuilder with its node up, as the launcher leaves it before activation"""
    builder = MapBuilder(
        "map_builder_test",
        cloud_topic=Topic(name="lidar", msg_type="PointCloud2", use_plugin=True),
        config=MapBuilderConfig(loop_rate=1.0, output_dir=str(tmp_path), **config),
    )
    builder.rclpy_init_node()
    return builder


def publish_map(builder, points):
    builder.callbacks[builder.map_topic.name].callback(cloud_msg(points.tolist()))


@pytest.fixture(autouse=True)
def ros():
    rclpy.init()
    yield
    rclpy.shutdown()


def test_the_preview_follows_the_map_and_the_ground_comes_from_the_mount(tmp_path):
    builder = make_builder(tmp_path, mount_heights={"livox_frame": 0.1}, base_height=0.3)
    builder.callbacks["lidar"].callback(cloud_msg([(1.0, 0.0, 0.0)], frame_id="livox_frame"))
    publish_map(builder, room())
    builder._execution_step()
    assert builder.expected_ground == pytest.approx(-0.4)
    state = json.load(open(tmp_path / STATE_FILE))
    assert state["ground"]["source"] == "expected" and state["occupied"] > 0
    assert state["points"] == len(room())

    written = os.stat(tmp_path / PREVIEW_FILE).st_mtime_ns
    builder._execution_step()  # no new map: no rebuild
    assert os.stat(tmp_path / PREVIEW_FILE).st_mtime_ns == written


def test_each_map_replaces_the_last(tmp_path):
    builder = make_builder(tmp_path)
    publish_map(builder, room())
    builder._execution_step()
    grown = np.vstack([room(), surface(6, 7, 6, 7, -LIDAR_HEIGHT)])  # floor outside the room
    publish_map(builder, grown)
    builder._execution_step()
    assert len(builder.points) == len(grown)
    _, grid = builder._build()
    assert grid.cell(6.5, 6.5) == FREE and grid.cell(5.0, 0.0) == OCCUPIED


def test_finish_writes_the_map_files_and_their_record(tmp_path):
    builder = make_builder(tmp_path, resolution=0.1)
    publish_map(builder, room())  # arrives after the last step: still saved
    paths = builder.finish({"name": "room", "provider": "native"})
    for role in ("grid", "yaml", "cloud", "preview", "metadata"):
        assert os.path.isfile(paths[role]), role
    record = json.load(open(paths["metadata"]))
    assert record["schema_version"] == 1 and record["name"] == "room" and record["provider"] == "native"
    assert record["grid"]["resolution"] == 0.1 and record["grid"]["ground"]["source"] == "estimated"
    assert record["points"] == len(room()) and "duration_s" in record


def test_nothing_is_written_when_glim_published_no_map(tmp_path):
    builder = make_builder(tmp_path)
    builder._execution_step()
    assert builder.finish({"name": "room"}) is None
    assert os.listdir(tmp_path) == []


def test_it_is_built_the_way_the_executable_builds_it(tmp_path):
    launched = make_builder(tmp_path)
    component = MapBuilder(config=MapBuilderConfig(output_dir=str(tmp_path)), component_name="b", config_file=None)
    component.rclpy_init_node()
    component._inputs_json = launched._inputs_json
    assert set(component.callbacks) == {"lidar", component.map_topic.name}
    assert component.callbacks["lidar"].input_topic.use_plugin


def test_points_are_read_from_a_cloud_with_mixed_field_types():
    fields = [
        PointField(name="x", offset=0, datatype=PointField.FLOAT32, count=1),
        PointField(name="y", offset=4, datatype=PointField.FLOAT32, count=1),
        PointField(name="z", offset=8, datatype=PointField.FLOAT32, count=1),
        PointField(name="ring", offset=12, datatype=PointField.UINT16, count=1),
    ]
    rows = [(1.0, 2.0, 3.0, 4), (float("nan"), 0.0, 0.0, 1), (-1.5, 0.5, 0.25, 7)]
    points = MapBuilder._points(point_cloud2.create_cloud(Header(frame_id="map"), fields, rows))
    assert points.dtype == np.float32
    assert points.tolist() == [[1.0, 2.0, 3.0], [-1.5, 0.5, 0.25]]
