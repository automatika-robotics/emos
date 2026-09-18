import json
import os

from emos_mapping.backend import (
    GLIM_NODE,
    MAP_TOPIC,
    glim_node,
    strip_json_comments,
    write_glim_config,
)


def load(path):
    """GLIM's files carry comments; the ones the session rewrites do not"""
    with open(path) as f:
        return json.loads(strip_json_comments(f.read()))


def test_comments_are_stripped_but_strings_are_kept():
    text = '{\n  // a comment\n  "url": "http://x/y", /* block\n comment */ "n": 1 // trailing\n}'
    assert json.loads(strip_json_comments(text)) == {"url": "http://x/y", "n": 1}


def test_session_config_names_the_topics_and_uses_the_cpu_modules(tmp_path):
    directory = write_glim_config(str(tmp_path / "config"), "/livox/lidar", "/livox/imu")
    assert directory == str(tmp_path / "config")
    ros = load(os.path.join(directory, "config_ros.json"))["glim_ros"]
    assert ros["points_topic"] == "/livox/lidar" and ros["imu_topic"] == "/livox/imu"
    # Only the module publishing the map; GLIM's own viewer needs a display.
    assert ros["extension_modules"] == ["librviz_viewer.so"]
    cfg = load(os.path.join(directory, "config.json"))["global"]
    assert cfg["config_odometry"] == "config_odometry_cpu.json"
    assert cfg["config_sub_mapping"] == "config_sub_mapping_cpu.json"
    assert cfg["config_global_mapping"] == "config_global_mapping_cpu.json"
    # Every file the config names is there.
    for name in cfg.values():
        if name.endswith(".json"):
            assert os.path.isfile(os.path.join(directory, name)), name


def test_gpu_and_imu_free_variants(tmp_path):
    cfg = load(os.path.join(write_glim_config(str(tmp_path / "gpu"), "/p", "/i", gpu=True), "config.json"))["global"]
    assert cfg["config_odometry"] == "config_odometry_gpu.json"
    assert cfg["config_global_mapping"] == "config_global_mapping_gpu.json"
    cfg = load(os.path.join(write_glim_config(str(tmp_path / "noimu"), "/p", None), "config.json"))["global"]
    assert cfg["config_odometry"] == "config_odometry_ct.json"


def test_glim_is_launched_with_the_session_paths():
    node = glim_node("/maps/a/glim/config", "/maps/a/glim/dump")
    assert (node["package"], node["executable"], node["name"]) == ("glim_ros", "glim_rosnode", GLIM_NODE)
    assert node["parameters"] == [{"config_path": "/maps/a/glim/config", "dump_path": "/maps/a/glim/dump"}]
    assert MAP_TOPIC == f"/{GLIM_NODE}/map"


def test_the_lidar_imu_offset_is_written_when_declared(tmp_path):
    directory = write_glim_config(str(tmp_path / "c"), "/p", "/i", lidar_imu=((0.011, 0.023, -0.044), (0.0, 0.0, 0.0)))
    sensors = load(os.path.join(directory, "config_sensors.json"))["sensors"]
    assert sensors["T_lidar_imu"] == [0.011, 0.023, -0.044, 0.0, 0.0, 0.0, 1.0]
    # Not declared: GLIM's own value stays
    directory = write_glim_config(str(tmp_path / "d"), "/p", "/i")
    assert load(os.path.join(directory, "config_sensors.json"))["sensors"]["T_lidar_imu"][:3] == [0.006, -0.012, 0.008]
