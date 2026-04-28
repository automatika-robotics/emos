package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/server"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/automatika-robotics/emos-cli/internal/webui"
)

var (
	serveAddr        string
	serveDisableMDNS bool
	serveDisableAuth bool
	serveQRCodeOnly  bool
	serveVerbose     bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the EMOS dashboard (REST API + web UI)",
	Long: `Starts the EMOS onboarding dashboard on the local network.

The dashboard is reachable via mDNS (http://emos.local) or at the device's
IP address; the configured port is printed at startup. On first launch, a
six-digit pairing code is printed to the terminal — use it once in the
browser to issue a long-lived token.`,
	Run: runServe,
}

func init() {
	// Default `--addr` is empty so we can detect user did not pass it and
	// fall back to the persisted port from config (or DefaultDashboardPort).
	// Passing the flag explicitly always wins.
	serveCmd.Flags().StringVar(&serveAddr, "addr", "", "address to bind (host:port); defaults to the configured port")
	serveCmd.Flags().BoolVar(&serveDisableMDNS, "no-mdns", false, "skip mDNS announcement")
	serveCmd.Flags().BoolVar(&serveDisableAuth, "no-auth", false, "DEV ONLY: accept unauthenticated requests")
	serveCmd.Flags().BoolVar(&serveQRCodeOnly, "qr", false, "print a QR code with the dashboard URL and exit")
	serveCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "log every HTTP request (default: only mutations and errors)")
	rootCmd.AddCommand(serveCmd)
}

// resolveBindAddr returns the address to bind. Explicit --addr wins;
// otherwise we use whatever the user has stored via `emos config set port`,
// falling back to the package default.
func resolveBindAddr() string {
	if serveAddr != "" {
		return serveAddr
	}
	return fmt.Sprintf(":%d", config.DashboardPort())
}

func runServe(cmd *cobra.Command, args []string) {
	ui.Banner(config.Version)

	addr := resolveBindAddr()

	if serveQRCodeOnly {
		if u := qrURL(addr); u != "" {
			printQR(u + "/")
		}
		return
	}

	// If the dashboard is already running as a systemd service, don't try to
	// print a friendly summary
	if installer.IsActive(config.DashboardServiceName) {
		PrintDashboardAccessSummary(addr, "service", "")
		return
	}

	logLevel := slog.LevelInfo
	if serveVerbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	deviceName, err := config.ResolveDeviceName()
	if err != nil {
		ui.Warn("Could not persist device name: " + err.Error())
	}

	srv, err := server.New(server.Options{
		Addr:        addr,
		DeviceName:  deviceName,
		DisableMDNS: serveDisableMDNS,
		DisableAuth: serveDisableAuth,
		UI:          webui.FS(),
		Logger:      logger,
	})
	if err != nil {
		ui.Error("Failed to initialise dashboard: " + err.Error())
		os.Exit(1)
	}

	printBootBanner(addr, deviceName, srv.PairingCode())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		ui.Error("Dashboard server exited: " + err.Error())
		os.Exit(1)
	}
	ui.Info("Dashboard stopped.")
}

// dashboardURLs lists every URL the dashboard can plausibly be reached at,
// in priority order: per-device mDNS name first (uniquely identifies this
// robot), then localhost, then each LAN IP, then the shared `emos.local`
// shortcut as a fallback.
func dashboardURLs(addr, deviceName string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []string{"http://emos.local" + addr}
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		// Explicit bind — single URL.
		return []string{fmt.Sprintf("http://%s:%s", host, port)}
	}
	var urls []string
	if deviceName != "" {
		urls = append(urls, fmt.Sprintf("http://%s.local:%s", deviceName, port))
	}
	urls = append(urls, fmt.Sprintf("http://localhost:%s", port))
	for _, ip := range server.LocalIPv4Strings() {
		urls = append(urls, fmt.Sprintf("http://%s:%s", ip, port))
	}
	urls = append(urls, fmt.Sprintf("http://emos.local:%s", port))
	return urls
}

// PrintDashboardAccessSummary is the single human readable success block
// for the dashboard
func PrintDashboardAccessSummary(addr, origin, freshCode string) {
	deviceName, _ := config.ResolveDeviceName()
	urls := dashboardURLs(addr, deviceName)
	scanURL := qrURL(addr)

	ui.Header("EMOS DASHBOARD")
	switch origin {
	case "service":
		ui.Info("The dashboard is already running as a systemd service.")
	case "install":
		ui.Success("Dashboard service enabled and started.")
	}

	fmt.Println()
	if deviceName != "" {
		ui.Info("Robot identity: " + deviceName)
	}
	if len(urls) == 1 {
		ui.Info("Browser URL: " + urls[0])
	} else {
		ui.Info("Reach the dashboard from a browser:")
		for _, u := range urls {
			ui.Faint("  " + u)
		}
	}
	fmt.Println()

	if freshCode != "" {
		ui.Success("Pairing code (shown once): " + freshCode)
		ui.Faint("Scan the QR with a phone to auto-pair, or enter the code in a browser.")
		ui.Faint("Save it now — it is not stored in plaintext on disk.")
	} else {
		ui.Info("Pairing already configured for this device.")
		ui.Faint("Lost the code? Run `emos config rotate-pairing` to issue a fresh one.")
	}
	fmt.Println()

	if scanURL != "" {
		target := scanURL + "/"
		if freshCode != "" {
			target = scanURL + "/#/pair?code=" + freshCode
		}
		printQR(target)
	}

	fmt.Println()
	ui.Faint("Manage the service:")
	ui.Faint("  systemctl status " + config.DashboardServiceName)
	ui.Faint("  systemctl restart " + config.DashboardServiceName)
	ui.Faint("  journalctl -u " + config.DashboardServiceName + " -f")
}

