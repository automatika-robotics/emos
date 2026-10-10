package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Show or manage the EMOS license of this robot",
	Long: "An EMOS license names the robot it is for and gives its holder professional\n" +
		"support. It is verified once and then kept on this machine, where it outlives\n" +
		"an uninstall.",
	Args: cobra.NoArgs,
	RunE: runLicenseShow,
}

func init() {
	licenseCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the license kept on this machine",
		Args:  cobra.NoArgs,
		RunE:  runLicenseShow,
	})
	licenseCmd.AddCommand(&cobra.Command{
		Use:   "activate <key>",
		Short: "Verify a license key, keep it and install the robot plugin it is for",
		Args:  cobra.ExactArgs(1),
		RunE:  runLicenseActivate,
	})
	licenseCmd.AddCommand(&cobra.Command{
		Use:   "refresh",
		Short: "Verify the kept license again and update what is known about it",
		Args:  cobra.NoArgs,
		RunE:  runLicenseRefresh,
	})
	licenseCmd.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove the license from this machine, for example before handing the robot over",
		Args:  cobra.NoArgs,
		RunE:  runLicenseRemove,
	})
}

func runLicenseShow(cmd *cobra.Command, args []string) error {
	lic := config.LoadLicense()
	if lic == nil {
		ui.Info("There is no license on this machine.")
		ui.Faint("Activate one with 'emos license activate <key>'.")
		licenseNudge()
		return nil
	}
	printLicense(lic)
	return nil
}

func runLicenseActivate(cmd *cobra.Command, args []string) error {
	key := strings.TrimSpace(args[0])
	if old := config.LoadLicense(); old != nil && old.Key != key {
		ui.Warn("This machine already has a license, for " + licensedRobot(old) + ".")
		if !ui.Confirm("Replace it?") {
			return fmt.Errorf("aborted by user")
		}
	}

	lic, err := verifyLicense(key, false)
	if err != nil {
		return err
	}
	if err := config.SaveLicense(lic); err != nil {
		return fmt.Errorf("could not save the license: %w", err)
	}
	ui.Success("License verified and saved.")
	printLicense(lic)
	fmt.Println()

	cfg := config.LoadConfig()
	switch {
	case !cfg.IsInstalled():
		ui.Info("EMOS is not installed yet. 'emos install' sets it up with the " + licensedRobot(lic) + " plugin.")
		return nil
	case cfg.Plugin != nil && cfg.Plugin.Slug == lic.PluginSlug:
		ui.Success("The " + licensedRobot(lic) + " plugin is already installed.")
		return nil
	}
	return runPluginInstall(cmd, []string{lic.PluginSlug})
}

func runLicenseRefresh(cmd *cobra.Command, args []string) error {
	old := config.LoadLicense()
	if old == nil {
		ui.Error("There is no license on this machine to refresh.")
		ui.Faint("Activate one with 'emos license activate <key>'.")
		return fmt.Errorf("no license")
	}
	lic, err := verifyLicense(old.Key, true)
	if err != nil {
		ui.Faint("The license kept on this machine is unchanged.")
		return err
	}
	if err := config.SaveLicense(lic); err != nil {
		return fmt.Errorf("could not save the license: %w", err)
	}
	ui.Success("License verified again.")
	printLicense(lic)
	if lic.PluginSlug != old.PluginSlug {
		fmt.Println()
		ui.Warn("This license is now for " + licensedRobot(lic) + ", it was for " + licensedRobot(old) + ".")
		ui.Faint("Install its plugin with 'emos plugin install " + lic.PluginSlug + "'.")
	}
	return nil
}

func runLicenseRemove(cmd *cobra.Command, args []string) error {
	lic := config.LoadLicense()
	if lic == nil {
		ui.Info("There is no license on this machine.")
		return nil
	}
	ui.Warn("This removes the license for " + licensedRobot(lic) + " from this machine.")
	ui.Faint("The license itself stays valid, and the robot plugin stays installed.")
	if !ui.Confirm("Remove it?") {
		return fmt.Errorf("aborted by user")
	}
	if err := config.RemoveLicense(); err != nil {
		return fmt.Errorf("could not remove the license: %w", err)
	}
	ui.Success("License removed. 'emos license activate <key>' puts it back.")
	return nil
}

