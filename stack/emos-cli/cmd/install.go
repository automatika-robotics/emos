package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/server"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	installModeFlag   string
	installDistroFlag string
)

var installCmd = &cobra.Command{
	Use:   "install [license-key]",
	Short: "Install EMOS (container, native or pixi mode)",
	Long: "Install EMOS in container, native or pixi mode. EMOS is free to install and use.\n\n" +
		"A license key is optional. With one, given here or typed when asked, the install\n" +
		"also sets up the robot plugin the license is for.",
	Args: cobra.MaximumNArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installModeFlag, "mode", "",
		"Installation mode: container, native or pixi")
	installCmd.Flags().StringVar(&installDistroFlag, "distro", "",
		"ROS 2 distribution (jazzy, humble, kilted)")
}

func runInstall(cmd *cobra.Command, args []string) error {
	banner()

	install, err := chooseInstall()
	if err != nil {
		return err
	}
	// The key is checked before the long install, so a mistake shows now
	lic, err := installLicense(args)
	if err != nil {
		return err
	}
	if err := install(); err != nil {
		return err
	}
	if lic != nil {
		installLicensedRobot(cmd, lic)
	} else {
		licenseNudge()
	}
	offerDashboardAutoStart()
	return nil
}

// chooseInstall returns the installer of the mode given by --mode, or of the
// one the operator picks from the menu.
func chooseInstall() (func() error, error) {
	switch installModeFlag {
	case "container", "oss-container":
		return installOSSContainer, nil
	case "native":
		return installNative, nil
	case "pixi":
		return installPixi, nil
	case "":
		// Show interactive menu
	default:
		return nil, fmt.Errorf("unknown mode: %s (use container, native or pixi)", installModeFlag)
	}

	fmt.Println("  Welcome to EMOS - The Embodied Operating System")
	fmt.Println()
	fmt.Println("  How would you like to install EMOS?")

	choice := ui.Select("Select installation mode:", []string{
		"Container Install         (No ROS required - runs in Docker)",
		"Native Install            (Requires existing ROS 2 installation)",
		"Pixi Install              (Self-contained ROS via pixi - no system ROS needed)",
	})
	return []func() error{installOSSContainer, installNative, installPixi}[choice], nil
}

// installLicense settles which licence this install is for. Nil installs EMOS
// without one.
func installLicense(args []string) (*config.License, error) {
	var key string
	if len(args) == 1 {
		key = args[0]
	} else if lic := config.LoadLicense(); lic != nil {
		ui.Info("Using the license kept on this machine, for " + licensedRobot(lic) + ".")
		return lic, nil
	} else {
		key = ui.Input("EMOS license key (press Enter to install without one)", "")
	}
	if key = strings.TrimSpace(key); key == "" {
		return nil, nil
	}

	lic, err := verifyLicense(key, false)
	if err != nil {
		if ui.Confirm("Install EMOS without a license? 'emos license activate <key>' adds it later.") {
			return nil, nil
		}
		return nil, err
	}
	if err := config.SaveLicense(lic); err != nil {
		return nil, fmt.Errorf("could not save the license: %w", err)
	}
	ui.Success("License verified, for " + licensedRobot(lic) + ".")
	return lic, nil
}

// installLicensedRobot installs the robot plugin a licence is for. EMOS is
// installed by now, so a failure is reported and does not undo it.
func installLicensedRobot(cmd *cobra.Command, lic *config.License) {
	fmt.Println()
	ui.Info("This license is for " + licensedRobot(lic) + ", so its plugin is installed next.")
	if err := runPluginInstall(cmd, []string{lic.PluginSlug}); err != nil {
		ui.Error("The " + licensedRobot(lic) + " plugin was not installed: " + err.Error())
		ui.Faint("EMOS itself is installed. Install the plugin with 'emos plugin install " + lic.PluginSlug + "'.")
	}
}

func selectDistro() string {
	if installDistroFlag != "" {
		return installDistroFlag
	}
	choice := ui.Select("Select ROS 2 distribution:", []string{
		"Jazzy (Recommended)",
		"Humble",
		"Kilted",
	})
	switch choice {
	case 0:
		return "jazzy"
	case 1:
		return "humble"
	case 2:
		return "kilted"
	}
	return "jazzy"
}

