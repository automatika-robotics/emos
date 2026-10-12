# Mapping

Navigation needs a map of the place the robot works in, and there are two ways to get one. If the site has already been surveyed, with a scanner or another robot, you can bring that map along and hand it to your recipes as it is. Otherwise you map the site with the robot itself, and that is where EMOS helps: `emos map` drives the whole session and keeps the result. How a particular robot maps is declared in its plugin. Some robots ship with mapping software of their own, and EMOS drives that software; robots that ship none but have a LiDAR are mapped by EMOS itself, with a mapping backend it builds on the robot. Either way the commands are the same, and either way you need a robot plugin installed first.

```{note}
A map is a picture of the site's permanent structure: walls, shelving, fixed equipment. Map when only those are present, ideally when the site is quiet. Anything that moves or changes, people, carts, a pallet left in an aisle, is handled at run time by the [Local Mapper](../navigation/mapping.md#local-mapper), and does not belong in the map.
```

## Set up the mapping backend

This step only applies to robots that EMOS maps itself, and `emos map setup` tells you if your robot does not need it.

```bash
emos map setup
```

The backend, GLIM and the libraries under it, is compiled from source into the EMOS workspace, which takes a while. On a machine with CUDA you are offered a GPU build. If a later update breaks the backend, `emos map setup --rebuild` builds it again, and an interrupted build continues from where it stopped when you run the command again.

The backend is built automatically for pixi installs only. On a native install the command prints what to build and where EMOS keeps its build recipe, and a container install cannot build maps with EMOS itself at all.

## Build a map

```bash
emos map new office
```

Mapping is interactive, so run it from a terminal. When EMOS builds the map itself, the first thing it asks is whether the site is indoors or outdoors, because the backend is tuned differently for rooms and corridors than for open ground, and if you would rather not be asked, `--env indoor` or `--env outdoor` answers in advance. Once the session is up, keep the robot still until the CLI says that mapping is ready. The backend spends those first seconds working out which way is down, and a robot that moves before it has finished ends up with a tilted map. After that, take the robot around the area you want mapped with its own controller.

```{tip}
Three things make a good map. Move slowly and turn gently, since fast turns blur the scans. Close loops: come back to places you have already scanned rather than driving one long line, or the map will not line up with itself. And finish in an area you have already covered.
```

Press Enter when you are done. The CLI stops the session, saves the map and reports where it is:

```text
✓ Map 'office-20260925-101530' saved.
  ~/emos/maps/office-20260925-101530/occ_grid.yaml
  Make it active with 'emos map use office-20260925-101530'.
```

A map that EMOS built itself carries a timestamp in its name. A robot that maps with its own software names the map the way that software does, and may still be processing it when the command returns; `emos map list` shows when it is ready.

Everything the session printed is kept in `~/emos/logs/map-<name>_<timestamp>.log`. The `--rmw` flag works the same as for `emos run`.

### Recording a session for debugging

Sometimes a map comes out wrong and nothing in the log explains why. The most useful thing you can do then is map the place again with the session recorded, so that whoever looks into it can replay exactly what the backend saw instead of guessing from the result.

```bash
emos map new office --debug
```

A debug session builds the map as usual and, alongside it, records the raw LiDAR and IMU data as it arrives. Such a recording grows quickly, which is why a debug session is best kept short, with the robot standing still until mapping is ready and then driven once, slowly, around a loop that brings it back to where it started. When the session ends, the recording and its log are kept together in a `debug` directory inside the map, and the whole map can be handed over in one piece:

```bash
emos map export --debug office-20260925-101530
```

This packs the map, its recording and log and the backend's own files into one archive that you can attach to a report. An archive made this way is meant for debugging only, and `emos map import` will not take it. The plain export is what moves a map from one robot to another.

## Manage maps

```bash
emos map list                          # the maps on this robot, the active one marked
emos map use office-20260925-101530    # make it the map the robot localizes against
emos map export                        # package the active map into ~/emos/map-archives
emos map import office.zip             # unpack an archive into the store
emos map rm old-office                 # delete a map
```

`use` is what makes a map count: recipes load the active map when they start. On a robot with its own mapping software, switching maps takes effect immediately and the robot needs to relocalize, so the CLI asks before doing it. The active map cannot be removed; switch to another one first.

`export` produces an archive you can copy to another robot or keep as a backup, and `import` takes such an archive, by path or by name when it is in `~/emos/map-archives`.

## Where maps live

Maps that EMOS builds are stored under `~/emos/maps`, one directory per map, and a link named `active` points at the map in use. Each map directory holds the occupancy grid as `occ_grid.yaml` and `occ_grid.pgm`, the full point cloud as `full_cloud.pcd`, a `preview.png`, and a `map.json` with the map's metadata. Robots with their own mapping software keep their maps where that software puts them, and `emos map list` shows that location.

`emos uninstall` leaves the maps alone.

## Bring your own map

A map made with other equipment does not have to go through `emos map` at all. Kompass's Map Server reads an occupancy grid in the usual `.yaml` and `.pgm` form, and also a 3D point cloud in `.pcd` form, which it flattens into a grid itself. Point a recipe at the file and the map is in use:

```python
from kompass.components import MapServer, MapServerConfig

map_server = MapServer(
    component_name="map_server",
    config=MapServerConfig(map_file_path="/home/robot/site/office.pcd"),
)
```

If you would rather have `emos map` manage it alongside the maps the robot builds, so that `use` and `export` work on it too, package it like an exported map, a zip with one directory holding `occ_grid.yaml` and `occ_grid.pgm`, and `emos map import` it.

## Use the map in a recipe

The occupancy grid of the active map is what Kompass's Map Server serves to the planner. The plugin knows where the active map is, so a recipe does not need a path:

```python
from kompass.components import MapServer, MapServerConfig

map_server = MapServer(
    component_name="map_server",
    config=MapServerConfig(map_file_path=robot.MAPPING.active_grid_path()),
)
```

See [Mapping & Localization](../navigation/mapping.md) for the Map Server and the local mapper.

## When something goes wrong

The messages `emos map` prints when there is no robot plugin, when the plugin declares no mapping, when the backend is missing, or when a session ends without a map are explained in [Troubleshooting](troubleshooting.md#mapping).
