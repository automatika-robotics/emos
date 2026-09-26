import os
import re
import sys

import pytest

pytest.importorskip("ros_sugar")
from ros_sugar.robot import Mount, RosTopicTransport  # noqa: E402
from ros_sugar.robot.mapping import NativeMapping  # noqa: E402

from emos_mapping.session import (  # noqa: E402
    feedback_topic,
    imu_offset,
    main,
    map_directory,
    mount_heights,
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


def test_feedback_keys_resolve_to_their_ros_topics():
    assert feedback_topic(Plugin(), "lidar") == "/livox/lidar"
    assert feedback_topic(Plugin(), None) is None
    with pytest.raises(ValueError, match="no feedback 'imu'"):
        feedback_topic(Plugin(), "imu")
    with pytest.raises(TypeError, match="not a ROS topic"):
        feedback_topic(Plugin(), "odom")


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