func installOSSContainer() error {
	// Check Docker is installed
	if _, err := exec.LookPath("docker"); err != nil {
		ui.Error("Docker is not installed or not in PATH.")
		fmt.Println("  Please install Docker first: https://docs.docker.com/get-docker/")
		return fmt.Errorf("docker not found")
	}
	ui.Success("Docker detected.")

	distro := selectDistro()
	image := config.PublicImageTag(distro)

	// Check for existing container
	if container.Exists(config.ContainerName) {
		ui.Warn("An existing EMOS container was found.")
		if !ui.Confirm("This will REMOVE the existing container and perform a fresh installation. Are you sure?") {
			return fmt.Errorf("aborted by user")
		}
		if err := ui.Spinner("Removing existing container...", func() error {
			return container.Remove(config.ContainerName)
		}); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	os.MkdirAll(config.ConfigDir, 0755)

	// Pull public image (no login needed)
	fmt.Println()
	ui.Info("Pulling EMOS container image: " + image)
	ui.Faint("This may take several minutes depending on your network connection.")
	if err := container.Pull(image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	ui.Success("Pulled image successfully.")

	// Create directories
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "recipes"), 0755)
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "logs"), 0755)

	// Start container
	if err := ui.Spinner("Starting EMOS container...", func() error {
		return container.Run(config.ContainerName, image)
	}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Save config
	cfg := &config.EMOSConfig{
		Mode:      config.ModeOSSContainer,
		ROSDistro: distro,
		ImageTag:  image,
	}
	if err := config.SaveConfig(cfg); err != nil {
		ui.Warn("Failed to save config: " + err.Error())
	}

	fmt.Println()
	ui.SuccessBox("EMOS installed successfully (container mode)!")
	ui.Faint("Run 'emos pull <recipe>' to download a recipe, then 'emos run <recipe>' to execute it.")
	ui.Faint("Ensure your sensor drivers are running externally (host or separate containers).")
	return nil
}

func installNative() error {
	// Detect ROS 2 installations
	installs := installer.DetectROS()

	if len(installs) == 0 {
		ui.Error("No ROS 2 installation detected.")
		fmt.Println()
		fmt.Println("  Native install requires ROS 2. Options:")
		fmt.Println("    1. Install ROS 2 first: https://docs.ros.org/")
		fmt.Println("    2. Use container install instead (no ROS needed)")
		fmt.Println()
		if ui.Confirm("Switch to container install?") {
			return installOSSContainer()
		}
		return fmt.Errorf("no ROS 2 installation found")
	}

	var chosen installer.ROSInstallation
	if len(installs) == 1 {
		chosen = installs[0]
		ui.Success(fmt.Sprintf("ROS 2 %s detected at %s", capitalize(chosen.Distro), chosen.Path))
		// Skip confirmation if distro was explicitly provided (e.g. CI / non-interactive)
		if installDistroFlag == "" {
			if !ui.Confirm(fmt.Sprintf("Proceed with native EMOS installation for %s?", capitalize(chosen.Distro))) {
				return fmt.Errorf("aborted by user")
			}
		}
	} else {
		fmt.Println()
		ui.Info("Multiple ROS 2 installations detected:")
		options := make([]string, len(installs))
		for i, inst := range installs {
			options[i] = fmt.Sprintf("%s  (%s)", capitalize(inst.Distro), inst.Path)
		}
		idx := ui.Select("Select ROS 2 distribution:", options)
		chosen = installs[idx]
	}

	os.MkdirAll(config.ConfigDir, 0755)

	// Create directories
	wsPath := filepath.Join(config.HomeDir, "emos", "ros_ws")
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "recipes"), 0755)
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "logs"), 0755)

	ui.Header("INSTALLING EMOS PACKAGES")

	if err := installer.InstallNative(wsPath, chosen.Distro); err != nil {
		return err
	}

	// Save config
	cfg := &config.EMOSConfig{
		Mode:          config.ModeNative,
		ROSDistro:     chosen.Distro,
		WorkspacePath: wsPath,
	}
	if err := config.SaveConfig(cfg); err != nil {
		ui.Warn("Failed to save config: " + err.Error())
	}

	fmt.Println()
	ui.SuccessBox("EMOS installed successfully (native mode)!")
	ui.Faint("EMOS packages are now installed in /opt/ros/" + chosen.Distro + "/")
	ui.Faint("You can run recipes directly: python3 ~/emos/recipes/<recipe>/recipe.py")
	ui.Faint("Or use the CLI: emos pull <recipe> && emos run <recipe>")
	return nil
}

// pixiBuildEnv adds SKBUILD_STRICT_CONFIG=false so scikit-build-core ignores pixi's unknown config settings.
func pixiBuildEnv() []string {
	return append(os.Environ(), "SKBUILD_STRICT_CONFIG=false")
}

// pixiCloneArgs is the git invocation that fetches the EMOS workspace. The
// default branch, or ref when the binary was built from one.
func pixiCloneArgs(ref, url, dir string) []string {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	return append(args, url, dir)
}