// verifyLicense asks the portal about key and explains a refusal to the
// operator; accepted says the portal has accepted this key before. The returned
// error is the short one cobra reports.
func verifyLicense(key string, accepted bool) (*config.License, error) {
	var lic *config.License
	err := ui.Spinner("Verifying the license...", func() error {
		var e error
		lic, e = api.VerifyLicense(key)
		return e
	})
	switch {
	case err == nil:
		return lic, nil
	case errors.Is(err, api.ErrInvalidLicense) && accepted:
		ui.Error("The portal no longer accepts this license.")
		ui.Faint("Ask about it at " + config.SupportURL + ".")
		return nil, fmt.Errorf("license not accepted")
	case errors.Is(err, api.ErrInvalidLicense):
		ui.Error("The portal does not accept this key.")
		ui.Faint("Check it for typing mistakes. If it is right, ask for help at " + config.SupportURL + ".")
		return nil, fmt.Errorf("license not accepted")
	case errors.Is(err, api.ErrLicenseNotClaimed):
		ui.Error("This license has not been activated yet.")
		ui.Faint("Sign in at " + config.SupportURL + ", activate it with the robot's serial number and")
		ui.Faint("this key, accept the license agreement, then run this again.")
		return nil, fmt.Errorf("license not activated")
	}
	ui.Error("The license could not be verified: " + err.Error())
	return nil, fmt.Errorf("license not verified")
}

// licensedRobot names the robot a licence is for.
func licensedRobot(lic *config.License) string {
	if lic.PluginName != "" {
		return lic.PluginName
	}
	return lic.PluginSlug
}

// printLicense shows a licence as it is kept on this machine.
func printLicense(lic *config.License) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if lic.ClientName != "" {
		fmt.Fprintf(w, "  Licensed to:\t%s\n", lic.ClientName)
	}
	fmt.Fprintf(w, "  Robot:\t%s\n", licensedRobot(lic))
	if lic.Tier != "" {
		fmt.Fprintf(w, "  Tier:\t%s\n", strings.ToUpper(lic.Tier[:1])+lic.Tier[1:])
	}
	if lic.SerialNumber != "" {
		fmt.Fprintf(w, "  Serial number:\t%s\n", lic.SerialNumber)
	}
	fmt.Fprintf(w, "  Key:\t%s\n", redact(lic.Key))
	if lic.ClaimedAt != nil {
		fmt.Fprintf(w, "  Activated:\t%s\n", lic.ClaimedAt.Local().Format("2006-01-02"))
	}
	fmt.Fprintf(w, "  Verified:\t%s\n", lic.VerifiedAt.Local().Format("2006-01-02"))
	fmt.Fprintf(w, "  Support:\t%s\n", config.SupportURL)
	w.Flush()
}

// banner prints the CLI banner and, for a licensed install, whose it is.
func banner() {
	if lic := config.LoadLicense(); lic != nil {
		ui.Banner(config.Version, licensedLine(lic))
		return
	}
	ui.Banner(config.Version)
}

// licensedLine says in one line who a licence belongs to and which robot it is for.
func licensedLine(lic *config.License) string {
	if lic.ClientName == "" {
		return "Licensed for " + licensedRobot(lic)
	}
	return "Licensed to " + lic.ClientName + " for " + licensedRobot(lic)
}

// statusLicense is the licence part of 'emos status'.
func statusLicense() {
	fmt.Println()
	lic := config.LoadLicense()
	if lic == nil {
		ui.Info("License: none")
		licenseNudge()
		return
	}
	ui.Info("License:")
	printLicense(lic)
}

// licenseNudge tells someone without a licence what one brings. Only on a
// terminal, so it never lands in piped output or in a service's log.
func licenseNudge() {
	if config.LoadLicense() != nil || !term.IsTerminal(os.Stdout.Fd()) {
		return
	}
	ui.Faint("An EMOS license brings professional support and recipes customised for your robots:")
	ui.Faint(config.SalesEmail)
}
