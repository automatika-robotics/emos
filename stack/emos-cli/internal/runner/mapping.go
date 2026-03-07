package runner

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

const mappingContainerName = "emos-mapping"

func RunMapping() error {
	cfg := config.LoadConfig()
	mode := config.ModeLicensed
	if cfg != nil {
		mode = cfg.Mode
	}

	if mode == config.ModeNative {
		return runMappingNative(cfg)
	}
	return runMappingContainer(mode)
}

func runMappingContainer(mode config.InstallMode) error {
	mapName := ui.Input("Enter a map name", "map_default")

	ui.Header("EMOS - PRE-MAPPING SETUP")
	rmwImpl := "rmw_cyclonedds_cpp"

	ui.Info("Map Name: " + mapName)
	ui.Info("RMW Implementation: " + rmwImpl)
	ui.Info("Container: " + config.ContainerName)

	// Container management
	ui.Header("HOST & CONTAINER MANAGEMENT")
	killROSProcesses()

	if !container.Exists(config.ContainerName) {
		return fmt.Errorf("container '%s' does not exist — run 'emos install' first", config.ContainerName)
	}

	if container.IsRunning(config.ContainerName) {
		ui.Spinner("Stopping existing EMOS container...", func() error {
			return container.Stop(config.ContainerName)
		})
	}

	if err := ui.Spinner("Starting EMOS container...", func() error {
		return container.Start(config.ContainerName)
	}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// RMW Configuration
	ui.Header("RMW CONFIGURATION")
	ui.Info("Setting RMW_IMPLEMENTATION=" + rmwImpl)
	setRMWImpl(rmwImpl)

	// Hardware Launch
	ui.Header("HARDWARE & SENSOR LAUNCH")

	os.MkdirAll(config.LogsDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(config.LogsDir, fmt.Sprintf("mapping_%s_%s.log", mapName, timestamp))
	ui.Info("All output will be saved to: " + logFile)

	mappingLaunch := emosRoot + "/robot/launch/bringup_mapping.py"
	hasBringup := container.FileExists(config.ContainerName, mappingLaunch)

	if hasBringup {
		if err := ui.Spinner("Launching mapping hardware...", func() error {
			return container.ExecDetached(config.ContainerName,
				"source ros_entrypoint.sh && ros2 launch "+mappingLaunch)
		}); err != nil {
			return err
		}
	} else {
		ui.Warn("No bringup_mapping.py found in container.")
		ui.Faint("Ensure your mapping launch files and sensor drivers are running externally.")
		if !ui.Confirm("Continue without launching mapping hardware?") {
			return fmt.Errorf("aborted by user")
		}
	}

	// Node Verification (only in licensed mode with bringup)
	if hasBringup && mode == config.ModeLicensed {
		ui.Header("VERIFYING ROS2 NODES")
		time.Sleep(5 * time.Second)

		if err := verifyNodes(nil); err != nil {
			container.Stop(config.ContainerName)
			return err
		}

		ui.Header("FINAL CONFIGURATION")
		ui.Spinner("Deactivating autonomous mode...", func() error {
			return container.ExecDetached(config.ContainerName,
				"source ros_entrypoint.sh && ./"+emosRoot+"/robot/scripts/deactivate_autonomous_mode.sh")
		})
	}

	// Data Recording
	ui.Header("LAUNCHING DATA RECORDING")

	bagDir := emosRoot + "/maps"
	bagPath := bagDir + "/" + mapName
	topics := []string{"/lidar/raw", "/imu/raw", "/tf", "/tf_static"}

	if !ui.Confirm("Get your robot ready and confirm to start mapping data recording") {
		ui.Warn("User canceled mapping.")
		return nil
	}

	// Ensure bag dir exists
	container.Exec(config.ContainerName, "mkdir -p "+bagDir)

	// Check for existing bag
	if container.FileExists(config.ContainerName, bagPath+".tar.gz") {
		ui.Warn("A map data file already exists at " + bagPath + ".tar.gz")
		if !ui.Confirm("Do you want to overwrite it?") {
			return fmt.Errorf("user chose not to overwrite")
		}
		container.Exec(config.ContainerName, "rm -f '"+bagPath+".tar.gz'")
	}

	// Start recording in background
	recordCmd := fmt.Sprintf("source ros_entrypoint.sh && ros2 bag record -o '%s' --storage mcap --compression-mode file --compression-format zstd --topics %s",
		bagPath, strings.Join(topics, " "))
	bagProcess := exec.Command("docker", "exec", config.ContainerName, "bash", "-c", recordCmd)
	bagProcess.Stdout = os.Stdout
	bagProcess.Stderr = os.Stderr
	if err := bagProcess.Start(); err != nil {
		return fmt.Errorf("failed to start bag recording: %w", err)
	}

	ui.Info("Map Data Recording Started...")
	ui.Info("Press Ctrl+C once to end data recording process.")

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	done := make(chan error, 1)
	go func() {
		done <- bagProcess.Wait()
	}()

	select {
	case <-sigChan:
		ui.Info("Got Ctrl+C, terminating mapping...")
		bagProcess.Process.Kill()
		container.Restart(config.ContainerName)
		time.Sleep(5 * time.Second)

		// Zip and save
		err := ui.Spinner("Zipping & saving mapped data...", func() error {
			zipCmd := fmt.Sprintf("tar -czvf %s.tar.gz -C %s %s && rm -rf %s",
				bagPath, bagDir, mapName, bagPath)
			_, err := container.Exec(config.ContainerName, zipCmd)
			return err
		})
		if err != nil {
			return err
		}
		ui.Success("Map data saved to " + bagPath + ".tar.gz")

	case err := <-done:
		if err != nil {
			return fmt.Errorf("bag recording ended with error: %w", err)
		}
	}

	return nil
}

func runMappingNative(cfg *config.EMOSConfig) error {
	mapName := ui.Input("Enter a map name", "map_default")

	ui.Header("EMOS - PRE-MAPPING SETUP (NATIVE)")

	distro := cfg.ROSDistro
	wsPath := cfg.WorkspacePath
	rosSetup := filepath.Join("/opt/ros", distro, "setup.bash")
	sourceCmd := "source " + rosSetup
	if wsPath != "" {
		wsSetup := filepath.Join(wsPath, "install", "setup.bash")
		if _, err := os.Stat(wsSetup); err == nil {
			sourceCmd += " && source " + wsSetup
		}
	}

	ui.Info("Map Name: " + mapName)
	ui.Info("ROS Distro: " + distro)

	// Check for bringup_mapping.py on host
	mappingLaunch := filepath.Join(config.HomeDir, "emos", "robot", "launch", "bringup_mapping.py")
	if _, err := os.Stat(mappingLaunch); err == nil {
		ui.Spinner("Launching mapping hardware...", func() error {
			cmd := exec.Command("bash", "-c", sourceCmd+" && ros2 launch "+mappingLaunch+" &")
			return cmd.Start()
		})
		time.Sleep(5 * time.Second)
	} else {
		ui.Warn("No bringup_mapping.py found.")
		ui.Faint("Ensure your mapping launch files and sensor drivers are running on the host.")
		if !ui.Confirm("Continue without launching mapping hardware?") {
			return fmt.Errorf("aborted by user")
		}
	}

	// Data Recording
	ui.Header("LAUNCHING DATA RECORDING")

	bagDir := filepath.Join(config.HomeDir, "emos", "maps")
	os.MkdirAll(bagDir, 0755)
	bagPath := filepath.Join(bagDir, mapName)
	topics := []string{"/lidar/raw", "/imu/raw", "/tf", "/tf_static"}

	if !ui.Confirm("Get your robot ready and confirm to start mapping data recording") {
		ui.Warn("User canceled mapping.")
		return nil
	}

	// Check for existing bag
	if _, err := os.Stat(bagPath + ".tar.gz"); err == nil {
		ui.Warn("A map data file already exists at " + bagPath + ".tar.gz")
		if !ui.Confirm("Do you want to overwrite it?") {
			return fmt.Errorf("user chose not to overwrite")
		}
		os.Remove(bagPath + ".tar.gz")
	}

	recordCmd := fmt.Sprintf("%s && ros2 bag record -o '%s' --storage mcap --compression-mode file --compression-format zstd --topics %s",
		sourceCmd, bagPath, strings.Join(topics, " "))
	bagProcess := exec.Command("bash", "-c", recordCmd)
	bagProcess.Stdout = os.Stdout
	bagProcess.Stderr = os.Stderr
	if err := bagProcess.Start(); err != nil {
		return fmt.Errorf("failed to start bag recording: %w", err)
	}

	ui.Info("Map Data Recording Started...")
	ui.Info("Press Ctrl+C once to end data recording process.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	done := make(chan error, 1)
	go func() {
		done <- bagProcess.Wait()
	}()

	select {
	case <-sigChan:
		ui.Info("Got Ctrl+C, terminating mapping...")
		bagProcess.Process.Kill()
		time.Sleep(2 * time.Second)

		err := ui.Spinner("Zipping & saving mapped data...", func() error {
			cmd := exec.Command("tar", "-czvf", bagPath+".tar.gz", "-C", bagDir, mapName)
			if err := cmd.Run(); err != nil {
				return err
			}
			return os.RemoveAll(bagPath)
		})
		if err != nil {
			return err
		}
		ui.Success("Map data saved to " + bagPath + ".tar.gz")

	case err := <-done:
		if err != nil {
			return fmt.Errorf("bag recording ended with error: %w", err)
		}
	}

	return nil
}

func InstallMapEditor() error {
	ui.Info("Starting Map Editor Installation...")

	licenseKey := ui.Input("Please enter your license key to proceed", "")
	if licenseKey == "" {
		return fmt.Errorf("a license key is required")
	}

	var creds *api.Credentials
	err := ui.Spinner("Validating license key...", func() error {
		var e error
		creds, e = api.ValidateLicense(licenseKey)
		return e
	})
	if err != nil {
		return err
	}

	// Check for existing mapping container
	if container.Exists(mappingContainerName) {
		ui.Warn("An existing '" + mappingContainerName + "' container was found.")
		if !ui.Confirm("Overwrite existing map editor container?") {
			return fmt.Errorf("installation aborted by user")
		}
		ui.Spinner("Removing existing container...", func() error {
			return container.Remove(mappingContainerName)
		})
	}

	// Login
	err = ui.Spinner("Logging into Docker registry...", func() error {
		return container.Login(creds.Registry, creds.Username, creds.Password)
	})
	if err != nil {
		return err
	}

	// Build mapping image name
	baseImage := creds.ImageName
	if idx := strings.LastIndex(baseImage, ":"); idx != -1 {
		baseImage = baseImage[:idx]
	}
	mappingImage := creds.Registry + "/" + baseImage + ":mapping"

	ui.Info("Pulling Map Editor container image: " + mappingImage)
	ui.Faint("This may take several minutes.")
	if err := container.Pull(mappingImage); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	ui.Success("Pulled latest image.")

	// Create the container
	ui.Info("Creating '" + mappingContainerName + "' container...")
	display := os.Getenv("DISPLAY")
	err = container.RunWithArgs(mappingContainerName, mappingImage, []string{
		"-d", "-it",
		"--device=/dev/dri",
		"--group-add", "video",
		"--volume=/tmp/.X11-unix:/tmp/.X11-unix",
		"--env=DISPLAY=" + display,
	})
	if err != nil {
		return fmt.Errorf("failed to create mapping container: %w", err)
	}

	ui.Spinner("Stopping and finalizing installation...", func() error {
		return container.Stop(mappingContainerName)
	})

	fmt.Println()
	ui.SuccessBox("Map Editor installed successfully!")
	ui.Faint("Start editing with 'emos map edit <path_to_bag.tar.gz>'")
	return nil
}

func EditMap(bagFilePath string) error {
	// Validate input
	if !strings.HasSuffix(bagFilePath, ".tar.gz") {
		return fmt.Errorf("file must be a .tar.gz archive")
	}
	if _, err := os.Stat(bagFilePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", bagFilePath)
	}

	bagFilename := filepath.Base(bagFilePath)
	mapName := strings.TrimSuffix(bagFilename, ".tar.gz")

	// Check container exists
	if !container.Exists(mappingContainerName) {
		ui.Error("Mapping container '" + mappingContainerName + "' not found.")
		ui.Faint("Please run 'emos map install-editor' first.")
		return fmt.Errorf("mapping container not found")
	}

	// Stop if running, then start
	if container.IsRunning(mappingContainerName) {
		ui.Spinner("Stopping existing container...", func() error {
			return container.Stop(mappingContainerName)
		})
	}

	if err := ui.Spinner("Starting map editor container...", func() error {
		return container.Start(mappingContainerName)
	}); err != nil {
		return err
	}

	ui.Header("Processing Map File: " + mapName)

	// Allow GUI access
	ui.Faint("Temporarily allowing container GUI access...")
	exec.Command("xhost", "+local:docker").Run()

	// Copy and extract bag file
	err := ui.Spinner("Copying '"+bagFilename+"' into container...", func() error {
		return container.Cp(mappingContainerName, bagFilePath, "/tmp/")
	})
	if err != nil {
		return err
	}

	err = ui.Spinner("Extracting bag file inside container...", func() error {
		_, e := container.Exec(mappingContainerName,
			fmt.Sprintf("tar -xzf '/tmp/%s' -C /tmp/", bagFilename))
		return e
	})
	if err != nil {
		return err
	}

	// Run the editor (detached)
	ui.Info("Starting the editor...")
	container.ExecDetached(mappingContainerName, fmt.Sprintf(
		"source /ros_entrypoint.sh && ros2 run glim_ros glim_roseditor --map_path /tmp/dump --save_path /tmp --map_name %s",
		mapName))

	// Run TF node
	ui.Info("Starting Data TF...")
	container.ExecDetached(mappingContainerName,
		"source /ros_entrypoint.sh && ros2 run glim_ros glim_transformer_node /lidar/raw /imu/raw /lidar /imu base_link")

	// Start bag playback
	ui.Info("Starting ROS bag playback...")
	container.ExecDetached(mappingContainerName,
		fmt.Sprintf("source /ros_entrypoint.sh && ros2 bag play /tmp/%s/*", mapName))

	// Wait for editor to finish
	ui.Info("Edit your map and close the editor window when done...")
	for {
		out, _ := container.Top(mappingContainerName)
		if !strings.Contains(out, "glim_roseditor") {
			break
		}
		time.Sleep(5 * time.Second)
	}
	ui.Success("Editor process has exited.")

	// Check PCD output
	pcdPath := fmt.Sprintf("/tmp/%s.pcd", mapName)
	if !container.FileExists(mappingContainerName, pcdPath) {
		return fmt.Errorf("editor exited but no PCD file found at %s", pcdPath)
	}
	ui.Success("PCD file generated successfully.")

	// Copy output to host
	hostOutput := "./" + mapName + ".pcd"
	err = ui.Spinner("Copying '"+mapName+".pcd' to host...", func() error {
		return container.CpFrom(mappingContainerName, pcdPath, hostOutput)
	})
	if err != nil {
		return err
	}

	// Cleanup
	ui.Faint("Cleaning up...")
	container.Exec(mappingContainerName, fmt.Sprintf(
		"rm -rf /tmp/%s /tmp/dump /tmp/%s %s", mapName, bagFilename, pcdPath))
	ui.Spinner("Stopping the map editor container...", func() error {
		return container.Stop(mappingContainerName)
	})
	exec.Command("xhost", "-local:docker").Run()

	fmt.Println()
	ui.SuccessBox(fmt.Sprintf("Map editing complete! Your map is ready at '%s'", hostOutput))
	return nil
}