func installPixi() error {
	// Pixi is required for this mode; fail early with install guidance.
	pixiBin, err := installer.ResolvePixi()
	if err != nil {
		ui.Error("pixi is required for a pixi-mode install but was not found.")
		fmt.Println()
		fmt.Println("  Install it with:")
		fmt.Println("    " + installer.PixiInstallHint)
		fmt.Println()
		fmt.Println("  Then restart your shell and re-run 'emos install'.")
		return fmt.Errorf("pixi not found")
	}
	ui.Success("pixi detected: " + pixiBin)

	projectDir := config.PixiDir

	// Clear any prior workspace at the canonical location.
	if _, err := os.Stat(projectDir); err == nil {
		if _, e := os.Stat(filepath.Join(projectDir, "pixi.toml")); e == nil {
			ui.Warn("An existing pixi workspace was found at " + projectDir)
			if !ui.Confirm("Reinstall? This removes and re-clones the workspace.") {
				return fmt.Errorf("aborted by user")
			}
		}
		if err := os.RemoveAll(projectDir); err != nil {
			return fmt.Errorf("failed to clear %s: %w", projectDir, err)
		}
	}

	os.MkdirAll(config.ConfigDir, 0755)
	if err := os.MkdirAll(filepath.Dir(projectDir), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}
	// User data dirs are shared across modes.
	os.MkdirAll(config.RecipesDir, 0755)
	os.MkdirAll(config.LogsDir, 0755)

	ui.Header("CLONING EMOS WORKSPACE")
	ui.Faint("Target: " + projectDir)
	if config.SourceRef != "" {
		ui.Faint("Source: " + config.SourceRef)
	}
	if err := ui.Spinner("Cloning EMOS repository...", func() error {
		c := exec.Command("git", pixiCloneArgs(config.SourceRef, config.RepoURL(), projectDir)...)
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		return nil
	}); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	if err := ui.Spinner("Fetching stack submodules...", func() error {
		c := exec.Command("git", "submodule", "update", "--init", "--depth", "1")
		c.Dir = projectDir
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		return nil
	}); err != nil {
		return fmt.Errorf("submodule init failed: %w", err)
	}

	// Resolve the env and build the stack.
	ui.Header("BUILDING EMOS PACKAGES (pixi)")
	ui.Faint("This can take 10-20 minutes on a first install.")

	if err := installer.RunPixi(projectDir, pixiBuildEnv(), "install"); err != nil {
		return err
	}

	if err := installer.RunPixi(projectDir, pixiBuildEnv(), "run", "setup"); err != nil {
		return err
	}

	// Re-save the full struct here so the canonical fields are authoritative
	// and any existing fields (auth, name) are preserved.
	cfg := config.LoadConfig()
	if cfg == nil {
		cfg = &config.EMOSConfig{}
	}
	cfg.Mode = config.ModePixi
	cfg.ROSDistro = "jazzy"
	cfg.PixiProjectDir = projectDir
	if err := config.SaveConfig(cfg); err != nil {
		ui.Warn("Failed to save config: " + err.Error())
	}

	fmt.Println()
	ui.SuccessBox("EMOS installed successfully (pixi mode)!")
	ui.Faint("Workspace: " + projectDir)
	ui.Faint("Run recipes with: emos pull <recipe> && emos run <recipe>")
	offerCUDAPackages(projectDir)
	return nil
}

// offerCUDAPackages offers to rebuild the packages that can use CUDA on a machine with it. A build that fails leaves the install on the CPU packages.
func offerCUDAPackages(projectDir string) {
	cuda := installer.DetectCUDA()
	if cuda == nil {
		return
	}
	packages := strings.Join(installer.CUDAPackages, " and ")
	fmt.Println()
	ui.Info(fmt.Sprintf("CUDA %s was detected at %s, so the CUDA-optimized versions of %s can be used.",
		cuda.Version, cuda.Root, packages))
	ui.Faint("They are compiled from source, which may take upto half an hour or more.")
	if !ui.Confirm(fmt.Sprintf("Build %s for CUDA %s now?", packages, cuda.Version)) {
		ui.Info("Keeping the CPU versions.")
		return
	}
	if err := installer.InstallCUDAPackages(projectDir, cuda.Root, pixiBuildEnv()); err != nil {
		ui.Error("The CUDA versions did not build, so the CPU versions will be installed: " + err.Error())
		ui.Faint("Run 'emos update' to be offered the build again.")
		return
	}
	ui.Success(fmt.Sprintf("%s now use CUDA %s.", packages, cuda.Version))
}

// offerDashboardAutoStart prompts the user to enable the dashboard at boot
// Soft-fails on systems without systemd or on permission errors
func offerDashboardAutoStart() {
	bin := "/usr/local/bin/emos"
	if _, err := os.Stat(bin); err != nil {
		return
	}
	port := config.DashboardPort()
	probe := installer.DashboardUnit(bin, "", port)
	if !probe.IsSupported() {
		return
	}
	fmt.Println()
	if !ui.Confirm("Enable the EMOS dashboard at boot? (browser-based onboarding console)") {
		return
	}

	// Establish auth state up-front so the service inherits it.
	auth, err := server.NewAuthForCLI()
	if err != nil {
		ui.Warn("Could not initialise dashboard auth state: " + err.Error())
		return
	}
	freshCode := auth.FreshPairingCode() // empty if pairing was already configured

	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	unit := installer.DashboardUnit(bin, user, port)
	if err := unit.Install(true, true); err != nil {
		ui.Warn("Could not enable dashboard service: " + err.Error())
		return
	}

	// Tiny grace period so `systemctl is-active` won't be racy if anyone
	// checks immediately after this returns.
	time.Sleep(400 * time.Millisecond)
	PrintDashboardAccessSummary(fmt.Sprintf(":%d", port), "install", freshCode)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
