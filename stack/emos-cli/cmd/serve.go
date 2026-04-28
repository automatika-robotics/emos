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

The dashboard is reachable at http://emos.local:8765 (mDNS) or at the
device's IP address. On first launch, a six-digit pairing code is printed
to the terminal — use it once in the browser to issue a long-lived token.`,
	Run: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8765", "address to bind (host:port)")
	serveCmd.Flags().BoolVar(&serveDisableMDNS, "no-mdns", false, "skip mDNS announcement")
	serveCmd.Flags().BoolVar(&serveDisableAuth, "no-auth", false, "DEV ONLY: accept unauthenticated requests")
	serveCmd.Flags().BoolVar(&serveQRCodeOnly, "qr", false, "print a QR code with the dashboard URL and exit")
	serveCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "log every HTTP request (default: only mutations and errors)")
	rootCmd.AddCommand(serveCmd)
}

// NOTE: DashboardServiceUnit is the canonical systemd unit name for the daemon.
// Hardcoded here (and in installer.DashboardUnit) because both call sites
// need to agree on it.
const DashboardServiceUnit = "emos-dashboard.service"

func runServe(cmd *cobra.Command, args []string) {
	ui.Banner(config.Version)

	if serveQRCodeOnly {
		if u := qrURL(serveAddr); u != "" {
			printQR(u + "/")
		}
		return
	}

	// If the dashboard is already running as a systemd service, don't try to
	// print a friendly summary
	if installer.IsActive(DashboardServiceUnit) {
		PrintDashboardAccessSummary(serveAddr, "service", "")
		return
	}

	logLevel := slog.LevelInfo
	if serveVerbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	srv, err := server.New(server.Options{
		Addr:        serveAddr,
		DisableMDNS: serveDisableMDNS,
		DisableAuth: serveDisableAuth,
		UI:          webui.FS(),
		Logger:      logger,
	})
	if err != nil {
		ui.Error("Failed to initialise dashboard: " + err.Error())
		os.Exit(1)
	}

	printBootBanner(serveAddr, srv.PairingCode())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		ui.Error("Dashboard server exited: " + err.Error())
		os.Exit(1)
	}
	ui.Info("Dashboard stopped.")
}

// dashboardURLs lists every URL the dashboard can plausibly be reached at,
// in priority order (most-likely-to-work first)
func dashboardURLs(addr string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []string{"http://emos.local" + addr}
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		// Explicit bind — single URL.
		return []string{fmt.Sprintf("http://%s:%s", host, port)}
	}
	urls := []string{fmt.Sprintf("http://localhost:%s", port)}
	for _, ip := range server.LocalIPv4Strings() {
		urls = append(urls, fmt.Sprintf("http://%s:%s", ip, port))
	}
	urls = append(urls, fmt.Sprintf("http://emos.local:%s", port))
	return urls
}

// PrintDashboardAccessSummary is the single human readable success block
// for the dashboard
func PrintDashboardAccessSummary(addr, origin, freshCode string) {
	urls := dashboardURLs(addr)
	scanURL := qrURL(addr)

	ui.Header("EMOS DASHBOARD")
	switch origin {
	case "service":
		ui.Info("The dashboard is already running as a systemd service.")
	case "install":
		ui.Success("Dashboard service enabled and started.")
	}

	fmt.Println()
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
		ui.Faint("Lost the code? Run `emos serve revoke` to issue a fresh one.")
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
	ui.Faint("  systemctl status " + DashboardServiceUnit)
	ui.Faint("  systemctl restart " + DashboardServiceUnit)
	ui.Faint("  journalctl -u " + DashboardServiceUnit + " -f")
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
func printBootBanner(addr, pairingCode string) {
	urls := dashboardURLs(addr)
	scanURL := qrURL(addr)

	ui.Header("EMOS DASHBOARD")
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
	} else {
		ui.Info("Pairing already configured.")
		ui.Faint("Lost the code? Run `emos serve revoke` to issue a fresh one.")
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

var serveRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke all bearer tokens and rotate the pairing code",
	Run: func(cmd *cobra.Command, args []string) {
		auth, err := server.NewAuthForCLI()
		if err != nil {
			ui.Error("Could not load auth state: " + err.Error())
			os.Exit(1)
		}
		if err := auth.RevokeAll(); err != nil {
			ui.Error("Revoke failed: " + err.Error())
			os.Exit(1)
		}
		code, err := auth.RegeneratePairingCode()
		if err != nil {
			ui.Error("Could not regenerate code: " + err.Error())
			os.Exit(1)
		}
		ui.Success("All tokens revoked.")
		ui.Info("New pairing code (shown once): " + code)
		ui.Faint("Save it now — it is not stored in plaintext on disk.")
	},
}

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
		unit := installer.DashboardUnit(bin, user, 8765)
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
	serveCmd.AddCommand(serveRevokeCmd)
	serveCmd.AddCommand(serveInstallServiceCmd)
	serveCmd.AddCommand(serveUninstallServiceCmd)
}
