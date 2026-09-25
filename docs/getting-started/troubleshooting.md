# Troubleshooting

The problems people run into most often, grouped by what you see on the screen.

```{tip}
Whatever the symptom, `emos status` is a good first stop. It checks that every EMOS package is present for your install mode and shows the state of the container or the pixi workspace, which rules out a broken install before you look further.
```

---

## Recipes

### The recipe waits for a topic that never arrives

A component that subscribes to a sensor topic does nothing until the first message comes in, so a recipe with a missing sensor looks like it is running but stays silent. Start with `emos info <recipe>`, which lists every topic the recipe uses and where it should come from.

| What `emos info` says              | What to check                                                                                                          |
| :--------------------------------- | :--------------------------------------------------------------------------------------------------------------------- |
| The topic comes from the robot plugin | Is a robot plugin installed? `emos plugin list` marks the installed ones. The plugin starts the robot's drivers itself. |
| The topic comes from a named plugin   | Is that sensor plugin installed, and does the recipe attach it under the same id?                                       |
| A plain ROS topic                     | Is the driver running, and does it publish under exactly that name? Compare `ros2 topic list` with the recipe's `Topic(name=...)`. |

Then confirm that data is actually flowing:

```bash
ros2 topic list
ros2 topic hz /image_raw
```

A topic that exists but shows no rate means the driver is up and the hardware is not delivering: check cables, power, and device permissions such as `sudo chmod 666 /dev/video0`.

If the driver runs in a different process than the recipe, both have to use the same RMW implementation. `emos run` does not set one unless you pass `--rmw`; if you do, export the same `RMW_IMPLEMENTATION` in the driver's shell. In container mode, run the driver on the host or in its own container, on the host network. The EMOS container is restarted for every run and stopped afterwards, so a driver started inside it does not survive.

### "ImportError: No module named 'agents'"

The EMOS packages are not on the Python path of the shell you are in. What to do depends on the install mode:

| Mode          | Fix                                                                                                                       |
| :------------ | :------------------------------------------------------------------------------------------------------------------------ |
| **pixi**      | `pixi shell --manifest-path ~/.local/share/emos/pixi.toml`, then `source ~/.local/share/emos/install/setup.sh`.            |
| **native**    | `source /opt/ros/<distro>/setup.bash`.                                                                                     |
| **container** | Run the recipe with `emos run`. The packages live inside the container, so `python3` on the host cannot find them.        |

### "ModuleNotFoundError: No module named 'myrobot_plugin'"

The recipe imports a plugin that is not installed, or that your shell cannot see. Install it from the catalog:

```bash
emos plugin list                       # find its name
emos plugin install <plugin>
emos plugin inspect                    # confirm it is there
```

`emos run` sources installed plugins automatically. When you run a script by hand, source the plugin overlay as well, `~/emos/workspace/install/setup.bash` (or `setup.sh` inside the pixi shell). For a plugin you wrote yourself, see [Plugins](plugins.md).

### "Plugins are being installed, updated or removed"

A plugin operation, started here or from the dashboard, is rebuilding the plugin overlay, and a recipe started now would load a half-built one. Wait for it to finish and try again. The same lock is behind "Another plugin install, update or removal is running" when you start a second plugin operation.

### "the zenoh router did not start listening on 127.0.0.1:7447"

Only relevant with `--rmw rmw_zenoh_cpp`. The CLI reuses a router that is already running on the machine, so this usually means a stale router holds the port without answering. Stop it and run again:

```bash
pkill -f rmw_zenohd
emos run <recipe> --rmw rmw_zenoh_cpp
```

Or leave `--rmw` out altogether and use the environment's default RMW.

### The recipe runs but nothing happens

The two usual causes are a sensor topic with no data, covered above, and a model server that is reachable but not answering. EMOS checks that the model server can be reached when a component starts; if the recipe stalls after that, look at the server's own log. Common reasons are a model that is not pulled or loaded (for Ollama, `ollama pull <model>` first), a model too large for the available memory, and a slow or rate-limited cloud endpoint.

### "container 'emos' does not exist — run 'emos install' first"

The container was removed. `emos install --mode container` recreates it; your recipes are on the host under `~/emos/recipes` and are untouched. Note that a container that shows as *Exited* in `emos status` is normal: the CLI starts it for each run and stops it afterwards.

### "no EMOS installation found — run 'emos install' first"

There is no install recorded in `~/.config/emos/config.json`. Run `emos install`.

---

## Installing and updating

### The board resets or powers off during a pixi install

A pixi install compiles kompass-core, and then the EMOS packages, on every core at once. On a board with a marginal power supply the sudden load can drop the voltage far enough to reset it, even when it is neither hot nor short of memory. Cap the number of compile jobs, for the install and for later updates:

