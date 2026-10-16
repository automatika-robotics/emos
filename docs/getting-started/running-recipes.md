# Running Recipes

One of the things that makes EMOS pleasant to work with is that a recipe is just a Python script. A complete robot behaviour, the components, the models and the topics that wire them together, fits in one file that you can run with `python recipe.py`. There is no build step, no launch file, no configuration scattered across a package. And the same file runs unchanged on a deployed robot through `emos run`, which adds the environment setup, a log of every run and a card on the dashboard.

So once you have a recipe, whether you wrote it yourself or pulled it from the catalog, there are two ways to run it. One is for you while you are still shaping it. The other is for whoever runs the finished recipe on a robot, which may well be your future self.

## Development vs production

While you are developing a recipe you want the shortest possible loop between an edit and seeing its effect, so you run the script directly. When the recipe is done and someone just wants to launch it, `emos run` and the dashboard take over: they prepare the environment for the install mode, keep the logs, and let an operator start and stop the recipe from a browser without ever opening a terminal.

|                          | Development                                          | Production                                                                                 |
| :----------------------- | :--------------------------------------------------- | :----------------------------------------------------------------------------------------- |
| **Who it's for**         | The recipe author iterating on the script.           | An operator, often a non-developer, running a finished recipe on a robot.                  |
| **How to launch**        | `python recipe.py`                                   | `emos run <name>` from a terminal, or **Run** on the recipe card in the dashboard.          |
| **Environment setup**    | You source ROS or activate the EMOS environment.     | The CLI activates the right environment for your install mode, including installed plugins. |
| **Logs**                 | Stream to the terminal. Nothing is kept.             | Stream to the terminal and are kept in `~/emos/logs/<name>_<timestamp>.log`.               |
| **Dashboard visibility** | None.                                                | A card on the **Recipes → Installed** tab, ready for an operator to launch from a browser. |

In short, run the script with `python` while you are shaping it, then drop it into `~/emos/recipes/` once it is solid and let `emos run` or the dashboard take it from there.

## Promoting a recipe to production

Moving a recipe to production is a matter of putting it in the right place. There is no registration step: the CLI and the dashboard both look at `~/emos/recipes/`, and anything there is picked up automatically.

### 1. Move the recipe into `~/emos/recipes/`

```bash
mkdir -p ~/emos/recipes/my_recipe
cp recipe.py ~/emos/recipes/my_recipe/recipe.py
```

The directory name is what you pass to `emos run`, and the file inside has to be called `recipe.py`. The recipe runs with that directory as its working directory, so it can refer to files next to it by relative path.

### 2. Add a `manifest.json` if you want

```json
{
  "name": "My Recipe",
  "description": "Does the thing.",
  "tags": ["vision", "demo"],
  "zenoh_router_config_file": "my_recipe/zenoh_config.json5"
}
```

| Field                      | Purpose                                                                                                        |
| :------------------------- | :------------------------------------------------------------------------------------------------------------- |
| `name`                     | Display name on the dashboard's recipe card. Without it, the directory name is used.                            |
| `description`              | A short blurb on the recipe's detail page.                                                                      |
| `tags`                     | Shown as small labels on the card.                                                                              |
| `zenoh_router_config_file` | Path, relative to `~/emos/recipes/`, to a Zenoh router config in `.json5` form. Only used with `--rmw rmw_zenoh_cpp`. |

The manifest is optional; without one the recipe still runs.

```{note}
The manifest does not list the recipe's sensors. Those are read from the `Topic(...)` declarations in `recipe.py`, which is what `emos info` and the dashboard's detail page show.
```

### 3. Check that EMOS sees it

```bash
emos ls                  # the recipe should be listed
emos info my_recipe      # its topics, and what provides each one
```

### 4. Launch it

```bash
emos run my_recipe
```

Open the dashboard and the recipe is there on the **Recipes → Installed** tab. From this point an operator can click **Run** and follow the live log in the browser, while the terminal route stays available: `emos run my_recipe` does the same thing and writes the same log file, just from a shell. Whichever way it was started, Ctrl+C in the terminal, or **Stop** in the dashboard, ends it cleanly; a second Ctrl+C kills it.

`emos run` leaves the choice of RMW to the environment. If a recipe needs a particular one, pass `--rmw rmw_fastrtps_cpp` or `--rmw rmw_zenoh_cpp`; with Zenoh, the CLI also starts a router for the run, or reuses one that is already running.

## Install-mode reference

The production flow works the same in every mode, because the CLI knows how to prepare each environment. Running the script yourself is where the modes differ.

| Mode          | Development (`python recipe.py`)                                                                                                                                   | Production (`emos run <name>`)                                                 |
| :------------ | :----------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----------------------------------------------------------------------------- |
| **pixi**      | `pixi shell --manifest-path ~/.local/share/emos/pixi.toml`, then `source ~/.local/share/emos/install/setup.sh`, then `python3 recipe.py`.                          | Works. The CLI activates the pixi environment and sources the EMOS packages.   |
| **native**    | Works after `source /opt/ros/<distro>/setup.bash`.                                                                                                                  | Works.                                                                         |
| **container** | Does not work from the host: the `agents`, `kompass` and `ros_sugar` packages live inside the container, and the container only runs during a recipe run.           | Works. The CLI starts the container and runs the recipe inside it.             |

If you have plugins installed, `emos run` sources their overlay after the stack, so a recipe can import them. When you run a script yourself, source `~/emos/workspace/install/setup.bash` (or `setup.sh` in the pixi shell) as well.

## Where logs go

Every `emos run` writes to `~/emos/logs/<recipe>_<YYYYMMDD_HHMMSS>.log`, and the same output streams to your terminal, or to the dashboard's run console when the run was started from there. A plain `python recipe.py` writes nothing under `~/emos/logs/`; redirect the output yourself if you want to keep it.

```{seealso}
- [Dashboard](dashboard.md) for pairing, the recipes tab and the run console.
- [EMOS CLI](cli.md) for the full command reference, including the `emos run` flags.
- [Troubleshooting](troubleshooting.md) for missing topics and mode-specific problems.
```
