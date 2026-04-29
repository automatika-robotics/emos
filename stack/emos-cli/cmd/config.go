package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/server"
	"github.com/automatika-robotics/emos-cli/internal/tlsca"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// configCmd groups the helpers that read / write ~/.config/emos/config.json
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect or modify EMOS device configuration",
}

// --- inspection ---------------------------------------------------------

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Pretty-print the device configuration and computed runtime info",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadConfig()
		ui.Header("EMOS DEVICE STATE")

		if !cfg.IsInstalled() {
			ui.Warn("No EMOS installation found.")
			ui.Faint("Run 'emos install' to create one.")
			if cfg != nil && cfg.Name != "" {
				ui.Faint("(A pre-install stub exists at " + config.ConfigFile + " with device name '" + cfg.Name + "'.)")
			}
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Identity:\t%s\n", display(cfg.Name))
		fmt.Fprintf(w, "  Mode:\t%s\n", display(string(cfg.Mode)))
		fmt.Fprintf(w, "  ROS distro:\t%s\n", display(cfg.ROSDistro))
		if cfg.LicenseKey != "" {
			fmt.Fprintf(w, "  License:\t%s\n", redact(cfg.LicenseKey))
		}
		port := cfg.Port
		if port == 0 {
			port = config.DefaultDashboardPort
		}
		fmt.Fprintf(w, "  Dashboard port:\t%d\n", port)
		if cfg.PixiProjectDir != "" {
			fmt.Fprintf(w, "  Pixi project:\t%s\n", cfg.PixiProjectDir)
		}
		if cfg.WorkspacePath != "" {
			fmt.Fprintf(w, "  Workspace:\t%s\n", cfg.WorkspacePath)
		}
		fmt.Fprintf(w, "  Recipes:\t%s\n", config.RecipesDir)
		fmt.Fprintf(w, "  Logs:\t%s\n", config.LogsDir)
		fmt.Fprintf(w, "  Config file:\t%s\n", config.ConfigFile)

		paired := cfg.PairedDeviceCount()
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "  Pairing configured:\t%s\n", yesNo(cfg.Auth.PairingCodeHash != ""))
		fmt.Fprintf(w, "  Active tokens:\t%d\n", paired)

		dashUnit := config.DashboardServiceName
		fmt.Fprintf(w, "  Dashboard service:\t%s\n",
			activeStatus(installer.IsActive(dashUnit), dashUnit))

		w.Flush()

		if cfg.Auth.PairingCodeHash != "" && paired == 0 {
			fmt.Println()
			ui.Warn("Pairing is configured but no devices are paired.")
			ui.Faint("Run `emos config rotate-pairing` to issue a fresh pairing code.")
		}
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path to the EMOS config file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.ConfigFile)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Print a single value, or the whole config as JSON if no key",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadConfig()
		if cfg == nil {
			ui.Warn("No EMOS config found at " + config.ConfigFile)
			return
		}
		if len(args) == 0 {
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return
		}
		switch args[0] {
		case "name":
			fmt.Println(cfg.Name)
		case "mode":
			fmt.Println(cfg.Mode)
		case "ros_distro":
			fmt.Println(cfg.ROSDistro)
		case "port":
			port := cfg.Port
			if port == 0 {
				port = config.DefaultDashboardPort
			}
			fmt.Println(port)
		default:
			ui.Error("Unknown key: " + args[0])
			os.Exit(1)
		}
	},
}

// --- mutation -----------------------------------------------------------

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update a config value (writable keys: name, port)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key, value := args[0], args[1]
		switch key {
		case "name":
			if err := config.SetDeviceName(value); err != nil {
				ui.Error(err.Error())
				os.Exit(1)
			}
			ui.Success("Device name set to: " + value)
			ui.Faint("Restart `emos serve` for mDNS to re-publish under the new name.")
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil || port < 1 || port > 65535 {
				ui.Error("Port must be a number between 1 and 65535")
				os.Exit(1)
			}
			cfg := config.LoadConfig()
			if cfg == nil {
				cfg = &config.EMOSConfig{}
			}
			cfg.Port = port
			if err := config.SaveConfig(cfg); err != nil {
				ui.Error(err.Error())
				os.Exit(1)
			}
			ui.Success(fmt.Sprintf("Dashboard port set to: %d", port))
			ui.Faint("Restart `emos serve` for the new port to take effect.")
		default:
			ui.Error("Cannot set key: " + key + " (writable keys: name, port)")
			os.Exit(1)
		}
	},
}

// --- pairing & tokens ---------------------------------------------------

var configTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List paired browsers / agents (label, ID, expiry — no hashes)",
	Run: func(cmd *cobra.Command, args []string) {
		auth, err := server.NewAuthForCLI()
		if err != nil {
			ui.Error("Could not load auth state: " + err.Error())
			os.Exit(1)
		}
		toks := auth.ListTokens()
		if len(toks) == 0 {
			ui.Info("No paired devices.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tLABEL\tISSUED\tEXPIRES")
		for _, t := range toks {
			label := t.Label
			if label == "" {
				label = "(unlabelled)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				t.ID, label,
				t.IssuedAt.Format("2006-01-02 15:04"),
				t.ExpiresAt.Format("2006-01-02 15:04"),
			)
		}
		w.Flush()
		fmt.Println()
		ui.Faint("Revoke one with: emos config revoke-token <id-or-label>")
	},
}