```bash
EMOS_BUILD_JOBS=4 emos install --mode pixi
```

Lower the number further if the board still resets. The build takes longer, and nothing else changes.

### "The CUDA versions did not build, so the CPU versions will be installed"

The GPU build of sherpa-onnx and llama-cpp-python failed, most often because a download during the build timed out. EMOS keeps working on the CPU versions. Run `emos update` when the network is better and you will be offered the build again.

### "Your local pixi dependencies could not be reapplied -- this release changed pixi.toml"

You had added packages to the EMOS environment with `pixi add`, and the new release changed the same file. Your changes are saved in `git stash`. Resolve the conflict in `~/.local/share/emos/pixi.toml`, then run `emos update` again.

### "Please run 'emos update' again to update your installation"

Not an error. `emos update` replaced its own binary with the newer release and stopped there; the second run, on the new binary, updates the installation.

### "Running under sudo"

`emos install`, `emos uninstall`, `emos serve install-service` and `emos config` keep their state in your home directory and escalate on their own for the steps that need root. Under `sudo` they would write to root's home instead, and the dashboard running as your user would never see it. Run them as yourself.

---

## Mapping

### "No robot plugin is installed, so there is no robot to map with"

Mapping is driven by the robot plugin, which declares how the robot maps. Install the plugin for your robot first.

### "This robot's plugin declares no mapping support"

The plugin neither drives the robot's own mapping software nor tells EMOS which LiDAR to map with, so `emos map` has nothing to work with on this robot.

### "The mapping backend (GLIM) is not installed in this EMOS environment"

The robot maps with EMOS's own backend, which is built on demand. Run `emos map setup` once; it compiles GLIM and its dependencies into the EMOS workspace and can take a while.

### "EMOS builds this robot's maps itself, which a container install cannot do in this version"

Building maps with EMOS itself needs a pixi or native install on the robot.

### "The mapping backend published no map"

The first map arrives a few seconds after the robot starts moving. Drive for longer, and check that the LiDAR is publishing. A mapping session that ends without a map can leave an empty map directory behind; `emos map list` shows it and `emos map rm` removes it.

---

## Dashboard

### "address already in use" when starting `emos serve`

Something else is bound to the dashboard's port, and it is usually a previous `emos serve`, either in another terminal or as the systemd service:

```bash
systemctl status emos-dashboard.service      # is it the service?
sudo systemctl stop emos-dashboard.service   # stop it, then retry
emos serve --addr :9000                      # or use another port for this run
emos config set port 9000                    # or change the port for good
```

### I lost the pairing code

The code is printed once and never stored in readable form. Issue a new one:

```bash
emos config rotate-pairing
# ✓ New pairing code (shown once): 829471
```

Browsers that are already paired stay paired, and a running dashboard accepts the new code straight away. `emos config tokens` lists who is paired, and `emos config revoke-token <id|label>` removes one of them.

### `emos.local` does not resolve

mDNS names work on most laptops out of the box but are unreliable on phones, Android in particular. Try the robot's own name (`<name>.local`) first; failing that, use one of the IP addresses that `emos serve` prints, which work everywhere. On a laptop, make sure an mDNS resolver is running (avahi on Linux; macOS has one built in; Windows needs Bonjour). After renaming the robot, restart the dashboard so it announces the new name:

```bash
emos config set name new-name
sudo systemctl restart emos-dashboard.service
```

### The browser warns about the certificate

Expected on first contact: the robot signs its own certificate. Compare the fingerprint in the browser's certificate details with `emos config tls-fingerprint`, then continue. To make the warning go away for good, import `~/emos/.ui-security/tls.crt` into the browser's trust store, as described under [Security](dashboard.md#security).

If a browser that used to be fine suddenly reports a name or address mismatch, the robot changed network or name since the certificate was made. Regenerate it and restart the dashboard and any running recipe:

```bash
emos config tls-regenerate
sudo systemctl restart emos-dashboard.service
```

### "503 service_unavailable, code: offline" on the recipe catalog

The dashboard could not reach the Automatika catalog. Everything already installed keeps working; only browsing and pulling need the internet. To check again without waiting for the 30-second cache:

```bash
curl -k https://emos.local:8765/api/v1/connectivity?refresh=1
```

If the robot should be online, the usual suspects are DNS, the default route and a firewall.

### The dashboard says "Not installed" but the CLI works

The dashboard reads `~/.config/emos/config.json` once, when it starts. If it was started before `emos install` finished, it keeps reporting the old state until it is restarted:

```bash
sudo systemctl restart emos-dashboard.service   # as a service
# or stop the foreground `emos serve` with Ctrl-C and start it again
```

Then refresh the browser.
