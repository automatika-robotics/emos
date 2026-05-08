# Running Recipes

One of the things that makes EMOS satisfying to work with is that a recipe is **just a Python script**. A complete robot behavior -- the components, the ML models, and the topics that wire them together -- fits in a single file that you can run with `python recipe.py`. No build step, no launch-file XML, no per-component configuration sprawl. The same file then runs unchanged on a deployed robot via `emos run`, with pre-launch sensor checks, persistent logs, and a dashboard card layered on top automatically.

Once you have a recipe in hand -- whether you wrote it from a tutorial or pulled one with `emos pull` -- there are two distinct flows: one tuned for the developer who is still shaping the recipe, and one tuned for the end user (or your future self, on a deployed robot) who just wants to launch a finished recipe and watch it work.

## Development vs production

When you are **developing** a recipe -- writing it, tweaking parameters, debugging a bad output -- you want the tightest edit-run-debug loop possible. When somebody **runs** a finished recipe in production, they want sensor checks, persistent logs, and a dashboard they can manage from a browser without ever opening a terminal. EMOS gives you one flow for each.

|                          | Development                                              | Production                                                                                 |
| :----------------------- | :------------------------------------------------------- | :----------------------------------------------------------------------------------------- |
| **Who it's for**         | The recipe author iterating on the script.               | An operator -- often a non-developer -- running a finished recipe on a robot.              |
| **How to launch**        | `python recipe.py`                                       | `emos run <name>` from a terminal, or click **Run** on the recipe card in the dashboard.   |
| **Environment setup**    | You source ROS / activate the EMOS env yourself.         | The CLI activates the right environment for your install mode (Native / Pixi / Container). |
| **Pre-launch checks**    | None -- the script crashes if a sensor topic is missing. | Pre-flight check that every sensor topic the recipe declares is actually publishing.       |
| **Logs**                 | Stream to the terminal. No retention.                    | Stream to the terminal **and** persist to `~/emos/logs/<name>_<ts>.log`.                   |
| **Dashboard visibility** | None.                                                    | The recipe is on the **Recipes → Installed** tab and every run is tracked in run history.  |

In short: ship the script with `python` while you're shaping it; drop it into `~/emos/recipes/` once it's solid and hand it to an operator (or your future self) to run via `emos run` or the dashboard.

---

## Promoting a recipe to production

Moving a recipe from development to production -- so an operator can launch it via `emos run` or the dashboard -- is **drop-in**: there is no registration step, no plugin manifest to declare. The CLI and the dashboard both look at `~/emos/recipes/`. Anything you put there is automatically picked up.

### 1. Move the recipe into `~/emos/recipes/`

```bash
mkdir -p ~/emos/recipes/my_recipe
cp recipe.py ~/emos/recipes/my_recipe/recipe.py
```

The directory must be named the same as what you'll pass to `emos run`. The file inside **must** be called `recipe.py`.

### 2. (Optional) Add a `manifest.json`

```json
{
  "name": "My Recipe",
  "description": "Does the thing.",
  "zenoh_router_config_file": "my_recipe/zenoh_config.json5"
}
```

| Field                      | Purpose                                                                                                       |
| :------------------------- | :------------------------------------------------------------------------------------------------------------ |
| `name`                     | Display name shown on the dashboard's recipe card. Falls back to the directory name.                          |
| `description`              | Short blurb shown on the recipe detail page.                                                                  |
| `zenoh_router_config_file` | Path (relative to `~/emos/recipes/`) to a Zenoh router `.json5` config. Only consulted under `rmw_zenoh_cpp`. |

The manifest is entirely optional -- omit it and the recipe still runs.

```{note}
Sensor requirements are auto-extracted from `recipe.py` by parsing `Topic(name=..., msg_type=...)` declarations. You do not list them in the manifest.
```

### 3. Verify the CLI sees it

```bash
emos ls                  # the recipe directory should be listed
emos info my_recipe      # inferred sensor topics, suggested drivers
```

### 4. Launch

```bash
emos run my_recipe
```

Open the dashboard in a browser: the recipe is on the **Recipes → Installed** tab, and the active run appears in the run console with live log streaming. You can also start the run from the dashboard itself -- click the recipe card and **Run**.

---

## Install-mode reference

The two flows behave differently across install modes. The production flow is mode-aware out of the box; in development you handle the environment yourself.

| Mode                | Development (`python recipe.py`)                                                                                                                                                             | Production (`emos run <name>`)                                                  |
| :------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------ |
| **Native**          | Works after `source /opt/ros/<distro>/setup.bash`.                                                                                                                                           | Works.                                                                          |
| **Pixi**            | From the EMOS repo directory: `pixi shell`, then `source install/setup.sh`, then `python3 recipe.py`.                                                                                        | Works -- the CLI activates the pixi env and sources the colcon overlay for you. |
| **Container (OSS)** | **Does not work from the host shell.** The `agents`, `kompass`, and `ros_sugar` Python packages live inside the container. To iterate directly, `docker exec -it emos-container bash` first. | Works -- the CLI runs the recipe inside the container.                          |

```{important}
Only `emos run` populates the dashboard's run history. A recipe started via direct `python` runs locally without any communication to the dashboard daemon. If you want the run to appear in the dashboard, launch it via `emos run` (terminal) or by clicking **Run** on the recipe card.
```

---

## Where logs go

Every `emos run` invocation writes to `~/emos/logs/<recipe>_<YYYYMMDD_HHMMSS>.log`. The same log is streamed to your terminal during a foreground run, and to the dashboard's run console when started from there. Direct `python recipe.py` runs do not write to `~/emos/logs/` -- redirect manually if you want the file.

```{seealso}
- [Dashboard](dashboard.md) -- pairing, recipes tab, run console.
- [EMOS CLI](cli.md) -- the full command reference, including `emos run` flags (`--rmw`, `--skip-sensor-check`).
- [Troubleshooting](troubleshooting.md) -- sensor verification failures and mode-specific gotchas.
```
