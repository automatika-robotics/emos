# Troubleshooting

Common issues when running EMOS recipes, organized by error message.

---

## "Sensor topic not found within 10s"

The most common error. `emos run` checks that every sensor topic declared in the recipe is actually publishing before launching.

**Possible causes:**

| Cause                                                 | Fix                                                                                                                                            |
| :---------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------- |
| Driver not installed                                  | Run `emos info <recipe>` for suggested packages, then `sudo apt install <package>`                                                             |
| Driver installed but not running                      | Start the driver node in a separate terminal before running the recipe                                                                         |
| Topic name mismatch                                   | Compare `ros2 topic list` output with `Topic(name=...)` in your recipe and correct the name                                                    |
| **Container mode**: driver not installed in container | Install the driver inside the container: `docker exec -it emos bash -c "apt-get update && apt-get install -y ros-jazzy-usb-cam"` and launch it |

**Diagnosis:**

```bash
# Check what topics exist
ros2 topic list

# Check if a specific topic has data
ros2 topic hz /image_raw
```

If the topic exists but shows 0 Hz, the driver is running but not producing data (check hardware connections).

To temporarily bypass this check while debugging, use `emos run <recipe> --skip-sensor-check`.

---

## "ImportError: No module named 'agents'"

EMOS Python packages are not on the Python path. The fix depends on your install mode:

| Mode          | Fix                                                                                                                             |
| :------------ | :------------------------------------------------------------------------------------------------------------------------------ |
| **Container** | Run the recipe via `emos run`, not directly with `python3`. The CLI executes inside the container where packages are installed. |
| **Native**    | Source the ROS2 environment first: `source /opt/ros/jazzy/setup.bash`                                                           |
| **Pixi**      | Enter the pixi shell first: `pixi shell`, then `source install/setup.sh`                                                        |

---

## "Robot plugin not found"

A recipe uses `Launcher(robot_plugin="some_plugin")` but the plugin package is not installed.

**Fix:**

1. Check if the plugin is available: `ros2 pkg list | grep some_plugin`
2. If missing, build and install the plugin package into your ROS2 workspace:
   ```bash
   cd ~/ros2_ws/src
   git clone <plugin_repo_url>
   cd ~/ros2_ws && colcon build
   source install/setup.bash
   ```

```{seealso}
See [Robot Plugins](../concepts/robot-plugins.md) for how plugins work and how to create one.
```

---

## "Zenoh router failed to start"

The Zenoh router (used by `rmw_zenoh_cpp`) typically fails when port 7447 is already in use from a previous run.

**Fix:**

```bash
# Kill any existing Zenoh routers
pkill -f rmw_zenohd

# Then retry
emos run <recipe>
```

To use a different RMW and skip Zenoh entirely:

```bash
emos run <recipe> --rmw rmw_cyclonedds_cpp
```

---

## "Recipe hangs with no output"

The recipe started but nothing happens. Two common causes:

**1. Sensor topic exists but has no data (0 Hz)**

The driver node is running but the hardware isn't producing data. Check:

```bash
ros2 topic hz /image_raw    # Should show non-zero rate
```

If 0 Hz: check physical connections (USB cable, power), device permissions (`sudo chmod 666 /dev/video0`), or driver configuration.

**2. Model inference failing**

EMOS verifies that the model server is reachable when the recipe starts. If the recipe hangs after that, the server is running but inference itself is failing — for example, the model isn't loaded, is out of memory, or the request format is wrong.

Check your model server's logs for errors. Common causes:

- **Model not pulled/loaded** — e.g. for Ollama, run `ollama pull qwen2.5vl:latest` before starting the recipe
- **Out of memory** — the model is too large for available RAM/VRAM. Try a smaller checkpoint.
- **Timeout on cloud endpoint** — network latency or rate limiting. Check the provider's status page.

---

## "Container 'emos' not found"

The Docker container was removed or never created.

**Fix:**

```bash
emos install --mode container
```