// qrURL picks the best URL to encode in the QR code: a phone-reachable LAN
// IPv4 on this device. Tailscale is the fallback (only members of the tailnet
// can reach it from a phone). emos.local is a last resort because Android
// can't resolve mDNS.
func qrURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host != "" && host != "0.0.0.0" && host != "::" && host != "localhost" {
		return fmt.Sprintf("http://%s:%s", host, port)
	}
	if ip := server.PreferredLANIPv4(); ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}
	return fmt.Sprintf("http://emos.local:%s", port)
}

// the live foreground `emos serve` greeting
func printBootBanner(addr, deviceName, pairingCode string) {
	urls := dashboardURLs(addr, deviceName)
	scanURL := qrURL(addr)

	ui.Header("EMOS DASHBOARD")
	if deviceName != "" {
		ui.Info("Robot identity: " + deviceName)
	}
	if len(urls) == 1 {
		ui.Info("Browser URL: " + urls[0])
	} else {
		ui.Info("Reach the dashboard from a browser:")
		for _, u := range urls {
			ui.Faint("  " + u)
		}
	}
	fmt.Println()

	if pairingCode != "" {
		ui.Success("Pairing code (shown once): " + pairingCode)
		ui.Faint("Enter it in the browser, or scan the QR below with a phone to auto-pair.")
		ui.Faint("Save it now — it is not stored in plaintext on disk.")
	} else if config.LoadConfig().PairedDeviceCount() == 0 {
		// Pairing hash is on disk but every token has been revoked
		ui.Warn("Pairing configured, but no devices are paired.")
		ui.Faint("Run `emos config rotate-pairing` for a fresh code to share.")
	} else {
		ui.Info("Pairing already configured.")
		ui.Faint("Lost the code? Run `emos config rotate-pairing` to issue a fresh one.")
	}
	fmt.Println()

	if scanURL != "" {
		ui.Info("Or scan with a phone on the same network:")
		target := scanURL + "/"
		if pairingCode != "" {
			target = scanURL + "/#/pair?code=" + pairingCode
		}
		fmt.Println()
		printQR(target)
	}
}

func printQR(url string) {
	qr, err := qrcode.New(url, qrcode.Low)
	if err != nil {
		return
	}
	// ToSmallString already prints two cells per character so it fits a terminal
	out := qr.ToSmallString(false)
	// Indent so it sits inside our UI rail.
	for _, line := range strings.Split(out, "\n") {
		fmt.Println("  " + line)
	}
}

// --- subcommands ---

var serveInstallServiceCmd = &cobra.Command{
	Use:   "install-service",
	Short: "Install a systemd unit that starts the dashboard at boot",
	Run: func(cmd *cobra.Command, args []string) {
		bin, err := os.Executable()
		if err != nil {
			ui.Error("Could not determine emos binary path: " + err.Error())
			os.Exit(1)
		}
		// `os.Executable` may return /tmp/<garbage> on `go run` — point at the
		// installed binary instead when possible.
		if _, err := os.Stat("/usr/local/bin/emos"); err == nil {
			bin = "/usr/local/bin/emos"
		}
		user := os.Getenv("SUDO_USER")
		if user == "" {
			user = os.Getenv("USER")
		}
		unit := installer.DashboardUnit(bin, user, config.DashboardPort())
		if err := unit.Install(true, true); err != nil {
			ui.Error("Install failed: " + err.Error())
			os.Exit(1)
		}
		ui.Success("Dashboard service installed and started.")
		ui.Info("Manage with: sudo systemctl {status,restart,stop} " + unit.Name)
	},
}

var serveUninstallServiceCmd = &cobra.Command{
	Use:   "uninstall-service",
	Short: "Remove the dashboard's systemd unit",
	Run: func(cmd *cobra.Command, args []string) {
		unit := installer.DashboardUnit("", "", 0)
		if err := unit.Uninstall(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}
		ui.Success("Dashboard service removed.")
	},
}

func init() {
	serveCmd.AddCommand(serveInstallServiceCmd)
	serveCmd.AddCommand(serveUninstallServiceCmd)
}