var configRevokeTokenCmd = &cobra.Command{
	Use:   "revoke-token <id-or-label>",
	Short: "Revoke a single paired device by ID prefix or label",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		auth, err := server.NewAuthForCLI()
		if err != nil {
			ui.Error("Could not load auth state: " + err.Error())
			os.Exit(1)
		}
		n, err := auth.RevokeMatching(args[0])
		if err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
		if n == 0 {
			ui.Warn("No tokens matched: " + args[0])
			ui.Faint("Run 'emos config tokens' to see what's paired.")
			os.Exit(1)
		}
		ui.Success(fmt.Sprintf("Revoked %d token(s).", n))
		// If that was the last device, the operator now has a "configured
		// pairing, no paired device, no plaintext code" state — point them
		// at the only way out.
		if len(auth.ListTokens()) == 0 {
			ui.Faint("No paired devices remain. Run `emos config rotate-pairing` to issue a fresh code for the next device.")
		}
	},
}

var configRotatePairingCmd = &cobra.Command{
	Use:   "rotate-pairing",
	Short: "Generate a new pairing code (existing browsers stay paired)",
	Run: func(cmd *cobra.Command, args []string) {
		auth, err := server.NewAuthForCLI()
		if err != nil {
			ui.Error("Could not load auth state: " + err.Error())
			os.Exit(1)
		}
		code, err := auth.RegeneratePairingCode()
		if err != nil {
			ui.Error("Could not regenerate code: " + err.Error())
			os.Exit(1)
		}
		ui.Success("New pairing code (shown once): " + code)
		ui.Faint("Save it now — it is not stored in plaintext on disk.")
	},
}

// --- TLS ----------------------------------------------------------------

var configTLSFingerprintCmd = &cobra.Command{
	Use:   "tls-fingerprint",
	Short: "Print the dashboard's TLS certificate SHA-256 fingerprint",
	Long: `Prints the active TLS certificate's SHA-256 fingerprint, in the same format
browsers display under the "Not Secure" warning's "view certificate" dialog.
On first connect, compare the two values to verify there's no MITM before
clicking through the warning.`,
	Run: func(cmd *cobra.Command, args []string) {
		info, err := tlsca.Load()
		if err != nil {
			ui.Warn("No TLS certificate on disk yet.")
			ui.Faint("It is created the first time `emos serve` runs.")
			return
		}
		ui.Header("TLS CERTIFICATE")
		ui.Info("Fingerprint (SHA-256):")
		ui.Faint("  " + info.Fingerprint)
		ui.Info(fmt.Sprintf("Expires: %s", info.Leaf.NotAfter.Format("2006-01-02")))
		certPath, keyPath := tlsca.Paths()
		ui.Info("Certificate: " + certPath)
		ui.Info("Private key: " + keyPath)
	},
}

var configTLSRegenerateCmd = &cobra.Command{
	Use:   "tls-regenerate",
	Short: "Mint a fresh self-signed TLS certificate for the dashboard",
	Long: `Generates a new self-signed certificate covering the device's current
hostname and LAN IP addresses. Run this after the device moves to a new
network so the certificate's SANs match the addresses callers actually
use. Already-paired browsers will see the new "Not Secure" warning until
they re-trust the cert.`,
	Run: func(cmd *cobra.Command, args []string) {
		deviceName, err := config.ResolveDeviceName()
		if err != nil {
			ui.Warn("Could not resolve device name: " + err.Error())
		}
		info, err := tlsca.Generate(deviceName)
		if err != nil {
			ui.Error("Regenerate failed: " + err.Error())
			os.Exit(1)
		}
		ui.Success("Fresh TLS certificate minted.")
		ui.Info("New fingerprint:")
		ui.Faint("  " + info.Fingerprint)
		ui.Faint("Restart `emos serve` (or the systemd service) to use it.")
	},
}

// --- destructive --------------------------------------------------------

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset device state (pairing, tokens, name, port) — keeps install info",
	Long: `Resets the dashboard's device state: revokes every paired browser, clears
the saved pairing code, and drops any custom name or port the user has set.
The actual install on disk (container, native packages, pixi project) is unaffected.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !ui.Confirm("This revokes all paired browsers and resets device name/port. Installation is preserved. Continue?") {
			return
		}
		cfg := config.LoadConfig()
		if cfg == nil {
			ui.Info("Nothing to reset — no config on disk.")
			return
		}
		// Clear only device-state fields.
		cfg.Name = ""
		cfg.Port = 0
		cfg.Auth = config.AuthState{}
		if err := config.SaveConfig(cfg); err != nil {
			ui.Error("Could not write config: " + err.Error())
			os.Exit(1)
		}
		ui.Success("Device state reset.")
		ui.Faint("On the next 'emos serve', a fresh pairing code will be printed.")
	},
}

// --- helpers ------------------------------------------------------------

func display(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// redact keeps the last four characters visible so a user can confirm.
// Anything shorter is fully redacted.
func redact(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

func activeStatus(active bool, unit string) string {
	if active {
		return "active (" + unit + ")"
	}
	return "not running"
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configTokensCmd)
	configCmd.AddCommand(configRevokeTokenCmd)
	configCmd.AddCommand(configRotatePairingCmd)
	configCmd.AddCommand(configTLSFingerprintCmd)
	configCmd.AddCommand(configTLSRegenerateCmd)
	configCmd.AddCommand(configResetCmd)
	rootCmd.AddCommand(configCmd)
}