This recreates the container. Your recipes in `~/emos/recipes/` are preserved (they're mounted from the host).

---

## "No EMOS installation found"

The config file at `~/.config/emos/config.json` is missing.

**Fix:** Run `emos install` to set up EMOS. If you're using the pixi install mode, run `pixi run setup` from the EMOS repo root to register the installation.

---

## Dashboard

The dashboard daemon (`emos serve`) and its [web UI](dashboard.md) have a few common failure modes worth knowing about.

### "address already in use" when starting `emos serve`

Another process is bound to the dashboard port (default `8765`). Most often this is a previous `emos serve` that's still running, either in another terminal or as a systemd service.

```bash
# Is it already running as a service?
systemctl status emos-dashboard.service

# Stop the service (or the foreground process), then retry
sudo systemctl stop emos-dashboard.service
emos serve

# Or pick a different port for one run
emos serve --addr :9000

# Or change the persisted port
emos config set port 9000
```

### I lost the pairing code

The pairing code is **only printed on first boot** and never persisted in plaintext on disk. Rotate to issue a fresh one:

```bash
emos config rotate-pairing
# ✓ New pairing code (shown once): 829471
```

Already-paired browsers keep working — rotation is a code rotation, not a token revocation. Use `emos config tokens` to see who is paired and `emos config revoke-token <id|label>` to remove a specific browser.

### `emos.local` does not resolve

mDNS (`*.local`) works on most laptops out of the box but is unreliable on phones (especially Android). The dashboard is also published as `<device-name>.local` — and the boot banner always prints LAN IPs you can use directly.

**Fixes, in order of preference:**

```bash
# 1. Use the device-specific name printed in the banner
ping epic-otter.local

# 2. Use a LAN IP from the banner
http://192.168.1.42:8765/

# 3. Confirm avahi/Bonjour is running on the laptop
#    (Linux: avahi-daemon, macOS: built-in, Windows: Bonjour from the iTunes installer)

# 4. Force a fresh mDNS publication after a hostname change
emos config set name new-name
sudo systemctl restart emos-dashboard.service
```

For headless or factory networks, just use the IP address; the dashboard works identically over IP and mDNS.

### Browser shows a persistent "Not Secure" warning under `--tls`

`emos serve --tls` mints a self-signed cert. Browsers always flag self-signed certs as Not Secure even after you click through the warning — the encryption is real, but the trust chain is not. To get rid of the lozenge:

- **Firefox:** *Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import* `~/.config/emos/tls.crt`, tick "Trust this CA to identify websites." Firefox keeps its own trust store separate from the OS.
- **Chrome / system-wide:** install the cert into the OS trust store (`update-ca-certificates` on Linux, Keychain on macOS, certmgr on Windows).

Verify the cert before trusting it:

```bash
emos config tls-fingerprint
```

Compare the SHA-256 fingerprint with the one the browser shows under "View Certificate".

```{seealso}
[CLI → HTTPS (Optional)](cli.md#https-optional) for the full rationale and trust-store steps.
```

### After moving to a new network, HTTPS shows a hostname-mismatch error

The TLS cert's SubjectAltNames are baked in at mint time. If the device's IP or `.local` name changed, the cert no longer covers the address you're using. Regenerate:

```bash
emos config tls-regenerate
sudo systemctl restart emos-dashboard.service
```

### "TLS handshake error from ..." floods the log

Pre-v0.6.2 — these were stdlib `log` lines bypassing slog. The current daemon routes them to slog at DEBUG (invisible at the default INFO level). If you still see them: probably a probe from a Tailscale agent, nmap, or a browser hitting the port over plain HTTP while the daemon is in `--tls` mode. Run `emos serve -v` to see the chatter when debugging.

### "503 service_unavailable, code: offline" on the recipe catalog

The dashboard tried to reach the Automatika recipe server and the device's reachability probe failed. This is by design — already-installed recipes still run while offline. If you expected the device to be online:

```bash
# Force a fresh probe (bypasses the 30 s cache)
curl http://emos.local:8765/api/v1/connectivity?refresh=1

# DNS, default route, firewall — the usual suspects
ping support.automatikarobotics.com
```

### Dashboard shows "Not installed" but the CLI works

The dashboard reads `~/.config/emos/config.json`; the CLI also falls back to migrating from a legacy `~/.config/emos/license.key` if it finds one. If `emos install` was interrupted before writing the config, the CLI sees the legacy file and migrates on the fly, but the dashboard might race the migration. Re-run `emos install` (or `emos config show`) to materialise a clean config, then refresh the browser.
