package cmd

import (
	"os"

	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// warnIfSudo emits a recommendation NOT to run the calling command under
// sudo when the binary is launched with EUID 0 and `SUDO_USER` is set.
//
// EMOS persistent state lives under the calling user's HOME:
//
//   - `~/.config/emos/config.json`  (mode, auth, pairing hash, tokens)
//   - `~/emos/recipes/`              (installed recipes)
//   - `~/emos/logs/`                 (run logs)
//
// Commands that need root for *some* steps -- writing to
// `/etc/systemd/system/`, calling `systemctl`, etc. -- escalate
// internally via the `sudo` binary on demand.
func warnIfSudo() {
	if os.Geteuid() != 0 {
		return
	}
	user := os.Getenv("SUDO_USER")
	if user == "" {
		return
	}
	ui.Warn("Running under sudo (SUDO_USER=" + user + "). EMOS state for user '" +
		user + "' lives in /home/" + user + "/, but under sudo this CLI writes to /root/'s HOME instead.")
	ui.Faint("The dashboard daemon (running as " + user +
		") won't see those changes. Re-run without sudo -- the command escalates internally where it needs to.")
}
