import os
import re
import sys

import pytest

pytest.importorskip("ros_sugar")
from ros_sugar.robot import Mount, RosTopicTransport  # noqa: E402
from ros_sugar.robot.mapping import NativeMapping  # noqa: E402

from emos_mapping.session import (  # noqa: E402
    SESSION_IMU_TOPIC,
    imu_offset,
    main,
    map_directory,
    mapping_input,
    mount_heights,
    origin_offset,
)


class Transport:
    """A transport that is not a ROS topic"""


class Feedback:
    def __init__(self, transport):
        self.transport = transport


class Plugin:
    feedbacks = {
        "lidar": Feedback(RosTopicTransport("lidar", topic_name="/livox/lidar", msg_type="PointCloud2")),
        "odom": Feedback(Transport()),
    }
    mounts = []


def test_a_feedback_on_ros_is_read_where_it_already_is():
    assert mapping_input(Plugin(), "lidar", "/unused") == ("/livox/lidar", None)
    assert mapping_input(Plugin(), None, "/unused") == (None, None)


def test_a_feedback_the_plugin_decodes_is_published_for_the_backend():
    """GLIM is a plain ROS node, so a sensor that only lives on the plugin's
    feedback bus -- the robot's own IMU -- has to be put on a topic."""
    plugin = Plugin()

    topic, feedback = mapping_input(plugin, "odom", SESSION_IMU_TOPIC)

    assert topic == SESSION_IMU_TOPIC
    assert feedback is plugin.feedbacks["odom"]


def test_a_key_naming_no_feedback_is_refused():
    with pytest.raises(ValueError, match="no feedback 'imu'"):
        mapping_input(Plugin(), "imu", "/unused")


def test_mount_heights_come_from_the_plugins_string_framed_mounts():
    plugin = Plugin()
    plugin.mounts = [Mount(parent=plugin, child="livox_frame", xyz=(0.2, 0.0, 0.15)), Mount(parent=plugin, child=object())]
    assert mount_heights(plugin) == {"livox_frame": 0.15}
    plugin.mounts = []
    assert mount_heights(plugin) == {}


def test_map_directory_is_the_name_with_a_timestamp():
    path = map_directory("/maps", "warehouse")
    assert os.path.dirname(path) == "/maps"
    assert re.fullmatch(r"warehouse-\d{8}-\d{6}", os.path.basename(path))


def test_a_plugin_that_cannot_be_loaded_is_refused(capsys):
    for entry in ("nocolon", "no_such_plugin_module:Plugin", "os.path:NoSuchClass"):
        assert main(["--plugin", entry, "--name", "x"]) == 2
        assert f"could not load plugin '{entry}'" in capsys.readouterr().err


def test_a_plugin_without_native_mapping_is_refused(capsys):
    sys.modules.setdefault("fake_plugin_module", type(sys)("fake_plugin_module"))
    sys.modules["fake_plugin_module"].Plain = Plugin
    assert main(["--plugin", "fake_plugin_module:Plain", "--name", "x"]) == 2
    assert "does not declare native mapping" in capsys.readouterr().err


def test_the_imu_offset_comes_from_the_declaration():
    declared = NativeMapping(cloud="lidar", imu_xyz=(0.011, 0.023, -0.044))
    assert imu_offset(declared) == ((0.011, 0.023, -0.044), (0.0, 0.0, 0.0))
    declared = NativeMapping(cloud="lidar", imu_xyz=(0.0, 0.0, 0.1), imu_rpy=(0.0, 0.0, 1.5))
    assert imu_offset(declared) == ((0.0, 0.0, 0.1), (0.0, 0.0, 1.5))
    assert imu_offset(NativeMapping(cloud="lidar")) is None  # GLIM's default applies


def test_glims_origin_is_at_the_imu_when_it_maps_with_one():
    body_imu = ((-0.12815, 0.0, -0.10596), (0.0, 0.0, 0.0))
    assert origin_offset("/emos_mapping/imu", body_imu) == pytest.approx(-0.10596)
    assert origin_offset(None, body_imu) == 0.0  # LiDAR-only: the LiDAR is the origin
    assert origin_offset("/livox/imu", None) == 0.0  # GLIM's default: no offset


def test_an_environment_emos_has_no_settings_for_is_refused(capsys):
    with pytest.raises(SystemExit) as exited:
        main(["--plugin", "p:P", "--name", "x", "--env", "underwater"])
    assert exited.value.code == 2
    assert "invalid choice: 'underwater'" in capsys.readouterr().err
